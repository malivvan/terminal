package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
)

// Cell is a public, immutable snapshot of a single grid cell.
//
// It mirrors the unexported `cell` type but is safe to expose: callers
// cannot mutate it to confuse the Session, and it includes the rune width
// so wide-character handling can be reproduced by tests.
type Cell struct {
	Rune      rune
	Combining []rune
	Style     tcell.Style
	Width     int
	// Overline indicates the SGR 53 overline decoration is active.
	Overline bool
	// Protected indicates the cell is protected from selective erase (DECSCA).
	Protected bool
	// Wrapped indicates the line continued from the previous row without an
	// explicit newline.
	Wrapped bool
}

// String returns the rune (with combining marks) as a string. An empty
// cell renders as a single space, matching Session.String().
func (c Cell) String() string {
	r := c.Rune
	if r == 0 {
		r = ' '
	}
	if len(c.Combining) == 0 {
		return string(r)
	}
	out := []rune{r}
	out = append(out, c.Combining...)
	return string(out)
}

// bufferSurface is a Screen implementation that stores cells in
// memory instead of forwarding them to a tcell.Screen. It is the
// rendering target for Terminal and is safe to use from multiple
// goroutines.
type bufferSurface struct {
	mu    sync.RWMutex
	w, h  int
	cells [][]Cell
}

func newBufferSurface(w, h int) *bufferSurface {
	s := &bufferSurface{}
	s.resize(w, h)
	return s
}

func (s *bufferSurface) resize(w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.w, s.h = w, h
	s.cells = make([][]Cell, h)
	for i := range s.cells {
		s.cells[i] = make([]Cell, w)
	}
}

// SetContent implements Screen.
func (s *bufferSurface) SetContent(x, y int, ch rune, comb []rune, style tcell.Style) {
	s.setContent(x, y, ch, comb, style, 1, false, false, false)
}

// SetContentWidth is an optional extension that accepts the true character
// width and additional cell metadata. It is recognised by Session.Draw via a
// type assertion to avoid breaking the Screen interface.
func (s *bufferSurface) SetContentWidth(x, y int, ch rune, comb []rune, style tcell.Style, width int, overline, protected, wrapped bool) {
	s.setContent(x, y, ch, comb, style, width, overline, protected, wrapped)
}

func (s *bufferSurface) setContent(x, y int, ch rune, comb []rune, style tcell.Style, width int, overline, protected, wrapped bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if y < 0 || y >= s.h || x < 0 || x >= s.w {
		return
	}
	var combCopy []rune
	if len(comb) > 0 {
		combCopy = append([]rune(nil), comb...)
	}
	if width <= 0 {
		width = 1
	}
	s.cells[y][x] = Cell{
		Rune:      ch,
		Combining: combCopy,
		Style:     style,
		Width:     width,
		Overline:  overline,
		Protected: protected,
		Wrapped:   wrapped,
	}
}

// Size implements Screen.
func (s *bufferSurface) Size() (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.w, s.h
}

func (s *bufferSurface) get(x, y int) Cell {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if y < 0 || y >= s.h || x < 0 || x >= s.w {
		return Cell{}
	}
	return s.cells[y][x]
}

// outputSink is the io.ReadWriteCloser used as Session.pty for headless
// terminals. The Session writes here when responding to queries (DA,
// cursor reports, OSC52 echo, etc.) or when HandleEvent forwards a
// key. Read returns io.EOF — the Session never reads from it because the
// parser is driven manually.
type outputSink struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	w, h   int
	closed bool
}

func (o *outputSink) Read(p []byte) (int, error) { return 0, io.EOF }

func (o *outputSink) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return 0, io.ErrClosedPipe
	}
	return o.buf.Write(p)
}

func (o *outputSink) Close() error {
	o.mu.Lock()
	o.closed = true
	o.mu.Unlock()
	return nil
}

// Resize satisfies the optional hook checked by Session.resizeUnlocked.
func (o *outputSink) Resize(w, h int) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.w, o.h = w, h
	return nil
}

func (o *outputSink) drain() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := o.buf.Bytes()
	dup := make([]byte, len(out))
	copy(dup, out)
	o.buf.Reset()
	return dup
}

// Terminal drives a Session entirely in-process without any tcell.Screen.
// It is intended for tests, fuzzing, snapshot-driven assertions, and
// any environment where a real renderer is unavailable (such as
// js/wasm).
//
// Unlike Session.Start / Session.StartWithPty, the parser is driven
// synchronously by Feed: when Feed returns, all input bytes have been
// fully processed and the screen reflects the final state. There is
// no parser goroutine and no need to wait for redraw events.
//
// The zero value is not usable; construct via NewTerminal.
type Terminal struct {
	s       *Session
	surface *bufferSurface
	out     *outputSink

	mu     sync.Mutex
	parser *Parser
	closed bool

	// delta tracking
	prevFrame                    [][]Cell
	prevCursorCol, prevCursorRow int
	prevCursorVis                bool
}

// NewTerminal returns a Terminal terminal of the given dimensions.
func NewTerminal(width, height int) (*Terminal, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("terminal: headless dimensions must be positive")
	}
	t := &Terminal{
		s:       New(),
		surface: newBufferSurface(width, height),
		out:     &outputSink{w: width, h: height},
	}
	t.s.SetSurface(t.surface)
	// Wire s.pty to the output sink so HandleEvent (key/mouse input)
	// and any sequences the Session writes back land in our buffer.
	t.s.mu.Lock()
	t.s.pty = t.out
	t.s.mu.Unlock()
	t.s.Resize(width, height)

	// Build a parser without starting its goroutine. The sequences
	// channel must be buffered enough to hold what a single rune
	// transition can emit before we drain it.
	t.parser = &Parser{
		sequences: make(chan Sequence, 64),
		state:     ground,
	}
	return t, nil
}

// Session exposes the underlying Session for advanced configuration (logger,
// TERM, OSC8, custom event handler, etc.). The Terminal instance
// retains ownership; callers must not call Session.Close directly — use
// Terminal.Close.
func (t *Terminal) Session() *Session { return t.s }

// Feed processes input bytes synchronously: when it returns, every
// rune has been parsed and the screen has been painted to the buffer
// surface. Subsequent calls to Cell, Lines, String, etc. observe the
// resulting state.
func (t *Terminal) Feed(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return 0, io.ErrClosedPipe
	}
	for _, r := range string(b) {
		next := anywhere(r, t.parser)
		if next == nil {
			break
		}
		t.parser.state = next
		// Drain sequences emitted by this transition and dispatch
		// them to the Session. The channel is buffered to 64 — far more
		// than any single rune transition produces.
		drain := true
		for drain {
			select {
			case seq := <-t.parser.sequences:
				t.s.update(seq)
			default:
				drain = false
			}
		}
	}
	// Paint to the buffer surface and clear the Session dirty flag.
	t.s.Draw()
	return len(b), nil
}

// FeedString is a convenience wrapper around Feed.
func (t *Terminal) FeedString(s string) (int, error) { return t.Feed([]byte(s)) }

// HandleEvent forwards a tcell.Event (key/mouse/paste/focus) to the
// Session, exactly as the Terminal widget does. The corresponding bytes
// are captured in the output buffer, accessible via Output.
func (t *Terminal) HandleEvent(ev tcell.Event) bool { return t.s.HandleEvent(ev) }

// Resize changes the headless terminal's dimensions and propagates
// the change to the underlying Session.
func (t *Terminal) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	t.surface.resize(width, height)
	t.s.Resize(width, height)
	t.s.Draw()
}

// Size returns the current (width, height).
func (t *Terminal) Size() (int, int) { return t.surface.Size() }

// Cell returns a snapshot of the cell at the given coordinates. Out
// of range coordinates yield a zero Cell.
func (t *Terminal) Cell(x, y int) Cell { return t.surface.get(x, y) }

// Cursor returns (col, row, visible).
func (t *Terminal) Cursor() (col, row int, visible bool) {
	r, c, _, vis := t.s.Cursor()
	return c, r, vis
}

// Output returns and clears the bytes the Session has written back through
// its pty side (replies to DA/cursor reports, OSC52 echoes, key
// bytes from HandleEvent, etc.).
func (t *Terminal) Output() []byte { return t.out.drain() }

// String returns the visible screen as a newline-separated string,
// matching Session.String().
func (t *Terminal) String() string { return t.s.String() }

// ScrollbackLen returns the number of lines currently saved in the
// scrollback buffer.  Scrollback is only populated for the primary
// screen.
func (t *Terminal) ScrollbackLen() int {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	return len(t.s.scrollback)
}

// ScrollbackLine returns the content of scrollback row n as a string,
// or ("", false) if n is out of range.
func (t *Terminal) ScrollbackLine(n int) (string, bool) {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	if n < 0 || n >= len(t.s.scrollback) {
		return "", false
	}
	line := t.s.scrollback[n]
	buf := make([]rune, 0, len(line))
	for _, c := range line {
		if c.content == 0 {
			buf = append(buf, ' ')
		} else {
			buf = append(buf, c.content)
			buf = append(buf, c.combining...)
		}
	}
	return string(buf), true
}

// GetModes returns a snapshot of the current terminal mode state.
// See Session.GetModes for details.
func (t *Terminal) GetModes() Modes { return t.s.GetModes() }

// DeltaCell describes a single cell that has changed since the last
// delta snapshot.
type DeltaCell struct {
	X, Y  int
	Rune  rune
	Style tcell.Style
}

// ScreenDelta holds the set of cells that changed since the last
// call to GetDelta, plus optional cursor change info.
type ScreenDelta struct {
	Changed       []DeltaCell
	CursorChanged bool
	CursorCol     int
	CursorRow     int
	CursorVisible bool
}

// GetDelta returns only the cells that have changed since the last call.
// On first call, the entire screen is returned as changed.
func (t *Terminal) GetDelta() ScreenDelta {
	t.mu.Lock()
	defer t.mu.Unlock()

	w, hh := t.surface.Size()
	var delta ScreenDelta
	delta.CursorCol, delta.CursorRow, delta.CursorVisible = t.Cursor()

	// Initialize previous frame if needed.
	if t.prevFrame == nil {
		t.prevFrame = make([][]Cell, hh)
		for y := 0; y < hh; y++ {
			t.prevFrame[y] = make([]Cell, w)
		}
	}

	for y := 0; y < hh; y++ {
		for x := 0; x < w; x++ {
			cur := t.surface.get(x, y)
			prev := t.prevFrame[y][x]
			if cur.Rune != prev.Rune || cur.Style != prev.Style {
				delta.Changed = append(delta.Changed, DeltaCell{
					X: x, Y: y,
					Rune:  cur.Rune,
					Style: cur.Style,
				})
				t.prevFrame[y][x] = cur
			}
		}
	}

	if t.prevCursorCol != delta.CursorCol || t.prevCursorRow != delta.CursorRow || t.prevCursorVis != delta.CursorVisible {
		delta.CursorChanged = true
		t.prevCursorCol = delta.CursorCol
		t.prevCursorRow = delta.CursorRow
		t.prevCursorVis = delta.CursorVisible
	}

	return delta
}

// WaitForText blocks until text appears anywhere on the visible screen,
// returning the (col, row) of the first match or an error on timeout.
// The search checks all visible lines on each poll tick.
func (t *Terminal) WaitForText(text string, timeout time.Duration) (col, row int, err error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		lines := t.Lines()
		for y, line := range lines {
			if x := strings.Index(line, text); x >= 0 {
				return x, y, nil
			}
		}
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return 0, 0, fmt.Errorf("WaitForText(%q): timed out after %v", text, timeout)
			}
		}
	}
}

// WaitForCursor blocks until the cursor reaches (col, row), or times out.
func (t *Terminal) WaitForCursor(col, row int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		c, r, _ := t.Cursor()
		if c == col && r == row {
			return nil
		}
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("WaitForCursor(%d,%d): timed out after %v (at %d,%d)", col, row, timeout, c, r)
			}
		}
	}
}

// WaitForStable blocks until the screen has not changed for quietTime,
// or until maxWait elapses. Useful for waiting for long-running commands
// to finish producing output.
func (t *Terminal) WaitForStable(quietTime, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	var lastSnapshot string
	stableSince := time.Now()
	for {
		current := t.String()
		if current != lastSnapshot {
			lastSnapshot = current
			stableSince = time.Now()
		} else if time.Since(stableSince) >= quietTime {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("WaitForStable: timed out after %v", maxWait)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// WaitForPattern blocks until a line on the visible screen matches re,
// returning the line text, row index, or an error on timeout.
func (t *Terminal) WaitForPattern(re *regexp.Regexp, timeout time.Duration) (line string, y int, err error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		lines := t.Lines()
		for i, l := range lines {
			if re.MatchString(l) {
				return l, i, nil
			}
		}
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return "", 0, fmt.Errorf("WaitForPattern(%s): timed out after %v", re, timeout)
			}
		}
	}
}

// ExpectEcho sends input bytes to the terminal and waits for them to
// appear in the output buffer (simulating local echo). Returns the
// echoed bytes or an error on timeout. Useful for verifying that a
// PTY is responding correctly.
func (t *Terminal) ExpectEcho(input string, timeout time.Duration) (echoed string, ok bool) {
	deadline := time.Now().Add(timeout)
	t.HandleEvent(tcell.NewEventKey(tcell.KeyRune, input, tcell.ModNone))
	for {
		out := t.Output()
		if len(out) > 0 {
			return string(out), true
		}
		if time.Now().After(deadline) {
			return "", false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// SendCommandAndWait is a high-level convenience for shell automation:
// it writes a command followed by Enter, waits for the echo and the
// prompt to reappear, and returns the accumulated output.
func (t *Terminal) SendCommandAndWait(cmd string, prompts []string, timeout time.Duration) (output string, err error) {
	// Send the command.
	t.FeedString(cmd + "\r")
	deadline := time.Now().Add(timeout)

	// Wait for prompt to reappear.
	var b strings.Builder
	for {
		out := t.Output()
		if len(out) > 0 {
			b.Write(out)
		}
		lines := t.Lines()
		for _, line := range lines {
			for _, p := range prompts {
				if strings.Contains(line, p) {
					return strings.TrimSpace(b.String()), nil
				}
			}
		}
		if time.Now().After(deadline) {
			return strings.TrimSpace(b.String()), fmt.Errorf("SendCommandAndWait: timed out waiting for prompt after %v", timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Lines returns the visible screen as a slice of strings, one per row.
func (t *Terminal) Lines() []string {
	w, hh := t.surface.Size()
	out := make([]string, hh)
	for y := 0; y < hh; y++ {
		buf := make([]rune, 0, w)
		for x := 0; x < w; x++ {
			c := t.surface.get(x, y)
			if c.Rune == 0 {
				buf = append(buf, ' ')
			} else {
				buf = append(buf, c.Rune)
				buf = append(buf, c.Combining...)
			}
		}
		out[y] = string(buf)
	}
	return out
}

// Close releases resources. Subsequent calls to Feed return
// io.ErrClosedPipe. Safe to call multiple times.
func (t *Terminal) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()
	_ = t.out.Close()
	// Drop the pty reference before Session.Close so it does not try to
	// double-close our output sink.
	t.s.mu.Lock()
	t.s.pty = nil
	t.s.mu.Unlock()
	t.s.Close()
	return nil
}

// LineType classifies the semantic role of a screen line.
type LineType int

const (
	// LineTypeUnknown is the default for unclassified lines.
	LineTypeUnknown LineType = iota
	// LineTypePrompt indicates a shell prompt (ends with $, #, >, %).
	LineTypePrompt
	// LineTypeCommand is a line printed immediately after a prompt.
	LineTypeCommand
	// LineTypeOutput is program output.
	LineTypeOutput
	// LineTypeError indicates an error (red colour or error keywords).
	LineTypeError
	// LineTypeStatusBar is a bottom-of-screen inverted line.
	LineTypeStatusBar
	// LineTypeBlank is an empty line.
	LineTypeBlank
)

// String returns a human-readable name for the line type.
func (lt LineType) String() string {
	switch lt {
	case LineTypeUnknown:
		return "unknown"
	case LineTypePrompt:
		return "prompt"
	case LineTypeCommand:
		return "command"
	case LineTypeOutput:
		return "output"
	case LineTypeError:
		return "error"
	case LineTypeStatusBar:
		return "statusbar"
	case LineTypeBlank:
		return "blank"
	default:
		return "unknown"
	}
}

// ScreenLine is a single row of the screen annotated with semantic metadata.
type ScreenLine struct {
	Text   string
	Styles []tcell.Style
	Type   LineType
}

var (
	// promptRE matches common shell prompt patterns.
	promptRE = regexp.MustCompile(`.*[$#%>❯]\s*$`)
	// errorKeywordsRE matches common error-indicating words.
	errorKeywordsRE = regexp.MustCompile(`(?i)\b(error|fail(ed|ure)?|fatal|exception|panic|warning|E\d{4})\b`)
)

// AnalyzeScreen scans the visible screen and classifies each line by
// its likely semantic role. It uses heuristics: terminal prompt patterns,
// colour, and keyword matching.
func (t *Terminal) AnalyzeScreen() []ScreenLine {
	w, hh := t.surface.Size()
	lines := make([]ScreenLine, hh)
	for y := 0; y < hh; y++ {
		buf := make([]rune, 0, w)
		styles := make([]tcell.Style, 0, w)
		for x := 0; x < w; x++ {
			c := t.surface.get(x, y)
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			buf = append(buf, r)
			styles = append(styles, c.Style)
		}
		text := strings.TrimRight(string(buf), " ")
		lines[y] = ScreenLine{Text: text, Styles: styles, Type: classifyLine(text, styles, y, hh)}
	}
	// Mark the line after a prompt as a command.
	for i := 1; i < hh; i++ {
		if lines[i-1].Type == LineTypePrompt && lines[i].Type == LineTypeUnknown {
			lines[i].Type = LineTypeCommand
		}
	}
	return lines
}

// classifyLine applies heuristics to determine a line's semantic type.
func classifyLine(text string, styles []tcell.Style, y, totalRows int) LineType {
	if text == "" {
		return LineTypeBlank
	}

	// Status bar heuristic: bottom row with any reverse-video cells.
	if y == totalRows-1 {
		for _, s := range styles {
			if s.HasReverse() {
				return LineTypeStatusBar
			}
		}
	}

	// Error detection: red foreground or error keywords.
	isRed := false
	for _, s := range styles {
		fg := s.GetForeground()
		if isRedColor(fg) {
			isRed = true
			break
		}
	}
	if isRed || errorKeywordsRE.MatchString(text) {
		return LineTypeError
	}

	// Prompt detection: ends with shell prompt pattern.
	if promptRE.MatchString(text) {
		return LineTypePrompt
	}

	return LineTypeOutput
}

// isRedColor returns true if the color appears to be a red shade.
func isRedColor(c interface{}) bool {
	switch v := c.(type) {
	case tcell.Color:
		// ANSI red or bright red
		return v == tcellcolor.Red || v == tcellcolor.Maroon ||
			v == tcellcolor.DarkRed || v == tcellcolor.OrangeRed
	default:
		return false
	}
}

// FindPrompt locates the last shell prompt on the screen.
// Returns the line index or -1 if not found.
func (t *Terminal) FindPrompt() int {
	lines := t.AnalyzeScreen()
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].Type == LineTypePrompt {
			return i
		}
	}
	return -1
}

// GetLastOutput returns the text output from the last prompt to the
// current cursor position.
func (t *Terminal) GetLastOutput() string {
	promptLine := t.FindPrompt()
	if promptLine < 0 {
		return ""
	}
	_, cursorRow, _ := t.Cursor()
	var b strings.Builder
	for y := promptLine + 1; y <= cursorRow; y++ {
		lines := t.Lines()
		if y < len(lines) {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(strings.TrimRight(lines[y], " "))
		}
	}
	return strings.TrimSpace(b.String())
}

// GetErrorLine returns the first error-classified line, or ("", -1, false).
func (t *Terminal) GetErrorLine() (text string, line int, ok bool) {
	lines := t.AnalyzeScreen()
	for i, l := range lines {
		if l.Type == LineTypeError {
			return l.Text, i, true
		}
	}
	return "", -1, false
}

package terminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/pty"
	"github.com/mattn/go-runewidth"
)

type (
	column int
	row    int
)

// EventTerminal is a generic terminal event
type EventTerminal struct {
	when time.Time
	s    *Session
}

func newEventTerminal(s *Session) *EventTerminal {
	return &EventTerminal{
		when: time.Now(),
		s:    s,
	}
}

func (ev *EventTerminal) When() time.Time {
	return ev.when
}

func (ev *EventTerminal) Session() *Session {
	return ev.s
}

// EventRedraw is emitted when the terminal requires redrawing
type EventRedraw struct {
	*EventTerminal
}

// EventClosed is emitted when the terminal exits
type EventClosed struct {
	*EventTerminal
}

// EventTitle is emitted when the terminal's title changes
type EventTitle struct {
	*EventTerminal
	title string
}

func (ev *EventTitle) Title() string {
	return ev.title
}

// EventMouseMode is emitted when the terminal mouse mode changes
type EventMouseMode struct {
	modes []tcell.MouseFlags

	*EventTerminal
}

func (ev *EventMouseMode) Flags() []tcell.MouseFlags {
	return ev.modes
}

// EventBell is emitted when BEL is received
type EventBell struct {
	*EventTerminal
}

type EventPanic struct {
	*EventTerminal
	Error error
}

// EventClipboard is emitted when an OSC 52 clipboard sequence is received
type EventClipboard struct {
	*EventTerminal
	selection string
	data      string
}

func (ev *EventClipboard) Selection() string {
	return ev.selection
}

func (ev *EventClipboard) Data() string {
	return ev.data
}

// EventDCS is emitted when a DCS (Device Control String) sequence begins.
// External handlers can use this to implement Sixel rendering, DECRQSS,
// or other DCS-based protocols.
type EventDCS struct {
	*EventTerminal
	Final      rune
	Parameters []int
}

// EventDCSData is emitted for each data rune within a DCS sequence.
type EventDCSData struct {
	*EventTerminal
	Data rune
}

// EventColour is emitted when an OSC 4 sequence changes a palette entry.
type EventColour struct {
	*EventTerminal
	Index int
	Value string
}

// EventDefaultColour is emitted when OSC 10/11 changes the default fg/bg.
type EventDefaultColour struct {
	*EventTerminal
	IsForeground bool
	Value        string
}

// EventLogEntry is a lightweight record of a terminal event suitable for
// debugging and audit trails. It is stored in a bounded ring-buffer on Session.
type EventLogEntry struct {
	Time     time.Time
	Type     string      // "CSI", "OSC", "C0", "ESC", "ModeSet", "ModeReset", "Title"
	Sequence string      // raw sequence or description
	Args     interface{} // parsed parameters
}

// Session models a virtual terminal
type Session struct {
	Logger *log.Logger
	// If true, OSC8 enables the output of OSC8 strings. Otherwise, any OSC8
	// sequences will be stripped
	OSC8 bool
	// Set the TERM environment variable to be passed to the command's
	// environment. If not set, xterm-256color will be used
	TERM string
	// MaxClipboardLen is the maximum size in bytes of OSC 52 clipboard
	// data that will be forwarded via EventClipboard. Data exceeding
	// this limit is silently dropped and a warning is logged. Default
	// is 1 MB (1 << 20). Set to 0 to disable the limit.
	MaxClipboardLen int

	mu sync.Mutex

	activeScreen  [][]cell
	altScreen     [][]cell
	primaryScreen [][]cell

	charsets charsets
	cursor   cursor
	margin   margin
	mode     mode
	sShift   charset
	tabStop  []column
	// lastCol is a flag indicating we printed in the last col
	lastCol bool

	primaryState cursorState
	altState     cursorState

	cmd           *sessionCmd
	dirty         bool
	eventHandler  func(tcell.Event)
	redrawHandler func()
	parser        *Parser
	pty           io.ReadWriteCloser
	surface       Screen
	events        chan tcell.Event

	mouseBtn tcell.ButtonMask

	// inDeccolmResize guards against recursion when DECCOLM triggers a resize.
	inDeccolmResize bool

	// scrollback is the history of lines that scrolled off the top of the
	// primary screen. scrollOffset > 0 means the viewport is shifted back
	// into this history (0 = show live screen). scrollbackLimit caps the
	// number of saved lines; 0 means use the default of 10 000.
	scrollback      [][]cell
	scrollOffset    int
	scrollbackLimit int

	// clipboard stores decoded OSC 52 clipboard data by selection name.
	clipboard map[string]string

	// eventLog records the most recent sequence/mode events for debugging.
	// Bounded to eventLogMax entries.
	eventLog []EventLogEntry
}

// sessionCmd is the handle to the process a Session started on its pty.
//
// It is nil when the session was started with StartWithPty, which spawns no
// process of its own, and it is deliberately not *exec.Cmd: a command started
// on Unix is an *exec.Cmd, while one started inside a Windows pseudo-console is
// a *pty.Cmd, and both are reduced here to the process and the call that reaps
// it.
type sessionCmd struct {
	// process is the running process, or nil before it has been started.
	process *os.Process

	// wait blocks until the process exits and releases its resources.
	wait func() error
}

type cursorState struct {
	cursor   cursor
	decawm   bool
	decom    bool
	charsets charsets
}

type margin struct {
	top    row
	bottom row
	left   column
	right  column
}

func (s *Session) homeCursor() {
	s.lastCol = false
	s.cursor.row = 0
	s.cursor.col = 0
	if s.mode&decom != 0 {
		s.cursor.row = s.margin.top
		s.cursor.col = s.margin.left
	}
}

func (s *Session) reportedCursor() (row, column) {
	r := s.cursor.row
	c := s.cursor.col
	if s.mode&decom != 0 {
		r -= s.margin.top
		c -= s.margin.left
	}
	if r < 0 {
		r = 0
	}
	if c < 0 {
		c = 0
	}
	return r, c
}

func New() *Session {
	tabs := []column{}
	for i := 8; i < (50 * 8); i += 8 {
		tabs = append(tabs, column(i))
	}
	return &Session{
		Logger:          log.New(io.Discard, "", log.Flags()),
		OSC8:            true,
		MaxClipboardLen: 1 << 20, // 1 MB default
		charsets: charsets{
			designations: map[charsetDesignator]charset{
				g0: ascii,
				g1: ascii,
				g2: ascii,
				g3: ascii,
			},
		},
		mode: decawm | dectcem,
		primaryState: cursorState{
			charsets: charsets{
				designations: map[charsetDesignator]charset{
					g0: ascii,
					g1: ascii,
					g2: ascii,
					g3: ascii,
				},
			},
			decawm: true,
		},
		altState: cursorState{
			charsets: charsets{
				designations: map[charsetDesignator]charset{
					g0: ascii,
					g1: ascii,
					g2: ascii,
					g3: ascii,
				},
			},
			decawm: true,
		},
		tabStop:      tabs,
		eventHandler: func(ev tcell.Event) { return },
		// Buffering to 32 events to reduce silent drops under bursty output.
		// If there is ever a case where one sequence can trigger many events,
		// this gives reasonable headroom before overflow.
		events: make(chan tcell.Event, 32),
	}
}

// NewWithSize is a convenience constructor equivalent to
//
//	s := New()
//	s.Resize(width, height)
//
// It returns a Session with the given dimensions already applied, so callers can
// start printing or feeding output without an explicit resize step.
func NewWithSize(width, height int) *Session {
	s := New()
	s.Resize(width, height)
	return s
}

// StartWithPty starts the terminal using an already-opened pty (or any
// io.ReadWriteCloser). No subprocess is spawned; the caller is responsible
// for driving the slave side of the pty. StartWithPty returns immediately.
func (s *Session) StartWithPty(p io.ReadWriteCloser) error {
	s.mu.Lock()
	w, h := s.surface.Size()
	s.pty = p
	s.cmd = nil
	s.mu.Unlock()

	s.Resize(w, h)

	s.mu.Lock()
	s.parser = NewParser(p)
	s.mu.Unlock()

	go s.runParseLoop()
	return nil
}

// Start starts the terminal with the specified command. Start returns when the
// command has been successfully started.
func (s *Session) Start(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("no command to run")
	}
	s.mu.Lock()
	w, h := s.surface.Size()
	term := s.TERM
	if term == "" {
		term = "xterm-256color"
		s.TERM = term
	}
	s.mu.Unlock()

	env := os.Environ()
	if cmd.Env != nil {
		env = cmd.Env
	}

	// Create and configure the command to run on the pty.
	args := cmd.Args
	if len(args) > 0 {
		args = args[1:]
	}
	c := exec.Command(cmd.Path, args...)
	c.Env = append(env, "TERM="+term)
	c.Dir = cmd.Dir

	// Start the command on a fresh pty and keep its master end.
	p, proc, err := startPTY(c, w, h)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.pty = p
	s.cmd = proc
	s.mu.Unlock()

	s.Resize(w, h)

	s.mu.Lock()
	s.parser = NewParser(p)
	s.mu.Unlock()

	go s.runParseLoop()
	return nil
}

// runParseLoop reads sequences from the parser, dispatches them to update(),
// and emits EventClosed when the input stream ends. It is shared by Start and
// StartWithPty.
func (s *Session) runParseLoop() {
	defer s.recover()
	for {
		// Drain all pending events before parsing the next
		// sequence to avoid oscillation (TERMINAL-F14).
		for {
			select {
			case ev := <-s.events:
				s.mu.Lock()
				fn := s.eventHandler
				rf := s.redrawHandler
				s.mu.Unlock()
				if fn != nil {
					fn(ev)
				}
				if rf != nil {
					rf()
				}
			default:
				goto parse
			}
		}
	parse:
		seq := s.parser.Next()
		switch seq := seq.(type) {
		case EOF:
			s.mu.Lock()
			fn := s.eventHandler
			s.mu.Unlock()
			if fn != nil {
				fn(&EventClosed{
					EventTerminal: newEventTerminal(s),
				})
			}
			return
		default:
			s.update(seq)
		}
	}
}

func (s *Session) update(seq Sequence) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch seq := seq.(type) {
	case Print:
		s.print(rune(seq))
	case C0:
		s.c0(rune(seq))
	case ESC:
		esc := append(seq.Intermediate, seq.Final)
		s.esc(string(esc))
		s.appendEventLog("ESC", string(esc), nil)
	case CSI:
		csi := append(seq.Intermediate, seq.Final)
		s.csi(string(csi), seq.Parameters)
		s.appendEventLog("CSI", string(csi), seq.Parameters)
	case OSC:
		s.osc(string(seq.Payload))
		s.appendEventLog("OSC", string(seq.Payload), nil)
	case DCS:
		// Forward DCS start as an event for external handlers.
		s.postEvent(&EventDCS{
			EventTerminal: newEventTerminal(s),
			Final:         rune(seq.Final),
			Parameters:    seq.Parameters,
		})
	case DCSData:
		s.postEvent(&EventDCSData{
			EventTerminal: newEventTerminal(s),
			Data:          rune(seq),
		})
	case DCSEndOfData:
	case error:
		s.Logger.Printf("parse error: %v", seq)
		return
	}
	// Post EventRedraw only once per dispatch batch.
	if !s.dirty {
		s.dirty = true
		s.postEvent(&EventRedraw{
			EventTerminal: newEventTerminal(s),
		})
	}
}

func (s *Session) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	str := strings.Builder{}
	for row := range s.activeScreen {
		for col := range s.activeScreen[row] {
			_, _ = str.WriteRune(s.activeScreen[row][col].rune())
			for _, comb := range s.activeScreen[row][col].combining {
				_, _ = str.WriteRune(comb)
			}
		}
		if row < s.height()-1 {
			str.WriteRune('\n')
		}
	}
	return str.String()
}

func (s *Session) recover() {
	err := recover()
	if err == nil {
		return
	}

	// Snapshot cursor and margin state under lock to avoid racing with
	// concurrent Resize, Draw, or HandleEvent calls.
	s.mu.Lock()
	cursorRow := s.cursor.row
	cursorCol := s.cursor.col
	marginLeft := s.margin.left
	marginRight := s.margin.right
	s.mu.Unlock()

	ret := strings.Builder{}
	ret.WriteString(fmt.Sprintf("cursor row=%d col=%d\n", cursorRow, cursorCol))
	ret.WriteString(fmt.Sprintf("margin left=%d right=%d\n", marginLeft, marginRight))
	ret.WriteString(fmt.Sprintf("%s\n", err))
	ret.Write(debug.Stack())

	s.postEvent(&EventPanic{
		EventTerminal: newEventTerminal(s),
		Error:         fmt.Errorf("%s", ret.String()),
	})
	s.Close()
}

// row, col, style, vis
func (s *Session) Cursor() (int, int, tcell.CursorStyle, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Hide the hardware cursor whenever the viewport is scrolled back into
	// history — the cursor position refers to the live screen, not the
	// scrollback slice that is currently visible.
	vis := s.mode&dectcem > 0 && s.scrollOffset == 0
	return int(s.cursor.row), int(s.cursor.col), s.cursor.style, vis
}

func (s *Session) Resize(w int, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resizeUnlocked(w, h)
}

func (s *Session) resizeUnlocked(w int, h int) {
	primary := s.primaryScreen
	s.altScreen = make([][]cell, h)
	s.primaryScreen = make([][]cell, h)
	for i := range s.altScreen {
		s.altScreen[i] = make([]cell, w)
		s.primaryScreen[i] = make([]cell, w)
	}

	// Save cursor state before resize — direct copy must not corrupt these.
	savedAttrs := s.cursor.attrs
	savedRow := s.cursor.row
	savedCol := s.cursor.col

	s.margin.top = 0
	s.margin.bottom = row(h) - 1
	s.margin.left = 0
	s.margin.right = column(w) - 1
	s.lastCol = false
	s.activeScreen = s.primaryScreen

	// Directly copy cell structs from the old primary screen to the new one.
	// This avoids the expense (and side-effects) of replaying every cell
	// through s.print(), which performs charset lookup, runewidth
	// calculation, and overwrites cursor attributes.
	if len(primary) > 0 && len(primary[0]) > 0 {
		copyRows := len(primary)
		// Only copy up to the row the cursor was on (content below the
		// cursor is uninitialised in the old screen).
		if limit := int(savedRow) + 1; copyRows > limit {
			copyRows = limit
		}
		if copyRows > h {
			copyRows = h
		}
		copyCols := len(primary[0])
		if copyCols > w {
			copyCols = w
		}
		for r := 0; r < copyRows; r++ {
			copy(s.primaryScreen[r][:copyCols], primary[r][:copyCols])
		}
	}

	// Restore cursor position clamped to the new dimensions.
	s.cursor.attrs = savedAttrs
	s.cursor.row = savedRow
	s.cursor.col = savedCol
	if h > 0 && s.cursor.row >= row(h) {
		s.cursor.row = row(h) - 1
	}
	if h == 0 {
		s.cursor.row = 0
	}
	if w > 0 && s.cursor.col >= column(w) {
		s.cursor.col = column(w) - 1
	}
	if w == 0 {
		s.cursor.col = 0
	}

	switch s.mode & smcup {
	case 0:
		s.activeScreen = s.primaryScreen
	default:
		s.activeScreen = s.altScreen
	}

	if s.pty != nil {
		switch p := s.pty.(type) {
		case interface{ Resize(int, int) error }:
			_ = p.Resize(w, h)
		case *os.File:
			// The master end of a Unix pty: resize the terminal the
			// child process sees.
			_ = pty.SetSize(p, ptyWinsize(w, h))
		}
	}
}

// ptyWinsize converts a terminal size in character cells to the pty package's
// window size.
func ptyWinsize(w, h int) *pty.Winsize {
	return &pty.Winsize{
		Cols: uint16(w),
		Rows: uint16(h),
	}
}

func (s *Session) width() int {
	if len(s.activeScreen) > 0 {
		return len(s.activeScreen[0])
	}
	return 0
}

func (s *Session) height() int {
	return len(s.activeScreen)
}

// print sets the current cell contents to the given rune. The attributes will
// be copied from the current cursor attributes
func (s *Session) print(r rune) {
	if s.charsets.designations[s.charsets.selected] == decSpecialAndLineDrawing {
		shifted, ok := decSpecial[r]
		if ok {
			r = shifted
		}
	}

	// If we are single-shifted, move the previous charset into the current
	if s.charsets.singleShift {
		s.charsets.selected = s.charsets.saved
		s.charsets.singleShift = false
	}

	if s.cursor.col == s.margin.right && s.lastCol {
		col := s.cursor.col
		rw := s.cursor.row
		s.activeScreen[rw][col].wrapped = true
		s.nel()
	}

	col := s.cursor.col
	rw := s.cursor.row
	w := runewidth.RuneWidth(r)

	if s.mode&irm != 0 {
		line := s.activeScreen[rw]
		for i := s.margin.right; i > col; i -= 1 {
			line[i] = line[i-column(w)]
		}
	}
	if col > column(s.width())-1 {
		col = column(s.width()) - 1
	}
	if rw > row(s.height()-1) {
		rw = row(s.height() - 1)
	}

	if w == 0 {
		if col-1 < 0 {
			return
		}
		s.activeScreen[rw][col-1].combining = append(s.activeScreen[rw][col-1].combining, r)
		return
	}
	cell := cell{
		content:   r,
		width:     w,
		attrs:     s.cursor.attrs,
		protected: s.cursor.protected,
		overline:  s.cursor.overline,
	}

	s.activeScreen[rw][col] = cell

	// Set trailing cells to a space if wide rune
	for i := column(1); i < column(w); i += 1 {
		if col+i > s.margin.right {
			break
		}
		s.activeScreen[rw][col+i].content = ' '
		s.activeScreen[rw][col+i].attrs = s.cursor.attrs
	}

	switch {
	case s.mode&decawm != 0 && col == s.margin.right:
		s.lastCol = true
	case col == s.margin.right:
		// don't move the cursor
	default:
		s.cursor.col += column(w)
	}
}

// scrollUp shifts all text upward by n rows. Semantically, this is backwards -
// usually scroll up would mean you shift rows down
func (s *Session) scrollUp(n int) {
	// Capture lines that are about to scroll off the top into the scrollback
	// buffer, but only when a full-width scroll region starts at the top of
	// the screen (i.e. the standard case for a shell / primary screen).
	if s.mode&smcup == 0 &&
		int(s.margin.top) == 0 &&
		int(s.margin.left) == 0 &&
		int(s.margin.right) == s.width()-1 {
		for i := 0; i < n; i++ {
			src := int(s.margin.top) + i
			if src >= s.height() {
				break
			}
			line := make([]cell, len(s.activeScreen[src]))
			copy(line, s.activeScreen[src])
			s.scrollback = append(s.scrollback, line)
		}
		limit := s.scrollbackLimit
		if limit <= 0 {
			limit = 10_000
		}
		if len(s.scrollback) > limit {
			s.scrollback = s.scrollback[len(s.scrollback)-limit:]
		}
	}

	for row := range s.activeScreen {
		if row > int(s.margin.bottom) {
			continue
		}
		if row < int(s.margin.top) {
			continue
		}
		if row+n > int(s.margin.bottom) {
			for col := s.margin.left; col <= s.margin.right; col += 1 {
				s.activeScreen[row][col].erase(s.cursor.attrs)
			}
			continue
		}
		for col := s.margin.left; col <= s.margin.right; col += 1 {
			s.activeScreen[row][col] = s.activeScreen[row+n][col]
		}
	}
}

// scrollDown shifts all lines down by n rows.
func (s *Session) scrollDown(n int) {
	for r := s.margin.bottom; r >= s.margin.top; r -= 1 {
		if r-row(n) < s.margin.top {
			for col := s.margin.left; col <= s.margin.right; col += 1 {
				s.activeScreen[r][col].erase(s.cursor.attrs)
			}
			continue
		}
		for col := s.margin.left; col <= s.margin.right; col += 1 {
			s.activeScreen[r][col] = s.activeScreen[r-row(n)][col]
		}
	}
}

func (s *Session) Close() {
	s.mu.Lock()

	// Kill the process under lock so no concurrent reader sees a
	// half-dead process.
	cmd := s.cmd
	if cmd != nil && cmd.process != nil {
		cmd.process.Kill()
	}

	// Release the lock before Wait() which may block, so that Draw,
	// Resize, and HandleEvent are not stalled.
	s.mu.Unlock()

	if cmd != nil && cmd.process != nil {
		_ = cmd.wait()
	}

	// Re-lock to close the pty and clear state.
	s.mu.Lock()
	s.cmd = nil
	if s.pty != nil {
		s.pty.Close()
		s.pty = nil
	}
	s.mu.Unlock()
}

func (s *Session) Attach(fn func(ev tcell.Event)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandler = fn
}

func (s *Session) Detach() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandler = func(ev tcell.Event) {
		return
	}
}

// SetRedrawHandler registers a callback that is called when the terminal
// content has changed and should be re-rendered. The callback is invoked
// from the terminal's internal parse goroutine for each EventRedraw and
// should not block — hand off to a channel and process in your main loop.
// This is a lighter alternative to Attach when you only need to know when
// to call Draw().
func (s *Session) SetRedrawHandler(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redrawHandler = fn
}

func (s *Session) postEvent(ev tcell.Event) {
	select {
	case s.events <- ev:
	default:
		// Drop the event if the channel is full to prevent deadlock,
		// particularly in the panic recovery path (TERMINAL-F13).
	}
}

func (s *Session) SetSurface(srf Screen) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.surface = srf
}

// HasSurface reports whether a Screen has been attached to the Session. It is
// safe to call from any goroutine.
func (s *Session) HasSurface() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.surface != nil
}

// SetOSC8 sets whether OSC8 hyperlink sequences are processed.
// It is safe to call from any goroutine.
func (s *Session) SetOSC8(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OSC8 = enabled
}

// SetTERM sets the TERM environment variable that will be passed to the
// command's environment. It is safe to call from any goroutine but must
// be called before Start().
func (s *Session) SetTERM(term string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TERM = term
}

// SetLogger sets the logger used for parse errors and diagnostics.
// It is safe to call from any goroutine.
func (s *Session) SetLogger(logger *log.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Logger = logger
}

// SetMaxClipboardLen configures the maximum accepted length in bytes of an
// OSC 52 clipboard payload. Values <= 0 disable the limit and forward all
// clipboard data as-is. It is safe to call from any goroutine.
func (s *Session) SetMaxClipboardLen(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MaxClipboardLen = n
}

func (s *Session) Draw() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mode&syncOutput != 0 {
		// Synchronized output is active — defer screen updates until the
		// mode is reset. Leave dirty flag set so a future Draw() paints.
		return
	}
	s.dirty = false
	if s.surface == nil {
		return
	}

	h := s.height()
	w := s.width()
	sbLen := len(s.scrollback)
	offset := s.scrollOffset

	// The virtual buffer is: scrollback[0..sbLen-1] followed by the h rows
	// of the active screen. The viewport always shows exactly h rows.
	//
	//   viewportStart  →  first virtual-buffer row rendered as display row 0
	virtualTotal := sbLen + h
	viewportStart := virtualTotal - h - offset
	if viewportStart < 0 {
		viewportStart = 0
	}

	for row := 0; row < h; row++ {
		virtualRow := viewportStart + row

		var line []cell
		if virtualRow < sbLen {
			line = s.scrollback[virtualRow]
		} else {
			activeRow := virtualRow - sbLen
			if activeRow >= 0 && activeRow < h {
				line = s.activeScreen[activeRow]
			}
		}

		for col := 0; col < w; {
			var c cell
			if line != nil && col < len(line) {
				c = line[col]
			}
			cw := c.width
			style := c.attrs
			// DECSCNM: when reverse video mode is active, invert all cells.
			// Only apply to the live screen, not replayed history rows.
			if s.mode&decscnm != 0 && virtualRow >= sbLen {
				style = style.Reverse(true)
			}
			if sw, ok := s.surface.(interface {
				SetContentWidth(x, y int, ch rune, comb []rune, style tcell.Style, width int, overline, protected, wrapped bool)
			}); ok {
				sw.SetContentWidth(col, row, c.content, c.combining, style, cw, c.overline, c.protected, c.wrapped)
			} else {
				s.surface.SetContent(col, row, c.content, c.combining, style)
			}
			if cw == 0 {
				cw = 1
			}
			col += cw
		}
	}
	// Show or hide the hardware cursor if the surface supports it.
	if cs, ok := s.surface.(interface{ ShowCursor(int, int) }); ok {
		if s.mode&dectcem > 0 && s.scrollOffset == 0 {
			cs.ShowCursor(int(s.cursor.col), int(s.cursor.row))
		} else {
			cs.ShowCursor(-1, -1) // hide
		}
	}
}

// Write sends raw bytes to the PTY input. It is used for programmatic input
// injection (recording playback, tests, etc.). Returns an error if the Session has
// not been started yet.
func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	w := s.pty
	s.mu.Unlock()
	if w == nil {
		return 0, fmt.Errorf("terminal: Write: terminal not started")
	}
	return w.Write(p)
}

func (s *Session) HandleEvent(e tcell.Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return false
	}
	switch e := e.(type) {
	case *tcell.EventKey:
		// Any keystroke snaps the viewport back to the live screen.
		s.scrollOffset = 0
		io.WriteString(s.pty, keyCode(e, s.mode&decckm != 0))
		return true
	case *tcell.EventPaste:
		switch {
		case s.mode&paste == 0:
			return false
		case e.Start():
			io.WriteString(s.pty, info.PasteStart)
			return true
		case e.End():
			io.WriteString(s.pty, info.PasteEnd)
			return true
		}
	case *tcell.EventMouse:
		str := s.handleMouse(e)
		io.WriteString(s.pty, str)
	case *tcell.EventFocus:
		if s.mode&focusEvents != 0 {
			if e.Focused {
				io.WriteString(s.pty, "\x1b[I")
			} else {
				io.WriteString(s.pty, "\x1b[O")
			}
			return true
		}
	}
	return false
}

// ScrollBy shifts the viewport by n lines relative to its current position.
// Positive n scrolls toward older history (up); negative n scrolls toward
// the live screen (down). The offset is clamped to [0, len(scrollback)].
// Safe to call from any goroutine.
func (s *Session) ScrollBy(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scrollOffset += n
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset > len(s.scrollback) {
		s.scrollOffset = len(s.scrollback)
	}
}

// ScrollOffset returns the number of lines the viewport is currently
// scrolled back from the live screen (0 means live). Safe to call from
// any goroutine.
func (s *Session) ScrollOffset() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scrollOffset
}

// IsAltScreen reports whether the alternate screen buffer is currently
// active (e.g. while running vim, less, htop). Safe to call from any
// goroutine.
func (s *Session) IsAltScreen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode&smcup != 0
}

// GetMargins returns the current scroll region margins as
// (top, bottom, left, right). Safe to call from any goroutine.
func (s *Session) GetMargins() (top, bottom, left, right int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(s.margin.top), int(s.margin.bottom), int(s.margin.left), int(s.margin.right)
}

// GetTabStops returns the current set of tab stop column positions.
// The returned slice is a copy; modifying it does not affect the terminal.
// Safe to call from any goroutine.
func (s *Session) GetTabStops() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int, len(s.tabStop))
	for i, c := range s.tabStop {
		out[i] = int(c)
	}
	return out
}

// GetCursorStyle returns the current cursor shape (block, underline, bar).
// Safe to call from any goroutine.
func (s *Session) GetCursorStyle() tcell.CursorStyle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor.style
}

// GetActiveCharset returns the currently selected charset designator.
// The returned string is one of "G0", "G1", "G2", or "G3".
// Safe to call from any goroutine.
func (s *Session) GetActiveCharset() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.charsets.selected {
	case g0:
		return "G0"
	case g1:
		return "G1"
	case g2:
		return "G2"
	case g3:
		return "G3"
	default:
		return "G0"
	}
}

// GetDimensions returns the current terminal dimensions as (width, height).
// Safe to call from any goroutine.
func (s *Session) GetDimensions() (width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.width(), s.height()
}

const eventLogMax = 1000

// EventLog returns a copy of the most recent sequence/mode event entries,
// up to limit. If limit <= 0, all entries are returned. Safe to call from
// any goroutine.
func (s *Session) EventLog(limit int) []EventLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.eventLog)
	if limit > 0 && limit < n {
		n = limit
	}
	out := make([]EventLogEntry, n)
	copy(out, s.eventLog[len(s.eventLog)-n:])
	return out
}

// ClearEventLog empties the event log. Safe to call from any goroutine.
func (s *Session) ClearEventLog() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventLog = nil
}

// appendEventLog adds an entry to the bounded event log.
func (s *Session) appendEventLog(typ, seq string, args interface{}) {
	s.eventLog = append(s.eventLog, EventLogEntry{
		Time: time.Now(), Type: typ, Sequence: seq, Args: args,
	})
	if len(s.eventLog) > eventLogMax {
		s.eventLog = s.eventLog[len(s.eventLog)-eventLogMax:]
	}
}

// GetClipboard returns the decoded clipboard content for the given
// selection (typically "c" for system or "p" for primary), or
// ("", false) if no data has been received.
func (s *Session) GetClipboard(selection string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clipboard == nil {
		return "", false
	}
	data, ok := s.clipboard[selection]
	return data, ok
}

// SetClipboard stores clipboard data directly without going through OSC 52.
// Useful for testing or programmatic injection.
func (s *Session) SetClipboard(selection, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clipboard == nil {
		s.clipboard = make(map[string]string)
	}
	s.clipboard[selection] = data
}

// SendSignal sends an OS signal to the underlying process, if one is running.
// Returns an error if no process is attached or the signal cannot be delivered.
func (s *Session) SendSignal(sig os.Signal) error {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.process == nil {
		return fmt.Errorf("terminal: SendSignal: no process running")
	}
	return cmd.process.Signal(sig)
}

type charset int

const (
	ascii charset = iota
	decSpecialAndLineDrawing
)

type charsets struct {
	selected     charsetDesignator
	saved        charsetDesignator
	designations map[charsetDesignator]charset
	singleShift  bool
}

type charsetDesignator int

const (
	g0 = iota
	g1
	g2
	g3
)

var decSpecial = map[rune]rune{
	0x5f: 0x00A0, // NO-BREAK SPACE
	0x60: 0x25C6, // BLACK DIAMOND
	0x61: 0x2592, // MEDIUM SHADE
	0x62: 0x2409, // SYMBOL FOR HORIZONTAL TABULATION
	0x63: 0x240C, // SYMBOL FOR FORM FEED
	0x64: 0x240D, // SYMBOL FOR CARRIAGE RETURN
	0x65: 0x240A, // SYMBOL FOR LINE FEED
	0x66: 0x00B0, // DEGREE SIGN
	0x67: 0x00B1, // PLUS-MINUS SIGN
	0x68: 0x2424, // SYMBOL FOR NEWLINE
	0x69: 0x240B, // SYMBOL FOR VERTICAL TABULATION
	0x6a: 0x2518, // BOX DRAWINGS LIGHT UP AND LEFT
	0x6b: 0x2510, // BOX DRAWINGS LIGHT DOWN AND LEFT
	0x6c: 0x250C, // BOX DRAWINGS LIGHT DOWN AND RIGHT
	0x6d: 0x2514, // BOX DRAWINGS LIGHT UP AND RIGHT
	0x6e: 0x253C, // BOX DRAWINGS LIGHT VERTICAL AND HORIZONTAL
	0x6f: 0x23BA, // HORIZONTAL SCAN LINE-1
	0x70: 0x23BB, // HORIZONTAL SCAN LINE-3
	0x71: 0x2500, // BOX DRAWINGS LIGHT HORIZONTAL
	0x72: 0x23BC, // HORIZONTAL SCAN LINE-7
	0x73: 0x23BD, // HORIZONTAL SCAN LINE-9
	0x74: 0x251C, // BOX DRAWINGS LIGHT VERTICAL AND RIGHT
	0x75: 0x2524, // BOX DRAWINGS LIGHT VERTICAL AND LEFT
	0x76: 0x2534, // BOX DRAWINGS LIGHT UP AND HORIZONTAL
	0x77: 0x252C, // BOX DRAWINGS LIGHT DOWN AND HORIZONTAL
	0x78: 0x2502, // BOX DRAWINGS LIGHT VERTICAL
	0x79: 0x2264, // LESS-THAN OR EQUAL TO
	0x7a: 0x2265, // GREATER-THAN OR EQUAL TO
	0x7b: 0x03C0, // GREEK SMALL LETTER PI
	0x7c: 0x2260, // NOT EQUAL TO
	0x7d: 0x00A3, // POUND SIGN
	0x7e: 0x00B7, // MIDDLE DOT
}

type cursor struct {
	attrs     tcell.Style
	style     tcell.CursorStyle
	protected bool // DECSCA protection attribute
	overline  bool // SGR 53 overline attribute

	// position
	row row    // 0-indexed
	col column // 0-indexed
}

// Screen represents a logical view on an area. It uses a subset of methods
// from a tcell.Screen or a views.View, in order to be a more broad
// implementation. Both a Screen and a View are also a Screen
type Screen interface {
	// SetContent is used to update the content of the Screen at the given
	// location.
	SetContent(x int, y int, ch rune, comb []rune, style tcell.Style)

	// Size represents the visible size.
	Size() (int, int)
}

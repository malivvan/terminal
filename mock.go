package terminal

// This file intentionally has no build constraint so it is part of
// the public API of the terminal package: external test code in any
// package can construct a MockTerminal.

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
)

// testingT is the subset of *testing.T used by the mock helpers. It
// allows the helpers to live in non-_test.go files (so external
// packages can import them) without taking a hard dependency on the
// concrete *testing.T type.
type testingT interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// MockTerminal wraps Terminal with assertion helpers commonly needed
// when writing unit tests for code that emits ANSI/ECMA-48 sequences
// or for widgets that integrate with the terminal package.
//
// Because Terminal.Feed is synchronous, no Sync/Wait barrier is
// needed between Feed and assertions.
type MockTerminal struct {
	*Terminal
	t testingT
}

// NewMockTerminal creates a MockTerminal wrapping a fresh Terminal of
// the given dimensions.
func NewMockTerminal(t testingT, width, height int) *MockTerminal {
	t.Helper()
	h, err := NewTerminal(width, height)
	if err != nil {
		t.Fatalf("terminal: NewTerminal(%d, %d) failed: %v", width, height, err)
		return nil
	}
	return &MockTerminal{Terminal: h, t: t}
}

// Feed writes the given string to the Session. Returns the number of
// bytes fed. Any error fails the test via Fatalf.
func (m *MockTerminal) Feed(s string) int {
	m.t.Helper()
	n, err := m.Terminal.FeedString(s)
	if err != nil {
		m.t.Fatalf("terminal: Feed(%q) failed: %v", s, err)
		return 0
	}
	return n
}

// SendKey forwards a synthesized key press to the Session. The emitted
// byte sequence lands in the output buffer drainable via Output.
// The str argument carries the rune(s) for printable keys; pass ""
// for control keys identified by k.
func (m *MockTerminal) SendKey(key tcell.Key, str string, mod tcell.ModMask) {
	m.t.Helper()
	m.Terminal.HandleEvent(tcell.NewEventKey(key, str, mod))
}

// SendRune is a convenience wrapper that synthesizes a printable rune
// keypress.
func (m *MockTerminal) SendRune(r rune) {
	m.t.Helper()
	m.Terminal.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
}

// AssertCell checks that the rune at (x, y) equals want.
func (m *MockTerminal) AssertCell(x, y int, want rune) {
	m.t.Helper()
	got := m.Terminal.Cell(x, y)
	gr := got.Rune
	if gr == 0 {
		gr = ' '
	}
	if gr != want {
		m.t.Errorf("cell(%d, %d) = %q, want %q\n%s", x, y, gr, want, m.Dump())
	}
}

// AssertLine checks that row y matches want exactly.
func (m *MockTerminal) AssertLine(y int, want string) {
	m.t.Helper()
	lines := m.Terminal.Lines()
	if y < 0 || y >= len(lines) {
		m.t.Errorf("line %d out of range [0, %d)", y, len(lines))
		return
	}
	if lines[y] != want {
		m.t.Errorf("line %d = %q, want %q", y, lines[y], want)
	}
}

// AssertContains checks that any visible row contains substr.
func (m *MockTerminal) AssertContains(substr string) {
	m.t.Helper()
	for _, l := range m.Terminal.Lines() {
		if strings.Contains(l, substr) {
			return
		}
	}
	m.t.Errorf("no line contains %q\nscreen:\n%s", substr, m.Terminal.String())
}

// AssertCursor checks that the cursor is at (col, row) and visible.
func (m *MockTerminal) AssertCursor(col, row int) {
	m.t.Helper()
	c, r, vis := m.Terminal.Cursor()
	if !vis {
		m.t.Errorf("cursor invisible, want visible at (%d, %d)", col, row)
		return
	}
	if c != col || r != row {
		m.t.Errorf("cursor at (%d, %d), want (%d, %d)", c, r, col, row)
	}
}

// AssertOutput drains the Session-to-host output buffer and checks that
// it equals want. Useful for verifying replies to DA/cursor-position
// queries or key bytes emitted by SendKey.
func (m *MockTerminal) AssertOutput(want string) {
	m.t.Helper()
	got := string(m.Terminal.Output())
	if got != want {
		m.t.Errorf("output = %q, want %q", got, want)
	}
}

// Snapshot returns the screen as a newline-separated string, useful
// for golden-file comparisons.
func (m *MockTerminal) Snapshot() string { return m.Terminal.String() }

// Dump returns a human-friendly multi-line string describing the
// current screen and cursor position. Intended for failure messages.
func (m *MockTerminal) Dump() string {
	c, r, vis := m.Terminal.Cursor()
	w, h := m.Terminal.Size()
	var b strings.Builder
	fmt.Fprintf(&b, "size=%dx%d cursor=(%d,%d) visible=%v\n", w, h, c, r, vis)
	for i, line := range m.Terminal.Lines() {
		fmt.Fprintf(&b, "%2d|%s|\n", i, line)
	}
	return b.String()
}

// AssertStyle checks that the tcell.Style at (x, y) equals want.
func (m *MockTerminal) AssertStyle(x, y int, want tcell.Style) {
	m.t.Helper()
	got := m.Terminal.Cell(x, y).Style
	if got != want {
		m.t.Errorf("cell(%d, %d) style = %v, want %v\n%s", x, y, got, want, m.Dump())
	}
}

// AssertForeground checks that the foreground colour at (x, y) equals want.
func (m *MockTerminal) AssertForeground(x, y int, want tcellcolor.Color) {
	m.t.Helper()
	got := m.Terminal.Cell(x, y).Style.GetForeground()
	if got != want {
		m.t.Errorf("cell(%d, %d) foreground = %v, want %v\n%s", x, y, got, want, m.Dump())
	}
}

// AssertBackground checks that the background colour at (x, y) equals want.
func (m *MockTerminal) AssertBackground(x, y int, want tcellcolor.Color) {
	m.t.Helper()
	got := m.Terminal.Cell(x, y).Style.GetBackground()
	if got != want {
		m.t.Errorf("cell(%d, %d) background = %v, want %v\n%s", x, y, got, want, m.Dump())
	}
}

// AssertBold checks that the cell at (x, y) has the bold attribute.
func (m *MockTerminal) AssertBold(x, y int) {
	m.t.Helper()
	if !m.Terminal.Cell(x, y).Style.HasBold() {
		m.t.Errorf("cell(%d, %d) not bold\n%s", x, y, m.Dump())
	}
}

// AssertItalic checks that the cell at (x, y) has the italic attribute.
func (m *MockTerminal) AssertItalic(x, y int) {
	m.t.Helper()
	if !m.Terminal.Cell(x, y).Style.HasItalic() {
		m.t.Errorf("cell(%d, %d) not italic\n%s", x, y, m.Dump())
	}
}

// AssertUnderline checks that the cell at (x, y) has the underline attribute.
func (m *MockTerminal) AssertUnderline(x, y int) {
	m.t.Helper()
	if !m.Terminal.Cell(x, y).Style.HasUnderline() {
		m.t.Errorf("cell(%d, %d) not underlined\n%s", x, y, m.Dump())
	}
}

// AssertCombining checks that the combining marks at (x, y) match want.
func (m *MockTerminal) AssertCombining(x, y int, want ...rune) {
	m.t.Helper()
	got := m.Terminal.Cell(x, y).Combining
	if len(got) != len(want) {
		m.t.Errorf("cell(%d, %d) combining = %v, want %v\n%s", x, y, got, want, m.Dump())
		return
	}
	for i := range got {
		if got[i] != want[i] {
			m.t.Errorf("cell(%d, %d) combining = %v, want %v\n%s", x, y, got, want, m.Dump())
			return
		}
	}
}

// AssertScrollbackLine checks that scrollback row n matches want.
func (m *MockTerminal) AssertScrollbackLine(n int, want string) {
	m.t.Helper()
	line, ok := m.Terminal.ScrollbackLine(n)
	if !ok {
		m.t.Errorf("scrollback line %d out of range", n)
		return
	}
	if line != want {
		m.t.Errorf("scrollback line %d = %q, want %q", n, line, want)
	}
}

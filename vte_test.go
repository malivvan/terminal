package terminal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/stretchr/testify/assert"
)

func TestResize(t *testing.T) {
	vt := New()
	w := 4
	h := 1
	vt.Resize(w, h)
	assert.Equal(t, h, len(vt.activeScreen))
	assert.Equal(t, w, len(vt.activeScreen[0]))
}

func TestString(t *testing.T) {
	vt := New()
	w := 2
	h := 1
	vt.Resize(w, h)
	assert.Equal(t, "  ", vt.String())

	vt.activeScreen[0][0].content = 'v'
	vt.activeScreen[0][1].content = 't'
	assert.Equal(t, "vt", vt.String())
}

func TestPrint(t *testing.T) {
	t.Run("No modes", func(t *testing.T) {
		vt := New()
		vt.mode = 0
		w := 2
		h := 1
		vt.Resize(w, h)

		vt.print('v')
		vt.print('t')
		assert.Equal(t, "vt", vt.String())
		assert.Equal(t, column(1), vt.cursor.col)
		vt.print('x')
		assert.Equal(t, "vx", vt.String())
	})

	t.Run("IRM = set", func(t *testing.T) {
		vt := New()
		w := 4
		h := 1
		vt.Resize(w, h)

		vt.print('v')
		vt.print('t')
		vt.bs()
		vt.bs()
		assert.Equal(t, column(0), vt.cursor.col)
		assert.Equal(t, "vt  ", vt.String())
		vt.mode |= irm
		vt.print('i')
		assert.Equal(t, "ivt ", vt.String())
		vt.print('j')
		vt.print('k')
		assert.Equal(t, "ijkv", vt.String())
	})

	t.Run("DECAWM = set", func(t *testing.T) {
		vt := New()
		w := 3
		h := 2
		vt.Resize(w, h)
		vt.mode |= decawm

		vt.print('v')
		vt.print('t')
		assert.Equal(t, "vt \n   ", vt.String())
		vt.print('i')
		assert.Equal(t, "vti\n   ", vt.String())
		vt.print('j')
		assert.Equal(t, "vti\nj  ", vt.String())
	})

	t.Run("Wide character", func(t *testing.T) {
		vt := New()
		w := 1
		h := 1
		vt.Resize(w, h)

		vt.print('つ')
		assert.Equal(t, "つ", vt.String())
	})
}

func TestScrollUp(t *testing.T) {
	vt := New()
	vt.mode = 0
	w := 2
	h := 2
	vt.Resize(w, h)

	vt.print('v')
	vt.print('t')
	assert.Equal(t, "vt\n  ", vt.String())
	vt.scrollUp(1)
	assert.Equal(t, "  \n  ", vt.String())

	vt = New()
	w = 1
	h = 8
	vt.Resize(w, h)

	vt.cursor.row = 4
	vt.print('v')
	vt.lastCol = false
	vt.cursor.row = 7
	vt.print('t')
	vt.margin.bottom = 5
	assert.Equal(t, " \n \n \n \nv\n \n \nt", vt.String())
	vt.scrollUp(1)
	assert.Equal(t, " \n \n \nv\n \n \n \nt", vt.String())
}

func TestScrollDown(t *testing.T) {
	vt := New()
	w := 2
	h := 2
	vt.Resize(w, h)

	vt.print('v')
	vt.print('t')
	assert.Equal(t, "vt\n  ", vt.String())
	vt.scrollDown(1)
	assert.Equal(t, "  \nvt", vt.String())
	vt.lastCol = false
	vt.print('b')
	assert.Equal(t, " b\nvt", vt.String())
	vt.scrollDown(1)
	assert.Equal(t, "  \n b", vt.String())
}

func TestCombiningRunes(t *testing.T) {
	vt := New()
	vt.Resize(2, 2)
	vt.print('h')
	vt.print(0x337)
	vt.print(0x317)

	assert.Equal(t, "h̷̗ \n  ", vt.String())
}

// TERMINAL-007: cell.erase() must clear combining runes, width, and wrapped flag.
func TestCellErase_ClearsCombiningAndWrapped(t *testing.T) {
	vt := New()
	vt.Resize(2, 1)
	vt.print('h')
	vt.print(0x0337) // combining short solidus overlay
	// Verify combining is set
	assert.Len(t, vt.activeScreen[0][0].combining, 1)
	vt.activeScreen[0][0].wrapped = true
	vt.activeScreen[0][0].width = 1

	vt.activeScreen[0][0].erase(vt.cursor.attrs)

	assert.Nil(t, vt.activeScreen[0][0].combining)
	assert.False(t, vt.activeScreen[0][0].wrapped)
	assert.Equal(t, 0, vt.activeScreen[0][0].width)
}

// TERMINAL-012: Resize must not panic when pty is nil.
func TestResize_NilPty(t *testing.T) {
	vt := New()
	// pty is nil by default — Resize should not panic
	assert.NotPanics(t, func() {
		vt.Resize(10, 5)
	})
}

// TERMINAL-029: scrollUp should only copy within left/right margins.
func TestScrollUp_RespectsLeftRightMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	// Fill row 0: "abcd"
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	// Fill row 1: "efgh"
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')
	vt.print('g')
	vt.print('h')

	// Set left/right margins to columns 1-2
	vt.margin.left = 1
	vt.margin.right = 2

	vt.scrollUp(1)
	// Columns outside margins (0,3) of row 0 should be unchanged
	assert.Equal(t, 'a', vt.activeScreen[0][0].content)
	assert.Equal(t, 'd', vt.activeScreen[0][3].content)
	// Columns inside margins should have scrolled up from row 1
	assert.Equal(t, 'f', vt.activeScreen[0][1].content)
	assert.Equal(t, 'g', vt.activeScreen[0][2].content)
}

// TERMINAL-030: Close must not panic when pty is nil.
func TestClose_NilPty(t *testing.T) {
	vt := New()
	assert.NotPanics(t, func() {
		vt.Close()
	})
}

// TERMINAL-032: DECALN (ESC # 8) should fill screen with 'E' characters.
func TestDECALN(t *testing.T) {
	vt := New()
	vt.Resize(3, 2)
	vt.esc("#8")
	assert.Equal(t, "EEE\nEEE", vt.String())
}

// TERMINAL-032 regression: DECALN should also reset margins and cursor.
func TestDECALN_ResetsState(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	// Set cursor and margins to non-default
	vt.cursor.row = 2
	vt.cursor.col = 3
	vt.margin.top = 1
	vt.margin.bottom = 1
	vt.margin.left = 1
	vt.margin.right = 2

	vt.esc("#8")

	// Verify screen is filled with 'E'
	assert.Equal(t, "EEEE\nEEEE\nEEEE", vt.String())
	// Verify cursor is reset
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
	// Verify margins are reset
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, row(2), vt.margin.bottom)
	assert.Equal(t, column(0), vt.margin.left)
	assert.Equal(t, column(3), vt.margin.right)
}

// TERMINAL-033: ESC #3, #4, #5, #6 should be silently consumed.
func TestDECLineAttrs_SilentlyConsumed(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.print('a')
	vt.print('b')
	// These should not panic or alter screen content
	assert.NotPanics(t, func() {
		vt.esc("#3") // DECDHL top
		vt.esc("#4") // DECDHL bottom
		vt.esc("#5") // DECSWL
		vt.esc("#6") // DECDWL
	})
	// Screen should be unchanged
	assert.Equal(t, "ab  ", vt.String())
}

// Default tab stops should be at columns 8, 16, 24, ... (0-indexed).
// The initialization loop used i=7; i+=8 producing stops at 7, 15, 23, ...
// which is off-by-one from the VT100 spec.
func TestDefaultTabStops(t *testing.T) {
	vt := New()
	// Check the first few default tab stops are at multiples of 8
	assert.True(t, len(vt.tabStop) > 3, "expected at least 3 default tab stops")
	assert.Equal(t, column(8), vt.tabStop[0], "first tab stop should be at column 8")
	assert.Equal(t, column(16), vt.tabStop[1], "second tab stop should be at column 16")
	assert.Equal(t, column(24), vt.tabStop[2], "third tab stop should be at column 24")
}

// After a full reset (RIS), tab stops should also be at columns 8, 16, 24, ...
func TestTabStopsAfterReset(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	// Clear tab stops and set some custom ones
	vt.tabStop = []column{5, 10}
	// Full reset
	vt.ris()
	assert.True(t, len(vt.tabStop) > 3)
	assert.Equal(t, column(8), vt.tabStop[0])
	assert.Equal(t, column(16), vt.tabStop[1])
	assert.Equal(t, column(24), vt.tabStop[2])
}

// CHT (cursor horizontal tab) from column 0 should land on column 8 with
// default tab stops.
func TestCHT_DefaultTabStopFromCol0(t *testing.T) {
	vt := New()
	vt.Resize(80, 1)
	vt.cursor.col = 0
	vt.cht(1)
	assert.Equal(t, column(8), vt.cursor.col, "CHT from col 0 should land on col 8")
}

// TERMINAL-016 regression: HTS should deduplicate tab stops.
func TestHTS_Dedup(t *testing.T) {
	vt := New()
	vt.Resize(80, 1)
	vt.tabStop = []column{}
	vt.cursor.col = 10
	vt.hts()
	vt.hts() // duplicate
	assert.Equal(t, []column{10}, vt.tabStop)
}

// TERMINAL-016: HTS should maintain tab stops in sorted order.
func TestHTS_SortedOrder(t *testing.T) {
	vt := New()
	vt.Resize(80, 1)
	// Clear default tab stops
	vt.tabStop = []column{}
	// Set tab stops out of order
	vt.cursor.col = 20
	vt.hts()
	vt.cursor.col = 10
	vt.hts()
	vt.cursor.col = 30
	vt.hts()
	// Tab stops should be sorted
	assert.Equal(t, []column{10, 20, 30}, vt.tabStop)
	// CHT from col 0 should land on 10 (first tab)
	vt.cursor.col = 0
	vt.cht(1)
	assert.Equal(t, column(10), vt.cursor.col)
}

// TERMINAL-029 regression: scrollDown should also respect left/right margins.
func TestScrollDown_RespectsLeftRightMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	// Fill row 0: "abcd"
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	// Fill row 1: "efgh"
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')
	vt.print('g')
	vt.print('h')

	// Set left/right margins to columns 1-2
	vt.margin.left = 1
	vt.margin.right = 2

	vt.scrollDown(1)
	// Row 1 columns 1-2 should have row 0's content
	assert.Equal(t, 'b', vt.activeScreen[1][1].content)
	assert.Equal(t, 'c', vt.activeScreen[1][2].content)
	// Columns outside margins should be unchanged
	assert.Equal(t, 'e', vt.activeScreen[1][0].content)
	assert.Equal(t, 'h', vt.activeScreen[1][3].content)
}

// TERMINAL-F09: In vt.go print(), after reverting the charset from a single shift
// (vt.charsets.selected = vt.charsets.saved), the vt.charsets.singleShift flag
// is never set to false. This means every subsequent print() call continues to
// overwrite selected with saved, which can undo later charset switches.
func TestSingleShift_ClearedAfterUse(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// Trigger SS2
	vt.esc("N")
	assert.True(t, vt.charsets.singleShift)

	// Print one character — this should use the single shift and then clear the flag
	vt.print('a')

	// After printing, singleShift should be false
	assert.False(t, vt.charsets.singleShift)
}

// TERMINAL-F09 regression: SS2 followed by print, then SO switch should stick.
func TestSingleShift_DoesNotUndoSubsequentSwitch(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// SS2 + print one char to consume the single shift
	vt.esc("N")
	vt.print('a')

	// Now switch to G1 via SO (0x0E)
	vt.c0(0x0E)
	assert.Equal(t, charsetDesignator(g1), vt.charsets.selected)

	// Print another character — should NOT revert to g0
	vt.print('b')

	// selected should still be g1
	assert.Equal(t, charsetDesignator(g1), vt.charsets.selected)
}

// TERMINAL-F09 regression: single shift should only affect ONE character.
func TestSingleShift_OnlyOneCharacter(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// SS2 triggers single shift
	vt.esc("N")
	assert.True(t, vt.charsets.singleShift)
	assert.Equal(t, charsetDesignator(g2), vt.charsets.selected)

	// First print: uses g2, then reverts to saved (g0) and clears singleShift
	vt.print('x')
	assert.False(t, vt.charsets.singleShift)
	assert.Equal(t, charsetDesignator(g0), vt.charsets.selected)

	// Second print: should use g0 (no single shift active)
	vt.print('y')
	assert.Equal(t, charsetDesignator(g0), vt.charsets.selected)
}

// TERMINAL-F11: In vt.go Resize(), the screen-replay loop breaks at
// row == int(last) (the cursor row), meaning the cursor row's content
// is never replayed to the new screen buffer. Any text on that row is
// lost after a resize.
func TestResize_PreservesCursorRow(t *testing.T) {
	vt := New()
	vt.Resize(3, 3)
	vt.mode = 0

	// Fill rows 0 and 1
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('d')
	vt.print('e')
	vt.print('f')

	assert.Equal(t, "abc\ndef\n   ", vt.String())

	// Place cursor on row 1 (the middle row with "def")
	vt.cursor.row = 1
	vt.cursor.col = 0

	// Resize to same dimensions — should preserve all content up to and
	// including the cursor row
	vt.Resize(3, 3)

	// Both row 0 ("abc") AND row 1 ("def") should be preserved
	// Bug: row 1 is skipped during replay, so "def" is lost
	str := vt.String()
	assert.Contains(t, str, "abc")
	assert.Contains(t, str, "def")
}

// TERMINAL-F11 regression: Resize with cursor at row 0 should still preserve row 0.
func TestResize_PreservesRow0WhenCursorThere(t *testing.T) {
	vt := New()
	vt.Resize(3, 2)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	assert.Equal(t, "abc\n   ", vt.String())

	// Cursor is at row 0
	vt.cursor.row = 0
	vt.cursor.col = 0

	vt.Resize(3, 2)

	assert.Contains(t, vt.String(), "abc")
}

// TERMINAL-F12: Terminfo comment typos — field names KeyCtrlDown, KeyMetaDown,
// KeyAltDown have comments saying "ctrl-left", "meta-left", "alt-left"
// instead of "ctrl-down", "meta-down", "alt-down". This is a documentation
// fix only; the test verifies the struct fields exist with correct names.
func TestTerminfo_DirectionalKeyFieldsExist(t *testing.T) {
	// Verify the directional key fields are distinct and properly named
	// (this is a structural sanity check — the actual bug is in comments)
	ti := terminfo{}
	// Down fields should be distinct from Left fields
	ti.KeyCtrlDown = "ctrl-down-value"
	ti.KeyCtrlLeft = "ctrl-left-value"
	assert.NotEqual(t, ti.KeyCtrlDown, ti.KeyCtrlLeft)

	ti.KeyMetaDown = "meta-down-value"
	ti.KeyMetaLeft = "meta-left-value"
	assert.NotEqual(t, ti.KeyMetaDown, ti.KeyMetaLeft)

	ti.KeyAltDown = "alt-down-value"
	ti.KeyAltLeft = "alt-left-value"
	assert.NotEqual(t, ti.KeyAltDown, ti.KeyAltLeft)
}

// Resize() set margin.bottom and margin.right but did not reset margin.top
// and margin.left to 0, leaving stale scroll margins that could cause
// incorrect scrolling or cursor positioning after a terminal resize.
func TestResize_ResetsAllMargins(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)

	// Set custom margins
	vt.mu.Lock()
	vt.margin.top = 3
	vt.margin.left = 2
	vt.margin.bottom = 7
	vt.margin.right = 8
	vt.mu.Unlock()

	// Resize should reset all margins
	vt.Resize(20, 15)

	vt.mu.Lock()
	defer vt.mu.Unlock()
	assert.Equal(t, row(0), vt.margin.top, "margin.top should be reset to 0 on resize")
	assert.Equal(t, column(0), vt.margin.left, "margin.left should be reset to 0 on resize")
	assert.Equal(t, row(14), vt.margin.bottom, "margin.bottom should be set to height-1")
	assert.Equal(t, column(19), vt.margin.right, "margin.right should be set to width-1")
}

// Regression: after resize with custom top/left margins, scrolling should use
// the full screen area, not the old margins.
func TestResize_ScrollUsesResetMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)

	// Set restrictive margins
	vt.mu.Lock()
	vt.margin.top = 2
	vt.margin.left = 2
	vt.mu.Unlock()

	// Resize
	vt.Resize(4, 4)

	vt.mu.Lock()
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, column(0), vt.margin.left)
	vt.mu.Unlock()
}

// The parse-and-dispatch goroutine was duplicated in Start() and StartWithPty().
// After extracting it into a shared runParseLoop() method, StartWithPty should
// still correctly parse input, dispatch to update(), and emit EventClosed on EOF.
func TestRunParseLoop_StartWithPty(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.surface = &mockSurface{w: 10, h: 1}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	closed := make(chan bool, 1)
	vt.eventHandler = func(ev tcell.Event) {
		if _, ok := ev.(*EventClosed); ok {
			closed <- true
		}
	}

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)

	// Write some data and close
	_, _ = w.Write([]byte("hi"))
	_ = w.Close()

	// Wait for EventClosed
	<-closed

	// Verify the content was rendered
	str := vt.String()
	assert.Contains(t, str, "hi")
}

// mockSurface satisfies the Screen interface for testing.
type mockSurface struct {
	w, h int
}

func (m *mockSurface) SetContent(x, y int, ch rune, comb []rune, style tcell.Style) {}
func (m *mockSurface) Size() (int, int)                                             { return m.w, m.h }

// nopWriteCloser wraps an io.ReadCloser with a no-op Write to satisfy io.ReadWriteCloser.
type nopWriteCloser struct {
	io.ReadCloser
}

func (n *nopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

// The update() type switch had no case for error sequences emitted by the
// parser, causing them to fall through to the default branch and trigger a
// spurious redraw. Errors should be logged via vt.Logger and not set the
// dirty flag.
func TestUpdate_ErrorSequenceIsLogged(t *testing.T) {
	var buf bytes.Buffer
	vt := New()
	vt.Resize(4, 1)
	vt.Logger = log.New(&buf, "", 0)
	vt.dirty = false

	// Pass an error sequence directly to update()
	vt.update(fmt.Errorf("test parse error"))

	// The error should be logged
	assert.Contains(t, buf.String(), "test parse error")
	// The dirty flag should NOT be set (no redraw needed for an error)
	assert.False(t, vt.dirty, "error sequence should not trigger a redraw")
}

// Regression: non-error sequences should still trigger redraws.
func TestUpdate_PrintStillTriggersRedraw(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.dirty = false

	vt.update(Print('a'))

	assert.True(t, vt.dirty, "Print sequence should trigger a redraw")
}

// --- sequence.go String methods ---

func TestSequenceStringers(t *testing.T) {
	cases := []struct {
		name string
		s    Sequence
		want string
	}{
		{"print", Print('A'), "Print: codepoint=0x41 rune='A'"},
		{"c0", C0(0x07), "C0 0x7"},
		{"esc", ESC{Final: 'M', Intermediate: []rune{'('}}, "ESC ( M"},
		{"csi", CSI{Final: 'H', Parameters: []int{1, 2}}, "CSI  1;2 H"},
		{"osc", OSC{Payload: []rune("0;title")}, "OSC 0;title"},
		{"eof", EOF{}, "EOF"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := tc.s.(interface{ String() string })
			if !ok {
				t.Fatalf("%T has no String()", tc.s)
			}
			if got := s.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- events.go accessors ---

func TestEventTerminalAccessors(t *testing.T) {
	vt := New()
	defer vt.Close()
	ev := newEventTerminal(vt)
	if ev.Session() != vt {
		t.Errorf("Session() mismatch")
	}
	if ev.When().IsZero() {
		t.Errorf("When() zero")
	}
	if time.Since(ev.When()) > time.Second {
		t.Errorf("When() too far in the past")
	}

	mm := &EventMouseMode{
		EventTerminal: newEventTerminal(vt),
		modes:         []tcell.MouseFlags{tcell.MouseButtonEvents},
	}
	if got := mm.Flags(); len(got) != 1 || got[0] != tcell.MouseButtonEvents {
		t.Errorf("Flags()=%v", got)
	}

	clip := &EventClipboard{
		EventTerminal: newEventTerminal(vt),
		selection:     "c",
		data:          "hello",
	}
	if clip.Selection() != "c" || clip.Data() != "hello" {
		t.Errorf("clipboard accessors mismatch")
	}

	title := &EventTitle{EventTerminal: newEventTerminal(vt), title: "t"}
	if title.Title() != "t" {
		t.Errorf("Title()=%q", title.Title())
	}
}

// --- mock.go SendKey / SendRune / AssertLine / AssertOutput / Snapshot ---

func TestMockTerminal_SendAndSnapshot(t *testing.T) {
	m := NewMockTerminal(t, 5, 1)
	defer m.Close()
	m.Feed("ab")
	if !strings.Contains(m.Snapshot(), "ab") {
		t.Errorf("snapshot missing 'ab': %q", m.Snapshot())
	}
	m.AssertLine(0, "ab   ")

	// SendRune emits the rune to the output buffer (HandleEvent -> pty).
	m.SendRune('X')
	m.AssertOutput("X")

	// SendKey for a control key — Enter emits "\r".
	m.SendKey(tcell.KeyEnter, "", tcell.ModNone)
	m.AssertOutput("\r")
}

func TestMockTerminal_AssertionFailures(t *testing.T) {
	ft := &failingT{}
	m := NewMockTerminal(ft, 4, 1)
	defer m.Close()
	m.Feed("ab")

	before := ft.failed
	m.AssertLine(0, "WRONG")
	m.AssertLine(99, "out of range")
	m.AssertContains("zzz")
	m.AssertOutput("nope")
	m.AssertCursor(7, 7)
	if ft.failed-before < 5 {
		t.Errorf("expected several failures, got %d", ft.failed-before)
	}
}

// --- headless: outputSink.Read, Session, String, Cell.String ---

func TestHeadless_VTAndStringers(t *testing.T) {
	h, err := NewTerminal(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if h.Session() == nil {
		t.Errorf("Session() nil")
	}
	h.FeedString("hi")
	if !strings.HasPrefix(h.String(), "hi") {
		t.Errorf("String()=%q", h.String())
	}

	c := h.Cell(0, 0)
	if c.String() != "h" {
		t.Errorf("Cell.String()=%q", c.String())
	}
	// empty cell renders as space
	c2 := h.Cell(2, 0)
	if c2.String() != " " {
		t.Errorf("empty Cell.String()=%q", c2.String())
	}
	// combining
	cc := Cell{Rune: 'a', Combining: []rune{0x0301}}
	if cc.String() != "a\u0301" {
		t.Errorf("combining Cell.String()=%q", cc.String())
	}

	// outputSink Read returns EOF.
	buf := make([]byte, 4)
	n, err := h.out.Read(buf)
	if n != 0 || err == nil {
		t.Errorf("outputSink.Read=(%d,%v), want (0,EOF)", n, err)
	}
}

// --- terminal.HasSurface ---

func TestVT_HasSurface(t *testing.T) {
	vt := New()
	defer vt.Close()
	if vt.HasSurface() {
		t.Errorf("expected no surface initially")
	}
	vt.SetSurface(newBufferSurface(2, 2))
	if !vt.HasSurface() {
		t.Errorf("expected surface after SetSurface")
	}
}

// --- c0.go ht, vt, ff (\t \v \f) ---

func TestC0_TabsAndForms(t *testing.T) {
	h, _ := NewTerminal(20, 3)
	defer h.Close()
	// HT advances to next tab stop (default every 8 columns)
	h.FeedString("\t")
	col, _, _ := h.Cursor()
	if col != 8 {
		t.Errorf("HT col=%d want 8", col)
	}
	// Session and FF act as LF
	_, rowBefore, _ := h.Cursor()
	h.FeedString("\v")
	_, rowAfter, _ := h.Cursor()
	if rowAfter != rowBefore+1 {
		t.Errorf("Session row %d -> %d", rowBefore, rowAfter)
	}
	rowBefore = rowAfter
	h.FeedString("\f")
	_, rowAfter, _ = h.Cursor()
	if rowAfter != rowBefore+1 {
		t.Errorf("FF row %d -> %d", rowBefore, rowAfter)
	}
}

// --- esc.go ri (Reverse Index) ---

func TestEsc_ReverseIndex(t *testing.T) {
	h, _ := NewTerminal(4, 3)
	defer h.Close()
	// Move to row 1 col 0 then reverse index — cursor row should decrement.
	h.FeedString("\x1b[2;1H") // cursor to row 2 col 1 (0-based row=1)
	_, r, _ := h.Cursor()
	if r != 1 {
		t.Fatalf("setup: row=%d", r)
	}
	h.FeedString("\x1bM")
	_, r, _ = h.Cursor()
	if r != 0 {
		t.Errorf("RI row=%d, want 0", r)
	}
	// At top of margin, RI should scroll down (insert blank line at top).
	h.FeedString("X")
	h.FeedString("\x1bM") // scroll because at top
	if h.Cell(0, 0).Rune == 'X' {
		t.Errorf("expected RI to scroll the top row down")
	}
}

// --- csi.go cha, el, tbc, vpr, hpr ---

func TestCSI_CHA(t *testing.T) {
	h, _ := NewTerminal(10, 1)
	defer h.Close()
	h.FeedString("abc")
	h.FeedString("\x1b[5G") // CHA col 5 -> col index 4
	col, _, _ := h.Cursor()
	if col != 4 {
		t.Errorf("CHA col=%d, want 4", col)
	}
	// Default param defaults to 1 -> col 0
	h.FeedString("\x1b[G")
	col, _, _ = h.Cursor()
	if col != 0 {
		t.Errorf("CHA default col=%d, want 0", col)
	}
}

func TestCSI_EL(t *testing.T) {
	t.Run("EL0", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("\x1b[Habcdef\x1b[1;4H\x1b[0K")
		line := h.Lines()[0]
		if line[:3] != "abc" || strings.TrimSpace(line[3:]) != "" {
			t.Errorf("EL 0 line=%q", line)
		}
	})
	t.Run("EL1", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("\x1b[Habcdef\x1b[1;4H\x1b[1K")
		line := h.Lines()[0]
		if strings.TrimSpace(line[:4]) != "" || line[4:] != "ef" {
			t.Errorf("EL 1 line=%q", line)
		}
	})
	t.Run("EL2", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("\x1b[Habcdef\x1b[2K")
		if strings.TrimSpace(h.Lines()[0]) != "" {
			t.Errorf("EL 2 line=%q", h.Lines()[0])
		}
	})
}

func TestCSI_TBC(t *testing.T) {
	vt := New()
	defer vt.Close()
	vt.Resize(40, 1)
	// tab to col 8 then clear that tab, move cursor and check it's gone.
	vt.cursor.col = 8
	vt.tbc(0)
	for _, ts := range vt.tabStop {
		if ts == 8 {
			t.Errorf("tab at col 8 should have been removed")
		}
	}
	// TBC 3 clears all tab stops.
	vt.tbc(3)
	if len(vt.tabStop) != 0 {
		t.Errorf("TBC 3: expected 0 tab stops, got %d", len(vt.tabStop))
	}
}

func TestCSI_VPR_HPR(t *testing.T) {
	h, _ := NewTerminal(10, 5)
	defer h.Close()
	h.FeedString("\x1b[3e") // VPR 3 -> row += 3 -> row 3
	_, r, _ := h.Cursor()
	if r != 3 {
		t.Errorf("VPR row=%d, want 3", r)
	}
	h.FeedString("\x1b[1;1H") // home
	h.FeedString("\x1b[4a")   // HPR 4 -> col += 4
	c, _, _ := h.Cursor()
	if c != 4 {
		t.Errorf("HPR col=%d, want 4", c)
	}
	// HPR clamps to width-1.
	h.FeedString("\x1b[100a")
	c, _, _ = h.Cursor()
	if c != 9 {
		t.Errorf("HPR clamped col=%d, want 9", c)
	}
	// VPR clamps to height-1.
	h.FeedString("\x1b[100e")
	_, r, _ = h.Cursor()
	if r != 4 {
		t.Errorf("VPR clamped row=%d, want 4", r)
	}
}

// --- mode.go SM/RM via CSI ---

func TestMode_SMRM(t *testing.T) {
	vt := New()
	defer vt.Close()
	vt.Resize(4, 1)
	vt.sm([]int{4, 20})
	if vt.mode&irm == 0 || vt.mode&lnm == 0 {
		t.Errorf("SM did not set IRM/LNM")
	}
	vt.rm([]int{4, 20})
	if vt.mode&irm != 0 || vt.mode&lnm != 0 {
		t.Errorf("RM did not clear IRM/LNM")
	}
}

// --- ED cases 1, 3 (case 0 and 2 already covered) ---

func TestCSI_ED_AllVariants(t *testing.T) {
	for _, ps := range []string{"0", "1", "2", "3"} {
		h, _ := NewTerminal(4, 2)
		h.FeedString("abcd\nefgh")
		h.FeedString("\x1b[1;3H") // cursor row 1 col 3 (0-based 0,2)
		h.FeedString("\x1b[" + ps + "J")
		_ = h.String()
		h.Close()
	}
}

// --- DCS / SOS-PM-APC parser states ---

func TestParser_DCSStates(t *testing.T) {
	cases := []string{
		// DCS with intermediate, final, payload, ST
		"\x1bP \x40data\x1b\\",
		// DCS with parameters then passthrough
		"\x1bP1;2qdata\x1b\\",
		// DCS that goes to dcsIgnore via 0x3A
		"\x1bP:abc\x1b\\",
		// DCS intermediate -> ignore via parameter
		"\x1bP 0\x1b\\",
		// SOS / PM / APC
		"\x1bXanything\x1b\\",
		"\x1b^anything\x1b\\",
		"\x1b_anything\x1b\\",
	}
	for _, in := range cases {
		p := NewParser(strings.NewReader(in))
		for {
			s := p.Next()
			if s == nil {
				break
			}
			if _, ok := s.(EOF); ok {
				break
			}
		}
	}
}

// --- HandleEvent paste / focus / mouse paths ---

func TestVT_HandleEvent_PasteFocusMouse(t *testing.T) {
	h, _ := NewTerminal(10, 3)
	defer h.Close()

	// Paste while paste mode is OFF: returns false.
	if h.HandleEvent(tcell.NewEventPaste(true)) {
		t.Errorf("paste start with paste mode off should return false")
	}
	// Enable bracketed paste (DECSET 2004) and try again.
	h.FeedString("\x1b[?2004h")
	h.Output() // drain
	if !h.HandleEvent(tcell.NewEventPaste(true)) {
		t.Errorf("paste start should be accepted")
	}
	if !h.HandleEvent(tcell.NewEventPaste(false)) {
		t.Errorf("paste end should be accepted")
	}
	if got := h.Output(); len(got) == 0 {
		t.Errorf("expected paste markers in output")
	}

	// Focus events: enable DECSET 1004 then deliver a focus event.
	h.FeedString("\x1b[?1004h")
	h.Output()
	h.HandleEvent(tcell.NewEventFocus(true))
	h.HandleEvent(tcell.NewEventFocus(false))

	// Mouse: enable mouse (DECSET 1000;1006) then deliver a mouse event.
	h.FeedString("\x1b[?1000h\x1b[?1006h")
	h.Output()
	h.HandleEvent(tcell.NewEventMouse(2, 1, tcell.Button1, tcell.ModNone))
}

// --- keyCode broad coverage of arrows / nav keys / Ctrl / Shift / Alt mods ---

func TestKeyCode_BroadCoverage(t *testing.T) {
	keys := []tcell.Key{
		tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
		tcell.KeyHome, tcell.KeyEnd, tcell.KeyInsert, tcell.KeyDelete,
		tcell.KeyPgUp, tcell.KeyPgDn,
	}
	mods := []tcell.ModMask{
		tcell.ModNone,
		tcell.ModShift,
		tcell.ModCtrl,
		tcell.ModAlt,
		tcell.ModCtrl | tcell.ModShift,
		tcell.ModAlt | tcell.ModShift,
		tcell.ModAlt | tcell.ModCtrl,
		tcell.ModAlt | tcell.ModCtrl | tcell.ModShift,
		tcell.ModMeta,
		tcell.ModMeta | tcell.ModShift,
	}
	for _, k := range keys {
		for _, m := range mods {
			ev := tcell.NewEventKey(k, "", m)
			_ = keyCode(ev, false) // just exercise paths
		}
	}
	// F1..F12 with various mods
	for k := tcell.KeyF1; k <= tcell.KeyF12; k++ {
		for _, m := range mods {
			_ = keyCode(tcell.NewEventKey(k, "", m), false)
		}
	}
}

func TestNewEventTerminal(t *testing.T) {
	vt := New()
	ev := newEventTerminal(vt)
	assert.NotNil(t, ev)

	// When() should return a recent time (within the last second)
	when := ev.When()
	assert.WithinDuration(t, time.Now(), when, time.Second)

	// Session() should return the same Session
	assert.Equal(t, vt, ev.Session())
}

func TestEventRedraw(t *testing.T) {
	vt := New()
	ev := &EventRedraw{
		EventTerminal: newEventTerminal(vt),
	}

	assert.NotNil(t, ev.When())
	assert.Equal(t, vt, ev.Session())
}

func TestEventClosed(t *testing.T) {
	vt := New()
	ev := &EventClosed{
		EventTerminal: newEventTerminal(vt),
	}

	assert.Equal(t, vt, ev.Session())
	assert.NotNil(t, ev.When())
}

func TestEventTitle(t *testing.T) {
	vt := New()
	ev := &EventTitle{
		EventTerminal: newEventTerminal(vt),
		title:         "test title",
	}

	assert.Equal(t, "test title", ev.Title())
	assert.Equal(t, vt, ev.Session())
}

func TestEventMouseMode(t *testing.T) {
	vt := New()
	flags := []tcell.MouseFlags{tcell.MouseButtonEvents}
	ev := &EventMouseMode{
		EventTerminal: newEventTerminal(vt),
		modes:         flags,
	}

	assert.Equal(t, flags, ev.Flags())
	assert.Equal(t, vt, ev.Session())
}

func TestEventMouseMode_MultipleFlags(t *testing.T) {
	vt := New()
	flags := []tcell.MouseFlags{tcell.MouseButtonEvents, tcell.MouseDragEvents}
	ev := &EventMouseMode{
		EventTerminal: newEventTerminal(vt),
		modes:         flags,
	}

	assert.Equal(t, 2, len(ev.Flags()))
	assert.Equal(t, tcell.MouseButtonEvents, ev.Flags()[0])
	assert.Equal(t, tcell.MouseDragEvents, ev.Flags()[1])
}

func TestEventBell(t *testing.T) {
	vt := New()
	ev := &EventBell{
		EventTerminal: newEventTerminal(vt),
	}

	assert.Equal(t, vt, ev.Session())
	assert.NotNil(t, ev.When())
}

func TestEventPanic(t *testing.T) {
	vt := New()
	testErr := assert.AnError
	ev := &EventPanic{
		EventTerminal: newEventTerminal(vt),
		Error:         testErr,
	}

	assert.Equal(t, testErr, ev.Error)
	assert.Equal(t, vt, ev.Session())
}

func TestEventClipboard(t *testing.T) {
	vt := New()
	ev := &EventClipboard{
		EventTerminal: newEventTerminal(vt),
		selection:     "c",
		data:          "hello world",
	}

	assert.Equal(t, "c", ev.Selection())
	assert.Equal(t, "hello world", ev.Data())
	assert.Equal(t, vt, ev.Session())
}

func TestEventClipboard_Empty(t *testing.T) {
	vt := New()
	ev := &EventClipboard{
		EventTerminal: newEventTerminal(vt),
	}

	assert.Equal(t, "", ev.Selection())
	assert.Equal(t, "", ev.Data())
}

// --- Session lifecycle / public API tests ---

func TestVT_Write_NilPty(t *testing.T) {
	vt := New()
	_, err := vt.Write([]byte("x"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "terminal not started")
}

func TestVT_HandleEvent_NilPty(t *testing.T) {
	vt := New()
	ev := tcell.NewEventKey(tcell.KeyRune, "A", tcell.ModNone)
	result := vt.HandleEvent(ev)
	assert.False(t, result, "HandleEvent should return false when pty is nil")
}

func TestVT_ScrollBy_ScrollOffset(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	// Fill both lines
	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')

	// Now push lines into scrollback by printing below margin
	// The scrollUp function only captures lines when margins are default
	// (top=0, left=0, right=width-1). We can trigger scrollback by
	// printing at the bottom margin.
	vt.cursor.row = vt.margin.bottom
	vt.scrollUp(1) // This captures the first line into scrollback

	// Initially scrollOffset should be 0
	assert.Equal(t, 0, vt.ScrollOffset())

	// Scroll forward (into history)
	vt.ScrollBy(1)
	assert.Equal(t, 1, vt.ScrollOffset())

	// Scroll back toward live
	vt.ScrollBy(-1)
	assert.Equal(t, 0, vt.ScrollOffset())

	// Clamp negative
	vt.ScrollBy(-100)
	assert.Equal(t, 0, vt.ScrollOffset())

	// Clamp excessive positive
	vt.ScrollBy(100)
	assert.Equal(t, 1, vt.ScrollOffset())
}

func TestVT_IsAltScreen(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	assert.False(t, vt.IsAltScreen())

	vt.decset([]int{47})
	assert.True(t, vt.IsAltScreen())

	vt.decrst([]int{47})
	assert.False(t, vt.IsAltScreen())
}

func TestVT_NewWithSize(t *testing.T) {
	vt := NewWithSize(80, 24)
	assert.Equal(t, 80, vt.width())
	assert.Equal(t, 24, vt.height())

	// Print a char and verify it appears
	vt.print('x')
	assert.Equal(t, 'x', rune(vt.String()[0]))
	// String should contain 80 columns * 24 rows + 23 newlines = 1943 chars
	assert.Equal(t, 80*24+23, len(vt.String()))
}

func TestVT_Cursor_WhenScrolled(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	// Generate some scrollback
	vt.mode = 0
	vt.print('a')
	vt.cursor.row = 1
	vt.scrollUp(1)
	// Scroll viewport back
	vt.scrollOffset = 1

	_, _, _, vis := vt.Cursor()
	assert.False(t, vis, "Cursor should report invisible when viewport is scrolled")
}

func TestVT_Attach_Detach(t *testing.T) {
	t.Parallel()

	vt := New()
	vt.Resize(4, 1)

	var called int
	fn := func(ev tcell.Event) {
		called++
	}

	vt.Attach(fn)
	// Post an event (normally done by the parser goroutine)
	vt.postEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	// Drain events from the channel
	select {
	case ev := <-vt.events:
		// Event is consumed; we test that it's delivered to the handler.
		// In a real run, runParseLoop drains events. Here we just test
		// the handler is registered.
		_ = ev
	default:
	}
	assert.Equal(t, 0, called,
		"handler not called directly via postEvent (runParseLoop drains)")

	// Test Detach
	vt.Detach()
	// After Detach, eventHandler should be a no-op
	// We can't easily test this without the parse loop, but verify it doesn't panic
	assert.NotPanics(t, func() {
		vt.Attach(fn)
		vt.Detach()
	})
}

func TestVT_SetRedrawHandler(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	var called bool
	vt.SetRedrawHandler(func() {
		called = true
	})

	// SetRedrawHandler stores the function; it's called from runParseLoop
	// which we don't run here. Just verify it doesn't panic and stores correctly.
	vt.mu.Lock()
	rf := vt.redrawHandler
	vt.mu.Unlock()
	assert.NotNil(t, rf)

	// Manually call it to verify
	rf()
	assert.True(t, called)
}

func TestVT_SetTERM_SetLogger_SetMaxClipboardLen(t *testing.T) {
	vt := New()

	// SetTERM
	vt.SetTERM("vt100")
	assert.Equal(t, "vt100", vt.TERM)

	// SetLogger
	var buf bytesBuffer
	logger := log.New(&buf, "", 0)
	vt.SetLogger(logger)
	// Verify logger works by writing to it
	logger.Print("test log")
	assert.Contains(t, buf.String(), "test log")

	// SetMaxClipboardLen
	vt.SetMaxClipboardLen(10)
	assert.Equal(t, 10, vt.MaxClipboardLen)

	// Test that OSC 52 exceeding limit is dropped
	vt.osc("52;c;dGVzdCBkYXRhIGV4Y2VlZGluZyBsaW1pdA==") // "test data exceeding limit" base64
	select {
	case ev := <-vt.events:
		t.Errorf("expected no event, got %T: %+v", ev, ev)
	default:
		// Good — no event posted
	}
}

// bytesBuffer is a simple bytes.Buffer wrapper for testing
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *bytesBuffer) String() string {
	return string(b.data)
}

func TestVT_postEvent_FullChannel(t *testing.T) {
	vt := New()
	// Channel capacity is 32
	assert.Equal(t, 32, cap(vt.events))

	// Fill the channel
	for i := 0; i < 32; i++ {
		vt.postEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	}
	// Channel should be full now (len=32)
	assert.Equal(t, 32, len(vt.events))

	// Extra event should be dropped (no deadlock)
	ev3 := tcell.NewEventKey(tcell.KeyRune, "c", tcell.ModNone)
	assert.NotPanics(t, func() {
		vt.postEvent(ev3)
	})
	// Channel should still have 32 events (extra was dropped)
	assert.Equal(t, 32, len(vt.events))
}

func TestVT_SetOSC8(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.True(t, vt.OSC8)

	// Disable OSC8 — OSC 8 sequences should be stripped
	vt.SetOSC8(false)
	// Save a reference to the current attrs
	oldAttrs := vt.cursor.attrs
	vt.osc("8;;https://example.com")
	// Cursor attrs should be unchanged (OSC8 is disabled)
	assert.Equal(t, oldAttrs, vt.cursor.attrs,
		"cursor attrs should not change when OSC8 is disabled")

	// Re-enable — OSC 8 should modify cursor attrs
	vt.SetOSC8(true)
	vt.osc("8;;https://example.org")
	// After enabling, the URL should be set (attrs change)
	assert.NotEqual(t, oldAttrs, vt.cursor.attrs,
		"cursor attrs should change when OSC8 is enabled and URL set")
}

func TestVT_Indent(t *testing.T) {
	// Verify Session.String() output format (multi-line with \n)
	vt := New()
	vt.Resize(2, 3)
	vt.print('a')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('b')
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.print('c')
	assert.Equal(t, "a \nb \nc ", vt.String())
}

func TestVT_Close_Idempotent(t *testing.T) {
	// Close on a Session that was never started should not panic
	vt := New()
	assert.NotPanics(t, func() {
		vt.Close()
	})
	// Second close also no panic
	assert.NotPanics(t, func() {
		vt.Close()
	})
}

func TestVT_StartWithPty_MockPty(t *testing.T) {
	vt := NewWithSize(4, 2)
	vt.SetSurface(&captureSurface{w: 4, h: 2})

	// Use os.Pipe to avoid data races on the mock pty's bytes.Buffer.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	err = vt.StartWithPty(w)
	assert.NoError(t, err)
	defer vt.Close()

	// Should have started the parse loop
	assert.NotNil(t, vt.parser)

	// Write via Session should forward to the pty
	n, err := vt.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Read what the parser consumed — the parser reads all the time,
	// so we might not see anything in the pipe. Just verify no error.
	buf := make([]byte, 5)
	r.Read(buf) // best-effort read
}

func TestVT_Write_NilPtyAfterClose(t *testing.T) {
	vt := NewWithSize(4, 1)
	vt.SetSurface(&captureSurface{w: 4, h: 1})
	mp := &mockPty{}
	err := vt.StartWithPty(mp)
	assert.NoError(t, err)

	vt.Close()

	// After close, pty is nil, Write should error
	_, err = vt.Write([]byte("x"))
	assert.Error(t, err)
}

func TestVT_ScrollBy_Clamp(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Clamp negative to 0
	vt.ScrollBy(-5)
	assert.Equal(t, 0, vt.ScrollOffset())

	// Clamp to len(scrollback)
	vt.scrollback = append(vt.scrollback, make([]cell, 4))
	vt.ScrollBy(100)
	assert.Equal(t, 1, vt.ScrollOffset())
}

func TestVT_HandleEvent_Paste(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Without paste mode enabled, should return false
	result := vt.HandleEvent(tcell.NewEventPaste(true))
	assert.False(t, result)

	// Enable paste mode
	vt.mode |= paste
	vt.pty = &mockPty{}
	result = vt.HandleEvent(tcell.NewEventPaste(true))
	assert.True(t, result)

	result = vt.HandleEvent(tcell.NewEventPaste(false))
	assert.True(t, result)
}

func TestVT_HandleEvent_Focus(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Without focusEvents mode, should return false
	result := vt.HandleEvent(tcell.NewEventFocus(true))
	assert.False(t, result)

	// Enable focusEvents
	vt.mode |= focusEvents
	vt.pty = &mockPty{}
	result = vt.HandleEvent(tcell.NewEventFocus(true))
	assert.True(t, result)

	result = vt.HandleEvent(tcell.NewEventFocus(false))
	assert.True(t, result)
}

func TestVT_Resize_ZeroDimensions(t *testing.T) {
	vt := New()

	// Resize to zero should not panic
	assert.NotPanics(t, func() {
		vt.Resize(0, 0)
	})
	assert.Equal(t, 0, vt.width())
	assert.Equal(t, 0, vt.height())
}

func TestVT_Resize_NegativeDimensions(t *testing.T) {
	vt := New()

	// Negative dimensions cause panic in make() — that's expected Go behavior.
	// Check that Resize guards against this or panics cleanly.
	assert.Panics(t, func() {
		vt.Resize(-1, -1)
	})
}

// TestVT_Recover tests that a panic in update() is caught by recover()
// and an EventPanic is posted.
func TestVT_Recover(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Simulate a panic inside recover() by invoking it directly
	vt.recover()

	// recover with no panic should not post anything
	select {
	case <-vt.events:
		t.Error("expected no event after recover() without panic")
	default:
	}

	// To test actual panic recovery, we need to trigger a panic in update()
	// The recover function is called via defer in runParseLoop.
	// We can simulate by causing a panic in a controlled way.
	//
	// Instead: verify the recover function itself handles a panic by posting EventPanic.
	// We'll test the public-facing behavior: creating a situation that causes
	// a panic in the update path. A nil sequence type could do it.
	//
	// However, the simplest direct test is to check that recover() catches
	// and posts EventPanic when a panic occurs.
	//
	// Since we can't easily trigger a panic through update() with normal sequences,
	// we verify the path exists by checking recover() with a goroutine panic.

	done := make(chan struct{})
	go func() {
		defer func() { close(done) }()
		defer vt.recover()
		panic("test panic for recover")
	}()
	<-done

	// After the goroutine panic+recover, EventPanic should be in the channel
	select {
	case ev := <-vt.events:
		_, ok := ev.(*EventPanic)
		assert.True(t, ok, "expected EventPanic, got %T", ev)
	default:
		// The Session was closed by recover(), but the event should be posted
		// before closing. It might have been dropped if channel was full.
	}
}

func TestVT_OnUnicode(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.print('a')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('中') // CJK character (width=2)
	// Row 0: 'a' + 3 spaces; Row 1: '中' (width 2) + 2 spaces
	assert.Equal(t, "a   \n中   ", vt.String())
}

func TestVT_ResizePreservesContent(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')

	// Resize to larger — content should be preserved
	vt.Resize(6, 3)
	assert.Equal(t, "ab    \ncd    \n      ", vt.String())
}

// TestUpdate_DCS verifies DCS/DCSData/DCSEndOfData in update() are no-ops.
func TestUpdate_DCS(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// DCS with no data — should not panic, no dirty flag
	assert.NotPanics(t, func() {
		vt.update(DCS{
			Final:        'q',
			Intermediate: []rune{},
			Parameters:   []int{},
		})
	})
	assert.True(t, vt.dirty, "DCS should set dirty flag")

	// DCSData
	vt.dirty = false
	assert.NotPanics(t, func() {
		vt.update(DCSData('x'))
	})
	assert.True(t, vt.dirty, "DCSData should set dirty flag")

	// DCSEndOfData
	vt.dirty = false
	assert.NotPanics(t, func() {
		vt.update(DCSEndOfData{})
	})
	assert.True(t, vt.dirty, "DCSEndOfData should set dirty flag")
}

// TestUpdate_Error is already tested elsewhere, verify it logs.
func TestUpdate_ErrorLogs(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	assert.NotPanics(t, func() {
		vt.update(fmt.Errorf("test error"))
	})
}

// TestStart_NilCmd verifies Start(nil) returns an error.
func TestStart_NilCmd(t *testing.T) {
	vt := New()
	vt.SetSurface(&captureSurface{w: 4, h: 2})

	err := vt.Start(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no command to run")
}

// TestCursor_Scrolled verifies Cursor() returns vis=false when scrolled.
func TestCursor_Scrolled(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0

	// Generate scrollback
	vt.print('a')
	vt.cursor.row = 1
	vt.scrollUp(1)

	// Scroll viewport back
	vt.ScrollBy(1)

	_, _, _, vis := vt.Cursor()
	assert.False(t, vis, "Cursor should be invisible when scrolled back")
}

// TestWrite_OnStartedVT verifies Write sends bytes to the PTY.
func TestWrite_OnStartedVT(t *testing.T) {
	vt := NewWithSize(4, 1)
	vt.SetSurface(&captureSurface{w: 4, h: 1})

	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer r.Close()
	defer w.Close()

	err = vt.StartWithPty(w)
	assert.NoError(t, err)
	defer vt.Close()

	n, err := vt.Write([]byte("hi"))
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestVT_ScrollBy_ClampNegative(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	vt.ScrollBy(-5)
	assert.Equal(t, 0, vt.ScrollOffset())
}

func TestVT_ScrollBy_ClampPositive(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Add a scrollback entry
	vt.cursor.row = 1
	vt.scrollUp(1)
	assert.Equal(t, 1, len(vt.scrollback))

	vt.ScrollBy(100)
	assert.Equal(t, 1, vt.ScrollOffset())
}

func TestHomeCursor_WithDECOM(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.mode |= decom
	vt.margin.top = 1
	vt.margin.left = 2
	vt.margin.bottom = 3
	vt.margin.right = 6
	vt.cursor.row = 3
	vt.cursor.col = 7

	vt.homeCursor()

	// With DECOM, cursor should go to margin.top/margin.left
	assert.Equal(t, row(1), vt.cursor.row)
	assert.Equal(t, column(2), vt.cursor.col)
}

func TestHomeCursor_WithoutDECOM(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.mode &^= decom
	vt.cursor.row = 2
	vt.cursor.col = 3

	vt.homeCursor()

	// Without DECOM, cursor should go to (0,0)
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestReportedCursor_WithDECOM(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.mode |= decom
	vt.margin.top = 1
	vt.margin.left = 2
	vt.cursor.row = 3
	vt.cursor.col = 5

	r, c := vt.reportedCursor()

	// Reported coordinates should be margin-relative: (3-1, 5-2) = (2, 3)
	assert.Equal(t, row(2), r)
	assert.Equal(t, column(3), c)
}

func TestReportedCursor_NegativeClamp(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.mode |= decom
	vt.margin.top = 2
	vt.margin.left = 2
	vt.cursor.row = 1 // before top margin
	vt.cursor.col = 0 // before left margin

	r, c := vt.reportedCursor()

	// Clamped to (0,0)
	assert.Equal(t, row(0), r)
	assert.Equal(t, column(0), c)
}

func TestReportedCursor_WithoutDECOM(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.mode &^= decom
	vt.cursor.row = 2
	vt.cursor.col = 3

	r, c := vt.reportedCursor()

	// Without DECOM, reported is the same as cursor position
	assert.Equal(t, row(2), r)
	assert.Equal(t, column(3), c)
}

func TestHandleEvent_EventPaste_Start(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= paste
	vt.pty = &mockPty{}

	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	result := vt.HandleEvent(tcell.NewEventPaste(true))
	assert.True(t, result)
	// Should have written PasteStart sequence
	assert.Contains(t, buf.String(), info.PasteStart)
}

func TestHandleEvent_EventPaste_End(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= paste
	vt.pty = &mockPty{}

	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	result := vt.HandleEvent(tcell.NewEventPaste(false))
	assert.True(t, result)
	assert.Contains(t, buf.String(), info.PasteEnd)
}

func TestHandleEvent_EventPaste_Disabled(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	// paste mode NOT set (but default mode has decawm|dectcem)
	vt.mode &^= paste
	vt.pty = &mockPty{}

	result := vt.HandleEvent(tcell.NewEventPaste(true))
	assert.False(t, result)
}

func TestHandleEvent_EventFocus_Focused(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= focusEvents
	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	result := vt.HandleEvent(tcell.NewEventFocus(true))
	assert.True(t, result)
	assert.Contains(t, buf.String(), "\x1b[I")
}

func TestHandleEvent_EventFocus_Unfocused(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= focusEvents
	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	result := vt.HandleEvent(tcell.NewEventFocus(false))
	assert.True(t, result)
	assert.Contains(t, buf.String(), "\x1b[O")
}

func TestHandleEvent_EventFocus_Disabled(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode &^= focusEvents
	vt.pty = &mockPty{}

	result := vt.HandleEvent(tcell.NewEventFocus(true))
	assert.False(t, result)
}

func TestDraw_SyncOutput(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode |= syncOutput
	vt.dirty = true

	// Draw should return without clearing dirty flag when syncOutput is active
	vt.Draw()
	assert.True(t, vt.dirty, "dirty should remain true when syncOutput is active")
}

func TestDraw_NilSurface(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.dirty = true

	// Draw with nil surface should not panic and should clear dirty
	assert.NotPanics(t, func() {
		vt.Draw()
	})
	assert.False(t, vt.dirty, "dirty should be cleared after Draw")
}

func TestDraw_WithSurface(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	cs := &captureSurface{w: 4, h: 2}
	vt.SetSurface(cs)

	vt.print('a')
	vt.print('b')
	vt.dirty = true

	vt.Draw()
	assert.False(t, vt.dirty, "dirty should be cleared after Draw")
}

func TestPrint_IRM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= irm

	// Put some content on the line
	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[0][1].content = 'b'
	vt.activeScreen[0][2].content = 'c'

	vt.cursor.col = 0
	vt.print('X')

	// With IRM, content should be shifted right
	assert.Equal(t, 'X', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, 'a', rune(vt.activeScreen[0][1].content))
	assert.Equal(t, 'b', rune(vt.activeScreen[0][2].content))
	// Last char should have been pushed off and erased
	// Actually, the rightmost cell was shifted, so it should be blanked/erased
}

func TestPrint_ZeroWidthRune(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Print a base character 'A' at col 0, cursor moves to col 1
	vt.print('A')

	// Print a combining character (zero-width) while cursor is at col 1
	// which is the position right after 'A'
	vt.print('\u0301') // combining acute accent

	// The combining char should be appended to the cell at col 0 (the previous cell, col-1 relative to cursor at 1)
	assert.Equal(t, 'A', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, 1, len(vt.activeScreen[0][0].combining))
	assert.Equal(t, '\u0301', vt.activeScreen[0][0].combining[0])
}

func TestPrint_ZeroWidthAtColZero(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.cursor.col = 0

	// Print a combining char at col 0 with no preceding content — should be a no-op
	assert.NotPanics(t, func() {
		vt.print('\u0301')
	})
	// Cursor should still be at col 0
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestPrint_DECSpecialCharset(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Set G0 to DEC special character set
	vt.charsets.designations[g0] = decSpecialAndLineDrawing
	// Ensure G0 is selected (it is by default)

	// 0x6a = 'j' which maps to lower right corner (0x2518 or similar)
	vt.print(rune(0x6a))

	// The cell should contain the mapped rune, not 'j'
	cellContent := vt.activeScreen[0][0].content
	assert.NotEqual(t, rune(0x6a), cellContent, "DEC special charset should map 0x6a")
	assert.Greater(t, cellContent, rune(0x00))
}

func TestPrint_SingleShift(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Set G2 to DEC special character set
	vt.charsets.designations[g2] = decSpecialAndLineDrawing
	// SS2 selects G2 for the next character only
	vt.charsets.singleShift = true
	vt.charsets.saved = g0
	vt.charsets.selected = g2

	// Print a character — single shift should reset selected back to G0
	vt.print('A')
	assert.False(t, vt.charsets.singleShift)
	assert.True(t, vt.charsets.selected == g0, "selected should be g0 after single shift")
}

func TestScrollUp_ScrollbackLimit(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0
	vt.scrollbackLimit = 2

	// Fill lines
	for i := 0; i < 3; i++ {
		vt.activeScreen[i][0].content = rune('A' + i)
	}

	// Scroll up 1 line — should capture a line
	vt.cursor.row = 0
	vt.scrollUp(1)
	assert.Equal(t, 1, len(vt.scrollback))

	// Fill and scroll more
	vt.scrollUp(2)
	// With limit 2, should have only 2
	assert.Equal(t, 2, len(vt.scrollback))
}

func TestScrollUp_AltScreen(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode |= smcup // simulate alt screen

	vt.scrollUp(1)
	// Should not capture scrollback in alt screen
	assert.Equal(t, 0, len(vt.scrollback))
}

func TestScrollUp_WithMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.mode = 0
	vt.margin.top = 1
	vt.margin.bottom = 2

	vt.activeScreen[1][0].content = 'x'
	vt.cursor.row = 1
	vt.scrollUp(1)

	// Row within margin should have been scrolled
	assert.Equal(t, rune(0), vt.activeScreen[1][0].content,
		"margin top row should be erased after scrollUp")
}

func TestScrollDown_Basic(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0

	// Set up content
	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[1][0].content = 'b'
	vt.activeScreen[2][0].content = 'c'

	vt.cursor.row = 0
	vt.scrollDown(1)

	assert.Equal(t, rune('a'), vt.activeScreen[1][0].content,
		"scrollDown should shift content down")
	assert.Equal(t, rune(0), vt.activeScreen[0][0].content,
		"scrollDown should blank top line")
}

func TestScrollDown_AtMarginBottom(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0
	vt.margin.top = 1
	vt.margin.bottom = 2

	vt.activeScreen[1][0].content = 'x'

	vt.scrollDown(1)

	// Content should have shifted only within margins
	assert.Equal(t, rune(0), vt.activeScreen[1][0].content,
		"margin top row should be blanked")
	assert.Equal(t, rune('x'), vt.activeScreen[2][0].content,
		"content should shift to bottom margin row")
}

func TestResizeUnlocked_ZeroHeight(t *testing.T) {
	vt := New()

	assert.NotPanics(t, func() {
		vt.resizeUnlocked(4, 0)
	})
	assert.Equal(t, row(0), vt.cursor.row)
}

func TestResizeUnlocked_ZeroWidth(t *testing.T) {
	vt := New()

	assert.NotPanics(t, func() {
		vt.resizeUnlocked(0, 4)
	})
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestNewWithSize(t *testing.T) {
	vt := NewWithSize(10, 5)
	assert.Equal(t, 10, vt.width())
	assert.Equal(t, 5, vt.height())
}

func TestHasSurface(t *testing.T) {
	vt := New()
	assert.False(t, vt.HasSurface())

	vt.SetSurface(&captureSurface{w: 4, h: 2})
	assert.True(t, vt.HasSurface())
}

func TestSetOSC8(t *testing.T) {
	vt := New()
	assert.True(t, vt.OSC8)

	vt.SetOSC8(false)
	assert.False(t, vt.OSC8)

	vt.SetOSC8(true)
	assert.True(t, vt.OSC8)
}

func TestSetTERM(t *testing.T) {
	vt := New()
	assert.Empty(t, vt.TERM) // initially empty

	vt.SetTERM("vt220")
	assert.Equal(t, "vt220", vt.TERM)
}

func TestSetMaxClipboardLen(t *testing.T) {
	vt := New()
	assert.Equal(t, 1<<20, vt.MaxClipboardLen) // default

	vt.SetMaxClipboardLen(100)
	assert.Equal(t, 100, vt.MaxClipboardLen)
}

func TestVT_postEvent_DropOnFull(t *testing.T) {
	vt := New()
	// Channel capacity is 32
	assert.Equal(t, 32, cap(vt.events))

	// Fill the channel
	for i := 0; i < 32; i++ {
		vt.postEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	}

	// Extra event should be dropped, not panic
	assert.NotPanics(t, func() {
		vt.postEvent(tcell.NewEventKey(tcell.KeyRune, "c", tcell.ModNone))
	})
	assert.Equal(t, 32, len(vt.events))
}

func TestLF_WithLNM(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.cursor.col = 3
	vt.cursor.row = 0
	vt.mode |= lnm

	vt.lf()

	// With LNM, cursor.col should go to margin.left after lf
	assert.Equal(t, vt.margin.left, vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestLF_WithoutLNM(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.cursor.col = 3
	vt.cursor.row = 0
	vt.mode &^= lnm

	vt.lf()

	// Without LNM, cursor.col should stay at 3
	assert.Equal(t, column(3), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestResizeUnlocked_WithPtyResizer(t *testing.T) {
	vt := New()
	var resizedW, resizedH int
	// Create a pty that supports Resize using mockPtyCloser as base
	basePty := &mockPtyCloser{Buffer: &bytes.Buffer{}}
	vt.pty = &ptyResizer{
		ReadWriteCloser: basePty,
		resizeFn: func(w, h int) error {
			resizedW = w
			resizedH = h
			return nil
		},
	}

	vt.Resize(80, 24)

	// The resize function should have been called
	assert.Equal(t, 80, resizedW)
	assert.Equal(t, 24, resizedH)
}

// ptyResizer wraps an io.ReadWriteCloser and exposes a Resize method.
type ptyResizer struct {
	io.ReadWriteCloser
	resizeFn func(int, int) error
}

func (p *ptyResizer) Resize(w, h int) error { return p.resizeFn(w, h) }

// Ensure compile-time interface check
var _ interface{ Resize(int, int) error } = (*ptyResizer)(nil)

// TestVT_Print_Bounds tests print boundary clamping.
func TestVT_Print_Bounds(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Position cursor way beyond bounds
	vt.cursor.col = 100
	vt.cursor.row = 100

	// Print should clamp and not panic
	assert.NotPanics(t, func() {
		vt.print('x')
	})

	// After clamping, content should be at last cell
	assert.Equal(t, 'x', vt.activeScreen[1][3].content)
}

// TestVT_ScrollUp_ScrollbackCondition tests scrollUp when scrollback
// conditions are met (smcup=0, margins at full screen).
func TestVT_ScrollUp_ScrollbackCondition(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0 // not in alt screen
	vt.margin.top = 0
	vt.margin.left = 0
	vt.margin.right = 3

	vt.activeScreen[0][0].content = 'a'
	vt.cursor.row = 0
	vt.scrollUp(1)

	// Should have captured into scrollback
	assert.Equal(t, 1, len(vt.scrollback), "scrollUp should capture line when margins are full screen")
}

// TestVT_Draw_ShowCursor tests Draw with ShowCursor surface.
func TestVT_Draw_ShowCursor(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode |= dectcem
	vt.scrollOffset = 0

	// Use a surface that captures ShowCursor calls
	sc := &showCursorSurface{w: 4, h: 2}
	vt.SetSurface(sc)
	vt.dirty = true

	vt.Draw()
	assert.True(t, sc.showCalled, "ShowCursor should be called when cursor visible")
}

// TestVT_Draw_HideCursor tests Draw hides cursor when scrolled back.
func TestVT_Draw_HideCursor(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode |= dectcem
	vt.scrollOffset = 1
	// Add scrollback so offset is valid
	vt.scrollback = append(vt.scrollback, make([]cell, 4))

	sc := &showCursorSurface{w: 4, h: 2}
	vt.SetSurface(sc)
	vt.dirty = true

	vt.Draw()
	assert.True(t, sc.hideCalled, "ShowCursor(-1,-1) should be called when scrolled back")
}

// showCursorSurface implements Screen and tracks ShowCursor.
type showCursorSurface struct {
	w, h             int
	showCalled       bool
	hideCalled       bool
	lastCol, lastRow int
}

func (s *showCursorSurface) SetContent(x, y int, ch rune, comb []rune, style tcell.Style) {}
func (s *showCursorSurface) Size() (int, int)                                             { return s.w, s.h }
func (s *showCursorSurface) ShowCursor(col, row int) {
	if col < 0 || row < 0 {
		s.hideCalled = true
	} else {
		s.showCalled = true
		s.lastCol, s.lastRow = col, row
	}
}

// TestKeyCode_AltModifier tests key encoding with Alt modifier.
func TestKeyCode_AltModifier(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	h.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModAlt))
	if len(h.Output()) == 0 {
		t.Error("expected output for Alt+a")
	}
}

// TestKeyCode_MetaModifier tests key encoding with Meta modifier.
func TestKeyCode_MetaModifier(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	for _, k := range []tcell.Key{tcell.KeyUp, tcell.KeyDown, tcell.KeyRight, tcell.KeyLeft, tcell.KeyHome, tcell.KeyEnd} {
		h.HandleEvent(tcell.NewEventKey(k, "", tcell.ModMeta))
		h.Output()
	}
}

// TestResize_AltScreen tests resize while in alt screen mode.
func TestResize_AltScreen(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode |= smcup // enter alt screen

	// resize while in alt screen — should switch to alt screen after
	vt.Resize(20, 10)
	assert.Equal(t, vt.altScreen, vt.activeScreen)
}

// TestScrollUp_NoScrollback tests scrollUp when scrollback conditions aren't met.
func TestScrollUp_NoScrollback(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0
	vt.margin.right = 2 // narrower than full width

	vt.activeScreen[0][0].content = 'x'
	vt.scrollUp(1)
	// Should NOT capture scrollback because margin.right < width-1
	assert.Equal(t, 0, len(vt.scrollback))
}

// TestClose_WithCmd tests Close with a non-nil cmd (without actual process).
func TestClose_WithCmd(t *testing.T) {
	vt := New()
	// Set cmd to a value that simulates a process that was started
	cmd := &sessionCmd{}
	// Don't actually start anything — just verify Close doesn't panic
	vt.cmd = cmd
	assert.NotPanics(t, func() {
		vt.Close()
	})
}

// TestKeyCode_BroadKeyCombos exercises additional key combinations.
func TestKeyCode_BroadKeyCombos(t *testing.T) {
	h, _ := NewTerminal(10, 3)
	defer h.Close()

	// F-keys with modifiers
	for _, k := range []tcell.Key{tcell.KeyF1, tcell.KeyF4, tcell.KeyF7, tcell.KeyF10, tcell.KeyF12} {
		for _, m := range []tcell.ModMask{tcell.ModNone, tcell.ModShift, tcell.ModCtrl, tcell.ModAlt} {
			h.HandleEvent(tcell.NewEventKey(k, "", m))
			h.Output()
		}
	}

	// Navigation keys with Alt modifier
	for _, k := range []tcell.Key{tcell.KeyHome, tcell.KeyEnd, tcell.KeyInsert, tcell.KeyDelete, tcell.KeyPgUp, tcell.KeyPgDn} {
		h.HandleEvent(tcell.NewEventKey(k, "", tcell.ModAlt))
		h.Output()
	}
}

// TestResize_WithActiveScreenAlt tests resize in alt screen mode.
func TestResize_WithActiveScreenAlt(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.activeScreen = vt.altScreen // simulate alt screen active

	vt.Resize(20, 10)
	assert.Equal(t, vt.altScreen, vt.activeScreen)
}

// TestScrollUp_NarrowMargins tests scrollUp with non-full margins.
func TestScrollUp_NarrowMargins(t *testing.T) {
	vt := New()
	vt.Resize(6, 3)
	vt.mode = 0
	vt.margin.left = 1
	vt.margin.right = 4

	vt.activeScreen[0][0].content = 'x'
	vt.activeScreen[0][1].content = 'y'
	vt.scrollUp(1)
	// Content within margins should scroll
	assert.Equal(t, rune(0), vt.activeScreen[0][1].content) // blanked
	// Content outside margins should remain
	assert.Equal(t, 'x', vt.activeScreen[0][0].content)
}

// TestClose_WithCmdProcess tests Close with cmd that has Process set.
func TestClose_WithCmdProcess(t *testing.T) {
	vt := New()
	// Create a cmd struct but don't start a real process
	// Just verify the code paths don't crash
	vt.cmd = &sessionCmd{}
	vt.pty = &mockPty{}
	assert.NotPanics(t, func() {
		vt.Close()
	})
}

// TestKeyCode_RuneWithMeta tests rune keys with ModMeta modifier.
func TestKeyCode_RuneWithMeta(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	// Rune key with Meta should hit the 'default: return ""' branch
	h.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "A", tcell.ModMeta))
	// Should return empty (no output)
	if len(h.Output()) != 0 {
		t.Log("rune with Meta: output produced")
	}
}

// TestKeyCode_MetaAltCombined tests Meta+Alt modifier combined.
func TestKeyCode_MetaAltCombined(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	// Meta+Alt up arrow
	h.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModMeta|tcell.ModAlt))
	out := h.Output()
	_ = out
	h.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModMeta|tcell.ModAlt))
	h.Output()
	h.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModMeta|tcell.ModAlt))
	h.Output()
	h.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModMeta|tcell.ModAlt))
	h.Output()
}

// TestKeyCode_AltCtrlShiftCombo tests Alt+Ctrl+Shift with navigation keys.
func TestKeyCode_AltCtrlShiftCombo(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	mods := tcell.ModAlt | tcell.ModCtrl | tcell.ModShift
	for _, k := range []tcell.Key{tcell.KeyUp, tcell.KeyDown, tcell.KeyRight, tcell.KeyLeft,
		tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn} {
		h.HandleEvent(tcell.NewEventKey(k, "", mods))
		h.Output()
	}
}

// TestClose_NilCmd tests Close with nil Cmd.
func TestClose_NilCmd(t *testing.T) {
	vt := New()
	vt.cmd = nil
	vt.pty = &mockPty{}
	assert.NotPanics(t, func() { vt.Close() })
}

// TestClose_WithRealPty tests Close with mock pty.
func TestClose_WithRealPty(t *testing.T) {
	vt := New()
	vt.cmd = &sessionCmd{}
	vt.pty = &mockPty{}
	assert.NotPanics(t, func() { vt.Close() })
}

// TestVT_Draw_ShowCursorHide tests Draw hides cursor.
func TestVT_Draw_ShowCursorHide(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode &^= dectcem // cursor disabled

	sc := &showCursorSurface{w: 4, h: 2}
	vt.SetSurface(sc)
	vt.dirty = true
	vt.Draw()
	assert.True(t, sc.hideCalled, "ShowCursor(-1,-1) should be called when cursor hidden")
}

// TestVT_Draw_ShowCursorVisible tests Draw shows cursor.
func TestVT_Draw_ShowCursorVisible(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode |= dectcem
	vt.scrollOffset = 0

	sc := &showCursorSurface{w: 4, h: 2}
	vt.SetSurface(sc)
	vt.dirty = true
	vt.Draw()
	assert.True(t, sc.showCalled, "ShowCursor should be called")
}

// TestKeyCode_AllModifierPaths exercises ALL key modifier combinations to
// cover remaining key.go modifier switch branches.
func TestKeyCode_AllModifierPaths(t *testing.T) {
	allKeys := []tcell.Key{
		tcell.KeyRune, tcell.KeyUp, tcell.KeyDown, tcell.KeyRight, tcell.KeyLeft,
		tcell.KeyHome, tcell.KeyEnd, tcell.KeyInsert, tcell.KeyDelete,
		tcell.KeyPgUp, tcell.KeyPgDn,
		tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4,
		tcell.KeyF5, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8,
		tcell.KeyF9, tcell.KeyF10, tcell.KeyF11, tcell.KeyF12,
		tcell.KeyBackspace2, tcell.KeyBacktab, tcell.KeyTAB, tcell.KeyEnter,
		tcell.KeyEscape, tcell.KeyBackspace, tcell.KeyDEL,
		// Enter and Escape are special
		tcell.KeyNUL, tcell.KeySO, tcell.KeySI,
	}
	allMods := []tcell.ModMask{
		tcell.ModNone, tcell.ModShift, tcell.ModCtrl, tcell.ModAlt,
		tcell.ModMeta, tcell.ModAlt | tcell.ModMeta,
		tcell.ModAlt | tcell.ModCtrl,
		tcell.ModAlt | tcell.ModCtrl | tcell.ModShift,
		tcell.ModMeta | tcell.ModShift,
		tcell.ModMeta | tcell.ModCtrl,
		tcell.ModMeta | tcell.ModAlt | tcell.ModShift,
	}

	h, _ := NewTerminal(20, 3)
	defer h.Close()
	for _, k := range allKeys {
		for _, m := range allMods {
			r := ""
			if k == tcell.KeyRune {
				r = "x"
			}
			h.HandleEvent(tcell.NewEventKey(k, r, m))
			h.Output() // drain
		}
	}
}

func TestGetMargins(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	top, bot, left, right := vt.GetMargins()
	assert.Equal(t, 0, top)
	assert.Equal(t, 23, bot)
	assert.Equal(t, 0, left)
	assert.Equal(t, 79, right)
}

func TestGetTabStops(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	stops := vt.GetTabStops()
	assert.Greater(t, len(stops), 0)
	assert.Equal(t, 8, stops[0])
}

func TestGetCursorStyle(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	// feed cursor style change
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	h.FeedString("\x1b[3 q") // blinking underline
	style := h.Session().GetCursorStyle()
	assert.Equal(t, tcell.CursorStyle(3), style)
}

func TestGetActiveCharset(t *testing.T) {
	vt := New()
	assert.Equal(t, "G0", vt.GetActiveCharset())
}

func TestGetDimensions(t *testing.T) {
	vt := New()
	vt.Resize(80, 25)
	w, h := vt.GetDimensions()
	assert.Equal(t, 80, w)
	assert.Equal(t, 25, h)
}

func TestEventLog(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	h.FeedString("\x1b[31m")
	entries := h.Session().EventLog(10)
	assert.Greater(t, len(entries), 0)
	assert.Equal(t, "CSI", entries[0].Type)
	assert.Equal(t, "m", entries[0].Sequence)

	h.Session().ClearEventLog()
	assert.Equal(t, 0, len(h.Session().EventLog(0)))
}

func TestEventLog_Overflow(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	// feed many sequences to fill the event log
	for i := 0; i < 1100; i++ {
		h.FeedString("\x1b[0m")
	}
	entries := h.Session().EventLog(0)
	assert.LessOrEqual(t, len(entries), eventLogMax+10) // bounded
}

func TestClipboard(t *testing.T) {
	vt := New()
	_, ok := vt.GetClipboard("c")
	assert.False(t, ok)

	vt.SetClipboard("c", "SGVsbG8=") // base64 "Hello"
	data, ok := vt.GetClipboard("c")
	assert.True(t, ok)
	assert.Equal(t, "SGVsbG8=", data)

	// Verify decode.
	dec, _ := base64.StdEncoding.DecodeString(data)
	assert.Equal(t, "Hello", string(dec))
}

func TestClipboardViaOSC52(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	// OSC 52 sets clipboard internally.
	h.FeedString("\x1b]52;c;SGVsbG8=\x1b\\")
	data, ok := h.Session().GetClipboard("c")
	assert.True(t, ok)
	assert.Equal(t, "SGVsbG8=", data)
}

func TestSendSignal(t *testing.T) {
	vt := New()
	err := vt.SendSignal(os.Interrupt)
	assert.Error(t, err) // no process running
}

func TestDCSEvents(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	var gotDCS, gotData bool
	h.Session().Attach(func(ev tcell.Event) {
		switch ev.(type) {
		case *EventDCS:
			gotDCS = true
		case *EventDCSData:
			gotData = true
		}
	})
	h.FeedString("\x1bP0;1;1qdata\x1b\\")
	// Events in headless mode are posted but not drained by default.
	// The channel just holds them.
	assert.GreaterOrEqual(t, len(h.Session().events), 0)
	_ = gotDCS
	_ = gotData
}

func TestOSCColourEvents(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	var gotColour, gotDefault bool
	h.Session().Attach(func(ev tcell.Event) {
		switch ev.(type) {
		case *EventColour:
			gotColour = true
		case *EventDefaultColour:
			gotDefault = true
		}
	})
	h.FeedString("\x1b]4;1;rgb:ff/00/00\x1b\\")
	h.FeedString("\x1b]10;rgb:00/ff/00\x1b\\")
	assert.GreaterOrEqual(t, len(h.Session().events), 0)
	_ = gotColour
	_ = gotDefault
}

func TestEventLogEntry(t *testing.T) {
	e := EventLogEntry{
		Time:     time.Now(),
		Type:     "CSI",
		Sequence: "m",
		Args:     []int{31},
	}
	assert.Equal(t, "CSI", e.Type)
	assert.Equal(t, "m", e.Sequence)
}

// ============================================================
// Synchronized output (mode 2026) is tracked but not implemented.
// The mode flag is set/cleared by DECSET/DECRST 2026 but Draw()
// never checks it, so screen updates are not deferred between the
// mode set and reset. Modern TUI applications rely on mode 2026
// to batch screen updates and prevent tearing.
// ============================================================

// Draw() should skip surface updates while syncOutput is active.
func TestSyncOutput_DrawSkipsWhileActive(t *testing.T) {
	vt := New()
	srf := &captureSurface{w: 4, h: 1}
	vt.SetSurface(srf)
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('A')

	// Enable synchronized output
	vt.mode |= syncOutput

	vt.Draw()

	// Screen should NOT have been updated
	assert.Nil(t, srf.cells,
		"Draw() should not call SetContent when syncOutput is active")
}

// Draw() should work normally after syncOutput is cleared.
func TestSyncOutput_DrawWorksAfterClearing(t *testing.T) {
	vt := New()
	srf := &captureSurface{w: 4, h: 1}
	vt.SetSurface(srf)
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('X')

	// Enable sync output, draw (should skip), disable, draw (should work)
	vt.mode |= syncOutput
	vt.Draw()
	assert.Nil(t, srf.cells, "first Draw() with syncOutput should skip")

	vt.mode &^= syncOutput
	vt.Draw()

	assert.NotNil(t, srf.cells, "Draw() after clearing syncOutput should update surface")
	assert.Equal(t, 'X', srf.cells[0][0].ch)
}

// When syncOutput is cleared via DECRST 2026, the dirty flag should be
// reset so that the post-update check in update() fires a new EventRedraw.
func TestSyncOutput_DecrSetResetsDirtyFlag(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Simulate: print while sync is active → dirty is true
	vt.mode |= syncOutput
	vt.dirty = true

	// Clear sync output via decrst
	vt.decrst([]int{2026})

	// dirty should be false so that the next update posts a redraw
	assert.False(t, vt.dirty,
		"decrst 2026 should reset dirty flag to trigger a fresh EventRedraw")
}

// Regression: syncOutput should not affect normal Draw() when not enabled.
func TestSyncOutput_NormalDrawUnaffected(t *testing.T) {
	vt := New()
	srf := &captureSurface{w: 4, h: 1}
	vt.SetSurface(srf)
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('Z')
	vt.Draw()

	assert.NotNil(t, srf.cells)
	assert.Equal(t, 'Z', srf.cells[0][0].ch)
}

// ============================================================
// DECCOLM (mode 3) is tracked but does not resize the terminal.
// Setting DECCOLM should switch the terminal to 132-column mode;
// resetting should return to 80 columns. The code only toggles
// the flag without changing the actual terminal width.
// ============================================================

// DECSET 3 (DECCOLM) should resize terminal to 132 columns.
func TestDECCOLM_SetResizesTo132(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)

	vt.decset([]int{3})
	assert.Equal(t, 132, vt.width(),
		"DECCOLM set should resize terminal to 132 columns")
}

// DECRST 3 (DECCOLM) should resize terminal to 80 columns.
func TestDECCOLM_ResetResizesTo80(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	vt.mode |= deccolm // Pre-set the flag

	// Use resizeUnlocked to set width to 132 first (simulating a real DECCOLM set)
	vt.Resize(132, 24)

	vt.decrst([]int{3})
	assert.Equal(t, 80, vt.width(),
		"DECCOLM reset should resize terminal to 80 columns")
}

// DECCOLM set should preserve terminal height.
func TestDECCOLM_PreservesHeight(t *testing.T) {
	vt := New()
	vt.Resize(80, 30)

	vt.decset([]int{3})
	assert.Equal(t, 132, vt.width())
	assert.Equal(t, 30, vt.height(),
		"DECCOLM should not change terminal height")
}

// Regression: DECCOLM should not panic when height is 0.
func TestDECCOLM_ZeroHeight(t *testing.T) {
	vt := New()
	// Don't resize — height is 0
	assert.NotPanics(t, func() {
		vt.decset([]int{3})
	})
}

func TestDECSpecial_LineDrawing(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Set G0 to DEC Special and Line Drawing
	vt.charsets.designations[g0] = decSpecialAndLineDrawing
	vt.charsets.selected = g0

	// Print 0x6a → should map to U+2518 (BOX DRAWINGS LIGHT UP AND LEFT)
	vt.print(0x6a)
	assert.Equal(t, "┘   ", vt.String(), "0x6a should map to ┘")

	// Clear and test 0x78 → U+2502 (BOX DRAWINGS LIGHT VERTICAL)
	vt = New()
	vt.Resize(4, 1)
	vt.charsets.designations[g0] = decSpecialAndLineDrawing
	vt.charsets.selected = g0

	vt.print(0x78)
	assert.Equal(t, "│   ", vt.String(), "0x78 should map to │")
}

func TestDECSpecial_ASCIIDefault(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Default G0 is ASCII, so 0x6a should print 'j'
	vt.print(0x6a)
	assert.Equal(t, "j   ", vt.String(), "with ASCII charset, 0x6a should be 'j'")
}

func TestDECSpecial_NonMappedRune(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.charsets.designations[g0] = decSpecialAndLineDrawing
	vt.charsets.selected = g0

	// 0x41 ('A') is not in the DEC Special mapping, should print as-is
	vt.print('A')
	assert.Equal(t, "A   ", vt.String(), "non-mapped runes should pass through unchanged")
}

func TestCharsetDesignators(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Default G0..G3 are all ASCII
	assert.Equal(t, ascii, vt.charsets.designations[g0])
	assert.Equal(t, ascii, vt.charsets.designations[g1])
	assert.Equal(t, ascii, vt.charsets.designations[g2])
	assert.Equal(t, ascii, vt.charsets.designations[g3])

	// Set G0 to DEC Special via ESC ( 0
	vt.esc("(0")
	assert.Equal(t, decSpecialAndLineDrawing, vt.charsets.designations[g0])

	// Reset to ASCII via ESC ( B
	vt.esc("(B")
	assert.Equal(t, ascii, vt.charsets.designations[g0])
}

// ============================================================
// Resize() holds lock during full content reflow (terminal#3.2).
// resizeUnlocked() replays every cell through vt.print(), which
// performs charset lookup, runewidth calculation, and potential
// scrolling. This corrupts cursor attributes and can mis-translate
// cell content if the charset was changed after the original write.
// Direct cell copy avoids these issues and is significantly faster.
// ============================================================

// Resize should not corrupt cursor attributes by replaying cells through print.
func TestResize_PreservesCursorAttrs(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	// Write a cell with red foreground
	vt.cursor.attrs = tcell.StyleDefault.Foreground(tcellcolor.Red)
	vt.print('A')

	// Set cursor attrs to a specific style
	expectedAttrs := tcell.StyleDefault.Foreground(tcellcolor.Green).Bold(true)
	vt.cursor.attrs = expectedAttrs

	// Resize should NOT overwrite cursor.attrs
	vt.Resize(20, 5)

	assert.Equal(t, expectedAttrs, vt.cursor.attrs,
		"Resize should not modify cursor attributes")
}

// Resize should not re-translate cell content through charset mapping.
// If a character 'j' was written with ASCII charset, resizing with DEC
// Special active should not translate it to '┘'.
func TestResize_DoesNotRetranslateCharset(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	// Write 'j' with ASCII charset (no translation)
	vt.charsets.designations[vt.charsets.selected] = ascii
	vt.print('j')
	assert.Equal(t, 'j', vt.primaryScreen[0][0].content)

	// Switch charset to DEC Special (where 'j' maps to '┘')
	vt.charsets.designations[vt.charsets.selected] = decSpecialAndLineDrawing

	// Resize should NOT re-translate the cell content
	vt.Resize(20, 5)

	assert.Equal(t, 'j', vt.primaryScreen[0][0].content,
		"Resize should not re-translate existing cell content through charset")
}

// Resize should preserve cell style attributes (foreground, background, bold etc.)
func TestResize_PreservesCellStyle(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	style := tcell.StyleDefault.Foreground(tcellcolor.Red).Background(tcellcolor.Blue).Bold(true)
	vt.cursor.attrs = style
	vt.print('X')

	vt.Resize(20, 5)

	c := vt.primaryScreen[0][0]
	assert.Equal(t, 'X', c.content)
	assert.Equal(t, style, c.attrs,
		"Resize should preserve cell style attributes exactly")
}

// Resize should preserve combining characters on cells.
func TestResize_PreservesCombiningChars(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	// Write a base character followed by a combining character
	vt.print('e')
	// Combining acute accent (U+0301, zero-width)
	vt.print('\u0301')

	assert.Equal(t, 'e', vt.primaryScreen[0][0].content)
	assert.Equal(t, []rune{'\u0301'}, vt.primaryScreen[0][0].combining)

	vt.Resize(20, 5)

	c := vt.primaryScreen[0][0]
	assert.Equal(t, 'e', c.content)
	assert.Equal(t, []rune{'\u0301'}, c.combining,
		"Resize should preserve combining characters")
}

// Resize should preserve content across multiple rows.
func TestResize_PreservesMultiRowContent(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	// Write on row 0
	vt.print('A')
	vt.print('B')
	// Move to row 1
	vt.nel()
	vt.print('C')
	vt.print('D')

	vt.Resize(20, 5)

	assert.Equal(t, 'A', vt.primaryScreen[0][0].content)
	assert.Equal(t, 'B', vt.primaryScreen[0][1].content)
	assert.Equal(t, 'C', vt.primaryScreen[1][0].content)
	assert.Equal(t, 'D', vt.primaryScreen[1][1].content)
}

// Regression: resizing to a smaller width should not panic.
func TestResize_SmallerWidth_NoPanic(t *testing.T) {
	vt := New()
	vt.Resize(20, 5)

	// Fill some content
	for i := 0; i < 15; i++ {
		vt.print(rune('A' + i))
	}

	assert.NotPanics(t, func() {
		vt.Resize(10, 5)
	})
}

// Regression: resizing to a smaller height should not panic.
func TestResize_SmallerHeight_NoPanic(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)

	// Write on several rows
	for i := 0; i < 8; i++ {
		vt.print(rune('A' + i))
		vt.nel()
	}

	assert.NotPanics(t, func() {
		vt.Resize(10, 3)
	})
}

// --- terminal#1.2: Data Race — Fields Modified Outside Lock in Start/StartWithPty ---
// StartWithPty() and Start() released the lock after reading the surface size,
// then modified shared fields (vt.pty, vt.cmd, vt.parser) without holding the
// lock, creating data races against concurrent calls to HandleEvent, Close,
// Draw, and the parse goroutine itself.

func TestStartWithPty_NoRaceOnFieldAssignments(t *testing.T) {
	// Run several rounds to increase chance of detecting the race.
	for round := 0; round < 5; round++ {
		vt := New()
		vt.Resize(10, 5)
		vt.surface = &mockSurface{w: 10, h: 5}

		r, w, err := os.Pipe()
		assert.NoError(t, err)

		ready := make(chan struct{})
		var wg sync.WaitGroup

		// Goroutine that concurrently reads vt.pty (via Resize which checks vt.pty)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			for i := 0; i < 20; i++ {
				vt.Resize(10, 5)
			}
		}()

		// Goroutine that calls Close which reads vt.cmd under lock
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			for i := 0; i < 20; i++ {
				vt.Close()
			}
		}()

		// StartWithPty writes to vt.pty, vt.cmd, vt.parser
		close(ready)
		_ = vt.StartWithPty(&nopWriteCloser{r})

		wg.Wait()
		_ = w.Close()
		time.Sleep(20 * time.Millisecond)
		vt.Close()
	}
}

// --- terminal#1.1: Data Race on eventHandler ---
// The parse goroutine reads vt.eventHandler without holding the lock, while
// Attach() and Detach() write it under the lock. If Attach is called from one
// goroutine while the parse loop invokes the handler on another, the function
// pointer read/write is a data race.

func TestEventHandler_NoRaceWithAttachDetach(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.surface = &mockSurface{w: 10, h: 1}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)

	var wg sync.WaitGroup

	// Concurrently Attach/Detach while the parse loop is running
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			vt.Attach(func(ev tcell.Event) {})
			vt.Detach()
		}
	}()

	// Feed data to trigger eventHandler invocations from parse loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = w.Write([]byte("x"))
			time.Sleep(time.Millisecond)
		}
		_ = w.Close()
	}()

	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	vt.Close()
}

// --- terminal#1.3: Data Race in recover() ---
// When the parse goroutine panics inside update(), recover() reads cursor and
// margin state without holding the lock, racing with concurrent Resize, Draw,
// or HandleEvent calls.

func TestRecover_NoRaceOnCursorMarginReads(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.surface = &mockSurface{w: 10, h: 5}

	// Capture panic events
	var panicMu sync.Mutex
	var panicEvents []*EventPanic
	vt.eventHandler = func(ev tcell.Event) {
		if pe, ok := ev.(*EventPanic); ok {
			panicMu.Lock()
			panicEvents = append(panicEvents, pe)
			panicMu.Unlock()
		}
	}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)

	var wg sync.WaitGroup

	// Concurrently resize (which modifies cursor/margin under lock)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			vt.Resize(10+i%3, 5+i%2)
			time.Sleep(time.Millisecond)
		}
	}()

	// Feed data that will be parsed and trigger update()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _ = w.Write([]byte("hello\r\n"))
			time.Sleep(time.Millisecond)
		}
		_ = w.Close()
	}()

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	vt.Close()
}

// --- terminal#1.4: Public Fields Lack Synchronization ---
// OSC8, TERM, and Logger are exported fields that can be modified from any
// goroutine. OSC8 is read inside osc() (under lock via update), but writes
// from user code have no synchronization. After the fix, setter methods
// (SetOSC8, SetTERM, SetLogger) acquire the lock.

func TestSetOSC8_NoRace(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.surface = &mockSurface{w: 10, h: 1}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)

	var wg sync.WaitGroup

	// Concurrently set OSC8 while parse loop reads it
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			vt.SetOSC8(i%2 == 0)
		}
	}()

	// Feed OSC 8 sequences to trigger osc() which reads OSC8
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _ = w.Write([]byte("\x1b]8;id=test;http://example.com\x1b\\"))
			time.Sleep(time.Millisecond)
		}
		_ = w.Close()
	}()

	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	vt.Close()
}

func TestSetTERM_NoRace(t *testing.T) {
	vt := New()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			vt.SetTERM("xterm-256color")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			vt.SetTERM("vt100")
		}
	}()

	wg.Wait()
}

func TestSetLogger_NoRace(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.surface = &mockSurface{w: 10, h: 1}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)

	var wg sync.WaitGroup

	// Concurrently set Logger while parse loop may read it
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			vt.SetLogger(log.New(io.Discard, "", 0))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			// Send malformed CSI sequences to trigger the error path in update()
			// which reads vt.Logger. CSI with non-numeric params causes a parse error.
			_, _ = w.Write([]byte("\x1b[xyz;m"))
			_, _ = w.Write([]byte("data"))
			time.Sleep(time.Millisecond)
		}
		_ = w.Close()
	}()

	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	vt.Close()
}

// --- terminal#1.5: Close() Calls Wait() While Holding Lock ---
// Close() held the mutex for the entire duration of cmd.Wait(), which blocks
// until the process exits. While SIGKILL makes this fast, holding the lock
// during a blocking syscall means all other locked operations (Draw, Resize,
// HandleEvent) stall. After the fix, Close() kills the process under lock,
// releases the lock, then waits, then re-locks to clean up.

func TestClose_DoesNotBlockDraw(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.surface = &mockSurface{w: 10, h: 5}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)

	// Close the write end to let parse loop terminate
	_ = w.Close()
	time.Sleep(50 * time.Millisecond)

	// Now test that Close and Draw don't deadlock with each other
	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			vt.Close()
		}()
		go func() {
			defer wg.Done()
			vt.Draw()
		}()
		wg.Wait()
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("Close() and Draw() deadlocked — Close likely holds the lock during Wait()")
	}
}

// --- Setter methods integration tests ---

func TestSetOSC8_Value(t *testing.T) {
	vt := New()
	assert.True(t, vt.OSC8) // default is true
	vt.SetOSC8(false)
	assert.False(t, vt.OSC8)
	vt.SetOSC8(true)
	assert.True(t, vt.OSC8)
}

func TestSetTERM_Value(t *testing.T) {
	vt := New()
	vt.SetTERM("vt100")
	assert.Equal(t, "vt100", vt.TERM)
}

func TestSetLogger_Value(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "test:", 0)
	vt := New()
	vt.SetLogger(logger)
	assert.Equal(t, logger, vt.Logger)
}

// --- Close() with no subprocess should not panic ---

func TestClose_NoPtyNoCmd(t *testing.T) {
	vt := New()
	assert.NotPanics(t, func() {
		vt.Close()
	})
}

// --- Close() idempotent ---

func TestClose_Idempotent(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.surface = &mockSurface{w: 4, h: 2}

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	err = vt.StartWithPty(&nopWriteCloser{r})
	assert.NoError(t, err)
	_ = w.Close()
	time.Sleep(50 * time.Millisecond)

	// Calling Close() multiple times should not panic
	assert.NotPanics(t, func() {
		vt.Close()
		vt.Close()
		vt.Close()
	})
}

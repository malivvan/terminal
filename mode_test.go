package terminal

import (
	"io"
	"os"
	"testing"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureSurface is a test Screen that captures SetContent calls.
type captureSurface struct {
	w, h  int
	cells [][]capturedCell
}

type capturedCell struct {
	ch    rune
	comb  []rune
	style tcell.Style
	set   bool
}

func (s *captureSurface) SetContent(x, y int, ch rune, comb []rune, style tcell.Style) {
	if s.cells == nil {
		s.cells = make([][]capturedCell, s.h)
		for i := range s.cells {
			s.cells[i] = make([]capturedCell, s.w)
		}
	}
	if y >= 0 && y < s.h && x >= 0 && x < s.w {
		s.cells[y][x] = capturedCell{ch: ch, comb: comb, style: style, set: true}
	}
}

func (s *captureSurface) Size() (int, int) {
	return s.w, s.h
}

func TestCUPDefaultsAndOriginMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	vt.cursor.row = 4
	vt.cursor.col = 9
	vt.cup([]int{0, 0})
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)

	vt.margin.top = 1
	vt.margin.bottom = 3
	vt.mode |= decom
	vt.cup([]int{1, 1})
	assert.Equal(t, row(1), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)

	vt.cup([]int{999, 999})
	assert.Equal(t, row(3), vt.cursor.row)
	assert.Equal(t, column(9), vt.cursor.col)
}

func TestDECSTBMDefaultsAndHome(t *testing.T) {
	vt := New()
	vt.Resize(10, 4)

	vt.cursor.row = 3
	vt.cursor.col = 7
	vt.decstbm([]int{0, 3})
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, row(2), vt.margin.bottom)
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)

	vt.mode |= decom
	vt.cursor.row = 0
	vt.cursor.col = 9
	vt.decstbm([]int{2, 4})
	assert.Equal(t, row(1), vt.margin.top)
	assert.Equal(t, row(3), vt.margin.bottom)
	assert.Equal(t, row(1), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestPrivateDSRResponses(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.margin.top = 1
	vt.margin.bottom = 3
	vt.mode |= decom
	vt.cursor.row = 2
	vt.cursor.col = 4

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	defer func() {
		_ = r.Close()
	}()
	vt.pty = w

	vt.csi("?n", []int{5})
	vt.csi("?n", []int{6})
	assert.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	assert.Equal(t, "\x1b[?0n\x1b[?2;5R", string(out))
}

func TestDECOMHomesCursor(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.margin.top = 2
	vt.margin.bottom = 4
	vt.cursor.row = 4
	vt.cursor.col = 8

	vt.decset([]int{6})
	assert.Equal(t, row(2), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)

	vt.cursor.row = 4
	vt.cursor.col = 8
	vt.decrst([]int{6})
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestAlternateScreenModeVariants(t *testing.T) {
	t.Run("47 and 1047 switch screens without clobbering primary", func(t *testing.T) {
		vt := New()
		vt.Resize(2, 1)
		vt.print('p')

		vt.decset([]int{47})
		vt.cursor.row = 0
		vt.cursor.col = 0
		vt.print('a')
		vt.decrst([]int{47})
		assert.Equal(t, "p ", vt.String())

		vt.decset([]int{1047})
		vt.cursor.row = 0
		vt.cursor.col = 0
		vt.print('b')
		vt.decrst([]int{1047})
		assert.Equal(t, "p ", vt.String())
	})

	t.Run("1048 saves and restores cursor", func(t *testing.T) {
		vt := New()
		vt.Resize(2, 2)
		vt.cursor.row = 1
		vt.cursor.col = 1

		vt.decset([]int{1048})
		vt.cursor.row = 0
		vt.cursor.col = 0
		vt.decrst([]int{1048})

		assert.Equal(t, row(1), vt.cursor.row)
		assert.Equal(t, column(1), vt.cursor.col)
	})
}

// TERMINAL-F18: DECSET/DECRST 1004 (Send FocusIn/FocusOut events) is not handled.
// Many modern TUI applications (neovim, tmux, etc.) enable this mode to detect
// when the terminal gains or loses focus. Without it, these applications cannot
// react to focus changes.
func TestFocusEventMode_DECSET1004(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Focus events mode should be off by default
	assert.Equal(t, mode(0), vt.mode&focusEvents)

	// DECSET 1004 should enable focus event reporting
	vt.decset([]int{1004})
	assert.NotEqual(t, mode(0), vt.mode&focusEvents)

	// DECRST 1004 should disable focus event reporting
	vt.decrst([]int{1004})
	assert.Equal(t, mode(0), vt.mode&focusEvents)
}

// TERMINAL-F18 regression: enabling/disabling 1004 should not affect other modes.
func TestFocusEventMode_DoesNotAffectOtherModes(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Enable paste mode and mouse
	vt.decset([]int{2004})
	vt.decset([]int{1000})

	// Enable then disable focus events
	vt.decset([]int{1004})
	vt.decrst([]int{1004})

	// Other modes should be unaffected
	assert.NotEqual(t, mode(0), vt.mode&paste)
	assert.NotEqual(t, mode(0), vt.mode&mouseButtons)
}

// TERMINAL-F19: DECSET/DECRST 2026 (synchronized output) is not handled. Modern
// terminals use this to batch screen updates and prevent tearing. Applications
// that enable it will not see the expected synchronization barrier.
func TestSyncOutputMode_DECSET2026(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Sync output mode should be off by default
	assert.Equal(t, mode(0), vt.mode&syncOutput)

	// DECSET 2026 should enable synchronized output
	vt.decset([]int{2026})
	assert.NotEqual(t, mode(0), vt.mode&syncOutput)

	// DECRST 2026 should disable synchronized output
	vt.decrst([]int{2026})
	assert.Equal(t, mode(0), vt.mode&syncOutput)
}

// TERMINAL-F19 regression: enabling/disabling 2026 should not affect other modes.
func TestSyncOutputMode_DoesNotAffectOtherModes(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{2004})
	vt.decset([]int{2026})
	vt.decrst([]int{2026})

	assert.NotEqual(t, mode(0), vt.mode&paste)
}

// TERMINAL-F20: DECSET/DECRST 5 (DECSCNM) is recognized in mode.go but has no
// rendering effect. Applications that enable reverse video mode will not see
// the screen colors inverted.
func TestDECSCNM_ReverseVideo(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// DECSCNM should be off by default
	assert.Equal(t, mode(0), vt.mode&decscnm)

	// DECSET 5 should enable reverse video mode
	vt.decset([]int{5})
	assert.NotEqual(t, mode(0), vt.mode&decscnm)

	// DECRST 5 should disable reverse video mode
	vt.decrst([]int{5})
	assert.Equal(t, mode(0), vt.mode&decscnm)
}

// TERMINAL-F20 regression: DECSCNM Draw() should invert fg/bg on all cells when set.
func TestDECSCNM_DrawInvertsCells(t *testing.T) {
	vt := New()
	srf := &captureSurface{w: 4, h: 1}
	vt.SetSurface(srf)
	vt.Resize(4, 1)
	vt.mode = 0

	// Print a character with known foreground/background
	fg := tcellcolor.Red
	bg := tcellcolor.Blue
	vt.cursor.attrs = tcell.StyleDefault.Foreground(fg).Background(bg)
	vt.print('A')

	// Enable DECSCNM (reverse video)
	vt.mode |= decscnm

	vt.Draw()

	// When DECSCNM is on, Draw() should invert the style (apply Reverse)
	c := srf.cells[0][0]
	assert.True(t, c.set)
	// The style should have Reverse applied
	assert.True(t, c.style.HasReverse())
}

// TERMINAL-F24: DECSET 9 (X10 mouse compatibility) and DECSET 1005 (UTF-8 mouse
// encoding) are not handled. While SGR mouse (1006) is the modern standard,
// some legacy applications still use these older protocols.
func TestX10MouseMode_DECSET9(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// X10 mouse mode should be off by default
	assert.Equal(t, mode(0), vt.mode&mouseX10)

	// DECSET 9 should enable X10 mouse mode
	vt.decset([]int{9})
	assert.NotEqual(t, mode(0), vt.mode&mouseX10)

	// DECRST 9 should disable X10 mouse mode
	vt.decrst([]int{9})
	assert.Equal(t, mode(0), vt.mode&mouseX10)
}

func TestUTF8MouseMode_DECSET1005(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// UTF-8 mouse encoding should be off by default
	assert.Equal(t, mode(0), vt.mode&mouseUTF8)

	// DECSET 1005 should enable UTF-8 mouse encoding
	vt.decset([]int{1005})
	assert.NotEqual(t, mode(0), vt.mode&mouseUTF8)

	// DECRST 1005 should disable UTF-8 mouse encoding
	vt.decrst([]int{1005})
	assert.Equal(t, mode(0), vt.mode&mouseUTF8)
}

// TERMINAL-F24 regression: enabling mouse mode 9 should not interfere with SGR mouse (1006).
func TestMouseModes_IndependentFlags(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{9})
	vt.decset([]int{1005})
	vt.decset([]int{1006})

	assert.NotEqual(t, mode(0), vt.mode&mouseX10)
	assert.NotEqual(t, mode(0), vt.mode&mouseUTF8)
	assert.NotEqual(t, mode(0), vt.mode&mouseSGR)

	// Disabling one should not affect the others
	vt.decrst([]int{9})
	assert.Equal(t, mode(0), vt.mode&mouseX10)
	assert.NotEqual(t, mode(0), vt.mode&mouseUTF8)
	assert.NotEqual(t, mode(0), vt.mode&mouseSGR)
}

func TestSM_RM_AllANSI(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Initially kam, irm, srm, lnm should all be clear
	assert.Equal(t, mode(0), vt.mode&(kam|irm|srm|lnm))

	// Set kam (2), irm (4), srm (12), lnm (20)
	vt.sm([]int{2, 4, 12, 20})
	assert.True(t, vt.mode&kam != 0, "kam should be set")
	assert.True(t, vt.mode&irm != 0, "irm should be set")
	assert.True(t, vt.mode&srm != 0, "srm should be set")
	assert.True(t, vt.mode&lnm != 0, "lnm should be set")

	// Clear them
	vt.rm([]int{2, 4, 12, 20})
	assert.True(t, vt.mode&kam == 0, "kam should be cleared")
	assert.True(t, vt.mode&irm == 0, "irm should be cleared")
	assert.True(t, vt.mode&srm == 0, "srm should be cleared")
	assert.True(t, vt.mode&lnm == 0, "lnm should be cleared")
}

func TestSM_SingleParam(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.sm([]int{4})
	assert.True(t, vt.mode&irm != 0, "irm should be set")

	vt.rm([]int{4})
	assert.True(t, vt.mode&irm == 0, "irm should be cleared")
}

func TestDECSET_DECRST_Uncommon(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)

	// DECSET 2 (decanm)
	vt.decset([]int{2})
	assert.True(t, vt.mode&decanm != 0)
	vt.decrst([]int{2})
	assert.True(t, vt.mode&decanm == 0)

	// DECSET 4 (decsclm)
	vt.decset([]int{4})
	assert.True(t, vt.mode&decsclm != 0)
	vt.decrst([]int{4})
	assert.True(t, vt.mode&decsclm == 0)

	// DECSET 8 (decarm)
	vt.decset([]int{8})
	assert.True(t, vt.mode&decarm != 0)
	vt.decrst([]int{8})
	assert.True(t, vt.mode&decarm == 0)

	// DECSET 1 (decckm)
	vt.decset([]int{1})
	assert.True(t, vt.mode&decckm != 0)
	vt.decrst([]int{1})
	assert.True(t, vt.mode&decckm == 0)
}

func TestDECSET_AltScreen(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Enter alt screen (DECSET 1049 with cursor save)
	vt.decset([]int{1049})
	assert.True(t, vt.mode&smcup != 0,
		"smcup should be set after DECSET 1049")

	// Exit alt screen (DECRST 1049 with cursor restore)
	vt.decrst([]int{1049})
	assert.True(t, vt.mode&smcup == 0,
		"smcup should be cleared after DECRST 1049")
}

func TestDECSET_FocusEvents(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{1004})
	assert.True(t, vt.mode&focusEvents != 0)

	vt.decrst([]int{1004})
	assert.True(t, vt.mode&focusEvents == 0)
}

func TestDECSET_SyncOutput(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{2026})
	assert.True(t, vt.mode&syncOutput != 0)

	vt.decrst([]int{2026})
	assert.True(t, vt.mode&syncOutput == 0)
}

func TestDECSET_BracketedPaste(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{2004})
	assert.True(t, vt.mode&paste != 0)

	vt.decrst([]int{2004})
	assert.True(t, vt.mode&paste == 0)
}

func TestDECSET_DECRST_ReverseVideo(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{5})
	assert.True(t, vt.mode&decscnm != 0)

	vt.decrst([]int{5})
	assert.True(t, vt.mode&decscnm == 0)
}

func TestDECSET_3_DECCOLM(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	vt.mode &^= deccolm

	vt.decset([]int{3})

	assert.True(t, vt.mode&deccolm != 0)
	// Should resize to 132 columns
	assert.Equal(t, 132, vt.width())
}

func TestDECRST_3_DECCOLM(t *testing.T) {
	vt := New()
	vt.Resize(132, 24)
	vt.mode |= deccolm

	vt.decrst([]int{3})

	assert.True(t, vt.mode&deccolm == 0)
	// Should resize to 80 columns
	assert.Equal(t, 80, vt.width())
}

func TestDECSET_6_DECOM(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.margin.top = 1
	vt.margin.left = 2
	vt.cursor.row = 3
	vt.cursor.col = 5

	vt.decset([]int{6})

	// DECOM set should move cursor to margin.top/margin.left
	assert.True(t, vt.mode&decom != 0)
	assert.Equal(t, row(1), vt.cursor.row)
	assert.Equal(t, column(2), vt.cursor.col)
}

func TestDECRST_6_DECOM(t *testing.T) {
	vt := New()
	vt.Resize(8, 4)
	vt.mode |= decom
	vt.cursor.row = 2
	vt.cursor.col = 3

	vt.decrst([]int{6})

	// DECOM reset should move cursor to (0,0)
	assert.True(t, vt.mode&decom == 0)
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestDECSET_7_DECCAWM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.lastCol = true

	vt.decset([]int{7})

	assert.True(t, vt.mode&decawm != 0)
	assert.False(t, vt.lastCol, "lastCol should be reset")
}

func TestDECSET_1007_AltScroll(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{1007})

	assert.True(t, vt.mode&altScroll != 0)
}

func TestDECRST_1007_AltScroll(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= altScroll

	vt.decrst([]int{1007})

	assert.True(t, vt.mode&altScroll == 0)
}

func TestDECSET_9_MouseX10(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{9})
	assert.True(t, vt.mode&mouseX10 != 0)

	vt.decrst([]int{9})
	assert.True(t, vt.mode&mouseX10 == 0)
}

func TestDECSET_1005_MouseUTF8(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{1005})
	assert.True(t, vt.mode&mouseUTF8 != 0)

	vt.decrst([]int{1005})
	assert.True(t, vt.mode&mouseUTF8 == 0)
}

func TestDECSET_25_DECTCEM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// dectcem is set by default, so first reset it
	vt.mode &^= dectcem
	assert.True(t, vt.mode&dectcem == 0)

	vt.decset([]int{25})
	assert.True(t, vt.mode&dectcem != 0)

	vt.decrst([]int{25})
	assert.True(t, vt.mode&dectcem == 0)
}

func TestDECSET_1048(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.cursor.col = 2
	vt.cursor.row = 1

	// Save cursor (DECSET 1048)
	vt.decset([]int{1048})

	vt.cursor.col = 0
	vt.cursor.row = 0

	// Restore cursor (DECRST 1048)
	vt.decrst([]int{1048})

	assert.Equal(t, column(2), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

// TestDECRST_7_DECCAWM resets autowrap mode.
func TestDECRST_7_DECCAWM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= decawm
	vt.lastCol = true

	vt.decrst([]int{7})

	assert.True(t, vt.mode&decawm == 0, "decawm should be cleared")
	assert.False(t, vt.lastCol, "lastCol should be reset")
}

// TestDECRST_1000_1002_1003 resets mouse modes.
func TestDECRST_1000_1002_1003(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= mouseButtons | mouseDrag | mouseMotion

	vt.decrst([]int{1000})
	assert.True(t, vt.mode&mouseButtons == 0, "mouseButtons should be cleared")

	vt.mode |= mouseDrag
	vt.decrst([]int{1002})
	assert.True(t, vt.mode&mouseDrag == 0, "mouseDrag should be cleared")

	vt.mode |= mouseMotion
	vt.decrst([]int{1003})
	assert.True(t, vt.mode&mouseMotion == 0, "mouseMotion should be cleared")
}

// TestDECRST_1005_1006_1007 resets mouse UTF8/SGR/altScroll modes.
func TestDECRST_1005_1006_1007(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= mouseUTF8 | mouseSGR | altScroll

	vt.decrst([]int{1005})
	assert.True(t, vt.mode&mouseUTF8 == 0, "mouseUTF8 should be cleared")

	vt.decrst([]int{1006})
	assert.True(t, vt.mode&mouseSGR == 0, "mouseSGR should be cleared")

	vt.decrst([]int{1007})
	assert.True(t, vt.mode&altScroll == 0, "altScroll should be cleared")
}

// TestDECRST_1048 resets cursor save/restore.
func TestDECRST_1048_CursorRestore(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.cursor.col = 2
	vt.cursor.row = 1

	// Save cursor via DECSET 1048
	vt.decset([]int{1048})

	vt.cursor.col = 0
	vt.cursor.row = 0

	// Restore cursor via DECRST 1048
	vt.decrst([]int{1048})

	assert.Equal(t, column(2), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

// TestDECRST_2026_SyncOutput resets synchronized output mode.
func TestDECRST_2026_SyncOutput(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode |= syncOutput
	vt.dirty = true

	vt.decrst([]int{2026})

	assert.True(t, vt.mode&syncOutput == 0, "syncOutput should be cleared")
	assert.False(t, vt.dirty, "dirty should be reset after syncOutput reset")
}

// TestDECSET_1005_1006_1007 enables mouse protocol modes.
func TestDECSET_1005_1006_1007(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.decset([]int{1005})
	assert.True(t, vt.mode&mouseUTF8 != 0, "mouseUTF8 should be set")

	vt.decset([]int{1006})
	assert.True(t, vt.mode&mouseSGR != 0, "mouseSGR should be set")

	vt.decset([]int{1007})
	assert.True(t, vt.mode&altScroll != 0, "altScroll should be set")
}

func TestGetModes_Defaults(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	m := vt.GetModes()
	assert.Equal(t, MouseModeNone, m.MouseMode)
	assert.True(t, m.Autowrap)
	assert.False(t, m.BracketedPaste)
	assert.False(t, m.ReverseVideo)
	assert.False(t, m.FocusEvents)
	assert.True(t, m.CursorVisible)
	assert.False(t, m.AltScreen)
	assert.False(t, m.CursorKeyMode)
	assert.False(t, m.InsertMode)
	assert.False(t, m.OriginMode)
	assert.False(t, m.SynchronizedOutput)
}

func TestGetModes_MouseModes(t *testing.T) {
	h, err := NewTerminal(80, 24)
	require.NoError(t, err)
	defer h.Close()

	assert.Equal(t, MouseModeNone, h.GetModes().MouseMode)

	h.FeedString("\x1b[?9h") // X10
	assert.Equal(t, MouseModeX10, h.GetModes().MouseMode)

	h.FeedString("\x1b[?9l\x1b[?1000h") // VT200 buttons
	assert.Equal(t, MouseModeVT200, h.GetModes().MouseMode)

	h.FeedString("\x1b[?1002h") // VT200 drag
	assert.Equal(t, MouseModeVT200Drag, h.GetModes().MouseMode)

	h.FeedString("\x1b[?1003h") // VT200 motion
	assert.Equal(t, MouseModeVT200Motion, h.GetModes().MouseMode)

	h.FeedString("\x1b[?1006h") // SGR
	assert.Equal(t, MouseModeSGR, h.GetModes().MouseMode)

	h.FeedString("\x1b[?1005h") // UTF-8 (takes precedence over SGR? mode.go says URXVT > SGR > UTF8...)
	// UTF8 is checked before SGR in our helper. Let me verify ordering.
	assert.Equal(t, MouseModeSGR, h.GetModes().MouseMode, "SGR still active")

	h.FeedString("\x1b[?1006l") // disable SGR, UTF8 still active
	assert.Equal(t, MouseModeUTF8, h.GetModes().MouseMode)
}

func TestGetModes_BracketedPaste(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	assert.False(t, h.GetModes().BracketedPaste)
	h.FeedString("\x1b[?2004h")
	assert.True(t, h.GetModes().BracketedPaste)
}

func TestGetModes_Other(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	h.FeedString("\x1b[?7l") // disable autowrap
	assert.False(t, h.GetModes().Autowrap)
	h.FeedString("\x1b[?5h") // reverse video
	assert.True(t, h.GetModes().ReverseVideo)
	h.FeedString("\x1b[?1004h") // focus events
	assert.True(t, h.GetModes().FocusEvents)
}

func TestMouseTrackingMode_String(t *testing.T) {
	assert.Equal(t, "none", MouseModeNone.String())
	assert.Equal(t, "x10", MouseModeX10.String())
	assert.Equal(t, "vt200", MouseModeVT200.String())
	assert.Equal(t, "vt200-drag", MouseModeVT200Drag.String())
	assert.Equal(t, "vt200-motion", MouseModeVT200Motion.String())
	assert.Equal(t, "sgr", MouseModeSGR.String())
	assert.Equal(t, "utf8", MouseModeUTF8.String())
	assert.Equal(t, "urxvt", MouseModeURXVT.String())
	assert.Equal(t, "unknown", TrackingMode(99).String())
}

func TestMouseFlagsEqual(t *testing.T) {
	// Equal slices
	a := []tcell.MouseFlags{tcell.MouseButtonEvents, tcell.MouseDragEvents}
	b := []tcell.MouseFlags{tcell.MouseButtonEvents, tcell.MouseDragEvents}
	assert.True(t, mouseFlagsEqual(a, b))

	// Unequal length
	c := []tcell.MouseFlags{tcell.MouseButtonEvents}
	assert.False(t, mouseFlagsEqual(a, c))

	// Different values
	d := []tcell.MouseFlags{tcell.MouseMotionEvents, tcell.MouseDragEvents}
	assert.False(t, mouseFlagsEqual(a, d))

	// Both nil
	assert.True(t, mouseFlagsEqual(nil, nil))

	// One nil
	assert.False(t, mouseFlagsEqual(a, nil))
	assert.False(t, mouseFlagsEqual(nil, a))
}

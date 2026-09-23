package terminal

import (
	"testing"

	"bytes"
	"github.com/stretchr/testify/assert"
)

func TestBS(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.cursor.col = 1
	vt.bs()
	assert.Equal(t, column(0), vt.cursor.col)
	vt.bs()
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestLF(t *testing.T) {
	t.Run("LNM reset", func(t *testing.T) {
		vt := New()
		vt.Resize(2, 2)
		vt.print('v')
		vt.print('t')
		assert.Equal(t, "vt\n  ", vt.String())
		vt.lf()
		assert.Equal(t, "vt\n  ", vt.String())
		assert.Equal(t, column(1), vt.cursor.col)
		assert.Equal(t, row(1), vt.cursor.row)
	})

	t.Run("LNM set", func(t *testing.T) {
		vt := New()
		vt.Resize(2, 2)
		vt.print('v')
		vt.print('t')
		assert.Equal(t, "vt\n  ", vt.String())
		vt.mode |= lnm
		vt.lf()
		assert.Equal(t, "vt\n  ", vt.String())
		assert.Equal(t, column(0), vt.cursor.col)
		assert.Equal(t, row(1), vt.cursor.row)

		vt.print('x')
		vt.lf()
		assert.Equal(t, "x \n  ", vt.String())
		assert.Equal(t, column(0), vt.cursor.col)
		assert.Equal(t, row(1), vt.cursor.row)
	})
}

// TERMINAL-001: SI (0x0F) should invoke G0 into GL, not G2.
func TestSI_SelectsG0(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	// SO selects G1
	vt.c0(0x0E)
	assert.Equal(t, charsetDesignator(g1), vt.charsets.selected)
	// SI should select G0
	vt.c0(0x0F)
	assert.Equal(t, charsetDesignator(g0), vt.charsets.selected)
}

// TERMINAL-008: Backspace reverse-wrap should only happen when DECAWM is set.
func TestBS_NoReverseWrapWithoutDECAWM(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	// Disable DECAWM
	vt.mode &^= decawm
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.bs()
	// Without DECAWM, cursor should stay at col 0, row 1 (no reverse wrap)
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestBS_ReverseWrapWithDECAWM(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	// Enable DECAWM (already default, but be explicit)
	vt.mode |= decawm
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.bs()
	// With DECAWM, cursor should reverse-wrap to end of previous line
	assert.Equal(t, column(3), vt.cursor.col)
	assert.Equal(t, row(0), vt.cursor.row)
}

// // Linefeed 0x10
// func (vt *vt) LF() {
// 	switch {
// 	case vt.cursor.row == vt.margin.bottom:
// 		vt.ScrollUp(1)
// 	default:
// 		vt.cursor.row += 1
// 	}
//
// 	if vt.mode&LNM != LNM {
// 		return
// 	}
// 	vt.cursor.col = vt.margin.left
// }

// ENQ (0x05) — Answerback writes an empty string to pty.
func TestENQ(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	vt.c0(0x05)
	// ENQ writes "" to pty, so buf should be empty
	assert.Empty(t, buf.String())
}

// mockPtyCloser implements io.ReadWriteCloser for testing.
type mockPtyCloser struct {
	*bytes.Buffer
}

func (m *mockPtyCloser) Close() error { return nil }

func (m *mockPtyCloser) Read(p []byte) (int, error) {
	return 0, nil
}

// CR (0x0D) — Carriage return resets cursor to left margin.
func TestCR(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.margin.left = 0
	vt.cursor.col = 3

	vt.cr()
	assert.Equal(t, column(0), vt.cursor.col)
}

// CR with custom left margin.
func TestCR_WithLeftMargin(t *testing.T) {
	vt := New()
	vt.Resize(8, 1)
	vt.margin.left = 2
	vt.cursor.col = 7

	vt.cr()
	assert.Equal(t, column(2), vt.cursor.col)
}

// FF (0x0C) acts like LF — cursor row increments.
func TestFF(t *testing.T) {
	t.Run("FF acts like LF", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 2)
		vt.cursor.row = 0

		vt.ff()
		assert.Equal(t, row(1), vt.cursor.row)
	})

	t.Run("FF at bottom margin scrolls", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 2)
		vt.cursor.row = 1 // bottom margin is row(1) by default

		vt.ff()
		// After scrolling, cursor row should still be at bottom
		assert.Equal(t, row(1), vt.cursor.row)
	})
}

// Session (0x0B) also acts like LF.
func TestVT_Control(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.cursor.row = 0

	vt.c0(0x0B)
	assert.Equal(t, row(1), vt.cursor.row,
		"Session (0x0B) should act like LF and increment row")
}

// SO (0x0E) and SI (0x0F) charset switching.
func TestSO_SI_CharsetSwitch(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Initially G0
	assert.Equal(t, charsetDesignator(g0), vt.charsets.selected)

	// SO selects G1
	vt.c0(0x0E)
	assert.Equal(t, charsetDesignator(g1), vt.charsets.selected)

	// SI selects G0
	vt.c0(0x0F)
	assert.Equal(t, charsetDesignator(g0), vt.charsets.selected)
}

// Bell (0x07) posts EventBell.
func TestBEL(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.c0(0x07)

	select {
	case ev := <-vt.events:
		_, ok := ev.(EventBell)
		assert.True(t, ok, "expected EventBell, got %T", ev)
	default:
		t.Error("expected EventBell event after BEL")
	}
}

// BS (0x08) — Reverse wrap: cursor at left margin with DECAWM set.
func TestBS_ReverseWrap(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode |= decawm
	vt.margin.left = 0
	vt.margin.right = 3
	vt.cursor.col = 0
	vt.cursor.row = 1

	vt.bs()

	// Should reverse-wrap to previous line right margin
	assert.Equal(t, column(3), vt.cursor.col)
	assert.Equal(t, row(0), vt.cursor.row)
}

// BS (0x08) — No reverse wrap when DECAWM is cleared.
func TestBS_NoReverseWrap(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode &^= decawm
	vt.cursor.col = 0
	vt.cursor.row = 1

	vt.bs()

	// Should NOT move (DECAWM is cleared)
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

// BS (0x08) — At top-left corner of margins, no movement.
func TestBS_AtTopLeft(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.margin.top = 0
	vt.margin.left = 0
	vt.cursor.col = 0
	vt.cursor.row = 0

	vt.bs()

	// Should stay at (0,0) — no panic, no movement
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, row(0), vt.cursor.row)
}

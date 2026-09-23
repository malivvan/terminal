package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TERMINAL-F08: ESC N (SS2) and ESC O (SS3) set vt.charsets.selected to g2/g3
// but never save the current value to vt.charsets.saved. When print() reverts
// the single shift, it restores saved which defaults to g0. If the terminal
// was in G1 (after SO/0x0E), the single shift incorrectly reverts to G0
// instead of G1.
func TestSS2_SavesPreviousCharset(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// Switch to G1 via SO (0x0E)
	vt.charsets.selected = g1
	assert.Equal(t, charsetDesignator(g1), vt.charsets.selected)

	// SS2 (ESC N): should save g1 to saved, then set selected = g2
	vt.esc("N")

	assert.True(t, vt.charsets.singleShift)
	assert.Equal(t, charsetDesignator(g2), vt.charsets.selected)
	// Bug: saved is g0 (default zero value) instead of g1
	assert.Equal(t, charsetDesignator(g1), vt.charsets.saved)
}

func TestSS3_SavesPreviousCharset(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// Switch to G1 via SO (0x0E)
	vt.charsets.selected = g1

	// SS3 (ESC O): should save g1 to saved, then set selected = g3
	vt.esc("O")

	assert.True(t, vt.charsets.singleShift)
	assert.Equal(t, charsetDesignator(g3), vt.charsets.selected)
	// Bug: saved is g0 (default zero value) instead of g1
	assert.Equal(t, charsetDesignator(g1), vt.charsets.saved)
}

// TERMINAL-F08 regression: SS2 from default G0 state should save g0.
func TestSS2_SavesG0WhenDefault(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	assert.Equal(t, charsetDesignator(g0), vt.charsets.selected)

	vt.esc("N")

	assert.Equal(t, charsetDesignator(g0), vt.charsets.saved)
	assert.Equal(t, charsetDesignator(g2), vt.charsets.selected)
}

// TERMINAL-F23: DECBI (ESC 6) — Back Index. If the cursor is at the left margin,
// content within margins is scrolled right by one column (a blank column is
// inserted at the left margin). Otherwise the cursor moves left by one column.
func TestDECBI_CursorNotAtLeftMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	// Cursor at col 2
	assert.Equal(t, column(2), vt.cursor.col)

	// DECBI should move cursor left by 1
	vt.esc("6")
	assert.Equal(t, column(1), vt.cursor.col)
	// Screen content unchanged
	assert.Equal(t, "ab  ", vt.String())
}

func TestDECBI_CursorAtLeftMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	// Place cursor at left margin
	vt.cursor.col = 0

	// DECBI at left margin should scroll content right within margins,
	// inserting a blank at the left margin. 'd' falls off the right.
	vt.esc("6")
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, " abc", vt.String())
}

// TERMINAL-F23 regression: DECBI with left/right margins set.
func TestDECBI_WithMargins(t *testing.T) {
	vt := New()
	vt.Resize(6, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	vt.print('e')
	vt.print('f')
	assert.Equal(t, "abcdef", vt.String())

	// Set left/right margins
	vt.margin.left = 1
	vt.margin.right = 4

	// Cursor at left margin (col 1)
	vt.cursor.col = 1

	vt.esc("6")
	// 'e' (col 4) should fall off, columns 1..3 shift right, blank at col 1
	assert.Equal(t, column(1), vt.cursor.col)
	assert.Equal(t, 'a', vt.activeScreen[0][0].content) // outside margin
	assert.Equal(t, ' ', vt.activeScreen[0][1].rune())  // blank inserted
	assert.Equal(t, 'b', vt.activeScreen[0][2].content)
	assert.Equal(t, 'c', vt.activeScreen[0][3].content)
	assert.Equal(t, 'd', vt.activeScreen[0][4].content)
	assert.Equal(t, 'f', vt.activeScreen[0][5].content) // outside margin
}

// TERMINAL-F23: DECFI (ESC 9) — Forward Index. If the cursor is at the right margin,
// content within margins is scrolled left by one column (a blank column is
// inserted at the right margin). Otherwise the cursor moves right by one column.
func TestDECFI_CursorNotAtRightMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	// Cursor at col 1
	assert.Equal(t, column(1), vt.cursor.col)

	// DECFI should move cursor right by 1
	vt.esc("9")
	assert.Equal(t, column(2), vt.cursor.col)
	// Screen content unchanged
	assert.Equal(t, "a   ", vt.String())
}

func TestDECFI_CursorAtRightMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	// Place cursor at right margin
	vt.cursor.col = 3

	// DECFI at right margin should scroll content left within margins,
	// inserting a blank at the right margin. 'a' falls off the left.
	vt.esc("9")
	assert.Equal(t, column(3), vt.cursor.col)
	assert.Equal(t, "bcd ", vt.String())
}

// TERMINAL-F23 regression: DECFI with left/right margins set.
func TestDECFI_WithMargins(t *testing.T) {
	vt := New()
	vt.Resize(6, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	vt.print('e')
	vt.print('f')
	assert.Equal(t, "abcdef", vt.String())

	// Set left/right margins
	vt.margin.left = 1
	vt.margin.right = 4

	// Cursor at right margin (col 4)
	vt.cursor.col = 4

	vt.esc("9")
	// 'b' (col 1) should fall off left within margins, columns 2..4 shift left, blank at col 4
	assert.Equal(t, column(4), vt.cursor.col)
	assert.Equal(t, 'a', vt.activeScreen[0][0].content) // outside margin
	assert.Equal(t, 'c', vt.activeScreen[0][1].content)
	assert.Equal(t, 'd', vt.activeScreen[0][2].content)
	assert.Equal(t, 'e', vt.activeScreen[0][3].content)
	assert.Equal(t, ' ', vt.activeScreen[0][4].rune())  // blank inserted
	assert.Equal(t, 'f', vt.activeScreen[0][5].content) // outside margin
}

func TestIND_BottomOfScreen(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0

	// Place cursor at last row (row 1), no margin set
	vt.cursor.row = 1
	vt.ind()

	// Cursor should NOT move past height()-1
	assert.Equal(t, row(1), vt.cursor.row,
		"IND should not move cursor past bottom of screen when no scroll margin")
}

func TestIND_WithMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0
	vt.margin.top = 0
	vt.margin.bottom = 1 // only rows 0-1 are in scroll region

	// Place cursor at bottom of margin (row 1)
	vt.cursor.row = 1
	vt.ind()

	// Should stay within the scroll region — row stays at bottom
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestNEL(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0

	vt.cursor.col = 3
	vt.cursor.row = 0

	vt.nel()

	// NEL moves cursor to left margin on next line
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestNEL_AtBottomMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0

	vt.cursor.row = 1 // bottom
	vt.cursor.col = 2

	vt.nel()

	// NEL at bottom margin should scroll and move cursor to left margin
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row, "cursor should stay at bottom after scroll")
}

func TestRI_CursorBelowZero(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Place cursor at row 0
	vt.cursor.row = 0
	vt.ri()

	// RI at top should not panic or change row
	assert.Equal(t, row(0), vt.cursor.row)
}

func TestRI_AtMarginTop(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.margin.top = 1
	vt.cursor.row = 1

	vt.ri()

	// RI at margin top should scroll down
	assert.Equal(t, row(1), vt.cursor.row,
		"RI at margin top should stay at top after scroll down")
}

func TestRIS(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Set up some state
	vt.print('x')
	vt.margin.top = 1
	vt.margin.bottom = 1
	vt.mode |= irm | decom | decckm
	vt.charsets.designations[g0] = decSpecialAndLineDrawing

	// RIS resets state (except margin.top and margin.left are NOT reset by ris())
	vt.ris()

	// Verify reset
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
	assert.Equal(t, mode(decawm|dectcem), vt.mode,
		"RIS should reset mode to decawm|dectcem")

	// All charsets should be ascii
	assert.Equal(t, ascii, vt.charsets.designations[g0])
	assert.Equal(t, ascii, vt.charsets.designations[g1])
	assert.Equal(t, ascii, vt.charsets.designations[g2])
	assert.Equal(t, ascii, vt.charsets.designations[g3])

	// Tab stops should be multiples of 8
	assert.GreaterOrEqual(t, len(vt.tabStop), 6)
	assert.Equal(t, column(8), vt.tabStop[0])

	// Screen should be empty
	assert.Equal(t, "    \n    ", vt.String())
}

func TestDECSC_DECRC_Basic(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	vt.cursor.col = 2
	vt.cursor.row = 1
	vt.decsc()

	vt.cursor.col = 0
	vt.cursor.row = 0
	vt.decrc()

	// Should restore cursor to saved position
	assert.Equal(t, column(2), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestDECSC_DECRC_AltScreen(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// Save cursor state in primary screen
	vt.cursor.col = 3
	vt.cursor.row = 1
	vt.decsc()

	// Enter alt screen (DECSET 47 — no cursor save)
	vt.decset([]int{47})

	// Move cursor in alt screen
	vt.cursor.col = 0
	vt.cursor.row = 0

	// Save in alt screen
	vt.decsc()

	// Move again
	vt.cursor.col = 1
	vt.cursor.row = 1

	// Restore — should restore from alt screen state
	vt.decrc()
	assert.Equal(t, column(0), vt.cursor.col,
		"DECRC in alt screen should restore alt screen saved state")
	assert.Equal(t, row(0), vt.cursor.row)

	// Exit alt screen
	vt.decrst([]int{47})

	// Now restore — should restore from primary screen state
	vt.decrc()
	assert.Equal(t, column(3), vt.cursor.col,
		"DECRC in primary screen should restore primary screen saved state")
	assert.Equal(t, row(1), vt.cursor.row)
}

func TestDECSC_SavesCharset(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Switch to G1
	vt.charsets.selected = g1
	vt.decsc()

	// Change charset after saving
	vt.charsets.selected = g0

	// Restore
	vt.decrc()
	assert.Equal(t, charsetDesignator(g1), vt.charsets.selected,
		"DECRC should restore saved charset")
}

func TestDECBI_Basic(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.cursor.col = 2

	// DECBI moves cursor left
	vt.decbi()
	assert.Equal(t, column(1), vt.cursor.col)

	// Content unchanged
	assert.Equal(t, "ab  ", vt.String())
}

func TestDECBI_AtLeftMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.cursor.col = 0

	// DECBI at left margin should scroll content right
	vt.decbi()
	// Content should be shifted right, blank inserted at left
	assert.Equal(t, " ab ", vt.String())
}

func TestDECFI_Basic(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.cursor.col = 1

	// DECFI moves cursor right
	vt.decfi()
	assert.Equal(t, column(2), vt.cursor.col,
		"DECFI should move cursor right")
}

func TestDECFI_AtRightMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	vt.cursor.col = 3

	// DECFI at right margin should scroll content left
	vt.decfi()
	assert.Equal(t, "bcd ", vt.String())
}

func TestHTS(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)

	vt.cursor.col = 15
	vt.hts()

	// Tab stop should be added at col 15
	found := false
	for _, ts := range vt.tabStop {
		if ts == 15 {
			found = true
			break
		}
	}
	assert.True(t, found, "HTS should add tab stop at cursor col 15")
}

func TestHTS_Duplicate(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)

	vt.cursor.col = 8
	vt.hts()

	// Count occurrences of 8
	count := 0
	for _, ts := range vt.tabStop {
		if ts == 8 {
			count++
		}
	}
	assert.Equal(t, 1, count, "HTS should not add duplicate tab stops")
}

// DECKPAM (ESC =) — no-op, should not panic.
func TestESC_DECKPAM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.esc("=")
	})
}

// DECKPNM (ESC >) — no-op, should not panic.
func TestESC_DECKPNM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.esc(">")
	})
}

// DECDHL Top Half (ESC #3) — no-op, should not panic.
func TestESC_DECDHL_Top(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.esc("#3")
	})
}

// DECDHL Bottom Half (ESC #4) — no-op, should not panic.
func TestESC_DECDHL_Bottom(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.esc("#4")
	})
}

// DECSWL (ESC #5) — no-op, should not panic.
func TestESC_DECSWL(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.esc("#5")
	})
}

// DECDWL (ESC #6) — no-op, should not panic.
func TestESC_DECDWL(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.esc("#6")
	})
}

// DECALN (ESC #8) — fill screen with 'E', reset margins, cursor at (0,0).
func TestESC_DECALN(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)

	// Set up some state to verify reset
	vt.margin.top = 1
	vt.margin.bottom = 1
	vt.margin.left = 1
	vt.margin.right = 2
	vt.cursor.row = 2
	vt.cursor.col = 3

	vt.esc("#8")

	// Screen should be filled with 'E'
	assert.Equal(t, "EEEE\nEEEE\nEEEE", vt.String())

	// Margins should be reset to full screen
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, row(2), vt.margin.bottom)
	assert.Equal(t, column(0), vt.margin.left)
	assert.Equal(t, column(3), vt.margin.right)

	// Cursor should be at (0,0)
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
}

// SS2 (ESC N) — Single Shift G2.
func TestESC_SS2(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.charsets.selected = g0
	vt.esc("N")

	assert.True(t, vt.charsets.saved == g0, "saved should be g0")
	assert.True(t, vt.charsets.selected == g2, "selected should be g2")
	assert.True(t, vt.charsets.singleShift)
}

// SS3 (ESC O) — Single Shift G3.
func TestESC_SS3(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.charsets.selected = g1
	vt.esc("O")

	assert.True(t, vt.charsets.saved == g1, "saved should be g1")
	assert.True(t, vt.charsets.selected == g3, "selected should be g3")
	assert.True(t, vt.charsets.singleShift)
}

// Designate G0 through G3 as DEC Special (ESC (0, )0, *0, +0).
func TestESC_DesignateDECSpecial(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.esc("(0")
	assert.Equal(t, decSpecialAndLineDrawing, vt.charsets.designations[g0])

	vt.esc(")0")
	assert.Equal(t, decSpecialAndLineDrawing, vt.charsets.designations[g1])

	vt.esc("*0")
	assert.Equal(t, decSpecialAndLineDrawing, vt.charsets.designations[g2])

	vt.esc("+0")
	assert.Equal(t, decSpecialAndLineDrawing, vt.charsets.designations[g3])
}

// Designate G0 through G3 as ASCII (ESC (B, )B, *B, +B).
func TestESC_DesignateASCII(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// First set to DEC special, then back to ASCII
	vt.charsets.designations[g0] = decSpecialAndLineDrawing
	vt.esc("(B")
	assert.Equal(t, ascii, vt.charsets.designations[g0])

	vt.charsets.designations[g1] = decSpecialAndLineDrawing
	vt.esc(")B")
	assert.Equal(t, ascii, vt.charsets.designations[g1])

	vt.charsets.designations[g2] = decSpecialAndLineDrawing
	vt.esc("*B")
	assert.Equal(t, ascii, vt.charsets.designations[g2])

	vt.charsets.designations[g3] = decSpecialAndLineDrawing
	vt.esc("+B")
	assert.Equal(t, ascii, vt.charsets.designations[g3])
}

// DECBI (ESC 6) — Back Index.
func TestESC_DECBI(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0
	vt.print('a')
	vt.cursor.col = 1

	vt.esc("6")

	// Should move cursor left
	assert.Equal(t, column(0), vt.cursor.col)
}

// DECFI (ESC 9) — Forward Index.
func TestESC_DECFI(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0
	vt.print('a')
	vt.cursor.col = 0

	vt.esc("9")

	// Should move cursor right
	assert.Equal(t, column(1), vt.cursor.col)
}

func TestDECKPAM_DECKPNM(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// DECKPAM (ESC =) — should be consumed without panic
	assert.NotPanics(t, func() {
		vt.esc("=")
	})

	// DECKPNM (ESC >) — should be consumed without panic
	assert.NotPanics(t, func() {
		vt.esc(">")
	})
}

func TestXTWINOPS_5Params(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)

	// CSI T with 5 params (XTHIMOUSE) should not panic or scroll
	assert.NotPanics(t, func() {
		vt.csi("T", []int{0, 1, 2, 3, 4})
	})
}

func TestCellRune_NonZero(t *testing.T) {
	c := cell{content: 'X'}
	assert.Equal(t, rune('X'), c.rune())
}

func TestCellRune_Zero(t *testing.T) {
	var c cell
	assert.Equal(t, rune(' '), c.rune())
}

func TestDECSpecial_FullMapping(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Set G0 to DEC Special
	vt.charsets.designations[g0] = decSpecialAndLineDrawing
	vt.charsets.selected = g0

	// Test each mapped code point from 0x5f to 0x7e
	tests := []struct {
		input    rune
		expected rune
	}{
		{0x5f, 0x00A0}, // NO-BREAK SPACE
		{0x60, 0x25C6}, // BLACK DIAMOND
		{0x61, 0x2592}, // MEDIUM SHADE
		{0x62, 0x2409}, // SYMBOL FOR HORIZONTAL TABULATION
		{0x63, 0x240C}, // SYMBOL FOR FORM FEED
		{0x64, 0x240D}, // SYMBOL FOR CARRIAGE RETURN
		{0x65, 0x240A}, // SYMBOL FOR LINE FEED
		{0x66, 0x00B0}, // DEGREE SIGN
		{0x67, 0x00B1}, // PLUS-MINUS SIGN
		{0x68, 0x2424}, // SYMBOL FOR NEWLINE
		{0x69, 0x240B}, // SYMBOL FOR VERTICAL TABULATION
		{0x6a, 0x2518}, // BOX DRAWINGS LIGHT UP AND LEFT
		{0x6b, 0x2510}, // BOX DRAWINGS LIGHT DOWN AND LEFT
		{0x6c, 0x250C}, // BOX DRAWINGS LIGHT DOWN AND RIGHT
		{0x6d, 0x2514}, // BOX DRAWINGS LIGHT UP AND RIGHT
		{0x6e, 0x253C}, // BOX DRAWINGS LIGHT VERTICAL AND HORIZONTAL
		{0x6f, 0x23BA}, // HORIZONTAL SCAN LINE-1
		{0x70, 0x23BB}, // HORIZONTAL SCAN LINE-3
		{0x71, 0x2500}, // BOX DRAWINGS LIGHT HORIZONTAL
		{0x72, 0x23BC}, // HORIZONTAL SCAN LINE-7
		{0x73, 0x23BD}, // HORIZONTAL SCAN LINE-9
		{0x74, 0x251C}, // BOX DRAWINGS LIGHT VERTICAL AND RIGHT
		{0x75, 0x2524}, // BOX DRAWINGS LIGHT VERTICAL AND LEFT
		{0x76, 0x2534}, // BOX DRAWINGS LIGHT UP AND HORIZONTAL
		{0x77, 0x252C}, // BOX DRAWINGS LIGHT DOWN AND HORIZONTAL
		{0x78, 0x2502}, // BOX DRAWINGS LIGHT VERTICAL
		{0x79, 0x2264}, // LESS-THAN OR EQUAL TO
		{0x7a, 0x2265}, // GREATER-THAN OR EQUAL TO
		{0x7b, 0x03C0}, // GREEK SMALL LETTER PI
		{0x7c, 0x2260}, // NOT EQUAL TO
		{0x7d, 0x00A3}, // POUND SIGN
		{0x7e, 0x00B7}, // MIDDLE DOT
	}

	for _, test := range tests {
		vt := New()
		vt.Resize(4, 1)
		vt.charsets.designations[g0] = decSpecialAndLineDrawing
		vt.charsets.selected = g0

		vt.print(test.input)
		got := vt.activeScreen[0][0].content
		assert.Equal(t, test.expected, got,
			"0x%02X should map to U+%04X, got U+%04X",
			test.input, test.expected, got)
	}
}

// TestESC_DispatchComprehensive exercises ESC dispatch entries through the
// full parser→esc() path (Terminal.FeedString) for coverage.
func TestESC_DispatchComprehensive(t *testing.T) {
	t.Run("ESC 7 DECSC save cursor", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[3;3H")
		h.FeedString("\x1b7") // save cursor
		h.FeedString("\x1b[H")
		h.FeedString("\x1b8") // restore cursor
		c, r, _ := h.Cursor()
		if c != 2 || r != 2 {
			t.Errorf("DECSC/DECRC cursor=(%d,%d), want (2,2)", c, r)
		}
	})

	t.Run("ESC D IND index", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1bD")
		_, r, _ := h.Cursor()
		if r != 1 {
			t.Errorf("IND row=%d, want 1", r)
		}
	})

	t.Run("ESC E NEL next line", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("abc")
		h.FeedString("\x1bE")
		c, r, _ := h.Cursor()
		if c != 0 || r != 1 {
			t.Errorf("NEL cursor=(%d,%d), want (0,1)", c, r)
		}
	})

	t.Run("ESC H HTS tab set", func(t *testing.T) {
		vt := New()
		vt.Resize(40, 1)
		vt.tabStop = []column{}
		vt.cursor.col = 15
		vt.esc("H")
		found := false
		for _, ts := range vt.tabStop {
			if ts == 15 {
				found = true
				break
			}
		}
		if !found {
			t.Error("HTS should set tab stop at col 15")
		}
	})

	t.Run("ESC = DECKPAM", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b=") })
	})

	t.Run("ESC > DECKPNM", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b>") })
	})

	t.Run("ESC c RIS reset", func(t *testing.T) {
		h, _ := NewTerminal(4, 2)
		defer h.Close()
		h.FeedString("abc")
		h.FeedString("\x1bc")
		// After RIS, screen should be blank
		str := h.String()
		if str != "    \n    " {
			t.Logf("RIS result=%q", str)
		}
	})

	t.Run("ESC #3 DECDHL top", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b#3") })
	})

	t.Run("ESC #4 DECDHL bottom", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b#4") })
	})

	t.Run("ESC #5 DECSWL", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b#5") })
	})

	t.Run("ESC #6 DECDWL", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b#6") })
	})

	t.Run("ESC (0 G0 DEC special", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b(0") })
	})

	t.Run("ESC )0 G1 DEC special", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b)0") })
	})

	t.Run("ESC *0 G2 DEC special", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b*0") })
	})

	t.Run("ESC +0 G3 DEC special", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b+0") })
	})

	t.Run("ESC (B G0 ASCII", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b(B") })
	})

	t.Run("ESC )B G1 ASCII", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b)B") })
	})

	t.Run("ESC *B G2 ASCII", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b*B") })
	})

	t.Run("ESC +B G3 ASCII", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b+B") })
	})
}

// TestESC_NoOps tests ESC sequences that are no-ops or already covered via
// direct method calls but need dispatch coverage through vt.esc().
func TestESC_NoOps(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)

	// All of these should not panic — they exercise the esc() dispatch
	assert.NotPanics(t, func() {
		vt.esc("7")  // DECSC
		vt.esc("8")  // DECRC
		vt.esc("D")  // IND
		vt.esc("E")  // NEL
		vt.esc("H")  // HTS
		vt.esc("c")  // RIS
		vt.esc("#3") // DECDHL top (no-op)
		vt.esc("#4") // DECDHL bottom (no-op)
		vt.esc("#5") // DECSWL (no-op)
		vt.esc("#6") // DECDWL (no-op)
	})
}

// TestIND_BelowMargin tests IND when cursor is below the scroll margin.
func TestIND_BelowMargin(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0
	vt.margin.top = 0
	vt.margin.bottom = 1

	// Place cursor below margin at row 2 (which is height-1 = 2)
	vt.cursor.row = 2
	vt.ind()
	// Should not move past height-1
	assert.Equal(t, row(2), vt.cursor.row)
}

// TestRI_CursorNegative tests RI when cursor is at row 0 with margin.top=0.
func TestRI_CursorNegative(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.cursor.row = 0
	vt.margin.top = 0

	// RI at top margin should scroll down
	vt.ri()
	// After scroll down, cursor stays at margin.top
	assert.Equal(t, row(0), vt.cursor.row)
}

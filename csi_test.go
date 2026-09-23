package terminal

import (
	"bytes"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/stretchr/testify/assert"
)

// mockPty implements io.ReadWriteCloser for capturing pty output in tests.
type mockPty struct {
	bytes.Buffer
}

func (m *mockPty) Close() error { return nil }

func TestICH(t *testing.T) {
	vt := New()
	vt.Resize(2, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	assert.Equal(t, "ab", vt.String())
	vt.cursor.col = 0
	vt.ich(0)
	assert.Equal(t, " a", vt.String())
	assert.Equal(t, column(0), vt.cursor.col)
}

// TERMINAL-F01: ICH boundary check `col+i > column(vt.width()-1)` adds cursor position
// to the loop variable (which is already an absolute column index), producing a
// meaningless value. This causes the shift loop to break prematurely when the cursor
// is not at column 0, so characters are not shifted right — only a blank is written
// at the cursor position.
func TestICH_BoundaryCheckAtNonZeroCursor(t *testing.T) {
	vt := New()
	vt.Resize(6, 1)
	vt.mode = 0

	// Fill screen: "abcdef"
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	vt.print('e')
	vt.print('f')
	assert.Equal(t, "abcdef", vt.String())

	// Place cursor at column 2 and insert 1 blank character.
	// Expected: characters at col 2..4 shift right by 1, 'f' falls off,
	// blank inserted at col 2 → "ab cde"
	vt.cursor.col = 2
	vt.ich(1)
	assert.Equal(t, "ab cde", vt.String())
	// Cursor must not move
	assert.Equal(t, column(2), vt.cursor.col)
}

// TERMINAL-F01 regression: ICH with cursor at column 0 (existing behavior should still work)
func TestICH_BoundaryCheckAtColumn0(t *testing.T) {
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

	vt.cursor.col = 0
	vt.ich(1)
	assert.Equal(t, " abcde", vt.String())
	assert.Equal(t, column(0), vt.cursor.col)
}

// TERMINAL-F01 regression: ICH inserting multiple blanks from a non-zero cursor
func TestICH_MultipleInsertAtNonZeroCursor(t *testing.T) {
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

	// Insert 2 blanks at column 1 → shift cdef right by 2, ef fall off
	// Expected: "a  bcd"
	vt.cursor.col = 1
	vt.ich(2)
	assert.Equal(t, "a  bcd", vt.String())
	assert.Equal(t, column(1), vt.cursor.col)
}

// TERMINAL-F02: ICH blank-fill loop uses `>= (vt.width() - 1)` which prevents writing
// a blank to the last screen column. The condition should be `>= vt.width()`.
func TestICH_BlankFillLastColumn(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	// Insert 3 blanks at column 0 → shifts 'a' to col 3, 'b','c','d' fall off.
	// Blanks should fill columns 0, 1, 2. But with the bug, the blank-fill
	// breaks at i=3 because int(0)+3 >= (4-1) = 3, so column 2 never gets blanked.
	vt.cursor.col = 0
	vt.ich(3)
	assert.Equal(t, "   a", vt.String())
}

// TERMINAL-F02 regression: blank-fill at last column position
func TestICH_BlankFillAtLastColumn(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	// Cursor at last column (3), insert 1 blank → 'd' falls off, blank at col 3
	vt.cursor.col = 3
	vt.ich(1)
	assert.Equal(t, "abc ", vt.String())
}

// TERMINAL-F03: ICH source validity check `(i - column(ps)) < 0` only prevents negative
// indices. It should be `(i - column(ps)) < col` to avoid copying characters from
// positions before the cursor, which violates the ICH spec (characters before the
// cursor are not affected).
func TestICH_NoCopyFromBeforeCursor(t *testing.T) {
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

	// Insert 4 blanks at column 3. Per ICH spec, only characters at col 3..5
	// are affected. Characters before cursor (a,b,c) must NOT be copied.
	// Expected: "abc   " (columns 3-5 become blanks, nothing shifts in because
	// all 3 chars at col 3..5 are pushed off the right edge by ps=4)
	vt.cursor.col = 3
	vt.ich(4)
	// With the bug, the shift loop copies from i-4 which can be 1 or 2,
	// pulling 'b' or 'c' into the post-cursor region.
	assert.Equal(t, "abc   ", vt.String())
}

// TERMINAL-F03 regression: ICH where ps exactly covers remaining columns
func TestICH_PsExactlyCoversRemaining(t *testing.T) {
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

	// Insert 3 blanks at column 3 → 3 chars at col 3..5 pushed off, all blanks
	vt.cursor.col = 3
	vt.ich(3)
	assert.Equal(t, "abc   ", vt.String())
}

// TERMINAL-F04: CUD unconditionally clamps cursor to margin.bottom. Per DEC VT510 spec,
// if cursor is already below the scroll region, CUD should stop at the last screen
// line, not the bottom margin. Same issue affects CUF and CUB.
func TestCUD_OutsideScrollRegion(t *testing.T) {
	vt := New()
	vt.Resize(4, 10)
	vt.mode = 0

	// Set scroll region to rows 2..5 (0-indexed)
	vt.margin.top = 2
	vt.margin.bottom = 5

	// Place cursor below the scroll region (row 7)
	vt.cursor.row = 7
	vt.cud(5)
	// Should clamp to last screen line (row 9), NOT to margin.bottom (row 5)
	assert.Equal(t, row(9), vt.cursor.row)
}

func TestCUD_InsideScrollRegion(t *testing.T) {
	vt := New()
	vt.Resize(4, 10)
	vt.mode = 0

	vt.margin.top = 2
	vt.margin.bottom = 5

	// Cursor inside scroll region
	vt.cursor.row = 3
	vt.cud(5)
	// Should clamp to margin.bottom (row 5)
	assert.Equal(t, row(5), vt.cursor.row)
}

func TestCUF_OutsideScrollRegion(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.mode = 0

	// Set left/right margins
	vt.margin.left = 2
	vt.margin.right = 5

	// Place cursor to the right of the scroll region (col 7)
	vt.cursor.col = 7
	vt.cuf(5)
	// Should clamp to last screen column (9), NOT margin.right (5)
	assert.Equal(t, column(9), vt.cursor.col)
}

func TestCUF_InsideScrollRegion(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.mode = 0

	vt.margin.left = 2
	vt.margin.right = 5

	// Cursor inside scroll region
	vt.cursor.col = 3
	vt.cuf(5)
	// Should clamp to margin.right (5)
	assert.Equal(t, column(5), vt.cursor.col)
}

func TestCUB_OutsideScrollRegion(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.mode = 0

	// Set left/right margins
	vt.margin.left = 2
	vt.margin.right = 5

	// Place cursor to the left of the scroll region (col 1)
	vt.cursor.col = 1
	vt.cub(5)
	// Should clamp to column 0, NOT margin.left (2)
	assert.Equal(t, column(0), vt.cursor.col)
}

func TestCUB_InsideScrollRegion(t *testing.T) {
	vt := New()
	vt.Resize(10, 1)
	vt.mode = 0

	vt.margin.left = 2
	vt.margin.right = 5

	// Cursor inside scroll region
	vt.cursor.col = 4
	vt.cub(5)
	// Should clamp to margin.left (2)
	assert.Equal(t, column(2), vt.cursor.col)
}

// TERMINAL-F05: CNL (Cursor Next Line) incorrectly scrolls at bottom margin.
// Per ECMA-48 §8.3.20, CNL should move cursor down Ps lines to column 1,
// stopping at the bottom margin WITHOUT scrolling.
func TestCNL_NoScroll(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.mode = 0

	// Fill all 4 rows
	for r := 0; r < 4; r++ {
		vt.cursor.row = row(r)
		vt.cursor.col = 0
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
	}
	assert.Equal(t, "aaaa\nbbbb\ncccc\ndddd", vt.String())

	// Place cursor at bottom margin (row 3) and CNL 5
	vt.cursor.row = 3
	vt.cursor.col = 2
	vt.cnl(5)

	// Cursor should stop at bottom margin row (3), col at left margin (0)
	assert.Equal(t, row(3), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)

	// Screen content must NOT have scrolled
	assert.Equal(t, "aaaa\nbbbb\ncccc\ndddd", vt.String())
}

// TERMINAL-F05 regression: CNL from a row above bottom margin
func TestCNL_FromAbove(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.mode = 0

	for r := 0; r < 4; r++ {
		vt.cursor.row = row(r)
		vt.cursor.col = 0
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
	}

	// CNL 2 from row 1 → should move to row 3, col 0
	vt.cursor.row = 1
	vt.cursor.col = 2
	vt.cnl(2)
	assert.Equal(t, row(3), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
	// No scrolling
	assert.Equal(t, "aaaa\nbbbb\ncccc\ndddd", vt.String())
}

// TERMINAL-F06: CPL (Cursor Preceding Line) incorrectly scrolls at top margin.
// Per ECMA-48 §8.3.13, CPL should move cursor up Ps lines to column 1,
// stopping at the top margin WITHOUT scrolling.
func TestCPL_NoScroll(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.mode = 0

	for r := 0; r < 4; r++ {
		vt.cursor.row = row(r)
		vt.cursor.col = 0
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
	}
	assert.Equal(t, "aaaa\nbbbb\ncccc\ndddd", vt.String())

	// Place cursor at top margin (row 0) and CPL 5
	vt.cursor.row = 0
	vt.cursor.col = 2
	vt.cpl(5)

	// Cursor should stop at top margin row (0), col at left margin (0)
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)

	// Screen content must NOT have scrolled
	assert.Equal(t, "aaaa\nbbbb\ncccc\ndddd", vt.String())
}

// TERMINAL-F06 regression: CPL from a row below top margin
func TestCPL_FromBelow(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.mode = 0

	for r := 0; r < 4; r++ {
		vt.cursor.row = row(r)
		vt.cursor.col = 0
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
		vt.print(rune('a' + r))
	}

	// CPL 2 from row 3 → should move to row 1, col 0
	vt.cursor.row = 3
	vt.cursor.col = 2
	vt.cpl(2)
	assert.Equal(t, row(1), vt.cursor.row)
	assert.Equal(t, column(0), vt.cursor.col)
	// No scrolling
	assert.Equal(t, "aaaa\nbbbb\ncccc\ndddd", vt.String())
}

func TestCUU(t *testing.T) {
	vt := New()
	vt.Resize(2, 2)

	vt.cursor.row = 1
	vt.cursor.col = 1
	assert.Equal(t, column(1), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
	vt.cuu(0)

	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(1), vt.cursor.col)
	vt.cuu(0)
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, column(1), vt.cursor.col)
}

func TestIL(t *testing.T) {
	vt := New()
	vt.Resize(2, 2)
	vt.print('a')
	vt.print('b')
	vt.cursor.col = 0
	vt.cursor.row = 0

	vt.il(1)
	assert.Equal(t, "  \nab", vt.String())

	vt = New()
	vt.Resize(2, 2)
	vt.print('a')
	vt.print('b')
	vt.cursor.col = 0
	vt.cursor.row = 0

	vt.il(2)
	assert.Equal(t, "  \n  ", vt.String())
}

func TestDL(t *testing.T) {
	vt := New()
	vt.Resize(2, 2)
	vt.cursor.row = 1
	vt.print('a')
	vt.print('b')
	assert.Equal(t, "  \nab", vt.String())
	vt.cursor.col = 0
	vt.cursor.row = 0

	vt.dl(1)
	assert.Equal(t, "ab\n  ", vt.String())

	vt = New()
	vt.Resize(2, 2)
	vt.cursor.row = 1
	vt.print('a')
	vt.print('b')
	assert.Equal(t, "  \nab", vt.String())
	vt.cursor.col = 0
	vt.cursor.row = 0
	vt.dl(2)
	assert.Equal(t, "  \n  ", vt.String())
}

func TestDCH(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	vt.dch(1)
	assert.Equal(t, "abc ", vt.String())
	vt.dch(2)
	assert.Equal(t, "abc ", vt.String())
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())
	vt.cursor.col = 1
	vt.dch(2)
	assert.Equal(t, "ad  ", vt.String())
}

// TERMINAL-006: ECH should erase the character at the last column when cursor is there.
func TestECH_LastColumn(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())
	// Move cursor to last column (index 3)
	vt.cursor.col = 3
	vt.ech(1)
	assert.Equal(t, "abc ", vt.String())
}

// TERMINAL-009: CHT should not count a tab stop at the current cursor position.
func TestCHT_AtTabStop(t *testing.T) {
	vt := New()
	vt.Resize(80, 1)
	// Default tab stops at 8, 16, 24, ...
	// Place cursor exactly on a tab stop
	vt.cursor.col = 8
	vt.cht(1)
	// Should advance to the NEXT tab stop (16), not stay at 8
	assert.Equal(t, column(16), vt.cursor.col)
}

// TERMINAL-010: CBT should not count a tab stop at the current cursor position.
func TestCBT_AtTabStop(t *testing.T) {
	vt := New()
	vt.Resize(80, 1)
	// Default tab stops at 8, 16, 24, ...
	// Place cursor exactly on a tab stop
	vt.cursor.col = 16
	vt.cbt(1)
	// Should move back to the PREVIOUS tab stop (8), not stay at 16
	assert.Equal(t, column(8), vt.cursor.col)
}

// TERMINAL-011: REP should advance the cursor and copy character attributes.
func TestREP_AdvancesCursor(t *testing.T) {
	vt := New()
	vt.Resize(6, 1)
	vt.mode = 0
	vt.print('x')
	assert.Equal(t, column(1), vt.cursor.col)
	vt.rep(3)
	// Cursor should have advanced by 3 positions
	assert.Equal(t, column(4), vt.cursor.col)
	assert.Equal(t, "xxxx  ", vt.String())
}

// TERMINAL-015: DECSCUSR should only call ps() once.
func TestDECSCUSR(t *testing.T) {
	vt := New()
	vt.Resize(2, 1)
	vt.csi(" q", []int{2})
	assert.Equal(t, tcell.CursorStyleSteadyBlock, vt.cursor.style)
}

// TERMINAL-028: DECSED (CSI ? J) should erase only unprotected characters.
// Selective Erase in Display erases cells that do NOT have the protected attribute.
func TestDECSED_SelectiveEraseDisplay(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	// Mark cell at column 1 ('b') as protected by setting a distinct attr
	// The protected attribute in DECSCA is tracked per-cell; for our implementation
	// we need to ensure selectiveErase() only clears content, not attrs.
	// For this test, we directly set the protected flag.
	vt.activeScreen[0][1].attrs = vt.activeScreen[0][1].attrs.Bold(true) // mark distinctly

	// CSI ? 2 J = selective erase entire display
	vt.cursor.col = 0
	vt.csi("?J", []int{2})

	// All unprotected cells should be erased (content = 0 → rendered as space)
	// Cell 1 has a non-default attr (bold) — but per spec, DECSED checks the
	// "erasable" (DECSCA) attribute, not bold. Since we haven't set DECSCA on any
	// cell, ALL cells should be erased in our basic implementation.
	assert.Equal(t, "    ", vt.String())
}

// TERMINAL-028: DECSEL (CSI ? K) should erase only unprotected characters on the current line.
func TestDECSEL_SelectiveEraseLine(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	// Fill row 0
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	// Fill row 1
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')
	vt.print('g')
	vt.print('h')

	// CSI ? 2 K on row 1 = selective erase entire line
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.csi("?K", []int{2})

	// Row 0 should be untouched, row 1 should be erased
	assert.Equal(t, 'a', vt.activeScreen[0][0].content)
	assert.Equal(t, 'b', vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[1][0].content)
	assert.Equal(t, rune(0), vt.activeScreen[1][1].content)
}

// TERMINAL-028: DECSED ps=0 should erase from cursor to end of screen.
func TestDECSED_EraseFromCursor(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	// Fill row 0
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	// Fill row 1
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')
	vt.print('g')
	vt.print('h')

	// Place cursor at row 0, col 2
	vt.cursor.row = 0
	vt.cursor.col = 2
	vt.csi("?J", []int{0})

	// a,b should remain; c,d and all of row 1 should be erased
	assert.Equal(t, 'a', vt.activeScreen[0][0].content)
	assert.Equal(t, 'b', vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][3].content)
	assert.Equal(t, rune(0), vt.activeScreen[1][0].content)
}

// TERMINAL-028: DECSEL ps=0 should erase from cursor to end of line.
func TestDECSEL_EraseFromCursor(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')

	vt.cursor.col = 2
	vt.csi("?K", []int{0})

	assert.Equal(t, 'a', vt.activeScreen[0][0].content)
	assert.Equal(t, 'b', vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][3].content)
}

// TERMINAL-F07: Private DSR 5 (CSI ? 5 n) responds with \x1b[?13n which means
// "no printer" (the response for CSI ? 15 n). The correct response for a
// general DEC status query is \x1b[?0n ("no malfunction detected").
func TestPrivateDSR5_Response(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	pty := &mockPty{}
	vt.pty = pty

	vt.csi("?n", []int{5})

	assert.Equal(t, "\x1b[?0n", pty.String())
}

// TERMINAL-F07 regression: Private DSR 6 (cursor position report) should still work.
func TestPrivateDSR6_Response(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	pty := &mockPty{}
	vt.pty = pty

	vt.cursor.row = 2
	vt.cursor.col = 5
	vt.csi("?n", []int{6})

	assert.Equal(t, "\x1b[?3;6R", pty.String())
}

// TERMINAL-F07 regression: Standard (non-private) DSR 5 should be unaffected.
func TestStandardDSR5_Response(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	pty := &mockPty{}
	vt.pty = pty

	vt.csi("n", []int{5})

	assert.Equal(t, "\x1b[0n", pty.String())
}

// TERMINAL-F10: DECSED/DECSEL erase all cells unconditionally. selectiveErase()
// should only erase cells that do NOT have the DECSCA protected attribute.
// Without the fix, protected cells are erased like any other cell.
func TestDECSED_SkipsProtectedCells(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')
	assert.Equal(t, "abcd", vt.String())

	// Mark cell at column 1 ('b') as protected
	vt.activeScreen[0][1].protected = true

	// Selective erase entire display (CSI ? 2 J)
	vt.cursor.col = 0
	vt.decsed(2)

	// 'b' should survive (protected), all others should be erased
	assert.Equal(t, rune(0), vt.activeScreen[0][0].content)
	assert.Equal(t, 'b', vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][3].content)
}

// TERMINAL-F10: DECSEL should also skip protected cells on the current line.
func TestDECSEL_SkipsProtectedCells(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')

	// Protect columns 0 and 3
	vt.activeScreen[0][0].protected = true
	vt.activeScreen[0][3].protected = true

	// Selective erase entire line (CSI ? 2 K)
	vt.cursor.col = 0
	vt.decsel(2)

	// Protected cells should survive
	assert.Equal(t, 'a', vt.activeScreen[0][0].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
	assert.Equal(t, 'd', vt.activeScreen[0][3].content)
}

// TERMINAL-F10 regression: regular erase (ED/EL) should still erase ALL cells
// including protected ones.
func TestED_IgnoresProtection(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')

	// Protect cell at column 1
	vt.activeScreen[0][1].protected = true

	// Regular erase entire display (CSI 2 J) — should erase everything
	vt.cursor.col = 0
	vt.ed(2)

	assert.Equal(t, "    ", vt.String())
}

// TERMINAL-F10: DECSCA (CSI Ps " q) should set the protection attribute on
// subsequently printed characters.
func TestDECSCA_ProtectsNewCharacters(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// Enable DECSCA protection
	vt.csi("\"q", []int{1})

	vt.print('a')
	vt.print('b')

	// Disable DECSCA protection
	vt.csi("\"q", []int{0})

	vt.print('c')
	vt.print('d')

	assert.Equal(t, "abcd", vt.String())

	// 'a' and 'b' should be protected, 'c' and 'd' should not
	assert.True(t, vt.activeScreen[0][0].protected)
	assert.True(t, vt.activeScreen[0][1].protected)
	assert.False(t, vt.activeScreen[0][2].protected)
	assert.False(t, vt.activeScreen[0][3].protected)

	// Selective erase should only erase 'c' and 'd'
	vt.cursor.col = 0
	vt.decsed(2)

	assert.Equal(t, 'a', vt.activeScreen[0][0].content)
	assert.Equal(t, 'b', vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][3].content)
}

// TERMINAL-F22: XTWINOPS (CSI Ps t) is not handled. This includes window size
// reporting (Ps=18 → report size in chars, Ps=14 → report size in pixels)
// used by applications like vim and tmux to query terminal dimensions.
func TestXTWINOPS_ReportSizeChars(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	pty := &mockPty{}
	vt.pty = pty

	// CSI 18 t → report size in characters: ESC [ 8 ; height ; width t
	vt.csi("t", []int{18})

	assert.Equal(t, "\x1b[8;24;80t", pty.String())
}

// TERMINAL-F22 regression: CSI 14 t should report pixel dimensions (approximated).
func TestXTWINOPS_ReportSizePixels(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	pty := &mockPty{}
	vt.pty = pty

	// CSI 14 t → report size in pixels: ESC [ 4 ; height_px ; width_px t
	// We approximate pixels as chars * a default cell size (e.g., 8x16)
	vt.csi("t", []int{14})

	assert.Equal(t, "\x1b[4;384;640t", pty.String())
}

// TERMINAL-F22 regression: unknown CSI t params should be silently ignored.
func TestXTWINOPS_UnknownParamIgnored(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)
	pty := &mockPty{}
	vt.pty = pty

	// Unknown param should not produce output or panic
	assert.NotPanics(t, func() {
		vt.csi("t", []int{99})
	})
	assert.Equal(t, "", pty.String())
}

// CSI s with no parameters should save cursor (SCOSC), but CSI s with
// parameters should set left/right margins (DECSLRM). The code always
// dispatched to decsc() (save cursor) regardless of parameters.
func TestCSI_S_DECSLRM_WithParams(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	// Set left/right margins via CSI 2;8 s
	vt.csi("s", []int{2, 8})

	// Left margin should be column 1 (parameter 2, 1-indexed -> 0-indexed = 1)
	assert.Equal(t, column(1), vt.margin.left, "DECSLRM should set left margin")
	// Right margin should be column 7 (parameter 8, 1-indexed -> 0-indexed = 7)
	assert.Equal(t, column(7), vt.margin.right, "DECSLRM should set right margin")
}

// CSI s with no parameters should still save cursor (SCOSC).
func TestCSI_S_SCOSC_NoParams(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	// Move cursor to a known position
	vt.cursor.row = 3
	vt.cursor.col = 7

	// CSI s with no params = save cursor
	vt.csi("s", []int{})

	// Move cursor elsewhere
	vt.cursor.row = 0
	vt.cursor.col = 0

	// CSI u = restore cursor
	vt.csi("u", []int{})

	// Cursor should be restored
	assert.Equal(t, row(3), vt.cursor.row, "SCOSC should save/restore cursor row")
	assert.Equal(t, column(7), vt.cursor.col, "SCOSC should save/restore cursor col")
}

// DECSLRM with invalid range (left >= right) should be ignored.
func TestCSI_S_DECSLRM_InvalidRange(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	origLeft := vt.margin.left
	origRight := vt.margin.right

	// left >= right should be ignored
	vt.csi("s", []int{5, 5})
	assert.Equal(t, origLeft, vt.margin.left)
	assert.Equal(t, origRight, vt.margin.right)

	// left > right should be ignored
	vt.csi("s", []int{8, 3})
	assert.Equal(t, origLeft, vt.margin.left)
	assert.Equal(t, origRight, vt.margin.right)
}

func TestIL_OutsideMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0

	// Fill screen
	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')

	// Set margins to middle row only
	vt.margin.top = 1
	vt.margin.bottom = 1
	vt.cursor.row = 0 // outside margin (above)

	vt.il(1)
	// Content should be unchanged since cursor is outside margin
	assert.Equal(t, "ab  \ncd  \nef  ", vt.String())
}

func TestIL_WithinMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')

	// Set margins
	vt.margin.top = 1
	vt.margin.bottom = 2
	vt.cursor.row = 1 // within margin

	vt.il(1)
	// Row 1 should be blanked, row 2 shifts down with old row 1 content
	assert.Equal(t, "ab  \n    \ncd  ", vt.String())
}

func TestDL_OutsideMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')

	// Set margins
	vt.margin.top = 1
	vt.margin.bottom = 2
	vt.cursor.row = 0 // outside margin (above)

	vt.dl(1)
	// Content should be unchanged since cursor is outside margin
	assert.Equal(t, "ab  \ncd  \nef  ", vt.String())
}

func TestDL_WithinMargins(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode = 0

	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.print('e')
	vt.print('f')

	// Set margins
	vt.margin.top = 1
	vt.margin.bottom = 2
	vt.cursor.row = 1 // within margin

	vt.dl(1)
	// Row 1 should be filled with row 2 content, row 2 blanked
	assert.Equal(t, "ab  \nef  \n    ", vt.String())
}

func TestREP_AtColumnZero(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// REP at column 0 with no preceding character does nothing
	vt.rep(3)
	assert.Equal(t, "    ", vt.String())
}

func TestREP_AfterCharacter(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.print('x')
	// Cursor is at col 1, repeat 'x' 2 more times
	vt.rep(2)
	assert.Equal(t, "xxx ", vt.String())
}

func TestDECSTBM_InvalidRange(t *testing.T) {
	vt := New()
	vt.Resize(10, 6)

	// Save current margins
	oldTop := vt.margin.top
	oldBottom := vt.margin.bottom

	// Invalid range: top >= bottom
	vt.decstbm([]int{5, 3})

	// Margins should be unchanged
	assert.Equal(t, oldTop, vt.margin.top)
	assert.Equal(t, oldBottom, vt.margin.bottom)
}

func TestDECSTBM_ClampToScreen(t *testing.T) {
	vt := New()
	vt.Resize(10, 4)

	vt.decstbm([]int{0, 99})

	// Should be clamped to screen height
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, row(3), vt.margin.bottom)
}

func TestDECSLRM_ZeroWidth(t *testing.T) {
	vt := New()
	vt.Resize(0, 1)

	// DECSLRM with zero width terminal — should not panic
	assert.NotPanics(t, func() {
		vt.decslrm([]int{0, 0})
	})
}

func TestDA_Response(t *testing.T) {
	t.Run("Primary DA", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		var buf bytes.Buffer
		vt.pty = &mockPtyCloser{Buffer: &buf}

		// CSI c (primary DA)
		vt.csi("c", []int{})
		// Should respond with device attributes
		assert.Contains(t, buf.String(), "\x1b[?")
		assert.Contains(t, buf.String(), "c")
	})

	t.Run("Secondary DA", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		var buf bytes.Buffer
		vt.pty = &mockPtyCloser{Buffer: &buf}

		// CSI > c (secondary DA)
		vt.csi(">c", []int{})
		assert.Contains(t, buf.String(), "\x1b[>")
		assert.Contains(t, buf.String(), ";")
	})
}

func TestDECSCA_Param2(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Set protected via DECSCA param 1
	vt.csi("\"q", []int{1})
	assert.True(t, vt.cursor.protected)

	// DECSCA param 2 deselects protection
	vt.csi("\"q", []int{2})
	assert.False(t, vt.cursor.protected)
}

func TestXTWINOPS_EmptyParams(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// CSI t with empty params should not panic
	assert.NotPanics(t, func() {
		vt.xtwinops([]int{})
	})
}

func TestXTWINOPS_SaveRestoreTitle(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Save/restore title (params 22/23) should not panic
	assert.NotPanics(t, func() {
		vt.xtwinops([]int{22})
	})
	assert.NotPanics(t, func() {
		vt.xtwinops([]int{23})
	})
}

func TestDECSCA_Param0(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// DECSCA param 0 (default): protection is off
	vt.csi("\"q", []int{0})
	assert.False(t, vt.cursor.protected)
}

// ICH (Insert Character) CSI Ps @ — ps=0 should default to 1.
func TestICH_Zero(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// Place characters
	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[0][1].content = 'b'
	vt.activeScreen[0][2].content = 'c'
	vt.cursor.col = 1

	vt.ich(0)

	// ps=0 defaults to 1, so one space inserted at col 1
	assert.Equal(t, 'a', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, ' ', rune(vt.activeScreen[0][1].content))
	assert.Equal(t, 'b', rune(vt.activeScreen[0][2].content))
}

// ICH (Insert Character) CSI Ps @ — basic insertion.
func TestICH_Basic(t *testing.T) {
	vt := New()
	vt.Resize(5, 1)
	vt.mode = 0

	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[0][1].content = 'b'
	vt.activeScreen[0][2].content = 'c'
	vt.activeScreen[0][3].content = 'd'
	vt.cursor.col = 1

	vt.ich(2)

	// Two spaces inserted starting at col 1, shifting content right
	assert.Equal(t, 'a', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, ' ', rune(vt.activeScreen[0][1].content))
	assert.Equal(t, ' ', rune(vt.activeScreen[0][2].content))
	assert.Equal(t, 'b', rune(vt.activeScreen[0][3].content))
}

// DCH (Delete Characters) CSI Ps P — ps=0 should default to 1.
func TestDCH_Zero(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[0][1].content = 'b'
	vt.activeScreen[0][2].content = 'c'
	vt.cursor.col = 1

	vt.dch(0)

	// Characters shift left, last char erased
	assert.Equal(t, 'a', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, 'c', rune(vt.activeScreen[0][1].content))
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
}

// DCH (Delete Characters) CSI Ps P — basic deletion.
func TestDCH_Basic(t *testing.T) {
	vt := New()
	vt.Resize(5, 1)
	vt.mode = 0

	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[0][1].content = 'b'
	vt.activeScreen[0][2].content = 'c'
	vt.activeScreen[0][3].content = 'd'
	vt.activeScreen[0][4].content = 'e'
	vt.cursor.col = 1

	vt.dch(2)

	// Two chars deleted at col 1, content shifted left
	assert.Equal(t, 'a', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, 'd', rune(vt.activeScreen[0][1].content))
	assert.Equal(t, 'e', rune(vt.activeScreen[0][2].content))
	assert.Equal(t, rune(0), vt.activeScreen[0][3].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][4].content)
}

// ECH (Erase Characters) CSI Ps X — basic erasure.
func TestECH_Basic(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.activeScreen[0][0].content = 'a'
	vt.activeScreen[0][1].content = 'b'
	vt.activeScreen[0][2].content = 'c'
	vt.cursor.col = 1

	vt.ech(2)

	// Two chars erased at cursor position
	assert.Equal(t, 'a', rune(vt.activeScreen[0][0].content))
	assert.Equal(t, rune(0), vt.activeScreen[0][1].content)
	assert.Equal(t, rune(0), vt.activeScreen[0][2].content)
}

// CBT (Cursor Backward Tabulation) CSI Ps Z — basic.
func TestCBT_Basic(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)

	vt.cursor.col = 25
	vt.cbt(1)

	// Should move to previous tab stop (at 24, since 16, 8 are earlier)
	// Actually stop 24 has column 8, 16, 24
	// Since cursor is at 25, CBT(1) should go to 24
	assert.Equal(t, column(24), vt.cursor.col)
}

// CBT (Cursor Backward Tabulation) — ps=0 defaults to 1.
func TestCBT_Zero(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)

	vt.cursor.col = 25
	vt.cbt(0)

	// Should move to previous tab stop
	assert.Equal(t, column(24), vt.cursor.col)
}

// HPA (Horizontal Position Absolute) CSI Ps ` — basic.
func TestHPA_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.cursor.col = 0

	vt.hpa(5)

	// Should move to column 4 (0-indexed, ps=5 -> col=4)
	assert.Equal(t, column(4), vt.cursor.col)
}

// HPA with DECOM — margin-relative.
func TestHPA_WithDECOM(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.mode |= decom
	vt.margin.left = 2
	vt.margin.right = 7
	vt.cursor.col = 0

	vt.hpa(3)

	// With DECOM, hpa(3) should set col = margin.left + (3-1) = 2+2 = 4
	assert.Equal(t, column(4), vt.cursor.col)
}

// HPA with DECOM — clamped to right margin.
func TestHPA_WithDECOM_Clamp(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.mode |= decom
	vt.margin.left = 2
	vt.margin.right = 7
	vt.cursor.col = 0

	vt.hpa(20)

	// Clamped to margin.right = 7
	assert.Equal(t, column(7), vt.cursor.col)
}

// HPR (Horizontal Position Relative) CSI Ps a — basic.
func TestHPR_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.cursor.col = 2

	vt.hpr(3)

	// Move right 3 columns
	assert.Equal(t, column(5), vt.cursor.col)
}

// HPR — clamped to right edge.
func TestHPR_Clamp(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.cursor.col = 2

	vt.hpr(10)

	// Clamped to width-1 = 3
	assert.Equal(t, column(3), vt.cursor.col)
}

// CNL (Cursor Next Line) CSI Ps E — basic.
func TestCNL_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.cursor.row = 1
	vt.cursor.col = 5

	vt.cnl(2)

	// Should move down 2 rows and to left margin
	assert.Equal(t, row(3), vt.cursor.row)
	assert.Equal(t, vt.margin.left, vt.cursor.col)
}

// CNL — limited to bottom margin.
func TestCNL_Limit(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.cursor.row = 1
	vt.margin.top = 0
	vt.margin.bottom = 2

	vt.cnl(5)

	// Should stop at bottom margin
	assert.Equal(t, row(2), vt.cursor.row)
	assert.Equal(t, vt.margin.left, vt.cursor.col)
}

// CPL (Cursor Previous Line) CSI Ps F — basic.
func TestCPL_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.cursor.row = 3
	vt.cursor.col = 5

	vt.cpl(2)

	// Should move up 2 rows and to left margin
	assert.Equal(t, row(1), vt.cursor.row)
	assert.Equal(t, vt.margin.left, vt.cursor.col)
}

// CPL — limited to top margin.
func TestCPL_Limit(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.cursor.row = 1
	vt.margin.top = 0

	vt.cpl(5)

	// Should stop at top margin
	assert.Equal(t, row(0), vt.cursor.row)
	assert.Equal(t, vt.margin.left, vt.cursor.col)
}

// VPA (Line Position Absolute) CSI Ps d — basic.
func TestVPA_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.cursor.row = 0

	vt.vpa(3)

	// Should move to row 2 (0-indexed)
	assert.Equal(t, row(2), vt.cursor.row)
}

// VPA with DECOM — margin-relative.
func TestVPA_WithDECOM(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode |= decom
	vt.margin.top = 1
	vt.margin.bottom = 4

	vt.vpa(2)

	// With DECOM, vpa(2) = margin.top + (2-1) = 1+1 = 2
	assert.Equal(t, row(2), vt.cursor.row)
}

// VPA with DECOM — clamped to bottom margin.
func TestVPA_WithDECOM_Clamp(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode |= decom
	vt.margin.top = 1
	vt.margin.bottom = 3

	vt.vpa(10)

	// Clamped to bottom margin
	assert.Equal(t, row(3), vt.cursor.row)
}

// VPR (Line Position Relative) CSI Ps e — basic.
func TestVPR_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.cursor.row = 1

	vt.vpr(2)

	// Move down 2 rows
	assert.Equal(t, row(3), vt.cursor.row)
}

// VPR — clamped.
func TestVPR_Clamp(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.cursor.row = 1

	vt.vpr(10)

	// Clamped to height-1 = 2
	assert.Equal(t, row(2), vt.cursor.row)
}

// Cursor Style (CSI Ps SP q).
func TestCursorStyle(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.csi(" q", []int{3})
	assert.Equal(t, tcell.CursorStyle(3), vt.cursor.style)
}

// DECSLRM — set left and right margins (params are 1-indexed).
func TestDECSLRM_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 4)

	vt.decslrm([]int{2, 7})

	// Params are 1-indexed: left=2 -> column(1), right=7 -> column(6)
	assert.Equal(t, column(1), vt.margin.left)
	assert.Equal(t, column(6), vt.margin.right)
}

// DECSTBM — set top and bottom margins (params are 1-indexed).
func TestDECSTBM_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 6)

	vt.decstbm([]int{1, 4})

	// Params are 1-indexed: top=1 -> row(0), bottom=4 -> row(3)
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, row(3), vt.margin.bottom)
}

// TBC (Tab Clear) CSI Ps g — ps=0 clear current tab.
func TestTBC_ClearCurrent(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)

	// Count initial tab stops
	initialCount := len(vt.tabStop)

	vt.cursor.col = 8
	vt.tbc(0)

	// Tab stop at 8 should be removed
	assert.Equal(t, initialCount-1, len(vt.tabStop))
	for _, ts := range vt.tabStop {
		if ts == 8 {
			t.Error("tab stop at 8 should have been removed")
		}
	}
}

// TBC (Tab Clear) CSI Ps g — ps=3 clear all tabs.
func TestTBC_ClearAll(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)

	vt.tbc(3)

	// All tab stops should be cleared
	assert.Empty(t, vt.tabStop)
}

// HVP (Horizontal Vertical Position) CSI Ps;Ps f — same as CUP.
func TestHVP_Basic(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.cursor.row = 0
	vt.cursor.col = 0

	vt.csi("f", []int{3, 5})

	// Should move cursor to row 2 (0-indexed), col 4 (0-indexed)
	assert.Equal(t, row(2), vt.cursor.row)
	assert.Equal(t, column(4), vt.cursor.col)
}

// CSI T with 5 params (XTHIMOUSE) should not panic.
func TestCSI_T_XTHIMOUSE(t *testing.T) {
	vt := New()
	vt.Resize(80, 24)

	assert.NotPanics(t, func() {
		vt.csi("T", []int{0, 1, 2, 3, 4})
	})
}

// CSI S (scroll up) with no params should default to 1.
func TestCSI_ScrollUp_Default(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	vt.print('a')
	vt.print('b')

	vt.csi("S", []int{})
	// Content should have scrolled up (4 columns per row, all blank now)
	assert.Equal(t, "    \n    ", vt.String())
}

// CSI s with invalid range (left >= right) should be ignored.
func TestCSI_S_DECSLRM_Invalid(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)

	origLeft := vt.margin.left
	origRight := vt.margin.right

	// left >= right should be ignored
	vt.csi("s", []int{5, 5})
	assert.Equal(t, origLeft, vt.margin.left)
	assert.Equal(t, origRight, vt.margin.right)

	// left > right should be ignored
	vt.csi("s", []int{8, 3})
	assert.Equal(t, origLeft, vt.margin.left)
	assert.Equal(t, origRight, vt.margin.right)
}

// CSI ? J (DECSED) with ps=1 should erase from beginning to cursor.
func TestDECSED_EraseToCursor(t *testing.T) {
	vt := New()
	vt.Resize(4, 2)
	vt.mode = 0
	vt.print('a')
	vt.print('b')
	vt.cursor.row = 1
	vt.cursor.col = 0
	vt.print('c')
	vt.print('d')

	// Move cursor to row 0, col 2
	vt.cursor.row = 0
	vt.cursor.col = 2

	vt.csi("?J", []int{1})

	// a,b should be erased, c,d should remain on row 1
	var zeroRune rune
	assert.Equal(t, zeroRune, vt.activeScreen[0][0].content)
	assert.Equal(t, zeroRune, vt.activeScreen[0][1].content)
	assert.Equal(t, 'c', vt.activeScreen[1][0].content)
	assert.Equal(t, 'd', vt.activeScreen[1][1].content)
}

// CSI ? K (DECSEL) with ps=1 should erase from beginning to cursor (inclusive).
func TestDECSEL_EraseToCursor(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0
	vt.print('a')
	vt.print('b')
	vt.print('c')
	vt.print('d')

	vt.cursor.col = 2
	vt.csi("?K", []int{1})

	var zeroRune rune
	// ps=1 erases from beginning to cursor (inclusive), so cols 0,1,2 erased
	assert.Equal(t, zeroRune, vt.activeScreen[0][0].content)
	assert.Equal(t, zeroRune, vt.activeScreen[0][1].content)
	assert.Equal(t, zeroRune, vt.activeScreen[0][2].content)
	assert.Equal(t, 'd', vt.activeScreen[0][3].content)
}

// TestCSI_DispatchComprehensive exercises the full parser→csi() path for
// all CSI final bytes so coverage counters at csi.go:11 are incremented.
func TestCSI_DispatchComprehensive(t *testing.T) {
	t.Run("CUU up", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[2B\x1b[2A")
	})

	t.Run("CUD down", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[2B")
	})

	t.Run("CUF forward", func(t *testing.T) {
		h, _ := NewTerminal(10, 3)
		defer h.Close()
		h.FeedString("\x1b[3C")
		c, _, _ := h.Cursor()
		if c != 3 {
			t.Errorf("CUF col=%d, want 3", c)
		}
	})

	t.Run("CUB back", func(t *testing.T) {
		h, _ := NewTerminal(10, 3)
		defer h.Close()
		h.FeedString("\x1b[5C\x1b[2D")
		c, _, _ := h.Cursor()
		if c != 3 {
			t.Errorf("CUB col=%d, want 3", c)
		}
	})

	t.Run("CNL next line", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[2E")
		_, r, _ := h.Cursor()
		if r != 2 {
			t.Errorf("CNL row=%d, want 2", r)
		}
	})

	t.Run("CPL prev line", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[4B\x1b[2F")
		_, r, _ := h.Cursor()
		if r != 2 {
			t.Errorf("CPL row=%d, want 2", r)
		}
	})

	t.Run("CHA col abs", func(t *testing.T) {
		h, _ := NewTerminal(10, 3)
		defer h.Close()
		h.FeedString("\x1b[5G")
		c, _, _ := h.Cursor()
		if c != 4 {
			t.Errorf("CHA col=%d, want 4", c)
		}
	})

	t.Run("CHT tab forward", func(t *testing.T) {
		h, _ := NewTerminal(40, 3)
		defer h.Close()
		h.FeedString("\x1b[2I")
		c, _, _ := h.Cursor()
		if c != 16 {
			t.Errorf("CHT col=%d, want 16", c)
		}
	})

	t.Run("ICH insert", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("abcde")
		h.FeedString("\x1b[1;2H\x1b[2@")
	})

	t.Run("DCH delete", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("abcde")
		h.FeedString("\x1b[2P")
	})

	t.Run("ECH erase", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("abcde")
		h.FeedString("\x1b[2X")
	})

	t.Run("CBT tab back", func(t *testing.T) {
		h, _ := NewTerminal(40, 3)
		defer h.Close()
		h.FeedString("\x1b[20G\x1b[2Z")
	})

	t.Run("HPA col abs", func(t *testing.T) {
		h, _ := NewTerminal(10, 3)
		defer h.Close()
		h.FeedString("\x1b[5`")
		c, _, _ := h.Cursor()
		if c != 4 {
			t.Errorf("HPA col=%d, want 4", c)
		}
	})

	t.Run("HPR col rel", func(t *testing.T) {
		h, _ := NewTerminal(10, 3)
		defer h.Close()
		h.FeedString("\x1b[3a")
		c, _, _ := h.Cursor()
		if c != 3 {
			t.Errorf("HPR col=%d, want 3", c)
		}
	})

	t.Run("REP repeat", func(t *testing.T) {
		h, _ := NewTerminal(6, 1)
		defer h.Close()
		h.FeedString("X\x1b[3b")
	})

	t.Run("VPA row abs", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[4d")
		_, r, _ := h.Cursor()
		if r != 3 {
			t.Errorf("VPA row=%d, want 3", r)
		}
	})

	t.Run("VPR row rel", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[2e")
		_, r, _ := h.Cursor()
		if r != 2 {
			t.Errorf("VPR row=%d, want 2", r)
		}
	})

	t.Run("HVP via f", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[3;5f")
		c, r, _ := h.Cursor()
		if c != 4 || r != 2 {
			t.Errorf("HVP cursor=(%d,%d), want (4,2)", c, r)
		}
	})

	t.Run("TBC clear tab", func(t *testing.T) {
		h, _ := NewTerminal(40, 3)
		defer h.Close()
		h.FeedString("\x1b[8G\x1b[0g")
	})

	t.Run("TBC clear all", func(t *testing.T) {
		h, _ := NewTerminal(40, 3)
		defer h.Close()
		h.FeedString("\x1b[3g")
	})

	t.Run("DSR5 ok", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		h.FeedString("\x1b[5n")
		out := h.Output()
		if !bytes.Contains(out, []byte("\x1b[0n")) {
			t.Errorf("DSR5 output=%q", out)
		}
	})

	t.Run("DSR6 cursor", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		h.FeedString("\x1b[6n")
		out := h.Output()
		if len(out) == 0 {
			t.Errorf("DSR6 expected output")
		}
	})

	t.Run("DEC DSR5 ok", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		h.FeedString("\x1b[?5n")
		out := h.Output()
		if !bytes.Contains(out, []byte("\x1b[?")) {
			t.Errorf("DEC DSR5 output=%q", out)
		}
	})

	t.Run("DEC DSR6 cursor", func(t *testing.T) {
		h, _ := NewTerminal(4, 3)
		defer h.Close()
		h.FeedString("\x1b[?6n")
		out := h.Output()
		if !bytes.Contains(out, []byte("\x1b[?")) {
			t.Errorf("DEC DSR6 output=%q", out)
		}
	})

	t.Run("cursor style", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b[3 q") })
	})

	t.Run("DECSTR !p", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b[!p") })
	})

	t.Run("DECRQM $p", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b[2$p") })
	})

	t.Run("DECSACE *x", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b[2*x") })
	})

	t.Run("DECRQPSR $w", func(t *testing.T) {
		h, _ := NewTerminal(4, 1)
		defer h.Close()
		assert.NotPanics(t, func() { h.FeedString("\x1b[1$w") })
	})

	t.Run("IL insert lines", func(t *testing.T) {
		h, _ := NewTerminal(4, 3)
		defer h.Close()
		h.FeedString("\x1b[2;1H\x1b[1L")
	})

	t.Run("DL delete lines", func(t *testing.T) {
		h, _ := NewTerminal(4, 3)
		defer h.Close()
		h.FeedString("\x1b[2;1H\x1b[1M")
	})

	t.Run("scroll up S", func(t *testing.T) {
		h, _ := NewTerminal(4, 2)
		defer h.Close()
		h.FeedString("\x1b[1S")
	})

	t.Run("scroll down T", func(t *testing.T) {
		h, _ := NewTerminal(4, 2)
		defer h.Close()
		h.FeedString("\x1b[1T")
	})

	t.Run("DECSTBM r", func(t *testing.T) {
		h, _ := NewTerminal(4, 5)
		defer h.Close()
		h.FeedString("\x1b[2;4r")
	})

	t.Run("DECSLRM s params", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[2;8s")
	})

	t.Run("SCOSC s/u", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[3;3H\x1b[s\x1b[H\x1b[u")
		c, r, _ := h.Cursor()
		if c != 2 || r != 2 {
			t.Errorf("SCOSC cursor=(%d,%d), want (2,2)", c, r)
		}
	})
}

// TestCSI_CHA_NearMarginLeft tests CHA clamping with non-zero left margin.
func TestCSI_CHA_NearMarginLeft(t *testing.T) {
	vt := New()
	vt.Resize(10, 3)
	vt.margin.left = 2
	vt.margin.right = 7
	// Set cursor at right margin
	vt.cursor.col = 7
	// CHA 1 should try to set col=0, but margin.left=2 clamps it to 2
	vt.csi("G", []int{1})
	assert.Equal(t, column(2), vt.cursor.col,
		"CHA should clamp to margin.left")
}

// TestIL_CursorOutOfBounds tests IL when cursor is outside margins.
func TestIL_CursorOutOfBounds(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.margin.top = 1
	vt.margin.bottom = 3
	vt.margin.left = 0
	vt.margin.right = 3

	// Cursor above top margin
	vt.cursor.row = 0
	vt.cursor.col = 0
	vt.il(1) // should be a no-op
	assert.Equal(t, row(0), vt.cursor.row)

	// Cursor left of left margin
	vt.margin.left = 2
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.il(1) // should be a no-op
	assert.Equal(t, column(0), vt.cursor.col)

	// Cursor right of right margin
	vt.margin.right = 3
	vt.cursor.col = 4 // past right margin
	vt.il(1)          // should be a no-op
	assert.Equal(t, column(4), vt.cursor.col)
}

// TestDL_CursorOutOfBounds tests DL when cursor is outside margins.
func TestDL_CursorOutOfBounds(t *testing.T) {
	vt := New()
	vt.Resize(4, 4)
	vt.margin.top = 1
	vt.margin.bottom = 3

	// Cursor above top margin
	vt.cursor.row = 0
	vt.dl(1) // should be a no-op
	assert.Equal(t, row(0), vt.cursor.row)

	// Cursor left of left margin
	vt.margin.left = 2
	vt.cursor.row = 2
	vt.cursor.col = 0
	vt.dl(1) // should be a no-op
	assert.Equal(t, column(0), vt.cursor.col)
}

// TestCBT_FromFirstTab tests CBT when already at the first tab stop.
func TestCBT_FromFirstTab(t *testing.T) {
	vt := New()
	vt.Resize(40, 1)
	vt.cursor.col = 8 // first tab stop

	vt.cbt(1) // should stay at col 8 (no earlier tab stop)
	assert.Equal(t, column(8), vt.cursor.col)
}

// TestDECSTBM_ZeroHeight tests DECSTBM with zero-height terminal.
func TestDECSTBM_ZeroHeight(t *testing.T) {
	vt := New()

	// DECSTBM on zero-height terminal should not panic
	assert.NotPanics(t, func() {
		vt.decstbm([]int{1, 5})
	})
}

// TestDECSTBM_InvalidTop tests DECSTBM with invalid (top >= bottom).
func TestDECSTBM_InvalidTop(t *testing.T) {
	vt := New()
	vt.Resize(10, 6)

	origTop := vt.margin.top
	origBottom := vt.margin.bottom

	// Top >= bottom should leave margins unchanged
	vt.decstbm([]int{5, 3})
	assert.Equal(t, origTop, vt.margin.top)
	assert.Equal(t, origBottom, vt.margin.bottom)
}

// TestDECSTBM_ParamDefaults tests DECSTBM with empty params (reset to full).
func TestDECSTBM_ParamDefaults(t *testing.T) {
	vt := New()
	vt.Resize(10, 6)
	vt.margin.top = 2
	vt.margin.bottom = 4

	// Empty params = reset margins to full screen
	vt.decstbm([]int{})
	assert.Equal(t, row(0), vt.margin.top)
	assert.Equal(t, row(5), vt.margin.bottom)
}

// TestDECRC_AltScreen tests cursor restore in alt screen mode.
func TestDECRC_AltScreen(t *testing.T) {
	vt := New()
	vt.Resize(4, 3)
	vt.mode |= smcup // simulate alt screen

	vt.cursor.col = 2
	vt.cursor.row = 1
	vt.decsc() // save in alt state

	vt.cursor.col = 0
	vt.cursor.row = 0

	vt.decrc() // restore from alt state
	assert.Equal(t, column(2), vt.cursor.col)
	assert.Equal(t, row(1), vt.cursor.row)
}

// TestDECSTBM_WithCustomLeftMargin tests DECSTBM interaction with left margin.
func TestDECSTBM_WithCustomLeftMargin(t *testing.T) {
	vt := New()
	vt.Resize(10, 6)
	vt.margin.left = 2
	vt.margin.top = 1
	vt.margin.bottom = 4

	vt.decstbm([]int{2, 3})
	assert.Equal(t, row(1), vt.margin.top)
	assert.Equal(t, row(2), vt.margin.bottom)
}

// TestCSI_FinalEdgeCases tests remaining single-statement CSI edge cases.
func TestCSI_FinalEdgeCases(t *testing.T) {
	// CUD inside margin but moving past (line 226)
	t.Run("CUD past margin", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 5)
		vt.margin.top = 1
		vt.margin.bottom = 3
		vt.cursor.row = 2
		vt.cud(5)                              // tries to go past margin.bottom
		assert.Equal(t, row(3), vt.cursor.row) // clamped to margin.bottom
	})

	// CUF inside margin but moving past (line 243)
	t.Run("CUF past margin", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 3)
		vt.margin.left = 1
		vt.margin.right = 5
		vt.cursor.col = 3
		vt.cuf(5)                                 // tries to go past margin.right
		assert.Equal(t, column(5), vt.cursor.col) // clamped to margin.right
	})

	// CUB inside margin but moving past (line 260)
	t.Run("CUB past margin", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 3)
		vt.margin.left = 1
		vt.margin.right = 5
		vt.cursor.col = 3
		vt.cub(5)                                 // tries to go past margin.left
		assert.Equal(t, column(1), vt.cursor.col) // clamped to margin.left
	})

	// CNL past bottom margin (line 277)
	t.Run("CNL past bottom", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 5)
		vt.margin.bottom = 3
		vt.cursor.row = 2
		vt.cnl(5)                              // tries to go past margin.bottom
		assert.Equal(t, row(3), vt.cursor.row) // stays at margin.bottom
	})

	// CPL past top margin (line 291)
	t.Run("CPL past top", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 5)
		vt.margin.top = 1
		vt.cursor.row = 2
		vt.cpl(5)
		assert.Equal(t, row(1), vt.cursor.row) // stops at margin.top
	})

	// CHA left of margin (line 310)
	t.Run("CHA left of margin", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 3)
		vt.margin.left = 2
		vt.cursor.col = 5
		vt.csi("G", []int{1})                     // CHA 1 -> col=0, but margin.left=2
		assert.Equal(t, column(2), vt.cursor.col) // clamped to margin.left
	})

	// CUP with DECOM and col < left (line 341)
	t.Run("CUP col below left margin", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 5)
		vt.mode |= decom
		vt.margin.left = 2
		vt.cup([]int{1, 1})                       // row=0, col=0, but margin.left=2
		assert.Equal(t, column(2), vt.cursor.col) // clamped to margin.left
	})

	// CUP with DECOM and row < top (line 347)
	t.Run("CUP row above top margin", func(t *testing.T) {
		vt := New()
		vt.Resize(8, 5)
		vt.mode |= decom
		vt.margin.top = 1
		vt.cup([]int{1, 1})                    // row=0, col=0, but margin.top=1
		assert.Equal(t, row(1), vt.cursor.row) // clamped to margin.top
	})

	// CHT cursor already at tab stop (line 356)
	t.Run("CHT at tab stop", func(t *testing.T) {
		vt := New()
		vt.Resize(40, 3)
		vt.cursor.col = 8
		vt.cht(1) // should move to next tab stop (16)
		assert.Equal(t, column(16), vt.cursor.col)
	})

	// IL with ps=0 (line 479)
	t.Run("IL ps=0", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 5)
		vt.cursor.row = 2
		vt.cursor.col = 0
		vt.il(0) // should default to 1
	})

	// DL with ps=0 (line 524)
	t.Run("DL ps=0", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 5)
		vt.cursor.row = 2
		vt.cursor.col = 0
		vt.dl(0) // should default to 1
	})

	// ECH at edge (line 574)
	t.Run("ECH at edge", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.col = 3
		vt.ech(5) // ps > remaining columns
	})

	// VPR with ps=0 (line 650)
	t.Run("VPR ps=0", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 3)
		vt.vpr(0) // should default to 1
	})

	// HPR with ps=0 (line 685)
	t.Run("HPR ps=0", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 3)
		vt.hpr(0) // should default to 1
	})

	// REP with ps=0 (line 713)
	t.Run("REP ps=0", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.rep(0) // ps=0 defaults to 1, but at col 0 no last char
	})

	// DECSTBM height=0 (line 732)
	t.Run("DECSTBM height=0", func(t *testing.T) {
		vt := New()
		vt.decstbm([]int{1, 5}) // no resize -> height=0 -> return
	})

	// DECSLRM width=0 (line 754)
	t.Run("DECSLRM width=0", func(t *testing.T) {
		vt := New()
		vt.decslrm([]int{2, 7}) // no resize -> width=0 -> return
	})

	// DECSLRM invalid range (line 757)
	t.Run("DECSLRM left>=right", func(t *testing.T) {
		vt := New()
		vt.Resize(10, 5)
		vt.decslrm([]int{5, 3}) // left >= right -> ignore
	})

	// T scroll down 0 params (line 55)
	t.Run("T scroll down no params", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 3)
		assert.NotPanics(t, func() {
			vt.csi("T", []int{})
		})
	})

	// S scroll up no params
	t.Run("S scroll up no params", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 3)
		assert.NotPanics(t, func() {
			vt.csi("S", []int{})
		})
	})

	// DSR cursor at non-zero position
	t.Run("DSR cursor at non-zero", func(t *testing.T) {
		h, _ := NewTerminal(10, 5)
		defer h.Close()
		h.FeedString("\x1b[2;3H") // cursor at row 2 col 3
		h.FeedString("\x1b[6n")   // DSR cursor position
		out := h.Output()
		if !bytes.Contains(out, []byte("\x1b[")) {
			t.Errorf("expected cursor position report, got %q", out)
		}
	})
}

// ============================================================
// VPA (Line Position Absolute) and HPA (Character Position
// Absolute) use absolute screen coordinates even when DECOM
// (origin mode) is active. Per the Session standard, when DECOM is
// set, cursor positioning should be relative to the scroll
// region margins, matching the behavior of CUP.
// ============================================================

// VPA should offset by margin.top when DECOM is active.
func TestVPA_WithOriginMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	vt.margin.top = 2
	vt.margin.bottom = 7
	vt.mode |= decom

	// VPA 1 (go to first line of scroll region) → row = margin.top + 0 = 2
	vt.vpa(1)
	assert.Equal(t, row(2), vt.cursor.row,
		"VPA 1 with DECOM should place cursor at margin.top")

	// VPA 3 → row = margin.top + 2 = 4
	vt.vpa(3)
	assert.Equal(t, row(4), vt.cursor.row,
		"VPA 3 with DECOM should place cursor at margin.top + 2")
}

// VPA should clamp to margin.bottom when DECOM is active.
func TestVPA_WithOriginMode_ClampsToBottom(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	vt.margin.top = 2
	vt.margin.bottom = 5
	vt.mode |= decom

	// VPA 99 → should clamp to margin.bottom = 5
	vt.vpa(99)
	assert.Equal(t, row(5), vt.cursor.row,
		"VPA with DECOM should clamp to margin.bottom")
}

// HPA should offset by margin.left when DECOM is active.
func TestHPA_WithOriginMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	vt.margin.left = 3
	vt.margin.right = 8
	vt.mode |= decom

	// HPA 1 → col = margin.left + 0 = 3
	vt.hpa(1)
	assert.Equal(t, column(3), vt.cursor.col,
		"HPA 1 with DECOM should place cursor at margin.left")

	// HPA 3 → col = margin.left + 2 = 5
	vt.hpa(3)
	assert.Equal(t, column(5), vt.cursor.col,
		"HPA 3 with DECOM should place cursor at margin.left + 2")
}

// HPA should clamp to margin.right when DECOM is active.
func TestHPA_WithOriginMode_ClampsToRight(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	vt.margin.left = 3
	vt.margin.right = 6
	vt.mode |= decom

	// HPA 99 → should clamp to margin.right = 6
	vt.hpa(99)
	assert.Equal(t, column(6), vt.cursor.col,
		"HPA with DECOM should clamp to margin.right")
}

// Regression: VPA without DECOM should still use absolute positioning.
func TestVPA_WithoutOriginMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	vt.margin.top = 3

	// VPA 2 without DECOM → row = 1 (absolute, 0-indexed)
	vt.vpa(2)
	assert.Equal(t, row(1), vt.cursor.row)
}

// Regression: HPA without DECOM should still use absolute positioning.
func TestHPA_WithoutOriginMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 10)
	vt.margin.left = 3

	// HPA 2 without DECOM → col = 1 (absolute, 0-indexed)
	vt.hpa(2)
	assert.Equal(t, column(1), vt.cursor.col)
}

package terminal

import (
	"fmt"
	"io"
	"strings"

	"github.com/gdamore/tcell/v3"
)

func (s *Session) csi(csi string, params []int) {
	switch csi {
	case "@":
		s.ich(ps(params))
	case "A":
		s.cuu(ps(params))
	case "B":
		s.cud(ps(params))
	case "C":
		s.cuf(ps(params))
	case "D":
		s.cub(ps(params))
	case "E":
		s.cnl(ps(params))
	case "F":
		s.cpl(ps(params))
	case "G":
		s.cha(ps(params))
	case "H":
		s.cup(params)
	case "I":
		s.cht(ps(params))
	case "J":
		s.ed(ps(params))
	case "K":
		s.el(ps(params))
	case "L":
		s.il(ps(params))
	case "M":
		s.dl(ps(params))
	case "P":
		s.dch(ps(params))
	case "S":
		ps := ps(params)
		if ps == 0 {
			ps = 1
		}
		s.scrollUp(ps)
	case "T":
		// 5 params is XTHIMOUSE, ignore
		if len(params) == 5 {
			return
		}
		ps := ps(params)
		if ps == 0 {
			ps = 1
		}
		s.scrollDown(ps)
	case "X":
		s.ech(ps(params))
	case "Z":
		s.cbt(ps(params))
	case "`":
		s.hpa(ps(params))
	case "a":
		s.hpr(ps(params))
	case "b":
		s.rep(ps(params))
	case "c":
		// Send device attributes
		resp := strings.Builder{}
		// Response introducer
		resp.WriteString("\x1B[?")
		// We are a vt220
		resp.WriteString("62;")
		// We have ANSI color support
		resp.WriteString("22")
		// Response terminator
		resp.WriteString("c")
		io.WriteString(s.pty, resp.String())
	case ">c":
		// Send secondary device attributes.
		// Report a generic VT220-compatible terminal.
		io.WriteString(s.pty, "\x1B[>1;0;0c")
	case "d":
		s.vpa(ps(params))
	case "e":
		s.vpr(ps(params))
	case "f":
		// Same as CUP
		s.cup(params)
	case "g":
		s.tbc(ps(params))
	case "h":
		s.sm(params)
	case "?h":
		s.decset(params)
	case "l":
		s.rm(params)
	case "?J":
		s.decsed(ps(params))
	case "?K":
		s.decsel(ps(params))
	case "?l":
		s.decrst(params)
	case "m":
		s.sgr(params)
	case "n":
		// Send device status report
		switch ps(params) {
		case 5:
			// "Ok"
			io.WriteString(s.pty, "\x1B[0n")
		case 6:
			// report cursor position
			// This sequence can be identical to a function key?
			// CSI r ; c R
			row, col := s.reportedCursor()
			resp := fmt.Sprintf("\x1B[%d;%dR", row+1, col+1)
			io.WriteString(s.pty, resp)
		}
	case "?n":
		// Send DEC device status report.
		switch ps(params) {
		case 5:
			io.WriteString(s.pty, "\x1B[?0n")
		case 6:
			row, col := s.reportedCursor()
			resp := fmt.Sprintf("\x1B[?%d;%dR", row+1, col+1)
			io.WriteString(s.pty, resp)
		}
	case "r":
		s.decstbm(params)
	case "s":
		if len(params) >= 2 {
			s.decslrm(params)
		} else {
			s.decsc()
		}
	case "u":
		s.decrc()
	case " q":
		s.cursor.style = tcell.CursorStyle(ps(params))
	case "\"q":
		// DECSCA — Select Character Protection Attribute
		switch ps(params) {
		case 1:
			s.cursor.protected = true
		default:
			// Ps=0 or Ps=2: deselect protection
			s.cursor.protected = false
		}
	case "t":
		// XTWINOPS — Window Manipulation
		s.xtwinops(params)
	}
}

// Returns a single parameter from a slice of parameters, or 0 if the slice is
// empty
func ps(params []int) int {
	var ps int
	if len(params) > 0 {
		ps = params[0]
	}
	return ps
}

func param(params []int, index int, def int) int {
	if index >= len(params) {
		return def
	}
	if params[index] == 0 {
		return def
	}
	return params[index]
}

// Insert Blank Character (ICH) CSI Ps @
// Insert Ps blank characters. Cursor does not change position.
func (s *Session) ich(ps int) {
	if ps == 0 {
		ps = 1
	}
	col := s.cursor.col
	row := s.cursor.row
	line := s.activeScreen[row]
	for i := s.margin.right; i > col; i -= 1 {
		if (i - column(ps)) < col {
			continue
		}
		line[i] = line[i-column(ps)]
	}
	for i := 0; i < ps; i += 1 {
		if int(col)+i >= s.width() {
			break
		}
		line[col+column(i)] = cell{
			content: ' ',
			width:   1,
		}
	}
}

// Cursur Up (CUU) CSI Ps A
// Move cursor up in same column, stopping at top margin
func (s *Session) cuu(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	clamp := row(0)
	if s.cursor.row >= s.margin.top {
		clamp = s.margin.top
	}
	s.cursor.row -= row(ps)
	if s.cursor.row < clamp {
		s.cursor.row = clamp
	}
}

// Cursur Down (CUD) CSI Ps B
// Move cursor down in same column, stopping at bottom margin
func (s *Session) cud(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	clamp := row(s.height() - 1)
	if s.cursor.row <= s.margin.bottom {
		clamp = s.margin.bottom
	}
	s.cursor.row += row(ps)
	if s.cursor.row > clamp {
		s.cursor.row = clamp
	}
}

// Cursur Forward (CUF) CSI Ps C
// Move cursor forward Ps columns, stopping at the right margin
func (s *Session) cuf(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	clamp := column(s.width() - 1)
	if s.cursor.col <= s.margin.right {
		clamp = s.margin.right
	}
	s.cursor.col += column(ps)
	if s.cursor.col > clamp {
		s.cursor.col = clamp
	}
}

// Cursur Backward (CUB) CSI Ps D
// Move cursor backward Ps columns, stopping at the left margin
func (s *Session) cub(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	clamp := column(0)
	if s.cursor.col >= s.margin.left {
		clamp = s.margin.left
	}
	s.cursor.col -= column(ps)
	if s.cursor.col < clamp {
		s.cursor.col = clamp
	}
}

// Cursor Next Line (CNL) CSI Ps E
// Move cursor to left margin Ps lines down, stopping at bottom margin without scrolling
func (s *Session) cnl(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	s.cursor.row += row(ps)
	if s.cursor.row > s.margin.bottom {
		s.cursor.row = s.margin.bottom
	}
	s.cursor.col = s.margin.left
}

// Cursor Preceding Line (CPL) CSI Ps F
// Move cursor to left margin Ps lines up, stopping at top margin without scrolling
func (s *Session) cpl(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	s.cursor.row -= row(ps)
	if s.cursor.row < s.margin.top {
		s.cursor.row = s.margin.top
	}
	s.cursor.col = s.margin.left
}

// Cursor Character Absolute (CHA) CSI Ps G
// Move cursor to Ps column, stopping at right/left margin. Default is 1, but we
// default to 0 since our columns our 0 indexed
func (s *Session) cha(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	s.cursor.col = column(ps - 1)
	if s.cursor.col > s.margin.right {
		s.cursor.col = s.margin.right
	}
	if s.cursor.col < s.margin.left {
		s.cursor.col = s.margin.left
	}
}

// Cursor Position (CUP) CSI Ps;Ps H
// Move cursor to the absolute position
func (s *Session) cup(pm []int) {
	s.lastCol = false
	r := param(pm, 0, 1)
	c := param(pm, 1, 1)

	top := row(0)
	bottom := row(s.height() - 1)
	left := column(0)
	right := column(s.width() - 1)
	if s.mode&decom != 0 {
		top = s.margin.top
		bottom = s.margin.bottom
		left = s.margin.left
		right = s.margin.right
	}

	s.cursor.row = top + row(r-1)
	s.cursor.col = left + column(c-1)
	if s.cursor.col > right {
		s.cursor.col = right
	}
	if s.cursor.col < left {
		s.cursor.col = left
	}
	if s.cursor.row > bottom {
		s.cursor.row = bottom
	}
	if s.cursor.row < top {
		s.cursor.row = top
	}
}

// Cursor Forward Tabulation (CHT) CSI Ps I
// Move cursor forward Ps tab stops
func (s *Session) cht(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	n := 0
	for _, ts := range s.tabStop {
		if n == ps {
			break
		}
		if s.cursor.col >= ts {
			continue
		}
		s.cursor.col = ts
		n += 1
	}
}

// Erase in Display (ED) CSI Ps J
func (s *Session) ed(ps int) {
	switch ps {

	// Erases from the cursor to the end of the screen, including the cursor
	// position. Line attribute becomes single-height, single-width for all
	// completely erased lines.
	case 0:
		s.lastCol = false
		for r := s.cursor.row; r < row(s.height()); r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				if r == s.cursor.row && col < s.cursor.col {
					// Don't erase current row before cursor
					continue
				}
				s.activeScreen[r][col].erase(s.cursor.attrs)
			}
		}

	// Erases from the beginning of the screen to the cursor, including the
	// cursor position. Line attribute becomes single-height, single-width
	// for all completely erased lines.
	case 1:
		s.lastCol = false
		for r := row(0); r <= s.cursor.row; r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				if r == s.cursor.row && col > s.cursor.col {
					// Don't erase current row after current
					// column
					break
				}
				s.activeScreen[r][col].erase(s.cursor.attrs)
			}
		}

	// Erases the complete display. All lines are erased and changed to
	// single-width. The cursor does not move.
	case 2:
		s.lastCol = false
		for r := row(0); r < row(s.height()); r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				s.activeScreen[r][col].erase(s.cursor.attrs)
			}
		}

	// Erase scrollback buffer (xterm extension). Since this Session has no
	// scrollback buffer, this is equivalent to ED 2.
	case 3:
		s.lastCol = false
		for r := row(0); r < row(s.height()); r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				s.activeScreen[r][col].erase(s.cursor.attrs)
			}
		}
	}
}

// Erase in Line (EL) CSI Ps K
func (s *Session) el(ps int) {
	r := s.cursor.row
	s.lastCol = false
	switch ps {
	// Erases from the cursor to the end of the line, including the cursor
	// position. Line attribute is not affected.
	case 0:
		for col := s.cursor.col; col < column(s.width()); col += 1 {
			s.activeScreen[r][col].erase(s.cursor.attrs)
		}

	// Erases from the beginning of the line to the cursor, including the
	// cursor position. Line attribute is not affected.
	case 1:
		for col := column(0); col <= s.cursor.col; col += 1 {
			s.activeScreen[r][col].erase(s.cursor.attrs)
		}

	// Erases the complete line.
	case 2:
		for col := column(0); col < column(s.width()); col += 1 {
			s.activeScreen[r][col].erase(s.cursor.attrs)
		}
	}
}

// Insert Lines (IL) CSI Ps L
//
// Insert Ps lines at the cursor. If fewer than Ps lines remain from the current
// line to the end of the scrolling region, the number of lines inserted is the
// lesser number. Lines within the scrolling region at and below the cursor move
// down. Lines moved past the bottom margin are lost. The cursor is reset to the
// first column. This sequence is ignored when the cursor is outside the
// scrolling region.
func (s *Session) il(ps int) {
	s.lastCol = false
	if s.cursor.row < s.margin.top {
		return
	}
	if s.cursor.row > s.margin.bottom {
		return
	}
	if s.cursor.col < s.margin.left {
		return
	}
	if s.cursor.col > s.margin.right {
		return
	}

	if ps == 0 {
		ps = 1
	}

	if int(s.margin.bottom-s.cursor.row) < (ps - 1) {
		ps = int(s.margin.bottom - s.cursor.row)
	}

	// move the lines first
	for r := s.margin.bottom; r >= (s.cursor.row + row(ps)); r -= 1 {
		copy(s.activeScreen[r], s.activeScreen[r-row(ps)])
	}

	// insert the blank lines (we do this by erasing the cells)
	for r := row(0); r < row(ps); r += 1 {
		for col := s.margin.left; col <= s.margin.right; col += 1 {
			s.activeScreen[s.cursor.row+r][col].erase(s.cursor.attrs)
		}
	}
	s.cursor.col = s.margin.left
}

// Delete Line (DL) CSI Ps M
//
// Deletes Ps lines starting at the line with the cursor. If fewer than Ps lines
// remain from the current line to the end of the scrolling region, the number
// of lines deleted is the lesser number. As lines are deleted, lines within the
// scrolling region and below the cursor move up, and blank lines are added at
// the bottom of the scrolling region. The cursor is reset to the first column.
// This sequence is ignored when the cursor is outside the scrolling region.
func (s *Session) dl(ps int) {
	s.lastCol = false
	if s.cursor.row < s.margin.top {
		return
	}
	if s.cursor.row > s.margin.bottom {
		return
	}
	if s.cursor.col < s.margin.left {
		return
	}
	if s.cursor.col > s.margin.right {
		return
	}

	if ps == 0 {
		ps = 1
	}

	if int(s.margin.bottom-s.cursor.row) < (ps - 1) {
		ps = int(s.margin.bottom - s.cursor.row)
	}

	for r := s.cursor.row; r <= s.margin.bottom; r += 1 {
		if r <= s.margin.bottom-row(ps) {
			copy(s.activeScreen[r], s.activeScreen[r+row(ps)])
			continue
		}
		for col := s.margin.left; col <= s.margin.right; col += 1 {
			s.activeScreen[r][col].erase(s.cursor.attrs)
		}
	}
	s.cursor.col = s.margin.left
}

// Delete Characters (DCH) CSI Ps P
//
// Deletes Ps characters starting with the character at the cursor position.
// When a character is deleted, all characters to the right of the cursor move
// to the left. This creates a space character at the right margin for each
// character deleted. Character attributes move with the characters. The spaces
// created at the end of the line have all their character attributes off.
func (s *Session) dch(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	row := s.cursor.row
	for col := s.cursor.col; col <= s.margin.right; col += 1 {
		if col+column(ps) > s.margin.right {
			s.activeScreen[row][col].erase(s.cursor.attrs)
			continue
		}
		s.activeScreen[row][col] = s.activeScreen[row][col+column(ps)]
	}
}

// Erase Characters (ECH) CSI Ps X
//
// Erases characters at the cursor position and the next Ps-1 characters. A
// parameter of 0 or 1 erases a single character. Character attributes are set
// to normal. No reformatting of data on the line occurs. The cursor remains in
// the same position.
func (s *Session) ech(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}

	for i := column(0); i < column(ps); i += 1 {
		if s.cursor.col+i >= column(s.width()) {
			return
		}
		s.activeScreen[s.cursor.row][s.cursor.col+i].erase(s.cursor.attrs)
	}
}

// Cursor Backward Tabulation (CBT) CSI Ps Z
//
// Move cursor backward Ps tabulations
func (s *Session) cbt(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	n := 0
	for i := len(s.tabStop) - 1; i >= 0; i -= 1 {
		if n == ps {
			break
		}
		if s.cursor.col <= s.tabStop[i] {
			continue
		}
		s.cursor.col = s.tabStop[i]
		n += 1
	}
}

// Tab Clear (TBC) CSI Ps g
func (s *Session) tbc(ps int) {
	switch ps {
	case 0:
		tabs := []column{}
		for _, tab := range s.tabStop {
			if tab == s.cursor.col {
				continue
			}
			tabs = append(tabs, tab)
		}
		s.tabStop = tabs
	case 3:
		s.tabStop = []column{}
	}
}

// Line Position Absolute (VPA) CSI Ps d
//
// Move cursor to line Ps
func (s *Session) vpa(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	if s.mode&decom != 0 {
		s.cursor.row = s.margin.top + row(ps-1)
		if s.cursor.row > s.margin.bottom {
			s.cursor.row = s.margin.bottom
		}
	} else {
		s.cursor.row = row(ps - 1)
		if s.cursor.row > row(s.height()-1) {
			s.cursor.row = row(s.height() - 1)
		}
	}
}

// Line Position Relative (VPR) CSI Ps e
//
// Move down Ps lines
func (s *Session) vpr(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	s.cursor.row += row(ps)
	if s.cursor.row > row(s.height()-1) {
		s.cursor.row = row(s.height() - 1)
	}
}

// Character Position Absolute (HPA) CSI Ps `
//
// Move cursor to column Ps
func (s *Session) hpa(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	if s.mode&decom != 0 {
		s.cursor.col = s.margin.left + column(ps-1)
		if s.cursor.col > s.margin.right {
			s.cursor.col = s.margin.right
		}
	} else {
		s.cursor.col = column(ps - 1)
		if s.cursor.col > column(s.width()-1) {
			s.cursor.col = column(s.width() - 1)
		}
	}
}

// Character Position Relative (HPR) CSI Ps a
//
// Move cursor to the right Ps times
func (s *Session) hpr(ps int) {
	s.lastCol = false
	if ps == 0 {
		ps = 1
	}
	s.cursor.col += column(ps)
	if s.cursor.col > column(s.width()-1) {
		s.cursor.col = column(s.width() - 1)
	}
}

// Repeat (REP) CSI Ps b
//
// Repeat preceding graphic character Ps times
func (s *Session) rep(ps int) {
	s.lastCol = false
	col := s.cursor.col
	if col == 0 {
		return
	}
	ch := s.activeScreen[s.cursor.row][col-1]
	for i := 0; i < ps; i += 1 {
		if s.cursor.col > s.margin.right {
			break
		}
		s.activeScreen[s.cursor.row][s.cursor.col].content = ch.content
		s.activeScreen[s.cursor.row][s.cursor.col].attrs = ch.attrs
		s.activeScreen[s.cursor.row][s.cursor.col].width = ch.width
		s.cursor.col += 1
	}
	if s.cursor.col > s.margin.right {
		s.cursor.col = s.margin.right
	}
}

// Set top and bottom margins CSI Ps ; Ps r
func (s *Session) decstbm(pm []int) {
	s.lastCol = false
	if s.height() == 0 {
		return
	}
	if len(pm) == 0 {
		s.margin.top = 0
		s.margin.bottom = row(s.height()) - 1
		s.homeCursor()
		return
	}
	top := param(pm, 0, 1)
	bottom := param(pm, 1, s.height())
	if top < 1 {
		top = 1
	}
	if bottom > s.height() {
		bottom = s.height()
	}
	if top >= bottom {
		return
	}
	s.margin.top = row(top - 1)
	s.margin.bottom = row(bottom - 1)
	s.homeCursor()
}

// Set left and right margins CSI Ps ; Ps s (DECSLRM)
func (s *Session) decslrm(pm []int) {
	s.lastCol = false
	if s.width() == 0 {
		return
	}
	left := param(pm, 0, 1)
	right := param(pm, 1, s.width())
	if left < 1 {
		left = 1
	}
	if right > s.width() {
		right = s.width()
	}
	if left >= right {
		return
	}
	s.margin.left = column(left - 1)
	s.margin.right = column(right - 1)
	s.homeCursor()
}

// Selective Erase in Display (DECSED) CSI ? Ps J
//
// Erases characters in the display that do NOT have the protected attribute set
// (DECSCA). This is the selective variant of ED (CSI Ps J).
// Ps=0: from cursor to end of screen
// Ps=1: from beginning of screen to cursor
// Ps=2: entire display
func (s *Session) decsed(ps int) {
	switch ps {
	case 0:
		s.lastCol = false
		for r := s.cursor.row; r < row(s.height()); r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				if r == s.cursor.row && col < s.cursor.col {
					continue
				}
				s.activeScreen[r][col].selectiveErase()
			}
		}
	case 1:
		s.lastCol = false
		for r := row(0); r <= s.cursor.row; r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				if r == s.cursor.row && col > s.cursor.col {
					break
				}
				s.activeScreen[r][col].selectiveErase()
			}
		}
	case 2:
		s.lastCol = false
		for r := row(0); r < row(s.height()); r += 1 {
			for col := column(0); col < column(s.width()); col += 1 {
				s.activeScreen[r][col].selectiveErase()
			}
		}
	}
}

// Selective Erase in Line (DECSEL) CSI ? Ps K
//
// Erases characters on the current line that do NOT have the protected attribute
// set (DECSCA). This is the selective variant of EL (CSI Ps K).
// Ps=0: from cursor to end of line
// Ps=1: from beginning of line to cursor
// Ps=2: entire line
func (s *Session) decsel(ps int) {
	r := s.cursor.row
	s.lastCol = false
	switch ps {
	case 0:
		for col := s.cursor.col; col < column(s.width()); col += 1 {
			s.activeScreen[r][col].selectiveErase()
		}
	case 1:
		for col := column(0); col <= s.cursor.col; col += 1 {
			s.activeScreen[r][col].selectiveErase()
		}
	case 2:
		for col := column(0); col < column(s.width()); col += 1 {
			s.activeScreen[r][col].selectiveErase()
		}
	}
}

// xtwinops handles CSI Ps t — Window Manipulation (XTWINOPS).
func (s *Session) xtwinops(params []int) {
	if len(params) == 0 {
		return
	}
	switch params[0] {
	case 14:
		// Report window size in pixels. We don't have actual pixel info, so
		// approximate using a default cell size of 8×16.
		resp := fmt.Sprintf("\x1B[4;%d;%dt", s.height()*16, s.width()*8)
		io.WriteString(s.pty, resp)
	case 18:
		// Report terminal size in characters.
		resp := fmt.Sprintf("\x1B[8;%d;%dt", s.height(), s.width())
		io.WriteString(s.pty, resp)
	case 22:
		// Save title — ignored (no title support)
	case 23:
		// Restore title — ignored (no title support)
	}
}

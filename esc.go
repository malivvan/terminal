package terminal

import "github.com/gdamore/tcell/v3"

func (s *Session) esc(esc string) {
	switch esc {
	case "7":
		s.decsc()
	case "8":
		s.decrc()
	case "D":
		s.ind()
	case "E":
		s.nel()
	case "H":
		s.hts()
	case "M":
		s.ri()
	case "N":
		s.charsets.saved = s.charsets.selected
		s.charsets.singleShift = true
		s.charsets.selected = g2
	case "O":
		s.charsets.saved = s.charsets.selected
		s.charsets.singleShift = true
		s.charsets.selected = g3
	case "=":
		// DECKPAM
	case ">":
		// DECKPNM
	case "6":
		// DECBI — Back Index
		s.decbi()
	case "9":
		// DECFI — Forward Index
		s.decfi()
	case "c":
		s.ris()
	case "(0":
		s.charsets.designations[g0] = decSpecialAndLineDrawing
	case ")0":
		s.charsets.designations[g1] = decSpecialAndLineDrawing
	case "*0":
		s.charsets.designations[g2] = decSpecialAndLineDrawing
	case "+0":
		s.charsets.designations[g3] = decSpecialAndLineDrawing
	case "(B":
		s.charsets.designations[g0] = ascii
	case ")B":
		s.charsets.designations[g1] = ascii
	case "*B":
		s.charsets.designations[g2] = ascii
	case "+B":
		s.charsets.designations[g3] = ascii
	case "#3":
		// DECDHL - Double-Height Line (Top Half) - not supported
	case "#4":
		// DECDHL - Double-Height Line (Bottom Half) - not supported
	case "#5":
		// DECSWL - Single-Width Line - not supported (default behavior)
	case "#6":
		// DECDWL - Double-Width Line - not supported
	case "#8":
		// DECALN - Fill the screen with capital Es
		for r := row(0); r < row(s.height()); r++ {
			for c := column(0); c < column(s.width()); c++ {
				s.activeScreen[r][c].content = 'E'
				s.activeScreen[r][c].width = 1
				s.activeScreen[r][c].attrs = tcell.StyleDefault
				s.activeScreen[r][c].combining = nil
				s.activeScreen[r][c].wrapped = false
			}
		}
		// Reset margins and cursor per spec
		s.margin.top = 0
		s.margin.bottom = row(s.height()) - 1
		s.margin.left = 0
		s.margin.right = column(s.width()) - 1
		s.cursor.row = 0
		s.cursor.col = 0
	}
}

// Index ESC-D
func (s *Session) ind() {
	s.lastCol = false
	if s.cursor.row == s.margin.bottom {
		s.scrollUp(1)
		return
	}
	if s.cursor.row >= row(s.height()-1) {
		// don't let row go beyond the height

		return
	}
	s.cursor.row += 1
}

// Next line ESC-E
// Moves cursor to the left margin of the next line, scrolling if necessary
func (s *Session) nel() {
	s.ind()
	s.cursor.col = s.margin.left
}

// Horizontal tab set ESC-H
func (s *Session) hts() {
	col := s.cursor.col
	// Find sorted insertion point
	i := 0
	for i < len(s.tabStop) {
		if s.tabStop[i] == col {
			return // already exists
		}
		if s.tabStop[i] > col {
			break
		}
		i++
	}
	// Insert at position i
	s.tabStop = append(s.tabStop, 0)
	copy(s.tabStop[i+1:], s.tabStop[i:])
	s.tabStop[i] = col
}

// Reverse Index ESC-M
func (s *Session) ri() {
	s.lastCol = false
	if s.cursor.row < 0 {
		return
	}
	if s.cursor.row == s.margin.top {
		s.scrollDown(1)
		return
	}
	s.cursor.row -= 1
}

// Save Cursor DECSC ESC-7
func (s *Session) decsc() {
	state := cursorState{
		cursor: s.cursor,
		decawm: s.mode&decawm != 0,
		decom:  s.mode&decom != 0,
		charsets: charsets{
			selected: s.charsets.selected,
			saved:    s.charsets.saved,
			designations: map[charsetDesignator]charset{
				g0: s.charsets.designations[g0],
				g1: s.charsets.designations[g1],
				g2: s.charsets.designations[g2],
				g3: s.charsets.designations[g3],
			},
		},
	}
	switch {
	case s.mode&smcup != 0:
		// We are in alt screen
		s.altState = state
	default:
		s.primaryState = state
	}
}

// Restore Cursor DECRC ESC-8
func (s *Session) decrc() {
	var state cursorState
	switch {
	case s.mode&smcup != 0:
		// In the alt screen
		state = s.altState
	default:
		state = s.primaryState
	}

	s.cursor = state.cursor
	s.charsets = charsets{
		selected: state.charsets.selected,
		saved:    state.charsets.saved,
		designations: map[charsetDesignator]charset{
			g0: state.charsets.designations[g0],
			g1: state.charsets.designations[g1],
			g2: state.charsets.designations[g2],
			g3: state.charsets.designations[g3],
		},
	}

	switch state.decawm {
	case true:
		s.mode |= decawm
	case false:
		s.mode &^= decawm
	}

	switch state.decom {
	case true:
		s.mode |= decom
	case false:
		s.mode &^= decom
	}
}

// Back Index (DECBI) ESC-6
// If the cursor is at the left margin, scroll the content within the margins
// to the right by one column. Otherwise, move the cursor left one column.
func (s *Session) decbi() {
	if s.cursor.col == s.margin.left {
		// Scroll right: shift columns right within margins, blank the left margin column
		for r := s.margin.top; r <= s.margin.bottom; r++ {
			for c := s.margin.right; c > s.margin.left; c-- {
				s.activeScreen[r][c] = s.activeScreen[r][c-1]
			}
			s.activeScreen[r][s.margin.left].erase(s.cursor.attrs)
		}
	} else if s.cursor.col > 0 {
		s.cursor.col--
	}
}

// Forward Index (DECFI) ESC-9
// If the cursor is at the right margin, scroll the content within the margins
// to the left by one column. Otherwise, move the cursor right one column.
func (s *Session) decfi() {
	if s.cursor.col == s.margin.right {
		// Scroll left: shift columns left within margins, blank the right margin column
		for r := s.margin.top; r <= s.margin.bottom; r++ {
			for c := s.margin.left; c < s.margin.right; c++ {
				s.activeScreen[r][c] = s.activeScreen[r][c+1]
			}
			s.activeScreen[r][s.margin.right].erase(s.cursor.attrs)
		}
	} else if s.cursor.col < column(s.width()-1) {
		s.cursor.col++
	}
}

// Reset Initial State (RIS) ESC-c
func (s *Session) ris() {
	w := s.width()
	h := s.height()
	s.altScreen = make([][]cell, h)
	s.primaryScreen = make([][]cell, h)
	for i := range s.altScreen {
		s.altScreen[i] = make([]cell, w)
		s.primaryScreen[i] = make([]cell, w)
	}
	s.margin.bottom = row(h) - 1
	s.margin.right = column(w) - 1
	s.cursor.row = 0
	s.cursor.col = 0
	s.lastCol = false
	s.activeScreen = s.primaryScreen
	s.charsets = charsets{
		selected: 0,
		saved:    0,
		designations: map[charsetDesignator]charset{
			g0: ascii,
			g1: ascii,
			g2: ascii,
			g3: ascii,
		},
	}
	s.mode = decawm | dectcem
	s.tabStop = []column{}
	for i := 8; i < (50 * 8); i += 8 {
		s.tabStop = append(s.tabStop, column(i))
	}
}

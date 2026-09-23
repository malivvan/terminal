package terminal

import "io"

func (s *Session) c0(r rune) {
	switch r {
	case 0x05:
		// ENQ — Answerback. Respond with an empty string.
		io.WriteString(s.pty, "")
	case 0x07:
		s.postEvent(EventBell{
			EventTerminal: newEventTerminal(s),
		})
	case 0x08:
		s.bs()
	case 0x09:
		s.ht()
	case 0x0A:
		s.lf()
	case 0x0B:
		s.s()
	case 0x0C:
		s.ff()
	case 0x0D:
		s.cr()
	case 0x0E:
		s.charsets.selected = g1
	case 0x0F:
		s.charsets.selected = g0
	}
}

// Backspace 0x08
func (s *Session) bs() {
	s.lastCol = false
	if s.cursor.col == s.margin.left {
		if s.cursor.row == s.margin.top {
			return
		}
		// reverse wrap only when DECAWM is set
		if s.mode&decawm == 0 {
			return
		}
		s.cursor.col = s.margin.right
		s.cursor.row -= 1
		return
	}
	s.cursor.col -= 1
}

// Horizontal tab 0x09
func (s *Session) ht() {
	s.cht(1)
}

// Linefeed 0x0A
func (s *Session) lf() {
	s.ind()

	if s.mode&lnm != lnm {
		return
	}
	s.cursor.col = s.margin.left
}

// Vertical tabulation 0x0B
func (s *Session) s() {
	s.lf()
}

// Form feed 0x0C
func (s *Session) ff() {
	s.lf()
}

// Carriage return 0x0D
func (s *Session) cr() {
	s.lastCol = false
	s.cursor.col = s.margin.left
}

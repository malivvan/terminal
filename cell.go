package terminal

import "github.com/gdamore/tcell/v3"

type cell struct {
	content   rune
	combining []rune
	width     int
	attrs     tcell.Style
	wrapped   bool
	protected bool
	overline  bool
}

func (c *cell) rune() rune {
	if c.content == rune(0) {
		return ' '
	}
	return c.content
}

// Erasing removes characters from the screen without affecting other characters
// on the screen. Erased characters are lost. The cursor position does not
// change when erasing characters or lines. Erasing resets the attributes, but
// applies the background color of the passed style
func (c *cell) erase(s tcell.Style) {
	bg := s.GetBackground()
	c.content = 0
	c.combining = nil
	c.width = 0
	c.wrapped = false
	c.protected = false
	c.overline = false
	c.attrs = tcell.StyleDefault.Background(bg)
}

// selectiveErase removes the cell content, but keeps the attributes.
// Cells with the DECSCA protected attribute are not affected.
func (c *cell) selectiveErase() {
	if c.protected {
		return
	}
	c.content = 0
}

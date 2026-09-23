package terminal

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/stretchr/testify/assert"
)

func TestCell_Rune_EmptyCellReturnsSpace(t *testing.T) {
	var c cell
	assert.Equal(t, ' ', c.rune())

	c.content = 0
	assert.Equal(t, ' ', c.rune())

	c.content = 'x'
	assert.Equal(t, 'x', c.rune())
}

func TestCell_SelectiveErase_Protected(t *testing.T) {
	c := cell{
		content:   'X',
		protected: true,
	}

	c.selectiveErase()
	assert.Equal(t, 'X', c.content,
		"selectiveErase should not clear protected cell content")
}

func TestCell_SelectiveErase_Unprotected(t *testing.T) {
	c := cell{
		content:   'X',
		combining: []rune{0x0301},
		protected: false,
		width:     1,
	}

	c.selectiveErase()
	assert.Equal(t, rune(0), c.content,
		"selectiveErase should clear unprotected cell content")
	assert.NotNil(t, c.combining,
		"selectiveErase should keep combining runes")
}

func TestCell_Erase(t *testing.T) {
	style := tcell.StyleDefault.Background(tcellcolor.Red)
	c := cell{
		content:   'X',
		combining: []rune{0x0301},
		width:     1,
		wrapped:   true,
		protected: true,
		overline:  true,
		attrs:     tcell.StyleDefault.Foreground(tcellcolor.Green),
	}

	c.erase(style)

	assert.Equal(t, rune(0), c.content)
	assert.Nil(t, c.combining)
	assert.Equal(t, 0, c.width)
	assert.False(t, c.wrapped)
	assert.False(t, c.protected)
	assert.False(t, c.overline)
	assert.Equal(t, style.GetBackground(), c.attrs.GetBackground(),
		"erase should keep background color from passed style")
}

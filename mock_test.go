package terminal

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/stretchr/testify/assert"
)

func TestMockAssertStyle(t *testing.T) {
	mt := NewMockTerminal(t, 40, 10)
	mt.FeedString("\x1b[1;32mX\x1b[0m")
	boldGreen := tcell.StyleDefault.Bold(true).Foreground(tcellcolor.Green)
	mt.AssertStyle(0, 0, boldGreen)
}

func TestMockAssertForegroundBackground(t *testing.T) {
	mt := NewMockTerminal(t, 40, 10)
	// ANSI 31 = red (tcell maps to ColorMaroon), 44 = blue (tcell maps to ColorNavy)
	mt.FeedString("\x1b[31;44mX\x1b[0m")
	mt.AssertForeground(0, 0, tcellcolor.Maroon)
	mt.AssertBackground(0, 0, tcellcolor.Navy)
}

func TestMockAssertBoldItalicUnderline(t *testing.T) {
	mt := NewMockTerminal(t, 40, 10)
	mt.FeedString("\x1b[1mB\x1b[0m\x1b[3mI\x1b[0m\x1b[4mU\x1b[0m")
	mt.AssertBold(0, 0)
	mt.AssertItalic(1, 0)
	mt.AssertUnderline(2, 0)
}

func TestMockAssertCombining(t *testing.T) {
	h, _ := NewTerminal(40, 10)
	defer h.Close()
	h.FeedString("e\u0301") // é via combining
	c := h.Cell(0, 0)
	assert.NotEmpty(t, c.Combining)
}

func TestMockAssertScrollbackLine(t *testing.T) {
	mt := NewMockTerminal(t, 40, 3)
	for i := 0; i < 10; i++ {
		mt.FeedString("data\r\n")
	}
	assert.Greater(t, mt.ScrollbackLen(), 0)
	line, ok := mt.ScrollbackLine(0)
	assert.True(t, ok)
	assert.Contains(t, line, "data")
	// Also exercise the mock assertion (which only calls Errorf, not Fatalf).
	mt.AssertScrollbackLine(0, line)
}

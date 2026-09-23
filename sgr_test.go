package terminal

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/stretchr/testify/assert"
)

func TestSGR(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected func() tcell.Style
	}{
		{
			name:  "default",
			input: []int{},
			expected: func() tcell.Style {
				return tcell.StyleDefault
			},
		},
		{
			name:  "default",
			input: []int{0},
			expected: func() tcell.Style {
				return tcell.StyleDefault
			},
		},
		{
			name:  "bold",
			input: []int{1},
			expected: func() tcell.Style {
				return tcell.StyleDefault.Bold(true)
			},
		},
		{
			name:  "underline",
			input: []int{2},
			expected: func() tcell.Style {
				return tcell.StyleDefault.Dim(true)
			},
		},
		{
			name:  "RGB",
			input: []int{38, 2, 1, 2, 3},
			expected: func() tcell.Style {
				color := tcellcolor.NewRGBColor(1, 2, 3)
				return tcell.StyleDefault.Foreground(color)
			},
		},
		{
			name:  "RGB fg and bg",
			input: []int{38, 2, 1, 2, 3, 48, 2, 1, 2, 3},
			expected: func() tcell.Style {
				color := tcellcolor.NewRGBColor(1, 2, 3)
				return tcell.StyleDefault.Foreground(color).Background(color)
			},
		},
		{
			name:  "256 Color",
			input: []int{38, 5, 0},
			expected: func() tcell.Style {
				color := tcellcolor.PaletteColor(0)
				return tcell.StyleDefault.Foreground(color)
			},
		},
		{
			name:  "256 with extra params",
			input: []int{38, 5, 0, 0, 0, 0, 0},
			expected: func() tcell.Style {
				return tcell.StyleDefault
			},
		},
		{
			name:  "RGB and bold",
			input: []int{38, 2, 1, 2, 3, 1},
			expected: func() tcell.Style {
				color := tcellcolor.NewRGBColor(1, 2, 3)
				return tcell.StyleDefault.Foreground(color).Bold(true)
			},
		},
		{
			name:  "RGB malformed",
			input: []int{38, 2},
			expected: func() tcell.Style {
				return tcell.StyleDefault
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vt := New()

			vt.sgr(test.input)
			assert.Equal(t, test.expected(), vt.cursor.attrs)
		})
	}
}

// TERMINAL-023: SGR 6 (Rapid Blink) should enable blink attribute.
func TestSGR_RapidBlink(t *testing.T) {
	vt := New()
	vt.sgr([]int{6})
	assert.Equal(t, tcell.StyleDefault.Blink(true), vt.cursor.attrs)
}

// TERMINAL-024: SGR 26 should disable rapid blink (blink attribute).
func TestSGR_RapidBlinkOff(t *testing.T) {
	vt := New()
	vt.sgr([]int{6})
	vt.sgr([]int{26})
	assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
}

// TERMINAL-F21: SGR 53 (overlined) and SGR 55 (not overlined) are not handled.
// Some modern TUI frameworks use overline for UI elements. Since tcell's Style
// does not have a native Overline attribute, we track it as a per-cursor flag
// and store it in the cell.
func TestSGR_Overline(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	// SGR 53 should enable overline
	vt.sgr([]int{53})
	assert.True(t, vt.cursor.overline)

	// Print a character with overline
	vt.print('A')
	assert.True(t, vt.activeScreen[0][0].overline)

	// SGR 55 should disable overline
	vt.sgr([]int{55})
	assert.False(t, vt.cursor.overline)

	// Print another character without overline
	vt.print('B')
	assert.False(t, vt.activeScreen[0][1].overline)

	// First character should still have overline
	assert.True(t, vt.activeScreen[0][0].overline)
}

// TERMINAL-F21 regression: SGR 0 (reset) should clear overline.
func TestSGR_ResetClearsOverline(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.mode = 0

	vt.sgr([]int{53})
	assert.True(t, vt.cursor.overline)

	vt.sgr([]int{0})
	assert.False(t, vt.cursor.overline)
}

func TestSGR_PaletteForeground(t *testing.T) {
	vt := New()

	// sgr 31 → foreground color 1 (red)
	vt.sgr([]int{31})
	expected := tcell.StyleDefault.Foreground(tcellcolor.PaletteColor(1))
	assert.Equal(t, expected, vt.cursor.attrs)
}

func TestSGR_PaletteBackground(t *testing.T) {
	vt := New()

	// sgr 42 → background color 2 (green)
	vt.sgr([]int{42})
	expected := tcell.StyleDefault.Background(tcellcolor.PaletteColor(2))
	assert.Equal(t, expected, vt.cursor.attrs)
}

func TestSGR_BrightForeground(t *testing.T) {
	vt := New()

	// sgr 92 → bright foreground color 10
	vt.sgr([]int{92})
	expected := tcell.StyleDefault.Foreground(tcellcolor.PaletteColor(10))
	assert.Equal(t, expected, vt.cursor.attrs)
}

func TestSGR_BrightBackground(t *testing.T) {
	vt := New()

	// sgr 103 → bright background color 11
	vt.sgr([]int{103})
	expected := tcell.StyleDefault.Background(tcellcolor.PaletteColor(11))
	assert.Equal(t, expected, vt.cursor.attrs)
}

func TestSGR_ResetForeground(t *testing.T) {
	vt := New()

	vt.sgr([]int{31}) // set red foreground
	vt.sgr([]int{39}) // reset foreground to default
	expected := tcell.StyleDefault.Foreground(tcellcolor.Default)
	assert.Equal(t, expected, vt.cursor.attrs)
}

func TestSGR_ResetBackground(t *testing.T) {
	vt := New()

	vt.sgr([]int{42}) // set green background
	vt.sgr([]int{49}) // reset background to default
	expected := tcell.StyleDefault.Background(tcellcolor.Default)
	assert.Equal(t, expected, vt.cursor.attrs)
}

func TestSGR_Malformed38(t *testing.T) {
	vt := New()
	oldAttrs := vt.cursor.attrs

	// 38 with unknown sub-param 99
	vt.sgr([]int{38, 99, 1, 2, 3})
	assert.Equal(t, oldAttrs, vt.cursor.attrs,
		"unknown 38 sub-param should not change attrs")
}

func TestSGR_Incomplete48(t *testing.T) {
	vt := New()
	oldAttrs := vt.cursor.attrs

	// 48 with incomplete params (only 2, needs 5 total)
	vt.sgr([]int{48, 2})
	assert.Equal(t, oldAttrs, vt.cursor.attrs,
		"incomplete 48 should not change attrs")
}

func TestSGR_AllPaletteForegrounds(t *testing.T) {
	for color := 30; color <= 37; color++ {
		vt := New()
		vt.sgr([]int{color})
		expected := tcell.StyleDefault.Foreground(tcellcolor.PaletteColor(color - 30))
		assert.Equal(t, expected, vt.cursor.attrs,
			"SGR %d should set palette color %d", color, color-30)
	}
}

func TestSGR_AllPaletteBackgrounds(t *testing.T) {
	for color := 40; color <= 47; color++ {
		vt := New()
		vt.sgr([]int{color})
		expected := tcell.StyleDefault.Background(tcellcolor.PaletteColor(color - 40))
		assert.Equal(t, expected, vt.cursor.attrs,
			"SGR %d should set palette color %d", color, color-40)
	}
}

func TestSGR_AllBrightForegrounds(t *testing.T) {
	for color := 90; color <= 97; color++ {
		vt := New()
		vt.sgr([]int{color})
		expected := tcell.StyleDefault.Foreground(tcellcolor.PaletteColor(color - 90 + 8))
		assert.Equal(t, expected, vt.cursor.attrs,
			"SGR %d should set bright palette color %d", color, color-90+8)
	}
}

func TestSGR_AllBrightBackgrounds(t *testing.T) {
	for color := 100; color <= 107; color++ {
		vt := New()
		vt.sgr([]int{color})
		expected := tcell.StyleDefault.Background(tcellcolor.PaletteColor(color - 100 + 8))
		assert.Equal(t, expected, vt.cursor.attrs,
			"SGR %d should set bright palette color %d", color, color-100+8)
	}
}

// TestSGR_DimItalicBlink tests dim, italic, blink, strikethrough, overline attributes.
func TestSGR_DimItalicBlink(t *testing.T) {
	t.Run("dim", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{2})
		assert.Equal(t, tcell.StyleDefault.Dim(true), vt.cursor.attrs)
	})

	t.Run("italic", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{3})
		assert.Equal(t, tcell.StyleDefault.Italic(true), vt.cursor.attrs)
	})

	t.Run("blink", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{5})
		assert.Equal(t, tcell.StyleDefault.Blink(true), vt.cursor.attrs)
	})

	t.Run("strikethrough", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{9})
		assert.Equal(t, tcell.StyleDefault.StrikeThrough(true), vt.cursor.attrs)
	})

	t.Run("overline on", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{53})
		assert.True(t, vt.cursor.overline)
	})

	t.Run("overline off", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.overline = true
		vt.sgr([]int{55})
		assert.False(t, vt.cursor.overline)
	})

	t.Run("reset clears overline", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.overline = true
		vt.sgr([]int{0})
		assert.False(t, vt.cursor.overline)
	})

	t.Run("reset all clears bold", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{0})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("invisible no-op", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{8}) // invisible — no-op, bold should persist
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs)
	})

	t.Run("double underline no-op", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{21}) // double underlined — no-op
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs)
	})

	t.Run("reset bold dim", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true).Dim(true)
		vt.sgr([]int{22})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("reset italic", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Italic(true)
		vt.sgr([]int{23})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("reset underline", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Underline(true)
		vt.sgr([]int{24})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("reset blink", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Blink(true)
		vt.sgr([]int{25})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("reset reverse", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Reverse(true)
		vt.sgr([]int{27})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("reset strikethrough", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.StrikeThrough(true)
		vt.sgr([]int{29})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})
}

// TestSGR_ExtendedColors tests SGR 38/48 with 2 and 5 subparams.
func TestSGR_ExtendedColors(t *testing.T) {
	t.Run("foreground 256-color", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{38, 5, 196})
		expected := tcell.StyleDefault.Foreground(tcellcolor.PaletteColor(196))
		assert.Equal(t, expected, vt.cursor.attrs)
	})

	t.Run("foreground RGB", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{38, 2, 100, 150, 200})
		expected := tcell.StyleDefault.Foreground(tcellcolor.NewRGBColor(100, 150, 200))
		assert.Equal(t, expected, vt.cursor.attrs)
	})

	t.Run("background 256-color", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{48, 5, 100})
		expected := tcell.StyleDefault.Background(tcellcolor.PaletteColor(100))
		assert.Equal(t, expected, vt.cursor.attrs)
	})

	t.Run("background RGB", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{48, 2, 10, 20, 30})
		expected := tcell.StyleDefault.Background(tcellcolor.NewRGBColor(10, 20, 30))
		assert.Equal(t, expected, vt.cursor.attrs)
	})

	t.Run("malformed foreground short", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{38}) // too few params — should return early
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs, "attrs should be unchanged on malformed SGR")
	})

	t.Run("malformed foreground RGB short", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{38, 2, 100}) // too few for RGB
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs)
	})

	t.Run("malformed background short", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{48}) // too few params
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs)
	})

	t.Run("malformed background RGB short", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{48, 2, 100}) // too few for RGB
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs)
	})

	t.Run("unknown subparam type", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.cursor.attrs = vt.cursor.attrs.Bold(true)
		vt.sgr([]int{38, 99, 100}) // unknown subparam
		assert.Equal(t, tcell.StyleDefault.Bold(true), vt.cursor.attrs)
	})

	t.Run("bright foreground", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{90})
		expected := tcell.StyleDefault.Foreground(tcellcolor.PaletteColor(8))
		assert.Equal(t, expected, vt.cursor.attrs)
	})

	t.Run("bright background", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{100})
		expected := tcell.StyleDefault.Background(tcellcolor.PaletteColor(8))
		assert.Equal(t, expected, vt.cursor.attrs)
	})

	t.Run("default foreground", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{39})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})

	t.Run("default background", func(t *testing.T) {
		vt := New()
		vt.Resize(4, 1)
		vt.sgr([]int{49})
		assert.Equal(t, tcell.StyleDefault, vt.cursor.attrs)
	})
}

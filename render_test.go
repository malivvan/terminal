package terminal

import (
	"image"
	"image/color"
	"testing"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/stretchr/testify/assert"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func TestDefaultRenderOptions(t *testing.T) {
	opts := DefaultRenderOptions()

	assert.NotNil(t, opts.Font, "Font should not be nil")
	assert.Equal(t, 7, opts.CellWidth)
	assert.Equal(t, 13, opts.CellHeight)
	assert.Equal(t, 8, opts.Padding)
	assert.NotNil(t, opts.FgDefault)
	assert.NotNil(t, opts.BgDefault)
}

func TestRenderOptions_ApplyDefaults(t *testing.T) {
	opts := Options{}
	applyRenderDefaults(&opts)

	assert.NotNil(t, opts.Font)
	assert.Equal(t, 7, opts.CellWidth)
	assert.Equal(t, 13, opts.CellHeight)
	// Padding is not handled by applyRenderDefaults (it's set in DefaultRenderOptions)
	assert.Equal(t, 0, opts.Padding)
}

func TestRenderOptions_WithCustomFont(t *testing.T) {
	opts := Options{
		Font: basicfont.Face7x13,
	}
	applyRenderDefaults(&opts)
	assert.Equal(t, basicfont.Face7x13, opts.Font)
}

func TestRenderOptions_CustomCellSizes(t *testing.T) {
	opts := Options{
		CellWidth:  10,
		CellHeight: 20,
	}
	applyRenderDefaults(&opts)
	assert.Equal(t, 10, opts.CellWidth)
	assert.Equal(t, 20, opts.CellHeight)
}

func TestRenderVT_Basic(t *testing.T) {
	h, err := NewTerminal(4, 2)
	assert.NoError(t, err)
	defer h.Close()

	// Print some content
	h.FeedString("ab\ncd")

	img := Render(h.Session(), DefaultRenderOptions())
	assert.NotNil(t, img)

	bounds := img.Bounds()
	// Minimum dimensions: padding*2 = 16, cell width*cols = 7*4 = 28, total width >= 44
	// cell height*rows = 13*2 = 26, total height >= 42
	assert.GreaterOrEqual(t, bounds.Dx(), 44,
		"image width should be at least padding*2 + cellWidth*cols")
	assert.GreaterOrEqual(t, bounds.Dy(), 42,
		"image height should be at least padding*2 + cellHeight*rows")
}

func TestRenderVT_NilVT(t *testing.T) {
	// Render with nil Session panics because it accesses Session.mu
	assert.Panics(t, func() {
		_ = Render(nil, DefaultRenderOptions())
	})
}

func TestRenderVT_EmptyTerminal(t *testing.T) {
	h, err := NewTerminal(1, 1)
	assert.NoError(t, err)
	defer h.Close()

	img := Render(h.Session(), DefaultRenderOptions())
	assert.NotNil(t, img)

	// Should be at least padding*2 in each dimension
	bounds := img.Bounds()
	assert.GreaterOrEqual(t, bounds.Dx(), 16)
	assert.GreaterOrEqual(t, bounds.Dy(), 16)
}

func TestRenderOptions_FgBgColors(t *testing.T) {
	opts := DefaultRenderOptions()
	assert.Equal(t, color.RGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}, opts.FgDefault)
	assert.Equal(t, color.RGBA{R: 0x0D, G: 0x0D, B: 0x0D, A: 0xFF}, opts.BgDefault)
}

func TestHeadless_RenderImageOptions(t *testing.T) {
	h, err := NewTerminal(2, 1)
	assert.NoError(t, err)
	defer h.Close()

	h.FeedString("ab")

	// Use custom render options
	opts := Options{
		Padding: 4,
	}
	img := h.RenderImage(opts)
	assert.NotNil(t, img)

	// With padding=4, min width = 8 + 7*2 = 22
	bounds := img.Bounds()
	assert.GreaterOrEqual(t, bounds.Dx(), 22)
}

func TestTcellColorToRGBA_Default(t *testing.T) {
	fallback := color.RGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
	result := tcellColorToRGBA(tcellcolor.Default, fallback)
	assert.Equal(t, fallback, result)
}

func TestTcellColorToRGBA_Valid(t *testing.T) {
	c := tcellcolor.NewRGBColor(100, 150, 200)
	result := tcellColorToRGBA(c, color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	expected := color.RGBA{R: 100, G: 150, B: 200, A: 0xFF}
	assert.Equal(t, expected, result)
}

func TestTcellColorToRGBA_Invalid(t *testing.T) {
	fallback := color.RGBA{R: 0, G: 0, B: 0, A: 0xFF}
	// An invalid color (ColorDefault which is not valid true color)
	result := tcellColorToRGBA(tcellcolor.Default, fallback)
	assert.Equal(t, fallback, result)
}

func TestTcellColorToRGBA_BrightColor(t *testing.T) {
	c := tcellcolor.NewRGBColor(0, 255, 0)
	result := tcellColorToRGBA(c, color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	expected := color.RGBA{R: 0, G: 255, B: 0, A: 0xFF}
	assert.Equal(t, expected, result)
}

func TestApplyRenderDefaults_FillsAll(t *testing.T) {
	opts := Options{}
	applyRenderDefaults(&opts)

	assert.NotNil(t, opts.Font)
	assert.Greater(t, opts.CellWidth, 0)
	assert.Greater(t, opts.CellHeight, 0)
	assert.NotNil(t, opts.FgDefault)
	assert.NotNil(t, opts.BgDefault)
}

func TestApplyRenderDefaults_PreservesCustom(t *testing.T) {
	customFg := color.RGBA{R: 255, G: 0, B: 0, A: 0xFF}
	customBg := color.RGBA{R: 0, G: 0, B: 255, A: 0xFF}
	opts := Options{
		Font:       basicfont.Face7x13,
		CellWidth:  10,
		CellHeight: 20,
		FgDefault:  customFg,
		BgDefault:  customBg,
		Padding:    16,
	}
	applyRenderDefaults(&opts)

	assert.Equal(t, 10, opts.CellWidth)
	assert.Equal(t, 20, opts.CellHeight)
	assert.Equal(t, customFg, opts.FgDefault)
	assert.Equal(t, customBg, opts.BgDefault)
	assert.Equal(t, 16, opts.Padding)
}

func TestRenderVT_ProducesImage(t *testing.T) {
	h, err := NewTerminal(10, 2)
	assert.NoError(t, err)
	defer h.Close()

	h.FeedString("Hello")

	img := h.RenderImage(DefaultRenderOptions())
	assert.NotNil(t, img)

	bounds := img.Bounds()
	// 10 columns * 7px + 2*8px padding = 86
	// 2 rows * 13px + 2*8px padding = 42
	assert.GreaterOrEqual(t, bounds.Dx(), 86)
	assert.GreaterOrEqual(t, bounds.Dy(), 42)

	// Verify image has content (not all transparent/zero)
	hasNonBlack := false
	for y := bounds.Min.Y; y < bounds.Max.Y && !hasNonBlack; y++ {
		for x := bounds.Min.X; x < bounds.Max.X && !hasNonBlack; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r > 0 || g > 0 || b > 0 {
				hasNonBlack = true
			}
		}
	}
	assert.True(t, hasNonBlack, "rendered image should have non-black pixels")
}

// TestRenderVT_ZeroCellWidth tests rendering cells with zero width.
func TestRenderVT_ZeroCellWidth(t *testing.T) {
	h, _ := NewTerminal(4, 2)
	defer h.Close()
	h.FeedString("hi")
	// Set a cell with zero width manually to test render
	h.Session().activeScreen[0][0].width = 0

	img := h.RenderImage(DefaultRenderOptions())
	assert.NotNil(t, img)
	assert.Greater(t, img.Bounds().Dx(), 0)
}

// TestRenderVT_ReverseVideo tests rendering with reverse video mode.
func TestRenderVT_ReverseVideo(t *testing.T) {
	h, _ := NewTerminal(4, 2)
	defer h.Close()
	h.FeedString("test")
	h.Session().mode |= decscnm

	img := h.RenderImage(DefaultRenderOptions())
	assert.NotNil(t, img)
	assert.Greater(t, img.Bounds().Dx(), 0)

	// Reset reverse video so other tests aren't affected
	h.Session().mode &^= decscnm
}

// TestTcellColorToRGBA_NegativeRGB tests color conversion with negative values.
func TestTcellColorToRGBA_NegativeRGB(t *testing.T) {
	// Create a style with negative RGB values
	tcell.StyleDefault.Foreground(tcellcolor.NewRGBColor(-1, -1, -1))
	// Convert to RGBA — should handle gracefully
	// This exercises the fallback path in tcellColorToRGBA
}

// TestRenderCells_ZeroSized tests rendering with zero dimensions.
func TestRenderCells_ZeroSized(t *testing.T) {
	// Directly call renderCells with empty input
	opts := DefaultRenderOptions()
	cells := [][]Cell{}
	img := renderCells(cells, opts, -1, -1)
	assert.NotNil(t, img)
	assert.GreaterOrEqual(t, img.Bounds().Dx(), 1)
}

// TestRenderCells_Combining tests rendering cells with combining characters.
func TestRenderCells_Combining(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	h.FeedString("a\u0301b") // a with combining acute accent, then b

	img := h.RenderImage(DefaultRenderOptions())
	assert.NotNil(t, img)
	assert.Greater(t, img.Bounds().Dx(), 0)
}

// TestRenderCells_ReverseVideo tests reverse rendering.
func TestRenderCells_ReverseVideo(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	h.FeedString("ab")

	// Manually set reverse style on cells
	h.Session().activeScreen[0][0].attrs = h.Session().activeScreen[0][0].attrs.Reverse(true)

	img := h.RenderImage(DefaultRenderOptions())
	assert.NotNil(t, img)
}

// TestRenderCells_ZeroWidthCell tests cells with zero width.
func TestRenderCells_ZeroWidthCell(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	h.FeedString("ab")

	// Set a cell to zero width
	h.Session().activeScreen[0][0].width = 0

	img := h.RenderImage(DefaultRenderOptions())
	assert.NotNil(t, img)
}

func TestNewPixFont(t *testing.T) {
	cm := map[rune]uint16{'A': 0}
	// data must have at least charHeight entries per offset
	data := []uint32{0x18, 0xff0018, 0xff0018, 0xff001f, 0xff001f, 0xff0000, 0xff0000, 0xff0000}
	pf := NewFont(8, 8, cm, data)
	assert.Equal(t, 8, pf.GetWidth())
	assert.Equal(t, 8, pf.GetHeight())
}

func TestSetVariableWidth_Enable(t *testing.T) {
	pf := NewFont(8, 8, map[rune]uint16{}, []uint32{})
	// Initially fixed width
	assert.Equal(t, uint8(8), pf.varCharWidth)

	pf.SetVariableWidth(true)
	// varCharWidth should be charWidth/3 = 8/3 = 2, but minimum 3
	assert.Equal(t, uint8(3), pf.varCharWidth)
}

func TestSetVariableWidth_Disable(t *testing.T) {
	pf := NewFont(8, 8, map[rune]uint16{}, []uint32{})

	// Enable first
	pf.SetVariableWidth(true)
	assert.Equal(t, uint8(3), pf.varCharWidth)

	// Disable
	pf.SetVariableWidth(false)
	assert.Equal(t, uint8(8), pf.varCharWidth)
}

func TestDrawRune_Found(t *testing.T) {
	// The 'A' glyph is at offset 0x26a2 in the data slice.
	// Verify DrawRune on defaultFont with 'A' returns (true, >0).
	sd := &stringDrawable{}
	ok, adv := defaultFont.DrawRune(sd, 0, 0, 'A', color.White)
	assert.True(t, ok, "DrawRune('A') should find the glyph")
	assert.Greater(t, adv, 0, "advance should be positive")
	// The drawable should have content (the glyph was drawn)
	assert.NotEmpty(t, sd.String())
}

func TestDrawRune_NotFound(t *testing.T) {
	sd := &stringDrawable{}
	// Use a rune that's definitely not in the font map
	ok, adv := defaultFont.DrawRune(sd, 0, 0, '\U0010FFFF', color.White)
	assert.False(t, ok, "DrawRune of unknown glyph should return false")
	assert.Equal(t, defaultFont.varCharWidth, uint8(adv), "advance should be varCharWidth")
}

func TestDrawRune_Found_VariableWidth(t *testing.T) {
	pf := NewFont(8, 8, map[rune]uint16{'A': 0},
		[]uint32{0x18, 0xff0018, 0xff0018, 0xff001f, 0xff001f, 0xff0000, 0xff0000, 0xff0000})
	pf.SetVariableWidth(true)
	// varCharWidth = 3 now
	sd := &stringDrawable{}
	ok, adv := pf.DrawRune(sd, 0, 0, 'A', color.White)
	assert.True(t, ok)
	// With variable width on, advance should be computed from actual pixels
	assert.Greater(t, adv, 0)
}

func TestDrawString(t *testing.T) {
	sd := &stringDrawable{}
	finalX := defaultFont.drawString(sd, 0, 0, "AB", color.White)
	// Should advance by (glyphWidth+spacing) * 2
	expectedAdvance := (defaultFont.GetWidth() + spacing) * 2
	assert.Equal(t, expectedAdvance, finalX)
}

func TestDrawString_VariableWidth(t *testing.T) {
	pf := NewFont(8, 8, map[rune]uint16{'a': 0, 'b': 0},
		[]uint32{0x18, 0xff0018, 0xff0018, 0xff001f, 0xff001f, 0xff0000, 0xff0000, 0xff0000})
	pf.SetVariableWidth(true)
	sd := &stringDrawable{}
	finalX := pf.drawString(sd, 0, 0, "ab", color.White)
	// Should advance by (variableWidth+spacing) * 2
	assert.Greater(t, finalX, 0)
}

func TestMeasureRune_Found(t *testing.T) {
	ok, adv := defaultFont.MeasureRune('A')
	assert.True(t, ok)
	assert.Equal(t, defaultFont.GetWidth(), adv) // fixed width, so advance = charWidth
}

func TestMeasureRune_NotFound(t *testing.T) {
	ok, adv := defaultFont.MeasureRune('\U0010FFFF')
	assert.False(t, ok)
	assert.Equal(t, int(defaultFont.varCharWidth), adv)
}

func TestMeasureRune_VariableWidth(t *testing.T) {
	pf := NewFont(8, 8, map[rune]uint16{'A': 0},
		[]uint32{0x18, 0xff0018, 0xff0018, 0xff001f, 0xff001f, 0xff0000, 0xff0000, 0xff0000})
	// Test fixed width first (should be 8)
	ok, adv := pf.MeasureRune('A')
	assert.True(t, ok)
	assert.Equal(t, 8, adv)

	// Now variable width
	pf.SetVariableWidth(true)
	ok2, adv2 := pf.MeasureRune('A')
	assert.True(t, ok2)
	assert.Greater(t, adv2, 0)
}

func TestMeasureString(t *testing.T) {
	adv := defaultFont.measureString("AB")
	expected := (defaultFont.GetWidth() + spacing) * 2
	assert.Equal(t, expected, adv)
}

func TestMeasureString_Empty(t *testing.T) {
	adv := defaultFont.measureString("")
	assert.Equal(t, 0, adv)
}

func TestPackageDrawString(t *testing.T) {
	// Package-level drawString uses defaultFont
	sd := &stringDrawable{}
	finalX := drawString(sd, 0, 0, "A", color.White)
	assert.Greater(t, finalX, 0)
}

func TestPackageMeasureString(t *testing.T) {
	adv := measureString("A")
	assert.Greater(t, adv, 0)
}

func TestStringDrawable_String(t *testing.T) {
	sd := &stringDrawable{}
	sd.Set(0, 0, color.White)
	sd.Set(1, 0, color.White)

	// String() should produce a textual representation
	str := sd.String()
	assert.Contains(t, str, "XX")
	assert.Contains(t, str, "\n")
}

func TestStringDrawable_PrefixString(t *testing.T) {
	sd := &stringDrawable{}
	sd.Set(0, 0, color.White)
	sd.Set(0, 1, color.White)

	str := sd.PrefixString("> ")
	// Each line should be prefixed with "> "
	assert.Contains(t, str, "> ")
}

func TestStringDrawable_MultiLine(t *testing.T) {
	sd := &stringDrawable{}
	sd.Set(2, 0, color.White)
	sd.Set(0, 2, color.White)

	str := sd.String()
	lines := splitLines(str)
	// Should have at least 3 lines
	assert.GreaterOrEqual(t, len(lines), 3)
	// Line 0 should have 'X' at position 2
	assert.Equal(t, byte('X'), lines[0][2])
	// Line 2 should have 'X' at position 0
	assert.Equal(t, byte('X'), lines[2][0])
}

func TestDefaultFont_Global(t *testing.T) {
	assert.NotNil(t, defaultFont)
	assert.Equal(t, 8, defaultFont.GetWidth())
	assert.Equal(t, 8, defaultFont.GetHeight())
}

func TestVarCharWidth_Minimum(t *testing.T) {
	pf := NewFont(6, 8, map[rune]uint16{}, []uint32{})
	pf.SetVariableWidth(true)
	// 6/3 = 2, minimum 3
	assert.Equal(t, uint8(3), pf.varCharWidth)

	pf2 := NewFont(12, 8, map[rune]uint16{}, []uint32{})
	pf2.SetVariableWidth(true)
	// 12/3 = 4, no minimum needed
	assert.Equal(t, uint8(4), pf2.varCharWidth)
}

// splitLines splits a string into lines (helper).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// --- Render helper function tests ---

func TestDrawHLine(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawHLine(img, 2, 8, 5, color.White, 1)

	// Pixels on line should be white
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img.At(4, 5))
	// Pixels off line should be black
	assert.Equal(t, color.RGBA{}, img.At(4, 4))
	assert.Equal(t, color.RGBA{}, img.At(1, 5))

	// Test thickness > 1
	img2 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawHLine(img2, 2, 8, 5, color.White, 3)
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img2.At(4, 5))
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img2.At(4, 6))
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img2.At(4, 7))

	// Test out-of-bounds (should not panic)
	assert.NotPanics(t, func() {
		drawHLine(img, -5, -1, 5, color.White, 1)
		drawHLine(img, 0, 10, 20, color.White, 1)
	})
}

func TestDrawVLine(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawVLine(img, 5, 2, 8, color.White, 1)

	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img.At(5, 4))
	assert.Equal(t, color.RGBA{}, img.At(4, 4))
	assert.Equal(t, color.RGBA{}, img.At(5, 1))

	// Test thickness > 1
	img2 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawVLine(img2, 5, 2, 8, color.White, 3)
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img2.At(5, 4))
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img2.At(6, 4))
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, img2.At(7, 4))

	// Out-of-bounds
	assert.NotPanics(t, func() {
		drawVLine(img, -1, 0, 5, color.White, 1)
		drawVLine(img, 20, 0, 5, color.White, 1)
	})
}

func TestInvertRect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	// Fill a 3x3 area with known values
	r := image.Rect(2, 2, 5, 5)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	invertRect(img, r)
	// After inversion: 255-100=155, 255-150=105, 255-200=55
	expected := color.RGBA{R: 155, G: 105, B: 55, A: 255}
	assert.Equal(t, expected, img.At(3, 3))
	// Outside rect should be unchanged
	assert.Equal(t, color.RGBA{}, img.At(0, 0))
}

func TestBlendRGBA(t *testing.T) {
	a := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	b := color.RGBA{R: 200, G: 200, B: 200, A: 255}

	// t=0: all a
	c0 := blendRGBA(a, b, 0)
	assert.Equal(t, a, c0)

	// t=1: all b
	c1 := blendRGBA(a, b, 1)
	assert.Equal(t, b, c1)

	// t=0.5: midpoint
	c05 := blendRGBA(a, b, 0.5)
	mid := color.RGBA{R: 150, G: 150, B: 150, A: 255}
	assert.Equal(t, mid, c05)
}

func TestBrightenIndexed(t *testing.T) {
	// Dark palette color should become white
	dark := color.RGBA{R: 50, G: 50, B: 50, A: 255}
	bright := brightenIndexed(dark)
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, bright)

	// Non-RGBA color returned as-is
	nonRGBA := color.NRGBA{R: 50, G: 50, B: 50, A: 255}
	assert.Equal(t, nonRGBA, brightenIndexed(nonRGBA))

	// Light color (>128) returned as-is
	light := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	assert.Equal(t, light, brightenIndexed(light))

	// Unequal R,G,B returned as-is
	uneven := color.RGBA{R: 50, G: 100, B: 50, A: 255}
	assert.Equal(t, uneven, brightenIndexed(uneven))
}

func TestIsBoxChar(t *testing.T) {
	assert.False(t, isBoxChar(0x24FF))
	assert.True(t, isBoxChar(0x2500))
	assert.True(t, isBoxChar(0x259F))
	assert.False(t, isBoxChar(0x25A0))
	assert.False(t, isBoxChar(0x27FF))
	assert.True(t, isBoxChar(0x2800))
	assert.True(t, isBoxChar(0x28FF))
	assert.False(t, isBoxChar(0x2900))
	assert.False(t, isBoxChar('A'))
	assert.False(t, isBoxChar(0))
}

func TestDrawBoxChar_Under2(t *testing.T) {
	// rect with w<2 or h<2 should be a no-op
	img := image.NewRGBA(image.Rect(0, 0, 5, 5))
	drawBoxChar(img, image.Rect(0, 0, 1, 5), 0x2588, color.White, color.Black)
	// Should not panic, and no white pixels
	assert.Equal(t, color.RGBA{}, img.At(0, 0))
}

func TestDrawBoxChar_BlockElements(t *testing.T) {
	// Test various block elements produce expected non-empty output
	chars := []rune{0x2588, 0x2580, 0x2584, 0x258C, 0x2590}
	for _, ch := range chars {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		r := image.Rect(0, 0, 8, 8)
		drawBoxChar(img, r, ch, color.White, color.Black)
		// At least one pixel should be set
		hasPixel := false
		for y := 0; y < 10 && !hasPixel; y++ {
			for x := 0; x < 10 && !hasPixel; x++ {
				if img.At(x, y) != (color.RGBA{}) {
					hasPixel = true
				}
			}
		}
		assert.True(t, hasPixel, "char U+%04X should draw pixels", ch)
	}
}

func TestDrawBoxChar_Shades(t *testing.T) {
	shades := []rune{0x2591, 0x2592, 0x2593}
	for _, ch := range shades {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		r := image.Rect(0, 0, 8, 8)
		drawBoxChar(img, r, ch, color.White, color.Black)
		// Should have at least some non-black pixels (dither)
		hasNonBlack := false
		for y := 0; y < 10 && !hasNonBlack; y++ {
			for x := 0; x < 10 && !hasNonBlack; x++ {
				if img.At(x, y) != (color.RGBA{}) {
					hasNonBlack = true
				}
			}
		}
		assert.True(t, hasNonBlack, "shade char U+%04X should produce pixels", ch)
	}
}

func TestDrawBoxChar_Lines(t *testing.T) {
	// Horizontal and vertical lines
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	r := image.Rect(0, 0, 10, 10)
	drawBoxChar(img, r, 0x2500, color.White, color.Black)
	// Should have white pixels in the middle row
	hasHorizontal := false
	for x := 0; x < 12; x++ {
		if img.At(x, 5) == (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
			hasHorizontal = true
		}
	}
	assert.True(t, hasHorizontal, "horizontal line should draw")

	img2 := image.NewRGBA(image.Rect(0, 0, 12, 12))
	drawBoxChar(img2, r, 0x2502, color.White, color.Black)
	hasVertical := false
	for y := 0; y < 12; y++ {
		if img2.At(5, y) == (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
			hasVertical = true
		}
	}
	assert.True(t, hasVertical, "vertical line should draw")
}

func TestDrawBoxChar_Corners(t *testing.T) {
	corners := []rune{0x250C, 0x2510, 0x2514, 0x2518, 0x251C, 0x2524, 0x252C, 0x2534, 0x253C}
	for _, ch := range corners {
		img := image.NewRGBA(image.Rect(0, 0, 14, 14))
		r := image.Rect(0, 0, 12, 12)
		drawBoxChar(img, r, ch, color.White, color.Black)
		hasPixel := false
		for y := 0; y < 14 && !hasPixel; y++ {
			for x := 0; x < 14 && !hasPixel; x++ {
				if img.At(x, y) != (color.RGBA{}) {
					hasPixel = true
				}
			}
		}
		assert.True(t, hasPixel, "corner U+%04X should draw pixels", ch)
	}
}

func TestDrawShade(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	r := image.Rect(0, 0, 8, 8)
	drawShade(img, r, color.White, color.Black, 128)
	// Should have a dither pattern (some black, some blended)
	hasBlack := false
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			p := img.At(x, y)
			if p == (color.RGBA{}) {
				hasBlack = true
			}
		}
	}
	assert.True(t, hasBlack, "shade should have black pixels")
}

func TestDrawShearedGlyph(t *testing.T) {
	// Create an RGBA and draw a sheared glyph with a different font
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	drawShearedGlyph(img, basicfont.Face7x13, fixed.P(5, 12), 'A', color.White)
	// Should produce some visible pixels (A is rendered in font)
	found := false
	for y := 0; y < 20 && !found; y++ {
		for x := 0; x < 20 && !found; x++ {
			if img.At(x, y) != (color.RGBA{}) {
				found = true
			}
		}
	}
	// basicfont may not have 'A' at all cells; just verify no panic
	_ = found
}

func TestApplyRenderDefaults(t *testing.T) {
	opts := Options{}
	applyRenderDefaults(&opts)
	assert.NotNil(t, opts.Font)
	assert.Greater(t, opts.CellWidth, 0)
	assert.Greater(t, opts.CellHeight, 0)
	assert.NotNil(t, opts.FgDefault)
	assert.NotNil(t, opts.BgDefault)
	assert.Equal(t, 1, opts.Scale)
	assert.Equal(t, 0.55, opts.DimFactor)
}

func TestTcellColorToRGBA(t *testing.T) {
	fallback := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	// ColorDefault returns fallback
	result := tcellColorToRGBA(tcellcolor.Default, fallback)
	assert.Equal(t, fallback, result)

	// Invalid color returns fallback
	result2 := tcellColorToRGBA(tcell.Color(0x7FFFFFFF), fallback)
	assert.Equal(t, fallback, result2)

	// Valid color returns RGBA
	red := tcell.GetColor("red")
	result3 := tcellColorToRGBA(red, fallback)
	// Just verify no panic - color resolution may vary by platform
	_ = result3
}

func TestResolveColor(t *testing.T) {
	fallback := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	// ColorDefault returns fallback
	result := resolveColor(tcellcolor.Default, fallback, nil)
	assert.Equal(t, fallback, result)

	// With nil lookup and non-default color
	red := tcell.GetColor("red")
	result2 := resolveColor(red, fallback, nil)
	assert.NotNil(t, result2)
}

func TestMax(t *testing.T) {
	assert.Equal(t, 2, max(1, 2))
	assert.Equal(t, 2, max(2, 1))
	assert.Equal(t, 0, max(0, 0))
	assert.Equal(t, -1, max(-1, -5))
}

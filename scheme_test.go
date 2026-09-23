package terminal

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHexColor(t *testing.T) {
	assert.Equal(t, color.RGBA{R: 0xFF, G: 0x55, B: 0x55, A: 0xFF}, hexColor("#FF5555"))
	assert.Equal(t, color.RGBA{R: 0x28, G: 0x2A, B: 0x36, A: 0xFF}, hexColor("282A36"))
	assert.Nil(t, hexColor(""))
	assert.Nil(t, hexColor("#FFF"))
	assert.Nil(t, hexColor("nothex!"))
}

func TestColorSchemeSpecBuild(t *testing.T) {
	cs := SchemeSpec{
		Name:       "custom",
		Foreground: "#AABBCC",
		Background: "#112233",
		Cursor:     "#FFFFFF",
		Palette:    []string{"#000000", "#FF0000"},
	}.Build()

	assert.Equal(t, "custom", cs.Name)
	assert.Equal(t, color.RGBA{R: 0xAA, G: 0xBB, B: 0xCC, A: 0xFF}, cs.Foreground)
	assert.Equal(t, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}, cs.Background)
	assert.Equal(t, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, cs.Cursor)

	c0, ok := cs.Color(0)
	assert.True(t, ok)
	assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 0xFF}, c0)
	c1, ok := cs.Color(1)
	assert.True(t, ok)
	assert.Equal(t, color.RGBA{R: 0xFF, G: 0, B: 0, A: 0xFF}, c1)
	// Unset index falls back.
	_, ok = cs.Color(5)
	assert.False(t, ok)
}

func TestSchemeByName(t *testing.T) {
	for _, name := range Schemes() {
		cs, ok := SchemeByName(name)
		assert.True(t, ok, "built-in scheme %q should resolve", name)
		assert.NotNil(t, cs)
	}
	// Aliases.
	_, ok := SchemeByName("solarized")
	assert.True(t, ok)
	_, ok = SchemeByName("GRUVBOX")
	assert.True(t, ok)
	// Unknown.
	_, ok = SchemeByName("does-not-exist")
	assert.False(t, ok)
}

func TestColorSchemeLookupNilSafe(t *testing.T) {
	var cs *Scheme
	assert.Nil(t, cs.lookup())
	_, ok := cs.Color(0)
	assert.False(t, ok)

	// A scheme that overrides nothing produces a nil lookup.
	empty := &Scheme{}
	assert.Nil(t, empty.lookup())
}

// TestRenderImage_WithScheme verifies the scheme drives both the default
// background (padding area) and an indexed palette colour (a cell painted with
// SGR background colour 1 → red).
func TestRenderImage_WithScheme(t *testing.T) {
	h, err := NewTerminal(2, 1)
	assert.NoError(t, err)
	defer h.Close()

	// A space with ANSI background colour 1 (red) fills the cell with palette
	// index 1; no glyph pixels interfere with the sample.
	_, _ = h.FeedString("\x1b[41m ")

	scheme, _ := SchemeByName("dracula")
	opts := DefaultRenderOptions()
	opts.Scheme = scheme

	img := h.RenderImage(opts)
	assert.NotNil(t, img)

	// Padding pixel (0,0) should be the scheme background (#282A36).
	assertPixel(t, img, 0, 0, color.RGBA{R: 0x28, G: 0x2A, B: 0x36, A: 0xFF})

	// Centre of cell (0,0): padding + half a cell. Should be Dracula red #FF5555.
	px := opts.Padding + opts.CellWidth/2
	py := opts.Padding + opts.CellHeight/2
	assertPixel(t, img, px, py, color.RGBA{R: 0xFF, G: 0x55, B: 0x55, A: 0xFF})
}

// TestRenderImage_SchemeDefaultForeground verifies Scheme.Foreground overrides
// the Options default foreground for default-coloured text.
func TestRenderImage_SchemeForegroundBackgroundPrecedence(t *testing.T) {
	h, err := NewTerminal(1, 1)
	assert.NoError(t, err)
	defer h.Close()
	_, _ = h.FeedString(" ")

	opts := DefaultRenderOptions()
	opts.FgDefault = color.RGBA{R: 1, G: 2, B: 3, A: 0xFF}
	opts.BgDefault = color.RGBA{R: 4, G: 5, B: 6, A: 0xFF}
	opts.Scheme = &Scheme{
		Background: color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF},
	}

	img := h.RenderImage(opts)
	// Scheme background takes precedence over BgDefault.
	assertPixel(t, img, 0, 0, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
}

func assertPixel(t *testing.T, img interface{ At(x, y int) color.Color }, x, y int, want color.RGBA) {
	t.Helper()
	r, g, b, a := img.At(x, y).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	assert.Equal(t, want, got, "pixel (%d,%d)", x, y)
}

package terminal

import (
	"image/color"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
)

// Scheme defines a customizable terminal colour palette used by the image
// renderer (Render / Terminal.RenderImage). It overrides the RGB values used
// for the 16 base ANSI colours (and, optionally, the full 256-colour palette),
// plus the default foreground, background, and cursor colours.
//
// A Scheme only affects rasterised output; it does not change the
// tcell.Color values stored in the terminal grid. This mirrors how a real
// terminal maps palette indices to on-screen RGB at draw time, so the same
// terminal state can be rendered under different themes.
//
// The zero value is valid but does nothing (all entries nil → fall back to
// tcell's built-in palette). Construct schemes with SchemeSpec.Build, one
// of the built-in Scheme* constructors, or by setting fields directly.
type Scheme struct {
	// Name is a human-readable identifier (optional).
	Name string

	// Foreground is the default text colour (used for cells with
	// tcell.ColorDefault foreground). nil falls back to Options.FgDefault.
	Foreground color.Color

	// Background is the default background colour (used for cells with
	// tcell.ColorDefault background and for padding). nil falls back to
	// Options.BgDefault.
	Background color.Color

	// Cursor is the colour of the cursor indicator. nil falls back to
	// Options.CursorColour (or white).
	Cursor color.Color

	// Palette holds up to 256 indexed colours. Index 0-7 are the standard
	// ANSI colours, 8-15 the bright variants, and 16-255 the extended
	// 256-colour cube / greyscale ramp. Entries left nil fall back to
	// tcell's built-in value for that index, so most schemes only need to
	// populate indices 0-15.
	Palette [256]color.Color
}

// lookup builds a fast reverse map from the tcell palette colours the SGR
// handler produces (tcellcolor.PaletteColor(i)) to the scheme's RGB overrides.
// Returns nil when the scheme is nil or overrides no indexed colours, so
// callers can cheaply skip the lookup.
func (s *Scheme) lookup() map[tcell.Color]color.Color {
	if s == nil {
		return nil
	}
	var m map[tcell.Color]color.Color
	for i := 0; i < 256; i++ {
		if s.Palette[i] == nil {
			continue
		}
		if m == nil {
			m = make(map[tcell.Color]color.Color)
		}
		m[tcellcolor.PaletteColor(i)] = s.Palette[i]
	}
	return m
}

// Color returns the RGB override for palette index i (0-255), or (nil, false)
// if the scheme does not override that index.
func (s *Scheme) Color(i int) (color.Color, bool) {
	if s == nil || i < 0 || i > 255 || s.Palette[i] == nil {
		return nil, false
	}
	return s.Palette[i], true
}

// SchemeSpec is a string-based (hex "#RRGGBB") description of a colour
// scheme, convenient for configuration files and JSON. Empty fields are left
// unset (nil) on the built Scheme. Build converts it to a Scheme.
type SchemeSpec struct {
	Name       string   `json:"name,omitempty"`
	Foreground string   `json:"foreground,omitempty"`
	Background string   `json:"background,omitempty"`
	Cursor     string   `json:"cursor,omitempty"`
	Palette    []string `json:"palette,omitempty"` // index-ordered, up to 256 entries
}

// Build converts the spec into a *Scheme. Invalid or empty hex strings
// become nil entries (falling back to defaults at render time).
func (spec SchemeSpec) Build() *Scheme {
	cs := &Scheme{Name: spec.Name}
	cs.Foreground = hexColor(spec.Foreground)
	cs.Background = hexColor(spec.Background)
	cs.Cursor = hexColor(spec.Cursor)
	for i := 0; i < len(spec.Palette) && i < 256; i++ {
		cs.Palette[i] = hexColor(spec.Palette[i])
	}
	return cs
}

// hexColor parses a "#RRGGBB" (or "RRGGBB") string into an opaque color.RGBA.
// It returns nil for empty or malformed input so callers can treat "unset" and
// "invalid" identically (both fall back to defaults).
func hexColor(s string) color.Color {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return nil
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return nil
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

// --- Built-in schemes ---

// SchemeDracula returns the Dracula colour scheme.
func schemeDracula() *Scheme {
	return SchemeSpec{
		Name:       "dracula",
		Foreground: "#F8F8F2", Background: "#282A36", Cursor: "#F8F8F2",
		Palette: []string{
			"#21222C", "#FF5555", "#50FA7B", "#F1FA8C", "#BD93F9", "#FF79C6", "#8BE9FD", "#F8F8F2",
			"#6272A4", "#FF6E6E", "#69FF94", "#FFFFA5", "#D6ACFF", "#FF92DF", "#A4FFFF", "#FFFFFF",
		},
	}.Build()
}

// SchemeNord returns the Nord colour scheme.
func schemeNord() *Scheme {
	return SchemeSpec{
		Name:       "nord",
		Foreground: "#D8DEE9", Background: "#2E3440", Cursor: "#D8DEE9",
		Palette: []string{
			"#3B4252", "#BF616A", "#A3BE8C", "#EBCB8B", "#81A1C1", "#B48EAD", "#88C0D0", "#E5E9F0",
			"#4C566A", "#BF616A", "#A3BE8C", "#EBCB8B", "#81A1C1", "#B48EAD", "#8FBCBB", "#ECEFF4",
		},
	}.Build()
}

// SchemeSolarizedDark returns the Solarized Dark colour scheme.
func schemeSolarizedDark() *Scheme {
	return SchemeSpec{
		Name:       "solarized-dark",
		Foreground: "#839496", Background: "#002B36", Cursor: "#93A1A1",
		Palette: []string{
			"#073642", "#DC322F", "#859900", "#B58900", "#268BD2", "#D33682", "#2AA198", "#EEE8D5",
			"#002B36", "#CB4B16", "#586E75", "#657B83", "#839496", "#6C71C4", "#93A1A1", "#FDF6E3",
		},
	}.Build()
}

// SchemeSolarizedLight returns the Solarized Light colour scheme.
func schemeSolarizedLight() *Scheme {
	return SchemeSpec{
		Name:       "solarized-light",
		Foreground: "#657B83", Background: "#FDF6E3", Cursor: "#586E75",
		Palette: []string{
			"#073642", "#DC322F", "#859900", "#B58900", "#268BD2", "#D33682", "#2AA198", "#EEE8D5",
			"#002B36", "#CB4B16", "#586E75", "#657B83", "#839496", "#6C71C4", "#93A1A1", "#FDF6E3",
		},
	}.Build()
}

// SchemeGruvboxDark returns the Gruvbox Dark colour scheme.
func schemeGruvboxDark() *Scheme {
	return SchemeSpec{
		Name:       "gruvbox-dark",
		Foreground: "#EBDBB2", Background: "#282828", Cursor: "#EBDBB2",
		Palette: []string{
			"#282828", "#CC241D", "#98971A", "#D79921", "#458588", "#B16286", "#689D6A", "#A89984",
			"#928374", "#FB4934", "#B8BB26", "#FABD2F", "#83A598", "#D3869B", "#8EC07C", "#EBDBB2",
		},
	}.Build()
}

// SchemeOneDark returns the Atom One Dark colour scheme.
func schemeOneDark() *Scheme {
	return SchemeSpec{
		Name:       "one-dark",
		Foreground: "#ABB2BF", Background: "#282C34", Cursor: "#528BFF",
		Palette: []string{
			"#282C34", "#E06C75", "#98C379", "#E5C07B", "#61AFEF", "#C678DD", "#56B6C2", "#ABB2BF",
			"#5C6370", "#E06C75", "#98C379", "#E5C07B", "#61AFEF", "#C678DD", "#56B6C2", "#FFFFFF",
		},
	}.Build()
}

// SchemeMonokai returns the Monokai colour scheme.
func schemeMonokai() *Scheme {
	return SchemeSpec{
		Name:       "monokai",
		Foreground: "#F8F8F2", Background: "#272822", Cursor: "#F8F8F0",
		Palette: []string{
			"#272822", "#F92672", "#A6E22E", "#F4BF75", "#66D9EF", "#AE81FF", "#A1EFE4", "#F8F8F2",
			"#75715E", "#F92672", "#A6E22E", "#F4BF75", "#66D9EF", "#AE81FF", "#A1EFE4", "#F9F8F5",
		},
	}.Build()
}

// SchemeTangoDark returns the Tango (dark) colour scheme, the classic
// GNOME/Ubuntu terminal palette.
func schemeTangoDark() *Scheme {
	return SchemeSpec{
		Name:       "tango-dark",
		Foreground: "#D3D7CF", Background: "#2E3436", Cursor: "#FFFFFF",
		Palette: []string{
			"#000000", "#CC0000", "#4E9A06", "#C4A000", "#3465A4", "#75507B", "#06989A", "#D3D7CF",
			"#555753", "#EF2929", "#8AE234", "#FCE94F", "#729FCF", "#AD7FA8", "#34E2E2", "#EEEEEC",
		},
	}.Build()
}

// builtinSchemes is the registry of named schemes returned by Schemes and
// resolved by SchemeByName.
var builtinSchemes = map[string]func() *Scheme{
	"dracula":         schemeDracula,
	"nord":            schemeNord,
	"solarized-dark":  schemeSolarizedDark,
	"solarized-light": schemeSolarizedLight,
	"gruvbox-dark":    schemeGruvboxDark,
	"one-dark":        schemeOneDark,
	"monokai":         schemeMonokai,
	"tango-dark":      schemeTangoDark,
}

// Schemes returns the sorted list of built-in colour scheme names.
func Schemes() []string {
	names := make([]string, 0, len(builtinSchemes))
	for name := range builtinSchemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SchemeByName returns a freshly built built-in scheme by name (case-insensitive),
// or (nil, false) if no such scheme exists. Common aliases are accepted.
func SchemeByName(name string) (*Scheme, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "solarized", "solarizeddark":
		key = "solarized-dark"
	case "solarizedlight":
		key = "solarized-light"
	case "gruvbox", "gruvboxdark":
		key = "gruvbox-dark"
	case "onedark":
		key = "one-dark"
	case "tango", "tangodark":
		key = "tango-dark"
	}
	if fn, ok := builtinSchemes[key]; ok {
		return fn(), true
	}
	return nil, false
}

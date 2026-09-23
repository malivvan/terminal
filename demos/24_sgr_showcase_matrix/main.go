// Package main renders a full SGR style/colour matrix to a PNG and prints a
// text summary. It exercises 16-colour, 256-colour and 24-bit truecolour
// output, plus attributes (bold, italic, underline, reverse, overline) and an
// OSC 8 hyperlink.
//
// Run with:
//
//	go run ./demo/24_sgr_showcase_matrix -o sgr.png
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"strings"

	"github.com/malivvan/terminal"
)

func main() {
	out := flag.String("o", "sgr.png", "output PNG file")
	schemeName := flag.String("scheme", "", "colour scheme (e.g. dracula, nord, solarized-dark, gruvbox-dark, one-dark, monokai, tango-dark)")
	flag.Parse()

	h, err := terminal.NewTerminal(64, 14)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	var b strings.Builder

	// Row: standard + bright 16 colours (as backgrounds).
	b.WriteString("16-color: ")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "\x1b[4%dm  \x1b[0m", i)
	}
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "\x1b[10%dm  \x1b[0m", i)
	}
	b.WriteString("\r\n")

	// Two rows of the 256-colour cube.
	b.WriteString("256-color:\r\n")
	for _, base := range []int{16, 124} {
		for n := base; n < base+32; n++ {
			fmt.Fprintf(&b, "\x1b[48;5;%dm \x1b[0m", n)
		}
		b.WriteString("\r\n")
	}

	// A 24-bit truecolour gradient.
	b.WriteString("truecolor:\r\n")
	for x := 0; x < 48; x++ {
		r := (x * 255) / 47
		g := 128
		bl := 255 - r
		fmt.Fprintf(&b, "\x1b[48;2;%d;%d;%dm \x1b[0m", r, g, bl)
	}
	b.WriteString("\r\n")

	// Attributes.
	b.WriteString("attrs: ")
	b.WriteString("\x1b[1mbold\x1b[0m ")
	b.WriteString("\x1b[3mitalic\x1b[0m ")
	b.WriteString("\x1b[4munderline\x1b[0m ")
	b.WriteString("\x1b[7mreverse\x1b[0m ")
	b.WriteString("\x1b[53moverline\x1b[0m ")
	b.WriteString("\x1b[9mstrike\x1b[0m\r\n")

	// OSC 8 hyperlink.
	b.WriteString("link:  ")
	b.WriteString("\x1b]8;;https://github.com/malivvan/terminal\x1b\\clickable text\x1b]8;;\x1b\\\r\n")

	_, _ = h.FeedString(b.String())

	// Text summary.
	fmt.Println("SGR showcase (see PNG for colours):")
	for _, l := range h.Lines() {
		if s := strings.TrimRight(l, " "); s != "" {
			fmt.Println("  " + s)
		}
	}

	// Render the colourful version to PNG.
	opts := terminal.DefaultRenderOptions()
	opts.CellWidth = 8
	opts.CellHeight = 16
	if *schemeName != "" {
		scheme, ok := terminal.SchemeByName(*schemeName)
		if !ok {
			fmt.Printf("unknown scheme %q; available: %v\n", *schemeName, terminal.Schemes())
			os.Exit(1)
		}
		opts.Scheme = scheme
		fmt.Printf("applying colour scheme: %s\n", scheme.Name)
	}
	img := h.RenderImage(opts)

	f, err := os.Create(*out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	fmt.Println("wrote", *out)
}

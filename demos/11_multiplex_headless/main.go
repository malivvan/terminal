// Package main demonstrates a "headless terminal multiplexer": four
// independent Terminal VTs each render into their own image, and the
// four images are composited into a single 2x2 tiled PNG.
//
// Run:  go run ./demo/11_multiplex_headless -o multiplex.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"

	"github.com/malivvan/terminal"
)

func main() {
	out := flag.String("o", "multiplex.png", "output PNG file")
	flag.Parse()

	panes := make([]*terminal.Terminal, 4)
	scripts := []string{
		"\x1b[1;32mpane #0\x1b[0m\r\n$ echo hi\r\nhi\r\n$ ",
		"\x1b[1;33mpane #1\x1b[0m\r\ntop - 12:00:01 up 1 day\r\nCPU: 3%\r\n",
		"\x1b[1;34mpane #2\x1b[0m\r\n\x1b[7mless -\x1b[0m foo.log\r\n...\r\n:",
		"\x1b[1;31mpane #3\x1b[0m\r\n\x1b[4mvim\x1b[0m foo.go\r\n:%s/old/new/g\r\n:w",
	}
	for i := 0; i < 4; i++ {
		h, err := terminal.NewTerminal(20, 4)
		if err != nil {
			panic(err)
		}
		_, _ = h.FeedString(scripts[i])
		panes[i] = h
	}
	defer func() {
		for _, h := range panes {
			h.Close()
		}
	}()

	opts := terminal.DefaultRenderOptions()
	frames := make([]*image.RGBA, 4)
	for i, h := range panes {
		frames[i] = h.RenderImage(opts)
	}

	w := frames[0].Bounds().Dx()
	hgt := frames[0].Bounds().Dy()
	gutter := 4
	canvas := image.NewRGBA(image.Rect(0, 0, 2*w+gutter, 2*hgt+gutter))

	positions := []image.Point{
		{0, 0},
		{w + gutter, 0},
		{0, hgt + gutter},
		{w + gutter, hgt + gutter},
	}
	for i, p := range positions {
		r := image.Rect(p.X, p.Y, p.X+w, p.Y+hgt)
		draw.Draw(canvas, r, frames[i], image.Point{}, draw.Src)
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, canvas); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", *out)
}

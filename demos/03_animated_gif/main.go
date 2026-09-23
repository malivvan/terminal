// Package main captures a sequence of terminal frames and encodes them
// as an animated GIF.
//
// Run with:
//
//	go run ./demo/03_animated_gif -o out.gif
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"

	"github.com/malivvan/terminal"
)

func main() {
	out := flag.String("o", "out.gif", "output GIF file")
	flag.Parse()

	h, err := terminal.NewTerminal(20, 3)
	if err != nil {
		fmt.Fprintln(os.Stderr, "headless:", err)
		os.Exit(1)
	}
	defer h.Close()

	frames := []*image.RGBA{}
	opts := terminal.DefaultRenderOptions()

	// Type "hello, world!" one character at a time, capturing after each.
	for _, r := range "hello, world!" {
		_, _ = h.FeedString(string(r))
		frames = append(frames, h.RenderImage(opts))
	}

	// Encode.
	g := &gif.GIF{LoopCount: 0}
	for _, f := range frames {
		p := image.NewPaletted(f.Bounds(), palette.Plan9)
		draw.FloydSteinberg.Draw(p, f.Bounds(), f, image.Point{})
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 8) // 80 ms per frame
	}

	fout, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
	defer fout.Close()
	if err := gif.EncodeAll(fout, g); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d frames)\n", *out, len(frames))
}

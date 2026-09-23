// Package main compares several Options configurations side-by-side by
// rendering the same terminal content with different padding, cell sizes,
// cursor rendering and default colours. Each variant is written to its own
// PNG and also combined into a single contact sheet.
//
// Run with:
//
//	go run ./demo/29_render_options_fonts -o render_variants.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/malivvan/terminal"
)

func main() {
	out := flag.String("o", "render_variants.png", "output contact-sheet PNG")
	flag.Parse()

	h, err := terminal.NewTerminal(24, 3)
	if err != nil {
		panic(err)
	}
	defer h.Close()
	_, _ = h.FeedString("\x1b[1;33mRenderOptions\x1b[0m demo\r\n")
	_, _ = h.FeedString("cell / padding / cursor\r\n")
	_, _ = h.FeedString("second line of text")

	variants := []struct {
		name string
		opts terminal.Options
	}{
		{"default", terminal.DefaultRenderOptions()},
		{"compact", terminal.Options{CellWidth: 7, CellHeight: 13, Padding: 2}},
		{"large-cells", terminal.Options{CellWidth: 12, CellHeight: 22, Padding: 8}},
		{"cursor-light", terminal.Options{
			CellWidth: 9, CellHeight: 18, Padding: 6,
			RenderCursor: true,
			CursorColour: color.RGBA{R: 0x33, G: 0xAA, B: 0xFF, A: 0xFF},
			FgDefault:    color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF},
			BgDefault:    color.RGBA{R: 0xF5, G: 0xF5, B: 0xF5, A: 0xFF},
		}},
	}

	imgs := make([]*image.RGBA, 0, len(variants))
	maxW := 0
	totalH := 0
	const gap = 12
	for _, v := range variants {
		img := h.RenderImage(v.opts)
		imgs = append(imgs, img)
		if img.Bounds().Dx() > maxW {
			maxW = img.Bounds().Dx()
		}
		totalH += img.Bounds().Dy() + gap

		// Also write each variant individually.
		fn := fmt.Sprintf("render_%s.png", v.name)
		writePNG(fn, img)
		fmt.Printf("wrote %s (%dx%d)\n", fn, img.Bounds().Dx(), img.Bounds().Dy())
	}

	// Build a vertical contact sheet.
	sheet := image.NewRGBA(image.Rect(0, 0, maxW, totalH))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.Gray{Y: 0x30}), image.Point{}, draw.Src)
	y := 0
	for _, img := range imgs {
		r := image.Rect(0, y, img.Bounds().Dx(), y+img.Bounds().Dy())
		draw.Draw(sheet, r, img, image.Point{}, draw.Src)
		y += img.Bounds().Dy() + gap
	}
	writePNG(*out, sheet)
	fmt.Println("wrote", *out)
}

func writePNG(name string, img image.Image) {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

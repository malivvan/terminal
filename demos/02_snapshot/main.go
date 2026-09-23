// Package main renders the current terminal state to a PNG file.
//
// Run with:
//
//	go run ./demo/02_snapshot -o screenshot.png
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"

	"github.com/malivvan/terminal"
)

func main() {
	out := flag.String("o", "screenshot.png", "output PNG file")
	flag.Parse()

	h, err := terminal.NewTerminal(40, 6)
	if err != nil {
		fmt.Fprintln(os.Stderr, "headless:", err)
		os.Exit(1)
	}
	defer h.Close()

	// Some colourful content.
	_, _ = h.FeedString("\x1b[1;38;2;255;120;0mterminal\x1b[0m package demo\r\n")
	_, _ = h.FeedString("--------------------------------\r\n")
	_, _ = h.FeedString("\x1b[7mreverse\x1b[0m + \x1b[4munderlined\x1b[0m + \x1b[3mitalic\x1b[0m\r\n")
	_, _ = h.FeedString("regular text on line 4\r\n")

	img := h.RenderImage(terminal.DefaultRenderOptions())

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", *out)
}

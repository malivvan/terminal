// Package main visualizes the resize copy-preservation semantics of the VT.
// On Resize the terminal copies existing cell content directly into the new
// grid (it does NOT replay the byte stream), so text you already printed is
// preserved and simply re-flowed to the new dimensions.
//
// Run with:
//
//	go run ./demo/22_resize_preserve_content
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

func show(h *terminal.Terminal, label string) {
	w, ht := h.Size()
	fmt.Printf("---- %s (%dx%d) ----\n", label, w, ht)
	top := "+" + strings.Repeat("-", w) + "+"
	fmt.Println(top)
	for _, l := range h.Lines() {
		if len(l) < w {
			l += strings.Repeat(" ", w-len(l))
		}
		fmt.Printf("|%s|\n", l[:w])
	}
	fmt.Println(top)
	fmt.Println()
}

func main() {
	h, err := terminal.NewTerminal(20, 4)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	_, _ = h.FeedString("Line one\r\nLine two\r\nLine three")
	show(h, "original")

	// Grow: existing rows/cols are preserved, new space is blank.
	h.Resize(32, 6)
	show(h, "after grow")

	// Shrink: content is clipped to the new bounds, not reflowed away.
	h.Resize(12, 4)
	show(h, "after shrink")

	fmt.Println("Note: content survived both resizes via direct cell copy,")
	fmt.Println("without replaying the original byte stream.")
}

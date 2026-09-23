// Package main demonstrates OSC 8 hyperlink ingestion. After feeding a
// URL-linked run of text, we print each cell rune to confirm the visible
// text landed on the grid. The associated URL/URL-id are stored on the
// per-cell tcell.Style but tcell v3 does not expose public getters for
// them — real applications typically forward the style directly to the
// renderer without ever inspecting it.
package main

import (
	"fmt"

	"github.com/malivvan/terminal"
)

func main() {
	h, err := terminal.NewTerminal(20, 1)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// OSC 8 hyperlink: click "here" -> https://example.com
	_, _ = h.FeedString("\x1b]8;id=my-id;https://example.com\x1b\\here\x1b]8;;\x1b\\ end")

	fmt.Println("Line:", h.Lines()[0])
	fmt.Println()
	for x := 0; x < 10; x++ {
		c := h.Cell(x, 0)
		fmt.Printf("col %2d rune %q\n", x, c.Rune)
	}
	fmt.Println()
	fmt.Println("(url and url-id are attached to the per-cell tcell.Style)")
}

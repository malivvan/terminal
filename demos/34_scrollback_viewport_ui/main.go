// Package main is a scripted walk-through of scrollback paging and search over
// a large volume of output. It generates many lines, then pages the viewport
// back through history and searches the combined scrollback + live screen.
//
// (Kept non-interactive so it runs anywhere; the same ScrollBy / ScrollOffset
// calls drive a real interactive pager.)
//
// Run with:
//
//	go run ./demo/34_scrollback_viewport_ui
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

const viewH = 6

func viewport(h *terminal.Terminal, label string) {
	h.Session().Draw() // repaint the surface at the current scroll offset
	fmt.Printf("---- %s (offset=%d, scrollback=%d) ----\n",
		label, h.Session().ScrollOffset(), h.ScrollbackLen())
	for _, l := range h.Lines() {
		fmt.Printf("  |%s\n", strings.TrimRight(l, " "))
	}
	fmt.Println()
}

func main() {
	h, err := terminal.NewTerminal(30, viewH)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Produce a lot of output; older lines spill into scrollback.
	var b strings.Builder
	for i := 1; i <= 40; i++ {
		marker := ""
		if i == 13 {
			marker = "  <== needle"
		}
		fmt.Fprintf(&b, "line %02d%s\r\n", i, marker)
	}
	_, _ = h.FeedString(b.String())

	viewport(h, "live (bottom)")

	// Page up two screens.
	h.Session().ScrollBy(viewH * 2)
	viewport(h, "paged up 2 screens")

	// Jump to the very top of history.
	h.Session().ScrollBy(h.ScrollbackLen())
	viewport(h, "top of history")

	// Search the whole buffer (scrollback + live) for "needle".
	fmt.Println("search for \"needle\":")
	found := false
	for i := 0; i < h.ScrollbackLen(); i++ {
		if line, ok := h.ScrollbackLine(i); ok && strings.Contains(line, "needle") {
			fmt.Printf("  scrollback[%d]: %s\n", i, strings.TrimRight(line, " "))
			found = true
		}
	}
	for y, line := range h.Lines() {
		if strings.Contains(line, "needle") {
			fmt.Printf("  screen[%d]: %s\n", y, strings.TrimRight(line, " "))
			found = true
		}
	}
	if !found {
		fmt.Println("  (not found)")
	}

	// Snap back to the live screen (any keystroke does this in a real app).
	h.Session().ScrollBy(-h.Session().ScrollOffset())
	viewport(h, "back to live")
}

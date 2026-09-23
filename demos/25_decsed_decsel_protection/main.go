// Package main demonstrates DECSCA character protection together with the
// selective erase operations DECSED (CSI ? Ps J) and DECSEL (CSI ? Ps K).
// Cells written while protection is on (CSI 1 "q) survive a selective erase;
// everything else is cleared.
//
// Run with:
//
//	go run ./demo/25_decsed_decsel_protection
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

func show(h *terminal.Terminal, label string) {
	fmt.Printf("---- %s ----\n", label)
	for y, l := range h.Lines() {
		// Build a protection map for this row.
		w, _ := h.Size()
		prot := make([]byte, 0, w)
		for x := 0; x < w; x++ {
			if h.Cell(x, y).Protected {
				prot = append(prot, '^')
			} else {
				prot = append(prot, ' ')
			}
		}
		fmt.Printf("  %q\n", strings.TrimRight(l, " "))
		if p := strings.TrimRight(string(prot), " "); p != "" {
			fmt.Printf("   %s  (^ = DECSCA-protected)\n", p)
		}
	}
	fmt.Println()
}

func main() {
	h, err := terminal.NewTerminal(32, 2)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Row 0: protected label, then unprotected value.
	// CSI 1 "q  = enable protection, CSI 0 "q = disable.
	_, _ = h.FeedString("\x1b[1\"qLOCKED\x1b[0\"q editable")
	// Row 1: entirely unprotected.
	_, _ = h.FeedString("\r\ntemporary status line")
	show(h, "before selective erase")

	// DECSED 2 — selective erase in display: clears only UNprotected cells.
	_, _ = h.FeedString("\x1b[?2J")
	show(h, "after DECSED (CSI ? 2 J)")

	fmt.Println("Only the DECSCA-protected \"LOCKED\" text survived the erase.")
}

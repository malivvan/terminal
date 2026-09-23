// Package main demonstrates DECSET 2026 (synchronized output). While
// synchronized output is enabled, the terminal keeps mutating its internal
// grid but defers painting to the Screen until the mode is reset. This lets
// applications build a complete frame and flush it atomically, avoiding
// tearing.
//
// We contrast two views of a Terminal terminal:
//
//   - String():  the internal grid (always up to date)
//   - Lines():   the painted Screen (frozen while sync is on)
//
// Run with:
//
//	go run ./demo/20_sync_output_mode
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

func dump(h *terminal.Terminal, label string) {
	fmt.Printf("---- %s ----\n", label)
	fmt.Println("internal grid (String):", strings.ReplaceAll(strings.TrimRight(h.String(), " \n"), "\n", " / "))
	painted := make([]string, 0)
	for _, l := range h.Lines() {
		painted = append(painted, strings.TrimRight(l, " "))
	}
	fmt.Println("painted surface (Lines):", strings.Join(painted, " / "))
	fmt.Println("SynchronizedOutput mode:", h.GetModes().SynchronizedOutput)
	fmt.Println()
}

func main() {
	h, err := terminal.NewTerminal(24, 2)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	_, _ = h.FeedString("BEFORE sync")
	dump(h, "baseline")

	// Turn on synchronized output, then write a brand new frame.
	_, _ = h.FeedString("\x1b[?2026h")
	_, _ = h.FeedString("\x1b[2J\x1b[H")          // clear + home
	_, _ = h.FeedString("DURING sync (buffered)") // grid updates, paint deferred
	dump(h, "while 2026h is active")

	// Flush by resetting synchronized output. The next paint reveals the frame.
	_, _ = h.FeedString("\x1b[?2026l")
	dump(h, "after 2026l flush")
}

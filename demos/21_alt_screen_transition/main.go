// Package main demonstrates the alternate-screen transition triggered by
// DECSET 1049 (?1049h / ?1049l). Entering saves the cursor and switches to a
// fresh alt buffer (as vim/less/htop do); leaving clears the alt buffer,
// restores the primary screen, and restores the saved cursor position.
//
// Run with:
//
//	go run ./demo/21_alt_screen_transition
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

func status(h *terminal.Terminal, label string) {
	col, row, _ := h.Cursor()
	fmt.Printf("---- %s ----\n", label)
	fmt.Printf("alt-screen=%v cursor=(%d,%d)\n", h.GetModes().AltScreen, col, row)
	for i, l := range h.Lines() {
		fmt.Printf("%d|%s\n", i, strings.TrimRight(l, " "))
	}
	fmt.Println()
}

func main() {
	h, err := terminal.NewTerminal(30, 4)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Populate the primary screen and move the cursor somewhere memorable.
	_, _ = h.FeedString("primary line A\r\nprimary line B\r\n")
	_, _ = h.FeedString("\x1b[1;5H") // row 1, col 5
	status(h, "primary screen")

	// Enter the alternate screen (saves cursor, blank alt buffer).
	_, _ = h.FeedString("\x1b[?1049h")
	_, _ = h.FeedString("\x1b[2J\x1b[HALTERNATE full-screen app\r\n(press q to quit)")
	status(h, "alternate screen")

	// Leave the alternate screen: primary content and cursor come back.
	_, _ = h.FeedString("\x1b[?1049l")
	status(h, "back on primary (cursor restored)")
}

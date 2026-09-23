// Package main demonstrates the simplest headless usage of the
// terminal package: create a Terminal Session, feed some bytes into it,
// and inspect the resulting screen state.
package main

import (
	"fmt"

	"github.com/malivvan/terminal"
)

func main() {
	h, err := terminal.NewTerminal(20, 4)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Ordinary text and a bright-red SGR sequence.
	_, _ = h.FeedString("hello, ")
	_, _ = h.FeedString("\x1b[1;31mworld!\x1b[0m\r\n")
	_, _ = h.FeedString("second line here.\r\n")

	fmt.Println("== Lines() ==")
	for i, l := range h.Lines() {
		fmt.Printf("%2d | %q\n", i, l)
	}

	col, row, vis := h.Cursor()
	fmt.Printf("\nCursor: col=%d row=%d visible=%v\n", col, row, vis)
}

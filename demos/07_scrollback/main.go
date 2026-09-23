// Package main demonstrates the scrollback ring buffer and viewport
// control via ScrollBy / ScrollOffset.
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

func main() {
	h, err := terminal.NewTerminal(20, 3)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Push 10 lines through — 7 will end up in scrollback.
	for i := 0; i < 10; i++ {
		_, _ = h.FeedString(fmt.Sprintf("line %02d\r\n", i))
	}

	fmt.Println("== live viewport ==")
	for _, l := range h.Lines() {
		fmt.Println(strings.TrimRight(l, " "))
	}

	// Scroll the viewport 3 lines into history.
	h.Session().ScrollBy(3)
	h.Session().Draw()

	fmt.Println("\n== after ScrollBy(3) ==")
	for _, l := range h.Lines() {
		fmt.Println(strings.TrimRight(l, " "))
	}

	fmt.Printf("\nScrollOffset now: %d\n", h.Session().ScrollOffset())
}

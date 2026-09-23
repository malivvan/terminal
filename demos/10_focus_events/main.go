// Package main enables DECSET 1004 focus event reporting and shows the
// byte sequences the Session emits when tcell focus events are delivered.
package main

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

func main() {
	h, err := terminal.NewTerminal(4, 1)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Enable focus event reporting via DECSET 1004.
	_, _ = h.FeedString("\x1b[?1004h")
	_ = h.Output() // drain

	h.HandleEvent(tcell.NewEventFocus(true))
	fmt.Printf("focus-in  bytes: %q\n", string(h.Output()))

	h.HandleEvent(tcell.NewEventFocus(false))
	fmt.Printf("focus-out bytes: %q\n", string(h.Output()))
}

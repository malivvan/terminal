// Package main demonstrates sending synthesized key events into a
// Terminal Session and reading the bytes it emits back through the PTY.
package main

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

func main() {
	h, err := terminal.NewTerminal(10, 1)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	events := []struct {
		name string
		ev   tcell.Event
	}{
		{"Rune 'A'", tcell.NewEventKey(tcell.KeyRune, "A", tcell.ModNone)},
		{"Ctrl+G", tcell.NewEventKey(tcell.KeyCtrlG, "G", tcell.ModCtrl)},
		{"Enter", tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)},
		{"Shift+Right", tcell.NewEventKey(tcell.KeyRight, "", tcell.ModShift)},
	}

	for _, e := range events {
		h.HandleEvent(e.ev)
		fmt.Printf("%-14s -> %q\n", e.name, string(h.Output()))
	}
}

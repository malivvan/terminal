// Package main showcases how DECSET mouse tracking modes interact with
// tcell mouse events fed through HandleEvent. The Session emits mouse-report
// byte sequences whose exact shape depends on the currently active
// modes (button tracking 1000, drag 1002, any-motion 1003, SGR 1006).
package main

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

func main() {
	h, err := terminal.NewTerminal(20, 5)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	_, _ = h.FeedString("\x1b[?1000h\x1b[?1006h")
	_ = h.Output()

	events := []struct {
		name string
		ev   tcell.Event
	}{
		{"Button1 press @(3,2)", tcell.NewEventMouse(3, 2, tcell.Button1, tcell.ModNone)},
		{"Button1 release @(3,2)", tcell.NewEventMouse(3, 2, tcell.ButtonNone, tcell.ModNone)},
		{"Button3 press @(9,4)", tcell.NewEventMouse(9, 4, tcell.Button3, tcell.ModNone)},
		{"Button3 release @(9,4)", tcell.NewEventMouse(9, 4, tcell.ButtonNone, tcell.ModNone)},
		{"WheelUp @(0,0)", tcell.NewEventMouse(0, 0, tcell.WheelUp, tcell.ModNone)},
		{"WheelDown @(0,0)", tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone)},
	}
	fmt.Println("== button-events (1000) + SGR (1006) ==")
	for _, e := range events {
		h.HandleEvent(e.ev)
		fmt.Printf("%-24s -> %q\n", e.name, string(h.Output()))
	}

	_, _ = h.FeedString("\x1b[?1002h")
	_ = h.Output()
	fmt.Println("\n== + drag events (1002) ==")
	drag := []struct {
		name string
		ev   tcell.Event
	}{
		{"Button1 press @(5,2)", tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModNone)},
		{"Button1 drag  @(6,2)", tcell.NewEventMouse(6, 2, tcell.Button1, tcell.ModNone)},
		{"Button1 drag  @(7,3)", tcell.NewEventMouse(7, 3, tcell.Button1, tcell.ModNone)},
		{"Button1 release @(7,3)", tcell.NewEventMouse(7, 3, tcell.ButtonNone, tcell.ModNone)},
	}
	for _, e := range drag {
		h.HandleEvent(e.ev)
		fmt.Printf("%-24s -> %q\n", e.name, string(h.Output()))
	}

	_, _ = h.FeedString("\x1b[?1003h")
	_ = h.Output()
	fmt.Println("\n== + any-motion events (1003) ==")
	motion := []struct {
		name string
		ev   tcell.Event
	}{
		{"motion @(1,1)", tcell.NewEventMouse(1, 1, tcell.ButtonNone, tcell.ModNone)},
		{"motion @(2,1)", tcell.NewEventMouse(2, 1, tcell.ButtonNone, tcell.ModNone)},
		{"motion @(2,2)", tcell.NewEventMouse(2, 2, tcell.ButtonNone, tcell.ModNone)},
	}
	for _, e := range motion {
		h.HandleEvent(e.ev)
		fmt.Printf("%-24s -> %q\n", e.name, string(h.Output()))
	}
}

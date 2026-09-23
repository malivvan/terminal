// Package main compares the byte sequences different mouse-reporting protocols
// emit for the same input events. Enabling X10, VT200, UTF-8, SGR or URXVT
// encoding changes how HandleEvent serializes a mouse click/drag/wheel.
//
// Run with:
//
//	go run ./demo/33_mouse_protocol_compare
package main

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

func main() {
	protocols := []struct {
		name   string
		enable string // DECSET sequence(s) selecting the encoding
	}{
		{"X10 (?9)", "\x1b[?9h"},
		{"VT200 (?1000)", "\x1b[?1000h"},
		{"UTF-8 (?1000;?1005)", "\x1b[?1000h\x1b[?1005h"},
		{"SGR (?1000;?1006)", "\x1b[?1000h\x1b[?1006h"},
		{"URXVT (?1000;?1015)", "\x1b[?1000h\x1b[?1015h"},
	}

	events := []struct {
		name string
		ev   tcell.Event
	}{
		{"Button1 press @(10,5)", tcell.NewEventMouse(10, 5, tcell.Button1, tcell.ModNone)},
		{"release @(10,5)", tcell.NewEventMouse(10, 5, tcell.ButtonNone, tcell.ModNone)},
		{"WheelUp @(0,0)", tcell.NewEventMouse(0, 0, tcell.WheelUp, tcell.ModNone)},
	}

	for _, proto := range protocols {
		// Fresh terminal per protocol so modes don't accumulate.
		h, err := terminal.NewTerminal(40, 12)
		if err != nil {
			panic(err)
		}
		_, _ = h.FeedString(proto.enable)
		_ = h.Output()

		fmt.Printf("== %s  [mouse mode: %s] ==\n", proto.name, h.GetModes().MouseMode)
		for _, e := range events {
			h.HandleEvent(e.ev)
			fmt.Printf("  %-22s -> %q\n", e.name, string(h.Output()))
		}
		fmt.Println()
		_ = h.Close()
	}
}

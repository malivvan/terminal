//go:build unix

// Package main is a live inspector: it opens a tcell.Screen, hands
// every mouse event to a Terminal Session that has SGR mouse mode enabled,
// and prints the byte sequence the Session would forward to a running
// application in a status line at the bottom of the screen.
//
// Press 'q' to quit.
package main

import (
	"fmt"
	"log"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

func main() {
	scr, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := scr.Init(); err != nil {
		log.Fatal(err)
	}
	defer scr.Fini()
	scr.EnableMouse()

	h, err := terminal.NewTerminal(80, 20)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()
	_, err = h.FeedString("\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h")
	if err != nil {
		log.Fatal(err)
	}
	_ = h.Output()

	drawHelp := func(last string) {
		W, H := scr.Size()
		st := tcell.StyleDefault.Reverse(true)
		msg := fmt.Sprintf(" terminal mouse reporter | press q to quit | last event: %q ", last)
		x := 0
		for _, r := range msg {
			if x >= W {
				break
			}
			scr.SetContent(x, H-1, r, nil, st)
			x++
		}
		for ; x < W; x++ {
			scr.SetContent(x, H-1, ' ', nil, st)
		}
		scr.Show()
	}
	drawHelp("(none)")

	for ev := range scr.EventQ() {
		switch e := ev.(type) {
		case *tcell.EventKey:
			if e.Key() == tcell.KeyCtrlB || (e.Key() == tcell.KeyRune && e.Str() == "q") {
				return
			}
			if e.Key() == tcell.KeyCtrlC {
				return
			}
		case *tcell.EventMouse:
			h.HandleEvent(e)
			drawHelp(string(h.Output()))
		case *tcell.EventResize:
			scr.Sync()
			drawHelp("")
		}
	}
}

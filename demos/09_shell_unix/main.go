//go:build unix

// Package main runs /bin/sh inside a real PTY, renders the terminal
// to a tcell.Screen, and forwards key/mouse events until the shell
// exits.
package main

import (
	"log"
	"os/exec"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

func main() {
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}
	defer screen.Fini()

	w, h := screen.Size()
	vt := terminal.NewWithSize(w, h)
	vt.SetSurface(screen)

	if err := vt.Start(exec.Command("/bin/sh")); err != nil {
		log.Fatal(err)
	}
	defer vt.Close()

	// redraw signals the main loop to re-render when terminal content
	// changes. The redraw handler is invoked from the Session's parse
	// goroutine for every EventRedraw.
	redraw := make(chan struct{}, 1)
	vt.SetRedrawHandler(func() {
		select {
		case redraw <- struct{}{}:
		default:
		}
	})

	// Forward events the main loop needs to act on (EventClosed, resize,
	// user input) through the screen's EventQ channel.
	vt.Attach(func(ev tcell.Event) {
		select {
		case screen.EventQ() <- ev:
		default:
		}
	})

	draw := func() {
		vt.Draw()
		screen.Show()
	}
	draw()

	for {
		select {
		case <-redraw:
		case ev, ok := <-screen.EventQ():
			if !ok {
				return
			}
			switch ev := ev.(type) {
			case *terminal.EventClosed:
				return
			case *tcell.EventResize:
				w, h := ev.Size()
				vt.Resize(w, h)
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyCtrlB {
					return
				}
				vt.HandleEvent(ev)
			default:
				// forward remaining events (mouse, paste, focus, etc.)
				vt.HandleEvent(ev)
			}
		}
		draw()
	}
}

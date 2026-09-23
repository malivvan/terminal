//go:build windows

// Package main runs a Windows shell (powershell/cmd) inside a ConPTY, renders
// the terminal to a tcell.Screen, and forwards key/mouse events until the
// shell exits. The ConPTY plumbing lives behind terminal.Start, which starts the
// command on a pseudo-console from github.com/malivvan/pty.
//
// Run with:
//
//	go run ./demo/30_windows_conpty_shell
package main

import (
	"log"
	"os"
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

	// Prefer PowerShell; fall back to cmd.exe.
	shell := "powershell.exe"
	if _, err := exec.LookPath(shell); err != nil {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
	}

	if err := vt.Start(exec.Command(shell)); err != nil {
		log.Fatal(err)
	}
	defer vt.Close()

	redraw := make(chan struct{}, 1)
	vt.SetRedrawHandler(func() {
		select {
		case redraw <- struct{}{}:
		default:
		}
	})
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
				// F10 quits the demo.
				if ev.Key() == tcell.KeyF10 {
					return
				}
				vt.HandleEvent(ev)
			default:
				vt.HandleEvent(ev)
			}
		}
		draw()
	}
}

// Package main shows OSC 52 clipboard events being surfaced via the
// event handler. To receive events we drive a real Session (not a Terminal
// one, which has no parser goroutine) through a pair of pipes and a
// dummy Screen that discards visual output.
package main

import (
	"fmt"
	"io"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

// discardWriter accepts a Reader for the parser to consume and swallows
// anything the Session writes back through its "pty".
type discardWriter struct {
	io.Reader
}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (discardWriter) Close() error                { return nil }

// dummyScreen satisfies terminal.Screen and provides Init/Fini/SetSize so
// the Session has a surface to render onto (all SetContent calls are no-ops).
type dummyScreen struct {
	w, h int
}

func (d *dummyScreen) Init() error                                    { return nil }
func (d *dummyScreen) Fini()                                          {}
func (d *dummyScreen) SetSize(w, h int)                               { d.w, d.h = w, h }
func (d *dummyScreen) SetContent(int, int, rune, []rune, tcell.Style) {}
func (d *dummyScreen) Size() (int, int)                               { return d.w, d.h }

func main() {
	scr := &dummyScreen{}
	_ = scr.Init()
	defer scr.Fini()
	scr.SetSize(4, 1)

	vt := terminal.NewWithSize(4, 1)
	vt.SetSurface(scr)
	vt.SetMaxClipboardLen(1024)

	got := make(chan string, 1)
	vt.Attach(func(ev tcell.Event) {
		if c, ok := ev.(*terminal.EventClipboard); ok {
			select {
			case got <- c.Data():
			default:
			}
		}
	})

	pr, pw := io.Pipe()
	if err := vt.StartWithPty(discardWriter{Reader: pr}); err != nil {
		panic(err)
	}
	defer vt.Close()

	// Send OSC 52 through the "master" side of the pipe.
	// Payload "aGk=" is base64 for "hi".
	_, _ = pw.Write([]byte("\x1b]52;c;aGk=\x1b\\"))

	select {
	case data := <-got:
		fmt.Printf("received clipboard: %q\n", data)
	case <-time.After(2 * time.Second):
		fmt.Println("timeout waiting for EventClipboard")
	}

	_ = pw.Close()
}

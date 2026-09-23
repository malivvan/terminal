// Package main contrasts the two ways to learn when a terminal needs
// repainting:
//
//   - SetRedrawHandler: a lightweight callback fired for every EventRedraw,
//     ideal when all you need is "call Draw() now".
//   - Attach: a full tcell.Event stream (EventRedraw, EventClosed, EventBell,
//     EventTitle, …) when you need to react to more than redraws.
//
// Run with:
//
//	go run ./demo/19_redraw_handler_loop
package main

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

type pipePTY struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newPipePTY() *pipePTY {
	r, w := io.Pipe()
	return &pipePTY{r: r, w: w}
}

func (p *pipePTY) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipePTY) Write(b []byte) (int, error) { return len(b), nil }
func (p *pipePTY) Close() error                { _ = p.w.Close(); return p.r.Close() }
func (p *pipePTY) feed(s string)               { _, _ = io.WriteString(p.w, s) }

type nullSurface struct{ w, h int }

func (s nullSurface) SetContent(int, int, rune, []rune, tcell.Style) {}
func (s nullSurface) Size() (int, int)                               { return s.w, s.h }

func main() {
	vt := terminal.New()
	vt.SetSurface(nullSurface{w: 40, h: 10})

	var redraws int64
	var events int64
	var closed int64

	// (1) The redraw handler — the minimal "please repaint" signal.
	vt.SetRedrawHandler(func() {
		atomic.AddInt64(&redraws, 1)
	})

	// (2) The full event stream via Attach — richer, but you must
	//     type-switch on the events you care about.
	vt.Attach(func(ev tcell.Event) {
		atomic.AddInt64(&events, 1)
		switch ev.(type) {
		case *terminal.EventClosed:
			atomic.AddInt64(&closed, 1)
		}
	})

	pty := newPipePTY()
	if err := vt.StartWithPty(pty); err != nil {
		panic(err)
	}
	defer vt.Close()

	for i := 0; i < 5; i++ {
		pty.feed(fmt.Sprintf("frame %d\r\n", i))
		time.Sleep(30 * time.Millisecond)
	}
	// Close the write side to end the input stream -> EventClosed via Attach.
	_ = pty.w.Close()
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("SetRedrawHandler fired: %d times (repaint signals)\n", atomic.LoadInt64(&redraws))
	fmt.Printf("Attach handler saw:    %d events (redraw + lifecycle)\n", atomic.LoadInt64(&events))
	fmt.Printf("EventClosed observed:  %v (only visible via Attach)\n", atomic.LoadInt64(&closed) > 0)
}

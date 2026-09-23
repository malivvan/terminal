// Package main demonstrates the terminal's panic-recovery path. The parse
// goroutine runs under a deferred recover(): if any callback it drives (here a
// deliberately buggy redraw handler) panics, the terminal captures the panic,
// posts an internal EventPanic (carrying the stack trace), and closes itself —
// the host process keeps running instead of crashing.
//
// Run with:
//
//	go run ./demo/35_error_recovery_panic_event
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
	fmt.Println("EventPanic anatomy (as posted internally by recover()):")
	sample := &terminal.EventPanic{Error: fmt.Errorf("example: index out of range")}
	fmt.Printf("  type=%T  Error=%q\n\n", sample, sample.Error)

	vt := terminal.New()
	vt.SetSurface(nullSurface{w: 40, h: 10})

	var redraws int64
	// A buggy redraw handler: it panics on the first repaint. The panic
	// happens inside the terminal's recover()-protected parse goroutine.
	vt.SetRedrawHandler(func() {
		if atomic.AddInt64(&redraws, 1) == 1 {
			panic("boom: simulated failure in redraw handler")
		}
	})
	vt.Attach(func(ev tcell.Event) { /* other events could be handled here */ })

	pty := newPipePTY()
	if err := vt.StartWithPty(pty); err != nil {
		panic(err)
	}

	// Feed enough to trigger multiple redraws (and thus the panic).
	for i := 0; i < 5; i++ {
		pty.feed(fmt.Sprintf("update %d\r\n", i))
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	// The terminal recovered from the panic and closed itself. Writing now
	// fails because the pty was released — proof the goroutine unwound safely.
	if _, err := vt.Write([]byte("still there?")); err != nil {
		fmt.Printf("terminal auto-closed after panic recovery: %v\n", err)
	} else {
		fmt.Println("terminal still open (unexpected)")
	}

	fmt.Println("host process survived the panic — recovery works.")
}

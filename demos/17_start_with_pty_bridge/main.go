// Package main shows how to drive a Session with StartWithPty and a custom
// io.ReadWriteCloser instead of spawning a subprocess. The custom "pty" here
// is a recorder/proxy: it forwards bytes we feed into the terminal and records
// every reply the terminal writes back (e.g. cursor-position reports).
//
// Run with:
//
//	go run ./demo/17_start_with_pty_bridge
package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

// recorderPTY is a minimal io.ReadWriteCloser that acts as a bidirectional
// bridge between the terminal and a "program" we simulate in this demo.
//
//   - The terminal reads program output via Read (fed by feed()).
//   - The terminal writes replies via Write, which we record.
type recorderPTY struct {
	r *io.PipeReader
	w *io.PipeWriter

	mu           sync.Mutex
	fromTerminal bytes.Buffer // replies: terminal -> program
	toTerminal   bytes.Buffer // feed:    program  -> terminal
}

func newRecorderPTY() *recorderPTY {
	r, w := io.Pipe()
	return &recorderPTY{r: r, w: w}
}

func (p *recorderPTY) Read(b []byte) (int, error) { return p.r.Read(b) }

func (p *recorderPTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.fromTerminal.Write(b)
	p.mu.Unlock()
	return len(b), nil
}

func (p *recorderPTY) Close() error {
	_ = p.w.Close()
	return p.r.Close()
}

// feed injects program output bytes into the terminal.
func (p *recorderPTY) feed(s string) {
	p.mu.Lock()
	p.toTerminal.WriteString(s)
	p.mu.Unlock()
	_, _ = io.WriteString(p.w, s)
}

func (p *recorderPTY) replies() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fromTerminal.String()
}

// memSurface is a tiny in-memory Screen so we can display the terminal
// without a real tcell.Screen.
type memSurface struct {
	mu   sync.Mutex
	w, h int
	ch   [][]rune
}

func newMemSurface(w, h int) *memSurface {
	s := &memSurface{w: w, h: h, ch: make([][]rune, h)}
	for y := range s.ch {
		s.ch[y] = make([]rune, w)
	}
	return s
}

func (s *memSurface) SetContent(x, y int, r rune, _ []rune, _ tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	if r == 0 {
		r = ' '
	}
	s.ch[y][x] = r
}

func (s *memSurface) Size() (int, int) { return s.w, s.h }

func (s *memSurface) lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, s.h)
	for y := 0; y < s.h; y++ {
		out[y] = strings.TrimRight(string(s.ch[y]), " ")
	}
	return out
}

func main() {
	srf := newMemSurface(40, 4)
	vt := terminal.New()
	vt.SetSurface(srf)

	bridge := newRecorderPTY()

	// Redraw signalling so we can repaint when the parser reports changes.
	redraw := make(chan struct{}, 1)
	vt.SetRedrawHandler(func() {
		select {
		case redraw <- struct{}{}:
		default:
		}
	})

	if err := vt.StartWithPty(bridge); err != nil {
		panic(err)
	}
	defer vt.Close()

	// The "program" prints a line and then asks for the cursor position
	// (DSR 6). A real program would read the reply; here we record it.
	bridge.feed("Hello from a bridge PTY!\r\n")
	bridge.feed("Querying cursor position... \x1b[6n")

	// Let the parser catch up, draining redraw signals.
	deadline := time.After(300 * time.Millisecond)
loop:
	for {
		select {
		case <-redraw:
		case <-deadline:
			break loop
		}
	}

	vt.Draw()

	fmt.Println("== screen (rendered via custom Screen) ==")
	for _, l := range srf.lines() {
		fmt.Printf("| %-38s |\n", l)
	}

	fmt.Printf("\n== bytes the terminal wrote back to the program ==\n%q\n", bridge.replies())
}

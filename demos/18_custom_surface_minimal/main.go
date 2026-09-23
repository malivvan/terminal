// Package main proves the renderer is backend-independent: the Session only
// needs a Screen with two methods (SetContent + Size). Here we implement a
// tiny custom Screen that renders into a []string "framebuffer" and nothing
// else — no SetContentWidth, no ShowCursor. Draw() gracefully falls back to
// the minimal SetContent path.
//
// Run with:
//
//	go run ./demo/18_custom_surface_minimal
package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

// tinySurface implements ONLY the mandatory terminal.Screen methods.
type tinySurface struct {
	mu   sync.Mutex
	w, h int
	rows [][]rune
}

func newTinySurface(w, h int) *tinySurface {
	s := &tinySurface{w: w, h: h, rows: make([][]rune, h)}
	for y := range s.rows {
		s.rows[y] = make([]rune, w)
		for x := range s.rows[y] {
			s.rows[y][x] = ' '
		}
	}
	return s
}

// SetContent is the only content method required by terminal.Screen.
func (s *tinySurface) SetContent(x, y int, r rune, _ []rune, _ tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	if r == 0 {
		r = ' '
	}
	s.rows[y][x] = r
}

// Size is the second (and last) required method.
func (s *tinySurface) Size() (int, int) { return s.w, s.h }

func (s *tinySurface) render() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	border := "+" + strings.Repeat("-", s.w) + "+\n"
	b.WriteString(border)
	for _, row := range s.rows {
		b.WriteString("|")
		b.WriteString(strings.TrimRight(string(row), " ") +
			strings.Repeat(" ", s.w-len(strings.TrimRight(string(row), " "))))
		b.WriteString("|\n")
	}
	b.WriteString(border)
	return b.String()
}

// pipePTY feeds bytes into the terminal; replies are discarded.
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

func main() {
	srf := newTinySurface(30, 5)

	vt := terminal.New()
	vt.SetSurface(srf)

	if !vt.HasSurface() {
		panic("surface not attached")
	}

	pty := newPipePTY()
	if err := vt.StartWithPty(pty); err != nil {
		panic(err)
	}
	defer vt.Close()

	// Colours and attributes are accepted by SetContent (and ignored by our
	// tiny surface) — the point is that the grid geometry still renders.
	pty.feed("\x1b[1;31mRED\x1b[0m and \x1b[4mplain\x1b[0m\r\n")
	pty.feed("second line\r\n")
	pty.feed("wide: \xe4\xb8\xad\xe6\x96\x87") // 中文 (wide runes)

	time.Sleep(150 * time.Millisecond)
	vt.Draw()

	fmt.Println("Rendered by a 2-method custom Screen:")
	fmt.Print(srf.render())
}

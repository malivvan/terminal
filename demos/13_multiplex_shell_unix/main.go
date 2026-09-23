//go:build unix

// Package main is a compact side-by-side shell multiplexer inspired by
// tmux. It launches two /bin/sh instances, each rendered into its own
// Terminal Session, and composites them into a single tcell.Screen with a
// vertical divider. Keyboard input goes to the focused pane; mouse
// events are routed to whichever pane the click landed in.
//
// Keybindings (Ctrl+B is the prefix):
//
//	Ctrl+B  <Left> / <Right> / 1 / 2   focus a pane
//	Ctrl+B  r                          toggle resize mode
//	  (in resize mode: Left/Right adjust split, Esc leaves)
//	Ctrl+B  d                          quit the multiplexer
//	Ctrl+B  Ctrl+B                     send a literal Ctrl+B to the pane
package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
	"github.com/malivvan/terminal"
)

type pane struct {
	h       *terminal.Terminal
	scr     tcell.Screen
	offsetX int
	width   int
	height  int
}

func newPane(scr tcell.Screen, w, h int) (*pane, error) {
	hv, err := terminal.NewTerminal(w, h)
	if err != nil {
		return nil, err
	}
	if err := hv.Session().Start(exec.Command("/bin/sh")); err != nil {
		hv.Close()
		return nil, err
	}
	return &pane{h: hv, scr: scr, width: w, height: h}, nil
}

func (p *pane) close() {
	if p != nil {
		p.h.Close()
	}
}

func (p *pane) resize(w, h int) {
	p.width, p.height = w, h
	p.h.Resize(w, h)
}

func (p *pane) draw() {
	for y := 0; y < p.height; y++ {
		for x := 0; x < p.width; x++ {
			c := p.h.Cell(x, y)
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			p.scr.SetContent(p.offsetX+x, y, r, c.Combining, c.Style)
		}
	}
}

func drawDivider(scr tcell.Screen, x, height int, focused bool) {
	st := tcell.StyleDefault.Foreground(tcellcolor.Gray)
	if focused {
		st = st.Foreground(tcellcolor.Yellow)
	}
	for y := 0; y < height; y++ {
		scr.SetContent(x, y, '│', nil, st)
	}
}

func drawStatus(scr tcell.Screen, y, width int, focused int, resize bool) {
	msg := fmt.Sprintf(" terminal-mux | focus=pane%d | Ctrl+B d = quit | Ctrl+B r = resize", focused+1)
	if resize {
		msg = " terminal-mux | RESIZE MODE | Left/Right adjust, Esc leaves "
	}
	st := tcell.StyleDefault.Reverse(true)
	x := 0
	for _, r := range msg {
		if x >= width {
			break
		}
		scr.SetContent(x, y, r, nil, st)
		x++
	}
	for ; x < width; x++ {
		scr.SetContent(x, y, ' ', nil, st)
	}
}

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

	W, H := scr.Size()
	statusH := 1
	paneH := H - statusH
	dividerX := W / 2
	leftW := dividerX
	rightW := W - dividerX - 1

	left, err := newPane(scr, leftW, paneH)
	if err != nil {
		log.Fatal(err)
	}
	defer left.close()
	right, err := newPane(scr, rightW, paneH)
	if err != nil {
		log.Fatal(err)
	}
	defer right.close()
	right.offsetX = dividerX + 1

	redraw := make(chan struct{}, 16)
	kick := func(ev tcell.Event) {
		select {
		case redraw <- struct{}{}:
		default:
		}
	}
	left.h.Session().Attach(kick)
	right.h.Session().Attach(kick)

	focused := 0
	panes := []*pane{left, right}
	resizeMode := false

	tcellEvs := make(chan tcell.Event, 16)
	go func() {
		for ev := range scr.EventQ() {
			tcellEvs <- ev
		}
	}()

	prefix := false
	render := func() {
		panes[0].draw()
		panes[1].draw()
		drawDivider(scr, dividerX, paneH, !resizeMode)
		drawStatus(scr, paneH, W, focused, resizeMode)
		// Show cursor in the focused pane.
		if cx, cy, vis := panes[focused].h.Cursor(); vis {
			scr.ShowCursor(panes[focused].offsetX+cx, cy)
		} else {
			scr.HideCursor()
		}
		scr.Show()
	}
	render()

loop:
	for {
		select {
		case <-redraw:
			render()
		case ev := <-tcellEvs:
			switch e := ev.(type) {
			case *tcell.EventResize:
				W, H = e.Size()
				paneH = H - statusH
				dividerX = W / 2
				leftW = dividerX
				rightW = W - dividerX - 1
				panes[0].resize(leftW, paneH)
				panes[1].resize(rightW, paneH)
				panes[1].offsetX = dividerX + 1
				render()
			case *tcell.EventKey:
				if prefix {
					prefix = false
					switch {
					case e.Key() == tcell.KeyRune && e.Str() == "d":
						break loop
					case e.Key() == tcell.KeyRune && e.Str() == "r":
						resizeMode = !resizeMode
						render()
					case e.Key() == tcell.KeyLeft:
						focused = 0
						render()
					case e.Key() == tcell.KeyRight:
						focused = 1
						render()
					case e.Key() == tcell.KeyRune && e.Str() == "1":
						focused = 0
						render()
					case e.Key() == tcell.KeyRune && e.Str() == "2":
						focused = 1
						render()
					case e.Key() == tcell.KeyCtrlB || (e.Modifiers()&tcell.ModCtrl != 0 && e.Str() == "b"):
						panes[focused].h.HandleEvent(e)
					}
					continue
				}
				if resizeMode {
					switch e.Key() {
					case tcell.KeyEsc:
						resizeMode = false
						render()
					case tcell.KeyLeft:
						if dividerX > 4 {
							dividerX--
							leftW = dividerX
							rightW = W - dividerX - 1
							panes[0].resize(leftW, paneH)
							panes[1].resize(rightW, paneH)
							panes[1].offsetX = dividerX + 1
							render()
						}
					case tcell.KeyRight:
						if dividerX < W-4 {
							dividerX++
							leftW = dividerX
							rightW = W - dividerX - 1
							panes[0].resize(leftW, paneH)
							panes[1].resize(rightW, paneH)
							panes[1].offsetX = dividerX + 1
							render()
						}
					}
					continue
				}
				if e.Key() == tcell.KeyCtrlB || (e.Modifiers()&tcell.ModCtrl != 0 && e.Str() == "b") {
					prefix = true
					continue
				}
				panes[focused].h.HandleEvent(e)
			case *tcell.EventMouse:
				x, y := e.Position()
				if y >= paneH {
					continue
				}
				which := 0
				if x > dividerX {
					which = 1
				} else if x == dividerX {
					continue
				}
				local := tcell.NewEventMouse(x-panes[which].offsetX, y, e.Buttons(), e.Modifiers())
				panes[which].h.HandleEvent(local)
				if focused != which {
					focused = which
					render()
				}
			case *tcell.EventPaste:
				panes[focused].h.HandleEvent(e)
			case *tcell.EventFocus:
				panes[0].h.HandleEvent(e)
				panes[1].h.HandleEvent(e)
			}
		}
	}
}

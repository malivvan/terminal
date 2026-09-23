//go:build js && wasm

// Package main is a js/wasm bridge demo. It drives the terminal from a host
// page through a JavaScript bridge object published on globalThis.__pty. The
// bridge is implemented by this demo (see bridge.go): on js/wasm there is no
// operating system terminal, only the browser.
//
//	globalThis.__pty.write("ls -la\r\n");   // feed program output/input
//	globalThis.__pty.read(chunk => ...);    // receive terminal replies
//	globalThis.__terminalScreen();               // read back the rendered screen
//	globalThis.__terminalResize(cols, rows);     // resize the terminal
//
// Build with:
//
//	GOOS=js GOARCH=wasm go build -o terminal.wasm ./demo/31_js_wasm_pty_bridge
package main

import (
	"strings"
	"sync"
	"syscall/js"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/terminal"
)

type memSurface struct {
	mu   sync.Mutex
	w, h int
	rows [][]rune
}

func newMemSurface(w, h int) *memSurface {
	s := &memSurface{w: w, h: h, rows: make([][]rune, h)}
	for y := range s.rows {
		s.rows[y] = make([]rune, w)
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
	s.rows[y][x] = r
}

func (s *memSurface) Size() (int, int) { return s.w, s.h }

func (s *memSurface) lines() []interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]interface{}, s.h)
	for y := 0; y < s.h; y++ {
		out[y] = strings.TrimRight(string(s.rows[y]), " ")
	}
	return out
}

func main() {
	srf := newMemSurface(80, 24)

	vt := terminal.New()
	vt.SetSurface(srf)

	// Create the js/wasm pty; this publishes the bridge on globalThis.__pty.
	p := newJSPTY()

	if err := vt.StartWithPty(p); err != nil {
		panic(err)
	}

	// Repaint on every change.
	vt.SetRedrawHandler(func() { vt.Draw() })

	// Expose a helper to read the current screen back into JavaScript.
	js.Global().Set("__terminalScreen", js.FuncOf(func(this js.Value, args []js.Value) any {
		vt.Draw()
		return js.ValueOf(srf.lines())
	}))

	// Expose a resize helper.
	js.Global().Set("__terminalResize", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 2 {
			cols, rows := args[0].Int(), args[1].Int()
			srf = newMemSurface(cols, rows)
			vt.SetSurface(srf)
			vt.Resize(cols, rows)
			vt.Draw()
		}
		return js.Undefined()
	}))

	// Block forever so the Go runtime stays alive to service the bridge.
	select {}
}

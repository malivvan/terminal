//go:build js && wasm

package main

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall/js"
)

// jsPTY is the js/wasm pseudo-terminal of this demo.
//
// It used to come from the terminal's own pty package, which published a
// JavaScript bridge on globalThis.__pty. That package is gone, and
// github.com/malivvan/pty has no js/wasm support, so the demo owns the bridge:
// on js/wasm there is no operating system terminal to open, only a host page to
// talk to.
//
// jsPTY is a duplex stream between that host page and the terminal. Bytes the
// host writes with __pty.write are handed to the terminal as its input, and
// bytes the terminal writes are delivered to the callbacks registered with
// __pty.read.
//
// The bridge object is published on globalThis.__pty:
//
//	__pty.write(data)          // feed input/output into the terminal
//	__pty.read(cb)             // receive terminal output; returns a disposer
//	__pty.onResize(cb)         // observe resizes; returns a disposer
//	__pty.close()              // tear the bridge down
//	__pty.cols, __pty.rows     // current terminal size
type jsPTY struct {
	bridge  js.Value
	inbound *byteQueue

	funcsMu sync.Mutex
	funcs   []js.Func

	mu         sync.Mutex
	readSubs   []js.Value
	resizeSubs []js.Value
	pending    []byte
	width      int
	height     int

	closed    atomic.Bool
	closeOnce sync.Once
}

// newJSPTY creates the pseudo-terminal and publishes it on globalThis.__pty.
func newJSPTY() *jsPTY {
	p := &jsPTY{inbound: newByteQueue()}
	p.bridge = p.buildBridge()
	js.Global().Set("__pty", p.bridge)
	return p
}

func (p *jsPTY) buildBridge() js.Value {
	obj := js.Global().Get("Object").New()

	p.addFunc(obj, "write", func(this js.Value, args []js.Value) any {
		if p.closed.Load() {
			return jsError("pty: closed")
		}
		if len(args) == 0 {
			return js.Undefined()
		}
		b, err := jsValueToBytes(args[0])
		if err != nil {
			return jsError("pty: " + err.Error())
		}
		if _, werr := p.inbound.Write(b); werr != nil {
			return jsError("pty: " + werr.Error())
		}
		return js.ValueOf(len(b))
	})

	p.addFunc(obj, "read", func(this js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].Type() != js.TypeFunction {
			return jsError("pty.read: callback function required")
		}
		cb := args[0]
		var pending []byte
		p.mu.Lock()
		if p.closed.Load() {
			p.mu.Unlock()
			return jsError("pty: closed")
		}
		p.readSubs = append(p.readSubs, cb)
		if len(p.pending) > 0 {
			pending = p.pending
			p.pending = nil
		}
		p.mu.Unlock()
		if len(pending) > 0 {
			invokeJSCallback(cb, bytesToUint8Array(pending))
		}
		return p.makeDisposer(func() { p.removeReadSub(cb) })
	})

	p.addFunc(obj, "onResize", func(this js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].Type() != js.TypeFunction {
			return jsError("pty.onResize: callback function required")
		}
		cb := args[0]
		p.mu.Lock()
		if p.closed.Load() {
			p.mu.Unlock()
			return jsError("pty: closed")
		}
		p.resizeSubs = append(p.resizeSubs, cb)
		w, h := p.width, p.height
		p.mu.Unlock()
		if w > 0 || h > 0 {
			invokeJSCallback(cb, js.ValueOf(w), js.ValueOf(h))
		}
		return p.makeDisposer(func() { p.removeResizeSub(cb) })
	})

	p.addFunc(obj, "close", func(this js.Value, args []js.Value) any {
		_ = p.Close()
		return js.Undefined()
	})

	p.installSizeAccessors(obj)
	return obj
}

func (p *jsPTY) installSizeAccessors(obj js.Value) {
	defineProp := js.Global().Get("Object").Get("defineProperty")
	if !defineProp.Truthy() {
		return
	}

	colsGetter := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.width
	})
	p.trackFunc(colsGetter)

	rowsGetter := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.height
	})
	p.trackFunc(rowsGetter)

	descCols := js.Global().Get("Object").New()
	descCols.Set("get", colsGetter)
	descCols.Set("configurable", true)
	defineProp.Invoke(obj, "cols", descCols)

	descRows := js.Global().Get("Object").New()
	descRows.Set("get", rowsGetter)
	descRows.Set("configurable", true)
	defineProp.Invoke(obj, "rows", descRows)
}

func (p *jsPTY) addFunc(obj js.Value, name string, fn func(this js.Value, args []js.Value) any) {
	f := js.FuncOf(fn)
	p.trackFunc(f)
	obj.Set(name, f)
}

func (p *jsPTY) trackFunc(f js.Func) {
	p.funcsMu.Lock()
	p.funcs = append(p.funcs, f)
	p.funcsMu.Unlock()
}

func (p *jsPTY) makeDisposer(fn func()) js.Value {
	once := &sync.Once{}
	f := js.FuncOf(func(this js.Value, args []js.Value) any {
		once.Do(fn)
		return js.Undefined()
	})
	p.trackFunc(f)
	return f.Value
}

// Read returns what the host page wrote with __pty.write. It blocks until data
// is available or the pty is closed.
func (p *jsPTY) Read(b []byte) (int, error) { return p.inbound.Read(b) }

// Write hands the terminal's output to every read callback registered by the
// host page. Output written before the first callback is buffered and delivered
// to the next one.
func (p *jsPTY) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	p.mu.Lock()
	if p.closed.Load() {
		p.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	subs := append([]js.Value(nil), p.readSubs...)
	if len(subs) == 0 {
		p.pending = append(p.pending, b...)
		p.mu.Unlock()
		return len(b), nil
	}
	p.mu.Unlock()

	chunk := bytesToUint8Array(b)
	for _, cb := range subs {
		invokeJSCallback(cb, chunk)
	}
	return len(b), nil
}

// Resize records the new size and notifies the host page.
func (p *jsPTY) Resize(width, height int) error {
	p.mu.Lock()
	if p.closed.Load() {
		p.mu.Unlock()
		return io.ErrClosedPipe
	}
	p.width, p.height = width, height
	subs := append([]js.Value(nil), p.resizeSubs...)
	p.mu.Unlock()

	for _, cb := range subs {
		invokeJSCallback(cb, js.ValueOf(width), js.ValueOf(height))
	}
	return nil
}

// Close tears the bridge down and removes it from globalThis.
func (p *jsPTY) Close() error {
	var err error
	p.closeOnce.Do(func() {
		p.closed.Store(true)
		if cerr := p.inbound.Close(); cerr != nil {
			err = cerr
		}

		p.mu.Lock()
		p.readSubs = nil
		p.resizeSubs = nil
		p.pending = nil
		p.mu.Unlock()

		if g := js.Global().Get("__pty"); g.Truthy() && g.Equal(p.bridge) {
			js.Global().Delete("__pty")
		}

		p.funcsMu.Lock()
		for _, f := range p.funcs {
			f.Release()
		}
		p.funcs = nil
		p.funcsMu.Unlock()
		p.bridge = js.Undefined()
	})
	return err
}

func (p *jsPTY) removeReadSub(cb js.Value) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.readSubs[:0]
	for _, c := range p.readSubs {
		if !c.Equal(cb) {
			out = append(out, c)
		}
	}
	for i := len(out); i < len(p.readSubs); i++ {
		p.readSubs[i] = js.Value{}
	}
	p.readSubs = out
}

func (p *jsPTY) removeResizeSub(cb js.Value) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.resizeSubs[:0]
	for _, c := range p.resizeSubs {
		if !c.Equal(cb) {
			out = append(out, c)
		}
	}
	for i := len(out); i < len(p.resizeSubs); i++ {
		p.resizeSubs[i] = js.Value{}
	}
	p.resizeSubs = out
}

// byteQueue is an unbounded byte buffer with a blocking Read, so that the
// terminal can read the host page's input without busy waiting.
type byteQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []byte
	closed bool
}

func newByteQueue() *byteQueue {
	q := &byteQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *byteQueue) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return 0, io.ErrClosedPipe
	}
	q.buf = append(q.buf, p...)
	q.cond.Broadcast()
	return len(p), nil
}

func (q *byteQueue) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.buf) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, q.buf)
	q.buf = q.buf[n:]
	return n, nil
}

func (q *byteQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	q.cond.Broadcast()
	return nil
}

// jsValueToBytes converts the value the host page passed to __pty.write into
// bytes: a string, a number, a typed array, an ArrayBuffer, or any array-like
// object of byte values.
func jsValueToBytes(v js.Value) ([]byte, error) {
	switch v.Type() {
	case js.TypeUndefined, js.TypeNull:
		return nil, nil
	case js.TypeString:
		return []byte(v.String()), nil
	case js.TypeNumber:
		return []byte{byte(v.Int())}, nil
	case js.TypeObject:
		ctor := v.Get("constructor")
		ctorName := ""
		if ctor.Truthy() {
			ctorName = ctor.Get("name").String()
		}
		switch ctorName {
		case "Uint8Array", "Uint8ClampedArray":
			n := v.Get("length").Int()
			b := make([]byte, n)
			js.CopyBytesToGo(b, v)
			return b, nil
		case "ArrayBuffer":
			view := js.Global().Get("Uint8Array").New(v)
			n := view.Get("length").Int()
			b := make([]byte, n)
			js.CopyBytesToGo(b, view)
			return b, nil
		case "Int8Array", "Int16Array", "Uint16Array", "Int32Array", "Uint32Array",
			"Float32Array", "Float64Array", "DataView":
			buf := v.Get("buffer")
			byteOffset := v.Get("byteOffset").Int()
			byteLen := v.Get("byteLength").Int()
			view := js.Global().Get("Uint8Array").New(buf, byteOffset, byteLen)
			b := make([]byte, byteLen)
			js.CopyBytesToGo(b, view)
			return b, nil
		}
		if length := v.Get("length"); length.Type() == js.TypeNumber {
			n := length.Int()
			b := make([]byte, n)
			for i := 0; i < n; i++ {
				b[i] = byte(v.Index(i).Int())
			}
			return b, nil
		}
		return nil, fmt.Errorf("cannot convert object of type %s to bytes", ctorName)
	}
	return nil, fmt.Errorf("cannot convert JS value of kind %v to bytes", v.Type())
}

func bytesToUint8Array(b []byte) js.Value {
	arr := js.Global().Get("Uint8Array").New(len(b))
	if len(b) > 0 {
		js.CopyBytesToJS(arr, b)
	}
	return arr
}

func jsError(msg string) js.Value {
	return js.Global().Get("Error").New(msg)
}

// invokeJSCallback calls a host page callback, swallowing any exception it
// throws so that a faulty callback cannot take the terminal down.
func invokeJSCallback(cb js.Value, args ...any) {
	defer func() { _ = recover() }()
	cb.Invoke(args...)
}

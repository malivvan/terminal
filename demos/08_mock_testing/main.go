// Package main shows the MockTerminal helper being driven the same way
// a unit test would drive it. Running it as a program simply invokes
// the assertions in a fake T that panics on failure.
package main

import (
	"fmt"

	"github.com/malivvan/terminal"
)

// panicT implements the helper interface used by terminal.MockTerminal and turns failures into panics.
type panicT struct{}

func (panicT) Helper() {}
func (panicT) Errorf(format string, args ...interface{}) {
	panic(fmt.Sprintf("assertion failed: "+format, args...))
}
func (panicT) Fatalf(format string, args ...interface{}) {
	panic(fmt.Sprintf("fatal: "+format, args...))
}

func main() {
	m := terminal.NewMockTerminal(panicT{}, 10, 1)
	defer m.Close()

	m.Feed("hi")
	m.AssertCell(0, 0, 'h')
	m.AssertCell(1, 0, 'i')
	m.AssertCursor(2, 0)
	m.AssertContains("hi")
	fmt.Println("all assertions passed")
	fmt.Println("screen dump:\n" + m.Dump())
}

// Package main uses terminal.NewParser directly to inspect the Sequence stream a
// byte buffer decodes into. This is the same VT500-series parser the terminal
// uses internally; running it standalone is handy for debugging escape
// sequences without a full terminal.
//
// Run with:
//
//	go run ./demo/23_parser_stream_inspector
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

func main() {
	// A mix of printable text, C0 controls, ESC, CSI (SGR + cursor move),
	// an OSC title, and a DCS passthrough sequence.
	input := "Hi\r\n" +
		"\x1b[1;32mgreen\x1b[0m" + // SGR
		"\x1b[10;5H" + // CSI cursor position
		"\x1b]0;window title\x07" + // OSC 0 (BEL-terminated)
		"\x1bP1;2|data\x1b\\" // DCS ... ST

	p := terminal.NewParser(strings.NewReader(input))

	fmt.Printf("input: %q\n\n", input)
	fmt.Println("parsed sequence stream:")
	for {
		seq := p.Next()
		switch s := seq.(type) {
		case terminal.Print:
			fmt.Printf("  Print        %q\n", rune(s))
		case terminal.C0:
			fmt.Printf("  C0           0x%02X\n", rune(s))
		case terminal.ESC:
			fmt.Printf("  ESC          inter=%q final=%q\n", string(s.Intermediate), s.Final)
		case terminal.CSI:
			fmt.Printf("  CSI          inter=%q params=%v final=%q\n", string(s.Intermediate), s.Parameters, s.Final)
		case terminal.OSC:
			fmt.Printf("  OSC          payload=%q\n", string(s.Payload))
		case terminal.DCS:
			fmt.Printf("  DCS start    inter=%q params=%v final=%q\n", string(s.Intermediate), s.Parameters, s.Final)
		case terminal.DCSData:
			fmt.Printf("  DCSData      %q\n", rune(s))
		case terminal.DCSEndOfData:
			fmt.Printf("  DCSEndOfData\n")
		case terminal.EOF:
			fmt.Println("  EOF")
			return
		case error:
			fmt.Printf("  error        %v\n", s)
		default:
			fmt.Printf("  <unknown %T>\n", s)
		}
	}
}

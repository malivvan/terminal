// Package main issues XTWINOPS window queries (CSI 14 t, CSI 18 t) plus a few
// related reports and inspects the byte sequences the terminal emits in
// response. Terminal captures those bytes via Output().
//
// Run with:
//
//	go run ./demo/26_xtwinops_reports
package main

import (
	"fmt"

	"github.com/malivvan/terminal"
)

func query(h *terminal.Terminal, name, seq string) {
	_ = h.Output() // clear any pending bytes
	_, _ = h.FeedString(seq)
	fmt.Printf("%-28s %-12q -> %q\n", name, seq, string(h.Output()))
}

func main() {
	h, err := terminal.NewTerminal(80, 24)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	fmt.Printf("terminal is %d columns x %d rows\n\n", 80, 24)

	fmt.Println("query                        request       response")
	fmt.Println("---------------------------------------------------------------")
	// XTWINOPS 18t: report text area size in characters -> CSI 8 ; rows ; cols t
	query(h, "XTWINOPS size (chars) 18t", "\x1b[18t")
	// XTWINOPS 14t: report text area size in pixels -> CSI 4 ; height ; width t
	query(h, "XTWINOPS size (pixels) 14t", "\x1b[14t")
	// Primary Device Attributes.
	query(h, "Primary DA (CSI c)", "\x1b[c")
	// Secondary Device Attributes.
	query(h, "Secondary DA (CSI > c)", "\x1b[>c")
	// Cursor Position Report after moving the cursor.
	query(h, "Cursor Position (CSI 6n)", "\x1b[10;20H\x1b[6n")

	fmt.Println("\nResponses would normally be read by the client program on the PTY.")
}

// Package main shows a minimal record/replay workflow built on the current
// public APIs: a Recording captures everything fed to a Terminal
// terminal, ExportRaw serializes it, and a Player replays the frames
// into a fresh terminal so the final screen is reproduced exactly.
//
// Run with:
//
//	go run ./demo/28_session_record_replay_mock
package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/malivvan/terminal"
)

func main() {
	// --- record ---
	src, err := terminal.NewTerminal(30, 4)
	if err != nil {
		panic(err)
	}
	defer src.Close()

	rec := terminal.NewRecorder(src)
	_, _ = rec.FeedString("\x1b[1;36mrecording session\x1b[0m\r\n")
	_, _ = rec.FeedString("step 1: init\r\n")
	_, _ = rec.FeedString("step 2: \x1b[32mok\x1b[0m")
	_ = rec.Close()

	data, err := rec.ExportRaw()
	if err != nil {
		panic(err)
	}
	fmt.Printf("recorded %d bytes of session JSON\n\n", len(data))

	original := src.String()

	// --- replay ---
	player, err := terminal.NewPlayer(data)
	if err != nil {
		panic(err)
	}
	dst, err := terminal.NewTerminal(30, 4)
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	cur, total := player.Pos()
	fmt.Printf("replaying %d frames...\n", total-cur)
	for {
		f, err := player.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if f.EventType == "input" && len(f.Input) > 0 {
			_, _ = dst.Feed(f.Input)
		}
	}

	// --- verify ---
	replayed := dst.String()
	fmt.Println("\noriginal screen:")
	printScreen(original)
	fmt.Println("\nreplayed screen:")
	printScreen(replayed)
	fmt.Printf("\nidentical: %v\n", original == replayed)
}

func printScreen(s string) {
	for _, l := range strings.Split(s, "\n") {
		fmt.Printf("  |%s|\n", strings.TrimRight(l, " "))
	}
}

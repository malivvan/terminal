// Package main is a live dashboard of terminal mode state. As various private
// modes are toggled via DECSET/DECRST, it prints the type-safe snapshot
// returned by GetModes — the same view an automation agent would use to probe
// terminal capabilities.
//
// Run with:
//
//	go run ./demo/27_mode_probe_dashboard
package main

import (
	"fmt"

	"github.com/malivvan/terminal"
)

func dashboard(h *terminal.Terminal, step string) {
	m := h.GetModes()
	fmt.Printf("== %s ==\n", step)
	fmt.Printf("  mouse=%-12s bracketed-paste=%v  focus-events=%v\n", m.MouseMode, m.BracketedPaste, m.FocusEvents)
	fmt.Printf("  alt-screen=%-6v sync-output=%v      reverse-video=%v\n", m.AltScreen, m.SynchronizedOutput, m.ReverseVideo)
	fmt.Printf("  autowrap=%-8v origin=%v            app-cursor-keys=%v\n", m.Autowrap, m.OriginMode, m.CursorKeyMode)
	fmt.Printf("  insert=%-10v cursor-visible=%v\n\n", m.InsertMode, m.CursorVisible)
}

func main() {
	h, err := terminal.NewTerminal(40, 6)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	dashboard(h, "defaults")

	steps := []struct {
		desc string
		seq  string
	}{
		{"enable SGR mouse (1000+1006)", "\x1b[?1000h\x1b[?1006h"},
		{"enable bracketed paste (2004)", "\x1b[?2004h"},
		{"enable focus events (1004)", "\x1b[?1004h"},
		{"enter alt-screen (1049)", "\x1b[?1049h"},
		{"enable synchronized output (2026)", "\x1b[?2026h"},
		{"reverse video on (5), hide cursor (25l)", "\x1b[?5h\x1b[?25l"},
		{"reset everything", "\x1b[?1000l\x1b[?1006l\x1b[?2004l\x1b[?1004l\x1b[?1049l\x1b[?2026l\x1b[?5l\x1b[?25h"},
	}
	for _, s := range steps {
		_, _ = h.FeedString(s.seq)
		dashboard(h, s.desc)
	}
}

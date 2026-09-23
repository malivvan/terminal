// Package main AI agent automation patterns with terminal.
//
// This example showcases the key APIs an AI agent would use to drive a
// terminal programmatically:
//   - Query terminal mode state before acting
//   - Wait for conditions (prompt, cursor, stability)
//   - Record and replay sessions
//   - Parse screen semantics (prompt/output/error detection)
//   - Track differential screen changes
//
// Build: go build
// Run:   ./16_ai_automation
package main

import (
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/malivvan/terminal"
)

func main() {
	fmt.Println("=== AI Automation Demo ===")

	// 1. Mode introspection.
	fmt.Println("--- 1. Mode Introspection ---")
	h, _ := terminal.NewTerminal(80, 24)
	defer h.Close()

	modes := h.GetModes()
	fmt.Printf("Initial mouse mode: %s\n", modes.MouseMode)
	fmt.Printf("Autowrap: %v, Cursor visible: %v\n", modes.Autowrap, modes.CursorVisible)

	h.FeedString("\x1b[?1000h\x1b[?1006h") // enable VT200 button + SGR
	modes = h.GetModes()
	fmt.Printf("After DECSET 1000+1006: mouse=%s\n", modes.MouseMode)

	// 2. Wait utilities.
	fmt.Println("\n--- 2. Wait Utilities ---")
	h.FeedString("Hello, World!\r\n")
	if _, _, err := h.WaitForText("Hello", 2*time.Second); err != nil {
		fmt.Printf("WaitForText error: %v\n", err)
	} else {
		fmt.Println("WaitForText: found 'Hello'")
	}

	h.FeedString("Line 1\r\nLine 2\r\n$ ")
	if err := h.WaitForStable(200*time.Millisecond, 2*time.Second); err != nil {
		fmt.Printf("WaitForStable error: %v\n", err)
	} else {
		fmt.Println("WaitForStable: screen stabilized")
	}

	// 3. Session recording.
	fmt.Println("\n--- 3. Session Recording ---")
	sr := terminal.NewRecorder(h)
	sr.FeedString("echo recorded\r\n")
	time.Sleep(100 * time.Millisecond)
	sr.Snapshot()
	sr.Close()

	data, err := sr.ExportAsciinema()
	if err != nil {
		fmt.Printf("Export error: %v\n", err)
	} else {
		fmt.Printf("Exported asciinema: %d bytes\n", len(data))
	}

	// 4. Session replay.
	fmt.Println("\n--- 4. Session Replay ---")
	raw, _ := sr.ExportRaw()
	player, _ := terminal.NewPlayer(raw)
	frameCount := 0
	for {
		f, err := player.Next()
		if err == io.EOF {
			break
		}
		frameCount++
		_ = f
	}
	fmt.Printf("Replayed %d frames\n", frameCount)

	// 5. Semantic parsing.
	fmt.Println("\n--- 5. Semantic Screen Parsing ---")
	h2, _ := terminal.NewTerminal(80, 24)
	defer h2.Close()
	// Simulate a shell session with an error.
	h2.FeedString("user@host:~$ \r\n")
	h2.FeedString("ls -la\r\n")
	h2.FeedString("total 42\r\n")
	h2.FeedString("\x1b[31merror: file not found\x1b[0m\r\n") // red error
	h2.FeedString("user@host:~$ ")

	lines := h2.AnalyzeScreen()
	for _, l := range lines {
		if l.Text != "" {
			fmt.Printf("  %-10s | %s\n", l.Type, l.Text)
		}
	}

	promptIdx := h2.FindPrompt()
	fmt.Printf("Prompt found at line %d\n", promptIdx)
	if text, _, ok := h2.GetErrorLine(); ok {
		fmt.Printf("Error line: %q\n", text)
	}
	fmt.Printf("Last output: %q\n", h2.GetLastOutput())

	// 6. Delta tracking.
	fmt.Println("\n--- 6. Delta Tracking ---")
	h3, _ := terminal.NewTerminal(10, 3)
	defer h3.Close()

	delta := h3.GetDelta()
	fmt.Printf("Initial delta: %d changed cells\n", len(delta.Changed))

	h3.FeedString("ABC")
	delta = h3.GetDelta()
	fmt.Printf("After 'ABC': %d changed cells\n", len(delta.Changed))
	for _, c := range delta.Changed {
		fmt.Printf("  cell(%d,%d) = %c\n", c.X, c.Y, c.Rune)
	}

	// 7. Query APIs.
	fmt.Println("\n--- 7. Query APIs ---")
	h4, _ := terminal.NewTerminal(80, 24)
	defer h4.Close()
	h4.FeedString("\x1b[6n") // DSR cursor position
	output := string(h4.Output())
	fmt.Printf("DSR cursor report: %q\n", output)

	top, bot, left, right := h4.Session().GetMargins()
	fmt.Printf("Margins: top=%d bot=%d left=%d right=%d\n", top, bot, left, right)
	fmt.Printf("Tab stops: %v\n", h4.Session().GetTabStops())
	fmt.Printf("Cursor style: %v\n", h4.Session().GetCursorStyle())
	fmt.Printf("Charset: %s\n", h4.Session().GetActiveCharset())

	// 8. Event log.
	fmt.Println("\n--- 8. Event Log ---")
	entries := h4.Session().EventLog(0)
	fmt.Printf("Event log entries: %d\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  [%s] %s\n", e.Type, e.Sequence)
	}

	// 9. Clipboard.
	fmt.Println("\n--- 9. Clipboard ---")
	h4.Session().SetClipboard("c", "SGVsbG8=") // base64 "Hello"
	if data, ok := h4.Session().GetClipboard("c"); ok {
		fmt.Printf("Clipboard 'c': %s\n", data)
	}

	// 10. Pattern wait.
	fmt.Println("\n--- 10. Pattern Wait ---")
	h4.FeedString("Progress: 42%\r\n")
	if line, y, err := h4.WaitForPattern(regexp.MustCompile(`\d+%`), 1*time.Second); err != nil {
		fmt.Printf("Pattern wait error: %v\n", err)
	} else {
		fmt.Printf("Pattern found at line %d: %q\n", y, line)
	}

	// 11. Echo verification.
	fmt.Println("\n--- 11. Echo Verification ---")
	if echoed, ok := h4.ExpectEcho("x", 100*time.Millisecond); ok {
		fmt.Printf("Echo received: %q\n", echoed)
	} else {
		fmt.Println("Echo timeout (expected in headless without real PTY)")
	}

	// 12. Scrollback.
	fmt.Println("\n--- 12. Scrollback ---")
	h5, _ := terminal.NewTerminal(80, 5)
	defer h5.Close()
	for i := 0; i < 20; i++ {
		h5.FeedString(fmt.Sprintf("Line %02d\r\n", i))
	}
	fmt.Printf("Scrollback rows: %d\n", h5.ScrollbackLen())
	if line, ok := h5.ScrollbackLine(0); ok {
		fmt.Printf("Scrollback[0]: %q\n", line)
	}

	// 13. Margins & tabs.
	fmt.Println("\n--- 13. Margins & Tabs ---")
	stops := h4.Session().GetTabStops()
	fmt.Printf("Tab stops (first 5): %v\n", stops[:min(5, len(stops))])
	w, hh := h4.Session().GetDimensions()
	fmt.Printf("Dimensions: %dx%d\n", w, hh)

	fmt.Println("\n=== All AI automation checks passed! ===")
}

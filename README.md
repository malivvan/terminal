# `terminal` - a Virtual Terminal Emulator for Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/malivvan/terminal)](https://github.com/malivvan/terminal)
[![License](https://img.shields.io/github/license/malivvan/terminal)](LICENSE)
[![CI](https://github.com/malivvan/terminal/actions/workflows/ci.yml/badge.svg)](https://github.com/malivvan/terminal/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/malivvan/terminal.svg)](https://pkg.go.dev/github.com/malivvan/terminal)

> A high-performance, cross-platform virtual terminal emulator for Go — headless, scriptable, and renderable.

This package started as a fork of [`git.sr.ht/~rockorager/tcell-term`](https://git.sr.ht/~rockorager/tcell-term) (v0.10.0, by Tim Culverhouse) and has since undergone substantial improvements: a richer API surface, headless `Terminal` with synchronous feed, image rendering with custom fonts and colour schemes, session recording and replay (asciinema v2), AI/automation helpers, a comprehensive mock terminal, delta tracking, SSH PTY bridging, WASM support, box-drawing / braille procedural rendering, 8 built-in colour schemes, and 37 runnable demos.

---

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Core Concepts](#core-concepts)
  - [Terminal (Headless)](#terminal-headless)
  - [Session (PTY-backed)](#session-pty-backed)
  - [Parser](#parser)
  - [Rendering](#rendering)
  - [Colour Schemes](#colour-schemes)
  - [Recording & Replay](#recording--replay)
  - [Screen Analysis](#screen-analysis)
  - [MockTerminal](#mockterminal)
  - [Delta Tracking](#delta-tracking)
- [PTY Support](#pty-support)
- [Demos](#demos)
- [Documentation](#documentation)
- [License](#license)
- [Acknowledgments](#acknowledgments)

---

## Features

| Area                   | Capabilities                                                                                                                                                                                                                                   |
|------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Parser**             | Full ANSI/ECMA-48 parser built on Paul Flo Williams' state machine                                                                                                                                                                             |
| **Headless Terminal**  | Synchronous `Terminal` — feed input, inspect screen instantly. No screen, no PTY, no goroutines needed                                                                                                                                         |
| **PTY Session**        | Live `Session` backed by a PTY subprocess; event-driven with tcell events                                                                                                                                                                      |
| **Image Rendering**    | Rasterise the terminal to `*image.RGBA` (PNG / GIF) with custom fonts, cursor styles, colour schemes, bold-as-bright, dim blending, italic synthesis, underlines, strikethrough, overline, and procedural box-drawing / braille characters     |
| **Colour Schemes**     | 8 built-in schemes (Dracula, Nord, Solarized Dark/Light, Gruvbox Dark, One Dark, Monokai, Tango Dark) plus custom `SchemeSpec`                                                                                                                 |
| **Session Recording**  | Record input/output/snapshots, export as asciinema v2 JSON, replay with `Player`                                                                                                                                                               |
| **Mock Terminal**      | `MockTerminal` with assertion helpers (`AssertCell`, `AssertLine`, `AssertCursor`, `AssertOutput`, `AssertStyle`, `AssertBold`, `AssertItalic`, `AssertUnderline`, `AssertCombining`, `AssertScrollbackLine`); ideal for unit tests            |
| **AI / Automation**    | `AnalyzeScreen` (line classification: blank, prompt, command, output, error, status-bar), `FindPrompt`, `GetLastOutput`, `GetErrorLine`, `WaitForText`, `WaitForCursor`, `WaitForStable`, `WaitForPattern`, `ExpectEcho`, `SendCommandAndWait` |
| **Delta Tracking**     | `GetDelta` returns incremental cell changes since last call                                                                                                                                                                                    |
| **PTY Backends**       | Unix ptys (Linux, macOS, BSDs, Solaris/illumos) and Windows ConPTY, via `github.com/malivvan/pty`                                                                                                                                              |
| **Mouse Protocols**    | X10, VT200 highlight, button-event, drag, motion, SGR, UTF-8, URXVT                                                                                                                                                                            |
| **Keyboard**           | Comprehensive key encoding with Shift/Ctrl/Alt/Meta modifiers                                                                                                                                                                                  |
| **DEC Modes**          | Alt screen, bracketed paste, focus events (1004), synchronised output (2026), origin mode, cursor keys, auto-wrap, insert/replace, reverse video, margin bells                                                                                 |
| **Extended Sequences** | OSC 8 hyperlinks, OSC 52 clipboard, OSC 4/10/11 colour queries, DECRQSS, DECRQM, XTWINOPS, DECSCA protected cells, selective erase, DECALN                                                                                                     |
| **Box Drawing**        | Procedural rendering of U+2500–259F (box drawing + block elements) and U+2800–28FF (Braille)                                                                                                                                                   |
| **Demos**              | 37 runnable example applications covering every major feature                                                                                                                                                                                  |
| **Custom Surface**     | `Screen` interface — swap in any rendering backend                                                                                                                                                                                             |

---

## Quick Start

### Headless Terminal

```go
package main

import (
    "fmt"
    "github.com/malivvan/terminal"
)

func main() {
    t, _ := terminal.NewTerminal(80, 24)
    defer t.Close()

    t.FeedString("Hello, 世界!\r\n")
    fmt.Println(t.String())           // "Hello, 世界!  " …
    fmt.Println(t.Cell(0, 0).Rune)    // 'H'
}
```

### Image Rendering

```go
t, _ := terminal.NewTerminal(80, 24)
defer t.Close()
t.FeedString("\x1b[31mRed text\x1b[0m normal\r\n")

opts := terminal.DefaultRenderOptions()
img := t.RenderImage(opts)
// img is *image.RGBA — save as PNG, GIF, encode to bytes, etc.
```

### Mock Testing

```go
func TestMyWidget(t *testing.T) {
    m := terminal.NewMockTerminal(t, 10, 3)
    defer m.Close()

    m.Feed("echo hello\r\n")

    m.AssertCell(0, 0, 'e')
    m.AssertLine(0, "echo hello")
    m.AssertContains("hello")
    m.AssertCursor(10, 0)
}
```

---

## Installation

```bash
go get github.com/malivvan/terminal
```

Requires Go 1.25 or later.

---

## Core Concepts

### Terminal (Headless)

`Terminal` is the primary entry point for headless operation. It wraps a `Session` internally, drives the parser synchronously via `Feed`, and exposes the screen buffer directly through `Cell`, `String`, `Lines`, `Cursor`, and `ScrollbackLine`. No real PTY, no screen, no background goroutines — ideal for tests, fuzzing, CI, WASM, and programmatic automation.

```go
t, _ := terminal.NewTerminal(80, 24)
t.FeedString("\x1b[32mgreen\x1b[0m")
col, row, visible := t.Cursor()
cell := t.Cell(5, 0)         // Cell{Rune:'g', Style:…}
```

Key methods: `Feed`, `FeedString`, `HandleEvent`, `Resize`, `Size`, `Cell`, `String`, `Lines`, `Cursor`, `Output`, `GetModes`, `GetDelta`, `RenderImage`, `ScrollbackLen`, `ScrollbackLine`, `WaitForText`, `WaitForCursor`, `WaitForStable`, `WaitForPattern`, `ExpectEcho`, `SendCommandAndWait`, `AnalyzeScreen`, `FindPrompt`, `GetLastOutput`, `GetErrorLine`, `Close`.

### Session (PTY-backed)

`Session` models a live interactive terminal backed by a PTY subprocess. It processes input through the parser in a background goroutine and emits events (`EventRedraw`, `EventTitle`, `EventBell`, `EventClosed`, `EventPanic`, `EventClipboard`, `EventDCS`, `EventDCSData`, `EventColour`, `EventDefaultColour`) that you receive via `Attach`.

```go
s := terminal.New()
s.SetSurface(mySurface)
err := s.Start(exec.Command("bash"))
```

Key methods: `New`, `NewWithSize`, `Start`, `StartWithPty`, `Write`, `Resize`, `HandleEvent`, `Attach`, `Detach`, `Draw`, `Cursor`, `String`, `ScrollBy`, `ScrollOffset`, `IsAltScreen`, `SetSurface`, `HasSurface`, `SetOSC8`, `SetTERM`, `SetLogger`, `SetMaxClipboardLen`, `SetRedrawHandler`, `GetActiveCharset`, `GetMargins`, `GetTabStops`, `GetCursorStyle`, `GetDimensions`, `ClearEventLog`, `EventLog`, `GetClipboard`, `SetClipboard`, `Close`.

### Parser

The parser implements a faithful reproduction of Paul Flo Williams' [VT500-series terminal state machine](https://vt100.net/emu/dec_ansi_parser). You can use it directly to inspect parsed sequences without driving a full terminal:

```go
p := terminal.NewParser(strings.NewReader("\x1b[31m"))
for {
    seq := p.Next()
    if _, ok := seq.(terminal.EOF); ok {
        break
    }
    switch s := seq.(type) {
    case terminal.CSI:
        fmt.Printf("CSI: final=%q params=%v\n", s.Final, s.Parameters)
    }
}
```

Sequence types: `Print`, `C0`, `ESC`, `CSI`, `OSC`, `DCS`, `DCSData`, `DCSEndOfData`, `EOF`.

### Rendering

The package can rasterise the terminal screen into an `*image.RGBA` via `Render` (for `Session`) or `RenderImage` (for `Terminal`). Configure appearance through `Options`:

```go
opts := terminal.DefaultRenderOptions()
opts.Font = myFont
opts.Scheme = myScheme
opts.BoldAsBright = true
opts.DimFactor = 0.5
opts.RenderCursor = true
opts.Scale = 2

img := t.RenderImage(opts)
```

**Options** include: `Font`, `FontBold`, `FontItalic`, `FontBoldItalic`, `CellWidth`, `CellHeight`, `Padding`, `Scale`, `FgDefault`, `BgDefault`, `DimFactor`, `BoldAsBright`, `RenderCursor`, `DrawBoxChars`, `Scheme`, `CursorStyle` (block, underline, bar).

The renderer handles:
- Bold, dim, italic, underline, blink, reverse, strikethrough, overline text attributes
- Bold-as-bright promotion of indexed palette colours (30–37 → 90–97)
- Dim blending via `blendRGBA`
- Italic synthesis via horizontal shear when no italic font is provided
- Procedural box-drawing characters (lines, corners, tees, crosses) and block elements (full block, upper/lower/left/right halves, quarter shades)
- Procedural Braille character rendering (U+2800–28FF)
- Block cursor (inverts cell rectangle), underline cursor, bar cursor
- Wide CJK characters spanning two cells

### Colour Schemes

Override the terminal's palette colours at render time. 8 built-in schemes are available:

| Function | Name |
|---|---|
| `SchemeDracula()` | dracula |
| `SchemeNord()` | nord |
| `SchemeSolarizedDark()` | solarized-dark |
| `SchemeSolarizedLight()` | solarized-light |
| `SchemeGruvboxDark()` | gruvbox-dark |
| `SchemeOneDark()` | one-dark |
| `SchemeMonokai()` | monokai |
| `SchemeTangoDark()` | tango-dark |

```go
// Look up by name (case-insensitive, supports aliases)
scheme, ok := terminal.SchemeByName("dracula")

// Or build a custom scheme
scheme := terminal.SchemeSpec{
    Name:       "my-theme",
    Foreground: "#AABBCC",
    Background: "#112233",
    Cursor:     "#FFFFFF",
    Palette:    []string{"#000000", "#FF0000", /* … */},
}.Build()
```

Apply via `Options.Scheme`:

```go
opts := terminal.DefaultRenderOptions()
opts.Scheme = terminal.SchemeDracula()
```

### Recording & Replay

Record a session and replay it later, or export to asciinema v2 format:

```go
// Record
t, _ := terminal.NewTerminal(80, 24)
rec := terminal.NewSessionRecorder(t)
rec.FeedString("echo hello\r\n")
rec.Close()

// Export asciinema v2
data, _ := rec.ExportAsciinema()
os.WriteFile("session.cast", data, 0644)

// Export raw JSON for replay
raw, _ := rec.ExportRaw()

// Replay
player, _ := terminal.NewSessionPlayer(raw)
for {
    frame, err := player.Next()
    if err == io.EOF {
        break
    }
    fmt.Printf("[%v] %s\n", frame.Timestamp, frame.Output)
}

// Player API: Next, Current, Seek, Pos, Elapsed, Metadata
```

### Screen Analysis

The `Terminal` provides AI-oriented helpers that parse the visible screen into semantically classified lines:

```go
lines := t.AnalyzeScreen()
for _, line := range lines {
    // line.Type is one of: LineTypeBlank, LineTypePrompt, LineTypeCommand,
    //                      LineTypeOutput, LineTypeError, LineTypeStatusBar
    fmt.Println(line.Text)
}

// Convenience wrappers:
promptRow := t.FindPrompt()
lastOutput := t.GetLastOutput()
errText, errRow, found := t.GetErrorLine()
```

Classification heuristics detect shell prompts (`$`, `#`, `>`, `%`), error keywords (`error`, `fail`, `fatal`, `exception`, `panic`, `warning`), red-foreground cells, and reverse-video status bars (bottom row).

### MockTerminal

`MockTerminal` wraps `Terminal` with testify-compatible assertion helpers — ideal for unit-testing code that generates ANSI sequences:

```go
func TestMyWidget(t *testing.T) {
    m := terminal.NewMockTerminal(t, 20, 5)
    defer m.Close()

    m.Feed("\x1b[31mERROR\x1b[0m")

    m.AssertCell(0, 0, 'E')
    m.AssertBold(0, 0)          // not bold
    m.AssertItalic(0, 0)        // not italic
    m.AssertUnderline(0, 0)     // not underlined
    m.AssertLine(0, "ERROR               ")  // width-padded
    m.AssertContains("ERROR")
    m.AssertCursor(5, 0)
    m.AssertStyle(0, 0, tcell.StyleDefault.Foreground(tcell.ColorRed))
    m.AssertForeground(0, 0, tcell.ColorRed)
    m.AssertOutput("\x1b[?2004h") // paste-mode enable
}
```

### Delta Tracking

`GetDelta` returns a `ScreenDelta` containing only the cells that changed since the last call. Useful for incremental rendering or minimal update detection:

```go
delta := t.GetDelta()
for _, d := range delta.Cells {
    fmt.Printf("(%d,%d): %c\n", d.X, d.Y, d.Cell.Rune)
}
// delta.Clear tells you whether the entire screen was cleared
```

---

## PTY Support

Sessions are backed by [`github.com/malivvan/pty`](https://github.com/malivvan/pty):
`Session.Start` starts the command on the pseudo-terminal that module provides.

| Platform | Build Tag | Notes |
|---|---|---|
| Linux / Android | `linux` | PTY via `/dev/ptmx` |
| macOS / iOS | `darwin` | BSD-style `/dev/ptmx` |
| FreeBSD | `freebsd` | BSD PTY |
| OpenBSD | `openbsd` | BSD PTY |
| NetBSD | `netbsd` | BSD PTY |
| DragonFly BSD | `dragonfly` | BSD PTY |
| Solaris / illumos | `solaris` | Solaris PTY |
| Windows | `windows` | ConPTY (`CreatePseudoConsole`) |
| Plan 9, AIX, js/wasm | `plan9`, `aix`, `js` | No pty backend: `Session.Start` returns `pty.ErrUnsupported`. Drive a session with `StartWithPty` and a stream of your own instead — demo 31 bridges a browser that way |

---

## Demos

The [`demos/`](demos/) directory contains 37 runnable example applications, each demonstrating a specific feature:

| Demo | Topic |
|---|---|
| `01_basic` | Minimal headless terminal |
| `02_snapshot` | Periodic screen snapshots |
| `03_animated_gif` | Render to animated GIF |
| `04_keys_and_output` | Keyboard input and output capture |
| `05_hyperlinks_osc8` | OSC 8 hyperlink rendering |
| `06_clipboard_osc52` | OSC 52 clipboard access |
| `07_scrollback` | Scrollback buffer navigation |
| `08_mock_testing` | MockTerminal assertion helpers |
| `09_shell_unix` | Interactive PTY shell |
| `10_focus_events` | DECSET 1004 focus reporting |
| `11_multiplex_headless` | Multiple headless terminals |
| `12_mouse_tracking` | Mouse event tracking |
| `13_multiplex_shell_unix` | Multiple PTY shells |
| `14_selector_multiplex` | Interactive terminal selector |
| `15_mouse_reporter` | Raw mouse protocol output |
| `16_ai_automation` | Screen analysis helpers |
| `17_start_with_pty_bridge` | SSH PTY proxying |
| `18_custom_surface_minimal` | Custom Screen interface |
| `19_redraw_handler_loop` | Event-driven redraw loop |
| `20_sync_output_mode` | DECSET 2026 synchronised output |
| `21_alt_screen_transition` | Alternate screen buffer |
| `22_resize_preserve_content` | Resize preserves cells |
| `23_parser_stream_inspector` | Raw sequence stream inspection |
| `24_sgr_showcase_matrix` | SGR attribute matrix |
| `25_decsed_decsel_protection` | Selective erase & protected cells |
| `26_xtwinops_reports` | XTWINOPS window ops reports |
| `27_mode_probe_dashboard` | Mode probing dashboard |
| `28_session_record_replay_mock` | Recording, replay & mock test |
| `29_render_options_fonts` | Font and colour scheme options |
| `30_windows_conpty_shell` | Windows ConPTY shell |
| `31_js_wasm_pty_bridge` | WASM virtual PTY bridge |
| `32_ssh_pty_proxy` | SSH PTY proxy automation |
| `33_mouse_protocol_compare` | Mouse protocol comparison |
| `34_scrollback_viewport_ui` | Scrollback viewport UI |
| `35_error_recovery_panic_event` | Panic recovery & `EventPanic` |
| `36_multi_pane_scheduler` | Multi-pane layout & scheduler |

```bash
# Run any demo:
cd demos/01_basic && go run .
```

---

## Documentation

Full API documentation is available at [pkg.go.dev/github.com/malivvan/terminal](https://pkg.go.dev/github.com/malivvan/terminal).

---

## License

MIT — see [LICENSE](LICENSE). Same license as the original `tcell-term`.

---

## Acknowledgments

- **Tim Culverhouse** — original author of `tcell-term` (licensed under MIT), which this project forked from at v0.10.0.
- **Paul Flo Williams** — designer of the [DEC/ANSI parser state machine](https://vt100.net/emu/dec_ansi_parser) that this parser implements.
- **The tcell project** ([github.com/gdamore/tcell](https://github.com/gdamore/tcell)) — terminal cell abstraction and event system used throughout this package.
- **Rob Pike** — original `text/template` parser that inspired the concurrency-free state machine design.
- **All contributors and users** of the original `tcell-term` and this fork.

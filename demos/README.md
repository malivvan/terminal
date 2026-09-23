# terminal demos

A collection of runnable examples showcasing the `github.com/malivvan/terminal`
package. Each demo is a standalone `main` package; run any of them with:

```
go run ./demos/<name>
```

| # | Directory              | What it demonstrates                                            |
| - | ---------------------- | --------------------------------------------------------------- |
| 1 | `01_basic`             | Terminal VT: feed bytes, read back the screen and cursor.       |
| 2 | `02_snapshot`          | Render the current screen to a PNG file with `Render`.        |
| 3 | `03_animated_gif`      | Capture a sequence of frames and encode them as animated GIF.   |
| 4 | `04_keys_and_output`   | Send synthesized key events and inspect what the VT emits.      |
| 5 | `05_hyperlinks_osc8`   | Ingest OSC 8 hyperlinks and inspect per-cell URL / URL-id.      |
| 6 | `06_clipboard_osc52`   | Receive OSC 52 clipboard events (with size-limit configured).   |
| 7 | `07_scrollback`        | Cause scrollback to accumulate and page through history.        |
| 8 | `08_mock_testing`      | Use `MockTerminal` to assert VT behaviour like in a unit test.  |
| 9 | `09_shell_unix`        | Run `/bin/sh` inside a real PTY and interact with it (Unix).    |
| 10| `10_focus_events`      | Enable DECSET 1004 focus reporting and observe emitted bytes.   |
| 11 | `11_multiplex_headless` | Composite 4 headless VTs into a tiled PNG (headless multiplex). |
| 12 | `12_mouse_tracking` | DECSET mouse modes + inspect emitted SGR mouse sequences. |
| 13 | `13_multiplex_shell_unix` | Real 2-pane shell multiplexer with Ctrl+B keybindings and mouse routing (Unix). |
| 14 | `14_selector_multiplex` | Multiplex 3 headless VTs concurrently and composite their line output. |
| 15 | `15_mouse_reporter` | Live inspector of the byte sequences the VT emits for tcell mouse events (Unix). |
| 16 | `16_ai_automation`  | AI-agent automation: mode probes, screen analysis, record/replay. |
| 17 | `17_start_with_pty_bridge` | Drive a Session via `StartWithPty` with a custom recorder/proxy `io.ReadWriteCloser`. |
| 18 | `18_custom_surface_minimal` | Implement a 2-method custom `Screen` to prove renderer backend independence. |
| 19 | `19_redraw_handler_loop` | Contrast `SetRedrawHandler` repaint signals with the full `Attach` event stream. |
| 20 | `20_sync_output_mode` | DECSET 2026 synchronized output: buffered grid vs deferred paint + flush. |
| 21 | `21_alt_screen_transition` | Enter/exit alt-screen (?1049h/l) with cursor save/restore. |
| 22 | `22_resize_preserve_content` | Resize and visualize direct-copy content preservation. |
| 23 | `23_parser_stream_inspector` | Use `NewParser` directly and print the parsed `Sequence` stream. |
| 24 | `24_sgr_showcase_matrix` | Full style/colour matrix (16/256/truecolor + attrs + OSC 8) to PNG. |
| 25 | `25_decsed_decsel_protection` | DECSCA-protected cells surviving DECSED/DECSEL selective erase. |
| 26 | `26_xtwinops_reports` | XTWINOPS `14t`/`18t`, DA and cursor-position query responses. |
| 27 | `27_mode_probe_dashboard` | Live `GetModes()` view as private modes are toggled. |
| 28 | `28_session_record_replay_mock` | Record with `Recording`, replay via `Player` (Feed/Output). |
| 29 | `29_render_options_fonts` | Compare `Options` (padding/cell size/cursor/colours) as a contact sheet. |
| 30 | `30_windows_conpty_shell` | Windows shell via the ConPTY path (Windows only). |
| 31 | `31_js_wasm_pty_bridge` | js/wasm demo driven through `globalThis.__pty` (js/wasm only). |
| 32 | `32_ssh_pty_proxy` | Relay a remote shell over SSH into a VT via `StartWithPty` (Unix). |
| 33 | `33_mouse_protocol_compare` | Compare X10/VT200/UTF8/SGR/URXVT encodings for identical events. |
| 34 | `34_scrollback_viewport_ui` | Scripted scrollback paging + search over large output. |
| 35 | `35_error_recovery_panic_event` | Panic-recovery path and `EventPanic` semantics. |
| 36 | `36_multi_pane_scheduler` | Fair round-robin scheduling of headless VTs with independent clocks. |

Demos 30–32 are platform-specific (see the build tag in each `main.go`).

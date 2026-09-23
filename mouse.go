package terminal

import (
	"fmt"
	"io"

	"github.com/gdamore/tcell/v3"
)

func (s *Session) handleMouse(ev *tcell.EventMouse) string {
	// X10 mouse mode — report button presses only (not releases or motion)
	if s.mode&mouseX10 != 0 {
		if ev.Buttons() == tcell.ButtonNone {
			return ""
		}
		var b int
		if ev.Buttons()&tcell.Button1 != 0 {
			b = 0
		} else if ev.Buttons()&tcell.Button2 != 0 {
			b = 1
		} else if ev.Buttons()&tcell.Button3 != 0 {
			b = 2
		} else if ev.Buttons()&tcell.Button4 != 0 {
			b = 3
		} else if ev.Buttons()&tcell.Button5 != 0 {
			b = 4
		} else {
			return ""
		}
		col, row := ev.Position()
		b += 32
		return fmt.Sprintf("\x1b[M%c%c%c", b, 32+col+1, 32+row+1)
	}

	if s.mode&mouseButtons == 0 && s.mode&mouseDrag == 0 && s.mode&mouseMotion == 0 && s.mode&mouseSGR == 0 {
		if s.mode&altScroll != 0 && s.mode&smcup != 0 {
			// Translate wheel motion into arrows up and down
			// 3x rows
			if ev.Buttons()&tcell.WheelUp != 0 {
				io.WriteString(s.pty, info.KeyUp)
				io.WriteString(s.pty, info.KeyUp)
				io.WriteString(s.pty, info.KeyUp)
			}
			if ev.Buttons()&tcell.WheelDown != 0 {
				io.WriteString(s.pty, info.KeyDown)
				io.WriteString(s.pty, info.KeyDown)
				io.WriteString(s.pty, info.KeyDown)
			}
		}
		return ""
	}
	// Filter motion/drag events based on the active tracking mode.
	if s.mouseBtn == ev.Buttons() {
		// Buttons unchanged — this is a motion or drag event.
		if s.mode&mouseButtons != 0 && s.mode&mouseDrag == 0 && s.mode&mouseMotion == 0 {
			// Only button-tracking mode (1000) without drag/motion: suppress all motion.
			return ""
		}
		if s.mode&mouseDrag != 0 && ev.Buttons() == tcell.ButtonNone {
			// Drag mode (1002) with no button held: suppress pure motion.
			return ""
		}
	}

	// Encode the button
	var b int
	if ev.Buttons()&tcell.Button1 != 0 {
		b += 0
	} else if ev.Buttons()&tcell.Button2 != 0 {
		b += 1
	} else if ev.Buttons()&tcell.Button3 != 0 {
		b += 2
	} else if ev.Buttons()&tcell.Button4 != 0 {
		b += 3
	} else if ev.Buttons()&tcell.Button5 != 0 {
		b += 4
	}
	if ev.Buttons() == tcell.ButtonNone {
		b += 3
	}
	if ev.Buttons()&tcell.WheelUp != 0 {
		b += 0 + 64
	}
	if ev.Buttons()&tcell.WheelDown != 0 {
		b += 1 + 64
	}
	if ev.Modifiers()&tcell.ModShift != 0 {
		b += 4
	}
	if ev.Modifiers()&tcell.ModAlt != 0 {
		b += 8
	}
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		b += 16
	}

	if s.mouseBtn == ev.Buttons() {
		// Motion event (button state unchanged, position changed).
		// Add the +32 motion flag when:
		//   - a button is held (drag in any tracking mode), or
		//   - mouseMotion (1003) is active (all motion gets the flag,
		//     even without a button held).
		if ev.Buttons() != tcell.ButtonNone || s.mode&mouseMotion != 0 {
			b += 32
		}
	}

	col, row := ev.Position()

	if s.mode&mouseSGR != 0 {
		switch {
		case ev.Buttons()&tcell.WheelUp != 0:
			return fmt.Sprintf("\x1b[<%d;%d;%dM", b, col+1, row+1)

		case ev.Buttons()&tcell.WheelDown != 0:
			return fmt.Sprintf("\x1b[<%d;%d;%dM", b, col+1, row+1)

		case ev.Buttons() == tcell.ButtonNone && s.mouseBtn != tcell.ButtonNone:
			// Button was in, and now it's not
			var button int
			switch s.mouseBtn {
			case tcell.Button1:
				button = 0
			case tcell.Button2:
				button = 1
			case tcell.Button3:
				button = 2
			case tcell.Button4:
				button = 3
			case tcell.Button5:
				button = 4
			}
			s.mouseBtn = ev.Buttons()
			return fmt.Sprintf("\x1b[<%d;%d;%dm", button, col+1, row+1)

		default:
			s.mouseBtn = ev.Buttons()
			return fmt.Sprintf("\x1b[<%d;%d;%dM", b, col+1, row+1)
		}
	}

	encodedCol := 32 + col + 1
	if encodedCol > 255 {
		encodedCol = 255
	}
	encodedRow := 32 + row + 1
	if encodedRow > 255 {
		encodedRow = 255
	}
	b += 32

	s.mouseBtn = ev.Buttons()
	if s.mode&mouseUTF8 != 0 {
		// UTF-8 mouse encoding: encode coordinates as UTF-8 characters
		return fmt.Sprintf("\x1b[M%c%c%c", rune(b), rune(encodedCol), rune(encodedRow))
	}
	return fmt.Sprintf("\x1b[M%c%c%c", b, encodedCol, encodedRow)
}

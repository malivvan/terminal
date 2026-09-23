package terminal

import (
	"bytes"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/stretchr/testify/assert"
)

func TestHandleMouse(t *testing.T) {
	tests := []struct {
		name     string
		mode     mode
		button   tcell.ButtonMask
		event    *tcell.EventMouse
		expected string
	}{
		{
			name:     "button 1",
			mode:     mouseButtons,
			button:   tcell.ButtonNone,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone),
			expected: "\x1b[M !!",
		},
		{
			name:     "button 1 + shift",
			mode:     mouseButtons,
			button:   tcell.ButtonNone,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModShift),
			expected: "\x1b[M$!!",
		},
		{
			name:     "button 1 drag, in normal mode",
			mode:     mouseButtons,
			button:   tcell.Button1,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone),
			expected: "",
		},
		{
			name:     "button 1 release, in normal mode",
			mode:     mouseButtons,
			button:   tcell.Button1,
			event:    tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone),
			expected: "\x1b[M#!!",
		},
		{
			name:     "button 1 drag, in drag mode",
			mode:     mouseDrag,
			button:   tcell.Button1,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone),
			expected: "\x1b[M@!!",
		},
		{
			name:     "button 1 sgr",
			mode:     mouseSGR,
			button:   tcell.ButtonNone,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone),
			expected: "\x1b[<0;1;1M",
		},
		{
			name:     "no button motion sgr",
			mode:     mouseSGR,
			button:   tcell.ButtonNone,
			event:    tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone),
			expected: "\x1b[<3;1;1M",
		},
		{
			name:     "button 1 drag, in buttons+drag+sgr mode",
			mode:     mouseButtons | mouseDrag | mouseSGR,
			button:   tcell.Button1,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone),
			expected: "\x1b[<32;1;1M",
		},
		{
			name:     "button 1 drag, in buttons+drag mode",
			mode:     mouseButtons | mouseDrag,
			button:   tcell.Button1,
			event:    tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone),
			expected: "\x1b[M@!!",
		},
		{
			name:     "no button motion suppressed in buttons+drag mode",
			mode:     mouseButtons | mouseDrag,
			button:   tcell.ButtonNone,
			event:    tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone),
			expected: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vt := New()
			vt.mouseBtn = test.button
			vt.mode |= test.mode
			actual := vt.handleMouse(test.event)
			assert.Equal(t, test.expected, actual)
		})
	}
}

// Mouse button encoding had Button2 (middle) and Button3 (right) swapped.
// Per the xterm protocol: Button1=0, Button2 (middle)=1, Button3 (right)=2.
func TestHandleMouse_MiddleAndRightButtons(t *testing.T) {
	t.Run("X10 mode: middle button should encode as 1", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseX10
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button2, tcell.ModNone))
		// Button2 (middle) = 1, +32 = 33 = '!'
		assert.Equal(t, "\x1b[M!!!", result)
	})

	t.Run("X10 mode: right button should encode as 2", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseX10
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button3, tcell.ModNone))
		// Button3 (right) = 2, +32 = 34 = '"'
		assert.Equal(t, "\x1b[M\"!!", result)
	})

	t.Run("normal mode: middle button should encode as 1", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button2, tcell.ModNone))
		// Button2 (middle) = 1, +32 = 33 = '!'
		assert.Equal(t, "\x1b[M!!!", result)
	})

	t.Run("normal mode: right button should encode as 2", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button3, tcell.ModNone))
		// Button3 (right) = 2, +32 = 34 = '"'
		assert.Equal(t, "\x1b[M\"!!", result)
	})

	t.Run("SGR mode: middle button release encodes as 1", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseSGR
		vt.mouseBtn = tcell.Button2
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone))
		// SGR release: lowercase 'm', button 1
		assert.Equal(t, "\x1b[<1;1;1m", result)
	})

	t.Run("SGR mode: right button release encodes as 2", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseSGR
		vt.mouseBtn = tcell.Button3
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone))
		// SGR release: lowercase 'm', button 2
		assert.Equal(t, "\x1b[<2;1;1m", result)
	})
}

// In normal (non-SGR) mouse encoding, coordinates are encoded as single-byte
// values with an offset of 32+1. Values exceeding 255 must be clamped to
// prevent wrap-around which would produce incorrect coordinates.
func TestHandleMouse_CoordinateClamping(t *testing.T) {
	// Helper to extract the column and row runes from a normal mouse sequence.
	// Format: \x1b[M<button><col><row>
	parseRunes := func(s string) (colRune, rowRune rune) {
		runes := []rune(s)
		// runes: \x1b [ M <button> <col> <row>
		return runes[4], runes[5]
	}

	t.Run("large column is clamped to 255", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		// col=300 -> 32+300+1 = 333, exceeds 255 without clamping
		result := vt.handleMouse(tcell.NewEventMouse(300, 0, tcell.Button1, tcell.ModNone))
		colR, _ := parseRunes(result)
		assert.Equal(t, rune(255), colR, "column rune should be clamped to 255")
	})

	t.Run("large row is clamped to 255", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		// row=300 -> 32+300+1 = 333, exceeds 255 without clamping
		result := vt.handleMouse(tcell.NewEventMouse(0, 300, tcell.Button1, tcell.ModNone))
		_, rowR := parseRunes(result)
		assert.Equal(t, rune(255), rowR, "row rune should be clamped to 255")
	})

	t.Run("both large coordinates clamped", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		result := vt.handleMouse(tcell.NewEventMouse(500, 500, tcell.Button1, tcell.ModNone))
		colR, rowR := parseRunes(result)
		assert.Equal(t, rune(255), colR, "column rune should be clamped to 255")
		assert.Equal(t, rune(255), rowR, "row rune should be clamped to 255")
	})

	t.Run("normal coordinates are not clamped", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		// col=5, row=10 -> encodedCol=38, encodedRow=43
		result := vt.handleMouse(tcell.NewEventMouse(5, 10, tcell.Button1, tcell.ModNone))
		colR, rowR := parseRunes(result)
		assert.Equal(t, rune(38), colR)
		assert.Equal(t, rune(43), rowR)
	})

	t.Run("UTF-8 mode: large coordinates clamped to 255", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons | mouseUTF8
		result := vt.handleMouse(tcell.NewEventMouse(300, 300, tcell.Button1, tcell.ModNone))
		colR, rowR := parseRunes(result)
		assert.Equal(t, rune(255), colR, "UTF-8 column rune should be clamped to 255")
		assert.Equal(t, rune(255), rowR, "UTF-8 row rune should be clamped to 255")
	})
}

// When multiple buttons are pressed simultaneously, only the lowest-numbered
// button should be reported. The old code used separate if statements which
// would add up all pressed button values.
func TestHandleMouse_MultipleButtonsReportsLowest(t *testing.T) {
	t.Run("Button1+Button2 reports Button1 (value 0)", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button1|tcell.Button2, tcell.ModNone))
		// Button1 = 0, +32 = 32 = ' '
		assert.Equal(t, "\x1b[M !!", result)
	})

	t.Run("Button1+Button3 reports Button1 (value 0)", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button1|tcell.Button3, tcell.ModNone))
		// Button1 = 0, +32 = 32 = ' '
		assert.Equal(t, "\x1b[M !!", result)
	})

	t.Run("Button2+Button3 reports Button2 (value 1)", func(t *testing.T) {
		vt := New()
		vt.mode |= mouseButtons
		result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button2|tcell.Button3, tcell.ModNone))
		// Button2 (middle) = 1, +32 = 33 = '!'
		assert.Equal(t, "\x1b[M!!!", result)
	})
}

func TestHandleMouse_AltScrollWheelUp(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = smcup | altScroll

	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.WheelUp, tcell.ModNone))
	assert.Empty(t, result, "altScroll should return empty string")
	assert.Equal(t, 3, bytes.Count(buf.Bytes(), []byte("\x1bOA")),
		"altScroll wheel up should write 3 up-arrow sequences")
}

func TestHandleMouse_AltScrollWheelDown(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = smcup | altScroll

	var buf bytes.Buffer
	vt.pty = &mockPtyCloser{Buffer: &buf}

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.WheelDown, tcell.ModNone))
	assert.Empty(t, result, "altScroll should return empty string")
	assert.Equal(t, 3, bytes.Count(buf.Bytes(), []byte("\x1bOB")),
		"altScroll wheel down should write 3 down-arrow sequences")
}

func TestHandleMouse_AltScrollNotInAltScreen(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = altScroll

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.WheelUp, tcell.ModNone))
	assert.Empty(t, result)
}

func TestHandleMouse_X10SuppressRelease(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseX10

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.ButtonNone, tcell.ModNone))
	assert.Empty(t, result, "X10 should suppress button release events")
}

func TestHandleMouse_NoModesReturnsEmpty(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = 0

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModNone))
	assert.Empty(t, result, "no mouse modes should return empty string")
}

func TestHandleMouse_X10Button1(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseX10

	result := vt.handleMouse(tcell.NewEventMouse(3, 2, tcell.Button1, tcell.ModNone))
	assert.Contains(t, result, "\x1b[M", "X10 should produce mouse sequence")
}

// TestHandleMouse_SGR_Wheel tests SGR mode with wheel events.
func TestHandleMouse_SGR_Wheel(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseSGR | mouseButtons

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.WheelUp, tcell.ModNone))
	assert.Contains(t, result, "\x1b[<")
	assert.Contains(t, result, "M") // uppercase M for button press

	// Wheel down
	result = vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.WheelDown, tcell.ModNone))
	assert.Contains(t, result, "\x1b[<")
}

// TestHandleMouse_SGR_Release tests SGR mode with button release.
func TestHandleMouse_SGR_Release(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseSGR | mouseButtons
	// First press
	result := vt.handleMouse(tcell.NewEventMouse(3, 2, tcell.Button1, tcell.ModNone))
	assert.Contains(t, result, "M") // press uses M

	// Then release
	vt.mouseBtn = tcell.Button1
	result = vt.handleMouse(tcell.NewEventMouse(3, 2, tcell.ButtonNone, tcell.ModNone))
	assert.Contains(t, result, "m") // release uses lowercase m
}

// TestHandleMouse_UTF8_Encoding tests UTF-8 mouse mode.
func TestHandleMouse_UTF8_Encoding(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseUTF8 | mouseButtons

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModNone))
	assert.NotEmpty(t, result)
}

// TestHandleMouse_Button2_Button3 tests SGR with button 2 and 3.
func TestHandleMouse_Button2_Button3(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseSGR | mouseButtons

	// Button 2
	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button2, tcell.ModNone))
	assert.Contains(t, result, "\x1b[<")

	// Button 3
	result = vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button3, tcell.ModNone))
	assert.Contains(t, result, "\x1b[<")
}

// TestHandleMouse_DragMode tests drag event filtering.
func TestHandleMouse_DragMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseDrag | mouseButtons

	// Drag event with button held
	vt.mouseBtn = tcell.Button1
	result := vt.handleMouse(tcell.NewEventMouse(5, 3, tcell.ButtonNone, tcell.ModNone))
	assert.NotEmpty(t, result)
}

// TestHandleMouse_MotionMode tests motion event filtering.
func TestHandleMouse_MotionMode(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseMotion | mouseButtons

	// Motion event without any button
	result := vt.handleMouse(tcell.NewEventMouse(5, 3, tcell.ButtonNone, tcell.ModNone))
	assert.NotEmpty(t, result)
}

// TestHandleMouse_Modifiers tests modifier keys in mouse events.
func TestHandleMouse_Modifiers(t *testing.T) {
	// Shift+click
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseSGR | mouseButtons
	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModShift))
	assert.Contains(t, result, "\x1b[<")

	// Ctrl+click
	vt2 := New()
	vt2.Resize(10, 5)
	vt2.mode = mouseSGR | mouseButtons
	result = vt2.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModCtrl))
	assert.Contains(t, result, "\x1b[<")

	// Alt+click
	vt3 := New()
	vt3.Resize(10, 5)
	vt3.mode = mouseSGR | mouseButtons
	result = vt3.handleMouse(tcell.NewEventMouse(5, 2, tcell.Button1, tcell.ModAlt))
	assert.Contains(t, result, "\x1b[<")
}

func TestHandleMouse_SGR_ButtonNoneNoBtn(t *testing.T) {
	vt := New()
	vt.Resize(10, 5)
	vt.mode = mouseSGR | mouseButtons
	vt.mouseBtn = 0 // no button held

	result := vt.handleMouse(tcell.NewEventMouse(5, 2, tcell.ButtonNone, tcell.ModNone))
	assert.Empty(t, result, "SGR with no button held and ButtonNone should return empty")
}

// ============================================================
// Mouse drag flag logic: the motion flag (+32) is not added for
// motion events in mouseMotion (1003) mode when no button is held.
// Per xterm protocol, in any-event tracking (1003), all motion
// events should carry the +32 flag, regardless of button state.
// ============================================================

// In mouseMotion (1003) mode, pure motion without a button held
// should have the +32 motion flag in the normal encoding.
func TestMouseMotion_NoButtonMotionHasMotionFlag(t *testing.T) {
	vt := New()
	vt.mode |= mouseMotion
	vt.mouseBtn = tcell.ButtonNone
	result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone))
	// b = 3 (no button) + 32 (motion flag) = 35; +32 (base offset) = 67 = 'C'
	// col = 32+0+1 = 33 = '!', row = 33 = '!'
	assert.Equal(t, "\x1b[MC!!", result,
		"mouseMotion mode: no-button motion should have +32 motion flag")
}

// In mouseMotion + SGR mode, pure motion without a button held
// should have the +32 motion flag in the SGR encoding.
func TestMouseMotion_SGR_NoButtonMotionHasMotionFlag(t *testing.T) {
	vt := New()
	vt.mode |= mouseMotion | mouseSGR
	vt.mouseBtn = tcell.ButtonNone
	result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone))
	// b = 3 (no button) + 32 (motion flag) = 35
	assert.Equal(t, "\x1b[<35;1;1M", result,
		"mouseMotion+SGR mode: no-button motion should have +32 motion flag")
}

// Regression: mouseDrag mode with button held should still get +32.
func TestMouseDrag_ButtonHeldStillGetsMotionFlag(t *testing.T) {
	vt := New()
	vt.mode |= mouseDrag
	vt.mouseBtn = tcell.Button1
	result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone))
	// b = 0 (Button1) + 32 (motion) = 32; +32 (base) = 64 = '@'
	assert.Equal(t, "\x1b[M@!!", result,
		"mouseDrag mode: button-held motion should still have +32 flag")
}

// Regression: mouseMotion mode with button held should get +32.
func TestMouseMotion_ButtonHeldGetsMotionFlag(t *testing.T) {
	vt := New()
	vt.mode |= mouseMotion
	vt.mouseBtn = tcell.Button1
	result := vt.handleMouse(tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone))
	// b = 0 (Button1) + 32 (motion) = 32; +32 (base) = 64 = '@'
	assert.Equal(t, "\x1b[M@!!", result)
}

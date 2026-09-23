package terminal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOSC8(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expected   string
		expectedID string
	}{
		{
			name:       "no semicolon in URI",
			input:      "8;;https://example.com",
			expected:   "https://example.com",
			expectedID: "",
		},
		{
			name:       "no semicolon in URI, with id",
			input:      "8;id=hello;https://example.com",
			expected:   "https://example.com",
			expectedID: "hello",
		},
		{
			name:       "semicolon in URI",
			input:      "8;;https://example.com/semi;colon",
			expected:   "https://example.com/semi;colon",
			expectedID: "",
		},
		{
			name:       "multiple semicolons in URI",
			input:      "8;;https://example.com/s;e;m;i;colon",
			expected:   "https://example.com/s;e;m;i;colon",
			expectedID: "",
		},
		{
			name:       "semicolon in URI, with id",
			input:      "8;id=hello;https://example.com/semi;colon",
			expected:   "https://example.com/semi;colon",
			expectedID: "hello",
		},
		{
			name:       "terminating sequence",
			input:      "8;;",
			expected:   "",
			expectedID: "",
		},
		{
			name:       "terminating sequence with id",
			input:      "8;id=hello;",
			expected:   "",
			expectedID: "hello",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Simulate vt.osc
			selector, val, found := cutString(test.input, ";")
			if !found {
				return
			}
			assert.Equal(t, "8", selector)
			// parse the result
			url, id := osc8(val)
			assert.Equal(t, test.expected, url)
			assert.Equal(t, test.expectedID, id)
		})
	}
}

// TERMINAL-026: OSC 1 (Set Icon Name) should emit an EventTitle.
func TestOSC1_SetIconName(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	var gotTitle string
	// Use a direct call to osc since we can't easily capture events without a pty
	// The osc function processes the payload; we just verify it doesn't crash
	// and recognizes selector "1".
	vt.osc("1;my-icon")
	// The event was posted to the events channel; drain it
	select {
	case ev := <-vt.events:
		if te, ok := ev.(*EventTitle); ok {
			gotTitle = te.Title()
		}
	default:
	}
	assert.Equal(t, "my-icon", gotTitle)
}

// TERMINAL-027: OSC 4 (Change/Query Color Number) should be recognized without panic.
func TestOSC4_ColorNumber(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.osc("4;1;rgb:ff/00/00")
	})
}

// TERMINAL-027: OSC 10 (Set/Query Default Foreground Color) should be recognized.
func TestOSC10_DefaultForeground(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.osc("10;rgb:ff/ff/ff")
	})
}

// TERMINAL-027: OSC 11 (Set/Query Default Background Color) should be recognized.
func TestOSC11_DefaultBackground(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.osc("11;rgb:00/00/00")
	})
}

// TERMINAL-027: OSC 12 (Set/Query Default Cursor Color) should be recognized.
func TestOSC12_CursorColor(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.osc("12;rgb:ff/ff/00")
	})
}

// TERMINAL-027: OSC 52 (Clipboard Access) should emit an EventClipboard.
func TestOSC52_Clipboard(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.osc("52;c;dGVzdA==")

	var gotClipboard string
	select {
	case ev := <-vt.events:
		if ce, ok := ev.(*EventClipboard); ok {
			gotClipboard = ce.Data()
		}
	default:
	}
	assert.Equal(t, "dGVzdA==", gotClipboard, "OSC 52 should emit clipboard event with base64 data")
}

// TERMINAL-027: OSC 104 (Reset Color Number) should be recognized without panic.
func TestOSC104_ResetColor(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	assert.NotPanics(t, func() {
		vt.osc("104;1")
	})
}

// TERMINAL-027: All recognized OSC selectors should not fall through to unhandled.
func TestOSC_AllRecognized(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	// These should all be consumed without panic
	selectors := []string{
		"4;0;rgb:00/00/00",
		"10;?",
		"11;?",
		"12;?",
		"52;c;",
		"104",
	}
	for _, s := range selectors {
		assert.NotPanics(t, func() { vt.osc(s) }, "OSC %q should be handled", s)
	}
}

func TestOSC0_SetTitle(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.osc("0;my-title")

	select {
	case ev := <-vt.events:
		te, ok := ev.(*EventTitle)
		assert.True(t, ok, "expected EventTitle, got %T", ev)
		assert.Equal(t, "my-title", te.Title())
	default:
		t.Error("expected EventTitle event")
	}
}

func TestOSC2_SetTitle(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.osc("2;window-title")

	select {
	case ev := <-vt.events:
		te, ok := ev.(*EventTitle)
		assert.True(t, ok, "expected EventTitle, got %T", ev)
		assert.Equal(t, "window-title", te.Title())
	default:
		t.Error("expected EventTitle event")
	}
}

func TestOSC8_Disabled(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.OSC8 = false

	oldAttrs := vt.cursor.attrs
	vt.osc("8;;https://example.com")

	// Cursor attrs should be unchanged
	assert.Equal(t, oldAttrs, vt.cursor.attrs,
		"cursor attrs should not change when OSC8 is disabled")
}

func TestOSC52_ExceedsLimit(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.MaxClipboardLen = 5

	vt.osc("52;c;dGVzdCBkYXRhIGV4Y2VlZGluZyBsaW1pdA==")

	select {
	case ev := <-vt.events:
		t.Errorf("expected no event, got %T: %+v", ev, ev)
	default:
		// Good — no event posted
	}
}

func TestOSC52_NoSelectionDelimiter(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// Only selector, no second semicolon
	vt.osc("52;c")

	select {
	case ev := <-vt.events:
		t.Errorf("expected no event for incomplete OSC 52, got %T: %+v", ev, ev)
	default:
		// Good — no event posted
	}
}

func TestOSC_NoSelector(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	// No semicolon at all
	assert.NotPanics(t, func() {
		vt.osc("nosemicolon")
	})
}

func TestOSC_EmptyString(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	assert.NotPanics(t, func() {
		vt.osc("")
	})
}

// ============================================================
// Security: OSC 52 clipboard — no size validation
//
// OSC 52 clipboard data is forwarded as-is with no size check.
// A malicious program can inject arbitrarily large clipboard
// data. Consumers that write to the system clipboard inherit
// this risk.
// ============================================================

// OSC 52 with data exceeding the default clipboard limit (1 MB)
// should be dropped without emitting an EventClipboard.
func TestOSC52_LargeDataDropped(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	largeData := strings.Repeat("A", 1<<20+1) // 1 MB + 1 byte
	vt.osc("52;c;" + largeData)

	select {
	case ev := <-vt.events:
		_, isClipboard := ev.(*EventClipboard)
		assert.False(t, isClipboard,
			"should not emit EventClipboard for oversized data")
	default:
		// No event emitted — correct behavior after fix
	}
}

// Regression: normal-sized OSC 52 data should still be forwarded.
func TestOSC52_NormalSizeDataAccepted(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)

	vt.osc("52;c;dGVzdA==")

	var gotData string
	select {
	case ev := <-vt.events:
		if ce, ok := ev.(*EventClipboard); ok {
			gotData = ce.Data()
		}
	default:
	}
	assert.Equal(t, "dGVzdA==", gotData,
		"normal OSC 52 data should be forwarded")
}

// Custom MaxClipboardLen should be respected.
func TestOSC52_CustomMaxClipboardLen(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.MaxClipboardLen = 100

	// Data within limit should produce an event
	smallData := strings.Repeat("A", 100)
	vt.osc("52;c;" + smallData)
	select {
	case ev := <-vt.events:
		_, ok := ev.(*EventClipboard)
		assert.True(t, ok, "data within limit should emit EventClipboard")
	default:
		t.Error("expected EventClipboard for data within limit")
	}

	// Data exceeding limit should not produce an event
	largeData := strings.Repeat("A", 101)
	vt.osc("52;c;" + largeData)
	select {
	case ev := <-vt.events:
		_, isClipboard := ev.(*EventClipboard)
		assert.False(t, isClipboard,
			"data exceeding custom limit should not emit EventClipboard")
	default:
		// No event emitted — correct
	}
}

// Regression: OSC 52 with MaxClipboardLen set to 0 should disable
// the limit and forward data of any size.
func TestOSC52_ZeroMaxClipboardLen_DisablesLimit(t *testing.T) {
	vt := New()
	vt.Resize(4, 1)
	vt.MaxClipboardLen = 0

	// Even a large payload should be forwarded when limit is disabled
	data := strings.Repeat("A", 2048)
	vt.osc("52;c;" + data)

	var gotData string
	select {
	case ev := <-vt.events:
		if ce, ok := ev.(*EventClipboard); ok {
			gotData = ce.Data()
		}
	default:
	}
	assert.Equal(t, data, gotData,
		"with MaxClipboardLen=0, all OSC 52 data should be forwarded")
}

package terminal

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	tests := []struct {
		name     string
		event    *tcell.EventKey
		expected string
	}{
		{
			name: "rune",
			event: tcell.NewEventKey(
				tcell.KeyRune,
				"j",
				tcell.ModNone,
			),
			expected: "j",
		},
		{
			name: "F1",
			event: tcell.NewEventKey(
				tcell.KeyF1,
				"",
				tcell.ModNone,
			),
			expected: "\x1bOP",
		},
		{
			name: "Shift-right",
			event: tcell.NewEventKey(
				tcell.KeyRight,
				"",
				tcell.ModShift,
			),
			expected: "\x1b[1;2C",
		},
		{
			name: "Ctrl-Shift-right",
			event: tcell.NewEventKey(
				tcell.KeyRight,
				"",
				tcell.ModShift|tcell.ModCtrl,
			),
			expected: "\x1b[1;6C",
		},
		{
			name: "Alt-Shift-right",
			event: tcell.NewEventKey(
				tcell.KeyRight,
				"",
				tcell.ModShift|tcell.ModAlt,
			),
			expected: "\x1b[1;4C",
		},
		{
			name: "rune + mod alt",
			event: tcell.NewEventKey(
				tcell.KeyRune,
				"j",
				tcell.ModAlt,
			),
			expected: "\x1Bj",
		},
		{
			name: "rune + mod ctrl",
			event: tcell.NewEventKey(
				tcell.KeyCtrlJ,
				string([]byte{0x0A}),
				tcell.ModCtrl,
			),
			expected: "\n",
		},
		{
			name: "shift + f5",
			event: tcell.NewEventKey(
				tcell.KeyF5,
				"",
				tcell.ModShift,
			),
			expected: "\x1B[15;2~",
		},
		{
			name: "shift + arrow",
			event: tcell.NewEventKey(
				tcell.KeyRight,
				"",
				tcell.ModShift,
			),
			expected: "\x1B[1;2C",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := keyCode(test.event, false)
			assert.Equal(t, test.expected, actual)
		})
	}
}

// TERMINAL-004: Alt+F4 should map to F52, not F53.
func TestKey_AltF4(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyF4, "", tcell.ModAlt)
	actual := keyCode(ev, false)
	assert.Equal(t, info.KeyF52, actual, "Alt+F4 should produce F52 code")
}

// TERMINAL-005: Meta+Shift+Left/Right escape codes should not be swapped.
func TestKey_MetaShfLeftRight(t *testing.T) {
	// Left should end with D, Right with C
	assert.Contains(t, info.KeyMetaShfLeft, "D", "Meta+Shift+Left should end with D")
	assert.Contains(t, info.KeyMetaShfRight, "C", "Meta+Shift+Right should end with C")
}

// TERMINAL-021: Alt+Shift+F5 through F12 should produce key codes.
func TestKey_AltShiftF5(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyF5, "", tcell.ModAlt|tcell.ModShift)
	actual := keyCode(ev, false)
	assert.NotEmpty(t, actual, "Alt+Shift+F5 should produce a key code")
}

// TERMINAL-004 regression: Alt+F5 through F12 should produce correct codes after fix.
func TestKey_AltFKeys(t *testing.T) {
	tests := []struct {
		key      tcell.Key
		expected string
	}{
		{tcell.KeyF1, info.KeyF49},
		{tcell.KeyF2, info.KeyF50},
		{tcell.KeyF3, info.KeyF51},
		{tcell.KeyF4, info.KeyF52},
		{tcell.KeyF5, info.KeyF53},
		{tcell.KeyF6, info.KeyF54},
		{tcell.KeyF7, info.KeyF55},
		{tcell.KeyF8, info.KeyF56},
		{tcell.KeyF9, info.KeyF57},
		{tcell.KeyF10, info.KeyF58},
		{tcell.KeyF11, info.KeyF59},
		{tcell.KeyF12, info.KeyF60},
	}
	for _, tt := range tests {
		ev := tcell.NewEventKey(tt.key, "", tcell.ModAlt)
		actual := keyCode(ev, false)
		assert.Equal(t, tt.expected, actual, "Alt+F%d", int(tt.key-tcell.KeyF1)+1)
	}
}

// TERMINAL-021 regression: Alt+Shift F5-F12 should produce non-empty key codes.
func TestKey_AltShiftFKeys(t *testing.T) {
	for k := tcell.KeyF5; k <= tcell.KeyF12; k++ {
		ev := tcell.NewEventKey(k, "", tcell.ModAlt|tcell.ModShift)
		actual := keyCode(ev, false)
		assert.NotEmpty(t, actual, "Alt+Shift+F%d should produce a key code", int(k-tcell.KeyF1)+1)
	}
}

// Ctrl+letter keys as created by tcell's input parser send
// NewEventKey(KeyRune, "G", ModCtrl) which normalises to
// Key()=KeyCtrlG, Str()="", Mod=ModCtrl. The keyCode(, false) function
// must convert these back to their ASCII control character byte so
// that terminal applications (e.g. micro's Ctrl+G help) receive
// the expected input.
func TestKey_CtrlG_FromInputParser(t *testing.T) {
	// tcell's input parser creates: NewEventKey(KeyRune, "G", ModCtrl)
	ev := tcell.NewEventKey(tcell.KeyRune, "G", tcell.ModCtrl)
	actual := keyCode(ev, false)
	assert.Equal(t, "\x07", actual,
		"Ctrl+G should produce the BEL control character (0x07)")
}

// All Ctrl+letter keys (A-Z) should produce their corresponding
// control character byte (0x01-0x1A).
func TestKey_CtrlLetters_FromInputParser(t *testing.T) {
	for ch := 'A'; ch <= 'Z'; ch++ {
		ev := tcell.NewEventKey(tcell.KeyRune, string(ch), tcell.ModCtrl)
		actual := keyCode(ev, false)
		expected := string(rune(ch - 'A' + 1))
		assert.Equal(t, expected, actual,
			"Ctrl+%c should produce control character 0x%02x", ch, ch-'A'+1)
	}
}

func TestKeyCode_CtrlRune(t *testing.T) {
	// Ctrl + 'a' as a rune event (before normalization)
	ev := tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModCtrl)
	result := keyCode(ev, false)
	assert.Equal(t, "\x01", result)
}

func TestKeyCode_CtrlA_Z(t *testing.T) {
	for k := tcell.KeyCtrlA; k <= tcell.KeyCtrlZ; k++ {
		ev := tcell.NewEventKey(k, "", tcell.ModCtrl)
		result := keyCode(ev, false)
		// KeyCtrlA = 0x0004 in tcell, so KeyCtrlA..KeyCtrlZ map to 0x01..0x1A
		expected := string(rune(k - tcell.KeyCtrlA + 1))
		assert.Equal(t, expected, result, "Ctrl+%c should produce 0x%02X",
			'a'+rune(k-tcell.KeyCtrlA), k-tcell.KeyCtrlA+1)
	}
}

func TestKeyCode_CtrlWithNonCtrlLetter(t *testing.T) {
	// Ctrl + non-letter (like '!') — ev.KeyRune with ModCtrl and Str()="!"
	ev := tcell.NewEventKey(tcell.KeyRune, "!", tcell.ModCtrl)
	result := keyCode(ev, false)
	// Falls into the ModCtrl default: ev.Key()==KeyRune, not in CtrlA-CtrlZ,
	// so uses Str() which is "!"
	assert.Equal(t, "!", result)
}

func TestKeyCode_ModNoneFallback(t *testing.T) {
	// KeyInsert is in keyCodes map but not in the explicit ModNone switch
	// It should be found via the default path
	ev := tcell.NewEventKey(tcell.KeyInsert, "", tcell.ModNone)
	result := keyCode(ev, false)
	assert.Equal(t, "\x1b[2~", result, "KeyInsert should produce \\x1b[2~")
}

func TestKeyCode_ModNoneBackspaceTab(t *testing.T) {
	// Backspace and Tab produce specific codes
	bs := keyCode(tcell.NewEventKey(tcell.KeyBackspace2, "", tcell.ModNone), false)
	assert.NotEmpty(t, bs)

	tab := keyCode(tcell.NewEventKey(tcell.KeyTAB, "", tcell.ModNone), false)
	assert.NotEmpty(t, tab)
}

func TestKeyCode_ModNoneKeyExit(t *testing.T) {
	// KeyExit (which may map to an empty string or a code)
	result := keyCode(tcell.NewEventKey(tcell.KeyExit, "", tcell.ModNone), false)
	// Should not panic; empty is acceptable if not in terminfo
	_ = result
}

func TestKeyCode_MetaPgUp(t *testing.T) {
	// ModMeta + KeyPgUp — tilde-ending key
	ev := tcell.NewEventKey(tcell.KeyPgUp, "", tcell.ModMeta)
	result := keyCode(ev, false)
	assert.Contains(t, result, ";9~", "Meta+PgUp should produce ;9~")
}

func TestKeyCode_MetaHome(t *testing.T) {
	// ModMeta + KeyHome — prefix-based key
	ev := tcell.NewEventKey(tcell.KeyHome, "", tcell.ModMeta)
	result := keyCode(ev, false)
	assert.Contains(t, result, "\x1b[1;9", "Meta+Home should produce \\x1b[1;9...")
}

func TestKeyCode_MetaShiftPgDn(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyPgDn, "", tcell.ModMeta|tcell.ModShift)
	result := keyCode(ev, false)
	assert.Contains(t, result, ";10~", "Meta+Shift+PgDn should produce ;10~")
}

func TestKeyCode_MetaCtrlPgUp(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyPgUp, "", tcell.ModMeta|tcell.ModCtrl)
	result := keyCode(ev, false)
	assert.Contains(t, result, ";13~", "Meta+Ctrl+PgUp should produce ;13~")
}

func TestKeyCode_MetaCtrlShiftHome(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyHome, "", tcell.ModMeta|tcell.ModCtrl|tcell.ModShift)
	result := keyCode(ev, false)
	assert.NotEmpty(t, result)
}

func TestKeyCode_MetaCtrlAltEnd(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModMeta|tcell.ModCtrl|tcell.ModAlt)
	result := keyCode(ev, false)
	assert.NotEmpty(t, result)
}

func TestKeyCode_MetaCtrlAltShiftInsert(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyInsert, "", tcell.ModMeta|tcell.ModCtrl|tcell.ModAlt|tcell.ModShift)
	result := keyCode(ev, false)
	assert.NotEmpty(t, result)
}

func TestKeyCode_MetaUnmodifiableKey(t *testing.T) {
	// ModMeta with KeyBackspace — not in the modifiable keys list
	ev := tcell.NewEventKey(tcell.KeyBackspace, "", tcell.ModMeta)
	result := keyCode(ev, false)
	assert.Empty(t, result, "Meta+Backspace should return empty string")
}

func TestKeyCode_MetaWithRune(t *testing.T) {
	// ModMeta + KeyRune — should emit ESC + rune
	ev := tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModMeta)
	result := keyCode(ev, false)
	assert.Equal(t, "\x1bx", result, "Meta+rune should emit ESC + rune")
}

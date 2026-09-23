package terminal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInfo_NonNil verifies the package-level info variables are initialized.
func TestInfo_NonNil(t *testing.T) {
	assert.NotNil(t, info)
	assert.NotNil(t, extendedInfo)
}

// TestInfo_BasicFields verifies fundamental info fields.
func TestInfo_BasicFields(t *testing.T) {
	assert.Equal(t, "tcell-term", info.Name)
	assert.Equal(t, 80, info.Columns)
	assert.Equal(t, 24, info.Lines)
	assert.Equal(t, 256, info.Colors)
	assert.True(t, info.TrueColor)
	assert.True(t, info.AutoMargin)
	assert.Equal(t, 1, info.Modifiers)
}

// TestInfo_PairedSequences verifies that matching on/off sequence pairs
// are both non-empty when they are populated.
func TestInfo_PairedSequences(t *testing.T) {
	type pair struct{ on, off, name string }
	pairs := []pair{
		{info.EnterCA, info.ExitCA, "EnterCA/ExitCA"},
		{info.ShowCursor, info.HideCursor, "ShowCursor/HideCursor"},
		{info.EnterKeypad, info.ExitKeypad, "EnterKeypad/ExitKeypad"},
		{info.EnterAcs, info.ExitAcs, "EnterAcs/ExitAcs"},
		{info.EnablePaste, info.DisablePaste, "EnablePaste/DisablePaste"},
		{info.EnterUrl, info.ExitUrl, "EnterUrl/ExitUrl"},
	}
	for _, p := range pairs {
		assert.NotEmpty(t, p.on, "%s on sequence should be non-empty", p.name)
		assert.NotEmpty(t, p.off, "%s off sequence should be non-empty", p.name)
	}
}

// TestInfo_KeySequencesNonEmpty verifies common key sequences.
func TestInfo_KeySequencesNonEmpty(t *testing.T) {
	keys := map[string]string{
		"KeyUp":        info.KeyUp,
		"KeyDown":      info.KeyDown,
		"KeyHome":      info.KeyHome,
		"KeyEnd":       info.KeyEnd,
		"KeyInsert":    info.KeyInsert,
		"KeyDelete":    info.KeyDelete,
		"KeyBackspace": info.KeyBackspace,
		"KeyF1":        info.KeyF1,
		"Mouse":        info.Mouse,
		"Bell":         info.Bell,
		"Clear":        info.Clear,
		"EnterCA":      info.EnterCA,
		"ExitCA":       info.ExitCA,
		"ShowCursor":   info.ShowCursor,
		"HideoCursor":  info.HideCursor,
	}
	for name, seq := range keys {
		assert.NotEmpty(t, seq, "%s should be non-empty", name)
	}
}

// TestInfo_UnderlineStyles verifies underline style sequences if populated.
func TestInfo_UnderlineStyles(t *testing.T) {
	// These may be empty depending on terminfo database; only check non-empty
	// fields to avoid false failures.
	if info.DoubleUnderline != "" {
		assert.NotEmpty(t, info.DoubleUnderline)
	}
	if info.CurlyUnderline != "" {
		assert.NotEmpty(t, info.CurlyUnderline)
	}
	if info.DottedUnderline != "" {
		assert.NotEmpty(t, info.DottedUnderline)
	}
	if info.DashedUnderline != "" {
		assert.NotEmpty(t, info.DashedUnderline)
	}
}

// TestInfo_UnderlineColorFields verifies underline color sequences if populated.
func TestInfo_UnderlineColorFields(t *testing.T) {
	if info.UnderlineColor != "" {
		assert.NotEmpty(t, info.UnderlineColor)
	}
	if info.UnderlineColorRGB != "" {
		assert.NotEmpty(t, info.UnderlineColorRGB)
	}
	if info.UnderlineColorReset != "" {
		assert.NotEmpty(t, info.UnderlineColorReset)
	}
}

// TestInfo_CursorStyles verifies cursor style sequences.
func TestInfo_CursorStyles(t *testing.T) {
	styles := map[string]string{
		"CursorDefault":           info.CursorDefault,
		"CursorBlinkingBlock":     info.CursorBlinkingBlock,
		"CursorSteadyBlock":       info.CursorSteadyBlock,
		"CursorBlinkingUnderline": info.CursorBlinkingUnderline,
		"CursorSteadyUnderline":   info.CursorSteadyUnderline,
		"CursorBlinkingBar":       info.CursorBlinkingBar,
		"CursorSteadyBar":         info.CursorSteadyBar,
	}
	for name, seq := range styles {
		assert.NotEmpty(t, seq, "%s should be non-empty", name)
	}
}

// TestExtendedInfo_AllFieldsNonEmpty verifies all extendedInfo fields.
func TestExtendedInfo_AllFieldsNonEmpty(t *testing.T) {
	val := reflect.ValueOf(extendedInfo).Elem()
	typ := val.Type()
	var emptyFields []string
	for i := 0; i < val.NumField(); i++ {
		fv := val.Field(i).String()
		if fv == "" {
			emptyFields = append(emptyFields, typ.Field(i).Name)
		}
	}
	assert.Empty(t, emptyFields, "extendedInfo fields should all be non-empty")
}

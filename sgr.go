package terminal

import (
	"github.com/gdamore/tcell/v3"
	tcellcolor "github.com/gdamore/tcell/v3/color"
)

func (s *Session) sgr(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i += 1 {
		switch params[i] {
		case 0:
			s.cursor.attrs = tcell.StyleDefault
			s.cursor.overline = false
		case 1:
			s.cursor.attrs = s.cursor.attrs.Bold(true)
		case 2:
			s.cursor.attrs = s.cursor.attrs.Dim(true)
		case 3:
			s.cursor.attrs = s.cursor.attrs.Italic(true)
		case 4:
			s.cursor.attrs = s.cursor.attrs.Underline(true)
		case 5:
			s.cursor.attrs = s.cursor.attrs.Blink(true)
		case 6:
			s.cursor.attrs = s.cursor.attrs.Blink(true)
		case 7:
			s.cursor.attrs = s.cursor.attrs.Reverse(true)
		case 8:
			// Invisible, not supported
		case 9:
			s.cursor.attrs = s.cursor.attrs.StrikeThrough(true)
		case 21:
			// Double underlined, not supported
		case 22:
			s.cursor.attrs = s.cursor.attrs.Bold(false).Dim(false)
		case 23:
			s.cursor.attrs = s.cursor.attrs.Italic(false)
		case 24:
			s.cursor.attrs = s.cursor.attrs.Underline(false)
		case 25:
			s.cursor.attrs = s.cursor.attrs.Blink(false)
		case 26:
			s.cursor.attrs = s.cursor.attrs.Blink(false)
		case 27:
			s.cursor.attrs = s.cursor.attrs.Reverse(false)
		case 28:
			// Not invisible, not supported
		case 29:
			s.cursor.attrs = s.cursor.attrs.StrikeThrough(false)
		case 30, 31, 32, 33, 34, 35, 36, 37:
			color := tcellcolor.PaletteColor(params[i] - 30)
			s.cursor.attrs = s.cursor.attrs.Foreground(color)
		case 38:
			var color tcell.Color
			if len(params[i:]) < 3 {
				// Malformed without at least 3 params. Don't
				// set any more attributes at this point
				return
			}
			switch params[i+1] {
			case 2:
				if len(params[i:]) < 5 {
					// Malformed without at least5 params.
					// Don't set any more attributes at this
					// point
					return
				}
				color = tcellcolor.NewRGBColor(
					int32(params[i+2]),
					int32(params[i+3]),
					int32(params[i+4]),
				)
				i += 4
			case 5:
				color = tcellcolor.PaletteColor(params[i+2])
				i += 2
			default:
				// Malformed
				return
			}
			s.cursor.attrs = s.cursor.attrs.Foreground(color)
		case 39:
			s.cursor.attrs = s.cursor.attrs.Foreground(tcellcolor.Default)
		case 40, 41, 42, 43, 44, 45, 46, 47:
			color := tcellcolor.PaletteColor(params[i] - 40)
			s.cursor.attrs = s.cursor.attrs.Background(color)
		case 48:
			var color tcell.Color
			if len(params[i:]) < 3 {
				// Malformed without at least 3 params. Don't
				// set any more attributes at this point
				return
			}
			switch params[i+1] {
			case 2:
				if len(params[i:]) < 5 {
					// Malformed without at least5 params.
					// Don't set any more attributes at this
					// point
					return
				}
				color = tcellcolor.NewRGBColor(
					int32(params[i+2]),
					int32(params[i+3]),
					int32(params[i+4]),
				)
				i += 4
			case 5:
				color = tcellcolor.PaletteColor(params[i+2])
				i += 2
			default:
				// Malformed
				return
			}
			s.cursor.attrs = s.cursor.attrs.Background(color)
		case 49:
			s.cursor.attrs = s.cursor.attrs.Background(tcellcolor.Default)
		case 53:
			s.cursor.overline = true
		case 55:
			s.cursor.overline = false
		case 90, 91, 92, 93, 94, 95, 96, 97:
			color := tcellcolor.PaletteColor(params[i] - 90 + 8)
			s.cursor.attrs = s.cursor.attrs.Foreground(color)
		case 100, 101, 102, 103, 104, 105, 106, 107:
			color := tcellcolor.PaletteColor(params[i] - 100 + 8)
			s.cursor.attrs = s.cursor.attrs.Background(color)
		}
	}
}

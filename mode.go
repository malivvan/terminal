package terminal

import "github.com/gdamore/tcell/v3"

type mode int

const (
	// ANSI-Standardized modes
	//
	// Keyboard Action mode
	kam mode = 1 << iota
	// Insert/Replace mode
	irm
	// Send/Receive mode
	srm
	// Line feed/new line mode
	lnm

	// ANSI-Compatible DEC Private Modes
	//
	// Cursor Key mode
	decckm
	// ANSI/VT52 mode
	decanm
	// Column mode
	deccolm
	// Scroll mode
	decsclm
	// Origin mode
	decom
	// Autowrap mode
	decawm
	// Autorepeat mode
	decarm
	// Printer form feed mode
	decpff
	// Printer extent mode
	decpex
	// Text Cursor Enable mode
	dectcem
	// National replacement character sets
	decnrcm

	// xterm
	//
	// Use alternate screen
	smcup
	// Bracketed paste
	paste
	// vt220 mouse
	mouseButtons
	// vt220 + drag
	mouseDrag
	// vt220 + all motion
	mouseMotion
	// Mouse SGR mode
	mouseSGR
	// Alternate scroll
	altScroll
	// Focus event reporting (DECSET 1004)
	focusEvents
	// Synchronized output (DECSET 2026)
	syncOutput
	// Reverse video (DECSCNM, DECSET 5)
	decscnm
	// X10 mouse compatibility (DECSET 9)
	mouseX10
	// UTF-8 mouse encoding (DECSET 1005)
	mouseUTF8
	// VT200 highlight mouse (DECSET 1001)
	mouseHighlight
	// urxvt extended mouse (DECSET 1015)
	mouseURXVT
)

func (s *Session) enterAltScreen(saveCursor bool) {
	if saveCursor {
		s.decsc()
	}
	s.activeScreen = s.altScreen
	s.mode |= smcup
	// Enable altScroll in the alt screen. This is only used
	// if the application doesn't enable mouse.
	s.mode |= altScroll
}

func (s *Session) exitAltScreen(restoreCursor bool) {
	if s.mode&smcup != 0 {
		// Only clear if we were in the alternate screen.
		s.ed(2)
	}
	s.activeScreen = s.primaryScreen
	s.mode &^= smcup
	s.mode &^= altScroll
	if restoreCursor {
		s.decrc()
	}
}

func (s *Session) sm(params []int) {
	for _, param := range params {
		switch param {
		case 2:
			s.mode |= kam
		case 4:
			s.mode |= irm
		case 12:
			s.mode |= srm
		case 20:
			s.mode |= lnm
		}
	}
}

func (s *Session) rm(params []int) {
	for _, param := range params {
		switch param {
		case 2:
			s.mode &^= kam
		case 4:
			s.mode &^= irm
		case 12:
			s.mode &^= srm
		case 20:
			s.mode &^= lnm
		}
	}
}

func (s *Session) decset(params []int) {
	prevMouse := s.mouseModeFlags()
	for _, param := range params {
		switch param {
		case 1:
			s.mode |= decckm
		case 2:
			s.mode |= decanm
		case 3:
			s.mode |= deccolm
			if !s.inDeccolmResize && s.height() > 0 {
				s.inDeccolmResize = true
				s.resizeUnlocked(132, s.height())
				s.inDeccolmResize = false
			}
		case 4:
			s.mode |= decsclm
		case 5:
			s.mode |= decscnm
		case 6:
			s.mode |= decom
			s.homeCursor()
		case 7:
			s.mode |= decawm
			s.lastCol = false
		case 8:
			s.mode |= decarm
		case 9:
			s.mode |= mouseX10
		case 25:
			s.mode |= dectcem
		case 47, 1047:
			s.enterAltScreen(false)
		case 1048:
			s.decsc()
		case 1000:
			s.mode |= mouseButtons
		case 1001:
			s.mode |= mouseHighlight
		case 1002:
			s.mode |= mouseDrag
		case 1003:
			s.mode |= mouseMotion
		case 1004:
			s.mode |= focusEvents
		case 1005:
			s.mode |= mouseUTF8
		case 1006:
			s.mode |= mouseSGR
		case 1007:
			s.mode |= altScroll
		case 1015:
			s.mode |= mouseURXVT
		case 1049:
			s.enterAltScreen(true)
		case 2004:
			s.mode |= paste
		case 2026:
			s.mode |= syncOutput
		}
	}
	curMouse := s.mouseModeFlags()
	if !mouseFlagsEqual(prevMouse, curMouse) {
		s.postEvent(&EventMouseMode{
			EventTerminal: newEventTerminal(s),
			modes:         curMouse,
		})
	}
}

func (s *Session) decrst(params []int) {
	prevMouse := s.mouseModeFlags()
	for _, param := range params {
		switch param {
		case 1:
			s.mode &^= decckm
		case 2:
			s.mode &^= decanm
		case 3:
			s.mode &^= deccolm
			if !s.inDeccolmResize && s.height() > 0 {
				s.inDeccolmResize = true
				s.resizeUnlocked(80, s.height())
				s.inDeccolmResize = false
			}
		case 4:
			s.mode &^= decsclm
		case 5:
			s.mode &^= decscnm
		case 6:
			s.mode &^= decom
			s.homeCursor()
		case 7:
			s.mode &^= decawm
			s.lastCol = false
		case 8:
			s.mode &^= decarm
		case 9:
			s.mode &^= mouseX10
		case 25:
			s.mode &^= dectcem
		case 47, 1047:
			s.exitAltScreen(false)
		case 1048:
			s.decrc()
		case 1000:
			s.mode &^= mouseButtons
		case 1001:
			s.mode &^= mouseHighlight
		case 1002:
			s.mode &^= mouseDrag
		case 1003:
			s.mode &^= mouseMotion
		case 1004:
			s.mode &^= focusEvents
		case 1005:
			s.mode &^= mouseUTF8
		case 1006:
			s.mode &^= mouseSGR
		case 1007:
			s.mode &^= altScroll
		case 1015:
			s.mode &^= mouseURXVT
		case 1049:
			s.exitAltScreen(true)
		case 2004:
			s.mode &^= paste
		case 2026:
			if s.mode&syncOutput != 0 {
				s.mode &^= syncOutput
				// Reset dirty so the post-update check in update() fires
				// a new EventRedraw now that output is no longer deferred.
				s.dirty = false
			}
		}
	}
	curMouse := s.mouseModeFlags()
	if !mouseFlagsEqual(prevMouse, curMouse) {
		s.postEvent(&EventMouseMode{
			EventTerminal: newEventTerminal(s),
			modes:         curMouse,
		})
	}
}

// mouseModeFlags returns the active mouse event flags derived from the
// current mode bitset.
func (s *Session) mouseModeFlags() []tcell.MouseFlags {
	var flags []tcell.MouseFlags
	if s.mode&mouseButtons != 0 {
		flags = append(flags, tcell.MouseButtonEvents)
	}
	if s.mode&mouseDrag != 0 {
		flags = append(flags, tcell.MouseDragEvents)
	}
	if s.mode&mouseMotion != 0 {
		flags = append(flags, tcell.MouseMotionEvents)
	}
	return flags
}

// mouseFlagsEqual compares two []tcell.MouseFlags slices for equality.
func mouseFlagsEqual(a, b []tcell.MouseFlags) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TrackingMode identifies which mouse protocol is currently active.
type TrackingMode int

const (
	// MouseModeNone indicates no mouse tracking mode is active.
	MouseModeNone TrackingMode = iota
	// MouseModeX10 indicates X10 mouse compatibility (DECSET 9).
	MouseModeX10
	// MouseModeVT200 indicates basic button tracking (DECSET 1000).
	MouseModeVT200
	// MouseModeVT200Drag indicates drag tracking (DECSET 1002).
	MouseModeVT200Drag
	// MouseModeVT200Motion indicates all-motion tracking (DECSET 1003).
	MouseModeVT200Motion
	// MouseModeSGR indicates SGR extended mouse encoding (DECSET 1006).
	MouseModeSGR
	// MouseModeUTF8 indicates UTF-8 mouse encoding (DECSET 1005).
	MouseModeUTF8
	// MouseModeURXVT indicates urxvt extended mouse encoding (DECSET 1015).
	MouseModeURXVT
)

// String returns a human-readable name for the mouse tracking mode.
func (m TrackingMode) String() string {
	switch m {
	case MouseModeNone:
		return "none"
	case MouseModeX10:
		return "x10"
	case MouseModeVT200:
		return "vt200"
	case MouseModeVT200Drag:
		return "vt200-drag"
	case MouseModeVT200Motion:
		return "vt200-motion"
	case MouseModeSGR:
		return "sgr"
	case MouseModeUTF8:
		return "utf8"
	case MouseModeURXVT:
		return "urxvt"
	default:
		return "unknown"
	}
}

// Modes is a public snapshot of the terminal's mode state,
// suitable for type-safe introspection by automation agents.
type Modes struct {
	// MouseMode is the currently active mouse tracking protocol.
	MouseMode TrackingMode
	// BracketedPaste indicates whether bracketed paste mode is active (DECSET 2004).
	BracketedPaste bool
	// Autowrap indicates whether line wrapping is active (DECAWM, DECSET 7).
	Autowrap bool
	// ReverseVideo indicates whether reverse video mode is active (DECSCNM, DECSET 5).
	ReverseVideo bool
	// FocusEvents indicates whether focus event reporting is active (DECSET 1004).
	FocusEvents bool
	// SynchronizedOutput indicates whether synchronized output is active (DECSET 2026).
	SynchronizedOutput bool
	// OriginMode indicates whether origin mode is active (DECOM, DECSET 6).
	OriginMode bool
	// CursorKeyMode indicates whether application cursor key mode is active (DECCKM, DECSET 1).
	CursorKeyMode bool
	// InsertMode indicates whether insert/replace mode is active (IRM, SM 4).
	InsertMode bool
	// CursorVisible indicates whether the text cursor is visible (DECTCEM, DECSET 25).
	CursorVisible bool
	// AltScreen indicates whether the alternate screen buffer is active.
	AltScreen bool
}

// mouseTrackingMode resolves the active mouse tracking protocol from the
// internal mode bitset.
func (s *Session) mouseTrackingMode() TrackingMode {
	if s.mode&mouseURXVT != 0 {
		return MouseModeURXVT
	}
	if s.mode&mouseSGR != 0 {
		return MouseModeSGR
	}
	if s.mode&mouseUTF8 != 0 {
		return MouseModeUTF8
	}
	if s.mode&mouseMotion != 0 {
		return MouseModeVT200Motion
	}
	if s.mode&mouseDrag != 0 {
		return MouseModeVT200Drag
	}
	if s.mode&mouseButtons != 0 {
		return MouseModeVT200
	}
	if s.mode&mouseX10 != 0 {
		return MouseModeX10
	}
	return MouseModeNone
}

// GetModes returns a snapshot of the current terminal mode state.
// Safe to call from any goroutine.
func (s *Session) GetModes() Modes {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Modes{
		MouseMode:          s.mouseTrackingMode(),
		BracketedPaste:     s.mode&paste != 0,
		Autowrap:           s.mode&decawm != 0,
		ReverseVideo:       s.mode&decscnm != 0,
		FocusEvents:        s.mode&focusEvents != 0,
		SynchronizedOutput: s.mode&syncOutput != 0,
		OriginMode:         s.mode&decom != 0,
		CursorKeyMode:      s.mode&decckm != 0,
		InsertMode:         s.mode&irm != 0,
		CursorVisible:      s.mode&dectcem != 0,
		AltScreen:          s.mode&smcup != 0,
	}
}

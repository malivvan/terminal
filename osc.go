package terminal

import (
	"strconv"
	"strings"
)

func (s *Session) osc(data string) {
	selector, val, found := cutString(data, ";")
	if !found {
		return
	}
	switch selector {
	case "0", "1", "2":
		ev := &EventTitle{
			EventTerminal: newEventTerminal(s),
			title:         val,
		}
		s.postEvent(ev)
	case "4":
		// OSC 4 — Change/Query Color Number
		parts := strings.SplitN(val, ";", 2)
		if len(parts) == 2 {
			idx, err := strconv.Atoi(parts[0])
			if err == nil {
				s.postEvent(&EventColour{
					EventTerminal: newEventTerminal(s),
					Index:         idx,
					Value:         parts[1],
				})
			}
		}
	case "8":
		if s.OSC8 {
			url, id := osc8(val)
			s.cursor.attrs = s.cursor.attrs.Url(url)
			s.cursor.attrs = s.cursor.attrs.UrlId(id)
		}
	case "10":
		// OSC 10 — Set/Query Default Foreground Color
		s.postEvent(&EventDefaultColour{
			EventTerminal: newEventTerminal(s),
			IsForeground:  true,
			Value:         val,
		})
	case "11":
		// OSC 11 — Set/Query Default Background Color
		s.postEvent(&EventDefaultColour{
			EventTerminal: newEventTerminal(s),
			IsForeground:  false,
			Value:         val,
		})
	case "12":
		// OSC 12 — Set/Query Default Cursor Color
		// Silently consumed; query responses not implemented.
	case "52":
		// OSC 52 — Clipboard Access
		// Format: OSC 52 ; <selection> ; <base64-data> ST
		sel, b64data, ok := cutString(val, ";")
		if !ok {
			return
		}
		if s.MaxClipboardLen > 0 && len(b64data) > s.MaxClipboardLen {
			s.Logger.Printf("OSC 52: clipboard data size %d exceeds limit %d, dropping", len(b64data), s.MaxClipboardLen)
			return
		}
		s.postEvent(&EventClipboard{
			EventTerminal: newEventTerminal(s),
			selection:     sel,
			data:          b64data,
		})
		// Decode and store for GetClipboard queries.
		if s.clipboard == nil {
			s.clipboard = make(map[string]string)
		}
		s.clipboard[sel] = b64data
	case "104":
		// OSC 104 — Reset Color Number
		// Silently consumed.
	}
}

// parses an osc8 payload into the URL and optional ID
func osc8(val string) (string, string) {
	// OSC 8 ; params ; url ST
	// params: key1=value1:key2=value2
	var id string
	params, url, found := cutString(val, ";")
	if !found {
		return "", ""
	}
	for _, param := range strings.Split(params, ":") {
		key, val, found := cutString(param, "=")
		if !found {
			continue
		}
		switch key {
		case "id":
			id = val
		}
	}
	return url, id
}

// Copied from stdlib to here for go 1.16 compat
func cutString(s string, sep string) (before string, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

package model

import "strings"

// Button is one control the device will let software reassign.
//
// "Remapped" here means what it says on the hardware: the button starts
// behaving as the target button, at the device, before anything the desktop
// sees. Remapping Forward to Middle means Forward middle-clicks.
type Button struct {
	// Slug is the stable name used in setting keys: back, forward, gesture.
	Slug  string `json:"slug"`
	Label string `json:"label"`
	// CID is the device's own control id.
	CID int `json:"cid"`
	// MappedTo is the slug of the button this one currently acts as. Equal to
	// Slug when the button does its own job.
	MappedTo string `json:"mappedTo"`
	Default  bool   `json:"default"`
	// Targets are what this button may be reassigned to, worked out from the
	// group mask the device reports rather than assumed.
	Targets []ButtonTarget `json:"targets"`
}

type ButtonTarget struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	CID   int    `json:"cid"`
}

// buttonNames are the control ids worth recognising on sight. These are
// Logitech-wide, not per-model, so the table lives here rather than in the
// catalog. An id missing here still works — it is shown as its hex.
var buttonNames = map[uint16]struct{ Slug, Label string }{
	0x0050: {"left", "Left Click"},
	0x0051: {"right", "Right Click"},
	0x0052: {"middle", "Middle Click"},
	0x0053: {"back", "Back"},
	0x0056: {"forward", "Forward"},
	0x005B: {"thumb", "Thumb Button"},
	0x00C3: {"gesture", "Gesture Button"},
	0x00C4: {"wheel-mode", "Wheel Mode"},
	0x00D7: {"virtual-gesture", "Virtual Gesture"},
}

// ButtonName returns the slug and label for a control id. An unrecognised id
// gets a hex slug, which still round-trips through a setting key.
func ButtonName(cid uint16) (slug, label string) {
	if known, ok := buttonNames[cid]; ok {
		return known.Slug, known.Label
	}
	hex := hex16(cid)
	return hex, "Button " + hex
}

// ButtonCID resolves a slug back to a control id, accepting the hex form that
// ButtonName falls back to.
func ButtonCID(slug string) (uint16, bool) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	for cid, known := range buttonNames {
		if known.Slug == slug {
			return cid, true
		}
	}
	if cid, ok := parseHex16(slug); ok {
		return cid, true
	}
	return 0, false
}

func hex16(value uint16) string {
	const digits = "0123456789abcdef"
	return "0x" + string([]byte{
		digits[(value>>12)&0xF], digits[(value>>8)&0xF],
		digits[(value>>4)&0xF], digits[value&0xF],
	})
}

func parseHex16(text string) (uint16, bool) {
	text = strings.TrimPrefix(strings.ToLower(text), "0x")
	if text == "" || len(text) > 4 {
		return 0, false
	}
	var value uint16
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c >= '0' && c <= '9':
			value = value<<4 | uint16(c-'0')
		case c >= 'a' && c <= 'f':
			value = value<<4 | uint16(c-'a'+10)
		default:
			return 0, false
		}
	}
	return value, true
}

// settingButtonPrefix namespaces the per-button setting keys. Unlike every
// other capability, the set of keys is not fixed: it is whatever controls the
// device says it has, so the key carries the button's slug.
const settingButtonPrefix = "button-"

// ButtonSettingKey is the setting that reassigns one button.
func ButtonSettingKey(slug string) SettingKey {
	return SettingKey(settingButtonPrefix + slug)
}

// ButtonSlugOf returns the button a setting key is about.
func ButtonSlugOf(key SettingKey) (string, bool) {
	slug := strings.TrimPrefix(string(key), settingButtonPrefix)
	if slug == string(key) || slug == "" {
		return "", false
	}
	return slug, true
}

// ButtonDefault is the word for "put this button back to its own job".
//
// It is resolved to the button's own control id while the setting is parsed,
// because the device ignores a remap of zero outright: the reset is to map a
// control to itself. Resolving it early also keeps the requested and written
// values identical, so a reset does not get reported as a snapped value.
const ButtonDefault = "default"

// Button returns the named reassignable button from a read, or nil when the
// device has no such button.
func (s *DeviceState) Button(slug string) *Button {
	for i := range s.Buttons {
		if s.Buttons[i].Slug == slug {
			return &s.Buttons[i]
		}
	}
	return nil
}

// ButtonSlugs lists what this device will let software reassign, for an error
// message that says what the caller could have asked for instead.
func (s *DeviceState) ButtonSlugs() []string {
	slugs := make([]string, 0, len(s.Buttons))
	for _, button := range s.Buttons {
		slugs = append(slugs, button.Slug)
	}
	return slugs
}

// Accepts reports whether this button may be reassigned to the given control
// id, according to the group mask the device itself reported.
func (b *Button) Accepts(cid uint16) bool {
	for _, target := range b.Targets {
		if target.CID == int(cid) {
			return true
		}
	}
	return false
}

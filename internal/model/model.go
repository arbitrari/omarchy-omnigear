// Package model is the vocabulary every device in the catalog is described
// with.
//
// Nothing here knows how to talk to hardware. A Driver turns a Device into a
// DeviceState; the QML side only ever sees the JSON these types marshal to.
package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// Category is what kind of peripheral it is. One per "###" section of the
// README.
type Category string

const (
	Mouse    Category = "mouse"
	Keyboard Category = "keyboard"
	Headset  Category = "headset"
)

// Brand is who makes it. One per "####" section of the README.
type Brand string

const (
	Logitech    Brand = "logitech"
	Razer       Brand = "razer"
	Corsair     Brand = "corsair"
	Glorious    Brand = "glorious"
	FinalMouse  Brand = "finalmouse"
	Keychron    Brand = "keychron"
	SteelSeries Brand = "steelseries"
	HyperX      Brand = "hyperx"
	Sony        Brand = "sony"
	Apple       Brand = "apple"
	Google      Brand = "google"
	Nothing     Brand = "nothing"
	OnePlus     Brand = "oneplus"
)

// brandLabels are the names as the README prints them.
var brandLabels = map[Brand]string{
	Logitech:    "Logitech",
	Razer:       "Razer",
	Corsair:     "Corsair",
	Glorious:    "Glorious",
	FinalMouse:  "FinalMouse",
	Keychron:    "Keychron",
	SteelSeries: "Steelseries",
	HyperX:      "HyperX",
	Sony:        "Sony",
	Apple:       "Apple",
	Google:      "Google",
	Nothing:     "Nothing",
	OnePlus:     "OnePlus",
}

func (b Brand) Label() string {
	if label, ok := brandLabels[b]; ok {
		return label
	}
	return string(b)
}

// Support is how far along support for a model is. Mirrors the README's
// traffic lights.
type Support string

const (
	// SupportFull — every capability the model has is implemented.
	SupportFull Support = "full"
	// SupportPartial — the model is driven, but some capabilities are missing.
	SupportPartial Support = "partial"
	// SupportPlanned — catalogued, not implemented.
	SupportPlanned Support = "planned"
)

func (s Support) Emoji() string {
	switch s {
	case SupportFull:
		return "🟩"
	case SupportPartial:
		return "🟨"
	default:
		return "🟥"
	}
}

// Capability is a thing a device can report or be told to do.
//
// Capabilities are the contract between a driver and the UI: a driver declares
// which of these a model has, and the panel renders the matching control. A
// new device never needs new QML — only a new catalog entry.
type Capability string

const (
	CapBattery     Capability = "battery"
	CapDPI         Capability = "dpi"
	CapPollingRate Capability = "polling-rate"
	// CapHITS — Haptic Inductive Trigger System, analog left/right click.
	CapHITS Capability = "hits"
	// CapSmartShift — the ratcheting scroll wheel: whether it clicks or spins
	// free, and how fast it must be flicked to break into a free spin.
	CapSmartShift Capability = "smart-shift"
	// CapHiResWheel — scroll resolution and direction.
	CapHiResWheel Capability = "hi-res-wheel"
	// CapLOD — lift-off distance.
	CapLOD Capability = "lod"
	// CapOnboardProfile — onboard vs host profile storage.
	CapOnboardProfile Capability = "onboard-profile"
)

// USBID is a vendor/product pair a model shows up as. A model that enumerates
// differently wired and wireless lists both.
type USBID struct {
	Vendor  uint16 `json:"vendor"`
	Product uint16 `json:"product"`
}

func (id USBID) String() string {
	return fmt.Sprintf("%04X:%04X", id.Vendor, id.Product)
}

// Driver knows how to read and write a family of devices. The catalog knows
// which models use it. Keeping the two apart is what stops every new Logitech
// mouse from needing new code.
type Driver interface {
	// Name is a human-readable name for diagnostics.
	Name() string

	// Read returns everything the device's catalog entry claims it can do.
	// Individual capability failures land in DeviceState.Errors rather than
	// failing the whole read — a mouse whose DPI read times out should still
	// show its battery.
	Read(device *Device) DeviceState

	// Write applies one change.
	//
	// The setting is passed by pointer so a driver can report back what it
	// actually attempted: a device with coarser granularity than the request
	// has its value snapped first, and the caller needs the snapped number to
	// judge whether the write took.
	Write(device *Device, setting *Setting) error
}

// Entry is one model, as catalogued. Entries live next to the driver that
// handles them, under devices/<category>/<brand>/.
type Entry struct {
	// Model is the name exactly as the README prints it.
	Model string
	// Slug is the stable identifier used in device ids and CLI arguments.
	Slug     string
	Brand    Brand
	Category Category
	USB      []USBID
	// Names matches a device reported over HID++ rather than by USB id, for
	// devices behind a receiver the kernel did not expand into its own node.
	// Compared case-insensitively against feature 0x0005's answer.
	Names        []string
	Support      Support
	Capabilities []Capability
	// Driver is nil for a planned model: it is listed, and nothing more.
	Driver Driver
}

func (e *Entry) Has(capability Capability) bool {
	for _, c := range e.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// MatchesName reports whether a device calling itself `name` is this model.
func (e *Entry) MatchesName(name string) bool {
	for _, candidate := range e.Names {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

func (e *Entry) Matches(vendor, product uint16) bool {
	for _, id := range e.USB {
		if id.Vendor == vendor && id.Product == product {
			return true
		}
	}
	return false
}

// Connection is how a device is attached, in terms a user recognises. A
// wireless mouse is not just "wireless": which dongle it is paired to decides
// what it can do, and the answer is on the box it came in.
type Connection struct {
	// Kind is machine-readable: wired, bluetooth, lightspeed, unifying, bolt,
	// nano, 27mhz, or receiver for a dongle that is not in the table.
	Kind string `json:"kind"`
	// Label is what the UI prints.
	Label string `json:"label"`
}

// ConnectionOf describes how node is attached.
func ConnectionOf(node hidraw.Node) Connection {
	// The node may itself be a receiver, when the device behind it has no node
	// of its own. Then the link is that dongle — not the USB cable the dongle
	// happens to be plugged in with, which is what the sysfs tree would say.
	if kind, known := hidraw.ReceiverKindOf(node.Vendor, node.Product); known {
		return Connection{Kind: string(kind), Label: kind.Label()}
	}

	switch node.Link {
	case hidraw.Bluetooth:
		return Connection{Kind: "bluetooth", Label: "Bluetooth"}

	case hidraw.Wireless:
		kind, known := hidraw.ReceiverKindOf(node.Receiver.Vendor, node.Receiver.Product)
		if known {
			return Connection{Kind: string(kind), Label: kind.Label()}
		}
		// Name the id so an unlisted dongle can be reported and added rather
		// than silently flattened into "wireless".
		return Connection{
			Kind: string(kind),
			Label: fmt.Sprintf("%s (%04x:%04x)", kind.Label(),
				node.Receiver.Vendor, node.Receiver.Product),
		}

	default:
		return Connection{Kind: "wired", Label: "Wired"}
	}
}

// Device is a catalogued model that is actually plugged in right now.
type Device struct {
	Entry *Entry
	// ID is "<category>/<brand>/<slug>", plus "#<serial>" when the kernel
	// knows one. Deliberately the same shape as the source path the driver
	// lives at.
	ID string
	// Node is the hidraw node that answered.
	Node hidraw.Node
	// Index addresses the device behind a receiver. Zero means the node
	// speaks for the device directly and the driver can find the index itself.
	Index byte
}

// --- state -----------------------------------------------------------------

type Battery struct {
	// Percent is nil when the device reports only a coarse level.
	Percent *int `json:"percent"`
	// Level is "critical", "low", "good" or "full", for devices that bucket.
	Level string `json:"level,omitempty"`
	// Status is "discharging", "charging", "full" or "unknown".
	Status string `json:"status"`
}

type DPI struct {
	Current uint32 `json:"current"`
	Min     uint32 `json:"min"`
	Max     uint32 `json:"max"`
	Step    uint32 `json:"step"`
	// Presets are stops worth offering; empty means "use min/max/step".
	Presets []uint32 `json:"presets"`
}

type PollingRate struct {
	// Current is in Hz.
	Current   uint32   `json:"current"`
	Supported []uint32 `json:"supported"`
}

// SmartShift is the scroll wheel's ratchet behaviour.
type SmartShift struct {
	// Mode is "ratchet" or "freespin".
	Mode string `json:"mode"`
	// Threshold is the flick speed that breaks a ratcheting wheel into a free
	// spin. ThresholdNever means it never does.
	Threshold uint8 `json:"threshold"`
	// Default is the value the device shipped with, which it reports alongside
	// the current one.
	Default uint8 `json:"default"`
	// Max is the largest value this project offers. The device takes a whole
	// byte and does not report a range; past about this point the wheel
	// effectively never shifts, which is what ThresholdNever is for.
	Max uint8 `json:"max"`
}

// ThresholdNever is the threshold at which a ratcheting wheel never breaks
// into a free spin.
const ThresholdNever = 255

// Wheel modes, as the device numbers them. Zero means "leave it alone".
const (
	WheelUnchanged = 0
	WheelFreespin  = 1
	WheelRatchet   = 2
)

func WheelModeName(value uint32) string {
	switch value {
	case WheelFreespin:
		return "freespin"
	case WheelRatchet:
		return "ratchet"
	default:
		return "unknown"
	}
}

func WheelModeValue(name string) (uint32, bool) {
	switch name {
	case "freespin":
		return WheelFreespin, true
	case "ratchet":
		return WheelRatchet, true
	default:
		return 0, false
	}
}

// HiResWheel is how the scroll wheel reports movement.
type HiResWheel struct {
	// HiRes is finer-grained scrolling: more, smaller steps per notch.
	HiRes bool `json:"hiRes"`
	// Inverted flips the scroll direction in the hardware itself, so it
	// applies before anything the desktop does.
	Inverted bool `json:"inverted"`
}

type HITSButton struct {
	Actuation    uint8 `json:"actuation"`
	RapidTrigger uint8 `json:"rapidTrigger"`
	Haptics      uint8 `json:"haptics"`
}

type HITS struct {
	Left  HITSButton `json:"left"`
	Right HITSButton `json:"right"`
	// Limits the device reports for itself. The units are the device's own —
	// a SUPERSTRIKE counts actuation to 40 and the other two to 20 — and what
	// one unit means in millimetres is not something it says.
	MaxActuation    uint8 `json:"maxActuation"`
	MaxRapidTrigger uint8 `json:"maxRapidTrigger"`
	MaxHaptics      uint8 `json:"maxHaptics"`
	// Step is the granularity the device actually stores. It does not report
	// one, and it does not refuse a value off the grid — it silently rounds
	// down, which reads as the device ignoring the change.
	Step uint8 `json:"step"`
}

// DeviceState is everything a driver managed to read. Every field is optional:
// a capability the device claims but the read failed for comes back null with
// a line in Errors, rather than failing the whole device.
type DeviceState struct {
	// Connected is false when the device is catalogued and its node is still
	// present, but nothing answers — a wireless mouse switched off leaves its
	// node behind, because the dongle it is paired to is still plugged in.
	Connected      bool         `json:"connected"`
	Battery        *Battery     `json:"battery"`
	DPI            *DPI         `json:"dpi"`
	PollingRate    *PollingRate `json:"pollingRate"`
	HITS           *HITS        `json:"hits"`
	SmartShift     *SmartShift  `json:"smartShift"`
	HiResWheel     *HiResWheel  `json:"hiResWheel"`
	LOD            *string      `json:"lod"`
	OnboardProfile *string      `json:"onboardProfile"`
	// Errors holds non-fatal problems, one per capability that could not be
	// read. Never nil, so it marshals as [] rather than null.
	Errors []string `json:"errors"`
}

func NewDeviceState() DeviceState {
	return DeviceState{Connected: true, Errors: []string{}}
}

func (s *DeviceState) Fail(capability Capability, err error) {
	s.Errors = append(s.Errors, fmt.Sprintf("%s: %v", capability, err))
}

// --- settings --------------------------------------------------------------

type SettingKey string

const (
	SettingDPI         SettingKey = "dpi"
	SettingPollingRate SettingKey = "polling-rate"
	SettingProfileMode SettingKey = "profile-mode"

	SettingWheelHiRes  SettingKey = "wheel-hi-res"
	SettingWheelInvert SettingKey = "wheel-invert"

	SettingSmartShiftMode      SettingKey = "smart-shift-mode"
	SettingSmartShiftThreshold SettingKey = "smart-shift-threshold"

	// HITS is per click and per field, so each combination is its own key.
	// Three fields across two buttons is small enough to name outright, and
	// keeps a Setting a plain key and number.
	SettingHITSLeftActuation    SettingKey = "hits-left-actuation"
	SettingHITSLeftRapidTrigger SettingKey = "hits-left-rapid-trigger"
	SettingHITSLeftHaptics      SettingKey = "hits-left-haptics"

	SettingHITSRightActuation    SettingKey = "hits-right-actuation"
	SettingHITSRightRapidTrigger SettingKey = "hits-right-rapid-trigger"
	SettingHITSRightHaptics      SettingKey = "hits-right-haptics"
)

// hitsSettings is every HITS key, so parsing and reading back stay in step.
var hitsSettings = map[SettingKey]struct{}{
	SettingHITSLeftActuation:     {},
	SettingHITSLeftRapidTrigger:  {},
	SettingHITSLeftHaptics:       {},
	SettingHITSRightActuation:    {},
	SettingHITSRightRapidTrigger: {},
	SettingHITSRightHaptics:      {},
}

// Profile modes, as both the wire and this API spell them. The numbers are
// HID++ feature 0x8100's own, so a Setting can stay a plain number.
const (
	ProfileModeOnboard = 1
	ProfileModeHost    = 2
)

// ProfileModeName turns the wire value into the word the UI and CLI use.
func ProfileModeName(value uint32) string {
	switch value {
	case ProfileModeOnboard:
		return "onboard"
	case ProfileModeHost:
		return "host"
	default:
		return "unknown"
	}
}

// ProfileModeValue is the inverse, for verifying a write against a fresh read.
func ProfileModeValue(name string) (uint32, bool) {
	switch name {
	case "onboard":
		return ProfileModeOnboard, true
	case "host":
		return ProfileModeHost, true
	default:
		return 0, false
	}
}

// Setting is a change the UI asks for.
type Setting struct {
	Key   SettingKey
	Value uint32
}

// ParseSetting parses the key and value of `omnigear set <device> <key> <value>`.
func ParseSetting(key, value string) (Setting, error) {
	var settingKey SettingKey
	switch key {
	case "dpi":
		settingKey = SettingDPI
	case "polling-rate", "rate":
		settingKey = SettingPollingRate
	case "wheel-hi-res", "wheel-invert":
		on, ok := parseSwitch(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not on or off", value)
		}
		return Setting{Key: SettingKey(key), Value: on}, nil
	case "smart-shift-mode", "wheel-mode":
		mode, ok := WheelModeValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a wheel mode (expected: ratchet, freespin)", value)
		}
		return Setting{Key: SettingSmartShiftMode, Value: mode}, nil
	case "smart-shift-threshold", "wheel-threshold":
		settingKey = SettingSmartShiftThreshold
	case "profile-mode", "profile":
		// The only setting named rather than numbered. Its values are the two
		// words a user would say, not 1 and 2.
		mode, ok := ProfileModeValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a profile mode (expected: onboard, host)", value)
		}
		return Setting{Key: SettingProfileMode, Value: mode}, nil
	default:
		if _, ok := hitsSettings[SettingKey(key)]; ok {
			settingKey = SettingKey(key)
			break
		}
		return Setting{}, fmt.Errorf("unknown setting %q (expected one of: dpi, "+
			"polling-rate, profile-mode, smart-shift-mode, smart-shift-threshold, "+
			"wheel-hi-res, wheel-invert, "+
			"hits-{left,right}-{actuation,rapid-trigger,haptics})", key)
	}

	number, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return Setting{}, fmt.Errorf("%q is not a whole number", value)
	}
	return Setting{Key: settingKey, Value: uint32(number)}, nil
}

// parseSwitch reads the words people actually type for a two-state setting.
func parseSwitch(value string) (uint32, bool) {
	switch strings.ToLower(value) {
	case "on", "true", "yes", "1", "high", "inverted":
		return 1, true
	case "off", "false", "no", "0", "low", "standard", "normal":
		return 0, true
	default:
		return 0, false
	}
}

func boolToValue(on bool) uint32 {
	if on {
		return 1
	}
	return 0
}

// Reading pulls the one number a setting is about back out of a fresh read.
// The second result is false when the device did not report it at all.
func (s *DeviceState) Reading(key SettingKey) (uint32, bool) {
	switch key {
	case SettingDPI:
		if s.DPI != nil {
			return s.DPI.Current, true
		}
	case SettingPollingRate:
		if s.PollingRate != nil {
			return s.PollingRate.Current, true
		}
	case SettingProfileMode:
		if s.OnboardProfile != nil {
			if value, ok := ProfileModeValue(*s.OnboardProfile); ok {
				return value, true
			}
		}

	case SettingWheelHiRes:
		if s.HiResWheel != nil {
			return boolToValue(s.HiResWheel.HiRes), true
		}

	case SettingWheelInvert:
		if s.HiResWheel != nil {
			return boolToValue(s.HiResWheel.Inverted), true
		}

	case SettingSmartShiftMode:
		if s.SmartShift != nil {
			if value, ok := WheelModeValue(s.SmartShift.Mode); ok {
				return value, true
			}
		}

	case SettingSmartShiftThreshold:
		if s.SmartShift != nil {
			return uint32(s.SmartShift.Threshold), true
		}

	case SettingHITSLeftActuation, SettingHITSLeftRapidTrigger, SettingHITSLeftHaptics,
		SettingHITSRightActuation, SettingHITSRightRapidTrigger, SettingHITSRightHaptics:
		if s.HITS == nil {
			return 0, false
		}
		button := s.HITS.Left
		if key == SettingHITSRightActuation || key == SettingHITSRightRapidTrigger ||
			key == SettingHITSRightHaptics {
			button = s.HITS.Right
		}
		switch key {
		case SettingHITSLeftActuation, SettingHITSRightActuation:
			return uint32(button.Actuation), true
		case SettingHITSLeftRapidTrigger, SettingHITSRightRapidTrigger:
			return uint32(button.RapidTrigger), true
		default:
			return uint32(button.Haptics), true
		}
	}
	return 0, false
}

// --- json ------------------------------------------------------------------

// EntryJSON is a catalog entry as JSON: the support matrix, with no hardware
// present.
type EntryJSON struct {
	Model        string       `json:"model"`
	Slug         string       `json:"slug"`
	Brand        Brand        `json:"brand"`
	BrandLabel   string       `json:"brandLabel"`
	Category     Category     `json:"category"`
	Support      Support      `json:"support"`
	Capabilities []Capability `json:"capabilities"`
	USB          []USBID      `json:"usb"`
}

func (e *Entry) JSON() EntryJSON {
	return EntryJSON{
		Model:        e.Model,
		Slug:         e.Slug,
		Brand:        e.Brand,
		BrandLabel:   e.Brand.Label(),
		Category:     e.Category,
		Support:      e.Support,
		Capabilities: nonNilCapabilities(e.Capabilities),
		USB:          nonNilUSB(e.USB),
	}
}

// DeviceJSON is a device plus its live state, as the QML side consumes it.
type DeviceJSON struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Brand        Brand        `json:"brand"`
	BrandLabel   string       `json:"brandLabel"`
	Category     Category     `json:"category"`
	Slug         string       `json:"slug"`
	Support      Support      `json:"support"`
	Capabilities []Capability `json:"capabilities"`
	Connection   Connection   `json:"connection"`
	Path         string       `json:"path"`
	State        DeviceState  `json:"state"`
}

func (d *Device) JSON(state DeviceState) DeviceJSON {
	return DeviceJSON{
		ID:           d.ID,
		Name:         d.Entry.Model,
		Brand:        d.Entry.Brand,
		BrandLabel:   d.Entry.Brand.Label(),
		Category:     d.Entry.Category,
		Slug:         d.Entry.Slug,
		Support:      d.Entry.Support,
		Capabilities: nonNilCapabilities(d.Entry.Capabilities),
		Connection:   ConnectionOf(d.Node),
		Path:         d.Node.Path,
		State:        state,
	}
}

// Empty slices marshal as [], nil marshals as null. The UI iterates these, so
// they are always a list.
func nonNilCapabilities(in []Capability) []Capability {
	if in == nil {
		return []Capability{}
	}
	return in
}

func nonNilUSB(in []USBID) []USBID {
	if in == nil {
		return []USBID{}
	}
	return in
}

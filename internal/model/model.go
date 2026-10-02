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
	// Unknown is a device found on the wire that is not in the catalog. What
	// kind of thing it is cannot be told from outside: a keyboard with
	// mouse-keys presents a mouse to the kernel, and a device behind an
	// unexpanded receiver presents nothing at all. Rather than guess wrong in
	// the UI, it is left unsaid.
	Unknown Category = "unknown"
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
	// SupportUnsupported — not catalogued at all. Found by asking the device
	// what it is and what it implements, so whatever works here works by
	// accident of the protocol rather than by anyone having tested it.
	SupportUnsupported Support = "unsupported"
)

func (s Support) Emoji() string {
	switch s {
	case SupportFull:
		return "🟩"
	case SupportPartial:
		return "🟨"
	case SupportUnsupported:
		return "⬜"
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
	// CapHost — the Easy-Switch host slots a device is paired to, and which
	// one it is currently talking to.
	CapHost Capability = "host"
	// CapThumbwheel — the horizontal wheel under the thumb: whether it
	// scrolls or has been handed to other software.
	CapThumbwheel Capability = "thumbwheel"
	// CapButtons — reassigning what the device's buttons do, in hardware.
	CapButtons Capability = "buttons"
	// CapNoiseControl — noise cancelling, ambient sound, or neither, and how
	// much of the room the ambient mode lets in.
	CapNoiseControl Capability = "noise-control"
	// CapEqualizer — a headset's preset sound profiles, and its adjustable
	// bands.
	CapEqualizer Capability = "equalizer"
	// CapCodec — which Bluetooth codec the audio is played over. Chosen by
	// this machine's sound server, not stored in the headset.
	CapCodec Capability = "codec"
	// CapAutoPowerOff — when a headset switches itself off.
	CapAutoPowerOff Capability = "auto-power-off"
	// CapDSEE — Sony's upscaling of compressed audio, which restores the
	// high frequencies lossy formats drop. DSEE HX on an XM3, DSEE Extreme on
	// an XM4.
	CapDSEE Capability = "dsee"
	// CapSpeakToChat — a headset that pauses and lets the room in when its
	// wearer starts talking.
	CapSpeakToChat Capability = "speak-to-chat"
	// CapTouchPanel — whether a headset's touch-sensitive earcup takes
	// gestures.
	CapTouchPanel Capability = "touch-panel"
	// CapSidetone — how much of the wearer's own voice the microphone plays
	// back into the headset.
	CapSidetone Capability = "sidetone"
	// CapMicVolume — the microphone's own gain, before anything on the host.
	CapMicVolume Capability = "mic-volume"
	// CapMuteLight — how bright the light that says the microphone is muted.
	CapMuteLight Capability = "mute-light"
	// CapGain — the headset's output range: low for sensitive ears, high for
	// headroom.
	CapGain Capability = "gain"
	// CapWirelessMode — whether a headset's own wireless link favours
	// latency or range.
	CapWirelessMode Capability = "wireless-mode"
	// CapSonar — separate outputs for game, chat, media and the rest, with a
	// ChatMix dial on the device balancing game against chat. The outputs
	// are the sound server's; the dial is the device's.
	CapSonar Capability = "sonar"
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

// Speaker is a driver that can tell, from a node's report descriptor alone,
// whether the node carries its protocol.
//
// A device owns several nodes and usually only one of them is any use. HID++
// is recognised without a driver's help; any other protocol is the driver's
// to recognise, and discovery asks it rather than guessing.
type Speaker interface {
	Speaks(node hidraw.Node) bool
}

// BatteryReader is a driver that can report charge cheaply enough for
// `omnigear battery`, which runs on every tick of the bar and must wake
// nothing. A base station that keeps its headset's status itself qualifies;
// asking a wireless mouse does not.
//
// It is handed a node of the device rather than a Device, because the cheap
// path never runs discovery. The second result is false when the node is not
// one the driver can read.
type BatteryReader interface {
	Battery(node hidraw.Node) (reading BatteryReading, ok bool)
}

// ChatMixer is a driver for a device with a ChatMix dial, which reports the
// dial as it turns.
//
// Watch blocks, calling onMix with the game and chat levels, each 0–100,
// every time the dial moves, and returns only when the device can no longer
// be read. The device says nothing until the dial moves; there is no reading
// to ask for.
type ChatMixer interface {
	WatchChatMix(node hidraw.Node, onMix func(game, chat uint8)) error
}

// BatteryReading is a cheap battery answer. Battery is nil when the device is
// present but has nothing to report, as a headset switched off beside its
// base station does.
type BatteryReading struct {
	Battery   *Battery
	Connected bool
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
	// Icon names a drawing style for models that look like something in
	// particular. Empty means the plain category icon, which is the right
	// answer for almost everything.
	Icon string
	// Driver is nil for a planned model: it is listed, and nothing more.
	Driver Driver
	// Discovered marks an entry assembled at runtime from what a device said
	// about itself, rather than one written down in the catalog. Everything
	// on such an entry is the device's own account of itself.
	Discovered bool
	// USBLabel is the vendor:product the device enumerated as, kept so a bug
	// report can name it. Catalogued entries carry their ids in USB instead.
	USBLabel string
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
	// A Bluetooth device reached without hidraw has an empty node; see
	// Device.Address.
	if node.Path == "" {
		return Connection{Kind: "bluetooth", Label: "Bluetooth"}
	}

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
	// Address is the Bluetooth MAC of a device that has no hidraw node at all
	// — headphones, which are audio devices with a vendor control channel
	// rather than HID. Node is empty for such a device.
	Address string
}

// Path names what the device is reached through: its hidraw node, or its
// Bluetooth address when it has none.
func (d *Device) Path() string {
	if d.Node.Path == "" {
		return d.Address
	}
	return d.Node.Path
}

// --- state -----------------------------------------------------------------

type Battery struct {
	// Percent is nil when the device reports only a coarse level.
	Percent *int `json:"percent"`
	// Level is "critical", "low", "good" or "full", for devices that bucket.
	Level string `json:"level,omitempty"`
	// Status is "discharging", "charging", "full" or "unknown".
	Status string `json:"status"`
	// Spare is a second battery's charge, for a headset whose base station
	// charges one to swap in. Nil for everything else.
	Spare *int `json:"spare,omitempty"`
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

// Thumbwheel modes. The wire uses these numbers directly.
const (
	// ThumbwheelScroll is the wheel doing what it is for: ordinary HID
	// horizontal scroll events.
	ThumbwheelScroll = 0
	// ThumbwheelDiverted sends movement as HID++ notifications instead.
	// Nothing in OmniGear reads those, so unless another client is handling
	// them the wheel does nothing at all.
	ThumbwheelDiverted = 1
)

func ThumbwheelModeName(value uint32) string {
	if value == ThumbwheelDiverted {
		return "diverted"
	}
	return "scroll"
}

func ThumbwheelModeValue(name string) (uint32, bool) {
	switch name {
	case "scroll", "on", "native":
		return ThumbwheelScroll, true
	case "diverted", "off", "divert":
		return ThumbwheelDiverted, true
	default:
		return 0, false
	}
}

// Thumbwheel is the horizontal wheel under the thumb on the MX line.
type Thumbwheel struct {
	// Mode is "scroll" or "diverted".
	Mode string `json:"mode"`
}

// Hosts is Easy-Switch: the machines a device is paired to, and which one it
// is talking to now.
//
// Slots are numbered from 1, the way the buttons on the underside of the
// device and Logitech's own software number them. The wire numbers them from
// 0; the driver does that conversion so nothing above it has to.
type Hosts struct {
	// Current is the slot this machine is talking to, which is by definition
	// the one being asked.
	Current int    `json:"current"`
	Slots   []Host `json:"slots"`
	// PairingKnown says whether Host.Paired means anything.
	//
	// Switching hosts is feature 0x1814; describing the slots is 0x1815, and
	// a device can have the first without the second — an MX Master 3 does.
	// Then all that is known is how many slots there are and which one is
	// live, so Paired is false everywhere and must not be read as "empty".
	PairingKnown bool `json:"pairingKnown"`
}

type Host struct {
	Slot int `json:"slot"`
	// Paired is false for a slot that has never been paired. Switching to one
	// leaves the device looking for a host that is not there, so the UI offers
	// it as a slot to pair rather than a slot to switch to.
	Paired bool `json:"paired"`
	// Name is what the host called itself when it paired, empty if the device
	// never stored one.
	Name   string `json:"name"`
	Active bool   `json:"active"`
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

// Noise control modes. The numbers are this project's, not the wire's: a
// Setting is a plain number, and these are what `noise-mode` carries.
const (
	NoiseOff        = 0
	NoiseAmbient    = 1
	NoiseCancelling = 2
)

func NoiseModeName(value uint32) string {
	switch value {
	case NoiseOff:
		return "off"
	case NoiseAmbient:
		return "ambient"
	case NoiseCancelling:
		return "noise-cancelling"
	default:
		return "unknown"
	}
}

func NoiseModeValue(name string) (uint32, bool) {
	switch strings.ToLower(name) {
	case "off":
		return NoiseOff, true
	case "ambient", "ambient-sound", "asm":
		return NoiseAmbient, true
	case "noise-cancelling", "nc", "anc":
		return NoiseCancelling, true
	default:
		return 0, false
	}
}

// NoiseControl is what a headset does with the sound of the room.
type NoiseControl struct {
	// Mode is "noise-cancelling", "ambient" or "off".
	Mode string `json:"mode"`
	// AmbientLevel is how much of the room ambient mode lets through, from
	// MinAmbientLevel to MaxAmbientLevel. The headset remembers it in every
	// mode, but it only has an effect in ambient.
	AmbientLevel    uint8 `json:"ambientLevel"`
	MinAmbientLevel uint8 `json:"minAmbientLevel"`
	MaxAmbientLevel uint8 `json:"maxAmbientLevel"`
	// FocusOnVoice filters ambient sound down to speech. Nil for a headset
	// that has no such filter.
	FocusOnVoice *bool `json:"focusOnVoice"`
	// AmbientLabel is the model's own name for ambient mode, where it is not
	// "Ambient Sound": SteelSeries calls it Transparency. Empty means the
	// usual name.
	AmbientLabel string `json:"ambientLabel,omitempty"`
}

// Equalizer is a headset's sound profile.
type Equalizer struct {
	// Available is false when the headset will not apply an equalizer right
	// now, and Unavailable says why. A WH-1000XM3 refuses one outright over
	// LDAC or aptX; the reading is still there, it just does nothing.
	Available   bool   `json:"available"`
	Unavailable string `json:"unavailable,omitempty"`
	// Codecs are the codecs, as CodecOption slugs, over which the device does
	// apply an equalizer — so the panel can offer to switch to one.
	Codecs []string `json:"codecs"`
	// Preset is a slug from EQPresets.
	Preset  string     `json:"preset"`
	Presets []EQPreset `json:"presets"`
	// Bands are what the current preset does, in order: Clear Bass first on a
	// Sony, then low to high frequency.
	Bands []EQBand `json:"bands"`
	Min   int      `json:"min"`
	Max   int      `json:"max"`
}

// EQPreset is one sound profile the device offers.
type EQPreset struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	// Editable presets keep bands of their own. Changing a band on any other
	// preset moves the device onto the first editable one, carrying the
	// bands over, which is what the Sony app does too.
	Editable bool `json:"editable"`
}

// EQBand is one adjustable band.
type EQBand struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

// eqPresetSlugs are every preset slug any driver knows, so a setting can name
// one before the device is asked. The number is the slug's index here, which
// is what a Setting carries; drivers map it to their own wire value.
var eqPresetSlugs = []string{
	"off", "bright", "excited", "mellow", "relaxed", "vocal",
	"treble-boost", "bass-boost", "speech", "manual", "custom-1", "custom-2",
}

func EQPresetValue(slug string) (uint32, bool) {
	for i, candidate := range eqPresetSlugs {
		if strings.EqualFold(candidate, slug) {
			return uint32(i), true
		}
	}
	return 0, false
}

func EQPresetSlug(value uint32) string {
	if int(value) < len(eqPresetSlugs) {
		return eqPresetSlugs[value]
	}
	return ""
}

// EQBias is added to a band level to carry it in a Setting, whose value is
// unsigned. A level of -3 travels as 125.
const EQBias = 128

// EQBandKey is the setting key for one band.
func EQBandKey(slug string) SettingKey { return SettingKey("eq-" + slug) }

// EQBandSlugOf is the inverse of EQBandKey.
func EQBandSlugOf(key SettingKey) (string, bool) {
	slug, found := strings.CutPrefix(string(key), "eq-")
	if !found || key == SettingEQPreset || slug == "" {
		return "", false
	}
	return slug, true
}

// Band returns the band with a slug, or nil.
func (e *Equalizer) Band(slug string) *EQBand {
	for i := range e.Bands {
		if e.Bands[i].Slug == slug {
			return &e.Bands[i]
		}
	}
	return nil
}

// Codec is which Bluetooth codec a headset's audio is played over, and which
// others the sound server would use instead.
type Codec struct {
	// Current is empty when the card is not playing to the headset at all —
	// in a call on the hands-free profile, or switched off.
	Current string        `json:"current"`
	Options []CodecOption `json:"options"`
}

type CodecOption struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

// codecSlugs are the codecs PipeWire's Bluetooth layer can play over, in
// rough order of quality. The index is what a Setting carries.
var codecSlugs = []string{
	"sbc", "sbc-xq", "faststream", "aac", "aac-eld", "aptx", "aptx-ll",
	"aptx-hd", "opus-05", "lc3", "lc3plus-hr", "ldac",
}

// CodecSlug turns the sound server's name for a codec — "aptX HD", "SBC-XQ" —
// into a slug.
func CodecSlug(label string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(label), " ", "-"))
}

func CodecValue(slug string) (uint32, bool) {
	for i, candidate := range codecSlugs {
		if candidate == slug {
			return uint32(i), true
		}
	}
	return 0, false
}

func CodecName(value uint32) string {
	if int(value) < len(codecSlugs) {
		return codecSlugs[value]
	}
	return ""
}

// Choice is a setting with a handful of named values, and the ones the device
// offers.
type Choice struct {
	Current string         `json:"current"`
	Options []ChoiceOption `json:"options"`
}

type ChoiceOption struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

// Level is a setting that is a whole number in a range.
type Level struct {
	Current uint8 `json:"current"`
	Min     uint8 `json:"min"`
	Max     uint8 `json:"max"`
}

// choiceValue and choiceName turn a choice's slug into the number a Setting
// carries, its index in slugs, and back.
func choiceValue(slugs []string, slug string) (uint32, bool) {
	for i, candidate := range slugs {
		if strings.EqualFold(candidate, slug) {
			return uint32(i), true
		}
	}
	return 0, false
}

func choiceName(slugs []string, value uint32) string {
	if int(value) < len(slugs) {
		return slugs[value]
	}
	return ""
}

// AutoPowerOff is when a headset switches itself off, and the choices it
// offers. Which choices exist differs by model, so the driver lists them.
type AutoPowerOff = Choice

type AutoPowerOffOption = ChoiceOption

// autoPowerOffSlugs are every auto power off choice any driver knows. The
// index is what a Setting carries, so new ones go on the end.
var autoPowerOffSlugs = []string{"never", "when-taken-off", "5-min", "30-min", "1-hour", "3-hours",
	"1-min", "10-min", "15-min"}

func AutoPowerOffValue(slug string) (uint32, bool) { return choiceValue(autoPowerOffSlugs, slug) }

func AutoPowerOffName(value uint32) string { return choiceName(autoPowerOffSlugs, value) }

// sidetoneSlugs, gainSlugs and wirelessModeSlugs are the values of the
// settings by those names. As with auto power off, the index is what a
// Setting carries.
var (
	sidetoneSlugs     = []string{"off", "low", "medium", "high"}
	gainSlugs         = []string{"low", "high"}
	wirelessModeSlugs = []string{"speed", "range"}
)

func SidetoneValue(slug string) (uint32, bool) { return choiceValue(sidetoneSlugs, slug) }
func SidetoneName(value uint32) string         { return choiceName(sidetoneSlugs, value) }

func GainValue(slug string) (uint32, bool) { return choiceValue(gainSlugs, slug) }
func GainName(value uint32) string         { return choiceName(gainSlugs, value) }

func WirelessModeValue(slug string) (uint32, bool) { return choiceValue(wirelessModeSlugs, slug) }
func WirelessModeName(value uint32) string         { return choiceName(wirelessModeSlugs, value) }

// DSEE is whether a Sony headset's upscaling is on. Label is the model's own
// name for it, since the same switch is sold under more than one.
type DSEE struct {
	On    bool   `json:"on"`
	Label string `json:"label"`
}

// Sonar is whether a headset's separate outputs exist.
type Sonar struct {
	Enabled bool `json:"enabled"`
	// Live is false when Sonar is on but the sound server has not made the
	// outputs, which needs the headset plugged in.
	Live     bool           `json:"live"`
	Channels []SonarChannel `json:"channels"`
	// Apps is what is playing now, and on which channel. Empty while Sonar
	// is off.
	Apps []SonarApp `json:"apps"`
}

// SonarApp is an application playing sound.
type SonarApp struct {
	// Name is the app's own name for its stream, and the key it is routed
	// by. Label is what to show; often better, since an app can name its
	// stream after its audio library.
	Name  string `json:"name"`
	Label string `json:"label"`
	// Channel is a channel's slug, or empty for an app playing somewhere
	// that is not a channel.
	Channel string `json:"channel"`
}

// sonarChannelSlugs are Sonar's channels, in order. The index is what a
// setting carries.
var sonarChannelSlugs = []string{"game", "chat", "media", "aux"}

func SonarChannelValue(slug string) (uint32, bool) { return choiceValue(sonarChannelSlugs, slug) }
func SonarChannelName(value uint32) string         { return choiceName(sonarChannelSlugs, value) }

// SonarAppKey is the setting key that moves one application to a channel.
// Like a button's, the key names something only the device's read knows of.
func SonarAppKey(name string) SettingKey { return SettingKey("sonar-app-" + name) }

// SonarVolumeKey is the setting key for one channel's volume.
func SonarVolumeKey(slug string) SettingKey { return SettingKey("sonar-volume-" + slug) }

// SonarMuteKey is the setting key for one channel's mute.
func SonarMuteKey(slug string) SettingKey { return SettingKey("sonar-mute-" + slug) }

// SonarMuteSlugOf is the inverse of SonarMuteKey.
func SonarMuteSlugOf(key SettingKey) (string, bool) {
	slug, found := strings.CutPrefix(string(key), "sonar-mute-")
	if !found {
		return "", false
	}
	_, known := SonarChannelValue(slug)
	return slug, known
}

// SonarVolumeSlugOf is the inverse of SonarVolumeKey.
func SonarVolumeSlugOf(key SettingKey) (string, bool) {
	slug, found := strings.CutPrefix(string(key), "sonar-volume-")
	if !found {
		return "", false
	}
	_, known := SonarChannelValue(slug)
	return slug, known
}

// SonarEQKey is the setting key for one channel's equalizer: its preset when
// band is "preset", otherwise one band.
func SonarEQKey(channel, band string) SettingKey {
	return SettingKey("sonar-eq-" + channel + "-" + band)
}

// SonarEQOf is the inverse of SonarEQKey. Band is "preset" for the preset.
func SonarEQOf(key SettingKey) (channel, band string, ok bool) {
	rest, found := strings.CutPrefix(string(key), "sonar-eq-")
	if !found {
		return "", "", false
	}
	channel, band, found = strings.Cut(rest, "-")
	if _, known := SonarChannelValue(channel); !known || !found || band == "" {
		return "", "", false
	}
	return channel, band, true
}

// SonarAppNameOf is the inverse of SonarAppKey.
func SonarAppNameOf(key SettingKey) (string, bool) {
	name, found := strings.CutPrefix(string(key), "sonar-app-")
	return name, found && name != ""
}

// SonarChannel is one output, and the sink applications are pointed at to
// use it.
type SonarChannel struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	Sink  string `json:"sink"`
	// Mixed is true for the channels the ChatMix dial balances.
	Mixed bool `json:"mixed"`
	// Volume is the channel's own level, in percent: the one a mixer shows
	// for it, which the dial scales rather than replaces. Zero while Sonar
	// is not live.
	Volume int  `json:"volume"`
	Muted  bool `json:"muted"`
	// Equalizer is the channel's own, applied in software.
	Equalizer *Equalizer `json:"equalizer"`
}

// DeviceState is everything a driver managed to read. Every field is optional:
// a capability the device claims but the read failed for comes back null with
// a line in Errors, rather than failing the whole device.
type DeviceState struct {
	// Connected is false when the device is catalogued and its node is still
	// present, but nothing answers — a wireless mouse switched off leaves its
	// node behind, because the dongle it is paired to is still plugged in.
	Connected bool `json:"connected"`
	// Presence is the finer answer behind Connected. See the Presence
	// constants: a device can be absent because it is switched off, or
	// present but have needed waking.
	Presence       string        `json:"presence"`
	Battery        *Battery      `json:"battery"`
	DPI            *DPI          `json:"dpi"`
	PollingRate    *PollingRate  `json:"pollingRate"`
	HITS           *HITS         `json:"hits"`
	SmartShift     *SmartShift   `json:"smartShift"`
	HiResWheel     *HiResWheel   `json:"hiResWheel"`
	LOD            *string       `json:"lod"`
	OnboardProfile *string       `json:"onboardProfile"`
	Hosts          *Hosts        `json:"hosts"`
	Thumbwheel     *Thumbwheel   `json:"thumbwheel"`
	Buttons        []Button      `json:"buttons"`
	NoiseControl   *NoiseControl `json:"noiseControl"`
	Equalizer      *Equalizer    `json:"equalizer"`
	Codec          *Codec        `json:"codec"`
	AutoPowerOff   *AutoPowerOff `json:"autoPowerOff"`
	DSEE           *DSEE         `json:"dsee"`
	SpeakToChat    *bool         `json:"speakToChat"`
	TouchPanel     *bool         `json:"touchPanel"`
	Sidetone       *Choice       `json:"sidetone"`
	MicVolume      *Level        `json:"micVolume"`
	MuteLight      *Level        `json:"muteLight"`
	Gain           *Choice       `json:"gain"`
	WirelessMode   *Choice       `json:"wirelessMode"`
	Sonar          *Sonar        `json:"sonar"`
	// Errors holds non-fatal problems, one per capability that could not be
	// read. Never nil, so it marshals as [] rather than null.
	Errors []string `json:"errors"`
}

// How present a device is, beyond the yes/no of Connected.
//
// "Asleep" is necessarily retrospective. A sleeping device cannot be observed
// while sleeping, because the only way to ask it anything is to wake it; what
// can be observed is that it needed the full wake window to answer, which an
// awake device never does. So the label means "was asleep when we reached
// it", and the device dozes off again between polls.
//
// The distinction that is *not* retrospective is off versus unreachable, and
// that comes from the kernel rather than the device: hid-logitech-hidpp
// tracks the wireless link and says so without touching the hardware.
const (
	// PresenceAwake — answered straight away.
	PresenceAwake = "awake"
	// PresenceAsleep — answered, but only after the wake window.
	PresenceAsleep = "asleep"
	// PresenceOff — did not answer, and the kernel says the link is down.
	PresenceOff = "off"
	// PresenceUnreachable — did not answer, and nothing knows why.
	PresenceUnreachable = "unreachable"
	// PresenceBlocked — the node is there and we are not allowed to open it.
	// Distinct from unreachable because the device is almost certainly fine
	// and the machine is the thing that needs changing.
	PresenceBlocked = "blocked"
)

func NewDeviceState() DeviceState {
	return DeviceState{Connected: true, Presence: PresenceAwake, Errors: []string{}}
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

	// SettingHost switches the device to another Easy-Switch slot. See
	// Verifiable: this is the one write that cannot be read back.
	SettingHost SettingKey = "host"

	SettingThumbwheel SettingKey = "thumbwheel"

	SettingNoiseMode    SettingKey = "noise-mode"
	SettingAmbientLevel SettingKey = "ambient-level"
	SettingFocusOnVoice SettingKey = "focus-on-voice"

	// SettingEQPreset picks a preset. Bands are eq-<band slug>; see EQBandKey.
	SettingEQPreset SettingKey = "eq-preset"

	SettingCodec SettingKey = "codec"

	SettingAutoPowerOff SettingKey = "auto-power-off"

	SettingDSEE SettingKey = "dsee"

	SettingSpeakToChat SettingKey = "speak-to-chat"

	SettingTouchPanel SettingKey = "touch-panel"

	SettingSidetone  SettingKey = "sidetone"
	SettingMicVolume SettingKey = "mic-volume"
	SettingMuteLight SettingKey = "mute-light"
	SettingGain      SettingKey = "gain"
	// SettingWirelessMode drops the headset's link while it moves over; see
	// drivers/steelseries.
	SettingWirelessMode SettingKey = "wireless-mode"
	// SettingSonar restarts the sound server; see package sonar.
	SettingSonar SettingKey = "sonar"

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
	case "host":
		settingKey = SettingHost
	case "thumbwheel":
		mode, ok := ThumbwheelModeValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a thumbwheel mode (expected: scroll, diverted)", value)
		}
		return Setting{Key: SettingThumbwheel, Value: mode}, nil
	case "noise-mode", "noise":
		mode, ok := NoiseModeValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a noise mode (expected: noise-cancelling, ambient, off)", value)
		}
		return Setting{Key: SettingNoiseMode, Value: mode}, nil
	case "ambient-level":
		settingKey = SettingAmbientLevel
	case "focus-on-voice", "voice":
		on, ok := parseSwitch(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not on or off", value)
		}
		return Setting{Key: SettingFocusOnVoice, Value: on}, nil
	case "codec":
		codec, ok := CodecValue(CodecSlug(value))
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a codec (expected one of: %s)",
				value, strings.Join(codecSlugs, ", "))
		}
		return Setting{Key: SettingCodec, Value: codec}, nil
	case "dsee", "speak-to-chat", "touch-panel", "sonar":
		on, ok := parseSwitch(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not on or off", value)
		}
		return Setting{Key: SettingKey(key), Value: on}, nil
	case "auto-power-off":
		choice, ok := AutoPowerOffValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not an auto power off choice (expected one of: %s)",
				value, strings.Join(autoPowerOffSlugs, ", "))
		}
		return Setting{Key: SettingAutoPowerOff, Value: choice}, nil
	case "sidetone":
		choice, ok := SidetoneValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a sidetone level (expected one of: %s)",
				value, strings.Join(sidetoneSlugs, ", "))
		}
		return Setting{Key: SettingSidetone, Value: choice}, nil
	case "gain":
		// Not a switch, although parseSwitch would take "low" and "high":
		// these are the gain's names, not off and on.
		choice, ok := GainValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a gain (expected one of: %s)",
				value, strings.Join(gainSlugs, ", "))
		}
		return Setting{Key: SettingGain, Value: choice}, nil
	case "wireless-mode":
		choice, ok := WirelessModeValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not a wireless mode (expected one of: %s)",
				value, strings.Join(wirelessModeSlugs, ", "))
		}
		return Setting{Key: SettingWirelessMode, Value: choice}, nil
	case "mic-volume", "mute-light":
		settingKey = SettingKey(key)
	case "eq-preset":
		preset, ok := EQPresetValue(value)
		if !ok {
			return Setting{}, fmt.Errorf("%q is not an equalizer preset (expected one of: %s)",
				value, strings.Join(eqPresetSlugs, ", "))
		}
		return Setting{Key: SettingEQPreset, Value: preset}, nil
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
		if slug, ok := ButtonSlugOf(SettingKey(key)); ok {
			// The value is the button it should act as, by name. "default" is
			// resolved to the button's own id here rather than in the driver,
			// so what was asked for and what gets written are the same number
			// and a plain reset does not report itself as a snapped value.
			target, ok := ButtonCID(value)
			if strings.EqualFold(value, "default") {
				target, ok = ButtonCID(slug)
			}
			if !ok {
				return Setting{}, fmt.Errorf("%q is not a button (expected one of: "+
					"left, right, middle, back, forward, gesture, wheel-mode, default)", value)
			}
			return Setting{Key: SettingKey(key), Value: uint32(target)}, nil
		}
		if _, ok := SonarVolumeSlugOf(SettingKey(key)); ok {
			settingKey = SettingKey(key)
			break
		}
		if _, ok := SonarMuteSlugOf(SettingKey(key)); ok {
			muted, ok := parseSwitch(value)
			if !ok {
				return Setting{}, fmt.Errorf("%q is not on or off", value)
			}
			return Setting{Key: SettingKey(key), Value: muted}, nil
		}
		if _, band, ok := SonarEQOf(SettingKey(key)); ok {
			if band == "preset" {
				preset, ok := EQPresetValue(value)
				if !ok {
					return Setting{}, fmt.Errorf("%q is not an equalizer preset", value)
				}
				return Setting{Key: SettingKey(key), Value: preset}, nil
			}
			level, err := strconv.ParseInt(value, 10, 32)
			if err != nil || level <= -EQBias || level >= EQBias {
				return Setting{}, fmt.Errorf("%q is not a band level", value)
			}
			return Setting{Key: SettingKey(key), Value: uint32(level + EQBias)}, nil
		}
		if _, ok := SonarAppNameOf(SettingKey(key)); ok {
			channel, ok := SonarChannelValue(value)
			if !ok {
				return Setting{}, fmt.Errorf("%q is not a Sonar channel (expected one of: %s)",
					value, strings.Join(sonarChannelSlugs, ", "))
			}
			return Setting{Key: SettingKey(key), Value: channel}, nil
		}
		if _, ok := EQBandSlugOf(SettingKey(key)); ok {
			// Levels are signed, and a Setting is not. Which bands exist is
			// the device's to say, and is checked against a read.
			level, err := strconv.ParseInt(value, 10, 32)
			if err != nil || level <= -EQBias || level >= EQBias {
				return Setting{}, fmt.Errorf("%q is not a band level", value)
			}
			return Setting{Key: SettingKey(key), Value: uint32(level + EQBias)}, nil
		}
		return Setting{}, fmt.Errorf("unknown setting %q (expected one of: dpi, "+
			"polling-rate, profile-mode, smart-shift-mode, smart-shift-threshold, "+
			"wheel-hi-res, wheel-invert, host, thumbwheel, button-<name>, "+
			"noise-mode, ambient-level, focus-on-voice, eq-preset, eq-<band>, codec, auto-power-off, dsee, speak-to-chat, touch-panel, "+
			"sidetone, mic-volume, mute-light, gain, wireless-mode, sonar, sonar-app-<app>, sonar-volume-<channel>, sonar-mute-<channel>, sonar-eq-<channel>-{preset,<band>}, "+
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

// Verifiable reports whether a write can be checked by reading the device
// back. Every setting here can, bar one.
//
// Switching Easy-Switch host is a write whose whole effect is that the device
// stops talking to this machine. There is nothing left to read back: the
// verifying read finds a device that is gone, which is exactly what a failed
// write looks like. Treating it like the rest would report every successful
// host switch as a failure, so the caller stops at "the device accepted it".
func (k SettingKey) Verifiable() bool { return k != SettingHost }

// Display turns a setting's value back into the number a person would say.
// Only equalizer bands differ: they travel biased, because a Setting is
// unsigned and a band level is not.
func (k SettingKey) Display(value uint32) any {
	if _, ok := EQBandSlugOf(k); ok {
		return int(value) - EQBias
	}
	if _, band, ok := SonarEQOf(k); ok && band != "preset" {
		return int(value) - EQBias
	}
	return value
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

	case SettingHost:
		if s.Hosts != nil {
			return uint32(s.Hosts.Current), true
		}

	case SettingThumbwheel:
		if s.Thumbwheel != nil {
			if value, ok := ThumbwheelModeValue(s.Thumbwheel.Mode); ok {
				return value, true
			}
		}

	case SettingNoiseMode:
		if s.NoiseControl != nil {
			if value, ok := NoiseModeValue(s.NoiseControl.Mode); ok {
				return value, true
			}
		}

	case SettingAmbientLevel:
		if s.NoiseControl != nil {
			return uint32(s.NoiseControl.AmbientLevel), true
		}

	case SettingFocusOnVoice:
		if s.NoiseControl != nil && s.NoiseControl.FocusOnVoice != nil {
			return boolToValue(*s.NoiseControl.FocusOnVoice), true
		}

	case SettingCodec:
		if s.Codec != nil {
			return CodecValue(s.Codec.Current)
		}

	case SettingAutoPowerOff:
		if s.AutoPowerOff != nil {
			return AutoPowerOffValue(s.AutoPowerOff.Current)
		}

	case SettingDSEE:
		if s.DSEE != nil {
			return boolToValue(s.DSEE.On), true
		}

	case SettingSpeakToChat:
		if s.SpeakToChat != nil {
			return boolToValue(*s.SpeakToChat), true
		}

	case SettingTouchPanel:
		if s.TouchPanel != nil {
			return boolToValue(*s.TouchPanel), true
		}

	case SettingEQPreset:
		if s.Equalizer != nil {
			return EQPresetValue(s.Equalizer.Preset)
		}

	case SettingSidetone:
		if s.Sidetone != nil {
			return SidetoneValue(s.Sidetone.Current)
		}

	case SettingGain:
		if s.Gain != nil {
			return GainValue(s.Gain.Current)
		}

	case SettingWirelessMode:
		if s.WirelessMode != nil {
			return WirelessModeValue(s.WirelessMode.Current)
		}

	case SettingSonar:
		if s.Sonar != nil {
			return boolToValue(s.Sonar.Enabled), true
		}

	case SettingMicVolume:
		if s.MicVolume != nil {
			return uint32(s.MicVolume.Current), true
		}

	case SettingMuteLight:
		if s.MuteLight != nil {
			return uint32(s.MuteLight.Current), true
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

	if slug, ok := EQBandSlugOf(key); ok && s.Equalizer != nil {
		if band := s.Equalizer.Band(slug); band != nil {
			return uint32(band.Value + EQBias), true
		}
		return 0, false
	}

	if slug, ok := SonarMuteSlugOf(key); ok && s.Sonar != nil && s.Sonar.Live {
		for _, channel := range s.Sonar.Channels {
			if channel.Slug == slug {
				return boolToValue(channel.Muted), true
			}
		}
		return 0, false
	}

	if slug, ok := SonarVolumeSlugOf(key); ok && s.Sonar != nil && s.Sonar.Live {
		for _, channel := range s.Sonar.Channels {
			if channel.Slug == slug {
				return uint32(channel.Volume), true
			}
		}
		return 0, false
	}

	if channel, band, ok := SonarEQOf(key); ok && s.Sonar != nil {
		for _, c := range s.Sonar.Channels {
			if c.Slug != channel || c.Equalizer == nil {
				continue
			}
			if band == "preset" {
				return EQPresetValue(c.Equalizer.Preset)
			}
			if b := c.Equalizer.Band(band); b != nil {
				return uint32(b.Value + EQBias), true
			}
		}
		return 0, false
	}

	if name, ok := SonarAppNameOf(key); ok && s.Sonar != nil {
		for _, app := range s.Sonar.Apps {
			if app.Name == name {
				return SonarChannelValue(app.Channel)
			}
		}
		return 0, false
	}

	// Button keys are per-device rather than from a fixed list, so they are
	// resolved by name against what the device reported.
	if slug, ok := ButtonSlugOf(key); ok {
		for _, button := range s.Buttons {
			if button.Slug != slug {
				continue
			}
			target, ok := ButtonCID(button.MappedTo)
			if !ok {
				return 0, false
			}
			return uint32(target), true
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
	Icon         string       `json:"icon,omitempty"`
	Connection   Connection   `json:"connection"`
	USBLabel     string       `json:"usbLabel,omitempty"`
	Path         string       `json:"path"`
	// Conflicts are other programs holding this device's node. Never nil, so
	// it marshals as [] rather than null.
	Conflicts []Contender `json:"conflicts"`
	State     DeviceState `json:"state"`
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
		Icon:         d.Entry.Icon,
		Connection:   ConnectionOf(d.Node),
		USBLabel:     d.Entry.USBLabel,
		Path:         d.Path(),
		Conflicts:    []Contender{},
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

// BatteryLevelWord normalises the kernel's coarse capacity_level to the words
// this project already uses for a level.
func BatteryLevelWord(level string) string {
	switch strings.ToLower(level) {
	case "critical":
		return "critical"
	case "low":
		return "low"
	case "normal", "high":
		return "good"
	case "full":
		return "full"
	default:
		return ""
	}
}

// BatteryStatusWord normalises the kernel's power_supply status to the same
// words the HID++ read produces, so the UI cannot tell which source it got.
func BatteryStatusWord(status string) string {
	switch strings.ToLower(status) {
	case "charging":
		return "charging"
	case "full":
		return "full"
	case "discharging", "not charging":
		return "discharging"
	default:
		return "unknown"
	}
}

package logitech

import "github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"

// Logitech PRO X2 SUPERSTRIKE.
//
// The reference device for this project: it is the first row of the README's
// mouse table and the only model with HITS, Logitech's analog left/right click
// with configurable actuation and haptic feedback.
//
// Battery, DPI and polling rate come from the shared HID++ driver. Anything
// model-specific lives here.

// The mouse enumerates as a different product depending on how it is attached,
// so both ids are catalogued. Wired it binds to hid-generic rather than
// logitech-hidpp-device, and it keeps the same serial in HID_UNIQ — spelled
// differently, which is why serials are normalised before they become part of
// a device id.
const (
	// ProX2SuperstrikeWireless is the id when paired to a Lightspeed receiver.
	ProX2SuperstrikeWireless = 0x40BD
	// ProX2SuperstrikeWired is the id when plugged in over USB.
	ProX2SuperstrikeWired = 0xC0A8
)

// ProX2SuperstrikeHITS is HID++ feature 0x1B0C. Reading and writing it is the
// next piece of work on this model; until then the catalog entry claims the
// capability so the UI can show it as present-but-unavailable rather than
// pretending the mouse lacks it.
const ProX2SuperstrikeHITS = hidpp.FeatureHITS

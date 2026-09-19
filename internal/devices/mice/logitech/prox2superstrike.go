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

// ProX2SuperstrikeWireless is the product id when paired to a Lightspeed
// receiver, as reported by the kernel's logitech-hidpp-device node.
const ProX2SuperstrikeWireless = 0x40BD

// ProX2SuperstrikeHITS is HID++ feature 0x1B0C. Reading and writing it is the
// next piece of work on this model; until then the catalog entry claims the
// capability so the UI can show it as present-but-unavailable rather than
// pretending the mouse lacks it.
const ProX2SuperstrikeHITS = hidpp.FeatureHITS

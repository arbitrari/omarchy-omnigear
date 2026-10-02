package logitech

// Logitech MX Master 3, the generation before the 3S.
//
// Catalogued from a real device rather than from the 3S's entry, and it is
// not the same mouse underneath. Three differences are load-bearing:
//
//   - **Battery is the older 0x1000 feature**, not 0x1004. The driver already
//     falls back, so this costs nothing, but it is the reason the 3S's entry
//     could not simply be copied.
//   - **It has no 0x1815.** It can switch Easy-Switch host through 0x1814,
//     and cannot say which slots hold a pairing or what they are called. The
//     panel offers every slot on this model for that reason.
//   - **Its buttons are more restricted than the 3S's.** Back and forward
//     accept only left, right, back and forward; middle, gesture and wheel
//     mode accept all seven. The group masks are read off the device, so this
//     needs no special case — but it is why "remap forward to middle" works
//     on a 3S and is refused here.
//
// Sensor tops out at 4000 DPI where the 3S reaches 8000. No configurable
// report rate and no onboard profiles, same as the 3S.

// MXMaster3Unifying is the product id the kernel gives it when paired to a
// Unifying receiver it has expanded into per-device nodes.
const MXMaster3Unifying = 0x4082

// MXMaster3Name is what it calls itself over HID++, which is how it is
// matched when it sits behind a receiver the kernel left unexpanded. Note it
// is not "MX Master 3": the device says "Wireless Mouse" first.
const MXMaster3Name = "Wireless Mouse MX Master 3"

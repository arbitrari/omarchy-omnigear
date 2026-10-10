package logitech

// Logitech MX Master 3S.
//
// It reaches the machine three ways and looks different on each: paired to a
// Logi Bolt receiver it has no node of its own and is known only by the name
// it reports, while over Bluetooth it enumerates as an ordinary HID device
// with its own product id. Both routes are catalogued, so it is found either
// way.
//
// Of what this project models it exposes battery and DPI — through the older
// 0x2201 feature, not the 0x2202 the gaming mice use. It has no configurable
// report rate and no onboard profiles.

// MXMaster3SBluetooth is the product id when connected over Bluetooth.
const MXMaster3SBluetooth = 0xB034

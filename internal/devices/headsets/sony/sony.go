// Package sony catalogues Sony headsets and headphones.
//
// These are not HID devices. They are found through BlueZ, matched by the
// vendor and product ids in their Bluetooth Device ID record, and driven over
// Sony's own RFCOMM control protocol. See drivers/sony.
package sony

import (
	driver "github.com/arbitrari/omarchy-omnigear/internal/drivers/sony"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
)

const vendor = 0x054C

// Product ids, as BlueZ reports them in the device's modalias.
const (
	// WH1000XM3 was observed as usb:v054Cp0CD3d0452.
	WH1000XM3 = 0x0CD3
	// WH1000XM4 was observed as usb:v054Cp0D58d0301.
	WH1000XM4 = 0x0D58
)

// Entries are the models catalogued for this brand.
var Entries = []model.Entry{
	{
		Model:    "WH-1000XM3",
		Slug:     "wh-1000xm3",
		Brand:    model.Sony,
		Category: model.Headset,
		USB:      []model.USBID{{Vendor: vendor, Product: WH1000XM3}},
		// Partial: the headset also has DSEE HX, auto power off and a touch
		// panel switch, all of which answer but none of which are driven yet.
		Support: model.SupportPartial,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapNoiseControl,
			model.CapEqualizer,
			model.CapCodec,
		},
		Driver: driver.MDR,
	},
	{
		Model:    "WH-1000XM4",
		Slug:     "wh-1000xm4",
		Brand:    model.Sony,
		Category: model.Headset,
		USB:      []model.USBID{{Vendor: vendor, Product: WH1000XM4}},
		// The XM4 publishes the XM3's control service and answers every request
		// the driver makes with the same layout. Partial: like the XM3 it has
		// settings that are not driven yet, and it adds speak-to-chat.
		Support: model.SupportPartial,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapNoiseControl,
			model.CapEqualizer,
			model.CapCodec,
			model.CapAutoPowerOff,
			model.CapDSEE,
		},
		Driver: driver.MDRXM4,
	},
}

// Package steelseries catalogues SteelSeries headsets and headphones.
//
// A wireless Arctis is reached through its base station, a USB device of its
// own, and never directly: the headset's status and settings are whatever the
// base station says they are. See drivers/steelseries.
package steelseries

import (
	driver "github.com/arbitrari/omarchy-omnigear/internal/drivers/steelseries"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
)

const vendor = 0x1038

// Product ids, as the base station enumerates.
const (
	// NovaProWireless was observed as 1038:12E0.
	NovaProWireless = 0x12E0
)

// Entries are the models catalogued for this brand.
var Entries = []model.Entry{
	{
		Model:    "Arctis Nova Pro Wireless",
		Slug:     "arctis-nova-pro-wireless",
		Brand:    model.SteelSeries,
		Category: model.Headset,
		USB:      []model.USBID{{Vendor: vendor, Product: NovaProWireless}},
		// The equalizer is not driven yet.
		Support: model.SupportPartial,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapNoiseControl,
			model.CapGain,
			model.CapSidetone,
			model.CapMicVolume,
			model.CapMuteLight,
			model.CapWirelessMode,
			model.CapAutoPowerOff,
			model.CapSonar,
		},
		Driver: driver.NovaProWireless,
	},
}

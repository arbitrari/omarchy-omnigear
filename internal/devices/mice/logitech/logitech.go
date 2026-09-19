// Package logitech catalogues Logitech mice.
//
// Every model here speaks HID++ 2.0, so they all share the one HID++ driver.
// Adding a model is usually one entry in Entries: its name, its USB ids, and
// which capabilities it has.
//
// USB ids are only listed for models whose ids have actually been observed.
// A planned model carries none, which simply never matches hardware — better
// than guessing an id and binding a driver to the wrong mouse.
package logitech

import (
	driver "github.com/arbitrari/omarchy-omnigear/internal/drivers/logitech"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
)

const vendor = 0x046D

// Entries are the Logitech mice in the README's support table.
var Entries = []model.Entry{
	{
		Model:    "PRO X2 SUPERSTRIKE",
		Slug:     "pro-x2-superstrike",
		Brand:    model.Logitech,
		Category: model.Mouse,
		USB: []model.USBID{
			{Vendor: vendor, Product: ProX2SuperstrikeWireless},
			{Vendor: vendor, Product: ProX2SuperstrikeWired},
		},
		// Every capability this model declares is now driven, HITS included.
		Support: model.SupportFull,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapDPI,
			model.CapPollingRate,
			model.CapOnboardProfile,
			model.CapHITS,
		},
		Driver: driver.HIDPP,
	},

	// Catalogued, not yet driven.
	planned("PRO X2 SUPERLIGHT 2", "pro-x2-superlight-2"),
	planned("PRO X2 SUPERLIGHT", "pro-x2-superlight"),
	planned("PRO 2 LIGHTSPEED", "pro-2-lightspeed"),
	planned("G502 X / PLUS", "g502-x-plus"),
	planned("G309 LIGHTSPEED", "g309-lightspeed"),
	planned("G305", "g305"),
	planned("MX Master 4", "mx-master-4"),
	{
		Model:    "MX Master 3S",
		Slug:     "mx-master-3s",
		Brand:    model.Logitech,
		Category: model.Mouse,
		// Over Bluetooth it enumerates with its own product id. Through a Logi
		// Bolt receiver this kernel leaves unexpanded it has no id of its own,
		// and is identified by the name it reports over HID++ instead. Both
		// routes are listed so it is found either way.
		USB:   []model.USBID{{Vendor: vendor, Product: MXMaster3SBluetooth}},
		Names: []string{"MX Master 3S"},
		// Battery and DPI are what it exposes of what this project models. It
		// has more that is not modelled yet — SmartShift, gestures,
		// reprogrammable buttons — so support is partial rather than full.
		Support: model.SupportPartial,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapDPI,
			model.CapSmartShift,
			model.CapHiResWheel,
		},
		Driver: driver.HIDPP,
	},
	planned("MX Master 3", "mx-master-3"),
	planned("MX Master 2", "mx-master-2"),
	planned("MX Master", "mx-master"),
}

// planned describes a model that is catalogued and nothing more: no driver,
// no ids yet.
func planned(name, slug string) model.Entry {
	return model.Entry{
		Model:    name,
		Slug:     slug,
		Brand:    model.Logitech,
		Category: model.Mouse,
		Support:  model.SupportPlanned,
	}
}

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
		// A white shell with black left and right clicks.
		Icon: "mouse-two-tone",
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
		// Full: everything this mouse exposes that is a setting is driven.
		//
		// What is left of its 35 features is not configuration. Sixteen are
		// flagged hidden or engineering by the firmware itself. Of the rest,
		// 0x2250 XY_STATS and 0x2251 WHEEL_STATS are telemetry counters,
		// 0x1D4B only raises notifications, 0x00C3 is firmware update, and
		// the remainder are names and ids the transport already uses.
		//
		// It has no gesture feature at all — nothing in the 0x65xx range. The
		// gesture button is an ordinary divertable control (cid 0x00C3), and
		// gestures are something host software builds on top by diverting it,
		// which would need a daemon. This plugin is a short-lived CLI on a
		// timer and deliberately has nowhere to put one.
		Support: model.SupportFull,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapDPI,
			model.CapSmartShift,
			model.CapHiResWheel,
			model.CapThumbwheel,
			model.CapButtons,
			model.CapHost,
		},
		Driver: driver.HIDPP,
	},
	{
		Model:    "MX Master 3",
		Slug:     "mx-master-3",
		Brand:    model.Logitech,
		Category: model.Mouse,
		// Its own node when the kernel expands the receiver it is paired to,
		// and its reported name for when it does not.
		USB:   []model.USBID{{Vendor: vendor, Product: MXMaster3Unifying}},
		Names: []string{MXMaster3Name},
		// Full: everything it exposes that is a setting is driven. What is
		// left of its 35 features is 17 the firmware flags hidden, telemetry
		// counters (0x2250, 0x2251), notifications (0x1D4B), firmware update
		// (0x00C2), and names and ids the transport already uses.
		Support: model.SupportFull,
		Capabilities: []model.Capability{
			model.CapBattery,
			model.CapDPI,
			model.CapSmartShift,
			model.CapHiResWheel,
			model.CapThumbwheel,
			model.CapButtons,
			model.CapHost,
		},
		Driver: driver.HIDPP,
	},
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

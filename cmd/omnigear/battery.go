package main

import (
	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/discovery"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/bluez"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// batteryReading is one device's charge, as the kernel has it.
type batteryReading struct {
	ID string `json:"id"`
	// Percent is -1 for a headset BlueZ has no charge for.
	Percent int    `json:"percent"`
	Level   string `json:"level,omitempty"`
	Status  string `json:"status"`
	// Spare is a second battery charging in a base station, where there is
	// one.
	Spare *int `json:"spare,omitempty"`
	// Connected is set only by a source that knows the headset is there or
	// not, as a base station does. A Bluetooth headset is listed only while
	// connected, so it needs no field to say so.
	Connected *bool `json:"connected,omitempty"`
}

// cmdBattery reports charge without saying a word to any device.
//
// This exists because the ordinary read does the opposite. A full poll is
// sixty-odd HID++ round trips, every one of which makes a wireless mouse
// power its radio and reply — so a battery monitor on a twenty-second timer
// is a machine for preventing the battery from ever resting. The kernel
// already keeps this figure, fed by reports the device sends of its own
// accord, and reading it wakes nothing.
//
// A driver that can answer as cheaply is asked instead: a SteelSeries base
// station keeps its headset's charge itself, and asking it reaches nothing
// that sleeps.
//
// It covers only devices the kernel drives directly. One behind a receiver it
// did not expand has no power_supply, and identifying it at all needs HID++,
// so it is left out here and picked up by the occasional full read instead.
// Reporting nothing is the honest answer; a stale number would not be.
func cmdBattery() (reply, error) {
	readings := make([]batteryReading, 0, 4)

	// Devices whose driver reads its own battery cheaply, by id. Each is
	// listed whether or not the read worked, because this reply is also how
	// the bar learns which headsets are here; a base station missing from it
	// would have the bar re-reading everything on every tick.
	var readerOrder []string
	readerFound := map[string]*batteryReading{}

	for _, node := range hidraw.Enumerate() {
		entry := catalog.FindByUSB(node.Vendor, node.Product)
		if entry == nil {
			continue
		}
		if reader, ok := entry.Driver.(model.BatteryReader); ok {
			id := discovery.IDFor(entry, node.Uniq)
			if _, seen := readerFound[id]; !seen {
				readerOrder = append(readerOrder, id)
				readerFound[id] = nil
			}
			if reading, ok := reader.Battery(node); ok {
				readerFound[id] = fromDriver(id, reading)
			}
			continue
		}
		supply, ok := node.Battery()
		if !ok {
			continue
		}
		readings = append(readings, batteryReading{
			ID:      discovery.IDFor(entry, node.Uniq),
			Percent: supply.Percent,
			Level:   model.BatteryLevelWord(supply.Level),
			Status:  model.BatteryStatusWord(supply.Status),
		})
	}

	for _, id := range readerOrder {
		if reading := readerFound[id]; reading != nil {
			readings = append(readings, *reading)
		} else {
			readings = append(readings, batteryReading{ID: id, Percent: -1, Status: "unknown"})
		}
	}

	// Bluetooth headsets are not in hidraw. BlueZ keeps their charge instead,
	// from what the headset reports over the hands-free link of its own
	// accord.
	//
	// Every connected one is listed, with or without a charge, because this
	// reply doubles as the list of which headsets are here at all. Headphones
	// come and go far more often than a mouse, and the bar learns of an
	// arrival or a departure from this rather than waiting on a full read.
	for _, device := range bluez.Connected() {
		entry := catalog.FindByUSB(device.Vendor, device.Product)
		if entry == nil || entry.Category != model.Headset {
			continue
		}
		percent := -1
		if device.Battery != nil {
			percent = *device.Battery
		}
		readings = append(readings, batteryReading{
			ID:      discovery.IDFor(entry, device.Address),
			Percent: percent,
			// BlueZ's Battery1 has a percentage and nothing about charging.
			Status: "unknown",
		})
	}

	return reply{"ok": true, "schema": schema, "batteries": readings}, nil
}

// fromDriver turns a driver's own battery reading into the reply's shape.
func fromDriver(id string, reading model.BatteryReading) *batteryReading {
	out := &batteryReading{ID: id, Percent: -1, Status: "unknown", Connected: &reading.Connected}
	if battery := reading.Battery; battery != nil {
		if battery.Percent != nil {
			out.Percent = *battery.Percent
		}
		out.Level = battery.Level
		out.Status = battery.Status
		out.Spare = battery.Spare
	}
	return out
}

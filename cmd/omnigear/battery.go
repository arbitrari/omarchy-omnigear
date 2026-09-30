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
	ID      string `json:"id"`
	Percent int    `json:"percent"`
	Level   string `json:"level,omitempty"`
	Status  string `json:"status"`
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
// It covers only devices the kernel drives directly. One behind a receiver it
// did not expand has no power_supply, and identifying it at all needs HID++,
// so it is left out here and picked up by the occasional full read instead.
// Reporting nothing is the honest answer; a stale number would not be.
func cmdBattery() (reply, error) {
	readings := make([]batteryReading, 0, 4)

	for _, node := range hidraw.Enumerate() {
		entry := catalog.FindByUSB(node.Vendor, node.Product)
		if entry == nil {
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

	// Headsets are not in hidraw. BlueZ keeps their charge instead, from what
	// the headset reports over the hands-free link of its own accord.
	for _, device := range bluez.Connected() {
		entry := catalog.FindByUSB(device.Vendor, device.Product)
		if entry == nil || entry.Category != model.Headset || device.Battery == nil {
			continue
		}
		readings = append(readings, batteryReading{
			ID:      discovery.IDFor(entry, device.Address),
			Percent: *device.Battery,
			// BlueZ's Battery1 has a percentage and nothing about charging.
			Status: "unknown",
		})
	}

	return reply{"ok": true, "schema": schema, "batteries": readings}, nil
}

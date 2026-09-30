// Package bluez lists the Bluetooth devices BlueZ has connected, over D-Bus.
//
// Headphones are not HID devices, so they never appear under hidraw: a pair of
// WH-1000XM3 is an audio sink, a headset profile and a vendor RFCOMM service,
// and nothing in /dev names it. BlueZ is what knows it is there, and it already
// knows the two things discovery needs without anyone talking to the device —
// the vendor and product ids from its Device ID record, and the battery level
// the headset reports over the hands-free link.
package bluez

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

// Device is one Bluetooth device BlueZ currently has connected.
type Device struct {
	// Address is the MAC as BlueZ prints it: 38:18:4C:6D:BA:69.
	Address string
	Name    string
	// Vendor and Product come from the device's PnP Information record, which
	// BlueZ exposes as a modalias. Zero when the device publishes none.
	Vendor  uint16
	Product uint16
	// Battery is BlueZ's Battery1 percentage, or nil when it has none. For a
	// headset this arrives over the hands-free link on the device's own
	// initiative, so reading it wakes nothing.
	Battery *int
	// UUIDs are the services the device advertised, lower case.
	UUIDs []string
}

// Offers reports whether the device advertised a service.
func (d Device) Offers(uuid string) bool {
	for _, u := range d.UUIDs {
		if strings.EqualFold(u, uuid) {
			return true
		}
	}
	return false
}

// Connected returns every device BlueZ currently has connected. An error from
// D-Bus — no system bus, no bluetoothd — is an empty list rather than a
// failure: a machine without Bluetooth simply has no Bluetooth devices.
func Connected() []Device {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil
	}
	defer conn.Close()

	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err = conn.Object("org.bluez", "/").
		Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).
		Store(&objects)
	if err != nil {
		return nil
	}

	var found []Device
	for _, interfaces := range objects {
		props, ok := interfaces["org.bluez.Device1"]
		if !ok {
			continue
		}
		if connected, _ := props["Connected"].Value().(bool); !connected {
			continue
		}

		device := Device{}
		device.Address, _ = props["Address"].Value().(string)
		device.Name, _ = props["Name"].Value().(string)
		if alias, ok := props["Alias"].Value().(string); ok && device.Name == "" {
			device.Name = alias
		}
		if modalias, ok := props["Modalias"].Value().(string); ok {
			device.Vendor, device.Product, _ = parseModalias(modalias)
		}
		if uuids, ok := props["UUIDs"].Value().([]string); ok {
			for _, u := range uuids {
				device.UUIDs = append(device.UUIDs, strings.ToLower(u))
			}
		}
		if battery, ok := interfaces["org.bluez.Battery1"]; ok {
			if percent, ok := battery["Percentage"].Value().(byte); ok {
				p := int(percent)
				device.Battery = &p
			}
		}
		found = append(found, device)
	}
	return found
}

// parseModalias reads vendor and product out of "usb:v054Cp0CD3d0452" or
// "bluetooth:v…". The source prefix says which registry the vendor id is
// from; both are 16-bit and the catalog matches on the pair regardless.
func parseModalias(modalias string) (vendor, product uint16, err error) {
	_, ids, found := strings.Cut(modalias, ":")
	if !found || len(ids) < 10 || ids[0] != 'v' || ids[5] != 'p' {
		return 0, 0, fmt.Errorf("unrecognised modalias %q", modalias)
	}
	v, err := strconv.ParseUint(ids[1:5], 16, 16)
	if err != nil {
		return 0, 0, err
	}
	p, err := strconv.ParseUint(ids[6:10], 16, 16)
	if err != nil {
		return 0, 0, err
	}
	return uint16(v), uint16(p), nil
}

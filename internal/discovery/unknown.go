package discovery

import (
	"fmt"
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/drivers/logitech"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// Unknown finds devices that speak a protocol this project understands but
// are not in the catalog, and describes them from what they say about
// themselves.
//
// Why only HID++ devices, when a user with an unsupported mouse of any brand
// would like to see it here: because nothing else can be identified without
// guessing, and guessing wrong is worse than saying nothing.
//
//   - The HID report descriptor is not the answer. A Keychron Q3 keyboard
//     declares a mouse collection, because it has mouse-keys, and would be
//     offered as an unsupported mouse.
//   - The kernel's input capabilities are not the answer either, for the same
//     reason: that keyboard publishes an input device named "… Mouse" with
//     REL_X, REL_Y and BTN_LEFT.
//   - And both miss the opposite case. The node an MX Master 3S is actually
//     reached through, behind an unexpanded Bolt receiver, declares no usages
//     and publishes no input device at all.
//
// A HID++ device, by contrast, will say its own name and list its own
// features, which is evidence rather than inference. Anything else is left to
// `omnigear report`, which describes every node on the machine without
// pretending to know what any of them is.
func Unknown() []model.Device {
	claimed := map[string]bool{}
	for _, device := range Devices() {
		claimed[device.Node.Path] = true
	}

	var found []model.Device
	for _, node := range hidraw.Enumerate() {
		if claimed[node.Path] || !hidpp.Speaks(node) {
			continue
		}
		// A receiver is not a device. The things behind it are, and they are
		// reached by index below rather than as the dongle itself.
		if _, isReceiver := hidraw.ReceiverKindOf(node.Vendor, node.Product); isReceiver {
			found = append(found, behindUnknownReceiver(node)...)
			continue
		}
		if catalog.FindByUSB(node.Vendor, node.Product) != nil {
			continue
		}
		if device, ok := describe(node, 0); ok {
			found = append(found, device)
		}
	}
	return found
}

// behindUnknownReceiver describes the paired devices on a receiver that the
// catalog does not account for. A receiver the kernel expanded is skipped for
// the same reason as in behindReceivers: its devices already have nodes of
// their own and would be found twice.
func behindUnknownReceiver(node hidraw.Node) []model.Device {
	if expandedByKernel(node) {
		return nil
	}

	var found []model.Device
	for _, paired := range hidpp.PairedDevices(node) {
		if catalog.FindByName(paired.Name) != nil {
			continue
		}
		if device, ok := describe(node, paired.Index); ok {
			found = append(found, device)
		}
	}
	return found
}

// describe builds a catalog entry for a device out of its own answers: the
// name it reports, and the capabilities implied by the features it lists.
func describe(node hidraw.Node, index byte) (model.Device, bool) {
	var link *hidpp.Device
	var err error
	if index != 0 {
		link, err = hidpp.OpenAt(node, index)
	} else {
		link, err = hidpp.Open(node)
	}
	if err != nil {
		return model.Device{}, false
	}
	defer link.Close()

	name, err := link.Name()
	if err != nil || strings.TrimSpace(name) == "" {
		// Without a name there is nothing to show a user and nothing to put in
		// a bug report that the node's USB id does not already say.
		return model.Device{}, false
	}

	features, err := link.Features()
	if err != nil {
		return model.Device{}, false
	}

	entry := &model.Entry{
		Model:        name,
		Slug:         slugify(name),
		Brand:        brandOf(node.Vendor),
		Category:     categoryOf(link),
		Support:      model.SupportUnsupported,
		Capabilities: logitech.CapabilitiesOf(features),
		Driver:       logitech.HIDPP,
		Discovered:   true,
		USBLabel:     fmt.Sprintf("%04X:%04X", node.Vendor, node.Product),
	}

	return model.Device{
		Entry: entry,
		ID:    deviceID(entry, node.Uniq),
		Node:  node,
		Index: index,
	}, true
}

// categoryOf asks the device what kind of thing it is, so an unsupported
// mouse still gets drawn as a mouse.
//
// A device that will not say, or says something this project has no drawing
// for, is left Unknown rather than assumed to be a mouse: the plain category
// icon is a better answer than a confidently wrong one.
func categoryOf(link *hidpp.Device) model.Category {
	kind, err := link.DeviceType()
	if err != nil {
		return model.Unknown
	}
	switch kind {
	case hidpp.DeviceTypeMouse, hidpp.DeviceTypeTrackball, hidpp.DeviceTypeTouchpad:
		return model.Mouse
	case hidpp.DeviceTypeKeyboard, hidpp.DeviceTypeNumpad:
		return model.Keyboard
	case hidpp.DeviceTypeHeadset:
		return model.Headset
	default:
		return model.Unknown
	}
}

// brandOf names the maker from the USB vendor id. Only vendors this project
// has a driver for can turn up here at all, so the list is short by
// construction.
func brandOf(vendor uint16) model.Brand {
	if vendor == 0x046D {
		return model.Logitech
	}
	return model.Brand("unknown")
}

// slugify turns a reported name into something usable as a device id and a
// CLI argument. The name comes off the wire, so it is reduced to characters
// that cannot change how an id parses.
func slugify(name string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			sb.WriteRune(r)
			dash = false
		case !dash && sb.Len() > 0:
			sb.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(sb.String(), "-")
}

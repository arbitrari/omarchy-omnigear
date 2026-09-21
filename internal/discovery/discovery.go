// Package discovery matches what is plugged in against what is catalogued.
package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// Devices returns every catalogued device currently present.
//
// One physical device owns several hidraw nodes, and can own them under more
// than one USB id at once: plug a wireless mouse in with a cable while its
// dongle is still in the machine and both sets exist side by side, with only
// one of them actually carrying the mouse. Nodes are therefore grouped by the
// device's serial and one is chosen per group.
func Devices() []model.Device {
	var order []string
	groups := map[string][]hidraw.Node{}
	entries := map[string]*model.Entry{}

	for _, node := range hidraw.Enumerate() {
		entry := catalog.FindByUSB(node.Vendor, node.Product)
		if entry == nil {
			continue
		}

		id := deviceID(entry, node.Uniq)
		if _, seen := groups[id]; !seen {
			order = append(order, id)
			entries[id] = entry
		}
		groups[id] = append(groups[id], node)
	}

	found := make([]model.Device, 0, len(order))
	for _, id := range order {
		found = append(found, model.Device{
			Entry: entries[id],
			ID:    id,
			Node:  choose(groups[id]),
		})
	}

	return append(found, behindReceivers()...)
}

// behindReceivers finds devices that have no hidraw node of their own.
//
// When the kernel recognises a dongle it expands it into one node per paired
// device, and those are matched by USB id like anything else. When it does not
// — a Logi Bolt on a kernel whose hid-logitech-dj has no entry for its product
// id — the dongle stays a single node and the devices behind it are invisible
// to a USB-id match. They are reachable only by index on the receiver's own
// node, and identify themselves by name rather than by id.
//
// Only unexpanded receivers are scanned. Scanning one the kernel already
// expanded would find the same device twice, once under each identity.
func behindReceivers() []model.Device {
	var found []model.Device

	for _, node := range hidraw.Enumerate() {
		if _, isReceiver := hidraw.ReceiverKindOf(node.Vendor, node.Product); !isReceiver {
			continue
		}
		if expandedByKernel(node) || !hidpp.Speaks(node) {
			continue
		}

		for _, paired := range hidpp.PairedDevices(node) {
			entry := catalog.FindByName(paired.Name)
			if entry == nil {
				continue
			}
			found = append(found, model.Device{
				Entry: entry,
				ID:    deviceID(entry, node.Uniq),
				Node:  node,
				Index: paired.Index,
			})
		}
	}

	return found
}

// expandedByKernel reports whether the kernel has already turned a receiver's
// paired devices into hidraw nodes of their own.
func expandedByKernel(node hidraw.Node) bool {
	return strings.Contains(node.Driver, "djreceiver")
}

// IDFor exposes the device id so a caller that never opens the device can
// still name it the same way a full read does. The cheap battery path needs
// exactly this: the same id, arrived at from sysfs alone.
func IDFor(entry *model.Entry, uniq string) string { return deviceID(entry, uniq) }

// deviceID is "<category>/<brand>/<slug>", plus "#<serial>" when the kernel
// knows one.
func deviceID(entry *model.Entry, uniq string) string {
	base := fmt.Sprintf("%s/%s/%s", entry.Category, entry.Brand, entry.Slug)
	serial := normalizeSerial(uniq)
	if serial == "" {
		return base
	}
	return base + "#" + serial
}

// normalizeSerial reduces a HID_UNIQ to its bare characters.
//
// The same mouse spells its serial differently depending on how it is
// attached: "5f-ba-c9-65" through its dongle, "5FBAC965" over USB. Those are
// one device, and a device id that changed when a cable was plugged in would
// split it in two.
func normalizeSerial(uniq string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(uniq) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// choose picks the node to talk to out of one device's nodes.
//
// Two filters, cheapest first. The report descriptor rules out the nodes that
// do not carry the protocol at all — a wired PRO X2 SUPERSTRIKE owns four and
// only one of them does. If more than one survives, the device is reachable
// through at most one of them, and only a ping says which: the dongle's node
// stays behind when the mouse moves to USB, looking entirely plausible and
// answering nothing.
//
// There is one protocol today, so this asks hidpp directly. A second transport
// would make the question a per-driver one.
func choose(nodes []hidraw.Node) hidraw.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}

	speaking := make([]hidraw.Node, 0, len(nodes))
	for _, node := range nodes {
		if hidpp.Speaks(node) {
			speaking = append(speaking, node)
		}
	}
	if len(speaking) == 0 {
		// Nothing declares the protocol. Hand back the first node so the
		// driver's own failure is what gets reported, rather than inventing a
		// reason here.
		return nodes[0]
	}
	if len(speaking) == 1 {
		return speaking[0]
	}

	// Ask them all at once and take the first answer: a live device replies in
	// milliseconds, while a stale node costs the whole budget, and there is no
	// reason to spend that serially.
	//
	// Two passes. The quick one settles the common case without making every
	// poll wait on a dead node. Only if nothing at all answers is it worth the
	// wake budget, because then the device may simply be asleep rather than
	// gone.
	if node, ok := firstToAnswer(speaking, hidpp.ProbeTimeout); ok {
		return node
	}
	if node, ok := firstToAnswer(speaking, hidpp.WakeTimeout); ok {
		return node
	}

	// None answered. Keep the first so the caller sees a connect failure.
	return speaking[0]
}

func firstToAnswer(nodes []hidraw.Node, within time.Duration) (hidraw.Node, bool) {
	type answer struct {
		node hidraw.Node
		ok   bool
	}
	replies := make(chan answer, len(nodes))
	for _, node := range nodes {
		go func(n hidraw.Node) {
			replies <- answer{node: n, ok: hidpp.Responds(n, within)}
		}(node)
	}
	for range nodes {
		if reply := <-replies; reply.ok {
			return reply.node, true
		}
	}
	return hidraw.Node{}, false
}

// Package discovery matches what is plugged in against what is catalogued.
package discovery

import (
	"fmt"
	"strings"

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
	return found
}

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
	// milliseconds, while a stale node costs the whole probe budget, and there
	// is no reason to spend that serially.
	type answer struct {
		node hidraw.Node
		ok   bool
	}
	replies := make(chan answer, len(speaking))
	for _, node := range speaking {
		go func(n hidraw.Node) {
			replies <- answer{node: n, ok: hidpp.Responds(n)}
		}(node)
	}
	for range speaking {
		if reply := <-replies; reply.ok {
			return reply.node
		}
	}

	// None answered. Keep the first so the caller sees a connect failure.
	return speaking[0]
}

// Package discovery matches what is plugged in against what is catalogued.
package discovery

import (
	"fmt"
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// Devices returns every catalogued device currently present.
func Devices() []model.Device {
	var found []model.Device

	for _, node := range hidraw.Enumerate() {
		entry := catalog.FindByUSB(node.Vendor, node.Product)
		if entry == nil {
			continue
		}

		id := deviceID(entry, node.Uniq)

		// One physical device usually owns several hidraw nodes. Keep the
		// first one seen, unless a later node is the better-bound one — the
		// kernel's HID++ node speaks the protocol, its siblings are plain
		// input endpoints.
		existing := -1
		for i := range found {
			if found[i].ID == id {
				existing = i
				break
			}
		}
		if existing >= 0 {
			if prefers(node, found[existing].Node) {
				found[existing].Node = node
			}
			continue
		}

		found = append(found, model.Device{Entry: entry, ID: id, Node: node})
	}

	return found
}

// deviceID is "<category>/<brand>/<slug>", plus "#<serial>" when the kernel
// knows one.
func deviceID(entry *model.Entry, uniq string) string {
	base := fmt.Sprintf("%s/%s/%s", entry.Category, entry.Brand, entry.Slug)
	if uniq == "" {
		return base
	}
	return base + "#" + uniq
}

// prefers reports whether candidate is a better node to talk to than current.
func prefers(candidate, current hidraw.Node) bool {
	return rank(candidate) > rank(current)
}

func rank(node hidraw.Node) int {
	if strings.Contains(node.Driver, "hidpp") {
		return 1
	}
	return 0
}

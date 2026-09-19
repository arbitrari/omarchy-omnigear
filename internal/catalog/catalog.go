// Package catalog looks things up in the device catalog.
package catalog

import (
	"fmt"
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/devices"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
)

// entries is built once so pointers handed out by FindByUSB stay valid.
var entries = devices.All()

// All returns every catalogued model.
func All() []model.Entry { return entries }

// FindByUSB returns the entry for a USB id, or nil if no model claims it.
func FindByUSB(vendor, product uint16) *model.Entry {
	for i := range entries {
		if entries[i].Matches(vendor, product) {
			return &entries[i]
		}
	}
	return nil
}

// Resolve matches a user-typed selector against discovered devices.
//
// It accepts the full id (mouse/logitech/pro-x2-superstrike#5f-ba-c9-65), the
// id without its serial, or a bare model slug — as long as exactly one device
// matches.
func Resolve(found []model.Device, selector string) (*model.Device, error) {
	for i := range found {
		if found[i].ID == selector {
			return &found[i], nil
		}
	}

	var matches []*model.Device
	for i := range found {
		if matchesLoosely(found[i].ID, selector) {
			matches = append(matches, &found[i])
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no connected device matches %q", selector)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, fmt.Errorf("%q matches more than one device: %s", selector, strings.Join(ids, ", "))
	}
}

func matchesLoosely(id, selector string) bool {
	if strings.HasPrefix(id, selector) {
		return true
	}
	withoutSerial, _, _ := strings.Cut(id, "#")
	if withoutSerial == selector {
		return true
	}
	if slash := strings.LastIndex(withoutSerial, "/"); slash >= 0 {
		return withoutSerial[slash+1:] == selector
	}
	return false
}

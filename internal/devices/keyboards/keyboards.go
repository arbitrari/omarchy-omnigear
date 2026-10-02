// Package keyboards collects keyboards, by brand.
package keyboards

import (
	"github.com/arbitrari/omarchy-omnigear/internal/model"

	"github.com/arbitrari/omarchy-omnigear/internal/devices/keyboards/keychron"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/keyboards/logitech"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/keyboards/razer"
)

// Entries is every model catalogued for this category.
func Entries() []model.Entry {
	var all []model.Entry
	for _, brand := range [][]model.Entry{
		logitech.Entries,
		razer.Entries,
		keychron.Entries,
	} {
		all = append(all, brand...)
	}
	return all
}

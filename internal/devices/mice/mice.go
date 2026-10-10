// Package mice collects mice, by brand.
package mice

import (
	"github.com/arbitrari/omarchy-omnigear/internal/model"

	"github.com/arbitrari/omarchy-omnigear/internal/devices/mice/corsair"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/mice/finalmouse"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/mice/glorious"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/mice/logitech"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/mice/razer"
)

// Entries is every model catalogued for this category.
func Entries() []model.Entry {
	var all []model.Entry
	for _, brand := range [][]model.Entry{
		logitech.Entries,
		razer.Entries,
		corsair.Entries,
		glorious.Entries,
		finalmouse.Entries,
	} {
		all = append(all, brand...)
	}
	return all
}

// Package headsets collects headsets and headphones, by brand.
package headsets

import (
	"github.com/arbitrari/omarchy-omnigear/internal/model"

	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/apple"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/google"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/hyperx"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/logitech"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/nothing"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/oneplus"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/razer"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/sony"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets/steelseries"
)

// Entries is every model catalogued for this category.
func Entries() []model.Entry {
	var all []model.Entry
	for _, brand := range [][]model.Entry{
		steelseries.Entries,
		logitech.Entries,
		razer.Entries,
		hyperx.Entries,
		sony.Entries,
		apple.Entries,
		google.Entries,
		nothing.Entries,
		oneplus.Entries,
	} {
		all = append(all, brand...)
	}
	return all
}

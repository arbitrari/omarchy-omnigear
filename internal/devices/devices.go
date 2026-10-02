// Package devices is the device catalog, laid out the way the README reads:
// category, then brand, then model.
//
//	devices/
//	  mice/
//	    logitech/            every Logitech mouse, as catalog entries
//	      prox2superstrike.go  what is specific to this one model
//	    razer/  corsair/  glorious/  finalmouse/
//	  keyboards/
//	    logitech/  razer/  keychron/
//	  headsets/
//	    steelseries/  logitech/  razer/  hyperx/
//	    sony/  apple/  google/  nothing/  oneplus/
//
// To find the code for a device, walk that path. To add one, add an Entry to
// its brand package — a model whose family already has a driver needs nothing
// else.
package devices

import (
	"github.com/arbitrari/omarchy-omnigear/internal/devices/headsets"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/keyboards"
	"github.com/arbitrari/omarchy-omnigear/internal/devices/mice"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
)

// All returns every catalogued model, supported or merely planned.
func All() []model.Entry {
	var all []model.Entry
	all = append(all, mice.Entries()...)
	all = append(all, keyboards.Entries()...)
	all = append(all, headsets.Entries()...)
	return all
}

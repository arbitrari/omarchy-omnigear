// Package razer catalogues Razer mice.
//
// Catalogued from the README's support table. None are driven yet, so none
// carry USB ids — an entry gains its ids when it gains a driver.
package razer

import "github.com/arbitrari/omarchy-omnigear/internal/model"

// Entries are the Razer mice in the README's support table.
var Entries = []model.Entry{
	planned("DeathAdder V4 Pro", "deathadder-v4-pro"),
	planned("DeathAdder V3 Pro", "deathadder-v3-pro"),
	planned("DeathAdder V3 HS", "deathadder-v3-hs"),
	planned("DeathAdder V3", "deathadder-v3"),
	planned("DeathAdder V2 X HS", "deathadder-v2-x-hs"),
	planned("Viper V4 Pro", "viper-v4-pro"),
	planned("Viper V3 Pro SE", "viper-v3-pro-se"),
	planned("Viper V3 Pro", "viper-v3-pro"),
	planned("Naga V3 Pro", "naga-v3-pro"),
	planned("Naga V2 Pro", "naga-v2-pro"),
	planned("Naga V2 HS", "naga-v2-hs"),
	planned("Basilisk V3 Pro", "basilisk-v3-pro"),
}

// planned describes a model that is catalogued and nothing more: no driver,
// no ids yet.
func planned(name, slug string) model.Entry {
	return model.Entry{
		Model:    name,
		Slug:     slug,
		Brand:    model.Razer,
		Category: model.Mouse,
		Support:  model.SupportPlanned,
	}
}

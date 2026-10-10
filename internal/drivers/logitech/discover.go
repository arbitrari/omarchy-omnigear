package logitech

import (
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
)

// CapabilitiesOf works out what an uncatalogued device can do by asking it
// which HID++ features it implements.
//
// For a catalogued model the capability list is written down, because a
// feature being present is not the same as it being tested: the list says
// "someone confirmed this works on this model". Here there is nobody to say
// that, so the device's own feature table is the only evidence available and
// the answer is marked unsupported to say exactly that.
//
// Only features this driver already knows how to read are offered. A device
// implementing something nothing here can decode gains nothing by having it
// listed.
func CapabilitiesOf(features []hidpp.Feature) []model.Capability {
	has := make(map[uint16]bool, len(features))
	for _, feature := range features {
		// A feature the firmware itself flags as hidden or engineering is not
		// something to drive on a device nobody has tested.
		if feature.Hidden() {
			continue
		}
		has[feature.ID] = true
	}

	// Ordered deliberately: this is the order the panel renders controls in,
	// and a discovered device should look like a catalogued one.
	candidates := []struct {
		capability model.Capability
		features   []uint16
	}{
		{model.CapBattery, []uint16{hidpp.FeatureUnifiedBattery, hidpp.FeatureBatteryStatus}},
		{model.CapDPI, []uint16{hidpp.FeatureExtendedAdjustDPI, hidpp.FeatureAdjustableDPI}},
		{model.CapPollingRate, []uint16{hidpp.FeatureExtendedReportRate, hidpp.FeatureReportRate}},
		{model.CapSmartShift, []uint16{hidpp.FeatureSmartShift}},
		{model.CapHiResWheel, []uint16{hidpp.FeatureHiResWheel}},
		{model.CapThumbwheel, []uint16{hidpp.FeatureThumbwheel}},
		{model.CapButtons, []uint16{hidpp.FeatureReprogrammableKeys}},
		{model.CapHost, []uint16{hidpp.FeatureChangeHost}},
		{model.CapOnboardProfile, []uint16{hidpp.FeatureOnboardProfiles}},
	}

	var capabilities []model.Capability
	for _, candidate := range candidates {
		for _, id := range candidate.features {
			if has[id] {
				capabilities = append(capabilities, candidate.capability)
				break
			}
		}
	}
	return capabilities
}

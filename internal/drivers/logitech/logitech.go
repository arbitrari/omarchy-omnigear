// Package logitech is the HID++ driver.
//
// One driver for every Logitech peripheral that speaks HID++ 2.0 — the G mice,
// the MX line, and the Logitech keyboards. It reads only the capabilities the
// model's catalog entry claims, so a mouse without HITS never pays for the
// HITS probe.
//
// Where Logitech ships two generations of a feature (0x2201 vs 0x2202 for DPI,
// 0x8060 vs 0x8061 for report rate) the newer one is tried first and the older
// is the fallback.
//
// The wire formats below are what a PRO X2 SUPERSTRIKE actually returns; each
// decoder carries the bytes it was read off. These formats are not documented
// anywhere public and the bytes are the only proof.
package logitech

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidpp"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// HIDPP is the shared instance catalog entries point at.
var HIDPP model.Driver = driver{}

type driver struct{}

func (driver) Name() string { return "logitech-hidpp" }

// connect opens the conversation, addressing the device explicitly when it
// sits behind a receiver. Letting the transport search for whichever index
// answers would pick the wrong device when two are paired to one dongle.
func connect(device *model.Device) (*hidpp.Device, error) {
	if device.Index != 0 {
		return hidpp.OpenAt(device.Node, device.Index)
	}
	return hidpp.Open(device.Node)
}

func (driver) Read(device *model.Device) model.DeviceState {
	state := model.NewDeviceState()

	// A device the kernel already knows is gone is not worth the wake budget.
	// Spending 2.5s discovering that a switched-off mouse is switched off, on
	// every poll, is the difference between a snappy panel and a stalling
	// one — and the kernel's answer costs a sysfs read and does not wake
	// anything.
	if online, known := device.Node.LinkOnline(); known && !online {
		state.Connected = false
		state.Presence = model.PresenceOff
		return state
	}

	link, err := connect(device)
	if err != nil {
		state.Connected = false
		state.Presence = model.PresenceUnreachable
		// Silence is the ordinary way a switched-off device presents itself,
		// and "disconnected" says that better than an error would. Anything
		// else — a permission problem, a broken node — is worth reporting.
		if !errors.Is(err, hidpp.ErrTimeout) {
			state.Errors = append(state.Errors, fmt.Sprintf("connect: %v", err))
		}
		return state
	}
	defer link.Close()

	if link.Woken() {
		state.Presence = model.PresenceAsleep
	}

	if device.Entry.Has(model.CapBattery) {
		battery, err := readBattery(link)
		if err != nil {
			state.Fail(model.CapBattery, err)
		} else {
			state.Battery = battery
		}
	}
	if device.Entry.Has(model.CapDPI) {
		dpi, err := readDPI(link)
		if err != nil {
			state.Fail(model.CapDPI, err)
		} else {
			state.DPI = dpi
		}
	}
	if device.Entry.Has(model.CapSmartShift) {
		shift, err := readSmartShift(link)
		if err != nil {
			state.Fail(model.CapSmartShift, err)
		} else {
			state.SmartShift = shift
		}
	}
	if device.Entry.Has(model.CapHiResWheel) {
		wheel, err := readHiResWheel(link)
		if err != nil {
			state.Fail(model.CapHiResWheel, err)
		} else {
			state.HiResWheel = wheel
		}
	}
	if device.Entry.Has(model.CapOnboardProfile) {
		mode, err := readOnboardMode(link)
		if err != nil {
			state.Fail(model.CapOnboardProfile, err)
		} else {
			state.OnboardProfile = &mode
		}
	}
	if device.Entry.Has(model.CapButtons) {
		buttons, err := readButtons(link)
		if err != nil {
			state.Fail(model.CapButtons, err)
		} else {
			state.Buttons = buttons
		}
	}
	if device.Entry.Has(model.CapThumbwheel) {
		wheel, err := readThumbwheel(link)
		if err != nil {
			state.Fail(model.CapThumbwheel, err)
		} else {
			state.Thumbwheel = wheel
		}
	}
	if device.Entry.Has(model.CapHost) {
		hosts, err := readHosts(link)
		if err != nil {
			state.Fail(model.CapHost, err)
		} else {
			state.Hosts = hosts
		}
	}
	if device.Entry.Has(model.CapHITS) {
		hits, err := readHITS(link)
		if err != nil {
			state.Fail(model.CapHITS, err)
		} else {
			state.HITS = hits
		}
	}
	if device.Entry.Has(model.CapPollingRate) {
		rate, err := readPollingRate(link, device.Node)
		if err != nil {
			state.Fail(model.CapPollingRate, err)
		} else {
			state.PollingRate = rate
		}
	}

	return state
}

func (driver) Write(device *model.Device, setting *model.Setting) error {
	link, err := connect(device)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer link.Close()

	switch setting.Key {
	case model.SettingDPI:
		if err := writeDPI(link, setting.Value); err != nil {
			return fmt.Errorf("dpi: %w", err)
		}
	case model.SettingPollingRate:
		if err := writePollingRate(link, device.Node, setting.Value); err != nil {
			return fmt.Errorf("polling-rate: %w", err)
		}
	case model.SettingWheelHiRes, model.SettingWheelInvert:
		if err := writeHiResWheel(link, setting); err != nil {
			return fmt.Errorf("%s: %w", setting.Key, err)
		}
	case model.SettingSmartShiftMode, model.SettingSmartShiftThreshold:
		if err := writeSmartShift(link, setting); err != nil {
			return fmt.Errorf("%s: %w", setting.Key, err)
		}
	case model.SettingProfileMode:
		if err := writeOnboardMode(link, setting.Value); err != nil {
			return fmt.Errorf("profile-mode: %w", err)
		}
	case model.SettingThumbwheel:
		if err := writeThumbwheel(link, setting.Value); err != nil {
			return fmt.Errorf("thumbwheel: %w", err)
		}
	case model.SettingHost:
		if err := writeHost(link, setting.Value); err != nil {
			return fmt.Errorf("host: %w", err)
		}
	default:
		if _, ok := model.ButtonSlugOf(setting.Key); ok {
			if err := writeButton(link, setting); err != nil {
				return fmt.Errorf("%s: %w", setting.Key, err)
			}
			return nil
		}
		if err := writeHITSSetting(link, setting); err != nil {
			return fmt.Errorf("%s: %w", setting.Key, err)
		}
	}
	return nil
}

// --- battery ---------------------------------------------------------------

// readBattery uses feature 0x1004 function 1, which replies
// [stateOfCharge%, levelBits, chargingStatus, …].
func readBattery(link *hidpp.Device) (*model.Battery, error) {
	reply, err := link.CallFeature(hidpp.FeatureUnifiedBattery, 0x01)
	if err != nil {
		if hidpp.Unsupported(err) {
			return readBatteryLegacy(link)
		}
		return nil, err
	}

	battery := &model.Battery{Status: "unknown"}
	if len(reply) > 0 && reply[0] <= 100 {
		percent := int(reply[0])
		battery.Percent = &percent
	}
	if len(reply) > 1 {
		switch bits := reply[1]; {
		case bits&0x08 != 0:
			battery.Level = "full"
		case bits&0x04 != 0:
			battery.Level = "good"
		case bits&0x02 != 0:
			battery.Level = "low"
		case bits&0x01 != 0:
			battery.Level = "critical"
		}
	}
	if len(reply) > 2 {
		switch reply[2] {
		case 0:
			battery.Status = "discharging"
		case 1, 2:
			battery.Status = "charging"
		case 3:
			battery.Status = "full"
		}
	}
	return battery, nil
}

// readBatteryLegacy uses feature 0x1000 function 0, the pre-unified report:
// [level%, nextLevel%, status].
func readBatteryLegacy(link *hidpp.Device) (*model.Battery, error) {
	reply, err := link.CallFeature(hidpp.FeatureBatteryStatus, 0x00)
	if err != nil {
		return nil, err
	}

	battery := &model.Battery{Status: "unknown"}
	if len(reply) > 0 && reply[0] <= 100 {
		percent := int(reply[0])
		battery.Percent = &percent
	}
	if len(reply) > 2 {
		switch reply[2] {
		case 0:
			battery.Status = "discharging"
		case 1, 2, 3:
			battery.Status = "charging"
		case 4:
			battery.Status = "full"
		}
	}
	return battery, nil
}

// --- hits ------------------------------------------------------------------
//
// Feature 0x1B0C, the analog left and right click on a PRO X2 SUPERSTRIKE.
//
//	fn 0 getCapabilities  → [_, buttons, maxActuation, maxRapidTrigger,
//	                         maxHaptics, min]   e.g. 00 03 28 14 14 01
//	fn 2 getSettings(btn) → [btn, actuation, rapidTrigger, haptics]
//	                        e.g. 00 10 08 08 left, 01 14 08 08 right
//	fn 1 setSettings(btn, actuation, rapidTrigger, haptics), echoing what it
//	     applied
//
// The units are the device's own and it does not say what they mean; 40 steps
// of actuation across a click is all that can honestly be claimed.

const (
	hitsButtonLeft  = 0x00
	hitsButtonRight = 0x01
)

// hitsStep is the granularity every HITS field is stored at.
//
// Not reported by the device and not enforced by it either: ask a SUPERSTRIKE
// for 17, 18 or 19 and it stores 16 without complaint, and 20 for 20. Every
// value it ships with is a multiple of 4, and max/4 gives the level counts
// Logitech's own software offers — 10 for actuation, 5 each for rapid trigger
// and haptics. Requests are snapped to the grid so a change either lands or
// says why, instead of appearing to be ignored.
const hitsStep = 4

func readHITS(link *hidpp.Device) (*model.HITS, error) {
	index, err := link.FeatureIndex(hidpp.FeatureHITS)
	if err != nil {
		return nil, err
	}

	caps, err := link.Call(index, 0x00)
	if err != nil {
		return nil, err
	}
	left, err := readHITSButton(link, index, hitsButtonLeft)
	if err != nil {
		return nil, err
	}
	right, err := readHITSButton(link, index, hitsButtonRight)
	if err != nil {
		return nil, err
	}

	return &model.HITS{
		Left:            left,
		Right:           right,
		MaxActuation:    at(caps, 2),
		MaxRapidTrigger: at(caps, 3),
		MaxHaptics:      at(caps, 4),
		Step:            hitsStep,
	}, nil
}

func readHITSButton(link *hidpp.Device, index, button byte) (model.HITSButton, error) {
	reply, err := link.Call(index, 0x02, button)
	if err != nil {
		return model.HITSButton{}, err
	}
	return model.HITSButton{
		Actuation:    at(reply, 1),
		RapidTrigger: at(reply, 2),
		Haptics:      at(reply, 3),
	}, nil
}

// writeHITSSetting changes one field of one click.
//
// The device only takes all three fields at once, so the other two are read
// back and resent unchanged rather than assumed.
func writeHITSSetting(link *hidpp.Device, setting *model.Setting) error {
	button, field, ok := hitsTarget(setting.Key)
	if !ok {
		return fmt.Errorf("this driver cannot set %q", setting.Key)
	}

	index, err := link.FeatureIndex(hidpp.FeatureHITS)
	if err != nil {
		return err
	}
	caps, err := link.Call(index, 0x00)
	if err != nil {
		return err
	}
	current, err := readHITSButton(link, index, button)
	if err != nil {
		return err
	}

	value := setting.Value
	var effective uint8
	switch field {
	case "actuation":
		// A click that never actuates would be a button the user cannot undo
		// the setting with, so the floor is one step rather than zero.
		effective = snapHITS(value, hitsStep, at(caps, 2))
		current.Actuation = effective
	case "rapid-trigger":
		effective = snapHITS(value, hitsStep, at(caps, 3))
		current.RapidTrigger = effective
	default:
		// Haptics genuinely has an off.
		effective = snapHITS(value, 0, at(caps, 4))
		current.Haptics = effective
	}
	setting.Value = uint32(effective)

	_, err = link.Call(index, 0x01, button,
		current.Actuation, current.RapidTrigger, current.Haptics)
	return err
}

func hitsTarget(key model.SettingKey) (button byte, field string, ok bool) {
	rest, found := strings.CutPrefix(string(key), "hits-")
	if !found {
		return 0, "", false
	}
	side, field, found := strings.Cut(rest, "-")
	if !found {
		return 0, "", false
	}
	switch side {
	case "left":
		return hitsButtonLeft, field, true
	case "right":
		return hitsButtonRight, field, true
	default:
		return 0, "", false
	}
}

// snapHITS rounds a request to the nearest value the device can hold, then
// clamps it into range.
func snapHITS(value uint32, min, max uint8) uint8 {
	if max == 0 {
		max = 0xFF
	}
	snapped := ((value + hitsStep/2) / hitsStep) * hitsStep
	if snapped > uint32(max) {
		snapped = uint32(max)
	}
	if snapped < uint32(min) {
		snapped = uint32(min)
	}
	return uint8(snapped)
}

// --- hi-res wheel ----------------------------------------------------------
//
// Feature 0x2121, one byte of mode flags.
//
//	fn 1 getWheelMode  → [flags]
//	fn 2 setWheelMode(flags), echoing what it applied
//
//	bit 0  target      where wheel movement is sent
//	bit 1  resolution  0 low, 1 high
//	bit 2  invert      0 normal, 1 inverted
//
// Bit 0 is never touched. Setting it diverts wheel movement from ordinary HID
// scroll events to HID++ notifications, which no part of this project reads —
// the wheel would simply stop scrolling until something set it back.
const (
	wheelTargetBit     = 1 << 0
	wheelResolutionBit = 1 << 1
	wheelInvertBit     = 1 << 2
)

func readHiResWheel(link *hidpp.Device) (*model.HiResWheel, error) {
	reply, err := link.CallFeature(hidpp.FeatureHiResWheel, 0x01)
	if err != nil {
		return nil, err
	}
	flags := at(reply, 0)
	return &model.HiResWheel{
		HiRes:    flags&wheelResolutionBit != 0,
		Inverted: flags&wheelInvertBit != 0,
	}, nil
}

func writeHiResWheel(link *hidpp.Device, setting *model.Setting) error {
	index, err := link.FeatureIndex(hidpp.FeatureHiResWheel)
	if err != nil {
		return err
	}
	current, err := link.Call(index, 0x01)
	if err != nil {
		return err
	}

	// Read, modify, write — so the target bit is carried over exactly as
	// found rather than cleared by accident.
	flags := at(current, 0)
	bit := byte(wheelResolutionBit)
	if setting.Key == model.SettingWheelInvert {
		bit = wheelInvertBit
	}
	if setting.Value != 0 {
		flags |= bit
	} else {
		flags &^= bit
	}

	_, err = link.Call(index, 0x02, flags)
	return err
}

// --- smart shift -----------------------------------------------------------
//
// Feature 0x2110, the ratcheting scroll wheel on the MX line.
//
//	fn 0 getRatchetSpeed        → [mode, threshold, defaultThreshold]
//	                              e.g. 02 0A 0A — ratcheting, breaks at 10
//	fn 1 setRatchetSpeed(mode, threshold, defaultThreshold), echoing what it
//	     applied
//
// Mode 1 is a free spin, 2 a ratchet, and 0 means "leave the mode alone" —
// which is not used here, because a read-modify-write says what it is doing.

// smartShiftMax is the largest threshold this project offers.
//
// The device takes a whole byte and reports no range; it accepted 20 and 255
// alike. Past roughly this point the wheel effectively never breaks into a
// spin, which is what the 255 "never" value already says more clearly.
const smartShiftMax = 50

func readSmartShift(link *hidpp.Device) (*model.SmartShift, error) {
	reply, err := link.CallFeature(hidpp.FeatureSmartShift, 0x00)
	if err != nil {
		return nil, err
	}
	return &model.SmartShift{
		Mode:      model.WheelModeName(uint32(at(reply, 0))),
		Threshold: at(reply, 1),
		Default:   at(reply, 2),
		Max:       smartShiftMax,
	}, nil
}

// writeSmartShift changes one field, carrying the others over unchanged: the
// device only takes all three at once.
func writeSmartShift(link *hidpp.Device, setting *model.Setting) error {
	index, err := link.FeatureIndex(hidpp.FeatureSmartShift)
	if err != nil {
		return err
	}
	current, err := link.Call(index, 0x00)
	if err != nil {
		return err
	}

	mode, threshold, fallback := at(current, 0), at(current, 1), at(current, 2)

	switch setting.Key {
	case model.SettingSmartShiftMode:
		mode = byte(setting.Value)
	default:
		// A threshold of zero would be a wheel that breaks into a spin at the
		// slightest touch, which is not a setting anyone means.
		threshold = clampThreshold(setting.Value)
		setting.Value = uint32(threshold)
	}

	_, err = link.Call(index, 0x01, mode, threshold, fallback)
	return err
}

func clampThreshold(value uint32) uint8 {
	if value >= model.ThresholdNever {
		return model.ThresholdNever
	}
	if value < 1 {
		return 1
	}
	if value > smartShiftMax {
		return smartShiftMax
	}
	return uint8(value)
}

// --- reprogrammable buttons ------------------------------------------------
//
// Feature 0x1B04, reassigning what a button does at the device.
//
//	fn 0 getCount                 → [count]
//	fn 1 getCidInfo(index)        → [cid(2), task(2), flags, pos, group, gmask]
//	fn 2 getCidReporting(cid)     → [cid(2), flags, remap(2)]
//	fn 3 setCidReporting(cid, flags, remap(2)), echoing what it applied
//
// The eight controls an MX Master 3S reports, read off the device:
//
//	cid 0x0050 left          flags 0x01  group 1  gmask 0x01
//	cid 0x0051 right         flags 0x01  group 1  gmask 0x01
//	cid 0x0052 middle        flags 0x31  group 2  gmask 0x03
//	cid 0x0053 back          flags 0x31  group 2  gmask 0x03
//	cid 0x0056 forward       flags 0x31  group 2  gmask 0x03
//	cid 0x00C3 gesture       flags 0x31  group 2  gmask 0x03
//	cid 0x00C4 wheel mode    flags 0x31  group 2  gmask 0x03
//	cid 0x00D7 virtual gest. flags 0xA0  group 3  gmask 0x00
//
// Left and right are not reprogrammable — no 0x10 bit — which is the reason
// a mouse cannot be rendered unclickable from here however the rest is set.
//
// Three things established on hardware:
//
//   - **Remap means what it says.** Forward set to 0x0052 middle-clicks, at
//     the device, before the desktop sees anything.
//   - **A remap of zero is ignored.** Writing remap 0x0000 to put a button
//     back is accepted and does nothing; the button keeps its last mapping.
//     The reset is to remap the control *to itself*, which is what
//     writeButton sends for ButtonDefault. A device that has never been
//     touched still *reads* 0x0000, so both spellings count as default.
//   - **Flags are left at zero on write.** For a set, the divert and persist
//     bits are each paired with a "change this" bit, and with those clear the
//     device leaves both alone. Sending zero is therefore a read-modify-write
//     for free, and it cannot divert a button by accident — which would stop
//     the button working at all, exactly as it does on the thumbwheel.
const (
	ctrlReprogrammable = 1 << 4
)

// control is one entry of the device's control table.
type control struct {
	cid   uint16
	flags byte
	group byte
	gmask byte
}

func readButtons(link *hidpp.Device) ([]model.Button, error) {
	index, err := link.FeatureIndex(hidpp.FeatureReprogrammableKeys)
	if err != nil {
		return nil, err
	}
	count, err := link.Call(index, 0x00)
	if err != nil {
		return nil, err
	}

	controls := make([]control, 0, at(count, 0))
	for i := byte(0); i < at(count, 0); i++ {
		info, err := link.Call(index, 0x01, i)
		if err != nil {
			return nil, err
		}
		controls = append(controls, control{
			cid:   uint16(be16(info, 0)),
			flags: at(info, 4),
			group: at(info, 6),
			gmask: at(info, 7),
		})
	}

	buttons := make([]model.Button, 0, len(controls))
	for _, source := range controls {
		if source.flags&ctrlReprogrammable == 0 {
			continue
		}
		reporting, err := link.Call(index, 0x02, byte(source.cid>>8), byte(source.cid))
		if err != nil {
			return nil, err
		}

		slug, label := model.ButtonName(source.cid)
		mapped := uint16(be16(reporting, 3))
		// A pristine control reports no mapping at all; one reset by hand
		// reports itself. Both mean the button does its own job.
		if mapped == 0 {
			mapped = source.cid
		}
		mappedSlug, _ := model.ButtonName(mapped)

		buttons = append(buttons, model.Button{
			Slug:     slug,
			Label:    label,
			CID:      int(source.cid),
			MappedTo: mappedSlug,
			Default:  mapped == source.cid,
			Targets:  targetsFor(source, controls),
		})
	}
	return buttons, nil
}

// targetsFor is what a control may be reassigned to: the controls whose group
// the source's group mask admits. Asking the device rather than assuming is
// what keeps this working on a mouse with a different button layout.
func targetsFor(source control, controls []control) []model.ButtonTarget {
	targets := make([]model.ButtonTarget, 0, len(controls))
	for _, candidate := range controls {
		if candidate.group == 0 || source.gmask&(1<<(candidate.group-1)) == 0 {
			continue
		}
		slug, label := model.ButtonName(candidate.cid)
		targets = append(targets, model.ButtonTarget{
			Slug:  slug,
			Label: label,
			CID:   int(candidate.cid),
		})
	}
	return targets
}

// writeButton reassigns one button. The setting carries the target; the key
// says which button is being changed.
func writeButton(link *hidpp.Device, setting *model.Setting) error {
	slug, ok := model.ButtonSlugOf(setting.Key)
	if !ok {
		return fmt.Errorf("not a button setting")
	}
	source, ok := model.ButtonCID(slug)
	if !ok {
		return fmt.Errorf("unknown button %q", slug)
	}

	// Zero is not a reset — the device ignores it and keeps the last mapping.
	// "default" is turned into the button's own id back in ParseSetting, so
	// reaching here with zero means a caller invented it.
	target := uint16(setting.Value)
	if target == 0 {
		return fmt.Errorf("no target button; to reset, map %s to itself", slug)
	}

	index, err := link.FeatureIndex(hidpp.FeatureReprogrammableKeys)
	if err != nil {
		return err
	}
	_, err = link.Call(index, 0x03,
		byte(source>>8), byte(source), 0x00, byte(target>>8), byte(target))
	return err
}

// --- thumbwheel ------------------------------------------------------------
//
// Feature 0x2150, the horizontal wheel under the thumb on the MX line.
//
//	fn 0 getThumbwheelInfo    → [nativeRes(2), divertedRes(2),
//	                             capabilities(2), timeUnit(2)]
//	fn 1 getThumbwheelStatus  → [reportingMode, invert]
//	fn 2 setThumbwheelReporting(reportingMode, invert)
//
// Read off an MX Master 3S:
//
//	fn 0 → 00 12 00 78 00 0F 03 E8   native 18/turn, diverted 120,
//	                                 capabilities 0x000F, time unit 1000
//	fn 1 → 01 00                     diverted, not inverted
//	fn 1 → 00 00                     after writing scroll mode
//
// Two things were established on hardware rather than assumed:
//
//   - **Diverted is not a preference, it is a dead wheel.** The mouse this was
//     written on arrived in reporting mode 1, and its thumbwheel did nothing
//     at all: movement was going out as HID++ notifications, which nothing was
//     reading. Writing mode 0 brought it straight back. That is the whole
//     reason this capability exists — the useful thing here is being able to
//     see the wheel is diverted and put it back.
//
//   - **Invert does nothing in scroll mode.** Setting byte 1 sticks (fn 1
//     reads back 00 01) and changes nothing about which way the wheel
//     scrolls, so it evidently applies only to the diverted stream. No control
//     is offered for it: a switch that visibly does nothing is worse than no
//     switch. The byte is still carried over untouched on every write, in case
//     something else has set it deliberately.
const (
	thumbwheelModeByte   = 0
	thumbwheelInvertByte = 1
)

func readThumbwheel(link *hidpp.Device) (*model.Thumbwheel, error) {
	reply, err := link.CallFeature(hidpp.FeatureThumbwheel, 0x01)
	if err != nil {
		return nil, err
	}
	return &model.Thumbwheel{
		Mode: model.ThumbwheelModeName(uint32(at(reply, thumbwheelModeByte))),
	}, nil
}

// writeThumbwheel sets the reporting mode, carrying the invert byte over as
// found: it is not ours to clear.
func writeThumbwheel(link *hidpp.Device, mode uint32) error {
	index, err := link.FeatureIndex(hidpp.FeatureThumbwheel)
	if err != nil {
		return err
	}
	current, err := link.Call(index, 0x01)
	if err != nil {
		return err
	}
	_, err = link.Call(index, 0x02, byte(mode), at(current, thumbwheelInvertByte))
	return err
}

// --- easy-switch hosts -----------------------------------------------------
//
// Two features cover this, and a device that has one has both:
//
//	0x1814 ChangeHost
//	  fn 0 getHostInfo      → [hostCount, currentHost]
//	  fn 1 setCurrentHost(hostIndex)
//
//	0x1815 HostsInfo
//	  fn 1 getHostInfo(hostIndex)  → [hostIndex, status, …, nameCapacity]
//	  fn 3 getHostFriendlyName(hostIndex, byteIndex)
//	                               → [hostIndex, byteIndex, name bytes…]
//	  fn 4 setHostFriendlyName     → [hostIndex, bytesWritten]
//
// fn 4 is a *writer*, and it writes whatever it is given: called with no name
// bytes it stores an empty name, wiping what was there. Nothing here calls it.
// It is documented only so the next person reading this does not discover its
// nature the way it was discovered here.
//
// Read off an MX Master 3S paired to two of its three slots, sitting on the
// first:
//
//	0x1814 fn 0        → 03 00                 three slots, on slot 0
//	0x1815 fn 1 (00)   → 00 01 05 01 08 18     slot 0, paired, room for 24
//	0x1815 fn 1 (01)   → 01 01 04 01 08 18     slot 1, paired
//	0x1815 fn 1 (02)   → 02 00 00 00 00 18     slot 2, never paired
//	0x1815 fn 3 (00 00)→ 00 00 6D 65 67 61 …   "megatron"
//
// That last byte is how much room a name has, not how long this one is: it
// reads 24 for an eight-character name, and stayed 24 when the name was
// emptied. The name itself is NUL-terminated, so the read stops at the NUL.
//
// The count comes from 0x1814 rather than 0x1815, because 0x1814 is the one
// that has to agree with it: it is the feature the switch is written through.
//
// Slot numbering is 0 on the wire and 1 in model.Hosts, matching the buttons
// on the underside of the device. The conversion happens here and nowhere
// else.
const (
	hostStatusByte   = 1
	hostNameRoomByte = 5
	// hostNamePerCall is how much of a name one reply carries: 16 bytes of
	// payload less the hostIndex and byteIndex echoed back at the front.
	hostNamePerCall = 14
	// hostNameCap bounds the chunk loop. Names are a couple of dozen bytes;
	// this is only here so a device reporting nonsense cannot spin forever.
	hostNameCap = 128
)

func readHosts(link *hidpp.Device) (*model.Hosts, error) {
	reply, err := link.CallFeature(hidpp.FeatureChangeHost, 0x00)
	if err != nil {
		return nil, err
	}
	count, current := at(reply, 0), at(reply, 1)
	if count == 0 {
		return nil, fmt.Errorf("device reports no host slots")
	}

	// 0x1815 is what describes the slots, and a device can switch hosts
	// without having it. Asked once rather than per slot, so a device without
	// it does not pay for a refusal on every slot.
	_, err = link.FeatureIndex(hidpp.FeatureHostsInfo)
	detailed := err == nil
	if err != nil && !hidpp.Unsupported(err) {
		return nil, err
	}

	hosts := &model.Hosts{Current: int(current) + 1, PairingKnown: detailed}
	for slot := byte(0); slot < count; slot++ {
		host := model.Host{Slot: int(slot) + 1, Active: slot == current}

		// Without 0x1815 nothing is known about the slot beyond its existing,
		// so it is left alone rather than being reported as empty.
		if detailed {
			// A slot that will not describe itself is left as it is: the
			// current slot is the useful part and is already in hand.
			if info, err := link.CallFeature(hidpp.FeatureHostsInfo, 0x01, slot); err == nil {
				host.Paired = at(info, hostStatusByte) != 0
				if host.Paired {
					host.Name = hostName(link, slot, at(info, hostNameRoomByte))
				}
			}
		}
		hosts.Slots = append(hosts.Slots, host)
	}
	return hosts, nil
}

// hostName reads a name out in chunks, stopping at the NUL that ends it so a
// short name costs one call rather than one per chunk of the capacity.
//
// An unreadable chunk ends the name where it got to. A truncated name is worth
// more to whoever is looking at it than no name at all.
func hostName(link *hidpp.Device, slot, room byte) string {
	if room == 0 || room > hostNameCap {
		return ""
	}
	name := make([]byte, 0, room)
	for offset := byte(0); offset < room; offset += hostNamePerCall {
		reply, err := link.CallFeature(hidpp.FeatureHostsInfo, 0x03, slot, offset)
		if err != nil {
			break
		}
		chunk := after(reply, 2)
		name = append(name, chunk...)
		if bytes.IndexByte(chunk, 0) >= 0 {
			break
		}
	}
	if len(name) > int(room) {
		name = name[:room]
	}
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	return strings.TrimSpace(string(name))
}

// writeHost switches the device to another slot.
//
// Nothing is verified here and nothing can be: the device answers, leaves for
// the other host, and is gone before a read could confirm anything. An error
// means the device refused outright, which is the only failure still visible
// from this side. See model.SettingKey.Verifiable.
func writeHost(link *hidpp.Device, slot uint32) error {
	if slot < 1 {
		return fmt.Errorf("host slots are numbered from 1")
	}

	current, err := link.CallFeature(hidpp.FeatureChangeHost, 0x00)
	if err != nil {
		return err
	}
	count := at(current, 0)
	if slot > uint32(count) {
		return fmt.Errorf("device has %d host slots, asked for %d", count, slot)
	}
	if at(current, 1) == byte(slot-1) {
		return fmt.Errorf("device is already on host %d", slot)
	}

	index, err := link.FeatureIndex(hidpp.FeatureChangeHost)
	if err != nil {
		return err
	}
	_, err = link.Call(index, 0x01, byte(slot-1))
	return err
}

// --- onboard profiles ------------------------------------------------------

// Feature 0x8100 getOnboardMode (fn 2) replies with one byte.
const (
	modeOnboard = 0x01
	modeHost    = 0x02
)

// readOnboardMode reports which of the two owns the device's settings.
//
// In onboard mode the mouse runs the profile stored in its own memory and
// refuses software writes; in host mode software owns them. A PRO X2
// SUPERSTRIKE is in host mode on its dongle and onboard mode over USB, so the
// same write succeeds or fails depending on which cable is in.
func readOnboardMode(link *hidpp.Device) (string, error) {
	reply, err := link.CallFeature(hidpp.FeatureOnboardProfiles, 0x02)
	if err != nil {
		return "", err
	}
	switch at(reply, 0) {
	case modeOnboard:
		return "onboard", nil
	case modeHost:
		return "host", nil
	default:
		return "unknown", nil
	}
}

// writeOnboardMode hands the device's settings to its own profile or to
// software, via setOnboardMode (fn 1).
//
// This is the one write that changes how the device behaves when OmniGear is
// not running: in host mode its stored profile stops applying. It is offered
// as an explicit choice for that reason, never taken automatically to make
// some other write succeed.
func writeOnboardMode(link *hidpp.Device, mode uint32) error {
	if mode != modeOnboard && mode != modeHost {
		return fmt.Errorf("%q is not a profile mode", model.ProfileModeName(mode))
	}
	_, err := link.CallFeature(hidpp.FeatureOnboardProfiles, 0x01, byte(mode))
	return err
}

// --- dpi -------------------------------------------------------------------

// defaultLOD is used only if the device will not say what its lift-off
// distance is: the middle of the three values every sensor seen so far offers.
const defaultLOD = 0x02

func readDPI(link *hidpp.Device) (*model.DPI, error) {
	index, err := link.FeatureIndex(hidpp.FeatureExtendedAdjustDPI)
	if err == nil {
		return readDPI2202(link, index)
	}
	if !hidpp.Unsupported(err) {
		return nil, err
	}

	index, err = link.FeatureIndex(hidpp.FeatureAdjustableDPI)
	if err != nil {
		return nil, err
	}
	return readDPI2201(link, index)
}

// readDPI2201 uses getSensorDpiList (fn 1) then getSensorDpi (fn 2).
func readDPI2201(link *hidpp.Device, index byte) (*model.DPI, error) {
	list, err := link.Call(index, 0x01, 0x00)
	if err != nil {
		return nil, err
	}
	presets, min, max, step := decodeDPIStream(after(list, 1))

	current, err := link.Call(index, 0x02, 0x00)
	if err != nil {
		return nil, err
	}
	return &model.DPI{Current: be16(current, 1), Min: min, Max: max, Step: step, Presets: presets}, nil
}

// readDPI2202 uses feature 0x2202.
//
// getSensorDpi (fn 5) replies [sensor, dpiX, defaultDpiX, dpiY, defaultDpiY,
// lod] with each dpi a big-endian uint16 — e.g. 00 0FA0 0FA0 0FA0 0FA0 02 for
// 4000 DPI.
//
// getSensorDpiList (fn 3) replies [sensor, _, dpi…] — the onboard stages, e.g.
// 00 00 0320 04B0 0FA0 0000 for 800/1200/4000.
func readDPI2202(link *hidpp.Device, index byte) (*model.DPI, error) {
	ranges, err := readDPIRanges2202(link, index)
	if err != nil {
		return nil, err
	}
	_, min, max, step := decodeDPIStream(ranges)

	list, err := link.Call(index, 0x03, 0x00, 0x00)
	if err != nil {
		return nil, err
	}
	presets, _, _, _ := decodeDPIStream(after(list, 2))

	current, err := link.Call(index, 0x05, 0x00, 0x00)
	if err != nil {
		return nil, err
	}
	return &model.DPI{Current: be16(current, 1), Min: min, Max: max, Step: step, Presets: presets}, nil
}

// readDPIRanges2202 walks getSensorDpiRanges (fn 2), which pages a single byte
// stream 13 bytes at a time: [sensor, chunkHi, chunkLo, …13 bytes…]. The chunk
// index counts chunks, not values, so a value can straddle two replies and the
// chunks must be concatenated before parsing. An all-zero chunk ends it.
func readDPIRanges2202(link *hidpp.Device, index byte) ([]byte, error) {
	const maxChunks = 8
	var stream []byte
	for chunk := 0; chunk < maxChunks; chunk++ {
		reply, err := link.Call(index, 0x02, 0x00, byte(chunk>>8), byte(chunk))
		if err != nil {
			return nil, err
		}
		body := after(reply, 3)
		if allZero(body) {
			break
		}
		stream = append(stream, body...)
	}
	return stream, nil
}

// decodeDPIStream reads big-endian uint16s. A value with the top three bits set
// is not a DPI but the step for the segment it sits in, so
// 100, E001, 200, E002, 500 means 100–200 by 1, then 200–500 by 2. A zero ends
// the stream.
func decodeDPIStream(bytes []byte) (presets []uint32, min, max, step uint32) {
	var values, steps []uint32

	for i := 0; i+1 < len(bytes); i += 2 {
		raw := uint16(bytes[i])<<8 | uint16(bytes[i+1])
		if raw == 0 {
			break
		}
		if raw&0xE000 == 0xE000 {
			steps = append(steps, uint32(raw&0x1FFF))
		} else {
			values = append(values, uint32(raw))
		}
	}

	for i, v := range values {
		if i == 0 || v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	// The range is stepped in segments that get coarser as the DPI climbs. A
	// single slider cannot express that, so report the finest step and let the
	// device round a value it dislikes.
	for i, s := range steps {
		if i == 0 || s < step {
			step = s
		}
	}

	// A stepped stream names segment boundaries, not presets worth offering.
	if len(steps) == 0 {
		presets = values
	} else {
		presets = []uint32{}
	}
	return presets, min, max, step
}

func writeDPI(link *hidpp.Device, value uint32) error {
	// A refusal here comes back as "logitech internal", which explains
	// nothing. Check the cause first so the message can name it.
	if mode, err := readOnboardMode(link); err == nil && mode == "onboard" {
		return fmt.Errorf("the device is in onboard mode, where its own stored " +
			"profile owns the DPI and software writes are refused")
	}
	if value > 0xFFFF {
		value = 0xFFFF
	}
	hi, lo := byte(value>>8), byte(value)

	index, err := link.FeatureIndex(hidpp.FeatureExtendedAdjustDPI)
	if err == nil {
		// setSensorDpi(sensor, dpiX, dpiY, lod). The lift-off distance is not
		// optional and zero is not a valid one — the device answers "invalid
		// argument" — so carry over whatever it is set to now.
		current, err := link.Call(index, 0x05, 0x00, 0x00)
		if err != nil {
			return err
		}
		lod := byte(defaultLOD)
		if len(current) > 9 && current[9] != 0 {
			lod = current[9]
		}
		_, err = link.Call(index, 0x06, 0x00, hi, lo, hi, lo, lod)
		return err
	}
	if !hidpp.Unsupported(err) {
		return err
	}

	index, err = link.FeatureIndex(hidpp.FeatureAdjustableDPI)
	if err != nil {
		return err
	}
	_, err = link.Call(index, 0x03, 0x00, hi, lo)
	return err
}

// --- polling rate ----------------------------------------------------------

// extendedRatesHz are the rates feature 0x8061's bitmaps and indices refer to,
// slowest bit first.
var extendedRatesHz = [7]uint32{125, 250, 500, 1000, 2000, 4000, 8000}

// connectionArg tells feature 0x8061 which link it is being asked about.
//
// A SUPERSTRIKE answers `00 0F` for 0 — 125–1000 Hz — and `00 7F` for 1, the
// full 125–8000 Hz. Asked for the current rate it says index 3 (1000) for 0 and
// index 4 (2000) for 1, and 2000 is what the mouse is really running on its
// Lightspeed dongle. So 1 is the cable-or-dongle link and 0 is the slow one.
//
// Read as Bluetooth vs everything else that fits: BLE tops out at 1000 Hz,
// while both a cable and a Lightspeed dongle reach 8000. That is an inference
// from one device — if a Bluetooth Logitech mouse ever reports its rate from
// the wrong list, this is the line to revisit.
func connectionArg(node hidraw.Node) byte {
	if node.Link == hidraw.Bluetooth {
		return 0x00
	}
	return 0x01
}

func readPollingRate(link *hidpp.Device, node hidraw.Node) (*model.PollingRate, error) {
	index, err := link.FeatureIndex(hidpp.FeatureExtendedReportRate)
	if err == nil {
		return readRate8061(link, index, node)
	}
	if !hidpp.Unsupported(err) {
		return nil, err
	}

	index, err = link.FeatureIndex(hidpp.FeatureReportRate)
	if err != nil {
		return nil, err
	}
	return readRate8060(link, index)
}

// readRate8061 uses feature 0x8061.
//
// getDeviceCapabilities(connection) (fn 0) replies [connection, bitmap] over
// extendedRatesHz — 00 0F wireless, 00 7F wired on a SUPERSTRIKE.
// getReportRate (fn 2) replies [rateIndex] into the same table.
func readRate8061(link *hidpp.Device, index byte, node hidraw.Node) (*model.PollingRate, error) {
	caps, err := link.Call(index, 0x00, connectionArg(node))
	if err != nil {
		return nil, err
	}
	supported := decodeRateBitmap(at(caps, 1))

	// getReportRate is per-link too. Calling it bare reports connection 0's
	// rate, which is not necessarily the one in use.
	current, err := link.Call(index, 0x02, connectionArg(node))
	if err != nil {
		return nil, err
	}
	rate := uint32(0)
	if slot := int(at(current, 0)); slot < len(extendedRatesHz) {
		rate = extendedRatesHz[slot]
	}
	return &model.PollingRate{Current: rate, Supported: supported}, nil
}

func decodeRateBitmap(bitmap byte) []uint32 {
	supported := []uint32{}
	for bit, hz := range extendedRatesHz {
		if bitmap&(1<<bit) != 0 {
			supported = append(supported, hz)
		}
	}
	return supported
}

// readRate8060 uses feature 0x8060, the older shape: a bitmap over report
// intervals, where bit n means "n+1 ms", and the rate reads back in
// milliseconds.
func readRate8060(link *hidpp.Device, index byte) (*model.PollingRate, error) {
	list, err := link.Call(index, 0x00)
	if err != nil {
		return nil, err
	}
	bitmap := at(list, 0)
	supported := []uint32{}
	for bit := 7; bit >= 0; bit-- {
		if bitmap&(1<<bit) != 0 {
			supported = append(supported, 1000/uint32(bit+1))
		}
	}

	current, err := link.Call(index, 0x01)
	if err != nil {
		return nil, err
	}
	rate := uint32(0)
	if millis := uint32(at(current, 0)); millis > 0 {
		rate = 1000 / millis
	}
	return &model.PollingRate{Current: rate, Supported: supported}, nil
}

func writePollingRate(link *hidpp.Device, node hidraw.Node, hz uint32) error {
	index, err := link.FeatureIndex(hidpp.FeatureExtendedReportRate)
	if err == nil {
		// Refuse a rate this link does not offer rather than let the mouse
		// interpret an out-of-range index.
		caps, err := link.Call(index, 0x00, connectionArg(node))
		if err != nil {
			return err
		}
		slot := -1
		for i, rate := range extendedRatesHz {
			if rate == hz && at(caps, 1)&(1<<i) != 0 {
				slot = i
			}
		}
		if slot < 0 {
			return &hidpp.ProtocolError{Code: 0x03}
		}
		// setReportRate takes the index alone — no connection byte, unlike
		// every other function on this feature. Passing one anyway is not
		// refused: the device reads the first byte as the index and quietly
		// sets the wrong rate.
		_, err = link.Call(index, 0x03, byte(slot))
		return err
	}
	if !hidpp.Unsupported(err) {
		return err
	}

	index, err = link.FeatureIndex(hidpp.FeatureReportRate)
	if err != nil {
		return err
	}
	if hz == 0 || 1000%hz != 0 {
		return &hidpp.ProtocolError{Code: 0x03}
	}
	_, err = link.Call(index, 0x02, byte(1000/hz))
	return err
}

// --- byte helpers ----------------------------------------------------------

func at(bytes []byte, i int) byte {
	if i < len(bytes) {
		return bytes[i]
	}
	return 0
}

func after(bytes []byte, i int) []byte {
	if i >= len(bytes) {
		return nil
	}
	return bytes[i:]
}

func be16(bytes []byte, i int) uint32 {
	if i+1 >= len(bytes) {
		return 0
	}
	return uint32(bytes[i])<<8 | uint32(bytes[i+1])
}

func allZero(bytes []byte) bool {
	for _, b := range bytes {
		if b != 0 {
			return false
		}
	}
	return true
}

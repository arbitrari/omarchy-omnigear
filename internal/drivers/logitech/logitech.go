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

func (driver) Read(device *model.Device) model.DeviceState {
	state := model.NewDeviceState()

	link, err := hidpp.Open(device.Node)
	if err != nil {
		state.Errors = append(state.Errors, fmt.Sprintf("connect: %v", err))
		return state
	}
	defer link.Close()

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
	if device.Entry.Has(model.CapOnboardProfile) {
		mode, err := readOnboardMode(link)
		if err != nil {
			state.Fail(model.CapOnboardProfile, err)
		} else {
			state.OnboardProfile = &mode
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
	link, err := hidpp.Open(device.Node)
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
	case model.SettingProfileMode:
		if err := writeOnboardMode(link, setting.Value); err != nil {
			return fmt.Errorf("profile-mode: %w", err)
		}
	default:
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

// Package steelseries drives SteelSeries headsets through their base
// station's vendor protocol. See transport/arctis for the framing.
//
// Every decoder here carries the bytes it was read off, from an Arctis Nova
// Pro Wireless base station (1038:12E0) on firmware 0000.003.082. Which
// command sets what was confirmed by writing it and reading it back, with
// each setting restored afterwards, and the audible ones by ear.
package steelseries

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/sonar"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// NovaProWireless is the driver for the Arctis Nova Pro Wireless.
var NovaProWireless model.Driver = driver{}

type driver struct{}

func (driver) Name() string { return "SteelSeries Arctis" }

// Speaks picks the base station's protocol interface out of its nodes; the
// other one is media keys.
func (driver) Speaks(node hidraw.Node) bool { return arctis.Speaks(node) }

// Queries. Each returns a record whose fields the setters below write one at
// a time.
const (
	queryStatus   = 0xB0
	querySettings = 0x20
)

// Setters, each one byte. A setter is not answered; it is read back.
const (
	setGain         = 0x27
	setMicVolume    = 0x37
	setSidetone     = 0x39
	setAmbientLevel = 0xB9
	setNoiseMode    = 0xBD
	setMuteLight    = 0xBF
	setAutoPowerOff = 0xC1
	setWirelessMode = 0xC3
)

const (
	statusLength   = 16
	settingsLength = 19

	// batterySteps is how many bars the base station counts a battery in.
	batterySteps = 8
	// Levels — transparency, microphone volume, mute light — run 01 to 0a,
	// and the base station clamps anything outside that.
	minLevel    = 1
	maxLevel    = 10
	maxSidetone = 3

	// headsetOnline is the status byte for a headset on the link.
	headsetOnline = 0x08

	// reconnectTimeout bounds the wait for a headset to come back after a
	// wireless mode change.
	reconnectTimeout = 15 * time.Second
)

// status is the base station's record of itself and its headset.
//
//	06 b0 00 00 01 00 02 08 08 01 02 01 05 00 08 08
//	                  │  │  │  │  │  │  │  │  │  └ headset: 08 online, 01 off
//	                  │  │  │  │  │  │  │  │  └ link: 08 connected, 04 paired, 01 not
//	                  │  │  │  │  │  │  │  └ wireless mode: 00 speed, 01 range
//	                  │  │  │  │  │  │  └ auto power off, 00–06
//	                  │  │  │  │  │  └ mute light, 01–0a
//	                  │  │  │  │  └ noise: 00 off, 01 transparency, 02 cancelling
//	                  │  │  │  └ microphone muted
//	                  │  │  └ transparency level, 01–0a
//	                  │  └ spare battery in the base, in eighths; 00 when empty
//	                  └ headset battery, in eighths
//
// Read with the headset on two bars of eight and the spare full, both
// confirmed on the base station's own screen. Bytes 2–5 are the Bluetooth
// side of the base station, which nothing here drives.
type status struct {
	headsetBattery byte
	spareBattery   byte
	ambientLevel   byte
	noiseMode      byte
	muteLight      byte
	autoPowerOff   byte
	wirelessMode   byte
	headset        byte
}

func readStatus(conn *arctis.Conn) (status, error) {
	reply, err := conn.Call(queryStatus)
	if err != nil {
		return status{}, err
	}
	if len(reply) < statusLength {
		return status{}, fmt.Errorf("short status reply % x", reply)
	}
	return status{
		headsetBattery: reply[6],
		spareBattery:   reply[7],
		ambientLevel:   reply[8],
		noiseMode:      reply[10],
		muteLight:      reply[11],
		autoPowerOff:   reply[12],
		wirelessMode:   reply[13],
		headset:        reply[15],
	}, nil
}

// settings is the base station's record of the headset's audio settings.
//
//	06 20 00 02 02 00 00 14 14 14 00 00 00 00 00 00 00 08 03 01 42 64 64 00 64
//	            │                                      │  └ sidetone, 00–03
//	            │                                      └ microphone volume, 01–0a
//	            └ gain: 01 low, 02 high
//
// Each field was found by writing its setter and watching this record
// change. The rest is unmapped.
type settings struct {
	gain      byte
	micVolume byte
	sidetone  byte
}

func readSettings(conn *arctis.Conn) (settings, error) {
	reply, err := conn.Call(querySettings)
	if err != nil {
		return settings{}, err
	}
	if len(reply) < settingsLength {
		return settings{}, fmt.Errorf("short settings reply % x", reply)
	}
	return settings{gain: reply[4], micVolume: reply[17], sidetone: reply[18]}, nil
}

// eighths turns the base station's battery bars into a percentage. It is a
// reading at the resolution of an eighth, not a bucket: the bars are the
// whole of what the base station knows, and 2 of 8 is a quarter.
func eighths(bars byte) int {
	if bars > batterySteps {
		bars = batterySteps
	}
	return (int(bars)*100 + batterySteps/2) / batterySteps
}

func (s status) online() bool { return s.headset == headsetOnline }

// battery is the headset's charge, with the spare's beside it. A headset that
// is not online has none to report: the base station reads it as 00.
//
// An empty spare slot reads 00 too, the same as a flat battery would, so 00
// is taken as no spare at all. A flat one in the slot starts charging at
// once and shows from its first bar.
func (s status) battery() *model.Battery {
	if !s.online() {
		return nil
	}
	percent := eighths(s.headsetBattery)
	battery := &model.Battery{Percent: &percent, Status: "discharging"}
	if s.spareBattery > 0 {
		spare := eighths(s.spareBattery)
		battery.Spare = &spare
	}
	return battery
}

// noiseModes are the wire's noise modes, by this project's numbering.
var noiseModes = map[uint32]byte{
	model.NoiseOff:        0x00,
	model.NoiseAmbient:    0x01,
	model.NoiseCancelling: 0x02,
}

func noiseModeName(wire byte) string {
	for value, candidate := range noiseModes {
		if candidate == wire {
			return model.NoiseModeName(value)
		}
	}
	return "unknown"
}

// autoPowerOffChoices are the base station's timers, by wire value. Each
// was written and read back; 07 is ignored.
var autoPowerOffChoices = []struct {
	slug, label string
}{
	{"never", "Never"},
	{"1-min", "1 Min"},
	{"5-min", "5 Min"},
	{"10-min", "10 Min"},
	{"15-min", "15 Min"},
	{"30-min", "30 Min"},
	{"1-hour", "1 Hour"},
}

func autoPowerOff(wire byte) (*model.AutoPowerOff, error) {
	off := &model.AutoPowerOff{}
	for i, c := range autoPowerOffChoices {
		off.Options = append(off.Options, model.AutoPowerOffOption{Slug: c.slug, Label: c.label})
		if byte(i) == wire {
			off.Current = c.slug
		}
	}
	if off.Current == "" {
		return nil, fmt.Errorf("unknown auto power off %02x", wire)
	}
	return off, nil
}

func autoPowerOffWire(slug string) (byte, bool) {
	for i, c := range autoPowerOffChoices {
		if c.slug == slug {
			return byte(i), true
		}
	}
	return 0, false
}

// choice builds a Choice from a list of slugs, the wire value being the index
// plus offset.
func choice(wire, offset byte, current func(uint32) string, labels ...string) (*model.Choice, error) {
	if wire < offset || int(wire-offset) >= len(labels) {
		return nil, fmt.Errorf("unknown value %02x", wire)
	}
	c := &model.Choice{Current: current(uint32(wire - offset))}
	for i, label := range labels {
		c.Options = append(c.Options, model.ChoiceOption{Slug: current(uint32(i)), Label: label})
	}
	return c, nil
}

func level(current byte) *model.Level {
	return &model.Level{Current: current, Min: minLevel, Max: maxLevel}
}

func (d driver) Read(device *model.Device) model.DeviceState {
	state := model.NewDeviceState()

	conn, err := open(device.Node)
	if err != nil {
		if errors.Is(err, arctis.ErrTimeout) {
			state.Connected = false
			state.Presence = model.PresenceUnreachable
			return state
		}
		state.Errors = append(state.Errors, err.Error())
		return state
	}
	defer conn.Close()

	st, err := readStatus(conn)
	if err != nil {
		state.Errors = append(state.Errors, err.Error())
		return state
	}

	// The base station knows whether its headset is there without asking it,
	// so off is a fact rather than a guess from silence. Everything the base
	// station holds is still read: a write to it is verified by reading it
	// back, and switching the wireless mode drops the headset while it moves.
	if !st.online() {
		state.Connected = false
		state.Presence = model.PresenceOff
	}

	if device.Entry.Has(model.CapBattery) {
		state.Battery = st.battery()
	}
	if device.Entry.Has(model.CapNoiseControl) {
		state.NoiseControl = &model.NoiseControl{
			Mode:            noiseModeName(st.noiseMode),
			AmbientLevel:    st.ambientLevel,
			MinAmbientLevel: minLevel,
			MaxAmbientLevel: maxLevel,
			AmbientLabel:    "Transparency",
		}
	}
	if device.Entry.Has(model.CapMuteLight) {
		state.MuteLight = level(st.muteLight)
	}
	if device.Entry.Has(model.CapAutoPowerOff) {
		if off, err := autoPowerOff(st.autoPowerOff); err != nil {
			state.Fail(model.CapAutoPowerOff, err)
		} else {
			state.AutoPowerOff = off
		}
	}
	if device.Entry.Has(model.CapWirelessMode) {
		if mode, err := choice(st.wirelessMode, 0, model.WirelessModeName, "Speed", "Range"); err != nil {
			state.Fail(model.CapWirelessMode, err)
		} else {
			state.WirelessMode = mode
		}
	}

	if device.Entry.Has(model.CapSonar) {
		if state.Sonar, err = readSonar(); err != nil {
			state.Fail(model.CapSonar, err)
		}
	}

	if !device.Entry.Has(model.CapGain) && !device.Entry.Has(model.CapMicVolume) &&
		!device.Entry.Has(model.CapSidetone) {
		return state
	}
	set, err := readSettings(conn)
	if err != nil {
		state.Errors = append(state.Errors, err.Error())
		return state
	}
	if device.Entry.Has(model.CapGain) {
		// The wire counts from 01.
		if gain, err := choice(set.gain, 1, model.GainName, "Low", "High"); err != nil {
			state.Fail(model.CapGain, err)
		} else {
			state.Gain = gain
		}
	}
	if device.Entry.Has(model.CapMicVolume) {
		state.MicVolume = level(set.micVolume)
	}
	if device.Entry.Has(model.CapSidetone) {
		if sidetone, err := choice(set.sidetone, 0, model.SidetoneName,
			"Off", "Low", "Medium", "High"); err != nil {
			state.Fail(model.CapSidetone, err)
		} else {
			state.Sidetone = sidetone
		}
	}
	return state
}

// settingCapabilities are the capabilities a write needs the model to have.
var settingCapabilities = map[model.SettingKey]model.Capability{
	model.SettingNoiseMode:    model.CapNoiseControl,
	model.SettingAmbientLevel: model.CapNoiseControl,
	model.SettingAutoPowerOff: model.CapAutoPowerOff,
	model.SettingSidetone:     model.CapSidetone,
	model.SettingMicVolume:    model.CapMicVolume,
	model.SettingMuteLight:    model.CapMuteLight,
	model.SettingGain:         model.CapGain,
	model.SettingWirelessMode: model.CapWirelessMode,
	model.SettingSonar:        model.CapSonar,
}

func (d driver) Write(device *model.Device, setting *model.Setting) error {
	capability, ok := settingCapabilities[setting.Key]
	if _, app := model.SonarAppNameOf(setting.Key); app {
		capability, ok = model.CapSonar, true
	}
	if _, volume := model.SonarVolumeSlugOf(setting.Key); volume {
		capability, ok = model.CapSonar, true
	}
	if !ok || !device.Entry.Has(capability) {
		return fmt.Errorf("%s has no %s", device.Entry.Model, setting.Key)
	}

	// Sonar is the sound server's, and the base station is not spoken to.
	if setting.Key == model.SettingSonar {
		return writeSonar(device, setting.Value != 0)
	}
	if name, ok := model.SonarAppNameOf(setting.Key); ok {
		return sonar.MoveApp(name, model.SonarChannelName(setting.Value))
	}
	if slug, ok := model.SonarVolumeSlugOf(setting.Key); ok {
		if setting.Value > sonar.MaxVolume {
			setting.Value = sonar.MaxVolume
		}
		return sonar.SetVolume(slug, int(setting.Value))
	}

	conn, err := open(device.Node)
	if err != nil {
		return err
	}
	defer conn.Close()

	switch setting.Key {
	case model.SettingNoiseMode:
		return conn.Send(setNoiseMode, noiseModes[setting.Value])
	case model.SettingAmbientLevel:
		// Kept in every mode, unlike a Sony's: the base station reports the
		// level it will go back to even while cancelling.
		return conn.Send(setAmbientLevel, clamp(setting, minLevel, maxLevel))
	case model.SettingMicVolume:
		return conn.Send(setMicVolume, clamp(setting, minLevel, maxLevel))
	case model.SettingMuteLight:
		return conn.Send(setMuteLight, clamp(setting, minLevel, maxLevel))
	case model.SettingSidetone:
		// Anything past 03 is ignored rather than clamped.
		return conn.Send(setSidetone, clamp(setting, 0, maxSidetone))
	case model.SettingGain:
		return conn.Send(setGain, byte(setting.Value)+1)
	case model.SettingAutoPowerOff:
		wire, ok := autoPowerOffWire(model.AutoPowerOffName(setting.Value))
		if !ok {
			return fmt.Errorf("%s has no auto power off %q", device.Entry.Model,
				model.AutoPowerOffName(setting.Value))
		}
		return conn.Send(setAutoPowerOff, wire)
	case model.SettingWirelessMode:
		if err := conn.Send(setWirelessMode, byte(setting.Value)); err != nil {
			return err
		}
		return awaitHeadset(conn)
	}
	return fmt.Errorf("%s has no %s", device.Entry.Model, setting.Key)
}

// clamp keeps a level inside what the base station takes, and reports the
// clamped value back so the caller verifies against what was sent. The base
// station clamps too, but silently.
func clamp(setting *model.Setting, min, max uint32) byte {
	if setting.Value < min {
		setting.Value = min
	}
	if setting.Value > max {
		setting.Value = max
	}
	return byte(setting.Value)
}

// awaitHeadset waits for the headset to come back after a wireless mode
// change.
//
// The change drops the link: the base station reports the headset off at
// once, sends 07 b5 and 07 b7 notifications as it re-pairs, and reports it
// online again some seconds later. Returning straight away would have the
// panel show the headset as off until the next poll, for a change that
// worked. A headset that does not come back in time is not an error — the
// mode is set either way, and the next poll will say where it got to.
func awaitHeadset(conn *arctis.Conn) error {
	deadline := time.Now().Add(reconnectTimeout)
	// Give the base station a moment to notice the link it just dropped,
	// so the first read is not the old online status.
	time.Sleep(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		st, err := readStatus(conn)
		if err == nil && st.online() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// Battery reads charge for `omnigear battery`. Asking the base station costs
// one query to a mains-powered device that keeps the headset's status
// itself, so nothing on the headset wakes.
func (driver) Battery(node hidraw.Node) (model.BatteryReading, bool) {
	if !arctis.Speaks(node) {
		return model.BatteryReading{}, false
	}
	conn, err := arctis.Open(node)
	if err != nil {
		return model.BatteryReading{}, false
	}
	defer conn.Close()
	st, err := readStatus(conn)
	if err != nil {
		return model.BatteryReading{}, false
	}
	return model.BatteryReading{Battery: st.battery(), Connected: st.online()}, true
}

// open opens the base station's protocol interface, and when it cannot, says
// why in terms a user can act on.
func open(node hidraw.Node) (*arctis.Conn, error) {
	if arctis.Speaks(node) {
		return arctis.Open(node)
	}
	// Discovery hands over the media-key node only when the protocol
	// interface has no node at all, which means something has taken it.
	if usbPath, claimed := arctis.Claimed(node); claimed {
		message := "another program has claimed the base station's control interface"
		if contenders := model.Contenders(hidraw.OtherHolders(usbPath)); len(contenders) > 0 {
			names := make([]string, 0, len(contenders))
			for _, c := range contenders {
				names = append(names, c.Label)
			}
			message = fmt.Sprintf("%s has claimed the base station's control interface",
				strings.Join(names, ", "))
		}
		return nil, errors.New(message + "; close it to use OmniGear")
	}
	return nil, errors.New("the base station's control interface is missing")
}

func readSonar() (*model.Sonar, error) {
	status, err := sonar.Read()
	out := &model.Sonar{Enabled: status.Enabled, Live: status.Live, Apps: []model.SonarApp{}}
	for i, c := range sonar.Channels {
		out.Channels = append(out.Channels, model.SonarChannel{
			Slug: c.Slug, Label: c.Label, Sink: c.SinkName(),
			// Game and Chat, the first two, are what the dial balances.
			Mixed: i < 2,
		})
	}
	if err != nil || !status.Live {
		return out, err
	}
	volumes, err := sonar.Volumes()
	if err != nil {
		return out, err
	}
	for i := range out.Channels {
		out.Channels[i].Volume = volumes[out.Channels[i].Slug]
	}
	apps, err := sonar.Apps()
	for _, app := range apps {
		out.Apps = append(out.Apps, model.SonarApp{Name: app.Name, Channel: app.Channel})
	}
	return out, err
}

// writeSonar points the channels at the headset's own output, found by the
// base station's USB ids: its sound card is the same USB device.
func writeSonar(device *model.Device, on bool) error {
	target, err := sonar.SinkFor(device.Node.Vendor, device.Node.Product)
	if err != nil {
		return fmt.Errorf("finding the headset's output: %w", err)
	}
	if on {
		return sonar.Enable(target)
	}
	return sonar.Disable(target)
}

// chatMix is the base station reporting the ChatMix dial, unprompted, every
// time it moves:
//
//	07 45 64 64   centred: game and chat both at 100
//	07 45 64 3a   part way toward game: chat down to 58
//	07 45 64 00   all the way to game: chat silent
//	07 45 3a 64   part way toward chat: game down to 58
//
// Turning toward one side lowers the other, from 100 to 0 in twelve steps of
// about eight; at no point are both below 100. Which side is which was read
// with the dial turned fully to Game and left there.
const chatMix = 0x45

// reportNotice is the report the base station speaks unprompted on.
const reportNotice = 0x07

// WatchChatMix follows the dial until the base station goes away. Reading
// alongside other readers is safe: a hidraw node hands every report to all
// of them.
func (driver) WatchChatMix(node hidraw.Node, onMix func(game, chat uint8)) error {
	handle, err := node.Open()
	if err != nil {
		return err
	}
	defer handle.Close()
	for {
		report, err := handle.Read(time.Minute)
		if errors.Is(err, hidraw.ErrTimeout) {
			continue
		}
		if err != nil {
			return err
		}
		if len(report) >= 4 && report[0] == reportNotice && report[1] == chatMix {
			onMix(min(report[2], 100), min(report[3], 100))
		}
	}
}

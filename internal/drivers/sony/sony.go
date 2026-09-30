// Package sony drives Sony headphones over their RFCOMM control protocol.
//
// Every decoder here carries the bytes it was read off, from a WH-1000XM3 on
// firmware 4.5.2. The protocol is Sony's own and undocumented; the bytes are
// the only proof of what a field means.
package sony

import (
	"errors"
	"fmt"

	"github.com/arbitrari/omarchy-omnigear/internal/drivers/a2dp"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/mdr"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/rfcomm"
)

// MDR is the driver for headphones that speak Sony's MDR protocol.
var MDR model.Driver = driver{}

type driver struct{}

func (driver) Name() string { return "Sony MDR" }

func (driver) Read(device *model.Device) model.DeviceState {
	state := model.NewDeviceState()

	conn, err := mdr.Open(device.Address)
	if err != nil {
		return unreachable(state, err)
	}
	defer conn.Close()

	if device.Entry.Has(model.CapBattery) {
		if battery, err := readBattery(conn); err != nil {
			state.Fail(model.CapBattery, err)
		} else {
			state.Battery = battery
		}
	}
	if device.Entry.Has(model.CapNoiseControl) {
		if noise, err := readNoiseControl(conn); err != nil {
			state.Fail(model.CapNoiseControl, err)
		} else {
			state.NoiseControl = noise
		}
	}
	if device.Entry.Has(model.CapCodec) {
		if codec, err := a2dp.Read(device.Address); err != nil {
			state.Fail(model.CapCodec, err)
		} else {
			state.Codec = codec
		}
	}
	if device.Entry.Has(model.CapEqualizer) {
		if eq, err := readEqualizer(conn); err != nil {
			state.Fail(model.CapEqualizer, err)
		} else {
			state.Equalizer = eq
		}
	}
	return state
}

func (driver) Write(device *model.Device, setting *model.Setting) error {
	// The codec is the sound server's to change, and the headset need not be
	// spoken to at all.
	if setting.Key == model.SettingCodec {
		return a2dp.Write(device.Address, model.CodecName(setting.Value))
	}

	conn, err := mdr.Open(device.Address)
	if err != nil {
		if errors.Is(err, rfcomm.ErrBusy) {
			return busyError
		}
		return err
	}
	defer conn.Close()

	switch setting.Key {
	case model.SettingNoiseMode, model.SettingAmbientLevel, model.SettingFocusOnVoice:
		// One message sets every field, so the others are carried over from
		// a read rather than reset to whatever a blank would mean.
		noise, err := readNoiseControl(conn)
		if err != nil {
			return err
		}
		ambient := model.NoiseModeName(model.NoiseAmbient)
		if setting.Key != model.SettingNoiseMode && noise.Mode != ambient {
			// Outside ambient mode the headset neither reports these nor
			// keeps a write to them, so the write would be accepted and lost.
			return fmt.Errorf("%s only applies in ambient sound mode", setting.Key)
		}
		switch setting.Key {
		case model.SettingNoiseMode:
			noise.Mode = model.NoiseModeName(setting.Value)
			// Noise cancelling reports the ambient level as 0, and ambient at
			// level 0 is the headset's way of saying off. The level it last
			// used is not readable from here, so a switch into ambient that
			// has none to carry over asks for the full room.
			if noise.Mode == ambient && noise.AmbientLevel < noise.MinAmbientLevel {
				noise.AmbientLevel = noise.MaxAmbientLevel
			}
		case model.SettingAmbientLevel:
			level := setting.Value
			if level < uint32(noise.MinAmbientLevel) {
				level = uint32(noise.MinAmbientLevel)
			}
			if level > uint32(noise.MaxAmbientLevel) {
				level = uint32(noise.MaxAmbientLevel)
			}
			setting.Value = level
			noise.AmbientLevel = uint8(level)
		case model.SettingFocusOnVoice:
			noise.FocusOnVoice = setting.Value != 0
		}
		return writeNoiseControl(conn, noise)
	}

	if setting.Key == model.SettingEQPreset {
		eq, err := readEqualizer(conn)
		if err != nil {
			return err
		}
		if !eq.Available {
			return errors.New(eq.Unavailable)
		}
		wire, ok := presetWire(model.EQPresetSlug(setting.Value))
		if !ok {
			return fmt.Errorf("%s has no %q preset", device.Entry.Model, model.EQPresetSlug(setting.Value))
		}
		return conn.Send(0x58, 0x01, wire, 0x00)
	}

	if slug, ok := model.EQBandSlugOf(setting.Key); ok {
		eq, err := readEqualizer(conn)
		if err != nil {
			return err
		}
		if !eq.Available {
			return errors.New(eq.Unavailable)
		}
		band := eq.Band(slug)
		if band == nil {
			return fmt.Errorf("%s has no %q band", device.Entry.Model, slug)
		}
		level := int(setting.Value) - model.EQBias
		level = max(eq.Min, min(eq.Max, level))
		setting.Value = uint32(level + model.EQBias)
		band.Value = level
		return writeBands(conn, eq)
	}

	return fmt.Errorf("%s cannot set %s", device.Entry.Model, setting.Key)
}

var busyError = errors.New("another program is connected to the headset's control channel; " +
	"only one can be at a time")

// unreachable describes a headset BlueZ says is connected but whose control
// channel would not open.
func unreachable(state model.DeviceState, err error) model.DeviceState {
	if errors.Is(err, rfcomm.ErrBusy) {
		// The headset is fine and answering someone else. That is an error to
		// show, not an absence.
		state.Errors = append(state.Errors, busyError.Error())
		return state
	}
	state.Connected = false
	state.Presence = model.PresenceUnreachable
	if !errors.Is(err, mdr.ErrTimeout) {
		state.Errors = append(state.Errors, err.Error())
	}
	return state
}

// readBattery asks for the headset's charge.
//
//	→ 10 00
//	← 11 00 3c 00      60%, not charging
//
// The second byte of the request picks which battery; 00 is the only one a
// pair of over-ear headphones has.
func readBattery(conn *mdr.Conn) (*model.Battery, error) {
	reply, err := conn.Call(0x11, 0x10, 0x00)
	if err != nil {
		return nil, err
	}
	if len(reply) < 4 {
		return nil, fmt.Errorf("short battery reply % x", reply)
	}
	percent := int(reply[2])
	status := "discharging"
	if reply[3] == 0x01 {
		status = "charging"
	}
	return &model.Battery{Percent: &percent, Status: status}, nil
}

// Ambient level range. The headset does not report one; 20 is the most it
// keeps (68 02 11 02 00 01 00 15 reads back as level 14 hex), and 0 is not a
// level at all but off.
const (
	minAmbientLevel = 1
	maxAmbientLevel = 20
)

// readNoiseControl reads noise cancelling and ambient sound, which the
// headset keeps as one record.
//
//	→ 66 02
//	← 67 02 01 02 02 01 00 00   noise cancelling
//	← 67 02 01 02 00 01 00 02   ambient sound, level 2
//	← 67 02 00 02 00 01 00 02   off
//	         │  │  │  │  │  └─ ambient level, 1–20
//	         │  │  │  │  │     (0 while cancelling)
//	         │  │  │  │  └──── focus on voice
//	         │  │  │  └─────── 01: ambient is level-adjustable
//	         │  │  └────────── 02 cancelling, 00 ambient
//	         │  └───────────── 02: dual NC/ambient model
//	         └──────────────── 00 off, anything else on
//
// Captured by pressing the NC/AMBIENT button through its cycle, which the
// headset reports as notifications of the same shape (69 02 …). Covering the
// right earcup for quick attention does not touch this record at all; it
// arrives separately as 85 01 01 00 / 85 01 00 00.
func readNoiseControl(conn *mdr.Conn) (*model.NoiseControl, error) {
	reply, err := conn.Call(0x67, 0x66, 0x02)
	if err != nil {
		return nil, err
	}
	if len(reply) < 8 {
		return nil, fmt.Errorf("short noise control reply % x", reply)
	}
	noise := &model.NoiseControl{
		AmbientLevel:    reply[7],
		MinAmbientLevel: minAmbientLevel,
		MaxAmbientLevel: maxAmbientLevel,
		FocusOnVoice:    reply[6] != 0,
	}
	switch {
	case reply[2] == 0x00:
		noise.Mode = model.NoiseModeName(model.NoiseOff)
	case reply[4] == 0x00:
		noise.Mode = model.NoiseModeName(model.NoiseAmbient)
	default:
		noise.Mode = model.NoiseModeName(model.NoiseCancelling)
	}
	return noise, nil
}

// writeNoiseControl sets the whole record. The layout is the read's, with 68
// in place of 67 and the on/off byte written as 11 rather than 01. Written as
// 01 the mode changes but a new level is ignored: 68 02 01 02 00 01 00 14
// left a headset on level 0a reading 0a.
//
// Everything is sent in one message, so the caller carries over what it is
// not changing.
func writeNoiseControl(conn *mdr.Conn, noise *model.NoiseControl) error {
	effect, cancelling := byte(0x11), byte(0x00)
	switch noise.Mode {
	case model.NoiseModeName(model.NoiseOff):
		effect = 0x00
	case model.NoiseModeName(model.NoiseCancelling):
		cancelling = 0x02
	}
	voice := byte(0)
	if noise.FocusOnVoice {
		voice = 1
	}
	return conn.Send(0x68, 0x02, effect, 0x02, cancelling, 0x01, voice, noise.AmbientLevel)
}

// Codecs, as 18 00 reports them, each read off the headset while PipeWire
// had it selected.
var codecNames = map[byte]string{
	0x01: "SBC",
	0x02: "AAC",
	0x10: "LDAC",
	0x20: "aptX",
	0x21: "aptX HD",
}

// readCodec asks which codec the audio is using right now.
//
//	→ 18 00
//	← 19 00 10      LDAC
func readCodec(conn *mdr.Conn) (byte, error) {
	reply, err := conn.Call(0x19, 0x18, 0x00)
	if err != nil {
		return 0, err
	}
	if len(reply) < 3 {
		return 0, fmt.Errorf("short codec reply % x", reply)
	}
	return reply[2], nil
}

// eqPresets are the XM3's presets, in the order the Sony app lists them, with
// their wire ids. Found by writing each id over AAC and reading it back; an id
// the headset does not have is not refused, it resets to off (58 01 18 00
// reads back as 57 01 00 …).
var eqPresets = []struct {
	slug     string
	label    string
	wire     byte
	editable bool
}{
	{"off", "Off", 0x00, false},
	{"bright", "Bright", 0x10, false},
	{"excited", "Excited", 0x11, false},
	{"mellow", "Mellow", 0x12, false},
	{"relaxed", "Relaxed", 0x13, false},
	{"vocal", "Vocal", 0x14, false},
	{"treble-boost", "Treble Boost", 0x15, false},
	{"bass-boost", "Bass Boost", 0x16, false},
	{"speech", "Speech", 0x17, false},
	{"manual", "Manual", 0xA0, true},
	{"custom-1", "Custom 1", 0xA1, true},
	{"custom-2", "Custom 2", 0xA2, true},
}

func presetWire(slug string) (byte, bool) {
	for _, p := range eqPresets {
		if p.slug == slug {
			return p.wire, true
		}
	}
	return 0, false
}

// eqBands are the six values the headset reports, in its order. The
// frequencies are the Sony app's labels; the headset only sends the numbers.
var eqBands = []struct{ slug, label string }{
	{"clear-bass", "Clear Bass"},
	{"400", "400"},
	{"1k", "1k"},
	{"2k5", "2.5k"},
	{"6k3", "6.3k"},
	{"16k", "16k"},
}

// Band levels on the wire are 00–14, centred on 0a. 15 is not refused; it is
// kept as 14.
const (
	eqCentre = 0x0A
	eqMin    = -10
	eqMax    = 10
)

// eqCodecs are the host's codecs over which the headset applies an
// equalizer. SBC-XQ is SBC at a higher bitpool, and the equalizer worked over
// it as well as over SBC and AAC.
var eqCodecs = []string{"sbc", "sbc-xq", "aac"}

// readEqualizer reads the preset and what its bands are set to.
//
//	→ 56 01
//	← 57 01 00 06 0a 0a 0a 0a 0a 0a   off: everything flat
//	← 57 01 10 06 09 0a 0f 11 11 13   bright: -1, 0, +5, +7, +7, +9
//	        │  │  └─ Clear Bass, then 400 Hz to 16 kHz
//	        │  └──── six values follow
//	        └─────── preset
//
// Over LDAC or aptX the headset still answers this, but refuses any change
// with 99 01 01 01 and applies no equalizer. The codec is read alongside so
// the panel can say so rather than offer controls that do nothing.
func readEqualizer(conn *mdr.Conn) (*model.Equalizer, error) {
	reply, err := conn.Call(0x57, 0x56, 0x01)
	if err != nil {
		return nil, err
	}
	if len(reply) < 4 || int(reply[3]) != len(eqBands) || len(reply) < 4+len(eqBands) {
		return nil, fmt.Errorf("unexpected equalizer reply % x", reply)
	}

	eq := &model.Equalizer{Available: true, Codecs: eqCodecs, Min: eqMin, Max: eqMax}
	for _, p := range eqPresets {
		eq.Presets = append(eq.Presets, model.EQPreset{Slug: p.slug, Label: p.label, Editable: p.editable})
		if p.wire == reply[2] {
			eq.Preset = p.slug
		}
	}
	for i, b := range eqBands {
		eq.Bands = append(eq.Bands, model.EQBand{Slug: b.slug, Label: b.label, Value: int(reply[4+i]) - eqCentre})
	}

	codec, err := readCodec(conn)
	if err != nil {
		return nil, err
	}
	if codec != 0x01 && codec != 0x02 {
		name := codecNames[codec]
		if name == "" {
			name = fmt.Sprintf("codec 0x%02x", codec)
		}
		eq.Available = false
		eq.Unavailable = fmt.Sprintf("the equalizer only works over SBC or AAC, and the headset is using %s", name)
	}
	return eq, nil
}

// writeBands sets every band at once. On a preset with bands of its own the
// preset is kept; on any other, the headset is moved to manual carrying the
// bands over, so a nudge to one band of Bright starts from Bright.
//
//	→ 58 01 a0 06 0c 0b 0a 09 08 07
//	← 59 01 a0 06 0c 0b 0a 09 08 07   (notification of the new state)
func writeBands(conn *mdr.Conn, eq *model.Equalizer) error {
	preset := byte(0xA0)
	for _, p := range eqPresets {
		if p.slug == eq.Preset && p.editable {
			preset = p.wire
		}
	}
	payload := []byte{0x58, 0x01, preset, byte(len(eq.Bands))}
	for _, band := range eq.Bands {
		payload = append(payload, byte(band.Value+eqCentre))
	}
	return conn.Send(payload...)
}

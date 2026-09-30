// Package sony drives Sony headphones over their RFCOMM control protocol.
//
// Every decoder here carries the bytes it was read off, from a WH-1000XM3 on
// firmware 4.5.2. The protocol is Sony's own and undocumented; the bytes are
// the only proof of what a field means.
package sony

import (
	"errors"
	"fmt"

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
	return state
}

func (driver) Write(device *model.Device, setting *model.Setting) error {
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
	default:
		return fmt.Errorf("%s cannot set %s", device.Entry.Model, setting.Key)
	}
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

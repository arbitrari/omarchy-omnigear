// Package hidpp speaks HID++ 2.0, Logitech's feature-addressed request and
// response protocol.
//
// Every Logitech device uses it, mouse or keyboard, so it lives under
// transport/ rather than under any one brand. A driver asks for a feature by
// its well-known id (0x1004 unified battery, 0x2201 adjustable DPI, …); this
// package resolves that to the per-device feature index and does the
// request/response dance.
//
// Wire format, short (7 bytes) and long (20 bytes) reports:
//
//	[0] report id      0x10 short, 0x11 long
//	[1] device index   0xFF direct, or 0x01..0x06 behind a receiver
//	[2] feature index  resolved via the root feature
//	[3] (function << 4) | software id
//	[4..] parameters
//
// An error comes back with feature index 0xFF, echoing the feature index and
// function byte of the request, then a one-byte error code.
package hidpp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

const (
	reportShort = 0x10
	reportLong  = 0x11
	lenShort    = 7
	lenLong     = 20

	// swID identifies our traffic so a reply can be told apart from an
	// unsolicited notification. Any value 1..15; 0 is reserved for those.
	swID = 0x0A

	// The root feature resolves every other feature's index. Its own index is
	// always 0.
	featureRoot    = 0x00
	rootGetFeature = 0x00
	rootPing       = 0x01
)

const callTimeout = 600 * time.Millisecond

// A reply can be preceded by unrelated notifications (battery, connection).
// Drain a bounded number of them before giving up.
const maxSkippedReports = 16

// Well-known HID++ 2.0 feature ids used by this project.
const (
	FeatureDeviceName         uint16 = 0x0005
	FeatureBatteryStatus      uint16 = 0x1000
	FeatureBatteryVoltage     uint16 = 0x1001
	FeatureUnifiedBattery     uint16 = 0x1004
	FeatureHITS               uint16 = 0x1B0C // Haptic Inductive Trigger System
	FeatureAdjustableDPI      uint16 = 0x2201
	FeatureExtendedAdjustDPI  uint16 = 0x2202
	FeatureReportRate         uint16 = 0x8060
	FeatureExtendedReportRate uint16 = 0x8061
	FeatureOnboardProfiles    uint16 = 0x8100
)

// ErrTimeout means the device did not answer in time.
var ErrTimeout = errors.New("device did not answer")

// UnsupportedError means the device does not implement the feature. Drivers
// match on it to fall back to an older generation of the same feature.
type UnsupportedError struct{ Feature uint16 }

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("feature 0x%04X not supported", e.Feature)
}

// ProtocolError is the device refusing a request it understood.
type ProtocolError struct{ Code byte }

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("device refused the request (%s)", errorName(e.Code))
}

func errorName(code byte) string {
	switch code {
	case 0x01:
		return "unknown request"
	case 0x02:
		return "invalid argument"
	case 0x03:
		return "out of range"
	case 0x04:
		return "hardware error"
	case 0x05:
		return "logitech internal"
	case 0x06:
		return "invalid feature index"
	case 0x07:
		return "invalid function id"
	case 0x08:
		return "busy"
	case 0x09:
		return "unsupported"
	default:
		return "unspecified"
	}
}

// Unsupported reports whether err means "this device does not have that
// feature", which is the cue to try an older one.
func Unsupported(err error) bool {
	var unsupported *UnsupportedError
	return errors.As(err, &unsupported)
}

// Device is an open HID++ conversation with one device.
type Device struct {
	handle *hidraw.Handle
	index  byte
}

// Open opens node and finds the device index that answers a ping.
//
// A device reached through its own hidraw node answers on 0xFF; one reached
// through a receiver answers on its paired slot. Trying both keeps drivers
// from having to care which node they were handed.
func Open(node hidraw.Node) (*Device, error) {
	handle, err := node.Open()
	if err != nil {
		return nil, err
	}

	device := &Device{handle: handle}
	var last error = ErrTimeout
	for _, index := range []byte{0xFF, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06} {
		device.index = index
		if err := device.Ping(); err == nil {
			return device, nil
		} else {
			last = err
		}
	}

	handle.Close()
	return nil, last
}

func (d *Device) Close() error { return d.handle.Close() }

// Index is the device index that answered, for diagnostics.
func (d *Device) Index() byte { return d.index }

// Ping round-trips the root feature's ping. A cheap liveness check.
func (d *Device) Ping() error {
	const marker = 0x5A
	reply, err := d.Call(featureRoot, rootPing, 0x00, 0x00, marker)
	if err != nil {
		return err
	}
	// The device echoes the marker in the third parameter byte.
	if len(reply) < 3 || reply[2] != marker {
		return ErrTimeout
	}
	return nil
}

// FeatureIndex resolves a feature id to this device's index for it.
func (d *Device) FeatureIndex(feature uint16) (byte, error) {
	reply, err := d.Call(featureRoot, rootGetFeature, byte(feature>>8), byte(feature))
	if err != nil {
		return 0, err
	}
	// Index 0 means "not implemented" for anything but the root itself.
	if len(reply) == 0 || reply[0] == 0 {
		return 0, &UnsupportedError{Feature: feature}
	}
	return reply[0], nil
}

// Call invokes a function on an already-resolved feature index and returns the
// reply's parameter bytes — everything after the four-byte header.
func (d *Device) Call(featureIndex, function byte, params ...byte) ([]byte, error) {
	funcByte := byte(function<<4) | swID

	length := lenLong
	reportID := byte(reportLong)
	if len(params) <= lenShort-4 {
		length = lenShort
		reportID = reportShort
	}

	out := make([]byte, length)
	out[0] = reportID
	out[1] = d.index
	out[2] = featureIndex
	out[3] = funcByte
	copy(out[4:], params)

	if err := d.handle.Write(out); err != nil {
		return nil, err
	}

	for i := 0; i < maxSkippedReports; i++ {
		reply, err := d.handle.Read(callTimeout)
		if err != nil {
			if errors.Is(err, hidraw.ErrTimeout) {
				return nil, ErrTimeout
			}
			return nil, err
		}
		if len(reply) < 4 || reply[1] != d.index {
			continue
		}

		// Error: feature index 0xFF, then the echoed request header.
		if reply[2] == 0xFF {
			if len(reply) >= 6 && reply[3] == featureIndex && reply[4] == funcByte {
				return nil, &ProtocolError{Code: reply[5]}
			}
			continue
		}

		if reply[2] == featureIndex && reply[3] == funcByte {
			return reply[4:], nil
		}
		// Anything else is a notification or another caller's reply.
	}
	return nil, ErrTimeout
}

// CallFeature resolves and calls in one step, for a feature used exactly once.
func (d *Device) CallFeature(feature uint16, function byte, params ...byte) ([]byte, error) {
	index, err := d.FeatureIndex(feature)
	if err != nil {
		return nil, err
	}
	return d.Call(index, function, params...)
}

// Name reads feature 0x0005, the device's own marketing name.
func (d *Device) Name() (string, error) {
	index, err := d.FeatureIndex(FeatureDeviceName)
	if err != nil {
		return "", err
	}
	count, err := d.Call(index, 0x00)
	if err != nil {
		return "", err
	}
	if len(count) == 0 || count[0] == 0 {
		return "", &UnsupportedError{Feature: FeatureDeviceName}
	}
	total := int(count[0])

	var name strings.Builder
	for name.Len() < total {
		chunk, err := d.Call(index, 0x01, byte(name.Len()))
		if err != nil {
			return "", err
		}
		before := name.Len()
		for _, b := range chunk {
			if b == 0 {
				break
			}
			name.WriteByte(b)
		}
		if name.Len() == before {
			break
		}
	}

	text := name.String()
	if len(text) > total {
		text = text[:total]
	}
	return strings.TrimSpace(text), nil
}

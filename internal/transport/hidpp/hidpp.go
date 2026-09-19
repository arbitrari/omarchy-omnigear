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
	"bytes"
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

// probeTimeout is the budget for deciding whether a node is worth talking to
// at all. A device that is there answers a ping in single-digit milliseconds;
// one that is not never answers, and paying the full call timeout seven times
// over to find that out would stall every poll.
const probeTimeout = 150 * time.Millisecond

// maxSkippedReports only guards against an endless stream; the real limit on
// waiting for a reply is the timeout.
//
// A count alone is not enough. On a connection where HID++ shares a node with
// the device's ordinary input — Bluetooth, where movement reports and replies
// arrive on the same stream — a mouse in use produces well over a hundred
// reports a second, and a reply sitting behind sixteen of them is a reply
// thrown away. That reads as the device being absent while it is in fact
// answering.
const maxSkippedReports = 4096

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
	handle  *hidraw.Handle
	index   byte
	timeout time.Duration
	// alwaysLong is set for a node that carries only the long report. Sending
	// a short one there is not refused — it simply goes nowhere.
	alwaysLong bool
}

// reports says which HID++ report sizes a node carries, read off its HID
// report descriptor.
//
// The pairing is not guaranteed. An MX Master 3S on its dongle carries both;
// the same mouse over Bluetooth carries only the long one, and a short request
// to it vanishes without an error — the device simply never answers, which
// looks exactly like absence.
func reports(node hidraw.Node) (short, long bool) {
	descriptor, err := node.ReportDescriptor()
	if err != nil {
		// Unreadable: assume the usual pair rather than rule the node out.
		return true, true
	}
	// 0x85 is the HID "Report ID" item; the byte after it is the id.
	return bytes.Contains(descriptor, []byte{0x85, reportShort}),
		bytes.Contains(descriptor, []byte{0x85, reportLong})
}

// Speaks reports whether a node carries HID++ at all, by looking for the short
// and long report ids in its HID report descriptor. It opens nothing.
//
// This is what separates a mouse's HID++ interface from its plain input ones:
// of the four nodes a wired PRO X2 SUPERSTRIKE owns, exactly one declares both.
func Speaks(node hidraw.Node) bool {
	// The long report is the one every HID++ device carries; the short one is
	// an optimisation some connections leave out.
	_, long := reports(node)
	return long
}

// Responds reports whether a device is actually reachable on this node.
//
// A node outlives the device behind it: unplug a mouse from its dongle and
// pair it over USB, and the dongle's node stays, answering nothing. Only a
// ping can tell the two apart.
func Responds(node hidraw.Node, within time.Duration) bool {
	device, err := openWith(node, within, within)
	if err != nil {
		return false
	}
	device.Close()
	return true
}

// ProbeTimeout is the quick budget for asking whether a node is live at all,
// and WakeTimeout the patient one for a device that may be asleep.
const (
	ProbeTimeout = probeTimeout
	WakeTimeout  = wakeTimeout
)

// Open opens node and finds the device index that answers a ping.
//
// A device reached through its own hidraw node answers on 0xFF; one reached
// through a receiver answers on its paired slot. Trying both keeps drivers
// from having to care which node they were handed.
func Open(node hidraw.Node) (*Device, error) {
	// Two passes over the indexes. The device usually answers on the first or
	// second one, and a quick sweep finds it without paying the wake budget
	// for every index that does not reply — a mouse that answers on index 1
	// would otherwise spend the whole wake window discovering that 0xFF is
	// silent, on every single read.
	//
	// Only when nothing answers at all is the patient sweep worth it, because
	// then the device may be asleep rather than absent.
	if device, err := openWith(node, probeTimeout, callTimeout); err == nil {
		return device, nil
	}
	return openWith(node, wakeTimeout, callTimeout)
}

// openWith searches the indexes for one that answers.
//
// The two budgets are separate on purpose. Finding the device is allowed the
// wake window, because a sleeping one is slow to say its first word; talking
// to it afterwards is held to the ordinary call timeout, because a device
// that is awake and still silent is a device with a problem.
func openWith(node hidraw.Node, ping, call time.Duration) (*Device, error) {
	handle, err := node.Open()
	if err != nil {
		return nil, err
	}

	short, _ := reports(node)
	device := &Device{handle: handle, timeout: ping, alwaysLong: !short}

	var last error = ErrTimeout
	for _, index := range []byte{0xFF, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06} {
		device.index = index
		if err := device.Ping(); err == nil {
			device.timeout = call
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
	if !d.alwaysLong && len(params) <= lenShort-4 {
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

	deadline := time.Now().Add(d.timeout)
	for i := 0; i < maxSkippedReports; i++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ErrTimeout
		}
		reply, err := d.handle.Read(remaining)
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

// --- devices behind a receiver ----------------------------------------------
//
// A dongle the kernel understands is expanded into one hidraw node per paired
// device, and those are found by USB id like anything else. A dongle it does
// not — a Logi Bolt on a kernel whose hid-logitech-dj has no entry for it —
// stays a single node, and the devices behind it are reachable only by their
// index on that node. This is how they are found.

// wakeTimeout is the window for the first word with a device behind a
// receiver. A sleeping MX Master 3S took over half a second to answer its
// first ping, so the ordinary call timeout reports it as absent; once awake it
// replies in milliseconds.
const wakeTimeout = 2500 * time.Millisecond

// maxPairedDevices is how many slots a receiver can hold.
const maxPairedDevices = 6

// Paired is one device reachable through a receiver.
type Paired struct {
	Index byte
	Name  string
}

// OpenAt opens a conversation with one specific device index, rather than
// searching for whichever answers. A receiver with two devices paired needs
// the caller to say which one it means.
func OpenAt(node hidraw.Node, index byte) (*Device, error) {
	handle, err := node.Open()
	if err != nil {
		return nil, err
	}

	short, _ := reports(node)
	device := &Device{handle: handle, index: index, timeout: wakeTimeout, alwaysLong: !short}
	if err := device.Ping(); err != nil {
		handle.Close()
		return nil, err
	}
	device.timeout = callTimeout
	return device, nil
}

// PairedDevices lists what is reachable through a receiver node, by index.
//
// The slots are probed at once rather than in turn: an empty one costs the
// whole wake timeout, and waiting out five of them in series to find the sixth
// device would make every poll crawl. The receiver's own count, when it gives
// one, ends the scan as soon as that many have answered.
func PairedDevices(node hidraw.Node) []Paired {
	expected := pairedCount(node)
	if expected == 0 {
		return nil
	}

	type result struct {
		paired Paired
		ok     bool
	}
	results := make(chan result, maxPairedDevices)

	for index := byte(1); index <= maxPairedDevices; index++ {
		go func(index byte) {
			device, err := OpenAt(node, index)
			if err != nil {
				results <- result{}
				return
			}
			defer device.Close()

			name, err := device.Name()
			if err != nil {
				results <- result{}
				return
			}
			results <- result{paired: Paired{Index: index, Name: name}, ok: true}
		}(index)
	}

	var found []Paired
	for i := 0; i < maxPairedDevices; i++ {
		if r := <-results; r.ok {
			found = append(found, r.paired)
			if expected > 0 && len(found) >= expected {
				break
			}
		}
	}
	return found
}

// pairedCount asks the receiver how many devices it holds, in HID++ 1.0:
//
//	request [0x10, 0xFF, 0x81, 0x02, 0, 0, 0]
//	reply   [0x10, 0xFF, 0x81, 0x02, _, count, _]
//
// It is an optimisation, not a gate. A receiver that refuses or answers
// strangely falls back to scanning every slot, which is slower but complete;
// only a zero is taken at its word.
func pairedCount(node hidraw.Node) int {
	handle, err := node.Open()
	if err != nil {
		return maxPairedDevices
	}
	defer handle.Close()

	if err := handle.Write([]byte{reportShort, 0xFF, 0x81, 0x02, 0x00, 0x00, 0x00}); err != nil {
		return maxPairedDevices
	}

	deadline := time.Now().Add(callTimeout)
	for time.Now().Before(deadline) {
		reply, err := handle.Read(callTimeout)
		if err != nil {
			break
		}
		// 0x8F is the HID++ 1.0 error reply; anything else addressed to the
		// receiver with our register is the answer.
		if len(reply) >= 6 && reply[1] == 0xFF && reply[2] == 0x81 && reply[3] == 0x02 {
			return int(reply[5])
		}
	}
	return maxPairedDevices
}

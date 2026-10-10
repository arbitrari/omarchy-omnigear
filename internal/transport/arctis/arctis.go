// Package arctis speaks the vendor protocol of a SteelSeries Arctis base
// station over hidraw.
//
// It is not documented anywhere public. What is here was read off an Arctis
// Nova Pro Wireless base station (1038:12E0, firmware 0000.003.082), with
// Linux-Arctis-Manager's device description (GPL-3.0) as a starting point for
// which commands exist. Each decoder in drivers/steelseries carries the bytes
// it was read from.
//
// The base station owns two HID interfaces. Interface 3 is media keys and
// nothing else; interface 4 carries the protocol, as two vendor collections:
//
//	06 c0 ff 0a 01 00 a1 01 85 06 …   usage page 0xFFC0, report 06
//	06 00 ff 0a 01 00 a1 01 85 07 …   usage page 0xFF00, report 07
//
// A request is report 06, a command byte and its arguments, zero-padded to
// the report's 63 bytes. A query is answered on the same report, echoing the
// command:
//
//	06 b0          → 06 b0 00 00 01 00 02 08 08 …
//
// A write is not answered at all; it is checked by reading it back. Report
// 07 is the base station speaking unprompted, when the headset connects or
// drops, and arrives interleaved with replies.
package arctis

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

const (
	// reportRequest carries requests, and the replies to them.
	reportRequest = 0x06
	// reportLength is the report id and its 63 bytes.
	reportLength = 64
)

// CallTimeout bounds one query. The base station is powered by its cable and
// answers in a few milliseconds whatever the headset is doing; it keeps the
// headset's status itself, so asking does not reach the headset at all.
const CallTimeout = 500 * time.Millisecond

// ErrTimeout is a query the base station never answered.
var ErrTimeout = errors.New("base station did not answer")

// vendorPage is the usage page item that opens the protocol's collection:
// Usage Page (0xFFC0), a two-byte global item.
var vendorPage = []byte{0x06, 0xC0, 0xFF}

// Speaks reports whether node is the base station's protocol interface,
// from its report descriptor alone. Nothing is opened.
func Speaks(node hidraw.Node) bool {
	descriptor, err := node.ReportDescriptor()
	if err != nil {
		return false
	}
	return bytes.Contains(descriptor, vendorPage)
}

// Conn is an open protocol interface.
type Conn struct {
	handle *hidraw.Handle
}

func Open(node hidraw.Node) (*Conn, error) {
	if !Speaks(node) {
		return nil, fmt.Errorf("%s is not the base station's control interface", node.Path)
	}
	handle, err := node.Open()
	if err != nil {
		return nil, err
	}
	return &Conn{handle: handle}, nil
}

func (c *Conn) Close() error { return c.handle.Close() }

// Send writes one request and expects no reply, as for every write.
func (c *Conn) Send(command byte, args ...byte) error {
	report := make([]byte, reportLength)
	report[0] = reportRequest
	report[1] = command
	copy(report[2:], args)
	return c.handle.Write(report)
}

// The screen. It has no off of its own: 85 sets its brightness, and 00 there
// is the dimmest rather than dark. What it does take is a picture, which it
// shows in place of its own screens until it is handed back or touched.
//
// A picture is drawn a tile at a time, each tile a feature report on report
// 06, too long for an output report:
//
//	06 93 xx yy ww hh <pixels>   1024 bytes in all
//
// at column xx and row yy, ww pixels wide and hh high, one bit a pixel,
// column by column, one for lit. Within a tile, pixel x, y is bit i = x*64 + y,
// in byte 6 + i/8 at bit i%8, lowest first:
//
//	06 93 00 00 40 40 01 00 …   the left tile, with only its top-left pixel lit
//
// The screen is two 64-pixel tiles side by side, at xx 00 and 40. 06 95 is an
// ordinary request, unanswered, that hands the screen back to the base
// station, which draws its own at once.
//
// ggoled (github.com/JerwuQu/ggoled) was the starting point for the
// framing. Read live on 1038:12E0 firmware 0000.003.082: two blank tiles
// left the whole screen dark, 15 and 30 seconds untouched, and 95 brought
// the base station's own screen straight back. Turning the volume knob or
// the ChatMix dial brings it back too, with no request at all. Each tile
// takes 18 ms to send, so a whole picture takes 36 ms, however much of it
// changed.
const (
	commandDraw      = 0x93
	commandOwnScreen = 0x95
	drawLength       = 1024
	tileSize         = 0x40
	screenTiles      = 2
	tileHeader       = 6
)

// The screen's size, in pixels.
const (
	ScreenWidth  = tileSize * screenTiles
	ScreenHeight = tileSize
)

// Frame is a picture for the whole screen, lit where true, indexed by column
// and then row from the top left.
type Frame [ScreenWidth][ScreenHeight]bool

// Draw shows a picture in place of the base station's own screen.
func (c *Conn) Draw(frame *Frame) error {
	for _, report := range frame.tiles() {
		if err := c.handle.SetFeature(report); err != nil {
			return err
		}
	}
	return nil
}

// tiles packs a frame into the two reports that draw it.
func (f *Frame) tiles() [screenTiles][]byte {
	var reports [screenTiles][]byte
	for tile := range screenTiles {
		report := make([]byte, drawLength)
		report[0] = reportRequest
		report[1] = commandDraw
		report[2] = byte(tile * tileSize)
		report[4] = tileSize
		report[5] = tileSize
		for x := range tileSize {
			for y := range tileSize {
				if f[tile*tileSize+x][y] {
					i := x*tileSize + y
					report[tileHeader+i/8] |= 1 << (i % 8)
				}
			}
		}
		reports[tile] = report
	}
	return reports
}

// Blank draws the whole screen dark.
func (c *Conn) Blank() error { return c.Draw(&Frame{}) }

// Unblank hands the screen back to the base station.
func (c *Conn) Unblank() error { return c.Send(commandOwnScreen) }

// Read returns the next report, a reply or a notification, for a caller
// that listens rather than asks.
func (c *Conn) Read(timeout time.Duration) ([]byte, error) { return c.handle.Read(timeout) }

// Call sends a query and returns the reply, report id and command included,
// so a decoder's byte offsets match the bytes quoted beside it.
//
// The wait is bounded by time rather than by a count of reports, because the
// base station's own notifications can arrive before the answer.
func (c *Conn) Call(command byte, args ...byte) ([]byte, error) {
	if err := c.Send(command, args...); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(CallTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ErrTimeout
		}
		reply, err := c.handle.Read(remaining)
		if errors.Is(err, hidraw.ErrTimeout) {
			return nil, ErrTimeout
		}
		if err != nil {
			return nil, err
		}
		if len(reply) >= 2 && reply[0] == reportRequest && reply[1] == command {
			return reply, nil
		}
	}
}

// Claimed finds the case where the protocol interface has been taken out of
// the kernel's hands altogether.
//
// A program driving the base station through libusb detaches usbhid from
// interface 4 and binds it to usbfs, and the hidraw node for that interface
// disappears with it. What is left is the media-key node, which looks
// entirely plausible and carries nothing. Linux-Arctis-Manager does exactly
// this while it runs.
//
// Given any node of the base station, it returns the USB device node a
// claiming program would hold open, and whether any HID interface of the
// device is currently bound to usbfs.
func Claimed(node hidraw.Node) (usbPath string, claimed bool) {
	device, err := filepath.EvalSymlinks(filepath.Join("/sys/class/hidraw", filepath.Base(node.Path), "device"))
	if err != nil {
		return "", false
	}
	// …/1-1/1-1:1.3/0003:1038:12E0.0003 — the HID device, its interface, and
	// the USB device that owns both.
	usb := filepath.Dir(filepath.Dir(device))
	usbPath, ok := devicePath(usb)
	if !ok {
		return "", false
	}

	interfaces, _ := filepath.Glob(usb + "/" + filepath.Base(usb) + ":*")
	for _, iface := range interfaces {
		if readTrimmed(filepath.Join(iface, "bInterfaceClass")) != "03" {
			continue
		}
		driver, err := os.Readlink(filepath.Join(iface, "driver"))
		if err == nil && filepath.Base(driver) == "usbfs" {
			return usbPath, true
		}
	}
	return usbPath, false
}

// devicePath is the /dev/bus/usb node for a USB device's sysfs directory.
func devicePath(usb string) (string, bool) {
	bus, errB := strconv.Atoi(readTrimmed(filepath.Join(usb, "busnum")))
	dev, errD := strconv.Atoi(readTrimmed(filepath.Join(usb, "devnum")))
	if errB != nil || errD != nil {
		return "", false
	}
	return fmt.Sprintf("/dev/bus/usb/%03d/%03d", bus, dev), true
}

func readTrimmed(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

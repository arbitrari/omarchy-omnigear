// Package hidraw enumerates and opens /dev/hidraw* nodes.
//
// Everything needed to identify a node is in sysfs, so this avoids
// HIDIOCGRAWINFO ioctls entirely: /sys/class/hidraw/hidrawN/device/uevent
// carries HID_ID (bus:vendor:product), HID_NAME, HID_UNIQ and DRIVER.
package hidraw

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const sysClass = "/sys/class/hidraw"

// ErrTimeout means the read window closed with nothing to read. It is not a
// failure on its own: a device can be quiet because nothing was asked of it.
var ErrTimeout = errors.New("timed out waiting for a report")

// Link is how a device is attached to the machine. Some settings differ by
// link: a Logitech mouse offers 8000 Hz polling over a cable or its own dongle
// but only 1000 Hz over Bluetooth.
type Link string

const (
	Wired     Link = "wired"
	Wireless  Link = "wireless"
	Bluetooth Link = "bluetooth"
)

// HID bus ids, as the kernel writes them into HID_ID.
const (
	busUSB       = 0x0003
	busBluetooth = 0x0005
)

// Node is one /dev/hidrawN and what sysfs says about it.
type Node struct {
	Path    string
	Vendor  uint16
	Product uint16
	Name    string
	// Kernel driver bound to the HID device, e.g. logitech-hidpp-device.
	Driver string
	// HID_UNIQ — a serial or MAC-ish string, when the device has one.
	Uniq string
	// Bus is the HID bus id: 0x0003 USB, 0x0005 Bluetooth.
	Bus  uint16
	Link Link
}

// Enumerate returns every hidraw node currently present, in node order.
func Enumerate() []Node {
	dir, err := os.ReadDir(sysClass)
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(dir))
	for _, entry := range dir {
		names = append(names, entry.Name())
	}
	sort.Slice(names, func(i, j int) bool {
		return nodeNumber(names[i]) < nodeNumber(names[j])
	})

	nodes := make([]Node, 0, len(names))
	for _, name := range names {
		if node, ok := readNode(name); ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func nodeNumber(name string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(name, "hidraw"))
	if err != nil {
		return 1 << 30
	}
	return n
}

func readNode(name string) (Node, bool) {
	raw, err := os.ReadFile(filepath.Join(sysClass, name, "device", "uevent"))
	if err != nil {
		return Node{}, false
	}

	node := Node{Path: filepath.Join("/dev", name)}
	haveUSB := false

	for _, line := range strings.Split(string(raw), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "HID_ID":
			// HID_ID=0003:0000046D:000040BD — bus:vendor:product, hex.
			parts := strings.Split(value, ":")
			if len(parts) < 3 {
				continue
			}
			if bus, err := strconv.ParseUint(parts[0], 16, 16); err == nil {
				node.Bus = uint16(bus)
			}
			vendor, errV := strconv.ParseUint(parts[1], 16, 16)
			product, errP := strconv.ParseUint(parts[2], 16, 16)
			if errV == nil && errP == nil {
				node.Vendor = uint16(vendor)
				node.Product = uint16(product)
				haveUSB = true
			}
		case "HID_NAME":
			node.Name = value
		case "DRIVER":
			node.Driver = value
		case "HID_UNIQ":
			node.Uniq = value
		}
	}

	node.Link = readLink(name, node.Bus)
	return node, haveUSB
}

// readLink works out how a node is attached.
//
// Bluetooth announces itself in the HID bus id. Otherwise, a device behind a
// dongle hangs off a receiver in sysfs:
//
//	…/0003:046D:C54D.0008/     logitech-djreceiver   <- the dongle
//	  0003:046D:40BD.0009/     logitech-hidpp-device <- the mouse
//
// so the parent's uevent separates a dongle from a plain cable.
func readLink(name string, bus uint16) Link {
	if bus == busBluetooth {
		return Bluetooth
	}
	device, err := filepath.EvalSymlinks(filepath.Join(sysClass, name, "device"))
	if err != nil {
		return Wired
	}
	uevent, err := os.ReadFile(filepath.Join(filepath.Dir(device), "uevent"))
	if err != nil {
		return Wired
	}
	if strings.Contains(string(uevent), "receiver") {
		return Wireless
	}
	return Wired
}

// Open opens the node for reading and writing. The caller closes it.
func (n Node) Open() (*Handle, error) {
	fd, err := unix.Open(n.Path, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", n.Path, err)
	}
	return &Handle{fd: fd, path: n.Path}, nil
}

// Handle is an open hidraw node. Reads are bounded by a timeout so a device
// that never answers cannot wedge the caller.
type Handle struct {
	fd   int
	path string
}

func (h *Handle) Close() error {
	if h.fd < 0 {
		return nil
	}
	err := unix.Close(h.fd)
	h.fd = -1
	return err
}

func (h *Handle) Write(report []byte) error {
	written, err := unix.Write(h.fd, report)
	if err != nil {
		return fmt.Errorf("write %s: %w", h.path, err)
	}
	if written != len(report) {
		return fmt.Errorf("short write to %s: %d of %d bytes", h.path, written, len(report))
	}
	return nil
}

// Read waits up to timeout for one report. It returns ErrTimeout if the window
// closes first.
func (h *Handle) Read(timeout time.Duration) ([]byte, error) {
	if err := h.waitReadable(timeout); err != nil {
		return nil, err
	}
	buf := make([]byte, 64)
	n, err := unix.Read(h.fd, buf)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", h.path, err)
	}
	return buf[:n], nil
}

func (h *Handle) waitReadable(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrTimeout
		}
		fds := []unix.PollFd{{Fd: int32(h.fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(fds, int(remaining.Milliseconds()))
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return fmt.Errorf("poll %s: %w", h.path, err)
		}
		if ready == 0 {
			return ErrTimeout
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			return nil
		}
		return ErrTimeout
	}
}

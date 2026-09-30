// Package rfcomm opens RFCOMM channels to Bluetooth devices, finding the
// channel a service lives on by asking the device's SDP server.
//
// This is how a device that is not HID gets talked to. Sony headphones put
// their control protocol on a vendor RFCOMM service; the channel number is not
// fixed, so it is looked up by the service's UUID every time rather than
// written down. A WH-1000XM3 answered channel 15, and nothing promises that
// holds on the next firmware.
//
// Sockets are opened non-blocking and handed to the Go runtime's poller, so
// every read and write takes a deadline. A headset that walks out of range
// mid-conversation costs one timeout, not a hung poll.
package rfcomm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// ErrBusy means another program already holds the channel. An RFCOMM service
// takes one client at a time, and the second gets EBUSY on connect — so this
// is the Bluetooth form of "someone else has the node open".
var ErrBusy = errors.New("another program is connected to this service")

// ConnectTimeout bounds both the SDP query and the channel connect. A
// connected headset answers either in well under a second.
const ConnectTimeout = 4 * time.Second

// Conn is an open RFCOMM channel.
type Conn struct {
	file *os.File
}

func (c *Conn) Read(p []byte) (int, error)  { return c.file.Read(p) }
func (c *Conn) Write(p []byte) (int, error) { return c.file.Write(p) }
func (c *Conn) Close() error                { return c.file.Close() }

// SetReadDeadline bounds the next Read.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.file.SetReadDeadline(t) }

// DialService opens the RFCOMM channel the device publishes `uuid` on.
func DialService(address, uuid string) (*Conn, error) {
	channel, err := Channel(address, uuid)
	if err != nil {
		return nil, err
	}
	return Dial(address, channel)
}

// Dial opens an RFCOMM channel by number.
func Dial(address string, channel uint8) (*Conn, error) {
	addr, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	// The kernel wants the address little-endian. SockaddrL2 reverses it for
	// the caller; SockaddrRFCOMM copies it as given, so it is reversed here.
	var reversed [6]uint8
	for i := range addr {
		reversed[i] = addr[len(addr)-1-i]
	}

	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.BTPROTO_RFCOMM)
	if err != nil {
		return nil, fmt.Errorf("rfcomm socket: %w", err)
	}
	if err := connect(fd, &unix.SockaddrRFCOMM{Addr: reversed, Channel: channel}); err != nil {
		unix.Close(fd)
		if errors.Is(err, unix.EBUSY) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("rfcomm channel %d: %w", channel, err)
	}
	return &Conn{file: os.NewFile(uintptr(fd), fmt.Sprintf("rfcomm:%s/%d", address, channel))}, nil
}

// connect is connect(2) on a non-blocking socket, bounded by ConnectTimeout.
func connect(fd int, sa unix.Sockaddr) error {
	err := unix.Connect(fd, sa)
	if err == nil {
		return nil
	}
	if !errors.Is(err, unix.EINPROGRESS) {
		return err
	}
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
	for {
		n, err := unix.Poll(fds, int(ConnectTimeout.Milliseconds()))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("timed out connecting")
		}
		break
	}
	soErr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return err
	}
	if soErr != 0 {
		return unix.Errno(soErr)
	}
	return nil
}

func parseAddress(address string) ([6]uint8, error) {
	var out [6]uint8
	parts := strings.Split(address, ":")
	if len(parts) != 6 {
		return out, fmt.Errorf("%q is not a Bluetooth address", address)
	}
	for i, part := range parts {
		b, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return out, fmt.Errorf("%q is not a Bluetooth address", address)
		}
		out[i] = uint8(b)
	}
	return out, nil
}

// --- SDP -------------------------------------------------------------------

const (
	sdpPSM                    = 1
	sdpServiceSearchAttrReq   = 0x06
	sdpServiceSearchAttrResp  = 0x07
	attrProtocolDescriptorLst = 0x0004
	uuidRFCOMM                = 0x0003
)

// Channel asks the device's SDP server which RFCOMM channel `uuid` is on.
//
// Only the protocol descriptor list is requested, which keeps the answer to a
// few dozen bytes. A WH-1000XM3's record for its control service reads:
//
//	09 00 04  35 0c  35 03 19 01 00  35 05 19 00 03 08 0f
//	attr 4    list   L2CAP           RFCOMM, channel 0x0f
func Channel(address, uuid string) (uint8, error) {
	service, err := parseUUID(uuid)
	if err != nil {
		return 0, err
	}
	addr, err := parseAddress(address)
	if err != nil {
		return 0, err
	}

	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_SEQPACKET|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.BTPROTO_L2CAP)
	if err != nil {
		return 0, fmt.Errorf("sdp socket: %w", err)
	}
	if err := connect(fd, &unix.SockaddrL2{PSM: sdpPSM, Addr: addr}); err != nil {
		unix.Close(fd)
		return 0, fmt.Errorf("sdp: %w", err)
	}
	file := os.NewFile(uintptr(fd), "sdp:"+address)
	defer file.Close()

	var lists []byte
	continuation := []byte{0}
	for tid := uint16(1); ; tid++ {
		params := []byte{0x35, 0x11, 0x1c}
		params = append(params, service[:]...)
		params = append(params, 0xff, 0xff) // max attribute bytes
		params = append(params, 0x35, 0x03, 0x09, 0x00, attrProtocolDescriptorLst)
		params = append(params, continuation...)

		pdu := []byte{sdpServiceSearchAttrReq, byte(tid >> 8), byte(tid), byte(len(params) >> 8), byte(len(params))}
		pdu = append(pdu, params...)

		file.SetDeadline(time.Now().Add(ConnectTimeout))
		if _, err := file.Write(pdu); err != nil {
			return 0, fmt.Errorf("sdp: %w", err)
		}
		reply := make([]byte, 1024)
		n, err := file.Read(reply)
		if err != nil {
			return 0, fmt.Errorf("sdp: %w", err)
		}
		reply = reply[:n]
		if len(reply) < 7 || reply[0] != sdpServiceSearchAttrResp {
			return 0, fmt.Errorf("sdp: unexpected reply % x", reply)
		}
		count := int(binary.BigEndian.Uint16(reply[5:7]))
		if len(reply) < 7+count+1 {
			return 0, fmt.Errorf("sdp: short reply")
		}
		lists = append(lists, reply[7:7+count]...)
		continuation = reply[7+count:]
		if continuation[0] == 0 {
			break
		}
	}

	records, _, err := parseElement(lists)
	if err != nil {
		return 0, fmt.Errorf("sdp: %w", err)
	}
	for _, record := range records.children {
		if channel, ok := rfcommChannel(record); ok {
			return channel, nil
		}
	}
	return 0, fmt.Errorf("%s does not offer service %s", address, uuid)
}

// rfcommChannel pulls the channel out of one record's protocol descriptor
// list: attribute/value pairs, where the list is a sequence of per-protocol
// sequences and RFCOMM's carries its channel as the second element.
func rfcommChannel(record element) (uint8, bool) {
	pairs := record.children
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i].value != attrProtocolDescriptorLst {
			continue
		}
		for _, protocol := range pairs[i+1].children {
			if len(protocol.children) >= 2 &&
				protocol.children[0].kind == deUUID && protocol.children[0].value == uuidRFCOMM {
				return uint8(protocol.children[1].value), true
			}
		}
	}
	return 0, false
}

// SDP data element kinds, as the high five bits of the header byte give them.
const (
	deUint     = 1
	deInt      = 2
	deUUID     = 3
	deSequence = 6
	deAlt      = 7
)

// element is a parsed SDP data element. Only what finding a channel needs is
// kept: integers and short UUIDs as a number, sequences as children.
type element struct {
	kind     byte
	value    uint64
	children []element
}

func parseElement(b []byte) (element, int, error) {
	if len(b) == 0 {
		return element{}, 0, fmt.Errorf("truncated element")
	}
	kind, sizeIndex := b[0]>>3, b[0]&7
	header, size := 1, 0
	switch {
	case kind == 0:
		size = 0
	case sizeIndex <= 4:
		size = 1 << sizeIndex
	case sizeIndex == 5 && len(b) >= 2:
		header, size = 2, int(b[1])
	case sizeIndex == 6 && len(b) >= 3:
		header, size = 3, int(binary.BigEndian.Uint16(b[1:3]))
	case sizeIndex == 7 && len(b) >= 5:
		header, size = 5, int(binary.BigEndian.Uint32(b[1:5]))
	default:
		return element{}, 0, fmt.Errorf("truncated element header")
	}
	if len(b) < header+size {
		return element{}, 0, fmt.Errorf("truncated element body")
	}
	body := b[header : header+size]
	out := element{kind: kind}

	switch kind {
	case deUint, deInt, deUUID:
		// A 128-bit UUID does not fit and is never what is being looked for
		// here; it is left as zero.
		if size <= 8 {
			for _, c := range body {
				out.value = out.value<<8 | uint64(c)
			}
		}
	case deSequence, deAlt:
		for rest := body; len(rest) > 0; {
			child, used, err := parseElement(rest)
			if err != nil {
				return element{}, 0, err
			}
			out.children = append(out.children, child)
			rest = rest[used:]
		}
	}
	return out, header + size, nil
}

func parseUUID(uuid string) ([16]byte, error) {
	var out [16]byte
	hex := strings.ReplaceAll(uuid, "-", "")
	if len(hex) != 32 {
		return out, fmt.Errorf("%q is not a 128-bit UUID", uuid)
	}
	for i := range out {
		b, err := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		if err != nil {
			return out, fmt.Errorf("%q is not a 128-bit UUID", uuid)
		}
		out[i] = byte(b)
	}
	return out, nil
}

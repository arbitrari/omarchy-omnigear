// Package mdr speaks Sony's headphone control protocol, as carried over an
// RFCOMM channel.
//
// It is not documented anywhere public. What is here was read off a
// WH-1000XM3 on firmware 4.5.2, and each decoder in drivers/sony carries the
// bytes it was read from.
//
// A frame on the wire:
//
//	3E  type  seq  len(4, big-endian)  payload…  checksum  3C
//
// The checksum is the low byte of the sum of everything between the markers.
// The three marker bytes 3C 3D 3E are escaped inside a frame as 3D followed by
// the byte with bit 4 cleared, so a payload can never end a frame early.
//
// Every data frame is acknowledged with an empty frame of type 01 carrying the
// other sequence number, in both directions. The headset's acknowledgement
// tells the host which sequence number to send with next; the host must
// acknowledge the headset's replies and notifications, or it repeats them.
package mdr

import (
	"errors"
	"fmt"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/rfcomm"
)

// ServiceUUID is the RFCOMM service the XM3 generation publishes its control
// protocol on. Later models publish a different one.
const ServiceUUID = "96cc203e-5068-46ad-b32d-e316f5e069ba"

const (
	frameStart  = 0x3E
	frameEscape = 0x3D
	frameEnd    = 0x3C

	typeAck  = 0x01
	typeData = 0x0C
)

// CallTimeout bounds one request. A connected XM3 answers in tens of
// milliseconds; this is generous because the radio is shared with audio.
const CallTimeout = 1500 * time.Millisecond

// ErrTimeout is a request the headset never answered.
var ErrTimeout = errors.New("headset did not answer")

// Conn is an open control channel to one headset.
type Conn struct {
	link *rfcomm.Conn
	seq  byte
	buf  []byte
}

// Open connects to the headset's control service and performs the init
// exchange, which the headset requires before it will answer anything else.
//
// A missed init is retried once on a fresh channel. Straight after a mode
// change the headset is busy announcing it, and one init in a dozen or so went
// unanswered there while the next connection was answered at once.
func Open(address string) (*Conn, error) {
	c, err := open(address)
	if errors.Is(err, ErrTimeout) {
		return open(address)
	}
	return c, err
}

func open(address string) (*Conn, error) {
	link, err := rfcomm.DialService(address, ServiceUUID)
	if err != nil {
		return nil, err
	}
	c := &Conn{link: link}
	// 00 00 → 01 00 40 10. The reply's bytes are the protocol version the
	// headset speaks; nothing here depends on them yet.
	if _, err := c.Call(0x01, 0x00, 0x00); err != nil {
		link.Close()
		return nil, fmt.Errorf("init: %w", err)
	}
	return c, nil
}

func (c *Conn) Close() error { return c.link.Close() }

// Call sends a request and returns the reply whose first byte is `want` and
// whose second matches the request's — the headset answers a GET of 66 02
// with 67 02, and 66 01 with 67 01, so both are needed to tell replies apart.
//
// Anything else arriving meanwhile is a notification: acknowledged, and
// dropped.
func (c *Conn) Call(want byte, payload ...byte) ([]byte, error) {
	if err := c.send(typeData, c.seq, payload); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(CallTimeout)
	acked := false
	var reply []byte
	for !(acked && reply != nil) {
		kind, seq, body, err := c.receive(deadline)
		if err != nil {
			return nil, err
		}
		if kind == typeAck {
			c.seq = seq
			acked = true
			continue
		}
		c.send(typeAck, 1-seq, nil)
		if len(body) >= 1 && body[0] == want &&
			(len(payload) < 2 || (len(body) >= 2 && body[1] == payload[1])) {
			reply = body
		}
	}
	return reply, nil
}

// Send issues a request that has no reply of its own, only the
// acknowledgement. Setting a value is like this: the headset takes it and
// says nothing more, beyond a notification of the new state.
func (c *Conn) Send(payload ...byte) error {
	if err := c.send(typeData, c.seq, payload); err != nil {
		return err
	}
	deadline := time.Now().Add(CallTimeout)
	for {
		kind, seq, _, err := c.receive(deadline)
		if err != nil {
			return err
		}
		if kind == typeAck {
			c.seq = seq
			return nil
		}
		c.send(typeAck, 1-seq, nil)
	}
}

func (c *Conn) send(kind, seq byte, payload []byte) error {
	body := []byte{kind, seq,
		byte(len(payload) >> 24), byte(len(payload) >> 16), byte(len(payload) >> 8), byte(len(payload))}
	body = append(body, payload...)
	var sum byte
	for _, b := range body {
		sum += b
	}
	body = append(body, sum)

	frame := []byte{frameStart}
	for _, b := range body {
		if b == frameStart || b == frameEscape || b == frameEnd {
			frame = append(frame, frameEscape, b&^0x10)
		} else {
			frame = append(frame, b)
		}
	}
	frame = append(frame, frameEnd)

	_, err := c.link.Write(frame)
	return err
}

// receive returns the next whole frame.
func (c *Conn) receive(deadline time.Time) (kind, seq byte, payload []byte, err error) {
	for {
		if frame, ok := c.nextFrame(); ok {
			return decode(frame)
		}
		c.link.SetReadDeadline(deadline)
		chunk := make([]byte, 1024)
		n, err := c.link.Read(chunk)
		if err != nil {
			if time.Now().After(deadline) {
				return 0, 0, nil, ErrTimeout
			}
			return 0, 0, nil, err
		}
		c.buf = append(c.buf, chunk[:n]...)
	}
}

// nextFrame takes one frame off the buffer, still escaped, without markers.
func (c *Conn) nextFrame() ([]byte, bool) {
	for i, b := range c.buf {
		if b != frameEnd {
			continue
		}
		raw := c.buf[:i]
		c.buf = c.buf[i+1:]
		for j, r := range raw {
			if r == frameStart {
				return raw[j+1:], true
			}
		}
		// An end with no start is line noise; drop it and keep looking.
		return c.nextFrame()
	}
	return nil, false
}

func decode(escaped []byte) (kind, seq byte, payload []byte, err error) {
	frame := make([]byte, 0, len(escaped))
	for i := 0; i < len(escaped); i++ {
		if escaped[i] == frameEscape && i+1 < len(escaped) {
			i++
			frame = append(frame, escaped[i]|0x10)
			continue
		}
		frame = append(frame, escaped[i])
	}
	if len(frame) < 7 {
		return 0, 0, nil, fmt.Errorf("short frame % x", frame)
	}
	length := int(frame[2])<<24 | int(frame[3])<<16 | int(frame[4])<<8 | int(frame[5])
	if len(frame) != 6+length+1 {
		return 0, 0, nil, fmt.Errorf("frame length %d does not match % x", length, frame)
	}
	var sum byte
	for _, b := range frame[:len(frame)-1] {
		sum += b
	}
	if sum != frame[len(frame)-1] {
		return 0, 0, nil, fmt.Errorf("bad checksum in % x", frame)
	}
	return frame[0], frame[1], frame[6 : 6+length], nil
}

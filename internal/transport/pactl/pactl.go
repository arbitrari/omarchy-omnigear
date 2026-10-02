// Package pactl talks to the sound server: a Bluetooth audio device's card
// profile, and the outputs and streams Sonar's channels are made of.
//
// A headset's codec is not the headset's to choose. The two ends offer what
// they support and the host picks one, and on a PipeWire system that choice is
// the card profile: a2dp-sink-aac is AAC, a2dp-sink-sbc is SBC, and so on. So
// switching codec is a conversation with PipeWire, not with the headset.
//
// pactl is used rather than a native client because it is already on any
// machine with PipeWire's pulse layer, speaks JSON, and is a short-lived
// process like the rest of this CLI.
package pactl

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Timeout bounds one pactl run. A profile switch renegotiates the Bluetooth
// link, which takes about a second.
const Timeout = 5 * time.Second

// Card is a Bluetooth device's card, as the sound server has it.
type Card struct {
	Name     string
	Active   string
	Profiles []Profile
}

// Profile is one way the card can be used.
type Profile struct {
	Name        string
	Description string
	Available   bool
}

// CardFor finds the card for a Bluetooth address. PipeWire names it after the
// address: bluez_card.38_18_4C_6D_BA_69.
func CardFor(address string) (*Card, error) {
	want := "bluez_card." + strings.ReplaceAll(strings.ToUpper(address), ":", "_")

	out, err := run("-f", "json", "list", "cards")
	if err != nil {
		return nil, err
	}
	var cards []struct {
		Name          string `json:"name"`
		ActiveProfile string `json:"active_profile"`
		Profiles      map[string]struct {
			Description string `json:"description"`
			Available   bool   `json:"available"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(out, &cards); err != nil {
		return nil, fmt.Errorf("pactl: %w", err)
	}
	for _, c := range cards {
		if c.Name != want {
			continue
		}
		card := &Card{Name: c.Name, Active: c.ActiveProfile}
		for name, p := range c.Profiles {
			card.Profiles = append(card.Profiles, Profile{Name: name, Description: p.Description, Available: p.Available})
		}
		return card, nil
	}
	return nil, fmt.Errorf("the sound server has no card for %s", address)
}

// SetProfile switches a card's profile, and waits until the sound server says
// it has.
//
// set-card-profile returns before the switch lands. Reading back straight
// afterwards sometimes found the new profile and sometimes the old one, so a
// verifying read reported every other codec switch as refused — naming, each
// time, the codec asked for one switch earlier.
func SetProfile(address, profile string) error {
	card, err := CardFor(address)
	if err != nil {
		return err
	}
	if _, err := run("set-card-profile", card.Name, profile); err != nil {
		return err
	}
	deadline := time.Now().Add(Timeout)
	for time.Now().Before(deadline) {
		card, err := CardFor(address)
		if err == nil && card.Active == profile {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("the sound server did not switch to %s", profile)
}

var codecPattern = regexp.MustCompile(`codec ([^)]+)\)`)

// Codec is the codec a profile carries, as the sound server names it in the
// description — "High Fidelity Playback (A2DP Sink, codec aptX HD)" — or ""
// for a profile that plays no audio to the device. The name is the only place
// it is said: the default profile is plain "a2dp-sink", whichever codec that
// turns out to be.
func (p Profile) Codec() string {
	if !strings.HasPrefix(p.Name, "a2dp-sink") {
		return ""
	}
	match := codecPattern.FindStringSubmatch(p.Description)
	if match == nil {
		return ""
	}
	return match[1]
}

func run(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pactl", args...).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("pactl: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("pactl: %w", err)
	}
	return out, nil
}

// Sink is an output the sound server offers.
type Sink struct {
	Name string
	// Vendor and Product are the USB ids of the sound card behind it, zero
	// for a sink that is not a USB device.
	Vendor  uint16
	Product uint16
	// Volume is the louder channel's, in percent.
	Volume int
}

// Sinks lists every output the sound server has.
func Sinks() ([]Sink, error) {
	out, err := run("-f", "json", "list", "sinks")
	if err != nil {
		return nil, err
	}
	var sinks []struct {
		Name       string            `json:"name"`
		Properties map[string]string `json:"properties"`
		Volume     map[string]struct {
			Percent string `json:"value_percent"`
		} `json:"volume"`
	}
	if err := json.Unmarshal(out, &sinks); err != nil {
		return nil, fmt.Errorf("pactl: %w", err)
	}
	found := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		sink := Sink{Name: s.Name}
		// "75%", per channel. The louder one is what a mixer's single
		// slider shows.
		for _, channel := range s.Volume {
			if v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(channel.Percent), "%")); err == nil && v > sink.Volume {
				sink.Volume = v
			}
		}
		// "0x1038", as ALSA writes it.
		if v, err := strconv.ParseUint(strings.TrimPrefix(s.Properties["device.vendor.id"], "0x"), 16, 16); err == nil {
			sink.Vendor = uint16(v)
		}
		if p, err := strconv.ParseUint(strings.TrimPrefix(s.Properties["device.product.id"], "0x"), 16, 16); err == nil {
			sink.Product = uint16(p)
		}
		found = append(found, sink)
	}
	return found, nil
}

// DefaultSink is the output new streams play to.
func DefaultSink() (string, error) {
	out, err := run("get-default-sink")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func SetDefaultSink(name string) error {
	_, err := run("set-default-sink", name)
	return err
}

// SetSinkVolume sets an output's volume, in percent, on every channel.
func SetSinkVolume(name string, percent int) error {
	_, err := run("set-sink-volume", name, fmt.Sprintf("%d%%", percent))
	return err
}

// SetStreamVolume sets the volume of the playback stream with a node name,
// in percent. A stream rather than a sink: a loopback's sink volume is the
// listener's to set, and its stream volume is free for something else to
// scale on top of it.
func SetStreamVolume(nodeName string, percent int) error {
	out, err := run("-f", "json", "list", "sink-inputs")
	if err != nil {
		return err
	}
	var inputs []struct {
		Index      int               `json:"index"`
		Properties map[string]string `json:"properties"`
	}
	if err := json.Unmarshal(out, &inputs); err != nil {
		return fmt.Errorf("pactl: %w", err)
	}
	for _, input := range inputs {
		if input.Properties["node.name"] == nodeName {
			_, err := run("set-sink-input-volume", strconv.Itoa(input.Index), fmt.Sprintf("%d%%", percent))
			return err
		}
	}
	return fmt.Errorf("the sound server has no stream %s", nodeName)
}

// Stream is something playing: an application's output, and the sink it
// plays to.
type Stream struct {
	Index int
	// App is the application's name, as it gave it. Empty for a stream that
	// did not say.
	App string
	// Node is the stream's node name, which for a loopback is its own.
	Node string
	Sink string
}

// Streams lists everything playing, with each one's sink by name.
func Streams() ([]Stream, error) {
	sinks, err := sinkNames()
	if err != nil {
		return nil, err
	}
	out, err := run("-f", "json", "list", "sink-inputs")
	if err != nil {
		return nil, err
	}
	var inputs []struct {
		Index      int               `json:"index"`
		Sink       int               `json:"sink"`
		Properties map[string]string `json:"properties"`
	}
	if err := json.Unmarshal(out, &inputs); err != nil {
		return nil, fmt.Errorf("pactl: %w", err)
	}
	streams := make([]Stream, 0, len(inputs))
	for _, input := range inputs {
		streams = append(streams, Stream{
			Index: input.Index,
			App:   input.Properties["application.name"],
			Node:  input.Properties["node.name"],
			Sink:  sinks[input.Sink],
		})
	}
	return streams, nil
}

// MoveStream moves a stream to another sink. WirePlumber remembers the
// choice for the application, so its next stream goes there too.
//
// The move is asynchronous: the stream was still on its old sink when read
// straight afterwards. The caller waits for it if it needs to.
func MoveStream(index int, sink string) error {
	_, err := run("move-sink-input", strconv.Itoa(index), sink)
	return err
}

// sinkNames maps a sink's index to its name.
func sinkNames() (map[int]string, error) {
	out, err := run("-f", "json", "list", "sinks")
	if err != nil {
		return nil, err
	}
	var sinks []struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(out, &sinks); err != nil {
		return nil, fmt.Errorf("pactl: %w", err)
	}
	names := make(map[int]string, len(sinks))
	for _, s := range sinks {
		names[s.Index] = s.Name
	}
	return names, nil
}

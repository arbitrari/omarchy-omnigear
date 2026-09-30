// Package pactl reads and sets a Bluetooth audio device's card profile
// through the sound server.
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

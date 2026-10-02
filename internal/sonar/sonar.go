// Package sonar gives a headset separate outputs for games, chat, media and
// everything else, the way SteelSeries Sonar does, and lets the headset's
// ChatMix dial balance the first two.
//
// Each channel is a PipeWire loopback: a virtual sink applications can be
// pointed at, whose output plays into the headset. They are declared in a
// PipeWire drop-in rather than created by this program, so PipeWire makes
// them at login whether OmniGear is running or not, and an application keeps
// the channel it was moved to across reboots.
//
// The dial scales a channel's loopback *stream*, not its sink. The sink's
// volume is the listener's own, set in any mixer; the dial works on top of
// it, so turning it toward Game quietens chat without touching the level
// chat was set to. WirePlumber restores stream volumes by node name across a
// restart, which matters because the base station only reports the dial
// when it moves: the last position applied is the one that stays.
package sonar

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/pactl"
)

// Channel is one virtual output.
type Channel struct {
	Slug  string
	Label string
}

// Channels are Sonar's outputs, in its order. Game and Chat are the two the
// dial balances; Media and Aux it leaves alone.
var Channels = []Channel{
	{"game", "Game"},
	{"chat", "Chat"},
	{"media", "Media"},
	{"aux", "Aux"},
}

// SinkName is the node a channel's applications play to.
func (c Channel) SinkName() string { return "omnigear_" + c.Slug }

// streamName is the loopback's playback into the headset, which the dial
// scales.
func (c Channel) streamName() string { return "omnigear_" + c.Slug + "_out" }

// Description is how the channel is listed in a sound menu.
func (c Channel) Description() string { return "OmniGear " + c.Label }

// restartTimeout bounds the wait for PipeWire to come back after a restart.
// It took about three seconds here.
const restartTimeout = 10 * time.Second

// ConfigPath is the drop-in that declares the channels. Its presence is what
// "enabled" means.
func ConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "pipewire", "pipewire.conf.d", "omnigear-sonar.conf")
}

// Status is whether the channels are configured, and whether the sound
// server actually has them.
type Status struct {
	Enabled bool
	// Live is false when the drop-in exists but PipeWire has not made the
	// channels — it has not been restarted since, or the headset the
	// loopbacks target is not plugged in.
	Live bool
}

func Read() (Status, error) {
	_, err := os.Stat(ConfigPath())
	if errors.Is(err, fs.ErrNotExist) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	sinks, err := pactl.Sinks()
	if err != nil {
		return Status{Enabled: true}, err
	}
	return Status{Enabled: true, Live: hasChannels(sinks)}, nil
}

func hasChannels(sinks []pactl.Sink) bool {
	names := map[string]bool{}
	for _, sink := range sinks {
		names[sink.Name] = true
	}
	for _, c := range Channels {
		if !names[c.SinkName()] {
			return false
		}
	}
	return true
}

// SinkFor finds the headset's own output by the USB ids of its sound card,
// which are the same ids the catalog knows it by.
func SinkFor(vendor, product uint16) (string, error) {
	sinks, err := pactl.Sinks()
	if err != nil {
		return "", err
	}
	for _, sink := range sinks {
		if sink.Vendor == vendor && sink.Product == product {
			return sink.Name, nil
		}
	}
	return "", fmt.Errorf("the sound server has no output for %04x:%04x", vendor, product)
}

// Enable writes the drop-in for channels playing into target, restarts
// PipeWire to load it, and makes Game the default output if the headset
// was — which is what Sonar does, and means everything plays somewhere
// sensible until it is moved to another channel.
//
// The restart interrupts every stream for a moment. There is no way around
// it: a drop-in is only read at startup.
func Enable(target string) error {
	wasDefault := false
	if current, err := pactl.DefaultSink(); err == nil && current == target {
		wasDefault = true
	}

	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(config(target)), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := restart(); err != nil {
		return err
	}
	if err := awaitRestart(target, true); err != nil {
		return err
	}
	if wasDefault {
		if err := pactl.SetDefaultSink(Channels[0].SinkName()); err != nil {
			return fmt.Errorf("making Game the default output: %w", err)
		}
	}
	return nil
}

// Disable removes the drop-in and restarts PipeWire without the channels.
// Default output goes back to the headset first, if it was on a channel,
// so nothing is left playing to a sink that is about to vanish.
func Disable(target string) error {
	if current, err := pactl.DefaultSink(); err == nil && strings.HasPrefix(current, "omnigear_") {
		_ = pactl.SetDefaultSink(target)
	}
	if err := os.Remove(ConfigPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := restart(); err != nil {
		return err
	}
	return awaitRestart(target, false)
}

// ApplyMix scales Game and Chat to the dial's levels, each 0–100.
func ApplyMix(game, chat uint8) error {
	if err := pactl.SetStreamVolume(Channels[0].streamName(), int(game)); err != nil {
		return err
	}
	return pactl.SetStreamVolume(Channels[1].streamName(), int(chat))
}

// config is the drop-in.
//
// The loopback's playback side is passive, so a channel with nothing playing
// does not hold the headset's output open, and will not fall back: were the
// headset unplugged with Game the default output, a fallback would have the
// loopback play into the default — itself.
func config(target string) string {
	var b strings.Builder
	b.WriteString("# Written by OmniGear: Sonar channels playing into the headset.\n")
	b.WriteString("# Turn Sonar off in OmniGear to remove it.\n")
	b.WriteString("context.modules = [\n")
	for _, c := range Channels {
		fmt.Fprintf(&b, `  { name = libpipewire-module-loopback
    args = {
      node.description = "%s"
      capture.props = {
        node.name = "%s"
        media.class = "Audio/Sink"
        audio.position = [ FL FR ]
      }
      playback.props = {
        node.name = "%s"
        node.description = "%s Output"
        target.object = "%s"
        node.dont-fallback = true
        node.passive = true
        stream.dont-remix = true
        audio.position = [ FL FR ]
      }
    }
  }
`, c.Description(), c.SinkName(), c.streamName(), c.Description(), target)
	}
	b.WriteString("]\n")
	return b.String()
}

func restart() error {
	out, err := exec.Command("systemctl", "--user", "restart",
		"pipewire.service", "pipewire-pulse.service", "wireplumber.service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("restarting PipeWire: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// awaitRestart waits for the sound server to come back whole: the headset's
// own output there, and the channels present or absent as asked.
//
// Both halves matter. The loopbacks are PipeWire's own and appear at once,
// while the headset's output waits on WirePlumber finding the sound card,
// which comes a second or so later. Returning on the channels alone hands
// back a sound server that is half up, and the next thing asked of it fails
// with "Not supported".
func awaitRestart(target string, present bool) error {
	deadline := time.Now().Add(restartTimeout)
	for time.Now().Before(deadline) {
		if sinks, err := pactl.Sinks(); err == nil && hasSink(sinks, target) &&
			hasChannels(sinks) == present {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	switch {
	case present:
		return errors.New("PipeWire restarted without the Sonar channels, or without the headset")
	default:
		return errors.New("PipeWire restarted with the Sonar channels still present, or without the headset")
	}
}

func hasSink(sinks []pactl.Sink, name string) bool {
	for _, sink := range sinks {
		if sink.Name == name {
			return true
		}
	}
	return false
}

// App is an application playing sound, and the channel it plays to.
type App struct {
	Name string
	// Channel is a channel's slug, or empty when the app plays somewhere
	// that is not a channel — straight to the headset, say.
	Channel string
}

// moveTimeout bounds the wait for a moved stream to land. A move is
// asynchronous, and the stream was still on its old sink when read straight
// afterwards.
const moveTimeout = 2 * time.Second

// Apps lists what is playing, one entry per application.
//
// By application rather than by stream: a music player can open a new
// stream for every track, and WirePlumber remembers an output per
// application name anyway, so the application is what a choice applies to.
// The channels' own loopbacks are left out; they are plumbing, not apps.
func Apps() ([]App, error) {
	streams, err := pactl.Streams()
	if err != nil {
		return nil, err
	}
	var apps []App
	seen := map[string]bool{}
	for _, stream := range streams {
		if stream.App == "" || strings.HasPrefix(stream.Node, "omnigear_") || seen[stream.App] {
			continue
		}
		seen[stream.App] = true
		apps = append(apps, App{Name: stream.App, Channel: channelOf(stream.Sink)})
	}
	return apps, nil
}

func channelOf(sink string) string {
	for _, c := range Channels {
		if c.SinkName() == sink {
			return c.Slug
		}
	}
	return ""
}

// MoveApp moves every stream an application has to a channel, and waits
// until they are there.
func MoveApp(name, slug string) error {
	var target string
	for _, c := range Channels {
		if c.Slug == slug {
			target = c.SinkName()
		}
	}
	if target == "" {
		return fmt.Errorf("%q is not a Sonar channel", slug)
	}

	streams, err := pactl.Streams()
	if err != nil {
		return err
	}
	found, moved := false, 0
	for _, stream := range streams {
		if stream.App != name {
			continue
		}
		found = true
		if stream.Sink == target {
			continue
		}
		if err := pactl.MoveStream(stream.Index, target); err != nil {
			return err
		}
		moved++
	}
	if !found {
		// Stopped since the panel last looked. WirePlumber has nothing to
		// move, and would not remember a choice for it.
		return fmt.Errorf("%s is not playing anything", name)
	}
	if moved == 0 {
		return nil
	}

	deadline := time.Now().Add(moveTimeout)
	for time.Now().Before(deadline) {
		if apps, err := Apps(); err == nil {
			for _, app := range apps {
				if app.Name == name && app.Channel == slug {
					return nil
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

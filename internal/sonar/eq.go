package sonar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Equalizing is done here rather than in the headset. With Sonar on, the
// base station hands its equalizer to the computer — its own menu says the
// EQ is "on Sonar" and will not change it — so each channel carries its own,
// as SteelSeries Sonar does: bass on Game, clarity on Chat.
//
// Each channel is a PipeWire filter-chain of ten peaking filters, an octave
// apart. A gain can be changed while it runs, with no restart. What it cannot
// do is report a change reliably: one made before the chain has played
// anything since PipeWire started is held and applied the moment audio
// starts, but reads back as the old value until then. So the gains are kept
// in a state file, which is what a read reports and what the drop-in is
// written from, and the live chain is told as well.

// Bands are the equalizer's centre frequencies, in Hz, low to high.
var Bands = []int{32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// BandSlug names a band: "32", "1k".
func BandSlug(freq int) string {
	if freq >= 1000 {
		return fmt.Sprintf("%dk", freq/1000)
	}
	return fmt.Sprint(freq)
}

// Gain limits, in dB.
const (
	MinGain = -12
	MaxGain = 12
)

// bandQ is an octave-wide peak, so neighbouring bands meet rather than
// leaving gaps or piling up.
const bandQ = 1.41

// Preset is a named curve. Only Custom keeps bands of its own; changing a
// band on any other preset carries its curve over to Custom first.
type Preset struct {
	Slug  string
	Label string
	Gains []int
}

// Presets are the curves offered, Flat first. Custom's gains are the
// listener's and are kept per channel.
var Presets = []Preset{
	{"off", "Flat", []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	{"bass-boost", "Bass Boost", []int{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}},
	{"treble-boost", "Treble Boost", []int{0, 0, 0, 0, 0, 0, 2, 4, 5, 6}},
	{"vocal", "Vocal", []int{-3, -2, -1, 0, 2, 3, 3, 2, 0, -1}},
	{"manual", "Custom", nil},
}

// customPreset is the one preset with bands of its own.
const customPreset = "manual"

// ChannelEQ is one channel's equalizer.
type ChannelEQ struct {
	Preset string `json:"preset"`
	// Custom is the Custom preset's curve, kept while another is selected.
	Custom []int `json:"custom"`
}

// Gains is the curve the channel plays with now.
func (eq ChannelEQ) Gains() []int {
	if eq.Preset != customPreset {
		for _, p := range Presets {
			if p.Slug == eq.Preset {
				return p.Gains
			}
		}
	}
	gains := make([]int, len(Bands))
	copy(gains, eq.Custom)
	return gains
}

// EQState is every channel's equalizer, by channel slug.
type EQState map[string]ChannelEQ

func eqPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "omnigear", "sonar-eq.json")
}

// ReadEQ returns every channel's equalizer, flat for any never set.
func ReadEQ() (EQState, error) {
	state := EQState{}
	raw, err := os.ReadFile(eqPath())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, fmt.Errorf("%s: %w", eqPath(), err)
		}
	}
	for _, c := range Channels {
		if _, ok := state[c.Slug]; !ok {
			state[c.Slug] = ChannelEQ{Preset: Presets[0].Slug}
		}
	}
	return state, nil
}

func writeEQ(state EQState) error {
	path := eqPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// SetPreset selects a preset for a channel.
func SetPreset(channel, preset string) error {
	known := false
	for _, p := range Presets {
		known = known || p.Slug == preset
	}
	if !known {
		return fmt.Errorf("%q is not an equalizer preset", preset)
	}
	return changeEQ(channel, func(eq *ChannelEQ) {
		eq.Preset = preset
	})
}

// SetBand sets one band of a channel, in dB. On a preset other than Custom,
// the preset's curve moves to Custom first and the band is changed there, so
// the rest of the curve is kept rather than reset.
func SetBand(channel, band string, gain int) error {
	index := -1
	for i, freq := range Bands {
		if BandSlug(freq) == band {
			index = i
		}
	}
	if index < 0 {
		return fmt.Errorf("%q is not an equalizer band", band)
	}
	gain = min(max(gain, MinGain), MaxGain)
	return changeEQ(channel, func(eq *ChannelEQ) {
		if eq.Preset != customPreset {
			eq.Custom = eq.Gains()
			eq.Preset = customPreset
		}
		custom := make([]int, len(Bands))
		copy(custom, eq.Custom)
		custom[index] = gain
		eq.Custom = custom
	})
}

// changeEQ applies a change to one channel's equalizer: saved, written into
// the drop-in for the next time PipeWire starts, and sent to the running
// chain. The drop-in is rewritten without a restart; the live chain takes the
// change as it plays.
func changeEQ(channel string, change func(*ChannelEQ)) error {
	var ch *Channel
	for i := range Channels {
		if Channels[i].Slug == channel {
			ch = &Channels[i]
		}
	}
	if ch == nil {
		return fmt.Errorf("%q is not a Sonar channel", channel)
	}
	state, err := ReadEQ()
	if err != nil {
		return err
	}
	eq := state[channel]
	change(&eq)
	state[channel] = eq
	if err := writeEQ(state); err != nil {
		return err
	}

	// Only while Sonar is on: otherwise there is no drop-in to rewrite and no
	// chain to tell, and the saved curve is applied when it is turned on.
	raw, err := os.ReadFile(ConfigPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	target, ok := targetOf(string(raw))
	if !ok {
		return fmt.Errorf("%s names no headset output", ConfigPath())
	}
	if err := os.WriteFile(ConfigPath(), []byte(config(target, state)), 0o644); err != nil {
		return err
	}
	return applyLive(*ch, eq.Gains())
}

// targetOf finds the headset output a drop-in plays into.
func targetOf(config string) (string, bool) {
	_, rest, found := strings.Cut(config, `target.object = "`)
	if !found {
		return "", false
	}
	target, _, found := strings.Cut(rest, `"`)
	return target, found && target != ""
}

// applyLive sends a curve to a channel's running chain.
func applyLive(ch Channel, gains []int) error {
	id, err := nodeID(ch.SinkName())
	if err != nil {
		return err
	}
	var params []string
	for i, gain := range gains {
		params = append(params, fmt.Sprintf(`"band%d:Gain" %d.0`, i+1, gain))
	}
	pod := "{ params = [ " + strings.Join(params, " ") + " ] }"
	out, err := exec.Command("pw-cli", "set-param", fmt.Sprint(id), "Props", pod).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pw-cli: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// nodeID finds a node's id by name. pactl does not reach a filter's
// controls, so this asks PipeWire itself.
func nodeID(name string) (int, error) {
	out, err := exec.Command("pw-dump").Output()
	if err != nil {
		return 0, fmt.Errorf("pw-dump: %w", err)
	}
	var objects []struct {
		ID   int `json:"id"`
		Info struct {
			Props map[string]any `json:"props"`
		} `json:"info"`
	}
	if err := json.Unmarshal(out, &objects); err != nil {
		return 0, fmt.Errorf("pw-dump: %w", err)
	}
	for _, o := range objects {
		if o.Info.Props["node.name"] == name {
			return o.ID, nil
		}
	}
	return 0, fmt.Errorf("PipeWire has no node %s", name)
}

// chain is one channel's filter-chain in the drop-in.
func chain(c Channel, target string, gains []int) string {
	var nodes, links strings.Builder
	for i, freq := range Bands {
		fmt.Fprintf(&nodes, "          { type = builtin name = band%d label = bq_peaking "+
			"control = { \"Freq\" = %d.0 \"Q\" = %.2f \"Gain\" = %d.0 } }\n", i+1, freq, bandQ, gains[i])
		if i > 0 {
			fmt.Fprintf(&links, "          { output = \"band%d:Out\" input = \"band%d:In\" }\n", i, i+1)
		}
	}
	return fmt.Sprintf(`  { name = libpipewire-module-filter-chain
    args = {
      node.description = "%s"
      audio.channels = 2
      audio.position = [ FL FR ]
      filter.graph = {
        nodes = [
%s        ]
        links = [
%s        ]
      }
      capture.props = {
        node.name = "%s"
        media.class = "Audio/Sink"
      }
      playback.props = {
        node.name = "%s"
        node.description = "%s Output"
        target.object = "%s"
        node.dont-fallback = true
        node.passive = true
        stream.dont-remix = true
      }
    }
  }
`, c.Description(), nodes.String(), links.String(), c.SinkName(), c.streamName(), c.Description(), target)
}

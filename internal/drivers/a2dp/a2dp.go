// Package a2dp reads and switches the codec a Bluetooth headset plays over.
//
// Not a driver in its own right: any headset's driver can hand its codec
// capability to this, because the codec is chosen by the host's sound server
// rather than stored in the headset. Nothing here says a word to the device.
package a2dp

import (
	"fmt"
	"sort"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/mpris"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/pactl"
)

// Read describes the codecs the card offers and which one is playing.
func Read(address string) (*model.Codec, error) {
	card, err := pactl.CardFor(address)
	if err != nil {
		return nil, err
	}

	codec := &model.Codec{Options: []model.CodecOption{}}
	seen := map[string]bool{}
	for _, profile := range card.Profiles {
		label := profile.Codec()
		slug := model.CodecSlug(label)
		if label == "" || !profile.Available || seen[slug] {
			continue
		}
		seen[slug] = true
		codec.Options = append(codec.Options, model.CodecOption{Slug: slug, Label: label})
		if profile.Name == card.Active {
			codec.Current = slug
		}
	}
	// Best last, the way the sound server ranks them, so a list reads from
	// the most compatible to the highest quality.
	sort.SliceStable(codec.Options, func(i, j int) bool {
		return rank(codec.Options[i].Slug) < rank(codec.Options[j].Slug)
	})
	return codec, nil
}

// resumeWindow is how long after a switch a paused player is still taken to
// have been paused by it, and brought back. The pause was seen half a second
// in; the rest is room for a Play the player ignores.
const resumeWindow = 3 * time.Second

// Write switches the card to the profile that carries a codec.
//
// The switch takes the audio output away for a moment and players pause when
// that happens, so whatever was playing is resumed once the new profile is
// in place.
func Write(address, slug string) error {
	card, err := pactl.CardFor(address)
	if err != nil {
		return err
	}
	for _, profile := range card.Profiles {
		if profile.Available && model.CodecSlug(profile.Codec()) == slug {
			playing := mpris.Playing()
			if err := pactl.SetProfile(address, profile.Name); err != nil {
				return err
			}
			mpris.Resume(playing, resumeWindow)
			return nil
		}
	}
	return fmt.Errorf("the sound server offers no %s profile for this headset", slug)
}

func rank(slug string) int {
	value, ok := model.CodecValue(slug)
	if !ok {
		return 1 << 30
	}
	return int(value)
}

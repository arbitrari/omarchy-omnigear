package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/arbitrari/omarchy-omnigear/internal/catalog"
	"github.com/arbitrari/omarchy-omnigear/internal/discovery"
	"github.com/arbitrari/omarchy-omnigear/internal/model"
	"github.com/arbitrari/omarchy-omnigear/internal/sonar"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/hidraw"
)

// chatMixEvent is one line of `omnigear chatmix`.
type chatMixEvent struct {
	ID   string `json:"id"`
	Game uint8  `json:"game"`
	Chat uint8  `json:"chat"`
	// Error is why the mix could not be applied, when it could not. The
	// reading is still good.
	Error string `json:"error,omitempty"`
}

// rescan is how long to wait before looking for a ChatMix device again, when
// there is none or the one there was went away.
const rescan = 5 * time.Second

// cmdChatMix follows a headset's ChatMix dial and applies it to the Sonar
// channels, for as long as it runs.
//
// It is the one command that does not print a single object and exit. The
// base station reports the dial only as it turns, so something has to be
// listening at the time; the bar keeps this running and reads one JSON
// object per line, one per movement.
//
// Finding the device is deliberately cheaper than `list`: it matches nodes
// by USB id and report descriptor and opens nothing else, so waiting for a
// base station to be plugged in does not ping every mouse on the machine
// every few seconds.
func cmdChatMix() error {
	// The bar starts this and should take it down with it. Without this, a
	// shell that crashed would leave it listening forever.
	_ = unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGTERM), 0, 0, 0)

	out := json.NewEncoder(os.Stdout)
	var mu sync.Mutex
	print := func(event chatMixEvent) {
		mu.Lock()
		defer mu.Unlock()
		_ = out.Encode(event)
	}

	for {
		node, entry, mixer, ok := findChatMixer()
		if !ok {
			time.Sleep(rescan)
			continue
		}
		id := discovery.IDFor(entry, node.Uniq)

		// The dial reports every step as it turns, faster than the sound
		// server can be told. Only the newest position matters, so one
		// worker applies whatever is latest and the rest are dropped.
		latest := make(chan chatMixEvent, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for event := range latest {
				if status, err := sonar.Read(); err == nil && status.Live {
					if err := sonar.ApplyMix(event.Game, event.Chat); err != nil {
						event.Error = err.Error()
					}
				}
				print(event)
			}
		}()

		_ = mixer.WatchChatMix(node, func(game, chat uint8) {
			event := chatMixEvent{ID: id, Game: game, Chat: chat}
			select {
			case latest <- event:
			default:
				// Replace the one waiting with this newer one.
				select {
				case <-latest:
				default:
				}
				latest <- event
			}
		})
		close(latest)
		<-done
		time.Sleep(rescan)
	}
}

// findChatMixer finds a catalogued device with a ChatMix dial and the node
// that reports it.
func findChatMixer() (hidraw.Node, *model.Entry, model.ChatMixer, bool) {
	for _, node := range hidraw.Enumerate() {
		entry := catalog.FindByUSB(node.Vendor, node.Product)
		if entry == nil || !entry.Has(model.CapSonar) {
			continue
		}
		mixer, ok := entry.Driver.(model.ChatMixer)
		if !ok {
			continue
		}
		if speaker, ok := entry.Driver.(model.Speaker); ok && !speaker.Speaks(node) {
			continue
		}
		return node, entry, mixer, true
	}
	return hidraw.Node{}, nil, nil, false
}

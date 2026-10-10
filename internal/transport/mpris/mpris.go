// Package mpris asks media players what they are doing, and tells them to
// play, over the session bus.
//
// It has two jobs. Switching a headset's codec takes its audio output away
// for a moment, and a player whose output disappears pauses. Spotify did,
// half a second into the switch and before the switch had even finished, and
// stayed paused. Nobody asked for that, so whoever was playing is put back.
// And the base station's Now Playing screensaver shows what is playing.
package mpris

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	namePrefix = "org.mpris.MediaPlayer2."
	objectPath = "/org/mpris/MediaPlayer2"
	player     = "org.mpris.MediaPlayer2.Player"
)

// Playing returns the bus names of every player that is playing right now.
// No session bus, or no players, is an empty list.
func Playing() []string {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	var names []string
	if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil
	}
	var playing []string
	for _, name := range names {
		if strings.HasPrefix(name, namePrefix) && status(conn, name) == "Playing" {
			playing = append(playing, name)
		}
	}
	return playing
}

// Resume plays again whichever of `names` has been paused since, waiting up
// to `within` for the pause to arrive: it comes from the player noticing its
// output went away, which is not synchronous with anything this side does.
//
// Play is repeated until the player says it is playing. A player can ignore
// one: Spotify took no notice of a Play half a second after a Pause, and did
// as asked when told again.
//
// A player that never paused is left alone, and so is one that stopped
// rather than paused — that is a player that ended, not one that was
// interrupted.
func Resume(names []string, within time.Duration) {
	if len(names) == 0 {
		return
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	// When each player was last told to play; zero until it pauses.
	waiting := map[string]time.Time{}
	for _, name := range names {
		waiting[name] = time.Time{}
	}
	deadline := time.Now().Add(within)
	for len(waiting) > 0 && time.Now().Before(deadline) {
		for name, told := range waiting {
			switch status(conn, name) {
			case "Paused":
				if told.IsZero() || time.Since(told) > retryAfter {
					conn.Object(name, objectPath).Call(player+".Play", 0)
					waiting[name] = time.Now()
				}
			case "Playing":
				if !told.IsZero() {
					delete(waiting, name)
				}
			default:
				delete(waiting, name)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// retryAfter is how long a Play is given to take before it is sent again.
const retryAfter = 700 * time.Millisecond

func status(conn *dbus.Conn, name string) string {
	value, err := conn.Object(name, objectPath).GetProperty(player + ".PlaybackStatus")
	if err != nil {
		return ""
	}
	s, _ := value.Value().(string)
	return s
}

// Track is what a player is playing.
type Track struct {
	Title   string
	Artists []string
	// Length is zero when the player does not say, as for a stream.
	Length   time.Duration
	Position time.Duration
	// Playing is false for a track that is paused.
	Playing bool
	// ArtURL is where the album cover is, or empty. Spotify gives an
	// https URL, and other players a file:// one.
	ArtURL string
}

// askWithin bounds each of NowPlaying's calls. A player that has hung would
// otherwise hold its caller for 25 s, until the bus gives up on it, and the
// players after it would go unasked.
const askWithin = 250 * time.Millisecond

// NowPlaying is the first player playing a track with a title, or else the
// first one paused on one. Players are taken in order of bus name, so of two
// the same one is shown each time. A player that has stopped, or says no
// title, as a browser tab with no media does, is passed over.
func NowPlaying() (Track, bool) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return Track{}, false
	}
	var names []string
	if err := ask(conn.BusObject(), "org.freedesktop.DBus.ListNames").Store(&names); err != nil {
		return Track{}, false
	}
	slices.Sort(names)
	var paused *Track
	for _, name := range names {
		if !strings.HasPrefix(name, namePrefix) {
			continue
		}
		var props map[string]dbus.Variant
		if ask(conn.Object(name, objectPath), "org.freedesktop.DBus.Properties.GetAll", player).Store(&props) != nil {
			continue
		}
		track, status := parseTrack(props)
		if track.Title == "" {
			continue
		}
		if status == "Playing" {
			return track, true
		}
		if status == "Paused" && paused == nil {
			paused = &track
		}
	}
	if paused != nil {
		return *paused, true
	}
	return Track{}, false
}

func ask(obj dbus.BusObject, method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), askWithin)
	defer cancel()
	return obj.CallWithContext(ctx, method, 0, args...)
}

// parseTrack reads a player's properties, as Spotify sends them:
//
//	PlaybackStatus  "Playing"
//	Position        int64 6866000                  µs
//	Metadata        xesam:title  "stardew"
//	                xesam:artist ["Purity Ring"]
//	                mpris:length int64 246653000   µs
//	                mpris:artUrl "https://i.scdn.co/image/…"
//
// The specification makes the times int64. Another integer is read too,
// rather than taken as zero.
func parseTrack(props map[string]dbus.Variant) (Track, string) {
	status, _ := props["PlaybackStatus"].Value().(string)
	metadata, _ := props["Metadata"].Value().(map[string]dbus.Variant)
	track := Track{Playing: status == "Playing", Position: micros(props["Position"])}
	track.Title, _ = metadata["xesam:title"].Value().(string)
	track.Artists, _ = metadata["xesam:artist"].Value().([]string)
	track.Length = micros(metadata["mpris:length"])
	track.ArtURL, _ = metadata["mpris:artUrl"].Value().(string)
	return track, status
}

func micros(v dbus.Variant) time.Duration {
	var n int64
	switch v := v.Value().(type) {
	case int64:
		n = v
	case uint64:
		n = int64(v)
	case int32:
		n = int64(v)
	case uint32:
		n = int64(v)
	case float64:
		n = int64(v)
	}
	return max(time.Duration(n)*time.Microsecond, 0)
}

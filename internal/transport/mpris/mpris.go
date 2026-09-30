// Package mpris asks media players what they are doing, and tells them to
// play, over the session bus.
//
// It exists for one reason: switching a headset's codec takes its audio
// output away for a moment, and a player whose output disappears pauses.
// Spotify did, half a second into the switch and before the switch had even
// finished, and stayed paused. Nobody asked for that, so whoever was playing
// is put back.
package mpris

import (
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

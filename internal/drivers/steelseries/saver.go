package steelseries

import "github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"

// screenSaver is what the screen shows while it is off. The screen is an
// OLED, which keeps a ghost of whatever stays lit on it, so every picture
// moves.
type screenSaver struct {
	slug, label string
	// play starts a run of the screensaver. It is nil for Blank, which
	// lights nothing and so need not move.
	play func() animation
}

// animation is one run of a screensaver.
type animation interface {
	// frame is the run's nth picture.
	frame(n int) *arctis.Frame
}

// screenSavers are the screensavers offered, by setting slug. Blank is the
// default, and was the only one before there were others.
var screenSavers = []screenSaver{
	{slug: "blank", label: "Blank"},
	// At the pace of a DVD player's, a pixel each way every frame, 25 a
	// second.
	{slug: "dvd", label: "DVD", play: drifting(mustArt("dvd"), 1)},
	{slug: "omarchy", label: "Omarchy", play: playOmarchy},
	// A wordmark drifts more slowly, about 8 a second, to be read rather
	// than watched.
	{slug: "omnigear", label: "OmniGear", play: drifting(mustArt("omnigear"), 3)},
}

func findScreenSaver(slug string) (*screenSaver, bool) {
	for i := range screenSavers {
		if screenSavers[i].slug == slug {
			return &screenSavers[i], true
		}
	}
	return nil, false
}

// drift is art moving on a diagonal and bouncing off each edge, as a DVD
// player's logo does. The art is narrower than the screen by a different
// amount each way, so it does not retrace one line but crosses the whole
// screen in time.
type drift struct {
	art *picture
	// step is how many frames the art holds still before it moves a pixel.
	step int
}

func drifting(art *picture, step int) func() animation {
	return func() animation { return drift{art, step} }
}

func (d drift) frame(n int) *arctis.Frame {
	var frame arctis.Frame
	moves := n / d.step
	left := bounce(moves, arctis.ScreenWidth-d.art.width)
	top := bounce(moves, arctis.ScreenHeight-d.art.height)
	for x := range d.art.width {
		for y := range d.art.height {
			frame[left+x][top+y] = d.art.at(x, y)
		}
	}
	return &frame
}

// bounce is how far along something is after moves pixels, going back and
// forth across room pixels.
func bounce(moves, room int) int {
	if room <= 0 {
		return 0
	}
	moves %= 2 * room
	return min(moves, 2*room-moves)
}

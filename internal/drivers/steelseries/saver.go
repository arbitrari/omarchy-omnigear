package steelseries

import "github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"

// screenSaver is what the screen shows while it is off. The screen is an
// OLED, which keeps a ghost of whatever stays lit on it, so every picture
// moves.
type screenSaver struct {
	slug, label string
	// art is nil for Blank, which lights nothing and so need not move.
	art *picture
	// step is how many frames the art holds still before it moves a pixel.
	step int
}

// screenSavers are the screensavers offered, by setting slug. Blank is the
// default, and was the only one before there were others.
var screenSavers = []screenSaver{
	{slug: "blank", label: "Blank"},
	// At the pace of a DVD player's, a pixel each way every frame, 25 a
	// second.
	{slug: "dvd", label: "DVD", art: mustArt("dvd"), step: 1},
	// A wordmark drifts more slowly, about 8 a second, to be read rather
	// than watched.
	{slug: "omarchy", label: "Omarchy", art: mustArt("omarchy"), step: 3},
	{slug: "omnigear", label: "OmniGear", art: mustArt("omnigear"), step: 3},
}

func findScreenSaver(slug string) (*screenSaver, bool) {
	for i := range screenSavers {
		if screenSavers[i].slug == slug {
			return &screenSavers[i], true
		}
	}
	return nil, false
}

// frame is the screensaver's nth picture: its art moving on a diagonal and
// bouncing off each edge, as a DVD player's logo does. The art is narrower
// than the screen by a different amount each way, so it does not retrace one
// line but crosses the whole screen in time.
func (s *screenSaver) frame(n int) *arctis.Frame {
	var frame arctis.Frame
	moves := n / s.step
	left := bounce(moves, arctis.ScreenWidth-s.art.width)
	top := bounce(moves, arctis.ScreenHeight-s.art.height)
	for x := range s.art.width {
		for y := range s.art.height {
			frame[left+x][top+y] = s.art.at(x, y)
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

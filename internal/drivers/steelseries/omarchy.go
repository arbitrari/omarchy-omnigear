package steelseries

import (
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"
)

// The Omarchy screensaver is Omarchy's own, on the base station: a round at a
// time, an effect picked at random builds the logo, which holds, then fades.
// Each round puts the logo somewhere else, so it never sits in one place.

// A round's frames, at 25 a second.
const (
	buildFrames = 50 // 2 s
	holdFrames  = 75 // 3 s
	clearFrames = 25 // 1 s
	darkFrames  = 12 // half a second
	roundFrames = buildFrames + holdFrames + clearFrames + darkFrames
)

type point struct{ x, y int }

// box is where a round puts the art on the screen.
type box struct{ left, top, width, height int }

// particle is a lit pixel of the art, which an effect brings home to it.
type particle struct {
	home point
	// via is where the particle goes on its way home, from the first point,
	// where it appears. A particle with none appears at home.
	via []point
	// start is the frame it appears, and frames how long it takes from
	// there to reach home.
	start, frames int
	// leave is how many frames into the clear it goes dark.
	leave int
}

// effect is a way of building the art. Its place gives each particle its
// way home, all of them there within buildFrames.
type effect struct {
	name  string
	place func(rng *rand.Rand, ps []particle, art box)
}

// effects are those of Omarchy's screensaver that one bit at 128×64 can show.
var effects = []effect{
	{"rain", func(rng *rand.Rand, ps []particle, _ box) {
		for i := range ps {
			p := &ps[i]
			p.via = []point{{p.home.x, -1}}
			p.start = rng.IntN(25)
			p.frames = 20 + rng.IntN(buildFrames-20-p.start+1)
		}
	}},
	{"scatter", func(rng *rand.Rand, ps []particle, _ box) {
		for i := range ps {
			p := &ps[i]
			p.via = []point{{rng.IntN(arctis.ScreenWidth), rng.IntN(arctis.ScreenHeight)}}
			p.frames = 30 + rng.IntN(buildFrames-30+1)
		}
	}},
	{"decrypt", func(rng *rand.Rand, ps []particle, _ box) {
		for i := range ps {
			ps[i].start = rng.IntN(buildFrames)
			ps[i].frames = 1
		}
	}},
	// A row slides in whole, every other one from the right.
	{"slice", func(_ *rand.Rand, ps []particle, art box) {
		for i := range ps {
			p := &ps[i]
			shift := -arctis.ScreenWidth
			if (p.home.y-art.top)%2 == 1 {
				shift = arctis.ScreenWidth
			}
			p.via = []point{{p.home.x + shift, p.home.y}}
			p.frames = 40
		}
	}},
	{"expand", func(_ *rand.Rand, ps []particle, art box) {
		center := point{art.left + art.width/2, art.top + art.height/2}
		for i := range ps {
			ps[i].via = []point{center}
			ps[i].frames = 40
		}
	}},
	// The art appears whole, flies apart to the edges, and comes back.
	{"unstable", func(rng *rand.Rand, ps []particle, _ box) {
		for i := range ps {
			p := &ps[i]
			p.via = []point{p.home, edgePoint(rng)}
			p.frames = 40 + rng.IntN(buildFrames-40+1)
		}
	}},
}

// edgePoint is a pixel at random on the screen's border.
func edgePoint(rng *rand.Rand) point {
	x, y := rng.IntN(arctis.ScreenWidth), rng.IntN(arctis.ScreenHeight)
	switch rng.IntN(4) {
	case 0:
		x = 0
	case 1:
		x = arctis.ScreenWidth - 1
	case 2:
		y = 0
	default:
		y = arctis.ScreenHeight - 1
	}
	return point{x, y}
}

// round is one of a run's rounds, numbered from 0, as planned.
type round struct {
	n, effect int
	particles []particle
}

// omarchyShow is a run of the Omarchy screensaver. Every round follows from
// the seed, so a run is the same each time it is drawn from that seed.
type omarchyShow struct {
	art   *picture
	seed  uint64
	round round
}

// playOmarchy reads the art afresh, so a logo rebranded since shows at the
// next run.
func playOmarchy() animation {
	return newOmarchyShow(omarchyArt(), uint64(time.Now().UnixNano()))
}

func newOmarchyShow(art *picture, seed uint64) *omarchyShow {
	s := &omarchyShow{art: art, seed: seed}
	s.round = s.plan(0, -1)
	return s
}

// embeddedOmarchy is the wordmark, for a machine whose Omarchy has no text
// logo, or one too big for the screen.
var embeddedOmarchy = mustArt("omarchy")

// omarchyArt is the art Omarchy's screensaver shows: the branding file, which
// `omarchy branding screensaver` writes, or else the logo it starts as. The
// OS screensaver reads ~/.config literally, so this does too. Art is shown at
// its size, a pixel a pixel, so a file that does not fit is passed over.
func omarchyArt() *picture {
	home, _ := os.UserHomeDir()
	omarchy := os.Getenv("OMARCHY_PATH")
	if omarchy == "" {
		omarchy = "/usr/share/omarchy"
	}
	for _, path := range []string{
		filepath.Join(home, ".config", "omarchy", "branding", "screensaver.txt"),
		filepath.Join(omarchy, "logo.txt"),
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		art := decodeBlocks(string(raw))
		if art.width > 0 && art.width <= arctis.ScreenWidth && art.height <= arctis.ScreenHeight {
			return &art
		}
	}
	return embeddedOmarchy
}

// plan lays out round n: an effect other than the last round's, and a place
// for the art anywhere it fits.
func (s *omarchyShow) plan(n, last int) round {
	rng := rand.New(rand.NewPCG(s.seed, uint64(n)))
	pick := rng.IntN(len(effects))
	if last >= 0 {
		pick = (last + 1 + rng.IntN(len(effects)-1)) % len(effects)
	}
	art := box{
		left:   rng.IntN(arctis.ScreenWidth - s.art.width + 1),
		top:    rng.IntN(arctis.ScreenHeight - s.art.height + 1),
		width:  s.art.width,
		height: s.art.height,
	}
	var ps []particle
	for y := range s.art.height {
		for x := range s.art.width {
			if s.art.at(x, y) {
				ps = append(ps, particle{home: point{art.left + x, art.top + y}})
			}
		}
	}
	effects[pick].place(rng, ps, art)
	for i := range ps {
		p := &ps[i]
		p.start = min(max(p.start, 0), buildFrames-1)
		p.frames = min(max(p.frames, 1), buildFrames-p.start)
		p.leave = rng.IntN(clearFrames)
	}
	return round{n: n, effect: pick, particles: ps}
}

func (s *omarchyShow) frame(n int) *arctis.Frame {
	r := n / roundFrames
	if r < s.round.n {
		s.round = s.plan(0, -1)
	}
	for s.round.n < r {
		s.round = s.plan(s.round.n+1, s.round.effect)
	}
	var frame arctis.Frame
	for _, p := range s.round.particles {
		if at, ok := p.at(n % roundFrames); ok &&
			at.x >= 0 && at.x < arctis.ScreenWidth && at.y >= 0 && at.y < arctis.ScreenHeight {
			frame[at.x][at.y] = true
		}
	}
	return &frame
}

// at is where the particle is in frame f of its round, if it is lit. Each
// leg of its way home slows as it ends.
func (p particle) at(f int) (point, bool) {
	if f < p.start || f >= buildFrames+holdFrames+p.leave {
		return point{}, false
	}
	t := min(float64(f-p.start+1)/float64(p.frames), 1)
	legs := len(p.via)
	if legs == 0 || t == 1 {
		return p.home, true
	}
	along := t * float64(legs)
	leg := min(int(along), legs-1)
	from, to := p.via[leg], p.home
	if leg+1 < legs {
		to = p.via[leg+1]
	}
	eased := 1 - math.Pow(1-(along-float64(leg)), 3)
	return point{
		int(math.Round(float64(from.x) + float64(to.x-from.x)*eased)),
		int(math.Round(float64(from.y) + float64(to.y-from.y)*eased)),
	}, true
}

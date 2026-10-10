package steelseries

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/mpris"
)

// The Now Playing screensaver shows the track a media player is playing:
//
//	stardew                  title, 6x13B
//	Purity Ring              artist, 6x10
//
//	[########............]   progress
//	1:12      ||      4:06   elapsed, paused, length, 5x7
//
// A line too long for the screen scrolls. The whole layout moves a pixel
// every 30 s, so nothing stays lit in one place. With nothing playing,
// "Nothing playing" bounces as the DVD logo does.
const (
	// pollEvery is how often the players are asked. Between answers the
	// position is counted on from the last one.
	pollEvery = time.Second
	// A line that does not fit holds for 1.5 s, then scrolls a pixel a
	// frame, 25 a second, until it comes round to where it started, with
	// marqueeGap between its end and its next start.
	marqueeHold = 38
	marqueeGap  = 24
	// shiftFrames is 30 s.
	shiftFrames = 750
	// The idle text moves a pixel every two frames, half the DVD logo's pace.
	idleStep = 2
)

// shifts are the offsets the layout cycles through. The layout leaves a
// pixel spare at the right and bottom for them.
var shifts = [...]point{{0, 0}, {1, 0}, {1, 1}, {0, 1}}

// The layout, before its shift. Text and the bar span textLeft to textRight.
const (
	textLeft   = 2
	textRight  = arctis.ScreenWidth - 3
	titleTop   = 5
	artistTop  = 21
	barTop     = 39
	barHeight  = 7
	timesTop   = 50
	pauseWidth = 6
)

// reading is a player's answer and when it came.
type reading struct {
	track mpris.Track
	ok    bool
	at    time.Time
}

// nowPlaying is a run of the screensaver. Frames are drawn from watch's
// loop, which cannot wait on a player, so the players are asked from a
// goroutine and each frame draws the latest answer.
type nowPlaying struct {
	read func() (mpris.Track, bool)
	now  func() time.Time
	idle drift

	mu sync.Mutex
	// latest is the last answer, and asking whether one is on its way;
	// asked is when it was sent.
	latest reading
	asking bool
	asked  time.Time

	// shown is the track the marquees run for, and since the frame they
	// started on. Only frame touches them.
	shown string
	since int
}

func playNowPlaying() animation { return newNowPlaying(mpris.NowPlaying, time.Now) }

// newNowPlaying asks once before the first frame, so a run that starts on a
// track does not show "Nothing playing" first.
func newNowPlaying(read func() (mpris.Track, bool), now func() time.Time) *nowPlaying {
	p := &nowPlaying{read: read, now: now, idle: drift{nothingPlaying, idleStep}}
	track, ok := read()
	p.asked = now()
	p.latest = reading{track, ok, p.asked}
	return p
}

var nothingPlaying = textPicture(font6x10, "Nothing playing")

// textPicture is s drawn as art, its cell's full height.
func textPicture(f *font, s string) *picture {
	var frame arctis.Frame
	f.draw(&frame, wholeScreen, s, 0, 0)
	p := &picture{width: f.measure(s), height: f.height}
	p.lit = make([]bool, p.width*p.height)
	for y := range p.height {
		for x := range p.width {
			p.lit[y*p.width+x] = frame[x][y]
		}
	}
	return p
}

// reading is the latest answer, and asks again if a second has passed and
// no question is out. A player that hangs holds back the next question
// only until mpris gives up on it.
func (p *nowPlaying) reading(now time.Time) reading {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.asking && now.Sub(p.asked) >= pollEvery {
		p.asking, p.asked = true, now
		go p.ask()
	}
	return p.latest
}

func (p *nowPlaying) ask() {
	track, ok := p.read()
	at := p.now()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latest, p.asking = reading{track, ok, at}, false
}

func (p *nowPlaying) frame(n int) *arctis.Frame {
	now := p.now()
	r := p.reading(now)
	if !r.ok {
		p.shown = ""
		return p.idle.frame(n)
	}
	if key := r.track.Title + "\x00" + strings.Join(r.track.Artists, "\x00"); key != p.shown {
		p.shown, p.since = key, n
	}
	position := r.track.Position
	if r.track.Playing {
		position += now.Sub(r.at)
	}
	if r.track.Length > 0 {
		position = min(position, r.track.Length)
	}
	return drawTrack(r.track, position, max(n-p.since, 0), shifts[n/shiftFrames%len(shifts)])
}

// drawTrack lays out a track at a position, its lines scrolled as they are
// after scrolled frames on screen.
func drawTrack(track mpris.Track, position time.Duration, scrolled int, shift point) *arctis.Frame {
	var frame arctis.Frame
	left, right := textLeft+shift.x, textRight+shift.x
	marquee(&frame, font6x13B, left, right, titleTop+shift.y, track.Title, scrolled)
	marquee(&frame, font6x10, left, right, artistTop+shift.y, strings.Join(track.Artists, ", "), scrolled)

	top := barTop + shift.y
	for x := left; x < right; x++ {
		frame[x][top], frame[x][top+barHeight-1] = true, true
	}
	for y := top; y < top+barHeight; y++ {
		frame[left][y], frame[right-1][y] = true, true
	}
	// The fill sits a dark pixel inside the outline.
	if track.Length > 0 {
		inner := right - left - 4
		filled := int(int64(inner) * int64(position) / int64(track.Length))
		for x := left + 2; x < left+2+filled; x++ {
			for y := top + 2; y < top+barHeight-2; y++ {
				frame[x][y] = true
			}
		}
	}

	times := timesTop + shift.y
	hours := track.Length >= time.Hour || position >= time.Hour
	font5x7.draw(&frame, wholeScreen, clockTime(position, hours), left, times)
	if track.Length > 0 {
		total := clockTime(track.Length, hours)
		font5x7.draw(&frame, wholeScreen, total, right-font5x7.measure(total), times)
	}
	if !track.Playing {
		pause := (arctis.ScreenWidth-pauseWidth)/2 + shift.x
		for y := times; y < times+font5x7.height-1; y++ {
			for _, x := range []int{0, 1, 4, 5} {
				frame[pause+x][y] = true
			}
		}
	}
	return &frame
}

// marquee draws s between left and right, scrolling it if it does not fit.
func marquee(frame *arctis.Frame, f *font, left, right, top int, s string, scrolled int) {
	c := clip{left, top, right, top + f.height}
	width := f.measure(s)
	if width <= right-left {
		f.draw(frame, c, s, left, top)
		return
	}
	loop := width + marqueeGap
	offset := max(scrolled%(marqueeHold+loop)-marqueeHold, 0)
	f.draw(frame, c, s, left-offset, top)
	f.draw(frame, c, s, left-offset+loop, top)
}

// clockTime is m:ss, or h:mm:ss.
func clockTime(d time.Duration, hours bool) string {
	s := int(d / time.Second)
	if hours {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

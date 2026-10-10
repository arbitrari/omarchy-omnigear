package steelseries

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"
	"github.com/arbitrari/omarchy-omnigear/internal/transport/mpris"
)

// A preview is a screensaver drawn to a GIF, for the panel to show under the
// Screensaver choice. It is the run watch would draw, from the same table,
// with the screen's own frame time.

// previewPalette is the screen's dark and its lit white.
var previewPalette = color.Palette{color.Black, color.RGBA{0xf0, 0xf4, 0xff, 0xff}}

// WriteScreenPreview draws the screensaver slug to a GIF that loops, kept
// until logout, and says where it is. Blank has nothing to show and no file,
// so its path is empty.
func WriteScreenPreview(slug string) (string, error) {
	saver, ok := findScreenSaver(slug)
	if !ok {
		return "", fmt.Errorf("no screensaver %q", slug)
	}
	if saver.play == nil {
		return "", nil
	}
	var clock atomic.Int64
	start := time.Now()
	clock.Store(start.UnixNano())
	show := saver.play(previewStage(start, func() time.Time { return time.Unix(0, clock.Load()) }))
	// A cover takes up to coverTimeout to fetch. Waiting costs the preview
	// that, once, and shows what the screen shows a moment into a track.
	if np, ok := show.(*nowPlaying); ok {
		np.loads.Wait()
	}
	anim := gif.GIF{Config: image.Config{
		ColorModel: previewPalette, Width: arctis.ScreenWidth, Height: arctis.ScreenHeight,
	}}
	for n := range saver.sample {
		clock.Store(start.Add(time.Duration(n) * saverFrame).UnixNano())
		anim.Image = append(anim.Image, previewImage(show.frame(n)))
		anim.Delay = append(anim.Delay, int(saverFrame/(10*time.Millisecond)))
	}
	var raw bytes.Buffer
	if err := gif.EncodeAll(&raw, &anim); err != nil {
		return "", err
	}
	path := runtimePath("screen-preview-" + slug + ".gif")
	return path, writeFile(path, raw.Bytes())
}

// previewStage plays a run on clock. The player is asked once, at the first
// question, and its answer is played back after, its position counted on
// by the clock: asking it again would answer in real time, which a preview
// runs ahead of by seconds.
func previewStage(start time.Time, clock func() time.Time) stage {
	first := sync.OnceValues(mpris.NowPlaying)
	return stage{now: clock, read: func() (mpris.Track, bool) {
		track, ok := first()
		if track.Playing {
			track.Position += clock().Sub(start)
		}
		return track, ok
	}}
}

func previewImage(frame *arctis.Frame) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, arctis.ScreenWidth, arctis.ScreenHeight), previewPalette)
	for x := range arctis.ScreenWidth {
		for y := range arctis.ScreenHeight {
			if frame[x][y] {
				img.SetColorIndex(x, y, 1)
			}
		}
	}
	return img
}

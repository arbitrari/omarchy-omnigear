package steelseries

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// A cover is fetched once a track, off the frame loop, and drawn coverSize
// pixels square.
const (
	coverSize = 48
	// Spotify's covers are 640 px JPEGs of around 100 KB.
	coverMaxBytes = 5 << 20
	coverMaxSide  = 4096
	coverTimeout  = 3 * time.Second
	// crushBlack is the gray level drawn as black. Below it, a dark cover's
	// near-black background dithers to a scatter of lit pixels.
	crushBlack = 0.10
)

var coverClient = &http.Client{Timeout: coverTimeout}

// loadCover fetches the cover at an http, https, or file URL and dithers it.
// A JPEG or a PNG is read, and any other format is not.
func loadCover(artURL string) (*picture, error) {
	u, err := url.Parse(artURL)
	if err != nil {
		return nil, err
	}
	var r io.ReadCloser
	switch u.Scheme {
	case "http", "https":
		resp, err := coverClient.Get(artURL)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("%s: %s", artURL, resp.Status)
		}
		r = resp.Body
	case "file":
		if r, err = os.Open(u.Path); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%s: cannot fetch a %q URL", artURL, u.Scheme)
	}
	defer r.Close()
	raw, err := io.ReadAll(io.LimitReader(r, coverMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > coverMaxBytes {
		return nil, fmt.Errorf("%s: over %d bytes", artURL, coverMaxBytes)
	}
	// A small file can still say it is huge, and decoding it would take
	// that much memory.
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", artURL, err)
	}
	if config.Width > coverMaxSide || config.Height > coverMaxSide {
		return nil, fmt.Errorf("%s: %dx%d is too big", artURL, config.Width, config.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", artURL, err)
	}
	return ditherCover(img), nil
}

// ditherCover shrinks the middle square of img to coverSize, stretches its
// grays to fill black to white, crushes the darkest to black, and diffuses
// each pixel's error onto its neighbors, as Floyd and Steinberg do.
//
// A plain threshold loses every midtone, and an ordered dither leaves a grid
// across a dark background. Both were tried on two covers, a dark one and a
// busy one, and so was brightening the grays first, which only added noise.
func ditherCover(img image.Image) *picture {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	left, top := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2

	// Each source pixel is added to the cell it falls in, so every cell is
	// the mean of the pixels it covers.
	var sum, count [coverSize][coverSize]float64
	for y := range side {
		cy := y * coverSize / side
		for x := range side {
			cx := x * coverSize / side
			r, g, bl, _ := img.At(left+x, top+y).RGBA()
			sum[cy][cx] += (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)) / 0xffff
			count[cy][cx]++
		}
	}
	var gray [coverSize][coverSize]float64
	lo, hi := 1.0, 0.0
	for y := range coverSize {
		for x := range coverSize {
			if count[y][x] > 0 {
				gray[y][x] = sum[y][x] / count[y][x]
			}
			lo, hi = min(lo, gray[y][x]), max(hi, gray[y][x])
		}
	}
	for y := range coverSize {
		for x := range coverSize {
			v := 0.0
			if hi > lo {
				v = (gray[y][x] - lo) / (hi - lo)
			}
			gray[y][x] = min(max((v-crushBlack)/(1-crushBlack), 0), 1)
		}
	}

	p := &picture{width: coverSize, height: coverSize, lit: make([]bool, coverSize*coverSize)}
	for y := range coverSize {
		for x := range coverSize {
			v := gray[y][x]
			lit := v >= 0.5
			p.lit[y*coverSize+x] = lit
			e := v
			if lit {
				e = v - 1
			}
			if x+1 < coverSize {
				gray[y][x+1] += e * 7 / 16
			}
			if y+1 < coverSize {
				if x > 0 {
					gray[y+1][x-1] += e * 3 / 16
				}
				gray[y+1][x] += e * 5 / 16
				if x+1 < coverSize {
					gray[y+1][x+1] += e * 1 / 16
				}
			}
		}
	}
	return p
}

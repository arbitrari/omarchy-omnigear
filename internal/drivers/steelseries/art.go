package steelseries

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"unicode"
)

// art holds the screensavers' pictures, drawn by scripts/screensaver-art.
//
//go:embed art/*.pbm
var art embed.FS

// picture is a 1-bit image, lit where set, a row at a time.
type picture struct {
	width, height int
	lit           []bool
}

func (p picture) at(x, y int) bool { return p.lit[y*p.width+x] }

// decodePBM reads a binary PBM, as ImageMagick writes it:
//
//	50 34 0a 33 37 20 32 30 0a <rows>   "P4\n37 20\n", then the pixels
//
// Each row is (width+7)/8 bytes, its first pixel in the top bit of the first
// byte, and a set bit is ink. Header comments are not read, since ImageMagick
// writes none.
func decodePBM(raw []byte) (picture, error) {
	r := bytes.NewReader(raw)
	var magic string
	var p picture
	if _, err := fmt.Fscan(r, &magic, &p.width, &p.height); err != nil || magic != "P4" {
		return picture{}, fmt.Errorf("not a binary PBM")
	}
	// One whitespace byte ends the header, and the pixels may start with a
	// byte that looks like whitespace too.
	if _, err := r.ReadByte(); err != nil {
		return picture{}, fmt.Errorf("PBM ends in its header")
	}
	stride := (p.width + 7) / 8
	pixels := raw[len(raw)-r.Len():]
	if p.width <= 0 || p.height <= 0 || len(pixels) != stride*p.height {
		return picture{}, fmt.Errorf("PBM %dx%d has %d bytes of pixels", p.width, p.height, len(pixels))
	}
	p.lit = make([]bool, p.width*p.height)
	for y := range p.height {
		for x := range p.width {
			p.lit[y*p.width+x] = pixels[y*stride+x/8]&(0x80>>(x%8)) != 0
		}
	}
	return p, nil
}

// decodeBlocks reads text art drawn in half blocks, as Omarchy's logo.txt
// is. Each character is a pixel wide and two tall, ▀ lighting the top, ▄ the
// bottom and █ both. The start of its second line becomes rows 2 and 3:
//
//	" ▄█████▄"   ..#####.
//	             .#######
//
// Any other character that is not a space lights both, so art in plain text
// shows too. The picture is trimmed to what is lit, and is empty if nothing is.
func decodeBlocks(text string) picture {
	type pixel struct{ x, y int }
	var lit []pixel
	for row, line := range strings.Split(text, "\n") {
		for col, r := range []rune(line) {
			top, bottom := r != '▄', r != '▀'
			if unicode.IsSpace(r) {
				top, bottom = false, false
			}
			if top {
				lit = append(lit, pixel{col, 2 * row})
			}
			if bottom {
				lit = append(lit, pixel{col, 2*row + 1})
			}
		}
	}
	if len(lit) == 0 {
		return picture{}
	}
	left, top, right, bottom := lit[0].x, lit[0].y, lit[0].x, lit[0].y
	for _, p := range lit {
		left, right = min(left, p.x), max(right, p.x)
		top, bottom = min(top, p.y), max(bottom, p.y)
	}
	pic := picture{width: right - left + 1, height: bottom - top + 1}
	pic.lit = make([]bool, pic.width*pic.height)
	for _, p := range lit {
		pic.lit[(p.y-top)*pic.width+p.x-left] = true
	}
	return pic
}

// mustArt decodes a picture embedded at build time, so a bad one is a bug in
// the build rather than something to report.
func mustArt(name string) *picture {
	raw, err := art.ReadFile("art/" + name + ".pbm")
	if err != nil {
		panic(err)
	}
	p, err := decodePBM(raw)
	if err != nil {
		panic(fmt.Sprintf("art/%s.pbm: %v", name, err))
	}
	return &p
}

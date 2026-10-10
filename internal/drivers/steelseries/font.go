package steelseries

import (
	"embed"
	"fmt"
	"unicode"

	"github.com/arbitrari/omarchy-omnigear/internal/transport/arctis"
)

// fonts holds the screensavers' fonts, converted by scripts/pixel-fonts from
// X11's misc-fixed, which are in the public domain.
//
//go:embed fonts/*.glyphs
var fonts embed.FS

// font is a character-cell font: every glyph is width by height, a row to a
// byte with its leftmost pixel in the top bit.
type font struct {
	width, height int
	glyphs        map[rune][]byte
}

var (
	font5x7   = mustFont("5x7")
	font6x10  = mustFont("6x10")
	font6x13B = mustFont("6x13B")
)

// decodeGlyphs reads a glyph file, the cell's size and then each glyph as
// its rune and its rows. 'A' in 5x7:
//
//	05 07                        5 wide, 7 tall
//	00 41 60 90 90 f0 90 90 00   U+0041, then 7 rows: .##.. #..#. …
func decodeGlyphs(raw []byte) (font, error) {
	if len(raw) < 2 || raw[0] == 0 || raw[0] > 8 || raw[1] == 0 {
		return font{}, fmt.Errorf("no cell size")
	}
	f := font{width: int(raw[0]), height: int(raw[1]), glyphs: map[rune][]byte{}}
	glyph := 2 + f.height
	raw = raw[2:]
	if len(raw)%glyph != 0 {
		return font{}, fmt.Errorf("%d bytes of glyphs is not a whole number of %d-byte glyphs", len(raw), glyph)
	}
	for ; len(raw) > 0; raw = raw[glyph:] {
		f.glyphs[rune(raw[0])<<8|rune(raw[1])] = raw[2:glyph]
	}
	if f.glyphs['?'] == nil {
		return font{}, fmt.Errorf("no '?' to stand in for missing glyphs")
	}
	return f, nil
}

// mustFont decodes a font embedded at build time, as mustArt does art.
func mustFont(name string) *font {
	raw, err := fonts.ReadFile("fonts/" + name + ".glyphs")
	if err != nil {
		panic(err)
	}
	f, err := decodeGlyphs(raw)
	if err != nil {
		panic(fmt.Sprintf("fonts/%s.glyphs: %v", name, err))
	}
	return &f
}

// cells is the glyphs that show s, a cell each. A combining mark is left out
// rather than shown as a '?' after its letter: a title written decomposed
// shows its letters bare, and none of the fonts' glyphs could sit on them.
func (f *font) cells(s string) [][]byte {
	var cells [][]byte
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		g, ok := f.glyphs[r]
		if !ok {
			g = f.glyphs['?']
		}
		cells = append(cells, g)
	}
	return cells
}

// measure is how wide s is drawn.
func (f *font) measure(s string) int { return len(f.cells(s)) * f.width }

// clip is the part of the screen something may light: columns left to
// right and rows top to bottom, each end exclusive.
type clip struct{ left, top, right, bottom int }

var wholeScreen = clip{0, 0, arctis.ScreenWidth, arctis.ScreenHeight}

// draw lights s with its top left at x, y, and nothing outside c.
func (f *font) draw(frame *arctis.Frame, c clip, s string, x, y int) {
	c = clip{max(c.left, 0), max(c.top, 0), min(c.right, arctis.ScreenWidth), min(c.bottom, arctis.ScreenHeight)}
	for i, g := range f.cells(s) {
		left := x + i*f.width
		if left >= c.right || left+f.width <= c.left {
			continue
		}
		for row, bits := range g {
			py := y + row
			if py < c.top || py >= c.bottom {
				continue
			}
			for col := range f.width {
				if px := left + col; bits&(0x80>>col) != 0 && px >= c.left && px < c.right {
					frame[px][py] = true
				}
			}
		}
	}
}

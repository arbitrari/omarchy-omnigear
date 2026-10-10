// Command bdf2glyphs converts a character-cell BDF font into the glyph files
// the SteelSeries driver embeds. scripts/pixel-fonts runs it.
//
//	go run ./scripts/bdf2glyphs <in.bdf> <out.glyphs>
//
// Only the runes the screen needs are kept: Latin, through Latin Extended-B,
// and the dashes, quotes and ellipsis of General Punctuation.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func kept(r int) bool {
	return r >= 0x20 && r <= 0x24f || r >= 0x2010 && r <= 0x2027
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: bdf2glyphs <in.bdf> <out.glyphs>")
		os.Exit(2)
	}
	out, err := convert(os.Args[1])
	if err == nil {
		err = os.WriteFile(os.Args[2], out, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bdf2glyphs:", err)
		os.Exit(1)
	}
}

// convert writes the cell's width and height, a byte each, then each glyph
// as its rune, two bytes big-endian, and a byte a row with the leftmost pixel
// in the top bit. Each glyph is placed in the cell by its BBX.
func convert(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var (
		width, height, descent int
		out                    []byte
		encoding               = -1
		bbx                    [4]int
		rows                   []byte
		inBitmap               bool
	)
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 0 {
			continue
		}
		ints := func() []int {
			var n []int
			for _, f := range fields[1:] {
				v, _ := strconv.Atoi(f)
				n = append(n, v)
			}
			return n
		}
		switch {
		case inBitmap && fields[0] != "ENDCHAR":
			b, err := hex.DecodeString(fields[0])
			if err != nil || len(b) != 1 {
				return nil, fmt.Errorf("U+%04X: bitmap row %q is not one byte", encoding, fields[0])
			}
			rows = append(rows, b[0])
		case fields[0] == "FONTBOUNDINGBOX":
			n := ints()
			width, height, descent = n[0], n[1], -n[3]
			if width > 8 {
				return nil, fmt.Errorf("cell is %d wide; at most 8 fit a byte", width)
			}
			out = append(out, byte(width), byte(height))
		case fields[0] == "ENCODING":
			encoding = ints()[0]
		case fields[0] == "BBX":
			copy(bbx[:], ints())
		case fields[0] == "BITMAP":
			inBitmap, rows = true, nil
		case fields[0] == "ENDCHAR":
			inBitmap = false
			if !kept(encoding) {
				continue
			}
			cell := make([]byte, height)
			// The glyph's bottom row sits bbx[3] above the baseline, which is
			// descent rows above the cell's bottom.
			top := height - descent - bbx[3] - bbx[1]
			for i, row := range rows {
				if y := top + i; y >= 0 && y < height {
					cell[y] = row >> bbx[2]
				}
			}
			out = binary.BigEndian.AppendUint16(out, uint16(encoding))
			out = append(out, cell...)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if width == 0 {
		return nil, fmt.Errorf("no FONTBOUNDINGBOX")
	}
	return out, nil
}

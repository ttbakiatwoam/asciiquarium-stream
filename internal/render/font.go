// Package render turns aquarium character grids into pixels (for the Linux
// framebuffer, raw video for ffmpeg, or PNG snapshots) or ANSI terminal
// output.
package render

import (
	"fmt"
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Font rasterizes monospace glyphs into fixed-size cells.
type Font struct {
	CellW, CellH int
	regular      font.Face
	bold         font.Face
	ascent       int
	cache        map[glyphKey]*image.Alpha
}

type glyphKey struct {
	r    rune
	bold bool
}

// NewFont loads the embedded Go Mono font sized so that one character cell
// is at most cellH pixels tall.
func NewFont(cellH int) (*Font, error) {
	if cellH < 6 {
		return nil, fmt.Errorf("cell height %d too small", cellH)
	}
	reg, err := opentype.Parse(gomono.TTF)
	if err != nil {
		return nil, err
	}
	bld, err := opentype.Parse(gomonobold.TTF)
	if err != nil {
		return nil, err
	}
	// find the largest size whose line height fits the cell
	var face font.Face
	size := float64(cellH)
	for ; size > 4; size -= 0.25 {
		f, err := opentype.NewFace(reg, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
		m := f.Metrics()
		if (m.Ascent + m.Descent).Ceil() <= cellH {
			face = f
			break
		}
		f.Close()
	}
	if face == nil {
		return nil, fmt.Errorf("no font size fits a %dpx cell", cellH)
	}
	bface, err := opentype.NewFace(bld, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	adv, _ := face.GlyphAdvance('M')
	m := face.Metrics()
	pad := cellH - (m.Ascent + m.Descent).Ceil()
	return &Font{
		CellW:   int(math.Ceil(float64(adv) / 64)),
		CellH:   cellH,
		regular: face,
		bold:    bface,
		ascent:  m.Ascent.Ceil() + pad/2,
		cache:   map[glyphKey]*image.Alpha{},
	}, nil
}

// Glyph returns the coverage mask for r, cached.
func (f *Font) Glyph(r rune, bold bool) *image.Alpha {
	k := glyphKey{r, bold}
	if g, ok := f.cache[k]; ok {
		return g
	}
	img := image.NewAlpha(image.Rect(0, 0, f.CellW, f.CellH))
	face := f.regular
	if bold {
		face = f.bold
	}
	if r != ' ' {
		d := font.Drawer{Dst: img, Src: image.Opaque, Face: face, Dot: fixed.P(0, f.ascent)}
		if _, ok := face.GlyphAdvance(r); !ok {
			r = '?'
		}
		d.DrawString(string(r))
	}
	f.cache[k] = img
	return img
}

var _ draw.Image = (*image.Alpha)(nil)

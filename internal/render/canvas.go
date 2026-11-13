package render

import (
	"encoding/binary"
	"image"
	"image/color"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// PixelFormat describes how a pixel is packed, little endian.
type PixelFormat struct {
	BytesPerPixel int
	// bit offsets and lengths of each channel
	ROff, RLen, GOff, GLen, BOff, BLen uint32
}

// Common pixel formats.
var (
	XRGB8888 = PixelFormat{4, 16, 8, 8, 8, 0, 8}
	RGB565   = PixelFormat{2, 11, 5, 5, 6, 0, 5}
	// RGB24 is three bytes per pixel in R, G, B order (ffmpeg "rgb24").
	RGB24 = PixelFormat{3, 0, 8, 8, 8, 16, 8}
)

func (p PixelFormat) pack(c RGB) uint32 {
	ch := func(v uint8, off, n uint32) uint32 { return (uint32(v) >> (8 - n)) << off }
	return ch(c.R, p.ROff, p.RLen) | ch(c.G, p.GOff, p.GLen) | ch(c.B, p.BOff, p.BLen)
}

func (p PixelFormat) put(b []byte, v uint32) {
	switch p.BytesPerPixel {
	case 4:
		binary.LittleEndian.PutUint32(b, v)
	case 3:
		b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16)
	case 2:
		binary.LittleEndian.PutUint16(b, uint16(v))
	}
}

// Canvas paints character grids into a pixel buffer, only repainting cells
// that changed since the last frame. This keeps CPU use on a Raspberry Pi
// low: most of the screen is static water.
type Canvas struct {
	W, H   int // pixels
	Stride int // bytes per row
	Buf    []byte
	Format PixelFormat
	Font   *Font
	// OffX, OffY center the grid in the buffer.
	OffX, OffY int
	Cols, Rows int

	prev   []engine.Cell
	blocks map[blockKey][]byte
	bg     uint32
}

type blockKey struct {
	r  rune
	fg engine.Color
}

// NewCanvas wraps buf (which may be framebuffer memory) as a canvas.
func NewCanvas(w, h, stride int, buf []byte, pf PixelFormat, f *Font) *Canvas {
	c := &Canvas{
		W: w, H: h, Stride: stride, Buf: buf, Format: pf, Font: f,
		Cols: w / f.CellW, Rows: h / f.CellH,
		blocks: map[blockKey][]byte{},
		bg:     pf.pack(Background),
	}
	c.OffX = (w - c.Cols*f.CellW) / 2
	c.OffY = (h - c.Rows*f.CellH) / 2
	c.Clear()
	return c
}

// Clear fills the whole buffer with the background and forgets what was
// drawn.
func (c *Canvas) Clear() {
	bpp := c.Format.BytesPerPixel
	row := make([]byte, c.W*bpp)
	for x := 0; x < c.W; x++ {
		c.Format.put(row[x*bpp:], c.bg)
	}
	for y := 0; y < c.H; y++ {
		copy(c.Buf[y*c.Stride:], row)
	}
	c.prev = nil
}

// block returns the pre-packed pixels for a character cell.
func (c *Canvas) block(cell engine.Cell) []byte {
	k := blockKey{cell.Ch, cell.Fg}
	if b, ok := c.blocks[k]; ok {
		return b
	}
	f := c.Font
	bpp := c.Format.BytesPerPixel
	b := make([]byte, f.CellW*f.CellH*bpp)
	g := f.Glyph(cell.Ch, cell.Fg.IsBright())
	fg := colorOf(cell.Fg)
	for y := 0; y < f.CellH; y++ {
		for x := 0; x < f.CellW; x++ {
			a := uint32(g.AlphaAt(x, y).A)
			mix := func(fc, bc uint8) uint8 { return uint8((uint32(fc)*a + uint32(bc)*(255-a)) / 255) }
			px := RGB{mix(fg.R, Background.R), mix(fg.G, Background.G), mix(fg.B, Background.B)}
			c.Format.put(b[(y*f.CellW+x)*bpp:], c.Format.pack(px))
		}
	}
	c.blocks[k] = b
	return b
}

// Draw paints g, which must be Cols x Rows. It returns the number of cells
// repainted.
func (c *Canvas) Draw(g *engine.Grid) int {
	if len(c.prev) != len(g.Cells) {
		c.prev = make([]engine.Cell, len(g.Cells))
		for i := range c.prev {
			c.prev[i] = engine.Cell{Ch: -1}
		}
	}
	f := c.Font
	bpp := c.Format.BytesPerPixel
	rowBytes := f.CellW * bpp
	n := 0
	for y := 0; y < g.H && y < c.Rows; y++ {
		for x := 0; x < g.W && x < c.Cols; x++ {
			i := y*g.W + x
			cell := g.Cells[i]
			if cell.Ch == ' ' {
				cell.Fg = engine.White // color of a blank doesn't matter
			}
			if c.prev[i] == cell {
				continue
			}
			c.prev[i] = cell
			n++
			b := c.block(cell)
			px := c.OffX + x*f.CellW
			py := c.OffY + y*f.CellH
			for r := 0; r < f.CellH; r++ {
				off := (py+r)*c.Stride + px*bpp
				copy(c.Buf[off:off+rowBytes], b[r*rowBytes:(r+1)*rowBytes])
			}
		}
	}
	return n
}

// Image converts the buffer to an image (for PNG snapshots).
func (c *Canvas) Image() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	bpp := c.Format.BytesPerPixel
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b := c.Buf[y*c.Stride+x*bpp:]
			var v uint32
			for i := bpp - 1; i >= 0; i-- {
				v = v<<8 | uint32(b[i])
			}
			ch := func(off, n uint32) uint8 { return uint8(((v >> off) & (1<<n - 1)) << (8 - n)) }
			img.SetRGBA(x, y, color.RGBA{ch(c.Format.ROff, c.Format.RLen), ch(c.Format.GOff, c.Format.GLen), ch(c.Format.BOff, c.Format.BLen), 255})
		}
	}
	return img
}

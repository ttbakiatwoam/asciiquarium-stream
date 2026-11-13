package render

import (
	"testing"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

func TestCanvasDrawsOnlyChanges(t *testing.T) {
	f, err := NewFont(20)
	if err != nil {
		t.Fatal(err)
	}
	if f.CellW < 6 || f.CellW > 14 {
		t.Fatalf("unexpected cell width %d", f.CellW)
	}
	w, h := 1920, 1080
	c := NewCanvas(w, h, w*4, make([]byte, w*h*4), XRGB8888, f)
	g := engine.NewGrid(c.Cols, c.Rows)
	g.Text(0, 0, "hello", engine.Red.Bright())
	if n := c.Draw(g); n != c.Cols*c.Rows {
		t.Fatalf("first frame should paint everything, painted %d", n)
	}
	g.Text(0, 0, "jello", engine.Red.Bright())
	if n := c.Draw(g); n != 1 {
		t.Fatalf("expected 1 changed cell, got %d", n)
	}
	// some red pixels were drawn where the text is
	img := c.Image()
	found := false
	for y := c.OffY; y < c.OffY+f.CellH; y++ {
		for x := c.OffX; x < c.OffX+5*f.CellW; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 > 200 && g>>8 < 50 && b>>8 < 50 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no red text pixels found")
	}
}

func TestPixelFormats(t *testing.T) {
	for _, pf := range []PixelFormat{XRGB8888, RGB565, RGB24} {
		b := make([]byte, 4)
		pf.put(b, pf.pack(RGB{255, 0, 0}))
		c := &Canvas{W: 1, H: 1, Stride: 4, Buf: b, Format: pf}
		r, g, bl, _ := c.Image().At(0, 0).RGBA()
		if r>>8 < 240 || g != 0 || bl != 0 {
			t.Fatalf("%+v: got %d %d %d", pf, r>>8, g>>8, bl>>8)
		}
	}
}

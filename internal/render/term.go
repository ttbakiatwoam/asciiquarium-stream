package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// Terminal draws grids to an ANSI terminal, writing only changed cells.
type Terminal struct {
	w    *bufio.Writer
	prev []engine.Cell
	fg   int
}

// NewTerminal starts drawing on w (switches to the alternate screen and
// hides the cursor). Call Close to restore the terminal.
func NewTerminal(w io.Writer) *Terminal {
	t := &Terminal{w: bufio.NewWriterSize(w, 1<<16), fg: -1}
	t.w.WriteString("\x1b[?1049h\x1b[?25l\x1b[40m\x1b[2J")
	t.w.Flush()
	return t
}

// Invalidate forces a full redraw on the next Draw (e.g. after a resize).
func (t *Terminal) Invalidate() {
	t.prev = nil
	t.w.WriteString("\x1b[40m\x1b[2J")
}

// Draw writes the changed cells of g.
func (t *Terminal) Draw(g *engine.Grid) error {
	if len(t.prev) != len(g.Cells) {
		t.prev = make([]engine.Cell, len(g.Cells))
		for i := range t.prev {
			t.prev[i] = engine.Cell{Ch: -1}
		}
	}
	lastX, lastY := -2, -2
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			i := y*g.W + x
			c := g.Cells[i]
			if t.prev[i] == c {
				continue
			}
			t.prev[i] = c
			if !(y == lastY && x == lastX+1) {
				fmt.Fprintf(t.w, "\x1b[%d;%dH", y+1, x+1)
			}
			if int(c.Fg) != t.fg {
				base := 30 + int(c.Fg.Base())
				if c.Fg.IsBright() {
					fmt.Fprintf(t.w, "\x1b[1;%dm", base)
				} else {
					fmt.Fprintf(t.w, "\x1b[22;%dm", base)
				}
				t.fg = int(c.Fg)
			}
			t.w.WriteRune(c.Ch)
			lastX, lastY = x, y
		}
	}
	return t.w.Flush()
}

// Close restores the terminal.
func (t *Terminal) Close() error {
	t.w.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
	return t.w.Flush()
}

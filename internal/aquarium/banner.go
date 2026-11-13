package aquarium

import (
	"strings"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// drawBanners draws the alert box and the ticker line over the scene.
func (q *Aquarium) drawBanners(g *engine.Grid) {
	now := q.anim.Now()
	if q.ticker != nil && now.After(q.ticker.Expires) {
		q.ticker = nil
	}
	for len(q.alerts) > 0 {
		a := &q.alerts[0]
		if a.Expires.IsZero() {
			a.Expires = now.Add(a.Dur)
		}
		if now.After(a.Expires) {
			q.alerts = q.alerts[1:]
			continue
		}
		break
	}

	if q.ticker != nil {
		text := " " + q.ticker.Text + " "
		y := g.H - 1
		x := 1
		for i, r := range text {
			g.Set(x+i, y, engine.Cell{Ch: r, Fg: engine.Yellow.Bright()})
		}
	}
	if len(q.alerts) > 0 {
		drawBox(g, q.alerts[0].Text, 10)
	}
}

// drawBox draws text in a box centered horizontally at row y, wrapping
// long text.
func drawBox(g *engine.Grid, text string, y int) {
	maxW := min(60, g.W-6)
	if maxW < 4 {
		return
	}
	lines := wrap(text, maxW)
	w := 0
	for _, l := range lines {
		w = max(w, len(l))
	}
	x := (g.W - (w + 4)) / 2
	border := engine.Cell{Ch: '-', Fg: engine.Magenta.Bright()}
	side := engine.Cell{Ch: '|', Fg: engine.Magenta.Bright()}
	corner := engine.Cell{Ch: '+', Fg: engine.Magenta.Bright()}
	for i := 0; i < w+4; i++ {
		c := border
		if i == 0 || i == w+3 {
			c = corner
		}
		g.Set(x+i, y, c)
		g.Set(x+i, y+len(lines)+1, c)
	}
	for j, l := range lines {
		row := y + 1 + j
		g.Set(x, row, side)
		g.Set(x+w+3, row, side)
		pad := (w - len(l)) / 2
		for i := 1; i < w+3; i++ {
			g.Set(x+i, row, engine.Cell{Ch: ' ', Fg: engine.White})
		}
		g.Text(x+2+pad, row, l, engine.White.Bright())
	}
}

func wrap(s string, w int) []string {
	var lines []string
	var cur string
	for _, word := range strings.Fields(s) {
		for len(word) > w {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, word[:w])
			word = word[w:]
		}
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= w:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

package engine

import "strings"

// Cell is one character on screen.
type Cell struct {
	Ch rune
	Fg Color
}

// spriteCell is a cell of a sprite; transparent cells are not drawn.
type spriteCell struct {
	Cell
	Opaque bool
}

// Sprite is a single animation frame: a rectangle of cells.
type Sprite struct {
	W, H  int
	cells []spriteCell
}

// At returns the cell at (x, y) and whether it is opaque.
func (s *Sprite) At(x, y int) (Cell, bool) {
	c := s.cells[y*s.W+x]
	return c.Cell, c.Opaque
}

// SpriteOpts controls how a shape string is turned into a sprite.
type SpriteOpts struct {
	// Mask is a color mask with the same layout as the shape. Each letter
	// gives the color of the character at the same position (see
	// ColorFromCode). Spaces, or positions outside the mask, use Default.
	Mask string
	// Default is the color for unmasked characters.
	Default Color
	// AutoTrans makes the leading whitespace on each line transparent.
	AutoTrans bool
	// Trans is the character that is always transparent. Zero means '?',
	// the Term::Animation default.
	Trans rune
}

// splitShape splits a Term::Animation shape string into lines. Like the Perl
// module, a single leading newline is dropped (so q{...} heredoc-style
// strings work), as is a single trailing newline.
func splitShape(s string) []string {
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// NewSprite builds a sprite from a shape string.
func NewSprite(shape string, o SpriteOpts) *Sprite {
	trans := o.Trans
	if trans == 0 {
		trans = '?'
	}
	lines := splitShape(shape)
	rows := make([][]rune, len(lines))
	w := 0
	for i, l := range lines {
		rows[i] = []rune(l)
		if len(rows[i]) > w {
			w = len(rows[i])
		}
	}
	var mask [][]rune
	if o.Mask != "" {
		for _, l := range splitShape(o.Mask) {
			mask = append(mask, []rune(l))
		}
	}
	s := &Sprite{W: w, H: len(rows), cells: make([]spriteCell, w*len(rows))}
	for y, row := range rows {
		leading := true
		for x := 0; x < w; x++ {
			if x >= len(row) {
				break // past the end of the line: transparent
			}
			ch := row[x]
			if ch != ' ' {
				leading = false
			}
			if ch == trans || (o.AutoTrans && leading && ch == ' ') {
				continue
			}
			fg := o.Default
			if y < len(mask) && x < len(mask[y]) && mask[y][x] < 128 {
				if c, ok := ColorFromCode(byte(mask[y][x])); ok {
					fg = c
				}
			}
			s.cells[y*w+x] = spriteCell{Cell: Cell{Ch: ch, Fg: fg}, Opaque: true}
		}
	}
	return s
}

// NewSprites builds one sprite per frame. masks may be shorter than shapes
// (missing masks are treated as empty) or contain a single mask used for
// every frame.
func NewSprites(shapes, masks []string, o SpriteOpts) []*Sprite {
	out := make([]*Sprite, len(shapes))
	for i, sh := range shapes {
		oo := o
		switch {
		case i < len(masks):
			oo.Mask = masks[i]
		case len(masks) == 1:
			oo.Mask = masks[0]
		default:
			oo.Mask = ""
		}
		out[i] = NewSprite(sh, oo)
	}
	return out
}

// Package engine is a small Go re-implementation of the parts of the Perl
// Term::Animation module that asciiquarium relies on: multi-frame ASCII
// sprites with color masks, z-ordering, simple movement, paths, collision
// detection and entity lifetimes.
package engine

import "strings"

// Color is one of the 16 classic terminal colors. Values 0-7 are the normal
// colors and 8-15 their bright ("bold") variants.
type Color uint8

const (
	Black Color = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Bright returns the bright variant of c.
func (c Color) Bright() Color { return c | 8 }

// IsBright reports whether c is one of the bright colors.
func (c Color) IsBright() bool { return c&8 != 0 }

// Base returns the normal (non-bright) variant of c.
func (c Color) Base() Color { return c &^ 8 }

var codeColors = map[byte]Color{
	'k': Black, 'r': Red, 'g': Green, 'y': Yellow,
	'b': Blue, 'm': Magenta, 'c': Cyan, 'w': White,
}

// ColorFromCode maps a Term::Animation color mask letter to a color.
// Lowercase letters are normal colors, uppercase letters bright ones.
func ColorFromCode(b byte) (Color, bool) {
	if c, ok := codeColors[b]; ok {
		return c, true
	}
	if b >= 'A' && b <= 'Z' {
		if c, ok := codeColors[b+('a'-'A')]; ok {
			return c.Bright(), true
		}
	}
	return 0, false
}

var nameColors = map[string]Color{
	"black": Black, "red": Red, "green": Green, "yellow": Yellow,
	"blue": Blue, "magenta": Magenta, "cyan": Cyan, "white": White,
}

// ParseColor parses a color name the way Term::Animation does: "cyan" is the
// normal color and "CYAN" the bright one. Single mask letters are accepted
// too. ok is false for unknown names.
func ParseColor(name string) (c Color, ok bool) {
	if len(name) == 1 {
		return ColorFromCode(name[0])
	}
	lower := strings.ToLower(name)
	c, ok = nameColors[lower]
	if !ok {
		return 0, false
	}
	if name == strings.ToUpper(name) {
		c = c.Bright()
	}
	return c, true
}

// ColorNames lists the accepted color names (lowercase).
func ColorNames() []string {
	return []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}
}

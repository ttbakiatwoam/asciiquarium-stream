package render

import "github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"

// RGB is a 24-bit color.
type RGB struct{ R, G, B uint8 }

// Palette maps the 16 terminal colors to RGB. These are the xterm defaults,
// which is what the stream showed when it ran in xterm.
var Palette = [16]RGB{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

// Background is the tank's background color.
var Background = RGB{0, 0, 0}

func colorOf(c engine.Color) RGB { return Palette[c&15] }

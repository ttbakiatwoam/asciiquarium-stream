// Package aquarium is a Go port of the asciiquarium scene: the water line,
// castle, seaweed, fish and the big "random object" creatures, plus the
// interactive additions (viewer fish, on-demand creatures and banners).
//
// The behavior follows the Perl original by Kirk Baucom and contributors
// (https://github.com/ttbakiatwoam/asciiquarium) as closely as practical.
package aquarium

import (
	"strings"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// z depths, from the Perl source. Lower is closer to the viewer.
const (
	depthGUIText   = 0
	depthGUI       = 1
	depthShark     = 2
	depthFishStart = 3
	depthFishEnd   = 20
	depthSeaweed   = 21
	depthCastle    = 22

	depthWaterLine3 = 2
	depthWaterGap3  = 3
	depthWaterLine2 = 4
	depthWaterGap2  = 5
	depthWaterLine1 = 6
	depthWaterGap1  = 7
	depthWaterLine0 = 8
	depthWaterGap0  = 9
)

var waterLineDepth = []float64{depthWaterLine0, depthWaterLine1, depthWaterLine2, depthWaterLine3}

func (q *Aquarium) addEnvironment() {
	a := q.anim
	for i := 0; i < 4; i++ {
		ae := art.get("environment", 0, i)
		seg := ae.Shape[0]
		repeat := a.W/len(seg) + 1
		q.anim.Add(&engine.Entity{
			Type:     "waterline",
			Frames:   []*engine.Sprite{engine.NewSprite(strings.Repeat(seg, repeat), engine.SpriteOpts{Default: engine.Cyan})},
			X:        0,
			Y:        float64(i + 5),
			Z:        waterLineDepth[i],
			Depth:    22,
			Physical: true,
		})
	}
}

func (q *Aquarium) addCastle() {
	a := q.anim
	ae := art.get("castle", 0, 0)
	if flag := SanitizeText(q.cfg.CastleFlag, 24); flag != "" {
		ae = withCastleFlag(ae, flag)
	}
	a.Add(&engine.Entity{
		Type:       "castle",
		Frames:     ae.sprites(true, engine.White),
		X:          float64(a.W - 32),
		Y:          float64(a.H - 13),
		Z:          depthCastle,
		FrameSpeed: 0.1,
	})
}

// withCastleFlag writes text on the castle's flag ("T~~" becomes
// "T~ASCIIFISH.TV~"), with green waves and bright white letters.
func withCastleFlag(ae artEntity, flag string) artEntity {
	out := artEntity{DefaultColor: ae.DefaultColor}
	for _, sh := range ae.Shape {
		out.Shape = append(out.Shape, strings.Replace(sh, "T~~", "T~"+flag+"~", 1))
	}
	for _, m := range ae.Color {
		out.Color = append(out.Color, strings.Replace(m, "GG", "G"+strings.Repeat("W", len(flag))+"G", 1))
	}
	return out
}

func (q *Aquarium) addAllSeaweed() {
	for i := 0; i < q.anim.W/15; i++ {
		q.addSeaweed()
	}
}

func (q *Aquarium) addSeaweed() {
	a := q.anim
	r := a.Rand
	height := r.IntN(4) + 3
	var left, right strings.Builder
	for i := 1; i <= height; i++ {
		if i%2 == 1 {
			right.WriteString("(\n")
			left.WriteString(" )\n")
		} else {
			left.WriteString("(\n")
			right.WriteString(" )\n")
		}
	}
	// Perl builds image[left_side] where left_side = i % 2; frame 0 is
	// the one that starts with " )".
	frames := []*engine.Sprite{
		engine.NewSprite(left.String(), engine.SpriteOpts{Default: engine.Green}),
		engine.NewSprite(right.String(), engine.SpriteOpts{Default: engine.Green}),
	}
	a.Add(&engine.Entity{
		Type:       "seaweed",
		Frames:     frames,
		X:          float64(r.IntN(max(a.W-2, 1)) + 1),
		Y:          float64(a.H - height),
		Z:          depthSeaweed,
		FrameSpeed: r.Float64()*0.05 + 0.25,
		// seaweed lives for 8 to 12 minutes
		DieTime: a.Now().Add(q.minutes(8 + r.Float64()*4)),
		OnDeath: func(_ *engine.Entity, _ *engine.Anim) { q.addSeaweed() },
	})
}

// addBubble adds an air bubble coming out of a fish.
func (q *Aquarium) addBubble(fish *engine.Entity) {
	w, h := fish.Size()
	x, y := fish.X, fish.Y
	if fish.DX > 0 {
		x += float64(w)
	}
	y += float64(h / 2)
	shapes := []string{".", "o", "O", "O", "O"}
	q.anim.Add(&engine.Entity{
		Type:         "bubble",
		Frames:       engine.NewSprites(shapes, nil, engine.SpriteOpts{Default: engine.Cyan.Bright()}),
		X:            x,
		Y:            y,
		Z:            fish.Z - 1,
		DY:           -1,
		FrameSpeed:   0.1,
		DieOffscreen: true,
		Physical:     true,
		OnCollide: func(e *engine.Entity, _ *engine.Anim, hits []*engine.Entity) {
			for _, h := range hits {
				if h.Type == "waterline" {
					e.Kill()
					return
				}
			}
		},
	})
}

func (q *Aquarium) addSplat(x, y, z float64) {
	ae := art.get("splat", 0, 0)
	frames := engine.NewSprites(ae.Shape, nil, engine.SpriteOpts{Default: engine.Red.Bright(), Trans: ' '})
	q.anim.Add(&engine.Entity{
		Type:       "splat",
		Frames:     frames,
		X:          x - 4,
		Y:          y - 2,
		Z:          z - 2,
		FrameSpeed: 0.25,
		DieAfter:   15,
	})
}

// randPalette picks a random color code for each mask digit 1-9, like
// rand_color in the Perl source.
func (q *Aquarium) randPalette() *[10]byte {
	colors := []byte("cCrRyYbBgGmM")
	var p [10]byte
	for i := 1; i <= 9; i++ {
		// Perl uses int(rand($#colors)), which never picks the last color.
		p[i] = colors[q.anim.Rand.IntN(len(colors)-1)]
	}
	return &p
}

// applyPalette replaces the digits 1-9 in a color mask.
func applyPalette(mask string, p *[10]byte) string {
	b := []byte(mask)
	for i, c := range b {
		if c >= '1' && c <= '9' {
			b[i] = p[c-'0']
		}
	}
	return string(b)
}

// randColor is rand_color from the Perl source.
func (q *Aquarium) randColor(mask string) string { return applyPalette(mask, q.randPalette()) }

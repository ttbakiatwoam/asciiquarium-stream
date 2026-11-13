package aquarium

import (
	"strings"
	"time"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

func (q *Aquarium) addAllFish() {
	a := q.anim
	// figure out how many fish to add by the size of the screen, minus the
	// stuff above the water
	screen := (a.H - 9) * a.W
	for i := 0; i < screen/350; i++ {
		q.addFish()
	}
}

// fishSet picks the fish art like add_fish in Perl: new species 3 out of
// 12 times unless classic mode is on.
func (q *Aquarium) fishSet() []fishArt {
	if !q.cfg.Classic && q.anim.Rand.IntN(12) > 8 {
		return art.newFish
	}
	return art.oldFish
}

// fishSpec describes how to build a fish.
type fishSpec struct {
	set   []fishArt
	num   int // index into set; odd entries swim right to left
	speed float64
	// palette maps mask digits to color codes; chosen once per fish so a
	// viewer's fish keeps its colors when it turns around.
	palette *[10]byte
	viewer  *viewerFish
}

func (s fishSpec) right() bool { return s.num%2 == 1 }

// randomFishSpec picks random art, direction and speed.
func (q *Aquarium) randomFishSpec(set []fishArt) fishSpec {
	r := q.anim.Rand
	num := r.IntN(len(set))
	return fishSpec{set: set, num: num, speed: r.Float64()*2 + 0.25}
}

func (q *Aquarium) addFish() {
	q.addFishEntity(q.randomFishSpec(q.fishSet()))
}

func (q *Aquarium) addFishEntity(s fishSpec) *engine.Entity {
	a := q.anim
	r := a.Rand
	if s.palette == nil {
		s.palette = q.randPalette()
	}
	fa := s.set[s.num]
	mask := applyPalette(strings.ReplaceAll(fa.Color, "4", "W"), s.palette)
	speed := s.speed
	if s.right() {
		speed = -speed
	}
	depth := float64(r.IntN(depthFishEnd-depthFishStart) + depthFishStart)
	e := &engine.Entity{
		Type: "fish",
		Frames: []*engine.Sprite{engine.NewSprite(fa.Shape, engine.SpriteOpts{
			Mask: mask, Default: engine.White, AutoTrans: true,
		})},
		Z:            depth,
		DX:           speed,
		DieOffscreen: true,
		Physical:     true,
	}
	e.Update = func(e *engine.Entity, a *engine.Anim) {
		if a.Chance(0.02) {
			q.addBubble(e)
		}
		a.Move(e)
	}
	e.OnCollide = q.fishCollision
	w, h := e.Size()
	minY, maxY := 9, a.H-h
	e.Y = float64(r.IntN(max(maxY-minY, 1)) + minY)
	if s.right() {
		e.X = float64(a.W - 2)
	} else {
		e.X = float64(1 - w)
	}
	if s.viewer != nil {
		vf := s.viewer
		vf.entity = e
		e.Label = vf.Name
		e.LabelColor = engine.White.Bright()
		e.Data = vf
		e.OnDeath = func(_ *engine.Entity, _ *engine.Anim) {
			vf.entity = nil
			if q.anim.Now().Before(vf.Expires) && !vf.removed {
				// keep swimming until the viewer's time is up,
				// alternating direction each crossing
				next := s
				next.num = s.num ^ 1 // the mirrored art
				q.addFishEntity(next)
				return
			}
			q.dropViewerFish(vf)
		}
	} else {
		e.OnDeath = func(_ *engine.Entity, _ *engine.Anim) { q.addFish() }
	}
	a.Add(e)
	return e
}

func (q *Aquarium) fishCollision(fish *engine.Entity, a *engine.Anim, hits []*engine.Entity) {
	for _, h := range hits {
		switch h.Type {
		case "teeth":
			q.addSplat(h.X, h.Y, h.Z)
			fish.Kill()
			return
		case "hook_point":
			q.retract(h)
			q.retract(fish)
			for _, t := range []string{"fishhook", "fishline"} {
				for _, e := range a.OfType(t) {
					q.retract(e)
				}
			}
			return
		}
	}
}

func colorCode(c engine.Color) byte {
	codes := "krgybmcw"
	b := codes[c.Base()]
	if c.IsBright() {
		b -= 'a' - 'A'
	}
	return b
}

func (q *Aquarium) minutes(m float64) time.Duration {
	return time.Duration(m * float64(time.Minute))
}

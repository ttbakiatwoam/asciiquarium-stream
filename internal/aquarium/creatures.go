package aquarium

import (
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// Creature kinds that can be spawned on demand.
const (
	Ship      = "ship"
	Whale     = "whale"
	Monster   = "monster"
	BigFish   = "bigfish"
	Shark     = "shark"
	Fishhook  = "fishhook"
	Swan      = "swan"
	Ducks     = "ducks"
	Dolphins  = "dolphins"
	Submarine = "submarine"
	Swordfish = "swordfish"
)

// CreatureKinds lists every creature that can be spawned, in the order of
// the Perl random object table.
var CreatureKinds = []string{Ship, Whale, Monster, BigFish, Shark, Fishhook, Swan, Ducks, Dolphins, Submarine, Swordfish}

// creature spawns one creature. onDone is called when it leaves the
// screen. dir is 0 (left to right) or 1 (right to left).
type creatureFn func(q *Aquarium, dir int, onDone func())

var creatures = map[string]creatureFn{
	Ship:      (*Aquarium).addShip,
	Whale:     (*Aquarium).addWhale,
	Monster:   (*Aquarium).addMonster,
	BigFish:   (*Aquarium).addBigFish,
	Shark:     (*Aquarium).addShark,
	Fishhook:  (*Aquarium).addFishhook,
	Swan:      (*Aquarium).addSwan,
	Ducks:     (*Aquarium).addDucks,
	Dolphins:  (*Aquarium).addDolphins,
	Submarine: (*Aquarium).addSubmarine,
	Swordfish: (*Aquarium).addSwordfish,
}

// randomObject adds one of the big creatures; when it leaves, another
// random one follows. This is the ambient "one big thing at a time" loop
// from the Perl version.
func (q *Aquarium) randomObject() {
	kind := CreatureKinds[q.anim.Rand.IntN(len(CreatureKinds))]
	creatures[kind](q, q.anim.Rand.IntN(2), q.randomObject)
}

func sign(dir int) float64 {
	if dir == 1 {
		return -1
	}
	return 1
}

// simple adds the common case: one entity crossing the screen.
func (q *Aquarium) simple(kind string, dir int, ae artEntity, x, y, z, speed, frameSpeed float64, def engine.Color, onDone func()) *engine.Entity {
	return q.anim.Add(&engine.Entity{
		Type:         kind,
		Frames:       ae.sprites(true, def),
		X:            x,
		Y:            y,
		Z:            z,
		DX:           speed * sign(dir),
		FrameSpeed:   frameSpeed,
		DieOffscreen: true,
		OnDeath:      func(*engine.Entity, *engine.Anim) { onDone() },
	})
}

// startX returns the starting x for a creature of the given width-ish
// offset: off the left edge for dir 0, at the right edge for dir 1.
func (q *Aquarium) startX(dir int, left float64, right float64) float64 {
	if dir == 1 {
		return float64(q.anim.W) + right
	}
	return left
}

func (q *Aquarium) addShip(dir int, onDone func()) {
	q.simple(Ship, dir, art.get("ship", dir, 0), q.startX(dir, -24, -2), 0, depthWaterGap1, 1, 0, engine.White.Bright(), onDone)
}

func (q *Aquarium) addWhale(dir int, onDone func()) {
	q.simple(Whale, dir, art.get("whale", dir, 0), q.startX(dir, -18, -2), 0, depthWaterGap2, 1, 1, engine.White.Bright(), onDone)
}

func (q *Aquarium) addMonster(dir int, onDone func()) {
	name, left := "new_monster", -54.0
	if q.cfg.Classic {
		name, left = "old_monster", -64
	}
	q.simple(Monster, dir, art.get(name, dir, 0), q.startX(dir, left, -2), 2, depthWaterGap2, 2, 0.25, engine.Green.Bright(), onDone)
}

func (q *Aquarium) randY(minY, fromBottom int) float64 {
	maxY := q.anim.H - fromBottom
	return float64(q.anim.Rand.IntN(max(maxY-minY, 1)) + minY)
}

func (q *Aquarium) addBigFish(dir int, onDone func()) {
	name, speed, left, fromBottom := "big_fish_1", 3.0, -34.0, 15
	if !q.cfg.Classic && q.anim.Rand.IntN(3) > 1 {
		name, speed, left, fromBottom = "big_fish_2", 2.5, -33, 14
	}
	ae := art.get(name, dir, 0)
	ae.Color = []string{q.randColor(ae.Color[0])}
	q.simple(BigFish, dir, ae, q.startX(dir, left, -1), q.randY(9, fromBottom), depthShark, speed, 0, engine.Yellow.Bright(), onDone)
}

func (q *Aquarium) addSwordfish(dir int, onDone func()) {
	ae := art.get("sword_fish", dir, 0)
	ae.Color = []string{q.randColor(ae.Color[0])}
	q.simple(Swordfish, dir, ae, q.startX(dir, -33, -1), q.randY(9, 14), depthShark, 3.5, 0, engine.Yellow.Bright(), onDone)
}

func (q *Aquarium) addDucks(dir int, onDone func()) {
	q.simple(Ducks, dir, art.get("ducks", dir, 0), q.startX(dir, -30, -2), 5, depthWaterGap3, 1, 0.25, engine.White.Bright(), onDone)
}

func (q *Aquarium) addSwan(dir int, onDone func()) {
	q.simple(Swan, dir, art.get("swan", dir, 0), q.startX(dir, -10, -2), 1, depthWaterGap3, 1, 0.25, engine.White.Bright(), onDone)
}

func (q *Aquarium) addShark(dir int, onDone func()) {
	a := q.anim
	speed := 2 * sign(dir)
	x := -53.0
	y := float64(a.Rand.IntN(max(a.H-(10+9), 1)) + 9)
	teethX := -9.0
	if dir == 1 {
		x = float64(a.W - 2)
		teethX = x + 9
	}
	teeth := a.Add(&engine.Entity{
		Type:     "teeth",
		Frames:   []*engine.Sprite{engine.NewSprite("*", engine.SpriteOpts{})},
		X:        teethX,
		Y:        y + 7,
		Z:        depthShark + 1,
		Depth:    depthFishEnd - depthFishStart,
		DX:       speed,
		Physical: true,
	})
	ae := art.get("shark", dir, 1)
	a.Add(&engine.Entity{
		Type:         Shark,
		Frames:       ae.sprites(true, engine.Cyan.Bright()),
		X:            x,
		Y:            y,
		Z:            depthShark,
		DX:           speed,
		DieOffscreen: true,
		OnDeath: func(*engine.Entity, *engine.Anim) {
			a.Remove(teeth)
			onDone()
		},
	})
}

// retract pulls the fishhook, line and whatever got caught back up.
func (q *Aquarium) retract(e *engine.Entity) {
	e.Physical = false
	if e.Type == "fish" {
		e.Z = depthWaterGap2
		e.Update = q.hookUpdate
		e.Data = hooked{prev: e.Data}
		return
	}
	e.Data = hooked{}
}

type hooked struct{ prev any }

// hookUpdate lowers the hook until it is 3/4 of the way down, and reels
// it (and any catch) in once something is hooked.
func (q *Aquarium) hookUpdate(e *engine.Entity, a *engine.Anim) {
	if _, ok := e.Data.(hooked); ok {
		e.Y -= a.Scale
		return
	}
	_, h := e.Size()
	if e.Y+float64(h) < float64(a.H)*0.75 {
		e.Y += a.Scale
	}
}

// hookTimeout is how many base ticks an unlucky hook waits before it is
// reeled back in empty. (The Perl version waited forever, which stalled the
// random creature rotation when no fish swam by.)
const hookTimeout = 10 * 90

func (q *Aquarium) addFishhook(dir int, onDone func()) {
	a := q.anim
	x := float64(10 + a.Rand.IntN(max(a.W-20, 1)))
	y := -4.0
	line := a.Add(&engine.Entity{
		Type:   "fishline",
		Frames: art.get("fishhook", 0, 0).sprites(true, engine.White),
		X:      x + 7,
		Y:      y - 50,
		Z:      depthWaterLine1,
		Update: q.hookUpdate,
	})
	point := a.Add(&engine.Entity{
		Type:     "hook_point",
		Frames:   art.get("fishhook", 0, 2).sprites(false, engine.Green.Bright()),
		X:        x + 1,
		Y:        y + 2,
		Z:        depthShark + 1,
		Depth:    depthFishEnd - depthFishStart,
		Physical: true,
		Update:   q.hookUpdate,
	})
	var waited float64
	hook := &engine.Entity{
		Type:         "fishhook",
		Frames:       art.get("fishhook", 0, 1).sprites(true, engine.Green.Bright()),
		X:            x,
		Y:            y,
		Z:            depthWaterLine1,
		DieOffscreen: true,
		OnDeath: func(*engine.Entity, *engine.Anim) {
			a.Remove(line)
			a.Remove(point)
			onDone()
		},
	}
	hook.Update = func(e *engine.Entity, a *engine.Anim) {
		waited += a.Scale
		if _, ok := e.Data.(hooked); !ok && waited > hookTimeout {
			for _, x := range []*engine.Entity{e, line, point} {
				q.retract(x)
			}
		}
		q.hookUpdate(e, a)
	}
	a.Add(hook)
}

func (q *Aquarium) addDolphins(dir int, onDone func()) {
	a := q.anim
	speed := sign(dir)
	distance := 15 * speed
	x := -13.0
	if dir == 1 {
		x = float64(a.W - 2)
	}
	up := engine.PathStep{speed, -0.5, 0, 0.5}
	down := engine.PathStep{speed, 0.5, 0, 0.5}
	glide := engine.PathStep{speed, 0, 0, 0.5}
	var path []engine.PathStep
	for i := 0; i < 14; i++ {
		path = append(path, up)
	}
	for i := 0; i < 2; i++ {
		path = append(path, glide)
	}
	for i := 0; i < 14; i++ {
		path = append(path, down)
	}
	for i := 0; i < 6; i++ {
		path = append(path, glide)
	}
	mk := func(idx int, dx, y, start float64, def engine.Color) *engine.Entity {
		return a.Add(&engine.Entity{
			Type:    Dolphins,
			Frames:  art.get("dolphins", dir, idx).sprites(true, def),
			X:       x - dx,
			Y:       y,
			Z:       depthWaterGap3,
			Path:    path,
			PathPos: start,
		})
	}
	d3 := mk(0, distance*2, 8, 0, engine.Blue)
	d2 := mk(1, distance, 2, 12, engine.Blue.Bright())
	d1 := mk(2, 0, 5, 24, engine.Cyan.Bright())
	// The lead dolphin dies offscreen first and then tells the others to,
	// since they start offscreen. The last one triggers the next creature.
	d1.DieOffscreen = true
	d3.OnDeath = func(*engine.Entity, *engine.Anim) { onDone() }
	d1.OnDeath = func(*engine.Entity, *engine.Anim) {
		d2.DieOffscreen = true
		d3.DieOffscreen = true
	}
}

func (q *Aquarium) addSubmarine(dir int, onDone func()) {
	a := q.anim
	speed := sign(dir)
	x := -40.0
	if dir == 1 {
		x = float64(a.W - 2)
	}
	frameDelay := []float64{1, 4, 4, 9, 9, 9, 4, 4, 1}
	frame, delay := 0, 0.0
	center := float64(a.W)/2 - 20
	e := q.simple(Submarine, dir, art.get("submarine", dir, 0), x, 6, depthWaterGap3, 1, 0, engine.Yellow.Bright(), onDone)
	e.Update = func(e *engine.Entity, a *engine.Anim) {
		step := speed * a.Scale
		crossing := (e.X < center && e.X+step > center) || (e.X > center && e.X+step < center) || e.X == center
		switch {
		case !crossing:
			e.X += step
		case frame < len(frameDelay)-1:
			// park in the middle and run the periscope animation
			e.X = center
			if delay < frameDelay[frame] {
				delay += a.Scale
			} else {
				delay = 0
				frame++
			}
		default:
			e.X += step
		}
		e.Frame = float64(frame)
	}
}

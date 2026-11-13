package engine

import (
	"math"
	"math/rand/v2"
	"sort"
	"time"
)

// BaseTickRate is the tick rate the original Perl asciiquarium ran at
// (curses halfdelay(1) gives roughly ten updates a second). All speeds in
// the scene are expressed per base tick, and the engine scales them to the
// configured tick rate so motion looks the same at any frame rate.
const BaseTickRate = 10.0

// PathStep is one step of a movement path: dx, dy, dz and frame advance.
type PathStep [4]float64

// Entity is an animated object on screen.
type Entity struct {
	ID   uint64
	Type string

	Frames []*Sprite
	// Frame is the current animation frame (fractional; the integer part
	// selects the sprite).
	Frame float64

	X, Y, Z float64
	// Depth is the number of z layers the entity occupies for collision
	// purposes. Zero is treated as one.
	Depth int

	// DX, DY, DZ and FrameSpeed are per-base-tick deltas used by the
	// default movement.
	DX, DY, DZ, FrameSpeed float64

	// Path, when set, replaces DX/DY/DZ/FrameSpeed: each tick applies the
	// next step, looping.
	Path    []PathStep
	PathPos float64

	DieOffscreen bool
	DieTime      time.Time
	// DieAfter kills the entity after this many base ticks (0 = never).
	DieAfter float64

	Physical bool

	// Label is optional text drawn centered under the entity, e.g. the
	// name of a viewer's fish.
	Label      string
	LabelColor Color

	// Update replaces the default movement when set.
	Update func(e *Entity, a *Anim)
	// OnCollide is called with the physical entities this entity overlaps.
	OnCollide func(e *Entity, a *Anim, hits []*Entity)
	// OnDeath is called when the entity dies (but not when it is removed
	// with Anim.Remove).
	OnDeath func(e *Entity, a *Anim)

	// Data is free for scene code.
	Data any

	age  float64
	dead bool
}

// Sprite returns the current frame.
func (e *Entity) Sprite() *Sprite {
	n := len(e.Frames)
	i := int(e.Frame) % n
	if i < 0 {
		i += n
	}
	return e.Frames[i]
}

// Size returns the width and height of the current frame.
func (e *Entity) Size() (int, int) {
	s := e.Sprite()
	return s.W, s.H
}

// Kill marks the entity dead; its OnDeath runs at the end of the tick.
func (e *Entity) Kill() { e.dead = true }

// Dead reports whether the entity has been killed.
func (e *Entity) Dead() bool { return e.dead }

func (e *Entity) ix() int { return int(math.Floor(e.X)) }
func (e *Entity) iy() int { return int(math.Floor(e.Y)) }

// Anim holds all entities and advances them.
type Anim struct {
	W, H int
	// Scale converts per-base-tick deltas to per-tick deltas
	// (BaseTickRate / tick rate).
	Scale float64
	Rand  *rand.Rand
	Now   func() time.Time

	ents   []*Entity
	nextID uint64
}

// NewAnim creates an animation of w x h cells running at tickRate ticks per
// second.
func NewAnim(w, h int, tickRate float64, rng *rand.Rand) *Anim {
	if tickRate <= 0 {
		tickRate = BaseTickRate
	}
	return &Anim{W: w, H: h, Scale: BaseTickRate / tickRate, Rand: rng, Now: time.Now}
}

// Add adds an entity and returns it.
func (a *Anim) Add(e *Entity) *Entity {
	a.nextID++
	e.ID = a.nextID
	a.ents = append(a.ents, e)
	return e
}

// Remove deletes an entity without running its death callback.
func (a *Anim) Remove(e *Entity) {
	for i, x := range a.ents {
		if x == e {
			a.ents = append(a.ents[:i], a.ents[i+1:]...)
			return
		}
	}
}

// RemoveAll deletes every entity.
func (a *Anim) RemoveAll() { a.ents = nil }

// Entities returns the live entities (do not modify the slice).
func (a *Anim) Entities() []*Entity { return a.ents }

// OfType returns all entities with the given type.
func (a *Anim) OfType(t string) []*Entity {
	var out []*Entity
	for _, e := range a.ents {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// Chance returns true with probability p per base tick, scaled to the
// current tick rate.
func (a *Anim) Chance(p float64) bool { return a.Rand.Float64() < p*a.Scale }

// Move applies the entity's default movement.
func (a *Anim) Move(e *Entity) {
	s := a.Scale
	if len(e.Path) > 0 {
		step := e.Path[int(e.PathPos)%len(e.Path)]
		e.X += step[0] * s
		e.Y += step[1] * s
		e.Z += step[2] * s
		e.Frame += step[3] * s
		e.PathPos += s
		return
	}
	e.X += e.DX * s
	e.Y += e.DY * s
	e.Z += e.DZ * s
	e.Frame += e.FrameSpeed * s
}

func (a *Anim) offscreen(e *Entity) bool {
	w, h := e.Size()
	x, y := e.ix(), e.iy()
	return x+w <= 0 || x >= a.W || y+h <= 0 || y >= a.H
}

func overlap(a, b *Entity) bool {
	aw, ah := a.Size()
	bw, bh := b.Size()
	ax, ay, bx, by := a.ix(), a.iy(), b.ix(), b.iy()
	if ax >= bx+bw || bx >= ax+aw || ay >= by+bh || by >= ay+ah {
		return false
	}
	ad, bd := max(a.Depth, 1), max(b.Depth, 1)
	az, bz := int(a.Z), int(b.Z)
	return az < bz+bd && bz < az+ad
}

// Tick advances the animation by one tick.
func (a *Anim) Tick() {
	ents := append([]*Entity(nil), a.ents...)
	for _, e := range ents {
		if e.dead {
			continue
		}
		if e.Update != nil {
			e.Update(e, a)
		} else {
			a.Move(e)
		}
		e.age += a.Scale
	}

	// collisions between physical entities
	var phys []*Entity
	for _, e := range a.ents {
		if e.Physical && !e.dead {
			phys = append(phys, e)
		}
	}
	hits := map[*Entity][]*Entity{}
	for i := 0; i < len(phys); i++ {
		for j := i + 1; j < len(phys); j++ {
			if overlap(phys[i], phys[j]) {
				hits[phys[i]] = append(hits[phys[i]], phys[j])
				hits[phys[j]] = append(hits[phys[j]], phys[i])
			}
		}
	}
	for _, e := range phys {
		if h := hits[e]; len(h) > 0 && e.OnCollide != nil && !e.dead {
			e.OnCollide(e, a, h)
		}
	}

	// deaths
	now := a.Now()
	var dying []*Entity
	kept := a.ents[:0]
	for _, e := range a.ents {
		if e.dead ||
			(e.DieOffscreen && a.offscreen(e)) ||
			(!e.DieTime.IsZero() && now.After(e.DieTime)) ||
			(e.DieAfter > 0 && e.age >= e.DieAfter) {
			e.dead = true
			dying = append(dying, e)
			continue
		}
		kept = append(kept, e)
	}
	for i := len(kept); i < len(a.ents); i++ {
		a.ents[i] = nil
	}
	a.ents = kept
	for _, e := range dying {
		if e.OnDeath != nil {
			e.OnDeath(e, a)
		}
	}
}

// Grid is a screen of cells.
type Grid struct {
	W, H  int
	Cells []Cell
}

// NewGrid allocates a blank grid.
func NewGrid(w, h int) *Grid {
	g := &Grid{W: w, H: h, Cells: make([]Cell, w*h)}
	g.Clear()
	return g
}

// Clear fills the grid with spaces.
func (g *Grid) Clear() {
	for i := range g.Cells {
		g.Cells[i] = Cell{Ch: ' ', Fg: White}
	}
}

// Set writes a cell if it is on screen.
func (g *Grid) Set(x, y int, c Cell) {
	if x >= 0 && y >= 0 && x < g.W && y < g.H {
		g.Cells[y*g.W+x] = c
	}
}

// At returns the cell at (x, y).
func (g *Grid) At(x, y int) Cell { return g.Cells[y*g.W+x] }

// Text writes a string starting at (x, y).
func (g *Grid) Text(x, y int, s string, fg Color) {
	for _, r := range s {
		g.Set(x, y, Cell{Ch: r, Fg: fg})
		x++
	}
}

// String renders the grid as plain text (for tests and debugging).
func (g *Grid) String() string {
	b := make([]rune, 0, (g.W+1)*g.H)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			b = append(b, g.Cells[y*g.W+x].Ch)
		}
		b = append(b, '\n')
	}
	return string(b)
}

// Render draws all entities into g, back to front (higher Z is further
// back).
func (a *Anim) Render(g *Grid) {
	g.Clear()
	order := append([]*Entity(nil), a.ents...)
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].Z != order[j].Z {
			return order[i].Z > order[j].Z
		}
		return order[i].ID < order[j].ID
	})
	for _, e := range order {
		s := e.Sprite()
		ex, ey := e.ix(), e.iy()
		for y := 0; y < s.H; y++ {
			gy := ey + y
			if gy < 0 || gy >= g.H {
				continue
			}
			for x := 0; x < s.W; x++ {
				c := s.cells[y*s.W+x]
				if c.Opaque {
					g.Set(ex+x, gy, c.Cell)
				}
			}
		}
		if e.Label != "" {
			lx := ex + (s.W-len([]rune(e.Label)))/2
			g.Text(lx, ey+s.H, e.Label, e.LabelColor)
		}
	}
}

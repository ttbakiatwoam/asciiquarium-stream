package aquarium

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
	"unicode"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// Config controls the scene and its safety limits.
type Config struct {
	// Classic only shows species from asciiquarium 1.0.
	Classic bool
	// TickRate is the number of animation ticks per second.
	TickRate float64

	// MaxViewerFish caps how many viewer fish can be in the tank at once.
	MaxViewerFish int
	// ViewerFishLifetime is how long a viewer's fish stays.
	ViewerFishLifetime time.Duration
	// MaxExtraCreatures caps on-demand creatures on screen at once (on top
	// of the ambient one).
	MaxExtraCreatures int
	// MaxNameLen limits viewer fish names.
	MaxNameLen int
	// MaxBannerLen limits banner text.
	MaxBannerLen int
	// CastleFlag is written on the castle's flag. Empty keeps the
	// original plain flag.
	CastleFlag string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		TickRate:           20,
		MaxViewerFish:      25,
		ViewerFishLifetime: 15 * time.Minute,
		MaxExtraCreatures:  3,
		MaxNameLen:         20,
		MaxBannerLen:       100,
		CastleFlag:         "ASCIIFISH.TV",
	}
}

// Errors returned by the interactive commands.
var (
	ErrTankFull      = errors.New("the tank is full, try again later")
	ErrTooManyActive = errors.New("too many creatures on screen, try again later")
	ErrUnknownKind   = errors.New("unknown creature")
	ErrUnknownColor  = errors.New("unknown color")
	ErrEmptyText     = errors.New("text is empty")
	ErrNotFound      = errors.New("not found")
)

type viewerFish struct {
	ID      uint64
	Name    string
	Color   string
	Expires time.Time
	spec    fishSpec
	entity  *engine.Entity
	removed bool
}

type banner struct {
	Text    string
	Dur     time.Duration
	Expires time.Time
	Style   BannerStyle
}

// BannerStyle selects where and how a banner is drawn.
type BannerStyle string

const (
	// BannerAlert is a boxed message near the top middle (donations, subs).
	BannerAlert BannerStyle = "alert"
	// BannerTicker is a single line in the bottom left (now playing).
	BannerTicker BannerStyle = "ticker"
)

// Aquarium is the whole scene. It is not safe for concurrent use; the
// caller serializes access (see cmd/tank).
type Aquarium struct {
	cfg    Config
	anim   *engine.Anim
	grid   *engine.Grid
	viewer []*viewerFish
	nextID uint64
	extra  int
	alerts []banner // queued; the first one is showing
	ticker *banner
}

// New creates an aquarium of w x h character cells.
func New(w, h int, cfg Config, seed uint64) *Aquarium {
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}
	q := &Aquarium{cfg: cfg}
	q.anim = engine.NewAnim(w, h, cfg.TickRate, rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)))
	q.Reset(w, h)
	return q
}

// Size returns the scene size in cells.
func (q *Aquarium) Size() (int, int) { return q.anim.W, q.anim.H }

// Reset rebuilds the scene at a new size. Viewer fish are kept.
func (q *Aquarium) Reset(w, h int) {
	q.anim.RemoveAll()
	q.anim.W, q.anim.H = w, h
	q.grid = engine.NewGrid(w, h)
	q.extra = 0
	q.addEnvironment()
	q.addCastle()
	q.addAllSeaweed()
	q.addAllFish()
	q.randomObject()
	for _, vf := range q.viewer {
		vf.entity = nil
		q.addFishEntity(vf.spec)
	}
}

// Tick advances the scene one tick.
func (q *Aquarium) Tick() {
	q.anim.Tick()
}

// Render draws the current frame and returns it. The grid is reused
// between calls.
func (q *Aquarium) Render() *engine.Grid {
	q.anim.Render(q.grid)
	q.drawBanners(q.grid)
	return q.grid
}

// SanitizeText strips anything but printable ASCII, collapses whitespace
// and truncates to n characters. Fonts on the stream only cover ASCII, and
// this keeps control characters and lookalike tricks out.
func SanitizeText(s string, n int) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if r < 0x21 || r > 0x7e {
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	out := b.String()
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// FishRequest asks for a viewer fish.
type FishRequest struct {
	// Name is shown under the fish. Optional.
	Name string `json:"name"`
	// Color is the body color, e.g. "red" or "RED" (bright). Optional.
	Color string `json:"color"`
	// Lifetime overrides the default lifetime (capped at 4x the default).
	Lifetime time.Duration `json:"-"`
}

// FishInfo describes a viewer fish.
type FishInfo struct {
	ID      uint64    `json:"id"`
	Name    string    `json:"name"`
	Color   string    `json:"color,omitempty"`
	Expires time.Time `json:"expires"`
}

// AddViewerFish adds a named fish to the tank.
func (q *Aquarium) AddViewerFish(req FishRequest) (FishInfo, error) {
	if len(q.viewer) >= q.cfg.MaxViewerFish {
		return FishInfo{}, ErrTankFull
	}
	spec := q.randomFishSpec(q.fishSet())
	spec.palette = q.randPalette()
	if req.Color != "" {
		c, ok := engine.ParseColor(req.Color)
		if !ok || c.Base() == engine.Black {
			return FishInfo{}, fmt.Errorf("%w %q (use one of %s)", ErrUnknownColor, req.Color, strings.Join(engine.ColorNames()[1:], ", "))
		}
		spec.palette[1] = colorCode(c)
	}
	life := q.cfg.ViewerFishLifetime
	if req.Lifetime > 0 {
		life = min(req.Lifetime, 4*q.cfg.ViewerFishLifetime)
	}
	q.nextID++
	vf := &viewerFish{
		ID:      q.nextID,
		Name:    SanitizeText(req.Name, q.cfg.MaxNameLen),
		Color:   req.Color,
		Expires: q.anim.Now().Add(life),
	}
	spec.viewer = vf
	vf.spec = spec
	q.viewer = append(q.viewer, vf)
	q.addFishEntity(spec)
	return vf.info(), nil
}

func (vf *viewerFish) info() FishInfo {
	return FishInfo{ID: vf.ID, Name: vf.Name, Color: vf.Color, Expires: vf.Expires}
}

func (q *Aquarium) dropViewerFish(vf *viewerFish) {
	for i, x := range q.viewer {
		if x == vf {
			q.viewer = append(q.viewer[:i], q.viewer[i+1:]...)
			return
		}
	}
}

// ViewerFish lists the viewer fish in the tank.
func (q *Aquarium) ViewerFish() []FishInfo {
	out := make([]FishInfo, 0, len(q.viewer))
	for _, vf := range q.viewer {
		out = append(out, vf.info())
	}
	return out
}

// RemoveViewerFish removes one viewer fish immediately (moderation).
func (q *Aquarium) RemoveViewerFish(id uint64) error {
	for _, vf := range q.viewer {
		if vf.ID == id {
			q.removeViewer(vf)
			return nil
		}
	}
	return ErrNotFound
}

// ClearViewerFish removes all viewer fish immediately (moderation).
func (q *Aquarium) ClearViewerFish() int {
	n := len(q.viewer)
	for _, vf := range append([]*viewerFish(nil), q.viewer...) {
		q.removeViewer(vf)
	}
	return n
}

func (q *Aquarium) removeViewer(vf *viewerFish) {
	vf.removed = true
	if vf.entity != nil {
		q.anim.Remove(vf.entity)
		vf.entity = nil
	}
	// viewer fish swim on top of the ambient population, so nothing
	// needs to replace them
	q.dropViewerFish(vf)
}

// SpawnCreature adds a creature right now, on top of the ambient one.
// kind "fish" is not a creature; use AddViewerFish.
func (q *Aquarium) SpawnCreature(kind string) error {
	fn, ok := creatures[strings.ToLower(kind)]
	if !ok {
		return fmt.Errorf("%w %q (use one of %s)", ErrUnknownKind, kind, strings.Join(CreatureKinds, ", "))
	}
	if q.extra >= q.cfg.MaxExtraCreatures {
		return ErrTooManyActive
	}
	q.extra++
	fn(q, q.anim.Rand.IntN(2), func() { q.extra-- })
	return nil
}

// ShowBanner queues an alert or replaces the ticker line.
func (q *Aquarium) ShowBanner(text string, d time.Duration, style BannerStyle) error {
	text = SanitizeText(text, q.cfg.MaxBannerLen)
	if text == "" {
		return ErrEmptyText
	}
	if d <= 0 {
		d = 8 * time.Second
	}
	d = min(d, 60*time.Second)
	switch style {
	case BannerTicker:
		q.ticker = &banner{Text: text, Expires: q.anim.Now().Add(d), Style: style}
	default:
		if len(q.alerts) >= 20 {
			return ErrTooManyActive
		}
		// the expiry of a queued alert is set when it starts showing
		q.alerts = append(q.alerts, banner{Text: text, Dur: d, Style: BannerAlert})
	}
	return nil
}

// Stats summarizes the tank.
type Stats struct {
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Entities      int    `json:"entities"`
	Fish          int    `json:"fish"`
	ViewerFish    int    `json:"viewer_fish"`
	MaxViewerFish int    `json:"max_viewer_fish"`
	ExtraActive   int    `json:"extra_creatures"`
	QueuedAlerts  int    `json:"queued_alerts"`
	Ticker        string `json:"ticker,omitempty"`
}

// Stats returns a summary of the tank.
func (q *Aquarium) Stats() Stats {
	s := Stats{
		Width: q.anim.W, Height: q.anim.H,
		Entities:      len(q.anim.Entities()),
		Fish:          len(q.anim.OfType("fish")),
		ViewerFish:    len(q.viewer),
		MaxViewerFish: q.cfg.MaxViewerFish,
		ExtraActive:   q.extra,
		QueuedAlerts:  len(q.alerts),
	}
	if q.ticker != nil {
		s.Ticker = q.ticker.Text
	}
	return s
}

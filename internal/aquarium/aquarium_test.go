package aquarium

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

func newTest(t *testing.T) (*Aquarium, *time.Time) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.TickRate = 10
	q := New(160, 50, cfg, 42)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q.anim.Now = func() time.Time { return now }
	return q, &now
}

func run(q *Aquarium, now *time.Time, ticks int) {
	for i := 0; i < ticks; i++ {
		*now = now.Add(100 * time.Millisecond)
		q.Tick()
	}
}

func TestSceneRendersAndRuns(t *testing.T) {
	q, now := newTest(t)
	run(q, now, 3000) // five simulated minutes
	g := q.Render()
	s := g.String()
	if !strings.Contains(s, "~~~~~~~~") {
		t.Fatalf("water line missing:\n%s", s)
	}
	if !strings.Contains(s, "[ ]_[ ]_[ ]") {
		t.Fatalf("castle missing:\n%s", s)
	}
	if n := len(q.anim.OfType("fish")); n < 10 {
		t.Fatalf("expected ambient fish to keep respawning, have %d", n)
	}
	// the ambient creature rotation must keep going: exactly one random
	// object family alive at a time
	if len(q.anim.Entities()) > 200 {
		t.Fatalf("entity leak: %d entities", len(q.anim.Entities()))
	}
}

func TestEveryCreatureSpawnsAndLeaves(t *testing.T) {
	for _, kind := range CreatureKinds {
		t.Run(kind, func(t *testing.T) {
			q, now := newTest(t)
			if err := q.SpawnCreature(kind); err != nil {
				t.Fatal(err)
			}
			if q.extra != 1 {
				t.Fatalf("extra = %d", q.extra)
			}
			run(q, now, 4000)
			if q.extra != 0 {
				t.Fatalf("%s never left the screen", kind)
			}
		})
	}
}

func TestSpawnLimits(t *testing.T) {
	q, _ := newTest(t)
	if err := q.SpawnCreature("kraken"); !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("got %v", err)
	}
	for i := 0; i < q.cfg.MaxExtraCreatures; i++ {
		if err := q.SpawnCreature(Shark); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.SpawnCreature(Shark); !errors.Is(err, ErrTooManyActive) {
		t.Fatalf("got %v", err)
	}
}

func TestViewerFish(t *testing.T) {
	q, now := newTest(t)
	info, err := q.AddViewerFish(FishRequest{Name: "  Bob\x1b[31m the\tfish  ", Color: "RED"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Bob[31m the fish" {
		t.Fatalf("name not sanitized: %q", info.Name)
	}
	if _, err := q.AddViewerFish(FishRequest{Color: "plaid"}); !errors.Is(err, ErrUnknownColor) {
		t.Fatalf("got %v", err)
	}
	// it keeps swimming back and forth until it expires
	run(q, now, 10*60*5)
	if len(q.viewer) != 1 || q.viewer[0].entity == nil {
		t.Fatalf("viewer fish gone early")
	}
	run(q, now, 10*60*15)
	if len(q.viewer) != 0 {
		t.Fatalf("viewer fish did not expire")
	}
}

func TestViewerFishCapAndClear(t *testing.T) {
	q, _ := newTest(t)
	for i := 0; i < q.cfg.MaxViewerFish; i++ {
		if _, err := q.AddViewerFish(FishRequest{Name: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.AddViewerFish(FishRequest{}); !errors.Is(err, ErrTankFull) {
		t.Fatalf("got %v", err)
	}
	if n := q.ClearViewerFish(); n != q.cfg.MaxViewerFish {
		t.Fatalf("cleared %d", n)
	}
	for _, e := range q.anim.OfType("fish") {
		if e.Data != nil {
			t.Fatal("viewer fish entity left behind")
		}
	}
}

func TestBanners(t *testing.T) {
	q, now := newTest(t)
	if err := q.ShowBanner("Thanks for the sub, Alice!", 5*time.Second, BannerAlert); err != nil {
		t.Fatal(err)
	}
	if err := q.ShowBanner("Second alert", 5*time.Second, BannerAlert); err != nil {
		t.Fatal(err)
	}
	if err := q.ShowBanner("Now playing: Deep Blue", 20*time.Second, BannerTicker); err != nil {
		t.Fatal(err)
	}
	s := q.Render().String()
	if !strings.Contains(s, "Thanks for the sub, Alice!") || !strings.Contains(s, "Now playing: Deep Blue") {
		t.Fatalf("banners missing:\n%s", s)
	}
	if strings.Contains(s, "Second alert") {
		t.Fatal("alerts should show one at a time")
	}
	run(q, now, 60)
	s = q.Render().String()
	if !strings.Contains(s, "Second alert") {
		t.Fatalf("queued alert did not show:\n%s", s)
	}
	if err := q.ShowBanner("\x00\x01", 0, BannerAlert); !errors.Is(err, ErrEmptyText) {
		t.Fatalf("got %v", err)
	}
}

func TestSpriteTransparency(t *testing.T) {
	s := engine.NewSprite("\n  ab?c\n", engine.SpriteOpts{AutoTrans: true, Mask: "\n  r  \n"})
	if _, ok := s.At(0, 0); ok {
		t.Fatal("leading space should be transparent")
	}
	if _, ok := s.At(4, 0); ok {
		t.Fatal("? should be transparent")
	}
	c, ok := s.At(2, 0)
	if !ok || c.Ch != 'a' || c.Fg != engine.Red {
		t.Fatalf("got %+v %v", c, ok)
	}
}

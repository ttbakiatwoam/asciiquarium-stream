package render

import (
	"testing"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/aquarium"
)

// BenchmarkFrame1080p measures one tick + render + framebuffer paint of a
// busy 1080p scene, which is the per-frame work on the Raspberry Pi.
func BenchmarkFrame1080p(b *testing.B) {
	f, err := NewFont(20)
	if err != nil {
		b.Fatal(err)
	}
	c := NewCanvas(1920, 1080, 1920*4, make([]byte, 1920*1080*4), XRGB8888, f)
	cfg := aquarium.DefaultConfig()
	q := aquarium.New(c.Cols, c.Rows, cfg, 3)
	for i := 0; i < 10; i++ {
		q.AddViewerFish(aquarium.FishRequest{Name: "viewer"})
	}
	for i := 0; i < 200; i++ {
		q.Tick()
		c.Draw(q.Render())
	}
	b.ResetTimer()
	cells := 0
	for i := 0; i < b.N; i++ {
		q.Tick()
		cells += c.Draw(q.Render())
	}
	b.ReportMetric(float64(cells)/float64(b.N), "cells/frame")
}

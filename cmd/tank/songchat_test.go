package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/music"
)

func TestSongChatMessage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "links.txt")
	os.WriteFile(p, []byte("# per-song links\nGalaxy Drift #7 = https://example.com/gd7\n"), 0o644)
	links, err := loadLinks(p)
	if err != nil {
		t.Fatal(err)
	}
	s := &songChat{template: "Now playing: {title} | Buy it: {link}", defaultLink: "https://example.com/all", links: links}
	gd7 := music.Track{Path: "/m/Galaxy Drift #7.mp3", Tags: music.Tags{Title: "Galaxy Drift #7"}}
	if got := s.message(gd7); got != "Now playing: Galaxy Drift #7 | Buy it: https://example.com/gd7" {
		t.Fatalf("got %q", got)
	}
	other := music.Track{Path: "/m/Espresso Lounge #1.mp3", Tags: music.Tags{Title: "Espresso Lounge #1"}}
	if got := s.message(other); got != "Now playing: Espresso Lounge #1 | Buy it: https://example.com/all" {
		t.Fatalf("got %q", got)
	}
	s.defaultLink = ""
	s.template = "Now playing: {title} | {link}"
	if got := s.message(other); got != "Now playing: Espresso Lounge #1" {
		t.Fatalf("got %q", got)
	}
}

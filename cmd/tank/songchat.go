package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/music"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/twitch"
)

// songChat announces song changes in Twitch chat, at most once per
// minInterval so short songs don't flood chat.
type songChat struct {
	chat        *twitch.Chat
	template    string
	defaultLink string
	links       map[string]string
	minInterval time.Duration
	log         *slog.Logger

	mu   sync.Mutex
	last time.Time
}

// loadLinks reads "song name = url" lines. The song name is the file name
// without .mp3 (or the title tag); blank lines and # comments are ignored.
func loadLinks(path string) (map[string]string, error) {
	links := map[string]string{}
	if path == "" {
		return links, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, url, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		links[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(url)
	}
	return links, sc.Err()
}

func (s *songChat) link(t music.Track) string {
	base := strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
	for _, k := range []string{base, t.Tags.Title} {
		if u, ok := s.links[strings.ToLower(k)]; ok {
			return u
		}
	}
	return s.defaultLink
}

func (s *songChat) message(t music.Track) string {
	msg := strings.NewReplacer("{title}", t.Display(), "{link}", s.link(t)).Replace(s.template)
	// drop a dangling separator when there is no link
	msg = strings.TrimRight(strings.TrimSpace(msg), "|:- ")
	return msg
}

// announce posts the track if enough time has passed since the last post.
func (s *songChat) announce(ctx context.Context, t music.Track) {
	s.mu.Lock()
	if !s.last.IsZero() && time.Since(s.last) < s.minInterval {
		s.mu.Unlock()
		return
	}
	s.last = time.Now()
	s.mu.Unlock()
	msg := s.message(t)
	if err := s.chat.Send(ctx, msg); err != nil {
		s.log.Error("could not post song to chat", "err", err)
		return
	}
	s.log.Info("posted song to chat", "message", msg)
}

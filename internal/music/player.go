package music

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Track is a song being played.
type Track struct {
	Path    string
	Tags    Tags
	Started time.Time
}

// Display returns "Title" or "Title - Artist".
func (t Track) Display() string {
	if t.Tags.Artist != "" {
		return t.Tags.Title + " - " + t.Tags.Artist
	}
	return t.Tags.Title
}

// Player plays every MP3 in Dir, shuffled, forever. The folder is rescanned
// before each round so new songs are picked up without a restart.
type Player struct {
	Dir string
	// Command is the player program and its arguments; the file path is
	// appended. For example: mpg123 -q -o alsa -a hw:0,0
	Command []string
	Shuffle bool
	// OnTrack is called when a new track starts.
	OnTrack func(Track)
	Log     *slog.Logger

	mu      sync.Mutex
	current *Track
}

// NowPlaying returns the current track, if any.
func (p *Player) NowPlaying() (Track, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return Track{}, false
	}
	return *p.current, true
}

// Scan lists the MP3s in Dir (recursively), sorted by path.
func (p *Player) Scan() ([]string, error) {
	var files []string
	err := filepath.WalkDir(p.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".mp3") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// Run plays until ctx is cancelled.
func (p *Player) Run(ctx context.Context) error {
	log := p.Log
	if log == nil {
		log = slog.Default()
	}
	for ctx.Err() == nil {
		files, err := p.Scan()
		if err != nil || len(files) == 0 {
			log.Warn("no songs to play", "dir", p.Dir, "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(30 * time.Second):
			}
			continue
		}
		if p.Shuffle {
			rand.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })
		}
		for _, f := range files {
			if ctx.Err() != nil {
				break
			}
			t := Track{Path: f, Tags: ReadTags(f), Started: time.Now()}
			p.mu.Lock()
			p.current = &t
			p.mu.Unlock()
			log.Info("now playing", "title", t.Tags.Title, "artist", t.Tags.Artist, "file", f)
			if p.OnTrack != nil {
				p.OnTrack(t)
			}
			args := append(append([]string(nil), p.Command[1:]...), f)
			cmd := exec.CommandContext(ctx, p.Command[0], args...)
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			start := time.Now()
			if err := cmd.Run(); err != nil && ctx.Err() == nil {
				log.Error("player failed", "file", f, "err", err)
			}
			// avoid a tight loop if the player is broken or the file
			// is empty
			if time.Since(start) < 2*time.Second {
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
				}
			}
		}
	}
	p.mu.Lock()
	p.current = nil
	p.mu.Unlock()
	return ctx.Err()
}

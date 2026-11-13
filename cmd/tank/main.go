// Command tank runs the interactive asciiquarium.
//
// It draws straight to the Linux framebuffer (the Raspberry Pi's HDMI
// output), to a terminal, or to raw video for ffmpeg, and serves a small
// HTTP API that adds viewer fish, spawns creatures and shows banners. It
// can also play a folder of MP3s and show the song title now and then.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/api"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/aquarium"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/music"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/presenter"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/render"
	"github.com/ttbakiatwoam/asciiquarium-stream/internal/twitch"
)

type options struct {
	output   string
	fbDev    string
	tty      string
	rows     int
	width    int
	height   int
	fps      float64
	classic  bool
	listen   string
	token    string
	seed     uint64
	snapshot string
	after    time.Duration
	demo     bool

	musicDir      string
	player        string
	shuffle       bool
	songEvery     time.Duration
	songSeconds   time.Duration
	maxViewerFish int
	fishLifetime  time.Duration
	maxCreatures  int
	castleFlag    string

	songSuffix string

	songChat         string
	songLink         string
	songLinks        string
	songChatInterval time.Duration
	twitch           *twitch.Chat
	presenterURL     string
	presenterWait    time.Duration
}

// env returns the TANK_<NAME> environment variable or def, so every flag
// can also be set from a systemd unit or Kubernetes manifest.
func env(name, def string) string {
	if v, ok := os.LookupEnv("TANK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))); ok {
		return v
	}
	return def
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(env(name, "")); err == nil {
		return v
	}
	return def
}

func envFloat(name string, def float64) float64 {
	if v, err := strconv.ParseFloat(env(name, ""), 64); err == nil {
		return v
	}
	return def
}

func envDur(name string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(name, "")); err == nil {
		return v
	}
	return def
}

func envBool(name string, def bool) bool {
	if v, err := strconv.ParseBool(env(name, "")); err == nil {
		return v
	}
	return def
}

func parseFlags() options {
	var o options
	flag.StringVar(&o.output, "output", env("output", "fb"), "where to draw: fb (Linux framebuffer/HDMI), term, raw (rgb24 frames on stdout for ffmpeg, no display needed), none (headless, API only), png (write -snapshot and exit)")
	flag.StringVar(&o.fbDev, "fb", env("fb", "/dev/fb0"), "framebuffer device")
	flag.StringVar(&o.tty, "tty", env("tty", ""), "console to switch to graphics mode while drawing to the framebuffer, e.g. /dev/tty1")
	flag.IntVar(&o.rows, "rows", envInt("rows", 54), "text rows on screen for fb/raw/png output (sets the font size)")
	flag.IntVar(&o.width, "width", envInt("width", 1920), "frame width for raw/png output")
	flag.IntVar(&o.height, "height", envInt("height", 1080), "frame height for raw/png output")
	flag.Float64Var(&o.fps, "fps", envFloat("fps", 20), "frames per second")
	flag.BoolVar(&o.classic, "classic", envBool("classic", false), "only show species from asciiquarium 1.0")
	flag.StringVar(&o.listen, "listen", env("listen", ":8080"), "address for the HTTP command API (empty disables it)")
	flag.StringVar(&o.token, "token", env("token", ""), "bearer token required by the API (also TANK_TOKEN)")
	flag.Uint64Var(&o.seed, "seed", uint64(envInt("seed", 0)), "random seed (0 = time based)")
	flag.StringVar(&o.snapshot, "snapshot", env("snapshot", "tank.png"), "PNG file for -output png")
	flag.DurationVar(&o.after, "after", envDur("after", 8*time.Second), "simulated time before taking the -output png snapshot")
	flag.BoolVar(&o.demo, "demo", envBool("demo", false), "add a few viewer fish, creatures and banners at start (for testing)")

	flag.StringVar(&o.musicDir, "music-dir", env("music-dir", ""), "folder of MP3s to play (empty disables the player)")
	flag.StringVar(&o.player, "player", env("player", "mpg123 -q"), "command that plays one MP3; the file is appended")
	flag.BoolVar(&o.shuffle, "shuffle", envBool("shuffle", true), "shuffle songs")
	flag.DurationVar(&o.songEvery, "song-banner-every", envDur("song-banner-every", 10*time.Minute), "how often to re-show the current song title (0 = only when a song starts)")
	flag.DurationVar(&o.songSeconds, "song-banner-seconds", envDur("song-banner-seconds", 20*time.Second), "how long the song title stays up")

	def := aquarium.DefaultConfig()
	flag.IntVar(&o.maxViewerFish, "max-viewer-fish", envInt("max-viewer-fish", def.MaxViewerFish), "most viewer fish in the tank at once")
	flag.DurationVar(&o.fishLifetime, "fish-lifetime", envDur("fish-lifetime", def.ViewerFishLifetime), "how long a viewer fish stays")
	flag.IntVar(&o.maxCreatures, "max-creatures", envInt("max-creatures", def.MaxExtraCreatures), "most on-demand creatures on screen at once")
	flag.StringVar(&o.songSuffix, "song-suffix", env("song-suffix", ""), `text added after the song title, e.g. "buy it at soundcloud.com/asciifishtv"`)
	o.twitch = &twitch.Chat{}
	flag.StringVar(&o.songChat, "song-chat", env("song-chat", "Now playing: {title} | Get the music: {link}"), "Twitch chat message posted when a song starts ({title}, {link})")
	flag.StringVar(&o.songLink, "song-link", env("song-link", ""), "link used for {link}, e.g. the SoundCloud page")
	flag.StringVar(&o.songLinks, "song-links", env("song-links", ""), `file of per-song links, one "song name = url" per line (falls back to -song-link)`)
	flag.DurationVar(&o.songChatInterval, "song-chat-min-interval", envDur("song-chat-min-interval", 3*time.Minute), "least time between song posts in chat")
	flag.StringVar(&o.twitch.ClientID, "twitch-client-id", env("twitch-client-id", ""), "Twitch app client id (song chat posts are off unless set)")
	flag.StringVar(&o.twitch.ClientSecret, "twitch-client-secret", env("twitch-client-secret", ""), "Twitch app client secret (also TANK_TWITCH_CLIENT_SECRET)")
	flag.StringVar(&o.twitch.BroadcasterID, "twitch-broadcaster-id", env("twitch-broadcaster-id", ""), "user id of the channel")
	flag.StringVar(&o.twitch.SenderID, "twitch-sender-id", env("twitch-sender-id", ""), "user id of the account that posts (needs the user:write:chat scope)")
	flag.StringVar(&o.twitch.RefreshToken, "twitch-refresh-token", env("twitch-refresh-token", ""), "OAuth refresh token for the posting account (also TANK_TWITCH_REFRESH_TOKEN)")
	flag.StringVar(&o.twitch.TokenFile, "twitch-token-file", env("twitch-token-file", ""), "file that keeps the latest refresh token across restarts")
	flag.StringVar(&o.presenterURL, "presenter-url", env("presenter-url", ""), "Blackmagic Web Presenter livestream URL to restart once the music starts, e.g. http://172.17.3.29/control/api/v1/livestreams/0 (empty disables)")
	flag.DurationVar(&o.presenterWait, "presenter-wait", envDur("presenter-wait", 30*time.Second), "pause between stopping and starting the Web Presenter stream")
	flag.StringVar(&o.castleFlag, "castle-flag", env("castle-flag", def.CastleFlag), "text on the castle's flag (empty for the plain flag)")
	flag.Parse()
	return o
}

// drawer is an output that can show a grid.
type drawer interface {
	Draw(g *engine.Grid) error
	Close() error
}

type fbDrawer struct{ *render.Framebuffer }

func (d fbDrawer) Draw(g *engine.Grid) error { d.Framebuffer.Draw(g); return nil }

type rawDrawer struct {
	c *render.Canvas
}

func (d rawDrawer) Draw(g *engine.Grid) error {
	d.c.Draw(g)
	_, err := os.Stdout.Write(d.c.Buf)
	return err
}
func (d rawDrawer) Close() error { return nil }

type nopDrawer struct{}

func (nopDrawer) Draw(*engine.Grid) error { return nil }
func (nopDrawer) Close() error            { return nil }

func main() {
	o := parseFlags()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)
	if err := run(o, log); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("tank stopped", "err", err)
		os.Exit(1)
	}
}

func run(o options, log *slog.Logger) error {
	cfg := aquarium.DefaultConfig()
	cfg.Classic = o.classic
	cfg.TickRate = o.fps
	cfg.MaxViewerFish = o.maxViewerFish
	cfg.ViewerFishLifetime = o.fishLifetime
	cfg.MaxExtraCreatures = o.maxCreatures
	cfg.CastleFlag = o.castleFlag

	if o.output == "png" {
		return snapshot(o, cfg)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var out drawer
	var cols, rows int
	var term *termInput
	switch o.output {
	case "fb":
		fb, err := render.OpenFramebuffer(o.fbDev, o.tty, 0)
		if err != nil {
			return fmt.Errorf("open framebuffer: %w", err)
		}
		if o.rows > 0 {
			fb.Close()
			fb, err = render.OpenFramebuffer(o.fbDev, o.tty, fb.H/o.rows)
			if err != nil {
				return fmt.Errorf("open framebuffer: %w", err)
			}
		}
		log.Info("drawing to framebuffer", "dev", o.fbDev, "pixels", fmt.Sprintf("%dx%d", fb.W, fb.H), "cells", fmt.Sprintf("%dx%d", fb.Cols, fb.Rows))
		out, cols, rows = fbDrawer{fb}, fb.Cols, fb.Rows
	case "raw":
		c, err := newCanvas(o, render.RGB24)
		if err != nil {
			return err
		}
		log.Info("writing rgb24 frames to stdout", "pixels", fmt.Sprintf("%dx%d", o.width, o.height), "fps", o.fps)
		out, cols, rows = rawDrawer{c}, c.Cols, c.Rows
	case "none":
		// headless: run the tank and API with nothing attached
		out, cols, rows = nopDrawer{}, 192, 54
		log.Info("headless, not drawing anywhere")
	case "term":
		t, err := newTermInput()
		if err != nil {
			return err
		}
		term = t
		defer term.restore()
		cols, rows = term.size()
		out = term.out
	default:
		return fmt.Errorf("unknown -output %q", o.output)
	}
	defer out.Close()

	q := aquarium.New(cols, rows, cfg, o.seed)
	if o.demo {
		demo(q)
	}

	cmds := make(chan func(*aquarium.Aquarium), 64)
	exec := func(f func(*aquarium.Aquarium)) {
		done := make(chan struct{})
		select {
		case cmds <- func(q *aquarium.Aquarium) { f(q); close(done) }:
		case <-ctx.Done():
			return
		}
		select {
		case <-done:
		case <-ctx.Done():
		}
	}

	// music
	var player *music.Player
	if o.musicDir != "" {
		player = &music.Player{
			Dir:     o.musicDir,
			Command: strings.Fields(o.player),
			Shuffle: o.shuffle,
			Log:     log,
			OnTrack: func(t music.Track) {
				go exec(func(q *aquarium.Aquarium) {
					q.ShowBanner(songLine(t, o.songSuffix), o.songSeconds, aquarium.BannerTicker)
				})
			},
		}
		if o.twitch.ClientID != "" {
			links, err := loadLinks(o.songLinks)
			if err != nil {
				return fmt.Errorf("song links: %w", err)
			}
			o.twitch.LoadToken()
			sc := &songChat{chat: o.twitch, template: o.songChat, defaultLink: o.songLink, links: links, minInterval: o.songChatInterval, log: log}
			show := player.OnTrack
			player.OnTrack = func(t music.Track) {
				show(t)
				go sc.announce(ctx, t)
			}
			log.Info("song changes will be posted in Twitch chat")
		}
		var once sync.Once
		onTrack := player.OnTrack
		player.OnTrack = func(t music.Track) {
			onTrack(t)
			if o.presenterURL == "" {
				return
			}
			// like the old StreamMusic script: when the music (re)starts,
			// bounce the Web Presenter stream so it picks up the audio
			once.Do(func() {
				go func() {
					log.Info("restarting Web Presenter stream", "url", o.presenterURL)
					c := &presenter.Client{BaseURL: o.presenterURL}
					if err := c.Restart(ctx, o.presenterWait); err != nil {
						log.Error("Web Presenter restart failed", "err", err)
					}
				}()
			})
		}
		go player.Run(ctx)
		if o.songEvery > 0 {
			go func() {
				tk := time.NewTicker(o.songEvery)
				defer tk.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-tk.C:
						if t, ok := player.NowPlaying(); ok && time.Since(t.Started) > o.songSeconds {
							exec(func(q *aquarium.Aquarium) {
								q.ShowBanner(songLine(t, o.songSuffix), o.songSeconds, aquarium.BannerTicker)
							})
						}
					}
				}
			}()
		}
	}

	// API
	if o.listen != "" {
		var nowPlaying func() string
		if player != nil {
			nowPlaying = func() string {
				if t, ok := player.NowPlaying(); ok {
					return t.Display()
				}
				return ""
			}
		}
		if o.token == "" {
			log.Warn("API has no token; anyone who can reach " + o.listen + " can add fish")
		}
		srv := &http.Server{Addr: o.listen, Handler: api.New(exec, o.token, api.DefaultLimits(), nowPlaying), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			log.Info("API listening", "addr", o.listen)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("API failed", "err", err)
				stop()
			}
		}()
		defer srv.Close()
	}

	// animation loop
	tick := time.NewTicker(time.Duration(float64(time.Second) / o.fps))
	defer tick.Stop()
	paused := false
	var keys <-chan byte
	var resized <-chan os.Signal
	if term != nil {
		keys = term.keys
		resized = term.resized
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case f := <-cmds:
			f(q)
		case k := <-keys:
			switch k {
			case 'q':
				return nil
			case 'p':
				paused = !paused
			case 'r':
				term.out.Invalidate()
				q.Reset(q.Size())
			}
		case <-resized:
			cols, rows := term.size()
			term.out.Invalidate()
			q.Reset(cols, rows)
		case <-tick.C:
			if !paused {
				q.Tick()
			}
			if err := out.Draw(q.Render()); err != nil {
				return err
			}
		}
	}
}

func newCanvas(o options, pf render.PixelFormat) (*render.Canvas, error) {
	f, err := render.NewFont(o.height / max(o.rows, 1))
	if err != nil {
		return nil, err
	}
	stride := o.width * pf.BytesPerPixel
	return render.NewCanvas(o.width, o.height, stride, make([]byte, stride*o.height), pf, f), nil
}

func demo(q *aquarium.Aquarium) {
	for _, n := range []struct{ name, color string }{{"andrew", "YELLOW"}, {"kelpie", "magenta"}, {"bubbles", "GREEN"}} {
		q.AddViewerFish(aquarium.FishRequest{Name: n.name, Color: n.color})
	}
	q.SpawnCreature(aquarium.Ducks)
	q.SpawnCreature(aquarium.Shark)
	q.ShowBanner("Thanks for the 5 gifted subs, kelpie! Here comes the shark!", 30*time.Second, aquarium.BannerAlert)
	q.ShowBanner("Now playing: Deep Blue Lullaby", 60*time.Second, aquarium.BannerTicker)
}

// snapshot runs the scene for o.after of simulated time and writes a PNG.
func snapshot(o options, cfg aquarium.Config) error {
	c, err := newCanvas(o, render.XRGB8888)
	if err != nil {
		return err
	}
	q := aquarium.New(c.Cols, c.Rows, cfg, o.seed)
	if o.demo {
		demo(q)
	}
	for i := 0; i < int(o.after.Seconds()*o.fps); i++ {
		q.Tick()
	}
	c.Draw(q.Render())
	f, err := os.Create(o.snapshot)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, c.Image())
}

// songLine is the ticker text for a track.
func songLine(t music.Track, suffix string) string {
	line := "Now playing: " + t.Display()
	if suffix != "" {
		line += "  |  " + suffix
	}
	return line
}

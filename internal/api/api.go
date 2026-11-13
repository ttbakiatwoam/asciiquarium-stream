// Package api is the tank's local HTTP command API. It is deliberately
// small: the bridge service (Twitch, Streamlabs, Buy Me a Coffee, Discord)
// decides who may do what, and this API enforces global safety limits so
// the screen can never be flooded, whatever the caller does.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/aquarium"
)

// Exec runs f on the animation goroutine and waits for it to finish.
type Exec func(f func(q *aquarium.Aquarium))

// Limits are the global rate limits (per second, burst).
type Limits struct {
	FishPerSec, FishBurst         float64
	CreaturePerSec, CreatureBurst float64
	BannerPerSec, BannerBurst     float64
}

// DefaultLimits allow short bursts (a raid, a gift-sub train) but cap the
// sustained rate.
func DefaultLimits() Limits {
	return Limits{
		FishPerSec: 0.5, FishBurst: 10,
		CreaturePerSec: 1.0 / 20, CreatureBurst: 3,
		BannerPerSec: 0.2, BannerBurst: 5,
	}
}

// Server serves the API.
type Server struct {
	exec       Exec
	token      string
	nowPlaying func() string
	fish       *bucket
	creature   *bucket
	banner     *bucket
	mux        *http.ServeMux
}

// New creates a server. If token is not empty, every request except
// /healthz must send "Authorization: Bearer <token>". nowPlaying may be nil.
func New(exec Exec, token string, limits Limits, nowPlaying func() string) *Server {
	s := &Server{
		exec:       exec,
		token:      token,
		nowPlaying: nowPlaying,
		fish:       newBucket(limits.FishPerSec, limits.FishBurst),
		creature:   newBucket(limits.CreaturePerSec, limits.CreatureBurst),
		banner:     newBucket(limits.BannerPerSec, limits.BannerBurst),
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	s.mux.HandleFunc("GET /v1/state", s.auth(s.state))
	s.mux.HandleFunc("GET /v1/creatures", s.auth(s.listCreatures))
	s.mux.HandleFunc("POST /v1/fish", s.auth(s.addFish))
	s.mux.HandleFunc("GET /v1/fish", s.auth(s.listFish))
	s.mux.HandleFunc("DELETE /v1/fish", s.auth(s.clearFish))
	s.mux.HandleFunc("DELETE /v1/fish/{id}", s.auth(s.removeFish))
	s.mux.HandleFunc("POST /v1/creatures/{kind}", s.auth(s.spawn))
	s.mux.HandleFunc("POST /v1/banner", s.auth(s.showBanner))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				writeErr(w, http.StatusUnauthorized, errors.New("missing or wrong bearer token"))
				return
			}
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func limited(w http.ResponseWriter, b *bucket) bool {
	ok, wait := b.allow()
	if ok {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	writeErr(w, http.StatusTooManyRequests, fmt.Errorf("slow down, try again in %s", wait.Round(time.Second)))
	return true
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, aquarium.ErrTankFull), errors.Is(err, aquarium.ErrTooManyActive):
		return http.StatusConflict
	case errors.Is(err, aquarium.ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusBadRequest
	}
}

func decode(r *http.Request, v any) error {
	if r.ContentLength == 0 {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("bad JSON: %w", err)
	}
	return nil
}

type stateResponse struct {
	aquarium.Stats
	NowPlaying string `json:"now_playing,omitempty"`
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	var st aquarium.Stats
	s.exec(func(q *aquarium.Aquarium) { st = q.Stats() })
	resp := stateResponse{Stats: st}
	if s.nowPlaying != nil {
		resp.NowPlaying = s.nowPlaying()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) listCreatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{"creatures": aquarium.CreatureKinds})
}

type fishBody struct {
	Name           string  `json:"name"`
	Color          string  `json:"color"`
	LifetimeMinute float64 `json:"lifetime_minutes"`
}

func (s *Server) addFish(w http.ResponseWriter, r *http.Request) {
	var body fishBody
	if err := decode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if limited(w, s.fish) {
		return
	}
	req := aquarium.FishRequest{Name: body.Name, Color: body.Color}
	if body.LifetimeMinute > 0 {
		req.Lifetime = time.Duration(body.LifetimeMinute * float64(time.Minute))
	}
	var info aquarium.FishInfo
	var err error
	s.exec(func(q *aquarium.Aquarium) { info, err = q.AddViewerFish(req) })
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) listFish(w http.ResponseWriter, r *http.Request) {
	var list []aquarium.FishInfo
	s.exec(func(q *aquarium.Aquarium) { list = q.ViewerFish() })
	writeJSON(w, http.StatusOK, map[string]any{"fish": list})
}

func (s *Server) clearFish(w http.ResponseWriter, r *http.Request) {
	var n int
	s.exec(func(q *aquarium.Aquarium) { n = q.ClearViewerFish() })
	writeJSON(w, http.StatusOK, map[string]int{"removed": n})
}

func (s *Server) removeFish(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, errors.New("bad fish id"))
		return
	}
	s.exec(func(q *aquarium.Aquarium) { err = q.RemoveViewerFish(id) })
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) spawn(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	known := false
	for _, k := range aquarium.CreatureKinds {
		known = known || k == strings.ToLower(kind)
	}
	if !known {
		writeErr(w, http.StatusNotFound, fmt.Errorf("%w %q (use one of %s)", aquarium.ErrUnknownKind, kind, strings.Join(aquarium.CreatureKinds, ", ")))
		return
	}
	if limited(w, s.creature) {
		return
	}
	var err error
	s.exec(func(q *aquarium.Aquarium) { err = q.SpawnCreature(kind) })
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"spawned": strings.ToLower(kind)})
}

type bannerBody struct {
	Text    string  `json:"text"`
	Seconds float64 `json:"seconds"`
	Style   string  `json:"style"`
}

func (s *Server) showBanner(w http.ResponseWriter, r *http.Request) {
	var body bannerBody
	if err := decode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	style := aquarium.BannerStyle(body.Style)
	if style == "" {
		style = aquarium.BannerAlert
	}
	if style != aquarium.BannerAlert && style != aquarium.BannerTicker {
		writeErr(w, http.StatusBadRequest, errors.New(`style must be "alert" or "ticker"`))
		return
	}
	if limited(w, s.banner) {
		return
	}
	var err error
	d := time.Duration(body.Seconds * float64(time.Second))
	s.exec(func(q *aquarium.Aquarium) { err = q.ShowBanner(body.Text, d, style) })
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"queued": true})
}

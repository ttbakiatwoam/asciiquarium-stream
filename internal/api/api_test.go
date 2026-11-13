package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/aquarium"
)

func newServer(t *testing.T, token string, l Limits) *httptest.Server {
	t.Helper()
	q := aquarium.New(160, 50, aquarium.DefaultConfig(), 1)
	var mu sync.Mutex
	exec := func(f func(*aquarium.Aquarium)) { mu.Lock(); defer mu.Unlock(); f(q) }
	ts := httptest.NewServer(New(exec, token, l, func() string { return "Deep Blue" }))
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, ts *httptest.Server, method, path, body, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestAuth(t *testing.T) {
	ts := newServer(t, "s3cret", DefaultLimits())
	if r := do(t, ts, "GET", "/healthz", "", ""); r.StatusCode != 200 {
		t.Fatalf("healthz: %d", r.StatusCode)
	}
	if r := do(t, ts, "GET", "/v1/state", "", "wrong"); r.StatusCode != 401 {
		t.Fatalf("wrong token: %d", r.StatusCode)
	}
	if r := do(t, ts, "GET", "/v1/state", "", "s3cret"); r.StatusCode != 200 {
		t.Fatalf("state: %d", r.StatusCode)
	}
}

func TestFishAndRateLimit(t *testing.T) {
	l := DefaultLimits()
	l.FishBurst = 2
	ts := newServer(t, "", l)
	for i := 0; i < 2; i++ {
		if r := do(t, ts, "POST", "/v1/fish", `{"name":"bob","color":"red"}`, ""); r.StatusCode != 201 {
			t.Fatalf("add fish: %d", r.StatusCode)
		}
	}
	r := do(t, ts, "POST", "/v1/fish", `{"name":"bob"}`, "")
	if r.StatusCode != 429 || r.Header.Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d", r.StatusCode)
	}
	if r := do(t, ts, "POST", "/v1/fish", `{"bogus":1}`, ""); r.StatusCode != 400 {
		t.Fatalf("unknown field: %d", r.StatusCode)
	}
	if r := do(t, ts, "DELETE", "/v1/fish", "", ""); r.StatusCode != 200 {
		t.Fatalf("clear: %d", r.StatusCode)
	}
}

func TestCreatures(t *testing.T) {
	ts := newServer(t, "", DefaultLimits())
	if r := do(t, ts, "POST", "/v1/creatures/shark", "", ""); r.StatusCode != 202 {
		t.Fatalf("shark: %d", r.StatusCode)
	}
	if r := do(t, ts, "POST", "/v1/creatures/kraken", "", ""); r.StatusCode != 404 {
		t.Fatalf("kraken: %d", r.StatusCode)
	}
	if r := do(t, ts, "POST", "/v1/banner", `{"text":"hi","style":"ticker"}`, ""); r.StatusCode != 202 {
		t.Fatalf("banner: %d", r.StatusCode)
	}
	if r := do(t, ts, "POST", "/v1/banner", `{"text":"hi","style":"blink"}`, ""); r.StatusCode != 400 {
		t.Fatalf("bad style: %d", r.StatusCode)
	}
}

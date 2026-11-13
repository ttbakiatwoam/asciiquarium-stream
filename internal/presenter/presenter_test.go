package presenter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRestart(t *testing.T) {
	var calls []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
	}))
	defer ts.Close()
	c := &Client{BaseURL: ts.URL + "/control/api/v1/livestreams/0/"}
	if err := c.Restart(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	want := []string{"PUT /control/api/v1/livestreams/0/stop", "PUT /control/api/v1/livestreams/0/start"}
	if len(calls) != 2 || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls = %v", calls)
	}
}

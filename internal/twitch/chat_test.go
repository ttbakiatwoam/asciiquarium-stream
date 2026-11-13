package twitch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSendRefreshesExpiredToken(t *testing.T) {
	var sent []string
	refreshes := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("refresh_token") == "" || r.Form.Get("client_secret") != "sec" {
			http.Error(w, "bad", 400)
			return
		}
		refreshes++
		tok := "good"
		if refreshes == 1 {
			tok = "stale" // first token is rejected, forcing a second refresh
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": tok, "refresh_token": "rotated"})
	})
	mux.HandleFunc("POST /helix/chat/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" || r.Header.Get("Client-Id") != "cid" {
			w.WriteHeader(401)
			return
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["broadcaster_id"] != "123" || body["sender_id"] != "456" {
			http.Error(w, "ids", 400)
			return
		}
		sent = append(sent, body["message"])
		w.Write([]byte(`{"data":[{"message_id":"x","is_sent":true}]}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	tokFile := filepath.Join(t.TempDir(), "token")
	c := &Chat{
		ClientID: "cid", ClientSecret: "sec", BroadcasterID: "123", SenderID: "456",
		RefreshToken: "r0", TokenFile: tokFile,
		APIBase: ts.URL + "/helix", AuthBase: ts.URL + "/oauth2",
	}
	if err := c.Send(context.Background(), "Now playing: Galaxy Drift #7"); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0] != "Now playing: Galaxy Drift #7" {
		t.Fatalf("sent = %v", sent)
	}
	if b, _ := os.ReadFile(tokFile); string(b) != "rotated\n" {
		t.Fatalf("rotated refresh token not saved: %q", b)
	}
}

func TestSendReportsDrop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.Write([]byte(`{"access_token":"a"}`))
			return
		}
		w.Write([]byte(`{"data":[{"is_sent":false,"drop_reason":{"message":"slow mode"}}]}`))
	}))
	defer ts.Close()
	c := &Chat{ClientID: "c", ClientSecret: "s", RefreshToken: "r", APIBase: ts.URL + "/helix", AuthBase: ts.URL + "/oauth2"}
	if err := c.Send(context.Background(), "hi"); err == nil {
		t.Fatal("expected drop error")
	}
}

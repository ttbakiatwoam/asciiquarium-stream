// Package twitch posts messages to a Twitch channel's chat through the
// Helix "Send Chat Message" API, refreshing the OAuth token as needed.
//
// This lives in the tank for now so song changes can be announced in chat
// right away; it moves to the bridge service when that exists.
package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Chat sends chat messages as a user (usually a bot account) that has the
// user:write:chat scope.
type Chat struct {
	ClientID      string
	ClientSecret  string // needed to refresh the token
	BroadcasterID string // the channel's user id
	SenderID      string // the bot account's user id
	// RefreshToken renews AccessToken when it expires.
	RefreshToken string
	// TokenFile, if set, stores the latest refresh token so a restart
	// keeps working after Twitch rotates it.
	TokenFile string

	HTTP     *http.Client
	APIBase  string // default https://api.twitch.tv/helix
	AuthBase string // default https://id.twitch.tv/oauth2

	mu          sync.Mutex
	accessToken string
}

func (c *Chat) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// LoadToken reads a refresh token saved in TokenFile, if any.
func (c *Chat) LoadToken() {
	if c.TokenFile == "" {
		return
	}
	if b, err := os.ReadFile(c.TokenFile); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			c.RefreshToken = t
		}
	}
}

func (c *Chat) refresh(ctx context.Context) error {
	if c.RefreshToken == "" || c.ClientSecret == "" {
		return errors.New("twitch token expired and no refresh token/client secret configured")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.RefreshToken},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, or(c.AuthBase, "https://id.twitch.tv/oauth2")+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Message      string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusOK || out.AccessToken == "" {
		return fmt.Errorf("twitch token refresh failed: %s %s", resp.Status, out.Message)
	}
	c.accessToken = out.AccessToken
	if out.RefreshToken != "" && out.RefreshToken != c.RefreshToken {
		c.RefreshToken = out.RefreshToken
		if c.TokenFile != "" {
			os.WriteFile(c.TokenFile, []byte(out.RefreshToken+"\n"), 0o600)
		}
	}
	return nil
}

// Send posts message to the channel's chat.
func (c *Chat) Send(ctx context.Context, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken == "" {
		if err := c.refresh(ctx); err != nil {
			return err
		}
	}
	for attempt := 0; ; attempt++ {
		status, body, err := c.post(ctx, message)
		if err != nil {
			return err
		}
		if status == http.StatusUnauthorized && attempt == 0 {
			if err := c.refresh(ctx); err != nil {
				return err
			}
			continue
		}
		if status != http.StatusOK {
			return fmt.Errorf("twitch send chat message: HTTP %d: %s", status, strings.TrimSpace(string(body)))
		}
		var out struct {
			Data []struct {
				IsSent     bool `json:"is_sent"`
				DropReason *struct {
					Message string `json:"message"`
				} `json:"drop_reason"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &out) == nil && len(out.Data) > 0 && !out.Data[0].IsSent {
			reason := "unknown reason"
			if out.Data[0].DropReason != nil {
				reason = out.Data[0].DropReason.Message
			}
			return fmt.Errorf("twitch dropped the chat message: %s", reason)
		}
		return nil
	}
}

func (c *Chat) post(ctx context.Context, message string) (int, []byte, error) {
	payload, _ := json.Marshal(map[string]string{
		"broadcaster_id": c.BroadcasterID,
		"sender_id":      c.SenderID,
		"message":        message,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, or(c.APIBase, "https://api.twitch.tv/helix")+"/chat/messages", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Client-Id", c.ClientID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, body, nil
}

// Package presenter controls a Blackmagic Web Presenter's livestream over
// its REST API, e.g. http://172.17.3.29/control/api/v1/livestreams/0.
//
// The old StreamMusic cron script restarted the Twitch stream whenever it
// (re)started the music, so the encoder picks up the new audio cleanly. The
// tank does the same after it starts.
package presenter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to one Web Presenter livestream endpoint.
type Client struct {
	// BaseURL is the livestream URL; "/stop" and "/start" are appended.
	BaseURL string
	HTTP    *http.Client
}

func (c *Client) put(ctx context.Context, action string) error {
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	url := strings.TrimSuffix(c.BaseURL, "/") + "/" + action
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("PUT %s: %s", url, resp.Status)
	}
	return nil
}

// Stop stops the livestream.
func (c *Client) Stop(ctx context.Context) error { return c.put(ctx, "stop") }

// Start starts the livestream.
func (c *Client) Start(ctx context.Context) error { return c.put(ctx, "start") }

// Restart stops the livestream, waits, and starts it again. Start is still
// attempted if Stop fails (the stream may already be stopped).
func (c *Client) Restart(ctx context.Context, wait time.Duration) error {
	stopErr := c.Stop(ctx)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}
	if err := c.Start(ctx); err != nil {
		return err
	}
	return stopErr
}

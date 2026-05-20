// Package notify delivers notifications (webhooks, Slack) when sync or
// health events occur. It is intentionally simple — fire-and-forget with
// a short retry on transient errors.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/orin/orin/internal/domain"
)

// Dispatcher sends notifications to configured webhooks.
type Dispatcher struct {
	client *http.Client
}

// New creates a Dispatcher with sensible defaults.
func New() *Dispatcher {
	return &Dispatcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Payload is the JSON body sent to webhooks/Slack.
type Payload struct {
	AppName     string `json:"appName"`
	Event       string `json:"event"`
	Status      string `json:"status,omitempty"`
	Health      string `json:"health,omitempty"`
	Message     string `json:"message,omitempty"`
	InitiatedBy string `json:"initiatedBy,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// Send dispatches a notification to all matching configs.
func (d *Dispatcher) Send(ctx context.Context, cfg *domain.NotificationConfig, p Payload) {
	if !cfg.Enabled {
		return
	}

	body, err := json.Marshal(p)
	if err != nil {
		slog.Error("notify: marshal payload", "err", err)
		return
	}

	go func() {
		d.sendWithRetry(ctx, cfg, body)
	}()
}

func (d *Dispatcher) sendWithRetry(ctx context.Context, cfg *domain.NotificationConfig, body []byte) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 2 * time.Second
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		// Slack-compatible: add bot name
		req.Header.Set("X-K8sUI-Event", "true")

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			slog.Warn("notify: request failed", "config", cfg.Name, "url", cfg.URL, "err", err, "attempt", attempt+1)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Info("notify: sent", "config", cfg.Name, "type", cfg.Type, "status", resp.Status)
			return
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		slog.Warn("notify: non-2xx response", "config", cfg.Name, "url", cfg.URL, "status", resp.Status, "attempt", attempt+1)
	}
	slog.Error("notify: failed after retries", "config", cfg.Name, "err", lastErr)
}

// Test sends a test payload to verify a webhook URL is reachable.
func (d *Dispatcher) Test(ctx context.Context, url string) error {
	body, _ := json.Marshal(Payload{
		AppName:   "test",
		Event:     "test",
		Message:   "This is a test notification from orin",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("test failed: HTTP %d", resp.StatusCode)
}

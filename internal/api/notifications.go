package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type notificationTestRequest struct {
	URL string `json:"url"`
}

// postNotificationTest POSTs a small JSON payload to an external webhook URL
// (Slack-compatible incoming webhooks, generic automation, etc.).
func (s *Server) postNotificationTest(w http.ResponseWriter, r *http.Request) {
	var req notificationTestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "missing_url", "url is required")
		return
	}
	body, _ := json.Marshal(map[string]any{
		"source":    "k8s-ui",
		"event":     "notification.test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_url", err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "webhook_failed", err.Error())
		return
	}
	defer resp.Body.Close()
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	writeJSON(w, http.StatusOK, map[string]any{
		"statusCode": resp.StatusCode,
		"bodyPrefix": string(snippet),
	})
}

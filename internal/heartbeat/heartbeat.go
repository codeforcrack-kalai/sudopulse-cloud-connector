// Package heartbeat periodically reports connector status to the SudoPulse API.
package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/sudopulse/connector/internal/proxy"
)

const version = "1.0.0"

// heartbeatPayload is the JSON body sent in each heartbeat request.
type heartbeatPayload struct {
	Status         string `json:"status"`
	Version        string `json:"version"`
	Platform       string `json:"platform"`
	Hostname       string `json:"hostname"`
	ActiveSessions int64  `json:"activeSessions"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Start sends periodic heartbeat reports to the API. It blocks until ctx is
// cancelled. Errors are logged but never cause a crash.
func Start(ctx context.Context, apiURL, connectorID, sessionToken string, interval time.Duration) {
	hostname, _ := os.Hostname()
	platform := runtime.GOOS + "/" + runtime.GOARCH

	slog.Info("heartbeat started", "interval", interval.String(), "connectorId", connectorID)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send one immediately, then on every tick.
	send(ctx, apiURL, connectorID, sessionToken, hostname, platform)

	for {
		select {
		case <-ctx.Done():
			slog.Info("heartbeat stopped")
			return
		case <-ticker.C:
			send(ctx, apiURL, connectorID, sessionToken, hostname, platform)
		}
	}
}

func send(ctx context.Context, apiURL, connectorID, sessionToken, hostname, platform string) {
	payload := heartbeatPayload{
		Status:         "active",
		Version:        version,
		Platform:       platform,
		Hostname:       hostname,
		ActiveSessions: proxy.ActiveSessions.Load(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("heartbeat marshal failed", "error", err)
		return
	}

	url := fmt.Sprintf("%s/api/connectors/%s/heartbeat", apiURL, connectorID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		slog.Error("heartbeat request creation failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("heartbeat send failed", "error", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("heartbeat non-OK response", "status", resp.StatusCode)
		return
	}

	slog.Debug("heartbeat sent", "activeSessions", payload.ActiveSessions)
}

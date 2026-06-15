// Package auth handles connector registration and session token refresh
// against the SudoPulse API.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// APIResponse wraps the standard API response format.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

// RegisterData is the nested data from the registration endpoint.
type RegisterData struct {
	ConnectorID   string `json:"connectorId"`
	SessionToken  string `json:"sessionToken"`
	SessionExpiry string `json:"sessionExpiry"`
	GatewayWSURL  string `json:"gatewayWsUrl"`
}

// RefreshData is the nested data from the refresh endpoint.
type RefreshData struct {
	SessionToken  string `json:"sessionToken"`
	SessionExpiry string `json:"sessionExpiry"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Register exchanges a one-time install token for a persistent connector identity.
func Register(ctx context.Context, apiURL, installToken string) (*RegisterData, error) {
	body, err := json.Marshal(map[string]string{"token": installToken})
	if err != nil {
		return nil, fmt.Errorf("marshal register body: %w", err)
	}

	url := apiURL + "/api/connectors/auth"
	slog.Info("registering connector", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read register response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("register failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var apiRes APIResponse
	if err := json.Unmarshal(respBody, &apiRes); err != nil {
		return nil, fmt.Errorf("parse register API response: %w", err)
	}

	var result RegisterData
	if err := json.Unmarshal(apiRes.Data, &result); err != nil {
		return nil, fmt.Errorf("parse register data: %w", err)
	}

	slog.Info("registration successful", "connectorId", result.ConnectorID)
	return &result, nil
}

// Refresh exchanges the current session token for a new one before expiry.
func Refresh(ctx context.Context, apiURL, connectorID, sessionToken string) (*RefreshData, error) {
	body, err := json.Marshal(map[string]string{"connectorId": connectorID})
	if err != nil {
		return nil, fmt.Errorf("marshal refresh body: %w", err)
	}

	url := apiURL + "/api/connectors/refresh"
	slog.Debug("refreshing session token", "url", url, "connectorId", connectorID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var apiRes APIResponse
	if err := json.Unmarshal(respBody, &apiRes); err != nil {
		return nil, fmt.Errorf("parse refresh API response: %w", err)
	}

	var result RefreshData
	if err := json.Unmarshal(apiRes.Data, &result); err != nil {
		return nil, fmt.Errorf("parse refresh data: %w", err)
	}

	slog.Info("session token refreshed", "connectorId", connectorID)
	return &result, nil
}

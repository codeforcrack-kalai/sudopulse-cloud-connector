// Package config handles configuration loading, state persistence, and
// environment/flag merging for the SudoPulse connector agent.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateFileName = "state.json"

// Config holds the runtime configuration parsed from flags and environment variables.
type Config struct {
	InstallToken string
	GatewayURL   string
	APIURL       string
	LogLevel     string
	StateDir     string
}

// State holds the persistent registration state saved between restarts.
type State struct {
	ConnectorID  string `json:"connectorId"`
	SessionToken string `json:"sessionToken"`
	GatewayWSURL string `json:"gatewayWsUrl"`
}

// Validate checks that the minimum required configuration is present.
func (c *Config) Validate() error {
	if c.APIURL == "" {
		return errors.New("api-url is required (--api-url or SUDOPULSE_API_URL)")
	}
	if c.GatewayURL == "" {
		return errors.New("gateway-url is required (--gateway-url or SUDOPULSE_GATEWAY_URL)")
	}
	return nil
}

// LoadState reads the connector state from disk. If the file does not exist,
// it returns nil with no error.
func LoadState(dir string) (*State, error) {
	path := filepath.Join(dir, stateFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}

	return &state, nil
}

// SaveState writes the connector state to disk, creating the directory if needed.
func SaveState(dir string, state *State) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := filepath.Join(dir, stateFileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

// EnvOrDefault returns the environment variable value if set, otherwise the fallback.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

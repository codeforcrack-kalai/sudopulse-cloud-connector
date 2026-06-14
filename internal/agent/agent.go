// Package agent orchestrates the connector lifecycle: registration, tunnel
// connection, heartbeat, token refresh, and reconnection with backoff.
package agent

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/sudopulse/connector/internal/auth"
	"github.com/sudopulse/connector/internal/config"
	"github.com/sudopulse/connector/internal/heartbeat"
	"github.com/sudopulse/connector/internal/proxy"
	"github.com/sudopulse/connector/internal/tunnel"
)

const (
	heartbeatInterval  = 30 * time.Second
	refreshBeforeExpiry = 1 * time.Hour
	backoffMin         = 1 * time.Second
	backoffMax         = 60 * time.Second
)

// Run is the main loop of the connector agent. It handles registration,
// connection, and automatic reconnection with exponential backoff.
func Run(ctx context.Context, cfg *config.Config) error {
	state, err := bootstrap(ctx, cfg)
	if err != nil {
		return err
	}

	var attempt int

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		slog.Info("connecting to gateway",
			"gatewayUrl", state.GatewayWSURL,
			"connectorId", state.ConnectorID,
			"attempt", attempt+1,
		)

		session, err := tunnel.Connect(ctx, state.GatewayWSURL, state.ConnectorID, state.SessionToken)
		if err != nil {
			slog.Error("tunnel connection failed", "error", err)
			attempt++
			if sleepErr := backoff(ctx, attempt); sleepErr != nil {
				return sleepErr
			}
			continue
		}

		// Reset backoff on successful connect.
		attempt = 0
		slog.Info("tunnel established")

		// Derive a child context for this connection's goroutines.
		connCtx, connCancel := context.WithCancel(ctx)

		// Heartbeat goroutine.
		go heartbeat.Start(connCtx, cfg.APIURL, state.ConnectorID, state.SessionToken, heartbeatInterval)

		// Token refresh goroutine.
		go runRefresh(connCtx, cfg, state)

		// Block accepting streams until disconnected.
		proxy.HandleStreams(connCtx, session)

		// Connection lost — clean up.
		connCancel()
		session.Close()
		slog.Warn("tunnel disconnected, will reconnect")

		attempt++
		if sleepErr := backoff(ctx, attempt); sleepErr != nil {
			return sleepErr
		}
	}
}

// bootstrap loads or creates the connector state (register on first boot).
func bootstrap(ctx context.Context, cfg *config.Config) (*config.State, error) {
	state, err := config.LoadState(cfg.StateDir)
	if err != nil {
		return nil, err
	}

	if state != nil && state.ConnectorID != "" {
		slog.Info("loaded existing state",
			"connectorId", state.ConnectorID,
			"stateDir", cfg.StateDir,
		)
		return state, nil
	}

	// First boot — register with install token.
	if cfg.InstallToken == "" {
		slog.Error("no state found and no install token provided; cannot register")
		return nil, ErrNoToken
	}

	slog.Info("no existing state, registering with install token")

	resp, err := auth.Register(ctx, cfg.APIURL, cfg.InstallToken)
	if err != nil {
		return nil, err
	}

	gatewayURL := resp.GatewayWSURL
	if gatewayURL == "" {
		gatewayURL = cfg.GatewayURL
	}

	state = &config.State{
		ConnectorID:  resp.ConnectorID,
		SessionToken: resp.SessionToken,
		GatewayWSURL: gatewayURL,
	}

	if err := config.SaveState(cfg.StateDir, state); err != nil {
		return nil, err
	}

	slog.Info("state saved", "stateDir", cfg.StateDir)
	return state, nil
}

// runRefresh periodically refreshes the session token before it expires.
func runRefresh(ctx context.Context, cfg *config.Config, state *config.State) {
	// Refresh every hour (well before typical expiry windows).
	ticker := time.NewTicker(refreshBeforeExpiry)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Info("refreshing session token")
			resp, err := auth.Refresh(ctx, cfg.APIURL, state.ConnectorID, state.SessionToken)
			if err != nil {
				slog.Error("token refresh failed", "error", err)
				continue
			}
			state.SessionToken = resp.SessionToken
			if err := config.SaveState(cfg.StateDir, state); err != nil {
				slog.Error("failed to save refreshed state", "error", err)
			}
		}
	}
}

// backoff sleeps for an exponentially increasing duration with jitter.
// Returns ctx.Err() if the context is cancelled during the sleep.
func backoff(ctx context.Context, attempt int) error {
	exp := math.Pow(2, float64(attempt-1))
	wait := time.Duration(exp) * backoffMin
	if wait > backoffMax {
		wait = backoffMax
	}

	// Add up to 25% jitter.
	jitter := time.Duration(rand.Int63n(int64(wait) / 4))
	wait += jitter

	slog.Info("backing off before reconnect", "wait", wait.String(), "attempt", attempt)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

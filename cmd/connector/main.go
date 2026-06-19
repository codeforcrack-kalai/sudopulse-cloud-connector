// SudoPulse Cloud Connector Agent
//
// This binary creates a reverse WebSocket tunnel from a customer's private
// network to a SudoPulse Gateway, allowing the platform to reach internal
// services without requiring inbound firewall rules.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/sudopulse/connector/internal/agent"
	"github.com/sudopulse/connector/internal/config"
)

const version = "1.0.0"

func main() {
	// ── Flags ────────────────────────────────────────────────────────────
	token := flag.String("token", config.EnvOrDefault("SUDOPULSE_TOKEN", ""),
		"one-time install token for first boot (env: SUDOPULSE_TOKEN)")
	gatewayURL := flag.String("gateway-url", config.EnvOrDefault("SUDOPULSE_GATEWAY_URL", ""),
		"gateway WSS URL (env: SUDOPULSE_GATEWAY_URL)")
	apiURL := flag.String("api-url", config.EnvOrDefault("SUDOPULSE_API_URL", ""),
		"backend API URL (env: SUDOPULSE_API_URL)")
	logLevel := flag.String("log-level", config.EnvOrDefault("LOG_LEVEL", "info"),
		"log level: debug, info, warn, error (env: LOG_LEVEL)")
	stateDir := flag.String("state-dir", config.EnvOrDefault("SUDOPULSE_STATE_DIR", "/etc/sudopulse-connector/"),
		"directory for state.json (env: SUDOPULSE_STATE_DIR)")
	allowedSubnets := flag.String("allowed-subnets", config.EnvOrDefault("SUDOPULSE_ALLOWED_SUBNETS", ""),
		"comma-separated list of allowed CIDRs to proxy (e.g., '10.0.0.0/8,192.168.1.0/24'). Empty means allow all. (env: SUDOPULSE_ALLOWED_SUBNETS)")

	flag.Parse()

	// ── Logging ──────────────────────────────────────────────────────────
	setupLogging(*logLevel)

	// ── Banner ───────────────────────────────────────────────────────────
	slog.Info("sudopulse-connector starting",
		"version", version,
		"platform", runtime.GOOS+"/"+runtime.GOARCH,
		"goVersion", runtime.Version(),
		"pid", os.Getpid(),
	)

	// ── Config ───────────────────────────────────────────────────────────
	cfg := &config.Config{
		InstallToken: *token,
		GatewayURL:   *gatewayURL,
		APIURL:         *apiURL,
		LogLevel:       *logLevel,
		StateDir:       *stateDir,
		AllowedSubnets: *allowedSubnets,
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// ── Graceful shutdown ────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig.String())
		cancel()
	}()

	// ── Run ──────────────────────────────────────────────────────────────
	if err := agent.Run(ctx, cfg); err != nil {
		if ctx.Err() != nil {
			slog.Info("connector shut down gracefully")
			os.Exit(0)
		}
		slog.Error("connector exited with error", "error", err)
		os.Exit(1)
	}
}

// setupLogging configures the global slog logger with structured JSON output.
func setupLogging(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})
	slog.SetDefault(slog.New(handler))
	fmt.Fprintf(os.Stderr, "sudopulse-connector v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
}

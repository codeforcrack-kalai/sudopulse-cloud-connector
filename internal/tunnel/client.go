// Package tunnel manages the WebSocket connection to the SudoPulse Gateway
// and creates a yamux multiplexed session over it.
package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

// Connect dials the gateway WebSocket endpoint and establishes a yamux client
// session over the connection. The returned session allows the gateway to open
// multiplexed streams back to this connector.
func Connect(ctx context.Context, gatewayWSURL, connectorID, sessionToken, tlsCert string) (*yamux.Session, error) {
	url := gatewayWSURL
	slog.Info("connecting to gateway", "url", url, "connectorId", connectorID)

	headers := http.Header{
		"Authorization":  []string{"Bearer " + sessionToken},
		"X-Connector-ID": []string{connectorID},
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: websocket.DefaultDialer.HandshakeTimeout,
	}

	if tlsCert != "" {
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM([]byte(tlsCert)); !ok {
			return nil, fmt.Errorf("failed to parse gateway tls cert")
		}
		dialer.TLSClientConfig = &tls.Config{
			RootCAs: certPool,
		}
	}

	wsConn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket dial failed (status %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	slog.Info("websocket connected", "url", url)

	// Wrap the WebSocket connection so yamux can use it as a stream transport.
	wrapped := newWSConn(wsConn)

	// yamux.Client creates a client-side session. The gateway acts as the
	// server and opens streams towards us.
	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.LogOutput = io.Discard // suppress yamux internal logging; we use slog
	yamuxCfg.EnableKeepAlive = true
	yamuxCfg.KeepAliveInterval = 15 * time.Second
	yamuxCfg.ConnectionWriteTimeout = 10 * time.Second
	yamuxCfg.MaxStreamWindowSize = 1024 * 1024 // 1MB per stream

	session, err := yamux.Client(wrapped, yamuxCfg)
	if err != nil {
		wsConn.Close()
		return nil, fmt.Errorf("yamux client init: %w", err)
	}

	slog.Info("yamux session established")
	return session, nil
}

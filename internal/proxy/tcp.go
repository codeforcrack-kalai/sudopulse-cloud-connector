// Package proxy handles forwarding of yamux streams to local TCP targets
// within the customer's private network.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/hashicorp/yamux"
)

// streamRequest is the initial JSON message sent by the gateway on each new
// yamux stream, indicating which host:port to forward traffic to.
type streamRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// ActiveSessions tracks the number of currently active proxy sessions.
var ActiveSessions atomic.Int64

// HandleStreams accepts new yamux streams from the gateway and proxies each
// one to the requested TCP target. It blocks until ctx is cancelled or the
// yamux session is closed.
func HandleStreams(ctx context.Context, session *yamux.Session, allowedSubnetsStr string) {
	var allowedSubnets []*net.IPNet
	if allowedSubnetsStr != "" {
		parts := strings.Split(allowedSubnetsStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(p)
			if err != nil {
				slog.Error("failed to parse allowed subnet, ignoring", "subnet", p, "error", err)
				continue
			}
			allowedSubnets = append(allowedSubnets, ipNet)
		}
		slog.Info("parsed allowed subnets", "count", len(allowedSubnets))
	}

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("stream handler shutting down")
				return
			}
			slog.Error("accept stream failed", "error", err)
			return
		}

		go handleStream(ctx, stream, allowedSubnets)
	}
}

func handleStream(ctx context.Context, stream *yamux.Stream, allowedSubnets []*net.IPNet) {
	streamID := stream.StreamID()
	started := time.Now()

	ActiveSessions.Add(1)
	defer ActiveSessions.Add(-1)

	slog.Info("stream accepted", "streamId", streamID)

	// Read the target request (first message on the stream).
	decoder := json.NewDecoder(stream)
	var req streamRequest
	if err := decoder.Decode(&req); err != nil {
		slog.Error("failed to read stream request", "streamId", streamID, "error", err)
		stream.Close()
		return
	}

	target := fmt.Sprintf("%s:%d", req.Host, req.Port)

	// Enforce ACL if subnets are configured
	if len(allowedSubnets) > 0 {
		targetIP := net.ParseIP(req.Host)
		if targetIP == nil {
			ips, err := net.LookupIP(req.Host)
			if err != nil || len(ips) == 0 {
				slog.Error("failed to resolve request host, rejected by ACL", "host", req.Host, "error", err)
				stream.Close()
				return
			}
			targetIP = ips[0]
		}

		allowed := false
		for _, net := range allowedSubnets {
			if net.Contains(targetIP) {
				allowed = true
				break
			}
		}

		if !allowed {
			slog.Error("request host is not in allowed subnets, rejected", "host", req.Host)
			stream.Close()
			return
		}
	}

	slog.Info("proxying stream", "streamId", streamID, "target", target)

	// Dial the target within the private network.
	dialer := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		slog.Error("failed to dial target", "streamId", streamID, "target", target, "error", err)
		stream.Close()
		return
	}

	// Bidirectional copy between the yamux stream and the TCP connection.
	var wg sync.WaitGroup
	var txBytes, rxBytes int64

	wg.Add(2)

	// stream → TCP target
	go func() {
		defer wg.Done()
		
		// Flush and trim any buffered data from the JSON decoder (removes trailing newline)
		buffered, _ := io.ReadAll(decoder.Buffered())
		trimmed := bytes.TrimLeftFunc(buffered, unicode.IsSpace)
		
		var n int64
		if len(trimmed) > 0 {
			nw, _ := conn.Write(trimmed)
			n = int64(nw)
		}
		
		rx, _ := io.Copy(conn, stream)
		rxBytes = n + rx
		// Signal the TCP side we're done writing.
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	// TCP target → stream
	go func() {
		defer wg.Done()
		tx, _ := io.Copy(stream, conn)
		txBytes = tx
		stream.Close()
	}()

	wg.Wait()
	conn.Close()

	duration := time.Since(started)
	slog.Info("stream closed",
		"streamId", streamID,
		"target", target,
		"txBytes", txBytes,
		"rxBytes", rxBytes,
		"duration", duration.String(),
	)
}

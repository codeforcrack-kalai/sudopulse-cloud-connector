# SudoPulse Cloud Connector Agent

A reverse-tunnel agent that establishes a persistent WebSocket connection from a customer's private network to the SudoPulse Gateway. The gateway can then open multiplexed streams (via [yamux](https://github.com/hashicorp/yamux)) back through the tunnel to reach internal services—no inbound firewall rules required.

## Architecture

```
┌──────────────────────────────────────────────┐
│  Customer Network                            │
│                                              │
│  ┌──────────────────┐    ┌────────────────┐  │
│  │ sudopulse-       │───▶│ Internal Host  │  │
│  │ connector        │    │ 10.0.0.5:22    │  │
│  │                  │    └────────────────┘  │
│  │  WSS ▲           │    ┌────────────────┐  │
│  │      │           │───▶│ Internal Host  │  │
│  └──────┼───────────┘    │ 10.0.0.10:443  │  │
│         │                └────────────────┘  │
│─────────┼────────────────────────────────────│
          │ (outbound only)
          ▼
┌──────────────────┐     ┌──────────────────┐
│ SudoPulse        │────▶│ SudoPulse        │
│ Gateway          │     │ API              │
│ (WSS endpoint)   │     │ (REST)           │
└──────────────────┘     └──────────────────┘
```

## Quick Start

### First Boot (Registration)

```bash
sudopulse-connector \
  --token "install-token-from-dashboard" \
  --gateway-url "wss://gateway.sudopulse.com" \
  --api-url "https://api.sudopulse.com"
```

On first boot the agent exchanges the one-time install token for a persistent identity (`connectorId` + `sessionToken`) which is saved to `state.json`. Subsequent restarts use the saved state automatically.

### Subsequent Starts

```bash
sudopulse-connector \
  --gateway-url "wss://gateway.sudopulse.com" \
  --api-url "https://api.sudopulse.com"
```

### Using Environment Variables

```bash
export SUDOPULSE_TOKEN="install-token-from-dashboard"
export SUDOPULSE_GATEWAY_URL="wss://gateway.sudopulse.com"
export SUDOPULSE_API_URL="https://api.sudopulse.com"
export SUDOPULSE_STATE_DIR="/var/lib/sudopulse-connector"
export LOG_LEVEL="debug"

sudopulse-connector
```

### Docker

```bash
docker build -t sudopulse-connector .

docker run -d \
  --name sudopulse-connector \
  --restart unless-stopped \
  -e SUDOPULSE_TOKEN="install-token-from-dashboard" \
  -e SUDOPULSE_GATEWAY_URL="wss://gateway.sudopulse.com" \
  -e SUDOPULSE_API_URL="https://api.sudopulse.com" \
  -v /var/lib/sudopulse-connector:/etc/sudopulse-connector \
  sudopulse-connector
```

## CLI Flags

| Flag             | Env Var                  | Default                      | Description                          |
|------------------|--------------------------|------------------------------|--------------------------------------|
| `--token`        | `SUDOPULSE_TOKEN`        | —                            | One-time install token (first boot)  |
| `--gateway-url`  | `SUDOPULSE_GATEWAY_URL`  | —                            | Gateway WSS URL                      |
| `--api-url`      | `SUDOPULSE_API_URL`      | —                            | Backend API URL                      |
| `--log-level`    | `LOG_LEVEL`              | `info`                       | Log level: debug/info/warn/error     |
| `--state-dir`    | `SUDOPULSE_STATE_DIR`    | `/etc/sudopulse-connector/`  | Directory for persistent state       |

## Building

```bash
# Current platform
make build

# All platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
make build-all

# Clean
make clean
```

Binaries are placed in the `build/` directory.

## How It Works

1. **Registration** — On first boot, the agent POSTs the install token to the API and receives a `connectorId` + `sessionToken`. These are saved to `state.json`.

2. **Tunnel** — The agent dials a WebSocket connection to the gateway and layers a [yamux](https://github.com/hashicorp/yamux) multiplexed session on top.

3. **Stream Proxying** — The gateway opens yamux streams when it needs to reach an internal service. Each stream's first message specifies the target (`host:port`). The agent dials that target and bidirectionally copies data.

4. **Heartbeat** — Every 30 seconds the agent reports its status (version, platform, active sessions) to the API.

5. **Token Refresh** — The session token is refreshed 1 hour before expiry to maintain a continuous session.

6. **Reconnection** — On disconnect, the agent reconnects with exponential backoff (1s → 2s → 4s → … → 60s max) plus jitter.

## Project Structure

```
sudopulse-connector/
├── cmd/connector/main.go         # CLI entry point
├── internal/
│   ├── agent/agent.go            # Main run loop & orchestration
│   ├── auth/auth.go              # Registration & token refresh
│   ├── config/config.go          # Configuration & state persistence
│   ├── heartbeat/heartbeat.go    # Periodic status reporting
│   ├── proxy/tcp.go              # TCP stream proxying
│   └── tunnel/
│       ├── client.go             # WebSocket + yamux connection
│       └── wsconn.go             # WebSocket → io.ReadWriteCloser adapter
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## License

Proprietary — © 2026 SudoPulse, Inc. All rights reserved.

#!/usr/bin/env bash
set -e

# SudoPulse Cloud Connector Agent Installation Script

usage() {
    echo "Usage: $0 --token <YOUR_TOKEN>"
    exit 1
}

TOKEN=""

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --token) TOKEN="$2"; shift ;;
        *) echo "Unknown parameter passed: $1"; usage ;;
    esac
    shift
done

if [ -z "$TOKEN" ]; then
    echo "Error: --token is required."
    usage
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH="arm64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
    echo "Unsupported operating system: $OS"
    exit 1
fi

BINARY_URL="https://github.com/sudopulse/connector/releases/latest/download/sudopulse-connector-${OS}-${ARCH}"
BIN_PATH="/usr/local/bin/sudopulse-connector"

echo "Downloading SudoPulse Connector for ${OS}-${ARCH}..."
curl -Lo /tmp/sudopulse-connector "${BINARY_URL}" || { echo "Failed to download binary."; exit 1; }

echo "Installing binary to ${BIN_PATH}..."
sudo mv /tmp/sudopulse-connector "${BIN_PATH}"
sudo chmod +x "${BIN_PATH}"

# Systemd setup for Linux
if [ "$OS" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
    echo "Configuring systemd service..."

    # Create user if it doesn't exist
    if ! id "sudopulse" >/dev/null 2>&1; then
        sudo useradd --system -M -s /bin/false sudopulse
    fi

    sudo mkdir -p /etc/sudopulse-connector
    sudo chown sudopulse:sudopulse /etc/sudopulse-connector
    sudo chmod 0700 /etc/sudopulse-connector

    cat <<EOF | sudo tee /etc/systemd/system/sudopulse-connector.service >/dev/null
[Unit]
Description=SudoPulse Cloud Connector Agent
After=network.target

[Service]
Type=simple
User=sudopulse
Environment="SUDOPULSE_TOKEN=${TOKEN}"
ExecStart=${BIN_PATH}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable sudopulse-connector.service
    sudo systemctl start sudopulse-connector.service
    echo "SudoPulse Connector installed and started via systemd."
else
    echo "SudoPulse Connector installed successfully."
    echo "To run it manually: SUDOPULSE_TOKEN=${TOKEN} ${BIN_PATH}"
fi

echo "Installation complete!"

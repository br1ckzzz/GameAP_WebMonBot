#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$SCRIPT_DIR/bin"
mkdir -p "$BIN_DIR"

echo "=========================================="
echo " Building GameAP WebMonBot (Go)"
echo "=========================================="

echo "-> Compiling Linux binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.AppVersion=1.0.1" -o "$BIN_DIR/webmonbot-linux-amd64" ./cmd/bot

echo "-> Compiling Windows binary..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.AppVersion=1.0.1" -o "$BIN_DIR/webmonbot.exe" ./cmd/bot

echo ""
echo "Build complete! Binaries:"
ls -lh "$BIN_DIR"

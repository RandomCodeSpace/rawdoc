#!/bin/bash
# Post-install hook: build rawdoc binary from Go source.
# Runs when the plugin is installed or updated.

set -e

PLUGIN_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="$PLUGIN_DIR/rawdoc"

# Skip if binary already exists
if [ -f "$BINARY" ]; then
    echo "[rawdoc] Binary exists: $BINARY"
    exit 0
fi

# Try building from source
if command -v go &>/dev/null; then
    echo "[rawdoc] Building from source..."
    cd "$PLUGIN_DIR"
    go build -ldflags="-s -w" -o "$BINARY" .
    echo "[rawdoc] Built: $BINARY"
    exit 0
fi

# Try go install
if command -v go &>/dev/null; then
    echo "[rawdoc] Installing via go install..."
    go install github.com/RandomCodeSpace/rawdoc@latest
    exit 0
fi

echo "[rawdoc] Error: Go is not installed."
echo "[rawdoc] Install Go from https://go.dev/dl/ then re-install this plugin."
exit 1

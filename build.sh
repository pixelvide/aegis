#!/bin/bash
# Build script for Aegis — builds the React UI and Go server into a single binary.
#
# Usage:
#   ./build.sh           # Build both UI and server
#   ./build.sh --server  # Build server only (assumes UI already built)
#
# Output: server/cmd/aegis-server/aegis-server

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
UI_DIR="$SCRIPT_DIR/ui"
SERVER_DIR="$SCRIPT_DIR/server"
EMBED_DIR="$SERVER_DIR/cmd/aegis-server/ui"

echo "🛡️  Building Aegis..."

# Step 1: Build React UI
if [[ "${1:-}" != "--server" ]]; then
    echo "📦 Building UI..."
    cd "$UI_DIR"
    npm run build
    echo "✓ UI built"
fi

# Step 2: Copy dist to embed directory
echo "📋 Copying UI dist to server embed..."
rm -rf "$EMBED_DIR"
cp -r "$UI_DIR/dist" "$EMBED_DIR"
echo "✓ UI copied to $EMBED_DIR"

# Step 3: Build Go binary
echo "🔨 Building Go server..."
cd "$SERVER_DIR"
go build -o "$SERVER_DIR/cmd/aegis-server/aegis-server" ./cmd/aegis-server/
echo "✓ Server built"

echo ""
echo "🛡️  Aegis built successfully!"
echo "   Binary: $SERVER_DIR/cmd/aegis-server/aegis-server"
echo "   Run:    $SERVER_DIR/cmd/aegis-server/aegis-server"

#!/usr/bin/env bash
# Install cc-cache by symlinking cc_cache.py into ~/.local/bin/
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$HOME/.local/bin/cc-cache"

mkdir -p "$HOME/.local/bin"
ln -sf "$SCRIPT_DIR/cc_cache.py" "$TARGET"
chmod +x "$SCRIPT_DIR/cc_cache.py"

echo "✓ Installed: cc-cache → $SCRIPT_DIR/cc_cache.py"
echo "  Make sure ~/.local/bin is on your PATH."

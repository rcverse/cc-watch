#!/usr/bin/env bash
# Install cc-watch by symlinking cc_watch.py into ~/.local/bin/
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$HOME/.local/bin/cc-watch"

mkdir -p "$HOME/.local/bin"
ln -sf "$SCRIPT_DIR/cc_watch.py" "$TARGET"
chmod +x "$SCRIPT_DIR/cc_watch.py"

echo "✓ Installed: cc-watch → $SCRIPT_DIR/cc_watch.py"
echo "  Make sure ~/.local/bin is on your PATH."

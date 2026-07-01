#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: ./install.sh [--yes] [--dry-run]

Build and install the cc-watch v2 Go binary to:
  $HOME/.local/bin/cc-watch

Options:
  --yes      required to write the installed command
  --dry-run  show what would happen without writing
  --help     show this help

Environment:
  CC_WATCH_BUILD_DIR  override build output directory (default: ./dist)
USAGE
}

YES=0
DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --yes)
      YES=1
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      usage >&2
      exit 2
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${CC_WATCH_BUILD_DIR:-$SCRIPT_DIR/dist}"
case "$BUILD_DIR" in
  /*) ;;
  *) BUILD_DIR="$SCRIPT_DIR/$BUILD_DIR" ;;
esac
while [ "$BUILD_DIR" != "/" ] && [ "${BUILD_DIR%/}" != "$BUILD_DIR" ]; do
  BUILD_DIR="${BUILD_DIR%/}"
done
BINARY="$BUILD_DIR/cc-watch"
BIN_DIR="$HOME/.local/bin"
TARGET="$BIN_DIR/cc-watch"

if [ "$BINARY" = "$TARGET" ]; then
  echo "build output must not equal install target: $TARGET" >&2
  echo "Choose a different CC_WATCH_BUILD_DIR or unset it." >&2
  exit 2
fi

echo "cc-watch v2 local installer"
echo "repo:    $SCRIPT_DIR"
echo "binary:  $BINARY"
echo "target:  $TARGET"

if [ -e "$TARGET" ] || [ -L "$TARGET" ]; then
  if [ -L "$TARGET" ]; then
    CURRENT_TARGET="$(readlink "$TARGET")"
    echo "current: symlink -> $CURRENT_TARGET"
  else
    echo "current: existing file"
  fi
else
  echo "current: not installed"
fi

if [ "$DRY_RUN" -eq 1 ]; then
  echo "dry run: would build and install cc-watch v2"
  exit 0
fi

if [ "$YES" -ne 1 ]; then
  echo "refusing to install without --yes" >&2
  echo "Run './install.sh --yes' only after you are ready to replace the local command path." >&2
  exit 2
fi

mkdir -p "$BUILD_DIR" "$BIN_DIR"

GOCACHE="${GOCACHE:-/private/tmp/cc-watch-go-build}" \
GOMODCACHE="${GOMODCACHE:-/private/tmp/cc-watch-go-mod}" \
  go build -C "$SCRIPT_DIR" -o "$BINARY" ./cmd/cc-watch

"$BINARY" --version >/dev/null
"$BINARY" --help >/dev/null

TMP_TARGET="$TARGET.tmp.$$"
cp "$BINARY" "$TMP_TARGET"
chmod 0755 "$TMP_TARGET"
if [ -L "$TARGET" ]; then
  rm "$TARGET"
fi
mv -f "$TMP_TARGET" "$TARGET"

echo "installed: $TARGET"
"$TARGET" --version

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d /private/tmp/cc-watch-install-test.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

TEST_HOME="$TMP_DIR/home"
BUILD_DIR="$TMP_DIR/build"
mkdir -p "$TEST_HOME" "$BUILD_DIR"
mkdir -p "$TEST_HOME/.local/bin"
ln -s "$ROOT/cc_watch.py" "$TEST_HOME/.local/bin/cc-watch"

HOME="$TEST_HOME" CC_WATCH_BUILD_DIR="$BUILD_DIR" bash "$ROOT/install.sh" --yes

TARGET="$TEST_HOME/.local/bin/cc-watch"
if [ ! -x "$TARGET" ]; then
  echo "installed target is not executable: $TARGET" >&2
  exit 1
fi

if [ -L "$TARGET" ]; then
  echo "installed target must be a copied Go binary, not a symlink" >&2
  exit 1
fi

VERSION="$("$TARGET" --version)"
case "$VERSION" in
  "cc-watch 2.0.0-dev") ;;
  *)
    echo "unexpected installed version: $VERSION" >&2
    exit 1
    ;;
esac

JSON_OUTPUT="$(HOME="$ROOT/internal/session/testdata/smoke-home" "$TARGET" --json)"
case "$JSON_OUTPUT" in
  *'"schema_version": 1'*'"sessions":'*'"error": null'*) ;;
  *)
    echo "installed binary JSON smoke failed" >&2
    echo "$JSON_OUTPUT" >&2
    exit 1
    ;;
esac

V1_HELP="$("$ROOT/cc_watch.py" --help)"
case "$V1_HELP" in
  *"Inspect Claude Code prompt cache TTL"*) ;;
  *)
    echo "root v1 cc_watch.py no longer behaves like the v1 script" >&2
    echo "$V1_HELP" >&2
    exit 1
    ;;
esac

if [ ! -x "$ROOT/cc_watch.py" ]; then
  echo "root v1 cc_watch.py is no longer executable" >&2
  exit 1
fi

"$ROOT/archive/v1/cc_watch.py" --help >/dev/null

assert_rejects_target_build_dir() {
  local name="$1"
  local build_dir_suffix="$2"
  local risk_home="$TMP_DIR/$name-home"
  local risk_build_dir="$risk_home/.local/bin"
  local sentinel="$TMP_DIR/$name-sentinel-cc-watch"
  mkdir -p "$risk_build_dir"
  printf 'legacy sentinel\n' >"$sentinel"
  ln -s "$sentinel" "$risk_build_dir/cc-watch"

  set +e
  local risk_output
  risk_output="$(HOME="$risk_home" CC_WATCH_BUILD_DIR="$risk_build_dir$build_dir_suffix" bash "$ROOT/install.sh" --yes 2>&1)"
  local risk_code="$?"
  set -e

  if [ "$risk_code" -ne 2 ]; then
    echo "dangerous build-dir-equals-target install exited $risk_code, want 2" >&2
    echo "$risk_output" >&2
    exit 1
  fi

  if ! printf '%s\n' "$risk_output" | grep -q 'build output must not equal install target'; then
    echo "dangerous install did not explain target/build collision" >&2
    echo "$risk_output" >&2
    exit 1
  fi

  if [ "$(cat "$sentinel")" != "legacy sentinel" ]; then
    echo "dangerous install modified symlink target sentinel" >&2
    exit 1
  fi
}

assert_rejects_target_build_dir "risk" ""
assert_rejects_target_build_dir "risk-trailing-slash" "/"

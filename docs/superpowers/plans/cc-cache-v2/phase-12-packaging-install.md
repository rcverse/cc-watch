# cc-cache v2 Phase 12: Local macOS Install Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not use implementation subagents for this phase; the install path touches command-path safety.

**Goal:** Provide a verified simple local macOS install path for the Go binary without touching the real `$HOME/.local/bin/cc-cache` during implementation.

**Architecture:** `install.sh` becomes the local installer for the verified Go binary. It builds the binary, verifies it, and installs a copied executable into `$HOME/.local/bin/cc-cache` only when invoked with explicit approval (`--yes`). Tests exercise the installer with a temporary `HOME`, preserving the current live command path and v1 rollback files.

**Tech Stack:** Bash, Go build/test, macOS local filesystem conventions, existing Go CLI binary.

---

## Scope

In scope:

- Build a local Go binary artifact at `dist/cc-cache`.
- Ignore local build artifacts without hiding source fixtures.
- Replace the legacy v1 symlink behavior in `install.sh` with a safe v2 local installer.
- Verify install behavior only against a temporary `HOME`.
- Keep root `cc_cache.py` and `archive/v1/*` intact for rollback/reference.
- Update docs/progress to say public release packaging remains deferred.

Out of scope:

- Running `install.sh` against the real user `$HOME`.
- Replacing the live `$HOME/.local/bin/cc-cache` command path.
- Homebrew, goreleaser, GitHub Releases, Linux/Windows packages, or release automation.
- A public `--watch` command.
- Any real Claude KeepAlive send.

## Files

- Modify: `.gitignore`
- Modify: `install.sh`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/cc-cache-v2/phase-12-packaging-install.md`
- Modify: `docs/superpowers/progress/cc-cache-v2-progress.md`
- Create: `scripts/test-install.sh`

## Gate A: Baseline And Build Artifact Hygiene

### Task A1: Confirm clean baseline and add artifact ignores

- [x] **Step 1: Confirm branch and clean worktree**

Run:

```bash
git status --short --branch
```

Expected: branch is `codex/phase-11.8-architecture-refactor` and there are no uncommitted changes except this plan if execution has already started.

- [x] **Step 2: Add local build artifact ignores**

Modify `.gitignore` so it contains:

```gitignore
dist/
coverage.out
*.test
```

Keep existing Python/cache ignores. Do not add patterns that hide `internal/session/testdata/**/*.jsonl`.

- [x] **Step 3: Verify fixtures remain visible to git**

Run:

```bash
git check-ignore internal/session/testdata/smoke-home/.claude/projects/-tmp-cc-cache/11111111-1111-1111-1111-111111111111.jsonl
```

Expected: exit code 1 and no output, meaning the fixture is not ignored.

- [x] **Step 4: Build local artifact**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o dist/cc-cache ./cmd/cc-cache
```

Expected: exits 0 and creates `dist/cc-cache`. The file is ignored by git.

- [x] **Step 5: Smoke the local artifact**

Run:

```bash
dist/cc-cache --version
dist/cc-cache --help
HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-cache --json
```

Expected: version/help exit 0; JSON exits 0, emits `schema_version: 1`, includes `sessions`, and has `"error": null`.

## Gate B: Test Installer With Temporary HOME

### Task B1: Add failing installer smoke test

- [x] **Step 1: Create shell test harness**

Create `scripts/test-install.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d /private/tmp/cc-cache-install-test.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

TEST_HOME="$TMP_DIR/home"
BUILD_DIR="$TMP_DIR/build"
mkdir -p "$TEST_HOME" "$BUILD_DIR"

HOME="$TEST_HOME" CC_CACHE_BUILD_DIR="$BUILD_DIR" "$ROOT/install.sh" --yes

TARGET="$TEST_HOME/.local/bin/cc-cache"
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
  "cc-cache 2.0.0-dev") ;;
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

if [ ! -x "$ROOT/cc_cache.py" ]; then
  echo "root v1 cc_cache.py is no longer executable" >&2
  exit 1
fi

"$ROOT/archive/v1/cc_cache.py" --help >/dev/null
```

- [x] **Step 2: Make test harness executable**

Run:

```bash
chmod +x scripts/test-install.sh
```

- [x] **Step 3: Verify red test**

Run:

```bash
scripts/test-install.sh
```

Expected before implementation: fail because current `install.sh` installs a symlink to `cc_cache.py`, which does not support `--version`.

## Gate C: Implement Safe Local Installer

### Task C1: Replace legacy installer behavior

- [x] **Step 1: Update `install.sh`**

Replace `install.sh` with:

```bash
#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: ./install.sh [--yes] [--dry-run]

Build and install the cc-cache v2 Go binary to:
  $HOME/.local/bin/cc-cache

Options:
  --yes      required to write the installed command
  --dry-run  show what would happen without writing
  --help     show this help

Environment:
  CC_CACHE_BUILD_DIR  override build output directory (default: ./dist)
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
BUILD_DIR="${CC_CACHE_BUILD_DIR:-$SCRIPT_DIR/dist}"
case "$BUILD_DIR" in
  /*) ;;
  *) BUILD_DIR="$SCRIPT_DIR/$BUILD_DIR" ;;
esac
BINARY="$BUILD_DIR/cc-cache"
BIN_DIR="$HOME/.local/bin"
TARGET="$BIN_DIR/cc-cache"

echo "cc-cache v2 local installer"
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
  echo "dry run: would build and install cc-cache v2"
  exit 0
fi

if [ "$YES" -ne 1 ]; then
  echo "refusing to install without --yes" >&2
  echo "Run './install.sh --yes' only after you are ready to replace the local command path." >&2
  exit 2
fi

mkdir -p "$BUILD_DIR" "$BIN_DIR"

GOCACHE="${GOCACHE:-/private/tmp/cc-cache-go-build}" \
GOMODCACHE="${GOMODCACHE:-/private/tmp/cc-cache-go-mod}" \
  go build -C "$SCRIPT_DIR" -o "$BINARY" ./cmd/cc-cache

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
```

- [x] **Step 2: Verify installer test passes**

Run:

```bash
scripts/test-install.sh
```

Expected: pass. This uses a temporary `HOME` and does not touch the real `$HOME/.local/bin/cc-cache`.

- [x] **Step 3: Verify dry run does not write**

Run:

```bash
TMP_HOME="$(mktemp -d /private/tmp/cc-cache-install-dry.XXXXXX)"
HOME="$TMP_HOME" ./install.sh --dry-run
test ! -e "$TMP_HOME/.local/bin/cc-cache"
rm -rf "$TMP_HOME"
```

Expected: exits 0 and writes no target.

- [x] **Step 4: Verify no-approval path refuses to install**

Run:

```bash
TMP_HOME="$(mktemp -d /private/tmp/cc-cache-install-refuse.XXXXXX)"
set +e
OUTPUT="$(HOME="$TMP_HOME" ./install.sh 2>&1)"
CODE="$?"
set -e
printf '%s\n' "$OUTPUT"
test "$CODE" -eq 2
printf '%s\n' "$OUTPUT" | rg 'refusing to install without --yes'
```

Expected: verifies exit 2 with `refusing to install without --yes`. Then remove the temp directory:

```bash
rm -rf "$TMP_HOME"
```

## Gate D: Documentation And Progress

### Task D1: Update docs for local install scope

- [x] **Step 1: Update README install section**

Modify `README.md` so the install section says:

````markdown
## Local Install

Phase 12 adds a simple local macOS install script:

```bash
./install.sh --dry-run
./install.sh --yes
```

The script builds the Go binary and installs a copied executable to `$HOME/.local/bin/cc-cache`.
It does not publish releases, install Homebrew formulae, or remove the v1 archive.
Run `./install.sh --yes` only when you are ready to replace the local command path.
````

- [x] **Step 2: Update progress current state**

Modify `docs/superpowers/progress/cc-cache-v2-progress.md` current state to:

```markdown
- Current phase: Phase 12 - Local macOS Install
- Current phase file: `docs/superpowers/plans/cc-cache-v2/phase-12-packaging-install.md`
- Current step: Local install implementation in progress
- Status: in progress
- Last updated: 2026-06-29
```

- [x] **Step 3: Keep public release deferred**

Verify these commands still find deferred/out-of-scope wording:

```bash
rg -n 'Homebrew|goreleaser|GitHub Releases|public release|deferred|out of scope' README.md docs/superpowers/specs/2026-06-18-cc-cache-v2-product-reality.md docs/superpowers/plans/cc-cache-v2/PLAN.md
```

Expected: matches show public release work is deferred or out of scope unless re-approved.

## Gate E: Final Verification And Review

### Task E1: Run final verification

- [x] **Step 1: Focused installer verification**

Run:

```bash
scripts/test-install.sh
```

Expected: pass and only use a temporary `HOME`.

- [x] **Step 2: Full suite**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test -count=1 ./...
```

Expected: pass.

- [x] **Step 3: Build and smoke local artifact**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o dist/cc-cache ./cmd/cc-cache
dist/cc-cache --version
HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-cache --json
```

Expected: pass. JSON emits `schema_version: 1`, includes `sessions`, and has `"error": null`.

- [x] **Step 4: Verify real command path was not changed**

Run:

```bash
command -v cc-cache || true
ls -l "$HOME/.local/bin/cc-cache" || true
```

Expected: inspect only. Do not run `./install.sh --yes` against the real `HOME` during implementation.

- [x] **Step 5: Whitespace and status**

Run:

```bash
git diff --check
git status --short --branch
```

Expected: whitespace clean; status shows only intended Phase 12 source/doc/test changes and ignored `dist/`.

- [x] **Step 6: Read-only review**

Dispatch one read-only reviewer focused on:

- no real `$HOME/.local/bin/cc-cache` replacement during implementation;
- installer does not overwrite `cc_cache.py` through an existing symlink;
- v1 archive/root script remain intact;
- public release/Homebrew/goreleaser work remains deferred;
- tests verify install behavior through temporary `HOME`.

Expected: reviewer PASS or actionable findings integrated with relevant verification rerun.

- [x] **Step 7: Update progress ledger**

Add a Phase 12 ledger row with exact commands run and results. Mark Phase 12 complete only after final verification and review pass.

## Notes

- The actual user-facing install command is `./install.sh --yes`, but this phase must not run that command with the real user `HOME`.
- If the user explicitly approves replacing `$HOME/.local/bin/cc-cache`, perform it as a separate operator action after this phase, with before/after `command -v`, `ls -l`, `cc-cache --version`, and JSON smoke checks.

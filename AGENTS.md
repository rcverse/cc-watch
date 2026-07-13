# AGENTS.md

cc-watch is a local, macOS-only Go TUI that watches Claude Code's
session cache (`~/.claude/projects/**/*.jsonl`) so a user knows when a cache
window is active, fading, or expired. It also runs an optional Reminder
alarm and a bounded KeepAlive workflow. See `README.md` for the user-facing
pitch.

## Layout

- `cmd/cc-watch/` — entry point.
- `internal/session/` — JSONL discovery and parsing (cache tier, gaps,
  token stats).
- `internal/snapshot/` — assembles one point-in-time view for TUI startup and
  refresh.
- `internal/refresh/` — fsnotify watcher, debounce, safety refresh.
- `internal/keepalive/` — the KeepAlive state machine and subprocess
  runner.
- `internal/ratelimit/` — account-wide 5-hour rate-limit tracking
  (momentum estimate, tier-TTL cache) for the `statusline` subcommand.
- `internal/notify/` — macOS `osascript` notifications.
- `internal/config/` — config file load/save/validate.
- `internal/tui/` — the Bubbletea model (List / Workspace / Config routes).
- `internal/app/` — CLI arg parsing and mode dispatch.
- `tools/ui-demo/` — build-tagged rare-state TUI fixture harness. It must
  use fake sessions, fake clocks, fake notifications, and fake KeepAlive
  runners; run only with `-tags demo`.
- `archive/v1/` — retired Python v1, kept for reference/rollback only. Not
  load-bearing; don't treat it as source of truth for v2 behavior.

## Documentation boundaries

- `AGENTS.md` is the governing engineering brief: durable architecture map,
  safety rules, verification commands, glossary, and documentation ownership.
  Keep it short and stable. Do not put one-off task plans, temporary UX copy,
  release notes, or speculative roadmap here.
- `README.md` is the current user-facing contract. It should describe what
  the installed tool does today, how to install/run/configure it, and the
  local-first safety model. Remove stale features instead of explaining their
  history.
- `docs/decisions.md` is for durable rationale and invariants that future
  code changes must respect. It is not a changelog.
- `docs/superpowers/plans/` contains execution artifacts for agents. Plans
  may mention old implementation states; do not treat them as current product
  docs or count them as codebase simplification.
- `archive/v1/` is historical reference only. Do not update it to match v2
  behavior, and do not use it as evidence for current behavior.

## Build, test, run

```bash
go build ./...
go vet ./...
go test ./...
go test ./... -race     # for anything touching internal/refresh or internal/keepalive concurrency
go run ./cmd/cc-watch --help
go test -tags demo ./... # include build-tagged UI demo tests
scripts/test-install.sh # exercises install.sh against a temp HOME, safe to run
```

## Tagged release protocol

Use this process for every `v*` release tag. A Git tag and a GitHub Release are
separate actions. Publishing the GitHub Release requires explicit user approval.

1. Update `internal/app/version.go`, `README.md`, and version assertions
   together. Keep the worktree clean and run the normal tests, vet, build,
   demo tests, and `scripts/test-install.sh` checks.
2. Create and verify the annotated tag, then push the commit and tag:

   ```bash
   TAG=v1.0.0-beta.2
   git tag -a "$TAG" -m "$TAG"
   git show --no-patch "$TAG"
   git push origin main
   git push origin "$TAG"
   ```

3. Build macOS release archives from the tagged commit, not from a newer
   `main`. On a clean checkout at the tag, verify
   `git rev-parse "$TAG^{}"` equals `git rev-parse HEAD`. If `main` has moved,
   use a temporary clean checkout at the tag first. Go selects the target with
   `GOOS` and `GOARCH`; this project ships `darwin/arm64` and `darwin/amd64`
   only:

   ```bash
   TAG=v1.0.0-beta.2
   VERSION="${TAG#v}"
   RELEASE_DIR="dist/releases/$TAG"
   mkdir -p "$RELEASE_DIR/arm64" "$RELEASE_DIR/amd64"
   test "$(go run ./cmd/cc-watch --version)" = "cc-watch $VERSION"
   CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -o "$RELEASE_DIR/arm64/cc-watch" ./cmd/cc-watch
   CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -o "$RELEASE_DIR/amd64/cc-watch" ./cmd/cc-watch
   tar -czf "$RELEASE_DIR/cc-watch_${TAG}_darwin_arm64.tar.gz" -C "$RELEASE_DIR/arm64" cc-watch
   tar -czf "$RELEASE_DIR/cc-watch_${TAG}_darwin_amd64.tar.gz" -C "$RELEASE_DIR/amd64" cc-watch
   (cd "$RELEASE_DIR" && shasum -a 256 cc-watch_*.tar.gz > SHA256SUMS && shasum -a 256 -c SHA256SUMS)
   ```

4. Create a draft prerelease with generated notes and the two archives plus
   `SHA256SUMS`, then inspect the assets and notes:

   ```bash
   gh release create "$TAG" "$RELEASE_DIR"/cc-watch_*.tar.gz "$RELEASE_DIR"/SHA256SUMS \
     --repo rcverse/cc-watch --title "cc-watch $TAG" --prerelease --draft \
     --verify-tag --generate-notes
   gh release view "$TAG" --repo rcverse/cc-watch
   ```

5. Publish only after review:

   ```bash
   gh release edit "$TAG" --repo rcverse/cc-watch --draft=false
   ```

6. After the release is public, update the separate
   `rcverse/homebrew-cc-watch` tap: change `Formula/cc-watch.rb` to the new
   release URLs and SHA256 values, run the formula checks, then commit and push
   the tap. Test the user path with `brew install rcverse/cc-watch/cc-watch`,
   `cc-watch --version`, and `brew test cc-watch`.

Never force-move a published tag. Use the next beta or patch version for
subsequent changes. Keep `dist/releases/` untracked and remove it after the
release if local disk space matters.

## Glossary

- **Session Snapshot** — the parsed view of sessions plus config-derived
  defaults, consumed by TUI startup and TUI refresh.
- **Cache Status** — one session's active/expired/unknown state, TTL
  timing, and cache tier.
- **Refresh Runtime** — the internal TUI mechanism that updates snapshots
  from manual refresh, filesystem events, and the safety tick. Not a
  public watch mode.
- **KeepAlive Runtime** — the bounded, visible, cancellable automation for
  optionally sending one configured Claude message to one selected
  session.
- **Send Limit** — the per-session KeepAlive cap. It prevents accidental
  infinite automation; reaching it is normal and resettable by the user.
- **Route** — a TUI screen's local focus/render/action behavior (List,
  Workspace, Ambiguous, Config).

## Hard rules

- Never let a test or manual verification run a real KeepAlive send (never
  actually invoke `claude -r ... -p ...` against a real session). Use fake
  runners and fixture homes.
- Never write to `~/.claude/projects/**/*.jsonl` — read-only, always.
- `statusline` is the only feature besides KeepAlive allowed to spawn a
  subprocess, and only the user's own configured statusline command,
  argv-only (never a shell), bounded 5s timeout, always relays output and
  exits 0. The runtime hook and `--check` never write
  `~/.claude/settings.json`; config TUI install/uninstall may edit it only
  for unambiguous statusLine states, with a timestamped backup first.
- Don't add Linux/Windows support, a daemon, a public watch/interval flag, or
  direct Anthropic API calls without the user asking first — these are
  deliberate non-goals, not oversights.
- This tool is beta and local-first. Do not preserve compatibility for
  removed or stale internal surfaces unless the user explicitly asks for a
  migration path; prefer deleting old flags, config knobs, tests, and docs.
- `cc-watch` (the Go binary) is the live installed command
  (`$HOME/.local/bin/cc-watch`, switched over from v1 on 2026-07-02).
  The Python implementation under `archive/v1/` is historical only.

## Where to look for "why"

`docs/decisions.md` has the distilled rationale for KeepAlive subprocess
safety, refresh timing, statusline subprocess behavior, and non-goals. Read it
before changing behavior in those areas.

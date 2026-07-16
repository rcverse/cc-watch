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
- `docs/releases/<tag>.md` is the versioned source for the exact GitHub
  Release body. Create one for every release; do not rely on generated notes.
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

## Update and tagged release protocol

Treat “update to …” as a gated release request. The target tag is the release
contract; do not edit a published tag.

1. **Propose and validate the version.** If the user did not provide a tag,
   propose one before editing: increment the beta suffix for continued beta
   work, use a patch for compatible fixes, a minor version for a compatible
   feature, and a major version for a breaking change. State the current
   version, target, rationale, and whether the target tag already exists. If a
   requested target conflicts with that classification, surface it for approval.

2. **Formalize the changelog.** Summarize the scoped work in
   `docs/releases/<tag>.md`, including the tag in the heading. This file is
   reviewed as the release body and is the only source used by the release
   workflow. Get approval for the version and notes before the release is
   prepared.

3. **Prepare and validate.** Update `internal/app/version.go`, `README.md`,
   and version assertions together. Run the local checks, and confirm the
   worktree is a clean `main`, the binary reports the target version, README
   contains the target tag, and the release-notes file exists and names it.

4. **Create the draft.** Commit and push the prepared changes, then run:

   ```bash
   scripts/release.sh v1.0.0-beta.5
   ```

   The script enforces the preflight gates, creates an annotated tag, and
   pushes it. `.github/workflows/release.yml` repeats the checks, builds the
   macOS archives and `SHA256SUMS`, and creates a draft prerelease using the
   versioned notes file.

5. **Review and publish.** Inspect the draft body, assets, checksums, and
   workflow result:

   ```bash
   TAG=v1.0.0-beta.5
   gh release view "$TAG" --repo rcverse/cc-watch
   ```

   Publishing is a separate explicit gate:

   ```bash
   gh release edit "$TAG" --repo rcverse/cc-watch --draft=false
   ```

6. **Verify distribution.** Publication triggers
   `.github/workflows/update-homebrew.yml`, which updates the separate
   `rcverse/homebrew-cc-watch` tap. The workflow registers the checkout as a
   Homebrew tap, trusts it, runs the formula checks, and commits the formula
   update to the tap's `main` branch. If the post-publish run needs recovery,
   dispatch it with the exact tag; do not create another release:

   ```bash
   gh workflow run update-homebrew.yml --ref main -f tag="$TAG"
   ```

   Before the first run, `HOMEBREW_TAP_TOKEN` must be a fine-grained token
   scoped only to `rcverse/homebrew-cc-watch` with **Contents: Read and write**.
   Verify the user path with `brew install rcverse/cc-watch/cc-watch`,
   `cc-watch --version`, and `brew test cc-watch`.

Never force-move a published tag. If an unpublished tag must be deleted and
recreated, obtain explicit approval first. Keep `dist/releases/` untracked and
remove it after a release if local disk space matters.

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

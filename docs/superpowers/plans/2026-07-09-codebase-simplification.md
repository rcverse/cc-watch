# Codebase Simplification Plan

Goal: remove the old KeepAlive product model that still exists underneath the new simpler UI.

Plain-language version: the UI now says "KeepAlive is armed, it will send automatically, and it stops at a limit." The code still partly thinks in the older model: auto-send can be off, manual prompt is a first-class path, scope has a mode enum, and the TUI keeps a second enabled map beside the KeepAlive manager. That is why the first cleanup only removed a small number of real code lines. The next cut needs to delete old concepts, not just rename UI copy.

This plan does not count plan files, local build artifacts, or archive files as code savings.

## Phase 1: Delete Dead Leftovers

Target: small, low-risk cuts left after the last pass.

- Delete unused `onOffText` in `internal/tui/workspace.go`.
- Remove stale README/docs copy that still explains `auto_send` and manual prompt as normal product behavior.
- Keep the demo tool; it is still useful for rare UI states and is build-tagged out of production.

Expected cut: 20-40 lines.

Verification:

```bash
env GOCACHE=/private/tmp/cc-watch-gocache go test ./...
env GOCACHE=/private/tmp/cc-watch-gocache go test -tags demo ./...
```

## Audit Corrections

The latest product decision is stronger than the audit's first assumption: cc-watch has not been released, and there are no compatibility obligations for unreleased JSON/config surfaces. Do not keep shims just to preserve shapes nobody uses.

Compatibility stance:

- Remove `--json` entirely instead of preserving or redesigning it.
- Remove JSON schema docs and tests.
- Remove old config knobs instead of accepting/saving compatibility aliases.
- `EvaluateTiming.AutoSendAllowed` must remain as a safety gate. "KeepAlive is automatic" must not become "send even when unsafe."

Revised order: Phase 0, Phase 1, Phase 3, Phase 2 plus Phase 4, Phase 5, Phase 6. Delete unreleased surfaces before reshaping the state machine.

## Phase 0: Remove JSON Output

Target: delete the unused machine-readable API before it keeps preserving stale product concepts.

Cut:

- Remove the `--json` flag, `ModeJSON`, and JSON help copy.
- Remove `runJSON`, `writeJSONError`, `jsonStateFromSnapshot`, and JSON conversion helpers from `internal/app`.
- Delete `internal/jsonout`.
- Delete JSON-only CLI tests.
- Remove the JSON output section from README and decisions docs.

Replacement: nothing. If a real script use case appears later, add a narrow single-session cache envelope then.

Expected cut: 450-700 lines.

Risk:

- Existing unreleased local scripts using `cc-watch --json` will break. This is accepted.

## Phase 2: Remove Per-Session AutoSend

Target: the biggest stale concept.

Current problem: `AutoSend` exists in config, state, snapshot, JSON, tests, and TUI logic. But the agreed product model is simpler: if KeepAlive is on, it is automatic. If automatic sending is unsafe, the UI should show paused/failed with fallback, not a separate manual mode.

Cut:

- Remove `KeepAliveConfig.AutoSend`.
- Remove `SessionState.AutoSend`.
- Remove `Manager.SetAutoSend`.
- Remove config validation branches that depend on `AutoSend`.
- Remove config tests that preserve/toggle `auto_send`.
- Update README/docs to say KeepAlive always auto-sends until max sends is reached.

Replacement:

- Keep `EvaluateTiming.AutoSendAllowed` as an internal safety result, or rename it to `SendAllowed`.
- When sending is not safe, enter the existing paused/failure path and show fallback.

Expected cut: 180-300 lines.

Risk:

- Do not remove the timing safety gate.

## Phase 3: Remove Scope Mode

Target: one-value enum.

Current problem: config has:

```json
"scope": { "mode": "max_sends", "max_sends": 5 }
```

There is only one mode. The UI calls this "Sends" now. Keeping `mode` means every loader, validator, snapshot, JSON output, and test carries a fake abstraction.

Cut:

- Remove `ScopeConfig.Mode`.
- Validate only max sends.
- Internally use `MaxSends` directly.

Replacement:

Use one current config shape; no compatibility aliases are required. Keep `scope.max_sends` only if it is still the simplest config shape after `mode` is removed.

Expected cut: 80-140 lines.

Risk:

- Existing local config files may need a one-time manual update. This is accepted because the tool is unreleased.

## Phase 4: Delete TUI keepAliveEnabled Mirror

Target: duplicated state.

Current problem: the TUI tracks KeepAlive state twice:

- `keepAliveManager.State(sessionID)`
- `Model.keepAliveEnabled[sessionID]`

This creates sync code and subtle questions like "is it enabled because the map says so or because manager state says so?"

Cut:

- Remove `Model.keepAliveEnabled`.
- Remove `Options.KeepAliveEnabled`.
- Remove writes to that map in list/workspace/update actions.
- Implement enabled as `state != off`.
- Make demo fixtures seed `KeepAliveStates` directly.
- Make unavailable/expired enforcement call manager disable only.

Replacement:

```go
func (m Model) KeepAliveEnabled(sessionID string) bool {
    return m.KeepAliveState(sessionID).State != keepalive.StateOff
}
```

Expected cut: 70-120 lines.

Risk:

- List chips and unavailable banners currently use the mirror map. Tests should catch this.

## Phase 5: Collapse Cancelled/Manual States

Target: internal states that still reflect old UI.

Current problem: `StateManualReady`, `ActionManualPromptShown`, and `StateCancelledInstance` are still first-class. The UI now treats them as paused/failed or canceled. Keeping them makes rendering and tests enumerate states the user should not need to understand.

Cut:

- Replace `StateManualReady` with an existing failure/paused state, or add one internal `StatePaused` only if reusing failure would lie to the UI.
- Replace `ActionManualPromptShown` with failure/paused notification only if still needed.
- Replace `StateCancelledInstance` with `StateMonitoringIdle` plus `TriggerArmed=false`.

Replacement:

- Public card remains `✕ Failed` or `✓ Armed`.
- Controls remain `Send now`, `Dismiss`, `Stop check`, `Reset limit` only where useful.

Expected cut: 140-220 lines.

Risk:

- Do not replace `StateCancelledInstance` with `StateOff`; that would disable KeepAlive instead of skipping the current window.
- Do this after `AutoSend` is gone, not before.

## Phase 6: Shrink Tests After Model Cuts

Target: tests that preserve the old complexity.

Current problem: tests enumerate internal states in render tests. That makes internal state cleanup expensive and encourages UI surface area to grow.

Cut:

- Keep focused state-machine tests in `internal/keepalive`.
- Keep TUI smoke tests for:
  - Armed
  - Countdown
  - Limit reached
  - Failed
  - Config save notice
  - Demo time travel
- Delete render matrix cases that assert old internal state labels.

Expected cut: 200-350 lines.

Risk:

- Do not delete tests for subprocess safety, stale token handling, expired-session blocking, or no-real-send guarantees.

## Not In This Plan

- Do not delete `tools/ui-demo`; it is useful and non-distributable.
- Do not delete `archive/v1` unless project policy changes.
- Do not count `.git-corrupt-2026-06-05`, `dist/cc-watch`, or plan docs as code savings.
- Do not add new abstractions to replace old abstractions.

## Expected Net

Real code/test/doc cut: about 700-950 lines.

Dependency cut: 0.

Best first implementation batch: Phase 0. Then Phase 1 plus Phase 3. Then do Phase 2 and Phase 4 together, because removing `AutoSend` makes the TUI enabled mirror mostly redundant. Phase 5 is last because it changes state-machine transitions.

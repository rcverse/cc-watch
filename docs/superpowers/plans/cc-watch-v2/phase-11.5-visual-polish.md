# cc-watch v2 Phase 11.5: Visual Polish

> Inserted after Phase 11 and before Phase 12 at user request because the Go TUI regressed from the v1 Python visual quality.

## Phase 11.5: Visual Parity And Terminal Polish

**Purpose:** Make the TUI feel like a polished terminal product before local packaging, without changing CLI contracts, JSON output, KeepAlive safety, or installer behavior.

- [x] **Step 11.5.1: Shared visual primitives**
  - Modify: `internal/tui/styles.go`, `internal/tui/render_test.go`.
  - Add reusable primitives for bordered panels, compact dividers, visual headers, status badges, and progress bars.
  - Assertions: progress bars use filled/empty glyphs, panels have visible boundaries, badges retain text labels, and color is not the only signal.
  - Run: `go test ./internal/tui -run 'Test.*Visual|Test.*Panel|Test.*Progress|Test.*Badge|TestSemanticStyles'`.
  - Expected: pass.

- [x] **Step 11.5.2: List and Workspace visual hierarchy**
  - Modify: `internal/tui/list.go`, `internal/tui/workspace.go`, `internal/tui/render_test.go`.
  - Improve List View with a persistent header, system-state badges, a sessions panel, row progress bars, and clearer selected/focus treatment.
  - Improve Session Workspace with bordered Evidence and Controls panels, compact section dividers, and progress bars where they express cache elapsed or KeepAlive countdown state.
  - Assertions: list rows and workspace evidence include progress bars and bounded panels while preserving degraded state text and KeepAlive safety copy.
  - Run: `go test ./internal/tui`.
  - Expected: pass.

- [x] **Step 11.5.3: Config Editor visual hierarchy**
  - Modify: `internal/tui/config_editor.go`, `internal/tui/render_test.go`.
  - Improve Config Editor with a persistent header, framed field groups, visible validation panel, and stable footer.
  - Assertions: Reminder, KeepAlive automation, What will happen, and Validation render as bounded groups; reset confirmation remains inline, not modal.
  - Run: `go test ./internal/tui`.
  - Expected: pass.

- [x] **Step 11.5.4: Full verification and smoke**
  - Run: `GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui ./internal/app`.
  - Run: `GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test -count=1 ./...`.
  - Build: `GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go build -o /private/tmp/cc-watch-phase115-bin/cc-watch ./cmd/cc-watch`.
  - Smoke: open List View, Session Workspace, and Config Editor with fixture or temporary `HOME`; exit with `q`.
  - Expected: visual hierarchy is visible, no real Claude send is run, and no installed command path is replaced.

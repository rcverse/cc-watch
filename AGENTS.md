# AGENTS.md

## cc-cache v2 Harness

Before implementation, read:

1. `docs/superpowers/runbooks/2026-06-03-cc-cache-v2-implementation-runbook.md`
2. `docs/superpowers/progress/cc-cache-v2-progress.md`
3. `docs/superpowers/plans/cc-cache-v2/PLAN.md`
4. The current phase file only

Product/design source of truth:

- `PRD.md`
- `docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md`

Rules:

- Execute linearly by phase.
- Use fresh context per phase when practical.
- Use the prompt bank in `docs/superpowers/prompts/cc-cache-v2/`.
- Do not run a real Claude KeepAlive send.
- Do not replace `$HOME/.local/bin/cc-cache` without explicit approval.
- Do not publish releases or create Homebrew tap work without approval.
- Do not use retired plans as source of truth.
- Update `docs/superpowers/progress/cc-cache-v2-progress.md` before stopping implementation work.

# ADR: Go Module And Release Identity

Date: 2026-06-03

## Status

Accepted

## Context

cc-cache v2 is a Go rewrite of the existing terminal utility. The implementation needs one repository-level identity for module imports, local builds, install scripts, docs, and later release metadata.

Public release work has more risk than local binary installation because it depends on GitHub Release and Homebrew tap setup. That work should not block the first verified local binary.

## Decision

Use one repository for v2 governance and implementation: `/Users/richardchen/Dev/cc-cache`.

Use this Go module path:

```bash
go mod init github.com/richardchen/cc-cache
```

Do not run `go mod init` until the Go skeleton phase. Phase 1 records and executes that command after verifying the local Go toolchain.

Local binary install is the first distribution target. GitHub Releases, goreleaser publishing, and Homebrew tap work are deferred release tasks and require separate user approval before publication.

Do not create a separate Homebrew tap repository during implementation.

## Consequences

Internal package imports have a stable module path from the start of Go implementation. Release metadata can be added later without changing the module identity.

The project can verify local behavior before taking on public distribution risk.

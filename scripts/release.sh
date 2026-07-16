#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

die() {
  echo "release: $*" >&2
  exit 1
}

if [[ $# -ne 1 ]]; then
  die "usage: scripts/release.sh v1.0.0-beta.5"
fi

TAG="$1"
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  die "tag must look like v1.0.0 or v1.0.0-beta.4"
fi

if [[ "$(git branch --show-current)" != "main" ]]; then
  die "run from the main branch"
fi

if [[ -n "$(git status --porcelain)" ]]; then
  die "worktree must be clean; commit the version and documentation changes first"
fi

if git rev-parse --verify --quiet "$TAG" >/dev/null; then
  die "tag already exists locally: $TAG"
fi

if git ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
  die "tag already exists on origin: $TAG"
fi

VERSION="${TAG#v}"
NOTES_FILE="docs/releases/$TAG.md"
if [[ ! -s "$NOTES_FILE" ]]; then
  die "missing release notes: $NOTES_FILE"
fi
if ! grep -Fq "$TAG" "$NOTES_FILE"; then
  die "release notes must mention $TAG: $NOTES_FILE"
fi
if ! grep -Fq "$TAG" README.md; then
  die "README.md must mention $TAG"
fi

EXPECTED="cc-watch $VERSION"
ACTUAL="$(go run ./cmd/cc-watch --version)"
if [[ "$ACTUAL" != "$EXPECTED" ]]; then
  die "binary reports $ACTUAL; expected $EXPECTED"
fi

go build ./...
go vet ./...
go test ./...
go test ./... -race
go test -tags demo ./...
scripts/test-install.sh

git push origin main
git tag -a "$TAG" -m "$TAG"
test "$(git rev-parse "$TAG^{}")" = "$(git rev-parse HEAD)"
git show --no-patch "$TAG"
git push origin "$TAG"

echo "release: pushed $TAG and triggered the GitHub draft-release workflow"

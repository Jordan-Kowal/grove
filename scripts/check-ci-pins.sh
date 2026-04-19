#!/bin/sh
# Verifies that tool versions pinned in GitHub Actions workflows match the
# versions declared in go.mod. Prevents CI drift where `@latest` silently
# breaks releases.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/build-and-release.yml"

fail() {
  echo "✗ $1" >&2
  exit 1
}

# Wails — pinned in CI must match go.mod
GOMOD_WAILS=$(grep -E '^require github.com/wailsapp/wails/v3 v' "$ROOT/go.mod" | awk '{print $3}')
CI_WAILS=$(grep -oE 'wails3@v[0-9][^ "]*' "$WORKFLOW" | head -1 | sed 's/^wails3@//')

if [ -z "$GOMOD_WAILS" ]; then
  fail "Could not find Wails version in go.mod"
fi
if [ -z "$CI_WAILS" ]; then
  fail "Could not find pinned Wails version in $WORKFLOW (expected wails3@v<version>)"
fi
if [ "$GOMOD_WAILS" != "$CI_WAILS" ]; then
  fail "Wails version mismatch: go.mod=$GOMOD_WAILS CI=$CI_WAILS"
fi
echo "✓ Wails: $GOMOD_WAILS"

# Task — CI must pin an explicit version (no @latest)
CI_TASK=$(grep -oE 'task/v3/cmd/task@v?[0-9][^ "]*' "$WORKFLOW" | head -1 | sed 's/^.*@//')
if [ -z "$CI_TASK" ]; then
  fail "Could not find pinned Task version in $WORKFLOW (expected task@v<version>)"
fi
if echo "$CI_TASK" | grep -q "latest"; then
  fail "Task is pinned to 'latest' in $WORKFLOW — pin an explicit version"
fi
echo "✓ Task: $CI_TASK"

echo ""
echo "All CI tool versions correctly pinned."

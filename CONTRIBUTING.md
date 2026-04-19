# Contributing

## Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- [Bun](https://bun.sh/)
- [Wails v3](https://v3alpha.wails.io/) — install the exact version used by the
  project: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.77`
  (pinned to match `go.mod`; `task lint` enforces this)
- [Task](https://taskfile.dev/) — `go install github.com/go-task/task/v3/cmd/task@v3.50.0`
- [golangci-lint](https://golangci-lint.run/)

## Setup

```shell
git config core.hooksPath .githooks
bun install --frozen-lockfile
go mod tidy
task dev:build
```

## Running

```shell
task dev
```

## Local release builds (signed + notarized)

`task release:local` produces a signed + notarized DMG. Useful for testing
features that depend on macOS TCC state (e.g. Accessibility permission).

1. Copy `.env.example` to `.env` (gitignored).
2. Adjust `SIGN_IDENTITY` + `NOTARY_PROFILE` to match your cert + notarytool
   profile. Defaults are Grove's — external contributors signing with their
   own Apple Developer ID should override both.
3. Run `task release:local`.

Output: `bin/Grove-dev.dmg`.

## Developer Commands

All commands go through [Task](https://taskfile.dev/) (see `Taskfile.yml`):

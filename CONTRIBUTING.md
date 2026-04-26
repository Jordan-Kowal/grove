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

| Task                 | Purpose                                                                   |
| -------------------- | ------------------------------------------------------------------------- |
| `task dev`           | Runs the app in dev mode (Vite + Wails live reload)                       |
| `task dev:build`     | Builds a development binary (no production tag)                           |
| `task build`         | Builds the application for the current platform                           |
| `task package`       | Packages a production build                                               |
| `task run`           | Runs the packaged app                                                     |
| `task release:local` | Builds a signed + notarized local release DMG (reads `.env`)              |
| `task lint`          | Runs all linters (Biome, `tsc`, `golangci-lint`, CI pin check)            |
| `task test`          | Runs the Go test suite (race detector) and the frontend vitest suite      |
| `task vuln`          | Runs `govulncheck` against Go deps (version pinned via `go.mod` tool directive) |
| `task check`         | Runs `lint` + `test` + `vuln`                                             |
| `task generate`      | Regenerates Wails TypeScript bindings from Go services                    |
| `task upgrade`       | Updates all dependencies to latest and runs `check`                       |
| `task version:bump`  | Bumps the app version across all config files (calls `scripts/bump-version.sh`) |
| `task clean`         | Removes build artifacts (`dist/`, `bin/`)                                 |

### Environment variables

- `WAILS_VITE_PORT` — override the Vite dev server port (default `9245`). Read in `Taskfile.yml` and passed to `wails3 dev`.

### Helper scripts

- `scripts/bump-version.sh` — bumps the version in `package.json`, `wails.json`, and other config files. Invoked by `task version:bump`.
- `scripts/check-ci-pins.sh` — lint step that enforces CI tool versions (Wails CLI, Task, govulncheck) match the versions pinned in `go.mod` and `Taskfile.yml`. Fails the `task lint` run if they drift.
- `scripts/claude-postwrite.sh` — helper run by a local git hook (`.githooks/pre-commit`) after Claude Code writes files; normalizes formatting before commit.
- `scripts/update.sh` — invoked by the in-app auto-updater; downloads the latest DMG, mounts it, and replaces the installed app bundle in `/Applications/`.

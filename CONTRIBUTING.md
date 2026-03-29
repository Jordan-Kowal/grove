# Contributing

## Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- [Bun](https://bun.sh/)
- [Wails v3](https://v3alpha.wails.io/) (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`)
- [Task](https://taskfile.dev/) (`go install github.com/go-task/task/v3/cmd/task@latest`)
- [golangci-lint](https://golangci-lint.run/)

## Setup

```shell
git config core.hooksPath .githooks
bun install --frozen-lockfile
go mod tidy
```

## Running

```shell
task dev
```

## Developer Commands

All commands go through [Task](https://taskfile.dev/) (see `Taskfile.yml`):

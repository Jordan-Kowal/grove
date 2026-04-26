# TODO

## Features

- [ ] **Workspace reordering**: Drag-and-drop to reorder workspaces. Requires activation mode with drag anchors on the side. Persisted order — must handle missing/new workspaces gracefully.
- [ ] Renaming a worktree with an alias

## Design

- [ ] Improve logo design

## Performance

- [ ] **Cache `readGroveSessions` by `(path, mtime)`** (`backend/monitor_service.go`): currently re-reads + re-parses every `*.json` in `~/.grove/sessions/` on each 2s tick. Marginal gain until session count grows; add a per-path mtime cache on the struct and reuse cached parse when mtime is unchanged.
- [ ] **Gate `refreshWorkspaces` on filesystem mtime** (`backend/monitor_service.go`): runs on every 2s claude tick, doing `os.ReadDir` + `readConfig` + `scanWorktreeStructure` per workspace even when nothing changed. For stable setups this is wasted IO. Stat `~/.grove/projects/` and each `<name>/config.json` first; skip the rescan when mtimes are unchanged since last pass.

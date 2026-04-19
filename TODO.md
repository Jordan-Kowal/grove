# TODO

## Features

- [ ] **Workspace reordering**: Drag-and-drop to reorder workspaces. Requires activation mode with drag anchors on the side. Persisted order — must handle missing/new workspaces gracefully.
- [ ] Identify unstashed files in the diff

## Bugs / UX

- [ ] When a git command fails, we dont see the logs

## Design

- [ ] Improve logo design

## Code quality

- [ ] **Split `backend/workspace_service.go`** (~900 lines): extract `workspace_script.go` (runScriptTracked, log streaming), `workspace_git.go` (rebase/checkout/new-branch/list-branches/sync-main/fetch-remote/force-remove/resolve-git-dir), `workspace_validate.go` (validateName, validateBranchName, regex consts). Same package, no visibility changes.
- [ ] **Split `backend/monitor_service.go`** (~690 lines): extract `monitor_hook.go` (hookScript + installHook), `monitor_claude.go` (groveSession, readGroveSessions, refreshClaude, resolveWorktreePath, groveStateToClaudeStatus, isProcessAlive, sessionCountsEqual, DismissDone). Preserve lock discipline.
- [ ] **Split `src/features/dashboard/contexts/DashboardProvider.tsx`** (~455 lines): extract `useEditorActions.ts` (focusEditor, closeEditor, closeAllEditors) and `useTaskEvents.ts` (worktree-task subscription, log streaming). Hooks take store setters as args. Preserve context identity.

## Security

- [ ] **Replace hand-rolled AppleScript escaping** (`backend/editor_service.go`): manual escape of paths passed to `osascript` is fragile long-term. Consider a library or switch to argv-style `osascript -e '...' -- "$path"` invocation.

## Performance

- [ ] **Cache `readGroveSessions` by `(path, mtime)`** (`backend/monitor_service.go`): currently re-reads + re-parses every `*.json` in `~/.grove/sessions/` on each 2s tick. Marginal gain until session count grows; add a per-path mtime cache on the struct and reuse cached parse when mtime is unchanged.
- [ ] **Combine `ansiToSegments` 8 sequential regex replaces** (`src/utils/ansiToSegments.ts:39-47`): fold them into a single alternation regex to halve string allocations on long log lines. Must preserve ANSI color + style attribute parsing — add tests before refactoring.

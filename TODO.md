# TODO

## Feature ideas

- [ ] Add a card + claude monitor for the main repo as well
- [ ] **Workspace filter**: Dropdown/select per workspace to filter worktrees (not persisted across sessions)
- [ ] **Keyboard shortcuts**: Global hotkeys for common actions (toggle visibility, create worktree, open in editor, navigate worktrees, dismiss notifications). User-configurable bindings.
- [ ] **Workspace reordering**: Drag-and-drop to reorder workspaces. Requires activation mode with drag anchors on the side. Persisted order — must handle missing/new workspaces gracefully.
- [ ] **Bulk workspace actions**: Add "Remove all worktrees" and "Rebase all worktrees" to the workspace `...` menu.
- [ ] **Stale worktree detection**: Highlight worktrees whose branch is behind `origin/main` by many commits or hasn't been updated in N days. Configurable threshold.
- [ ] **More themes**: Expose additional DaisyUI themes beyond Nord and Forest. Test for sidebar compatibility at 250px.
- [ ] **Claude session count**: Show the number of ongoing Claude Code sessions per worktree (and/or in tray icon badge).
- [ ] **Toast notifications**: Transient toasts when Claude needs attention or finishes — shows worktree name and action needed. Clicking the toast navigates to / highlights the relevant worktree card.
- [ ] **Copy branch name**: One-click copy of branch name from worktree card context menu.
- [ ] **Configurable "done" duration**: Let users configure how long the "done" status persists (currently hardcoded at 30 minutes). Options: instant dismiss, custom duration, persist until clicked.
- [ ] **Improve logo design**

## Code quality

### Bugs

- [ ] `scheduleDoneExpiry` timer callback re-acquires `s.mu` held during scheduling — can revert a `DismissDone` action (`monitor_service.go:411-431`). Move call to after `s.mu.Unlock()`.
- [ ] `refreshClaude` reads `s.workspaces` without lock (`monitor_service.go:300-305`). Add `s.mu.RLock()`/`s.mu.RUnlock()` like `refreshGit` does.
- [ ] `SetPreferences` doesn't validate `mode`/`sound` unlike `PlayPreview` — invalid values silently cause sounds to never play (`sound_service.go:53-58`).
- [ ] Click-through on unfocused window: First click on the sidebar when unfocused only focuses the window, requiring a second click to trigger the action.
- [ ] Allow running dev version while running the app (without conflict)
- [ ] New version builds reset macOS Accessibility/Automation permissions — codesigning with a stable identity may fix this
- [ ] Better handling of issues in settings/workspaces

### Improvements

- [ ] `WorkspaceSettings` calls `WorkspaceService.UpdateWorkspaceConfig` directly, bypassing `DashboardContext` and skipping `MonitorService.RefreshNow()` (`WorkspaceSettings.tsx:73`).
- [ ] Duplicated `Section` component in `GeneralSettings.tsx` and `WorkspaceSettings.tsx` — extract to shared component.
- [ ] Duplicated `createEffect` patterns: extract `useElapsedTimer` hook (used in `TaskStatusBar.tsx`, `ErrorLog.tsx`) and `useOutsideClick` hook (used in `BranchSelect.tsx`, `WorkspaceSection.tsx`, `WorktreeCard.tsx`).
- [ ] Missing barrel re-exports in `src/types/index.ts` for `LogLine`, `WorktreeLogEvent`, `WailsEvent`.

### Nice to have

- [ ] Auto-update: add checksum/signature verification to `curl | bash` update mechanism (`app_service.go`)
- [ ] Pin GitHub Actions to commit SHAs instead of mutable tags in `build-and-release.yml` and `code-quality.yml` (supply chain hardening for release pipeline).
- [ ] Add explicit `permissions` blocks to GitHub Actions workflows (limit token scope — release job needs `contents: write`, others only need read).
- [ ] Guard `ErrorLog.tsx:66` split destructuring — add fallback for `worktreeName` when `logKey` has no `/`.
- [ ] Replace `let scrollRef` with signal-based ref in `ErrorLog.tsx` auto-scroll `createEffect` — current approach works by `queueMicrotask` timing coincidence.

### Tests

- [ ] Add tests for `resolveWorktreePath` (pure function, easy win).
- [ ] Add tests for `refreshClaude`/`DismissDone`/`scheduleDoneExpiry` state machine (requires extracting `soundSvc`/`traySvc` behind interfaces and mocking Wails events).

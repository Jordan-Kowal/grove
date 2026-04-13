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
- [ ] **Click-through on unfocused window**: First click on the sidebar when unfocused only focuses the window, requiring a second click. Fix requires CGo + Objective-C to override `acceptsFirstMouse:` on the WKWebView via `NativeWindow()`.
- [ ] **Accessibility permission notification**: When snap-to-edge fails due to missing Accessibility permission, show an inline notification/banner with a link to open System Settings (using `OpenAccessibilitySettings()`).

### Nice to have

- [ ] Auto-update: add checksum/signature verification to `curl | bash` update mechanism (`app_service.go`)
- [ ] Pin GitHub Actions to commit SHAs instead of mutable tags in `build-and-release.yml` and `code-quality.yml` (supply chain hardening for release pipeline).

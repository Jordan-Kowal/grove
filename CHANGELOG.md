# Changelog

## Legend

- 🚀 Features
- ✨ Improvements
- 🐞 Bugfixes
- 🔧 Others
- 💥 Breaking

## 0.2.1 - 2026-04-15

- 🚀 Detect open editor windows and show "active" badge on worktree cards
- 🚀 Close editor window from worktree card context menu
- 🚀 Close all editor windows from workspace context menu

## 0.2.0 - 2026-04-14

- 🚀 Main repo is now visible as a card and tracked like any worktree
- 🚀 Clicking on a card when the grove app is not focused is now correctly registered
- 🚀 "Remove all worktrees" bulk action in workspace context menu
- ✨ Copy branch name from worktree card context menu
- ✨ All 35 built-in DaisyUI themes now available in settings
- ✨ Session count badge and hover breakdown when multiple Claude sessions run in the same worktree
- ✨ Configurable "done" badge duration in the settings
- ✨ Claude hook settings auto-update on app launch when commands change
- 🐞 "Done" status now surfaces above "working" when multiple sessions run in the same worktree
- 🐞 Fix crash in error log when worktree name is missing from log key
- 🐞 Fix data race in Claude session polling
- 🐞 Workspace config changes (base branch, scripts) now refresh the dashboard immediately
- 🐞 Sound preference validation — invalid values are rejected instead of silently accepted

## 0.1.2 - 2026-03-30

- ✨ Add link to macOS Accessibility settings near "Dock to edge" toggle — helps re-grant permission after updates
- ✨ Update dialog now warns about re-granting Accessibility permission
- 🐞 Install script resets stale TCC permissions on update so macOS re-prompts for Automation
- 🐞 Fix scripts missing CLI tools (`go`, `task`) when launched from Finder — PATH resolution now uses interactive shell mode

## 0.1.1 - 2026-03-30

- 🚀 Added "Sync main checkout" action in workspace menu — resets the main working tree to match HEAD (with confirmation)
- 🐞 Fix PATH resolution for scripts run from Grove — use the user's login shell instead of `/bin/sh`
- 🐞 Fix worktree card inputs (branch switch, new branch) losing focus when another worktree updates
- 🐞 Fix Claude session detection when running from a subdirectory of a worktree
- 🐞 Branch name input now allows `/` for namespaced branches (e.g. `feature/my-branch`)
- 🐞 Git operations targeting remote branches (rebase, checkout, new branch, worktree creation) now fetch the remote first to ensure refs are up to date
- 🐞 Fix dashboard showing empty branch/diff data on app startup until a Claude session is detected

## 0.1.0 - 2026-03-30

Initial release. See the [README](README.md) for the full feature list.

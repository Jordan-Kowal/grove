# Changelog

## Legend

- 🚀 Features
- ✨ Improvements
- 🐞 Bugfixes
- 🔧 Others
- 💥 Breaking

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

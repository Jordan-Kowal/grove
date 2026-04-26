# Changelog

## Legend

- 🚀 Features
- ✨ Improvements
- 🐞 Bugfixes
- 🔧 Others
- 💥 Breaking

## TBD

- 🚀 New "Track active editor windows" setting: turn off the `active` badge and editor polling for editors that share a single window across folders (e.g. recent Zed builds)
- ✨ Failed git operations (worktree create/remove, rebase, checkout, new branch) now expose a "View logs" button with the captured git output, matching the setup-script behavior
- ✨ Update prompt now sets clearer expectations: warns the update runs in the background, may take a few seconds, and points to `~/.grove/update.log` if the app does not reopen
- 🔧 Editor `osascript` calls now pass app/window names via AppleScript `argv` instead of string interpolation, removing hand-rolled escaping and any residual injection risk
- 🔧 Log view ANSI parser collapses 8 sequential regex passes into a single alternation, halving string allocations on long log lines (covered by new vitest suite)

## 0.3.3 - 2026-04-22

- 🚀 New "IDE width" setting: choose what share of the remaining screen space your editor takes when docked (useful for large screens)

## 0.3.2 - 2026-04-20

- 🐞 Auto-update now installs new versions reliably instead of failing with a cryptic "mount failed" error
- 🐞 New files (not yet committed) now show up in the worktree card's diff count instead of being invisible
- 🔧 Monitor loop is more resilient: slow git commands can't freeze the dashboard, and memory no longer grows as worktrees come and go

## 0.3.1 - 2026-04-19

- 🚀 Settings now surface warnings when the Accessibility permission is missing or the configured editor app is not installed
- ✨ Improved performance through various changes (log view cap, bounded git concurrency, refresh debounce, race fix)
- 🔧 Hardened auto-update: version-pinned DMG URL, Apple Developer ID signature verification + Gatekeeper, strict bash, and stderr logged to `~/.grove/update.log`
- 🔧 Atomic writes for `~/.claude/settings.json` to avoid races with Claude Code
- 🔧 Allowlisted event arg in the embedded Claude hook script
- 🔧 Tightened `~/.grove/sessions/` and sound cache directory permissions to `0o700`
- 🔧 Added `govulncheck` as a pinned Go tool dependency (`go.mod`), new `task vuln`, pre-commit + CI enforcement via `check-ci-pins.sh`
- 🔧 Added `task release:local` + `.env` support for producing a signed, notarized DMG locally (see CONTRIBUTING.md)
- 🔧 Pinned Wails and Task CLI versions in CI; added `scripts/check-ci-pins.sh` lint step that enforces CI pins match `go.mod`
- 🔧 Documentation refresh across README, CONTRIBUTING, and CLAUDE.md

## 0.3.0 - 2026-04-19

- 🚀 App is now signed with Apple Developer ID and notarized — no more Gatekeeper warnings or `xattr` workarounds
- 🚀 Installation via `.dmg` with drag-to-Applications window (replaces `.zip` + setup script)
- ✨ Accessibility and AppleEvents permissions now persist across updates (stable signing identity)
- 💥 Removed `setup.sh` one-line installer; download the `.dmg` from the Releases page instead
- 🔧 Auto-update now downloads a `.dmg` and no longer resets TCC permissions
- 🔧 Pinned all GitHub Actions in CI workflows to commit SHAs for supply chain hardening

## 0.2.3 - 2026-04-17

- ✨ Pulsing animation on green (done) and red (needs attention) status badges
- ✨ Loading spinner color now matches the idle blue for visual consistency
- 🐞 Editor "active" badge and Claude session counts now persist across workspace rebuilds
- 🔧 Reduced monitor overhead and improved overall performances

## 0.2.2 - 2026-04-15

- ✨ Remove empty-state placeholder now that the main repo card is always visible

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

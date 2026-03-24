# Changelog

## Legend

- 🚀 Features
- ✨ Improvements
- 🐞 Bugfixes
- 🔧 Others
- 💥 Breaking

## 0.1.0 - 2026-03-18

Initial release.

### 🚀 Features

- **Workspace management**: Register git repositories, create/delete named worktrees with configurable setup and teardown scripts.
- **Collapsible workspaces**: Collapse/expand workspace sections to focus on what matters.
- **Claude Code monitoring**: Real-time session status detection (working, idle, permission, question) via hook-based integration.
- **Git status**: Branch name and diff stats per worktree, with configurable poll interval per workspace (10s–120s, or disabled).
- **Editor integration**: Click a worktree card to open/focus it in your editor (configurable: Zed, VS Code, Cursor, etc.).
- **Sound notifications**: Play a system sound when Claude finishes working. 5 bundled sounds with preview.
- **Dock badge**: Show a red badge on the dock icon when Claude needs input.
- **System tray**: Optional menu bar icon to show/hide Grove (toggleable in settings).
- **Window snapping**: Snap to left/right screen edges at full height.
- **Safe delete**: Confirmation prompt with uncommitted changes and unpushed commits warnings.
- **Live script logs**: Stream setup/teardown script output in real-time, viewable during execution and after failure.
- **/tmp monitor**: Track Claude's temporary file usage with one-click cleanup.
- **Auto-update**: Check for new versions from GitHub releases with one-click update.
- **Settings**: General settings (theme, always on top, snap, sound, system tray, dock badge, editor) and per-workspace settings (base branch, git diff polling, delete branch, setup/teardown scripts) with auto-save.

# Grove

<div align="middle">
  <div>
    <img src="public/app-icon.png" alt="Grove Logo" width="250" height="250" style="position: relative; top: 16px;"/>
  </div>
  <strong>Lightweight worktree dashboard — manage workspaces, monitor Claude Code sessions, and git diffs across multiple worktrees.</strong>
  <br />
  <br />
  <div>
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="License" />
  <img src="https://img.shields.io/github/v/release/Jordan-Kowal/grove" alt="Release" />
  <img src="https://img.shields.io/badge/TypeScript-007ACC?logo=typescript&logoColor=white" alt="TypeScript" />
  <img src="https://img.shields.io/badge/SolidJS-2C4F7C?logo=solid&logoColor=white" alt="SolidJS" />
  <img src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Wails-EB5E28?logo=wails&logoColor=white" alt="Wails" />
  </div>
  <br />
  <br />
</div>

- [Grove](#grove)
  - [Overview](#overview)
  - [Features](#features)
  - [Installation](#installation)
    - [Download](#download)
    - [First Run](#first-run)
  - [Claude Code Hook Setup](#claude-code-hook-setup)
  - [How It Works](#how-it-works)
  - [Settings](#settings)
    - [General Settings](#general-settings)
    - [Workspace Settings](#workspace-settings)
  - [Contributing](#contributing)
  - [License](#license)
  - [Support](#support)

## Overview

**Grove** is a macOS desktop app that sits alongside your editor as a narrow sidebar. It manages git worktrees for parallel development — each worktree gets its own Claude Code session, and Grove monitors them all in real-time.

## Features

- **Workspace management**: Register git repos, create/delete named worktrees with setup and teardown scripts
- **Collapsible workspaces**: Collapse/expand workspace sections to focus on what matters
- **Claude Code monitoring**: Real-time session status (working, idle, waiting for permission/input)
- **Git status**: Branch name and diff stats per worktree (polled every 10s)
- **Editor integration**: Click a worktree card to open/focus it in your editor
- **Sound notifications**: Audio alert when Claude finishes working
- **Dock badge**: Red badge when Claude needs input
- **System tray**: Optional menu bar icon to show/hide Grove
- **Window snapping**: Snap to left/right screen edges at full height
- **Safe delete**: Confirmation prompt with uncommitted changes and unpushed commits warnings
- **Live script logs**: Stream setup/teardown script output in real-time, viewable during execution and after failure
- **/tmp monitor**: Track Claude's temporary file usage with one-click cleanup
- **Auto-update**: One-click update from GitHub releases

## Installation

### Download

**Option 1: One-line installer (Recommended)**

```bash
curl -fsSL https://raw.githubusercontent.com/Jordan-Kowal/grove/main/setup.sh | bash
```

**Option 2: Manual installation**

1. Go to the [Releases page](https://github.com/Jordan-Kowal/grove/releases)
2. Download the latest `Grove-x.x.x.zip` file
3. Double-click the ZIP file to extract it
4. Run: `xattr -cr Grove.app` to remove quarantine attributes
5. Drag the `Grove.app` to your Applications folder
6. Launch the app from Applications or Spotlight

### First Run

On macOS, you may see a security warning when first opening the app. To resolve this:

1. Go to **System Preferences** → **Security & Privacy**
2. Click **"Open Anyway"** next to the Grove warning
3. Alternatively, right-click the app and select **"Open"** from the context menu

## Claude Code Hook Setup

Grove detects Claude session status via a hook script at `~/.grove/hook.sh`, automatically installed when the app starts. Add the following hooks to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.grove/hook.sh working",
            "async": true
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.grove/hook.sh working",
            "async": true
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.grove/hook.sh permission",
            "async": true
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "elicitation_dialog",
        "hooks": [
          {
            "type": "command",
            "command": "~/.grove/hook.sh question",
            "async": true
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.grove/hook.sh done",
            "async": true
          }
        ]
      }
    ]
  }
}
```

## How It Works

1. **Add a workspace** — click `+` and select a git repository folder
2. **Create worktrees** — click `+` on a workspace, enter a name. Grove runs `git worktree add` from your configured base branch, then your setup script
3. **Monitor** — worktree cards show branch, diff stats, and Claude session status in real-time
4. **Click to open** — click a worktree card to focus/open it in your editor
5. **Delete** — use the `...` menu to remove worktrees (confirmation with uncommitted/unpushed warnings, runs teardown script, then git remove)

Workspace config is stored in `~/.grove/projects/<name>/config.json`. Worktrees are created in `~/.grove/projects/<name>/worktrees/`.

## Settings

### General Settings

| Section       | Setting                                    | Default | Description                                               |
| ------------- | ------------------------------------------ | ------- | --------------------------------------------------------- |
| Display       | Theme                                      | Nord    | Switch between Nord and Forest themes                     |
| Display       | Keep window on top                         | On      | Keep Grove above other windows                            |
| Display       | Snap to screen edges                       | On      | Snap window to left/right edges at full height            |
| Notifications | Play sound when Claude finishes            | On      | Audio alert with selectable sound (Glass, Ping, Pop, etc) |
| Notifications | Show menu bar icon                         | Off     | System tray icon to show/hide Grove                       |
| Notifications | Show dock badge when Claude needs input    | On      | Red badge on dock icon                                    |
| Editor        | Default editor                             | Zed     | macOS app name for editor integration                     |

### Workspace Settings

| Setting                   | Default     | Description                                            |
| ------------------------- | ----------- | ------------------------------------------------------ |
| Branch new worktrees from | origin/main | Start point for new worktrees                          |
| Delete local branch       | On          | Clean up the branch after deleting a worktree          |
| Setup script              | —           | Shell command to run after creating a worktree         |
| Teardown script           | —           | Shell command to run before removing a worktree        |

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and quality checks
5. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/Jordan-Kowal/grove/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Jordan-Kowal/grove/discussions)

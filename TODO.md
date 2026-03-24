# TODO

## 0.1.0

- [ ] App icon (replace click-launch placeholder), same for tray icon
- [ ] Readme/Changelog/Contributing update with screenshots

## Later

- [ ] Improve performance for git polling on large repos. It is consuming lots of CPU every 10s when the repo is massive. Maybe: skip untracked files (`-uno`), disable rename detection (`--no-renames`), cache `.git/index` mtime to skip unchanged worktrees, parallelize per-worktree git calls, etc. Or maybe disable the setting on specific repo?
- [ ] Allow seeing the multiple existing claude session in the card
- [ ] Better handling of issues in settings/workspaces
- [ ] Auto-update: add checksum/signature verification to `curl | bash` update mechanism (`app_service.go`)
- [ ] Click-through on unfocused window: First click on the sidebar when unfocused only focuses the window, requiring a second click to trigger the action.

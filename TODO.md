# TODO

## Feature ideas

- [ ] **Workspace reordering**: Drag-and-drop to reorder workspaces. Requires activation mode with drag anchors on the side. Persisted order — must handle missing/new workspaces gracefully.
- [ ] **Accessibility permission notification**: When snap-to-edge fails due to missing Accessibility permission, show an inline notification/banner with a link to open System Settings (using `OpenAccessibilitySettings()`).

### Nice to have

- [ ] Auto-update: add checksum/signature verification to `curl | bash` update mechanism (`app_service.go`)
- [ ] Pin GitHub Actions to commit SHAs instead of mutable tags in `build-and-release.yml` and `code-quality.yml` (supply chain hardening for release pipeline).
- [ ] **Improve logo design**
- [ ] When a git command fails, we dont see the logs

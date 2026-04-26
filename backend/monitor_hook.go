package backend

import (
	"log"
	"os"
	"path/filepath"
)

// hookScript is a defensive-only allowlist: Grove itself always passes one of
// the four known states, but the script lives in $HOME and is referenced from
// user-editable ~/.claude/settings.json. If a future refactor starts shelling
// out with $1, or a user hand-edits the settings file to pass an unusual
// value, the allowlist keeps garbage out of the session JSON.
const hookScript = `#!/bin/sh
# Grove status hook — writes Claude session state to ~/.grove/sessions/
# Usage: hook.sh <state>  (working|permission|question|done)
# Called by Claude Code hooks. Uses PPID (the Claude Code process) as the stable identifier.
case "$1" in
  working|permission|question|done) ;;
  *) exit 1 ;;
esac
mkdir -p "$HOME/.grove/sessions"
escaped_cwd=$(printf '%s' "$PWD" | sed 's/\\/\\\\/g; s/"/\\"/g')
printf '{"state":"%s","cwd":"%s","pid":%d}\n' "$1" "$escaped_cwd" "$PPID" > "$HOME/.grove/sessions/$PPID.json"
`

// installHook ensures ~/.grove/sessions/ exists, ~/.grove/hook.sh is up to date,
// and Claude Code settings.json has the required hook entries.
func (s *MonitorService) installHook() {
	homeDir, _ := os.UserHomeDir()
	groveDir := filepath.Join(homeDir, ".grove")
	sessionsDir := filepath.Join(groveDir, "sessions")
	hookPath := filepath.Join(groveDir, "hook.sh")

	if err := os.MkdirAll(sessionsDir, 0o700); err != nil { // #nosec G301 -- user-only dir
		log.Printf("grove: failed to create sessions dir: %v", err)
		return
	}
	if err := os.WriteFile(hookPath, []byte(hookScript), 0o700); err != nil { // #nosec G306 -- needs execute permission
		log.Printf("grove: failed to write hook script: %v", err)
	}

	ensureClaudeSettings(hookPath)
}

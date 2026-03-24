package backend

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// EditorService handles focusing editor windows.
type EditorService struct{}

// NewEditorService creates a new EditorService.
func NewEditorService() *EditorService {
	return &EditorService{}
}

// escapeAppleScript escapes characters for safe use in AppleScript strings and
// single-quoted shell contexts within `do shell script`.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `'`, `'"'"'`)
	return s
}

// IsValidApp checks that a macOS application exists by name.
func (s *EditorService) IsValidApp(name string) bool {
	if name == "" {
		return false
	}
	cmd := exec.Command("open", "-Ra", name) // #nosec G204 -- user-selected app name, escaped downstream
	return cmd.Run() == nil
}

// FocusEditor focuses or opens the editor window for the given worktree path.
func (s *EditorService) FocusEditor(worktreePath string, editorApp string) error {
	if editorApp == "" {
		editorApp = "Zed"
	}

	if !s.IsValidApp(editorApp) {
		return fmt.Errorf("application %q not found", editorApp)
	}

	safePath := escapeAppleScript(worktreePath)
	safeApp := escapeAppleScript(editorApp)
	safeName := escapeAppleScript(filepath.Base(worktreePath))

	script := fmt.Sprintf(`
tell application "System Events"
	if not (exists process "%s") then
		do shell script "open -a '%s' '%s'"
		return
	end if
	set editorProcess to first process whose name is "%s"
	set frontmost of editorProcess to true
	set foundWindow to false
	tell editorProcess
		repeat with w in windows
			if name of w contains "%s" then
				perform action "AXRaise" of w
				set foundWindow to true
				exit repeat
			end if
		end repeat
	end tell
	if not foundWindow then
		do shell script "open -a '%s' '%s'"
	end if
end tell
`, safeApp, safeApp, safePath, safeApp, safeName, safeApp, safePath)

	cmd := exec.Command("osascript", "-e", script) // #nosec G204 -- app validated via isValidApp + escaped
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("AppleScript failed: %s: %w", string(out), err)
	}
	return nil
}

// PositionWindow sets the frontmost window of the named app to the given bounds.
func (s *EditorService) PositionWindow(appName string, x, y, width, height int) error {
	if appName == "" || width <= 0 || height <= 0 {
		return nil
	}

	safeApp := escapeAppleScript(appName)
	// AppleScript bounds are {left, top, right, bottom}
	script := fmt.Sprintf(`
tell application "System Events"
	if exists process "%s" then
		tell process "%s"
			set position of front window to {%d, %d}
			set size of front window to {%d, %d}
		end tell
	end if
end tell
`, safeApp, safeApp, x, y, width, height)

	cmd := exec.Command("osascript", "-e", script) // #nosec G204 -- app validated upstream + escaped
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("AppleScript position failed: %s: %w", string(out), err)
	}
	return nil
}

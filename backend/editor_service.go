package backend

import (
	"fmt"
	"os/exec"
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

	cmd := exec.Command("open", "-a", editorApp, worktreePath) // #nosec G204 -- app validated via IsValidApp
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("open editor failed: %s: %w", string(out), err)
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

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

// IsValidApp checks that a macOS application exists by name.
func (s *EditorService) IsValidApp(name string) bool {
	if name == "" {
		return false
	}
	cmd := exec.Command("open", "-Ra", name) // #nosec G204 -- user-selected app name passed as exec argv, not shell-interpreted
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

// GetOpenEditorPaths returns the list of directory paths currently open in the editor,
// by querying macOS for the window titles of the given application.
func (s *EditorService) GetOpenEditorPaths(editorApp string) []string {
	if editorApp == "" {
		editorApp = "Zed"
	}
	script := `
on run argv
	set appName to item 1 of argv
	tell application "System Events"
		if exists process appName then
			tell process appName
				set windowNames to name of every window
			end tell
			set output to ""
			repeat with wName in windowNames
				set output to output & wName & linefeed
			end repeat
			return output
		end if
	end tell
	return ""
end run
`

	cmd := exec.Command("osascript", "-e", script, "--", editorApp) // #nosec G204 -- app name passed via argv, not interpolated
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var titles []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			titles = append(titles, line)
		}
	}
	return titles
}

// MatchOpenPaths takes window titles and a list of known worktree paths,
// and returns the set of paths that have a matching open window.
// Each window title is matched to the longest (most specific) path whose
// base directory name appears in the title, preventing parent directories
// from false-matching when a child worktree is the one actually open.
func (s *EditorService) MatchOpenPaths(windowTitles []string, paths []string) map[string]bool {
	result := make(map[string]bool)
	for _, title := range windowTitles {
		bestPath := ""
		for _, p := range paths {
			dirName := filepath.Base(p)
			if strings.Contains(title, dirName) && len(p) > len(bestPath) {
				bestPath = p
			}
		}
		if bestPath != "" {
			result[bestPath] = true
		}
	}
	return result
}

// CloseEditorWindow closes the editor window matching the given worktree path.
func (s *EditorService) CloseEditorWindow(worktreePath string, editorApp string) error {
	if editorApp == "" {
		editorApp = "Zed"
	}
	dirName := filepath.Base(worktreePath)
	script := `
on run argv
	set appName to item 1 of argv
	set needle to item 2 of argv
	tell application "System Events"
		if exists process appName then
			tell process appName
				set windowList to every window whose name contains needle
				repeat with w in windowList
					click (first button of w whose subrole is "AXCloseButton")
				end repeat
			end tell
		end if
	end tell
end run
`

	cmd := exec.Command("osascript", "-e", script, "--", editorApp, dirName) // #nosec G204 -- args passed via argv, not interpolated
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("close editor window failed: %s: %w", string(out), err)
	}
	return nil
}

// PositionWindow sets the frontmost window of the named app to the given bounds.
func (s *EditorService) PositionWindow(appName string, x, y, width, height int) error {
	if appName == "" || width <= 0 || height <= 0 {
		return nil
	}

	// Integer bounds are interpolated; app name is passed via argv to avoid string injection.
	script := fmt.Sprintf(`
on run argv
	set appName to item 1 of argv
	tell application "System Events"
		if exists process appName then
			tell process appName
				set position of front window to {%d, %d}
				set size of front window to {%d, %d}
			end tell
		end if
	end tell
end run
`, x, y, width, height)

	cmd := exec.Command("osascript", "-e", script, "--", appName) // #nosec G204 -- app name passed via argv, not interpolated
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("AppleScript position failed: %s: %w", string(out), err)
	}
	return nil
}

package backend

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	snapLeft          = "left"
	snapRight         = "right"
	snapDebounceDelay = 150 * time.Millisecond
	snapCooldown      = 500 * time.Millisecond
)

// EditorBounds represents the position and size for the editor window.
type EditorBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// SnapService handles window edge snapping and editor positioning.
type SnapService struct {
	mu       sync.RWMutex
	enabled  bool
	snapping bool   // guard flag to prevent re-entry from programmatic moves
	snapSide string // "left", "right", or "" (not snapped)
	window   *application.WebviewWindow
	debounce *time.Timer
}

// NewSnapService creates a new SnapService.
func NewSnapService() *SnapService {
	return &SnapService{enabled: true}
}

// SetWindow stores the window reference for screen queries.
func (s *SnapService) SetWindow(window *application.WebviewWindow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.window = window
}

// SetEnabled updates the snap preference from the frontend.
func (s *SnapService) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	if !enabled {
		s.snapSide = ""
	}
}

// GetSnapSide returns the current snap side ("left", "right", or "").
func (s *SnapService) GetSnapSide() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapSide
}

// OpenAccessibilitySettings opens System Settings to the Accessibility privacy pane.
func (s *SnapService) OpenAccessibilitySettings() {
	cmd := exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")
	_ = cmd.Run()
}

// getGroveWindowPosition returns the current position and size of the Grove window via AppleScript.
// Returns x, y, w, h or an error.
func getGroveWindowPosition() (int, int, int, int, error) { //nolint:unparam // height may be used later
	script := `
tell application "System Events"
	tell process "Grove"
		set {x, y} to position of front window
		set {w, h} to size of front window
		return "" & x & "," & y & "," & w & "," & h
	end tell
end tell
`
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) != 4 {
		return 0, 0, 0, 0, fmt.Errorf("unexpected output: %s", string(out))
	}

	vals := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, 0, 0, 0, err
		}
		vals[i] = v
	}
	return vals[0], vals[1], vals[2], vals[3], nil
}

// getScreenBoundsForWindow returns the visible frame of the screen containing the given point.
// Uses AppleScript + NSScreen to get correct coordinates on multi-monitor setups.
// Returns x, y, w, h (where y is in macOS screen coordinates, top of usable area below menu bar).
func getScreenBoundsForWindow(windowX, windowY int) (int, int, int, int, error) {
	// NSScreen uses bottom-left origin. We need to find the screen containing our point
	// and get its visibleFrame (which excludes menu bar and dock).
	script := fmt.Sprintf(`
use framework "AppKit"
set allScreens to current application's NSScreen's screens()
set wx to %d
set wy to %d

-- NSRect from frame() is {{originX, originY}, {sizeW, sizeH}}
-- Convert from top-left (AppleScript window coords) to bottom-left (NSScreen coords)
set pf to (item 1 of allScreens)'s frame()
set primaryHeight to item 2 of item 2 of pf
set nsY to primaryHeight - wy

set bestScreen to item 1 of allScreens
set bestDist to 999999
repeat with scr in allScreens
	set sf to scr's frame()
	set scrX to item 1 of item 1 of sf
	set scrY to item 2 of item 1 of sf
	set scrW to item 1 of item 2 of sf
	set scrH to item 2 of item 2 of sf
	if wx >= scrX and wx < (scrX + scrW) and nsY >= scrY and nsY < (scrY + scrH) then
		set bestScreen to scr
		exit repeat
	end if
	set dx to 0
	if wx < scrX then set dx to scrX - wx
	if wx >= (scrX + scrW) then set dx to wx - (scrX + scrW)
	set dy to 0
	if nsY < scrY then set dy to scrY - nsY
	if nsY >= (scrY + scrH) then set dy to nsY - (scrY + scrH)
	set dist to dx + dy
	if dist < bestDist then
		set bestDist to dist
		set bestScreen to scr
	end if
end repeat

set vf to bestScreen's visibleFrame()
set vfX to item 1 of item 1 of vf
set vfY to item 2 of item 1 of vf
set vfW to item 1 of item 2 of vf
set vfH to item 2 of item 2 of vf

set topY to primaryHeight - vfY - vfH

return "" & (vfX as integer) & "," & (topY as integer) & "," & (vfW as integer) & "," & (vfH as integer)
`, windowX, windowY)

	cmd := exec.Command("osascript", "-e", script) //nolint:gosec // coordinates are integers
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) != 4 {
		return 0, 0, 0, 0, fmt.Errorf("unexpected output: %s", string(out))
	}

	vals := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, 0, 0, 0, err
		}
		vals[i] = v
	}
	return vals[0], vals[1], vals[2], vals[3], nil
}

// setGroveWindowPosition sets the Grove window position and size via AppleScript.
func setGroveWindowPosition(x, y, w, h int) {
	script := fmt.Sprintf(`
tell application "System Events"
	tell process "Grove"
		set position of front window to {%d, %d}
		set size of front window to {%d, %d}
	end tell
end tell
`, x, y, w, h)
	cmd := exec.Command("osascript", "-e", script) //nolint:gosec // coordinates are integers
	_ = cmd.Run()
}

// snapToNearest snaps the window to the nearest screen edge at full height.
func (s *SnapService) snapToNearest() {
	s.mu.Lock()
	if s.snapping || !s.enabled {
		s.mu.Unlock()
		return
	}
	s.snapping = true
	if s.debounce != nil {
		s.debounce.Stop()
		s.debounce = nil
	}
	s.mu.Unlock()

	defer func() {
		time.AfterFunc(snapCooldown, func() {
			s.mu.Lock()
			s.snapping = false
			s.mu.Unlock()
		})
	}()

	// Get current window position via AppleScript (accurate on all screens)
	wx, wy, ww, _, err := getGroveWindowPosition()
	if err != nil {
		return
	}

	// Get the screen's visible area (excludes menu bar / dock)
	sx, sy, sw, sh, err := getScreenBoundsForWindow(wx, wy)
	if err != nil {
		return
	}

	// Snap to whichever edge the window center is closer to
	windowCenter := wx + ww/2
	screenCenter := sx + sw/2

	var side string
	var targetX int
	if windowCenter <= screenCenter {
		targetX = sx
		side = snapLeft
	} else {
		targetX = sx + sw - ww
		side = snapRight
	}

	setGroveWindowPosition(targetX, sy, ww, sh)

	s.mu.Lock()
	s.snapSide = side
	s.mu.Unlock()
}

// SnapNow forces an immediate snap to the nearest edge.
func (s *SnapService) SnapNow() {
	s.snapToNearest()
}

// HandleMove is called on window move events. Debounces then snaps to nearest edge.
func (s *SnapService) HandleMove(_ *application.WebviewWindow) {
	s.mu.Lock()
	if s.snapping || !s.enabled {
		s.mu.Unlock()
		return
	}
	if s.debounce != nil {
		s.debounce.Stop()
	}
	s.debounce = time.AfterFunc(snapDebounceDelay, s.snapToNearest)
	s.mu.Unlock()
}

// GetEditorBounds returns the bounds for the editor window to fill the opposite screen half.
// widthPercent (1-100) sizes the editor as a fraction of the remaining space.
// Values outside [1, 100] are clamped. Returns a zero EditorBounds if not snapped.
func (s *SnapService) GetEditorBounds(widthPercent int) EditorBounds {
	s.mu.RLock()
	side := s.snapSide
	s.mu.RUnlock()

	if side == "" {
		return EditorBounds{}
	}

	if widthPercent < 1 {
		widthPercent = 1
	} else if widthPercent > 100 {
		widthPercent = 100
	}

	// Get current window position and screen bounds via AppleScript
	wx, wy, ww, _, err := getGroveWindowPosition()
	if err != nil {
		return EditorBounds{}
	}

	sx, sy, sw, sh, err := getScreenBoundsForWindow(wx, wy)
	if err != nil {
		return EditorBounds{}
	}

	remaining := sw - ww
	editorW := remaining * widthPercent / 100

	switch side {
	case snapLeft:
		return EditorBounds{
			X:      sx + ww,
			Y:      sy,
			Width:  editorW,
			Height: sh,
		}
	case snapRight:
		groveLeft := sx + sw - ww
		return EditorBounds{
			X:      groveLeft - editorW,
			Y:      sy,
			Width:  editorW,
			Height: sh,
		}
	default:
		return EditorBounds{}
	}
}

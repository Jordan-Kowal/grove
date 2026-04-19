//go:build !darwin

package backend

// IsAccessibilityTrusted is a no-op stub for non-darwin builds.
// Grove only ships on macOS; this stub exists so cross-platform builds compile.
func IsAccessibilityTrusted() bool {
	return false
}

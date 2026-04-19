//go:build darwin

package backend

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>
*/
import "C"

// IsAccessibilityTrusted returns true if Grove has been granted macOS Accessibility permission.
// Non-prompting — never surfaces a native dialog.
func IsAccessibilityTrusted() bool {
	return C.AXIsProcessTrusted() != 0
}

package backend

import (
	"fmt"
	"regexp"
	"strings"
)

var validNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)
var validBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_/]*[a-zA-Z0-9\-_])?$`)

// validateName checks that a name is safe for use as a directory name.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid name %q", name)
	}
	if !validNamePattern.MatchString(name) {
		return fmt.Errorf("name %q contains invalid characters (only letters, numbers, hyphens, underscores)", name)
	}
	return nil
}

// validateBranchName checks that a name is valid as a git branch name.
// Allows slashes for namespaced branches (e.g. feature/my-branch).
func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid branch name %q", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name %q contains '..'", name)
	}
	if strings.Contains(name, "//") {
		return fmt.Errorf("branch name %q contains consecutive slashes", name)
	}
	if !validBranchPattern.MatchString(name) {
		return fmt.Errorf("branch name %q contains invalid characters (only letters, numbers, hyphens, underscores, slashes)", name)
	}
	return nil
}

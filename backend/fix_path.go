package backend

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

// FixPath resolves the user's full shell PATH and sets it on the current process.
// macOS GUI apps launched from Finder inherit a minimal PATH (/usr/bin:/bin:/usr/sbin:/sbin),
// missing Homebrew and other tools. This mirrors the "fix-path" npm package used in Electron.
//
// We use $SHELL (the user's login shell) so that shell-specific profile files are sourced
// (e.g. ~/.zprofile with brew shellenv). The inner /bin/sh ensures PATH is printed in
// colon-separated format even when $SHELL is fish (which uses space-separated $PATH).
func FixPath() {
	shell := resolveShell()

	// Launch the user's shell as login to source profile files (picks up Homebrew, nvm, etc.),
	// then hand off to /bin/sh to print PATH in guaranteed colon-separated format.
	out, err := exec.Command(shell, "-lc", `/bin/sh -c 'echo "$PATH"'`).Output() // #nosec G204
	if err != nil {
		log.Printf("grove: failed to resolve shell PATH: %v — git commands may fail", err)
		return
	}

	resolvedPath := strings.TrimSpace(string(out))
	if resolvedPath != "" {
		_ = os.Setenv("PATH", resolvedPath)
	}
}

// resolveShell returns the user's login shell, falling back to /bin/sh.
// Only well-known shell paths are allowed to avoid command injection via $SHELL.
func resolveShell() string {
	shell := os.Getenv("SHELL")
	switch shell {
	case "/bin/sh", "/bin/bash", "/bin/zsh", "/bin/fish",
		"/opt/homebrew/bin/bash", "/opt/homebrew/bin/zsh", "/opt/homebrew/bin/fish":
		return shell
	default:
		return "/bin/sh"
	}
}

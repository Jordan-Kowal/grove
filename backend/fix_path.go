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
// (e.g. ~/.zprofile with brew shellenv). The -i flag is required because shells like fish
// guard PATH setup behind `status is-interactive` checks. The inner /bin/sh ensures PATH
// is printed in colon-separated format even when $SHELL is fish (which uses space-separated $PATH).
//
// Security note: this function sources the user's shell dotfiles (`.zshrc`,
// `.bashrc`, `.config/fish/config.fish`, etc.) in an interactive login. A
// poisoned dotfile can therefore inject entries into PATH and shadow trusted
// binaries (e.g. a fake `git` earlier in PATH). Practical impact is low because
// any attacker with write access to those files already controls the user's
// shell, but it means Grove's subprocess trust boundary is bounded by the
// user's shell-init integrity rather than by the app itself.
func FixPath() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	out, err := exec.Command(shell, "-lic", `/bin/sh -c 'echo "$PATH"'`).Output() //nolint:gosec // $SHELL is set by the OS login system, not user input
	if err != nil {
		log.Printf("grove: failed to resolve shell PATH: %v — git commands may fail", err)
		return
	}

	resolvedPath := strings.TrimSpace(string(out))
	if resolvedPath != "" {
		_ = os.Setenv("PATH", resolvedPath)
	}
}

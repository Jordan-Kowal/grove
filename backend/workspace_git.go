package backend

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveGitDir returns the filesystem path for a git operation.
// For the main repo (worktreeName == MainWorktreeName), it returns config.RepoPath.
// For regular worktrees, it returns the worktree directory under groveDir.
func (s *WorkspaceService) resolveGitDir(workspaceName, worktreeName string) (string, error) {
	if worktreeName == MainWorktreeName {
		config := s.readConfig(workspaceName)
		if config.RepoPath == "" {
			return "", fmt.Errorf("workspace %q has no repo path configured", workspaceName)
		}
		return config.RepoPath, nil
	}
	if validateName(workspaceName) != nil || validateName(worktreeName) != nil {
		return "", fmt.Errorf("invalid name")
	}
	return filepath.Join(s.groveDir, workspaceName, "worktrees", worktreeName), nil
}

// fetchRemoteIfNeeded fetches the remote when ref looks like a remote tracking branch (e.g. "origin/main").
// When workspace/worktree are non-empty, the fetch output is streamed via worktree-log so the user can
// open the failure logs from the dashboard. Pass empty strings to skip log emission.
func (s *WorkspaceService) fetchRemoteIfNeeded(workspaceName, worktreeName, repoPath, ref string) error {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) < 2 {
		return nil // local branch, no fetch needed
	}
	remote := parts[0]
	// Verify it's an actual remote
	cmd := exec.Command("git", "-C", repoPath, "remote") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return nil //nolint:nilerr // intentional: if we can't list remotes, skip fetch gracefully
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == remote {
			return s.runGitCmdTracked(workspaceName, worktreeName, repoPath,
				fmt.Sprintf("fetch %s", remote),
				"fetch", remote)
		}
	}
	return nil // not a remote ref, skip
}

// ListBranches returns all local and remote branches for a workspace.
func (s *WorkspaceService) ListBranches(workspaceName string) ([]BranchInfo, error) {
	if err := validateName(workspaceName); err != nil {
		return nil, fmt.Errorf("invalid workspace name: %w", err)
	}
	config := s.readConfig(workspaceName)
	if config.RepoPath == "" {
		return nil, fmt.Errorf("workspace not found")
	}

	cmd := exec.Command("git", "-C", config.RepoPath, "branch", "-a", "--format=%(refname)") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %w", err)
	}

	var branches []BranchInfo
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" || strings.Contains(ref, "HEAD") {
			continue
		}
		isRemote := strings.HasPrefix(ref, "refs/remotes/")
		var name string
		if isRemote {
			name = strings.TrimPrefix(ref, "refs/remotes/")
		} else {
			name = strings.TrimPrefix(ref, "refs/heads/")
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		branches = append(branches, BranchInfo{
			Name:     name,
			IsRemote: isRemote,
		})
	}
	return branches, nil
}

// RebaseWorktree rebases the worktree's branch onto the given target branch.
func (s *WorkspaceService) RebaseWorktree(workspaceName, worktreeName, targetBranch string) {
	worktreePath, err := s.resolveGitDir(workspaceName, worktreeName)
	if err != nil {
		s.emitTask(workspaceName, worktreeName, StepRebase, StatusFailed, err.Error())
		return
	}
	go func() {
		s.emitTask(workspaceName, worktreeName, StepRebase, StatusInProgress, "")
		// Fetch remote so the target ref is up to date
		if err := s.fetchRemoteIfNeeded(workspaceName, worktreeName, worktreePath, targetBranch); err != nil {
			s.emitTask(workspaceName, worktreeName, StepRebase, StatusFailed, err.Error())
			return
		}
		if err := s.runGitCmdTracked(workspaceName, worktreeName, worktreePath,
			"rebase "+targetBranch, "rebase", targetBranch); err != nil {
			// Abort the failed rebase to leave the worktree in a clean state
			_ = exec.Command("git", "-C", worktreePath, "rebase", "--abort").Run() // #nosec G204
			s.emitTask(workspaceName, worktreeName, StepRebase, StatusFailed, err.Error())
			return
		}
		s.emitTask(workspaceName, worktreeName, StepRebase, StatusSuccess, "")
		s.refreshMonitor()
	}()
}

// CheckoutBranch checks out an existing branch in the worktree.
func (s *WorkspaceService) CheckoutBranch(workspaceName, worktreeName, branch string) {
	worktreePath, err := s.resolveGitDir(workspaceName, worktreeName)
	if err != nil {
		s.emitTask(workspaceName, worktreeName, StepCheckout, StatusFailed, err.Error())
		return
	}
	go func() {
		s.emitTask(workspaceName, worktreeName, StepCheckout, StatusInProgress, "")
		// Fetch remote so the branch ref is up to date
		if err := s.fetchRemoteIfNeeded(workspaceName, worktreeName, worktreePath, branch); err != nil {
			s.emitTask(workspaceName, worktreeName, StepCheckout, StatusFailed, err.Error())
			return
		}
		if err := s.runGitCmdTracked(workspaceName, worktreeName, worktreePath,
			"checkout "+branch, "checkout", branch); err != nil {
			s.emitTask(workspaceName, worktreeName, StepCheckout, StatusFailed, err.Error())
			return
		}
		s.emitTask(workspaceName, worktreeName, StepCheckout, StatusSuccess, "")
		s.refreshMonitor()
	}()
}

// NewBranchOnWorktree creates a fresh branch from baseBranch on the worktree.
func (s *WorkspaceService) NewBranchOnWorktree(workspaceName, worktreeName, branchName string) {
	if validateBranchName(branchName) != nil {
		s.emitTask(workspaceName, worktreeName, StepNewBranch, StatusFailed, "invalid branch name")
		return
	}
	worktreePath, err := s.resolveGitDir(workspaceName, worktreeName)
	if err != nil {
		s.emitTask(workspaceName, worktreeName, StepNewBranch, StatusFailed, err.Error())
		return
	}
	go func() {
		config := s.readConfig(workspaceName)
		baseBranch := config.BaseBranch
		if baseBranch == "" {
			baseBranch = "origin/main"
		}
		s.emitTask(workspaceName, worktreeName, StepNewBranch, StatusInProgress, "")
		// Fetch remote so the base branch ref is up to date
		if err := s.fetchRemoteIfNeeded(workspaceName, worktreeName, worktreePath, baseBranch); err != nil {
			s.emitTask(workspaceName, worktreeName, StepNewBranch, StatusFailed, err.Error())
			return
		}
		if err := s.runGitCmdTracked(workspaceName, worktreeName, worktreePath,
			fmt.Sprintf("checkout -b %s %s", branchName, baseBranch),
			"checkout", "-b", branchName, baseBranch); err != nil {
			s.emitTask(workspaceName, worktreeName, StepNewBranch, StatusFailed, err.Error())
			return
		}
		// Remove inherited upstream tracking so the first push uses -u
		_ = exec.Command("git", "-C", worktreePath, "branch", "--unset-upstream").Run() // #nosec G204
		s.emitTask(workspaceName, worktreeName, StepNewBranch, StatusSuccess, "")
		s.refreshMonitor()
	}()
}

// SyncMainCheckout resets the main checkout's working tree to match HEAD.
// This discards all local changes and removes untracked files in the repo root.
func (s *WorkspaceService) SyncMainCheckout(workspaceName string) error {
	if validateName(workspaceName) != nil {
		return fmt.Errorf("invalid workspace name")
	}
	config := s.readConfig(workspaceName)
	if config.RepoPath == "" {
		return fmt.Errorf("workspace not found")
	}
	if err := s.runGitCmdTracked("", "", config.RepoPath, "restore .", "restore", "."); err != nil {
		return err
	}
	return s.runGitCmdTracked("", "", config.RepoPath, "clean -fd", "clean", "-fd")
}

func (s *WorkspaceService) forceRemoveWorktree(workspaceName, worktreeName string) error {
	config := s.readConfig(workspaceName)
	worktreePath := filepath.Join(s.groveDir, workspaceName, "worktrees", worktreeName)

	if err := s.runGitCmdTracked(workspaceName, worktreeName, config.RepoPath,
		fmt.Sprintf("worktree remove --force %s", worktreePath),
		"worktree", "remove", "--force", worktreePath); err != nil {
		// Fallback: direct removal + prune stale worktree index entries
		s.emitLogLines(workspaceName, worktreeName, "Falling back to manual cleanup of "+worktreePath)
		if removeErr := os.RemoveAll(worktreePath); removeErr != nil {
			s.emitLogLines(workspaceName, worktreeName, "manual cleanup failed: "+removeErr.Error())
			return fmt.Errorf("git worktree remove failed (%s) and cleanup failed: %w", err.Error(), removeErr)
		}
		if pruneErr := exec.Command("git", "-C", config.RepoPath, "worktree", "prune").Run(); pruneErr != nil { // #nosec G204
			log.Printf("[grove] worktree prune failed after manual cleanup of %s/%s: %v", workspaceName, worktreeName, pruneErr)
		}
	}

	// Clean up the branch so the name can be reused next time
	if config.DeleteBranch {
		_ = exec.Command("git", "-C", config.RepoPath, "branch", "-D", worktreeName).Run() // #nosec G204
	}

	return nil
}

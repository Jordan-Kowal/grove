package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskStep identifies which operation is running.
type TaskStep string

const (
	StepGitWorktree    TaskStep = "git_worktree"
	StepSetupScript    TaskStep = "setup_script"
	StepTeardownScript TaskStep = "archive_script"
	StepGitRemove      TaskStep = "git_remove"
	StepRebase         TaskStep = "rebase"
	StepCheckout       TaskStep = "checkout"
	StepNewBranch      TaskStep = "new_branch"
)

// TaskStatus identifies the state of the current step.
type TaskStatus string

const (
	StatusInProgress TaskStatus = "in_progress"
	StatusSuccess    TaskStatus = "success"
	StatusFailed     TaskStatus = "failed"
)

// WorktreeTaskEvent is emitted during async worktree operations.
type WorktreeTaskEvent struct {
	WorkspaceName string     `json:"workspaceName"`
	WorktreeName  string     `json:"worktreeName"`
	Step          TaskStep   `json:"step"`
	Status        TaskStatus `json:"status"`
	Error         string     `json:"error,omitempty"`
}

const unknownBranch = "unknown"

// MainWorktreeName is the sentinel worktree name used for operations on the
// main repo checkout (as opposed to a git worktree). It is deliberately
// invalid per validateName so it can never collide with real worktree names.
const MainWorktreeName = "."

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

// WorkspaceService manages workspace and worktree CRUD operations.
type WorkspaceService struct {
	groveDir    string
	mu          sync.Mutex
	runningCmds map[string]*exec.Cmd // key: "workspace/worktree"
}

// NewWorkspaceService creates a new WorkspaceService.
func NewWorkspaceService() *WorkspaceService {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	return &WorkspaceService{
		groveDir:    filepath.Join(homeDir, ".grove", "projects"),
		runningCmds: make(map[string]*exec.Cmd),
	}
}

// GroveDir returns the base directory for workspace data.
func (s *WorkspaceService) GroveDir() string {
	return s.groveDir
}

// GetWorkspaces returns all registered workspaces with their worktree info.
func (s *WorkspaceService) GetWorkspaces() []Workspace {
	entries, err := os.ReadDir(s.groveDir)
	if err != nil {
		return nil
	}

	var workspaces []Workspace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		config := s.readConfig(name)
		if config.RepoPath == "" {
			continue
		}
		if _, err := os.Stat(config.RepoPath); err != nil {
			continue
		}
		ws := Workspace{
			Name:      name,
			Config:    config,
			Worktrees: s.scanWorktrees(name),
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces
}

// AddWorkspace registers a new git repository as a workspace.
func (s *WorkspaceService) AddWorkspace(repoPath string) (string, error) {
	if !filepath.IsAbs(repoPath) {
		return "", fmt.Errorf("repository path must be absolute: %s", repoPath)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return "", fmt.Errorf("not a git repository: %s", repoPath)
	}

	name := filepath.Base(repoPath)
	if err := validateName(name); err != nil {
		return "", fmt.Errorf("invalid workspace name: %w", err)
	}

	projectDir := filepath.Join(s.groveDir, name)
	if _, err := os.Stat(projectDir); err == nil {
		return "", fmt.Errorf("workspace %q already exists", name)
	}

	worktreesDir := filepath.Join(projectDir, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o750); err != nil { // #nosec G301
		return "", fmt.Errorf("failed to create workspace dir: %w", err)
	}

	config := WorkspaceConfig{RepoPath: repoPath, DeleteBranch: true}
	if err := s.writeConfig(name, config); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return name, nil
}

// RemoveWorkspace deletes a workspace after force-removing all its worktrees.
func (s *WorkspaceService) RemoveWorkspace(name string) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid workspace name: %w", err)
	}
	worktrees := s.scanWorktrees(name)
	for _, wt := range worktrees {
		if err := s.forceRemoveWorktree(name, wt.Name); err != nil {
			log.Printf("[grove] failed to remove worktree %s/%s: %v", name, wt.Name, err)
		}
	}
	projectDir := filepath.Join(s.groveDir, name)
	return os.RemoveAll(projectDir)
}

// CreateWorktree runs git worktree add + setup script, fully async.
func (s *WorkspaceService) CreateWorktree(workspaceName string, worktreeName string) {
	if err := validateName(workspaceName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed, err.Error())
		return
	}
	go s.createWorktreeAsync(workspaceName, worktreeName)
}

// RemoveWorktree runs archive script then removes worktree, async.
func (s *WorkspaceService) RemoveWorktree(workspaceName string, worktreeName string) {
	if !s.validatePair(workspaceName, worktreeName, StepTeardownScript) {
		return
	}
	go s.removeWorktreeAsync(workspaceName, worktreeName)
}

// ForceRemoveWorktree skips archive and force-deletes.
func (s *WorkspaceService) ForceRemoveWorktree(workspaceName string, worktreeName string) {
	if !s.validatePair(workspaceName, worktreeName, StepGitRemove) {
		return
	}
	go func() {
		s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusInProgress, "")
		if err := s.forceRemoveWorktree(workspaceName, worktreeName); err != nil {
			s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusFailed, err.Error())
			return
		}
		s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusSuccess, "")
		s.refreshMonitor()
	}()
}

// CancelTask stops a running script for a worktree.
func (s *WorkspaceService) CancelTask(workspaceName string, worktreeName string) {
	if validateName(workspaceName) != nil || validateName(worktreeName) != nil {
		return
	}
	key := workspaceName + "/" + worktreeName
	s.mu.Lock()
	cmd, ok := s.runningCmds[key]
	s.mu.Unlock()

	if ok && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// RetrySetup re-runs setup script on existing worktree.
func (s *WorkspaceService) RetrySetup(workspaceName string, worktreeName string) {
	if validateName(workspaceName) != nil || validateName(worktreeName) != nil {
		return
	}
	config := s.readConfig(workspaceName)
	if config.SetupScript == "" {
		return
	}
	worktreePath := filepath.Join(s.groveDir, workspaceName, "worktrees", worktreeName)
	go func() {
		s.emitTask(workspaceName, worktreeName, StepSetupScript, StatusInProgress, "")
		err := s.runScriptTracked(workspaceName, worktreeName, config.SetupScript, worktreePath)
		if err != nil {
			s.emitTask(workspaceName, worktreeName, StepSetupScript, StatusFailed, err.Error())
		} else {
			s.emitTask(workspaceName, worktreeName, StepSetupScript, StatusSuccess, "")
		}
	}()
}

// RetryArchive re-runs archive script then deletes.
func (s *WorkspaceService) RetryArchive(workspaceName string, worktreeName string) {
	if validateName(workspaceName) != nil || validateName(worktreeName) != nil {
		return
	}
	go s.removeWorktreeAsync(workspaceName, worktreeName)
}

// GetWorkspaceConfig returns the config for a workspace.
func (s *WorkspaceService) GetWorkspaceConfig(name string) WorkspaceConfig {
	if err := validateName(name); err != nil {
		return WorkspaceConfig{}
	}
	return s.readConfig(name)
}

// UpdateWorkspaceConfig updates the config for a workspace.
func (s *WorkspaceService) UpdateWorkspaceConfig(name string, config WorkspaceConfig) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid workspace name: %w", err)
	}
	return s.writeConfig(name, config)
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

// OpenFolderDialog opens a native folder picker.
func (s *WorkspaceService) OpenFolderDialog() string {
	path, _ := application.Get().Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		SetTitle("Select Git Repository").
		PromptForSingleSelection()
	return path
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

// --- Async creation flow ---

func (s *WorkspaceService) createWorktreeAsync(workspaceName, worktreeName string) {
	if err := validateName(worktreeName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed, err.Error())
		return
	}
	config := s.readConfig(workspaceName)
	if config.RepoPath == "" {
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed, "workspace not found")
		return
	}

	worktreePath := filepath.Join(s.groveDir, workspaceName, "worktrees", worktreeName)

	// Fail early if the directory already exists — never destroy existing work
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed,
			fmt.Sprintf("directory %q already exists", worktreePath))
		return
	}

	// Step 1: git worktree add
	s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusInProgress, "")

	baseBranch := config.BaseBranch
	if baseBranch == "" {
		baseBranch = "origin/main"
	}

	// Fetch remote so the base branch ref is up to date
	if err := s.fetchRemoteIfNeeded(workspaceName, worktreeName, config.RepoPath, baseBranch); err != nil {
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed, err.Error())
		return
	}

	// Try creating a new branch; if it already exists, reuse it.
	// We run the first attempt directly here (rather than via runGitCmdTracked)
	// so we can detect the "already exists" case before surfacing it as failure.
	key := workspaceName + "/" + worktreeName
	addArgs := []string{"-C", config.RepoPath, "worktree", "add", "-b", worktreeName, worktreePath, baseBranch}
	addLabel := fmt.Sprintf("worktree add -b %s %s %s", worktreeName, worktreePath, baseBranch)
	cmd := exec.Command("git", addArgs...) // #nosec G204
	s.mu.Lock()
	s.runningCmds[key] = cmd
	s.mu.Unlock()

	s.emitLogLines(workspaceName, worktreeName, "$ git "+addLabel)
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimRight(string(out), "\n")
	if trimmed != "" {
		s.emitLogLines(workspaceName, worktreeName, strings.Split(trimmed, "\n")...)
	}

	if err != nil && strings.Contains(string(out), "already exists") {
		// Branch exists — reuse it (directory cannot exist thanks to the early check above)
		s.emitLogLines(workspaceName, worktreeName,
			fmt.Sprintf("Branch %q already exists, reusing it.", worktreeName))
		retryArgs := []string{"-C", config.RepoPath, "worktree", "add", worktreePath, worktreeName}
		retryLabel := fmt.Sprintf("worktree add %s %s", worktreePath, worktreeName)
		cmd = exec.Command("git", retryArgs...) // #nosec G204
		s.mu.Lock()
		s.runningCmds[key] = cmd
		s.mu.Unlock()
		s.emitLogLines(workspaceName, worktreeName, "$ git "+retryLabel)
		out, err = cmd.CombinedOutput()
		trimmed = strings.TrimRight(string(out), "\n")
		if trimmed != "" {
			s.emitLogLines(workspaceName, worktreeName, strings.Split(trimmed, "\n")...)
		}
	}

	s.mu.Lock()
	delete(s.runningCmds, key)
	s.mu.Unlock()

	if err != nil {
		_ = os.RemoveAll(worktreePath)
		errMsg := trimmed
		if errMsg == "" {
			errMsg = err.Error()
		}
		s.emitLogLines(workspaceName, worktreeName, "git worktree add failed: "+errMsg)
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed, errMsg)
		return
	}

	// Step 2: setup script (if configured) — go directly from git to setup, no intermediate event
	if config.SetupScript != "" {
		s.emitTask(workspaceName, worktreeName, StepSetupScript, StatusInProgress, "")
		s.refreshMonitor() // card now has real git data
		scriptErr := s.runScriptTracked(workspaceName, worktreeName, config.SetupScript, worktreePath)
		if scriptErr != nil {
			s.emitTask(workspaceName, worktreeName, StepSetupScript, StatusFailed, scriptErr.Error())
		} else {
			s.emitTask(workspaceName, worktreeName, StepSetupScript, StatusSuccess, "")
		}
	} else {
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusSuccess, "")
		s.refreshMonitor()
	}
}

// --- Async deletion flow ---

func (s *WorkspaceService) removeWorktreeAsync(workspaceName, worktreeName string) {
	config := s.readConfig(workspaceName)
	worktreePath := filepath.Join(s.groveDir, workspaceName, "worktrees", worktreeName)

	// Step 1: archive script (if configured)
	if config.TeardownScript != "" {
		s.emitTask(workspaceName, worktreeName, StepTeardownScript, StatusInProgress, "")
		err := s.runScriptTracked(workspaceName, worktreeName, config.TeardownScript, worktreePath)
		if err != nil {
			s.emitTask(workspaceName, worktreeName, StepTeardownScript, StatusFailed, err.Error())
			return
		}
		s.emitTask(workspaceName, worktreeName, StepTeardownScript, StatusSuccess, "")
	}

	// Step 2: git worktree remove
	s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusInProgress, "")
	if err := s.forceRemoveWorktree(workspaceName, worktreeName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusFailed, err.Error())
		return
	}
	s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusSuccess, "")
	s.refreshMonitor()
}

// --- Helpers ---

const scriptTimeout = 10 * time.Minute

// WorktreeLogEvent is emitted during script execution with batched log lines.
type WorktreeLogEvent struct {
	WorkspaceName string   `json:"workspaceName"`
	WorktreeName  string   `json:"worktreeName"`
	Lines         []string `json:"lines"`
	Timestamp     int64    `json:"timestamp"` // Unix milliseconds
}

// emitLogLines emits a worktree-log event with the given lines. Skipped when
// either workspace or worktree name is empty (callers that don't have a target).
func (s *WorkspaceService) emitLogLines(workspaceName, worktreeName string, lines ...string) {
	if workspaceName == "" || worktreeName == "" || len(lines) == 0 {
		return
	}
	application.Get().Event.Emit("worktree-log", WorktreeLogEvent{
		WorkspaceName: workspaceName,
		WorktreeName:  worktreeName,
		Lines:         lines,
		Timestamp:     time.Now().UnixMilli(),
	})
}

// runGitCmdTracked runs a git command in repoDir, captures combined output,
// and on failure emits the output as worktree-log so the user can open it
// from the dashboard. label is a short human description of the command
// (e.g. "rebase origin/main") used for the header line.
func (s *WorkspaceService) runGitCmdTracked(workspaceName, worktreeName, repoDir, label string, args ...string) error {
	s.emitLogLines(workspaceName, worktreeName, "$ git "+label)
	gitArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", gitArgs...) // #nosec G204
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimRight(string(out), "\n")
	if trimmed != "" {
		s.emitLogLines(workspaceName, worktreeName, strings.Split(trimmed, "\n")...)
	}
	if err != nil {
		s.emitLogLines(workspaceName, worktreeName, fmt.Sprintf("git %s failed: %s", label, err.Error()))
		if trimmed != "" {
			return fmt.Errorf("git %s failed: %s", label, trimmed)
		}
		return fmt.Errorf("git %s failed: %w", label, err)
	}
	return nil
}

func (s *WorkspaceService) runScriptTracked(workspaceName, worktreeName, script, workDir string) error {
	key := workspaceName + "/" + worktreeName

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script) // #nosec G204
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"GROVE_WORKTREE_PATH="+workDir,
		"GROVE_WORKTREE_NAME="+filepath.Base(workDir),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	s.mu.Lock()
	s.runningCmds[key] = cmd
	s.mu.Unlock()

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		delete(s.runningCmds, key)
		s.mu.Unlock()
		return err
	}

	// Emit a synthetic first log line immediately so the button appears instantly
	application.Get().Event.Emit("worktree-log", WorktreeLogEvent{
		WorkspaceName: workspaceName,
		WorktreeName:  worktreeName,
		Lines:         []string{"Running script..."},
		Timestamp:     time.Now().UnixMilli(),
	})

	// Stream output lines via batched events
	var pending []string
	var pendingMu sync.Mutex

	flush := func() {
		pendingMu.Lock()
		if len(pending) == 0 {
			pendingMu.Unlock()
			return
		}
		batch := pending
		pending = nil
		pendingMu.Unlock()
		application.Get().Event.Emit("worktree-log", WorktreeLogEvent{
			WorkspaceName: workspaceName,
			WorktreeName:  worktreeName,
			Lines:         batch,
			Timestamp:     time.Now().UnixMilli(),
		})
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	stopTicker := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopTicker:
				return
			case <-ticker.C:
				flush()
			}
		}
	}()

	var wg sync.WaitGroup
	scanPipe := func(pipe io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			pendingMu.Lock()
			pending = append(pending, line)
			pendingMu.Unlock()
		}
	}

	wg.Add(2)
	go scanPipe(stdout)
	go scanPipe(stderr)
	wg.Wait()

	cmdErr := cmd.Wait()

	ticker.Stop()
	close(stopTicker)
	flush() // final flush

	s.mu.Lock()
	delete(s.runningCmds, key)
	s.mu.Unlock()

	if cmdErr != nil {
		log.Printf("[grove] script failed in %s: %s", workDir, cmdErr.Error())
	}

	return cmdErr
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

func (s *WorkspaceService) emitTask(workspaceName, worktreeName string, step TaskStep, status TaskStatus, errMsg string) {
	application.Get().Event.Emit("worktree-task", WorktreeTaskEvent{
		WorkspaceName: workspaceName,
		WorktreeName:  worktreeName,
		Step:          step,
		Status:        status,
		Error:         errMsg,
	})
}

func (s *WorkspaceService) refreshMonitor() {
	application.Get().Event.Emit("refresh-requested", nil)
}

// validatePair validates workspace and worktree names. On failure, it emits a
// failed task event tagged with `step` and returns false so the caller can
// return early. Use when a public API needs both names to be well-formed.
func (s *WorkspaceService) validatePair(workspaceName, worktreeName string, step TaskStep) bool {
	if err := validateName(workspaceName); err != nil {
		s.emitTask(workspaceName, worktreeName, step, StatusFailed, err.Error())
		return false
	}
	if err := validateName(worktreeName); err != nil {
		s.emitTask(workspaceName, worktreeName, step, StatusFailed, err.Error())
		return false
	}
	return true
}

func (s *WorkspaceService) configPath(name string) string {
	return filepath.Join(s.groveDir, name, "config.json")
}

func (s *WorkspaceService) readConfig(name string) WorkspaceConfig {
	data, err := os.ReadFile(s.configPath(name)) // #nosec G304
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("grove: failed to read config for %s: %v", name, err)
		}
		return WorkspaceConfig{}
	}
	var config WorkspaceConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("grove: failed to parse config for %s: %v", name, err)
		return WorkspaceConfig{}
	}
	return config
}

func (s *WorkspaceService) writeConfig(name string, config WorkspaceConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath(name), data, 0o600) // #nosec G306
}

func (s *WorkspaceService) scanWorktrees(workspaceName string) []WorktreeInfo {
	worktreesDir := filepath.Join(s.groveDir, workspaceName, "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return nil
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(worktreesDir, entry.Name())
		if _, err := os.Stat(filepath.Join(dirPath, ".git")); err != nil {
			continue
		}
		wt := WorktreeInfo{
			Name:         entry.Name(),
			Path:         dirPath,
			ClaudeStatus: ClaudeStatusIdle,
		}
		wt.Branch = getGitBranch(dirPath)
		wt.FilesChanged, wt.Insertions, wt.Deletions = getGitDiffStats(dirPath, nil, nil)
		worktrees = append(worktrees, wt)
	}
	return worktrees
}

// scanWorktreeStructure returns worktrees with name/path only (no git commands).
// Branch and diff data are populated separately by refreshGit.
func (s *WorkspaceService) scanWorktreeStructure(workspaceName string) []WorktreeInfo {
	worktreesDir := filepath.Join(s.groveDir, workspaceName, "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return nil
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(worktreesDir, entry.Name())
		if _, err := os.Stat(filepath.Join(dirPath, ".git")); err != nil {
			continue
		}
		worktrees = append(worktrees, WorktreeInfo{
			Name:         entry.Name(),
			Path:         dirPath,
			ClaudeStatus: ClaudeStatusIdle,
		})
	}
	return worktrees
}

// getGitBranch reads the current branch from the filesystem (no process spawn).
// In a worktree, .git is a file containing "gitdir: <path>". We follow that to find HEAD.
func getGitBranch(dir string) string {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return unknownBranch
	}

	var headPath string
	if info.IsDir() {
		// Regular repo: .git/HEAD
		headPath = filepath.Join(gitPath, "HEAD")
	} else {
		// Worktree: .git is a file with "gitdir: <path>"
		data, err := os.ReadFile(gitPath) // #nosec G304
		if err != nil {
			return unknownBranch
		}
		gitdir := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir:"))
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(dir, gitdir)
		}
		headPath = filepath.Join(gitdir, "HEAD")
	}

	headPath = filepath.Clean(headPath)
	head, err := os.ReadFile(headPath) // #nosec G304 G703
	if err != nil {
		return unknownBranch
	}
	ref := strings.TrimSpace(string(head))
	// HEAD contains "ref: refs/heads/<branch>" for normal branches
	if strings.HasPrefix(ref, "ref: refs/heads/") {
		return strings.TrimPrefix(ref, "ref: refs/heads/")
	}
	// Detached HEAD — return short hash
	if len(ref) >= 7 {
		return ref[:7]
	}
	return unknownBranch
}

// getGitDiffStats returns combined diff statistics for a directory: tracked changes
// from `git diff HEAD --shortstat` plus untracked (non-ignored) files counted as insertions.
// Each git subprocess is bounded by pollGitTimeout so a hung git can't freeze the poller.
// Pass a non-nil cache to memoize line counts across calls, and a non-nil seen map to
// participate in mark-and-sweep cleanup; pass nil for one-shot callers (initial scans, tests).
func getGitDiffStats(dir string, cache *untrackedCache, seen map[string]struct{}) (files, insertions, deletions int) {
	ctx, cancel := context.WithTimeout(context.Background(), pollGitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "--no-optional-locks", "diff", "HEAD", "--shortstat") // #nosec G204
	out, err := cmd.Output()
	if err == nil {
		files, insertions, deletions = parseGitDiffStat(string(out))
	}
	uf, uins := getUntrackedStats(ctx, dir, cache, seen)
	files += uf
	insertions += uins
	return files, insertions, deletions
}

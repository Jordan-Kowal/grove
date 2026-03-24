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
	StepGitWorktree   TaskStep = "git_worktree"
	StepSetupScript   TaskStep = "setup_script"
	StepArchiveScript TaskStep = "archive_script"
	StepGitRemove     TaskStep = "git_remove"
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

var validNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

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
	if err := validateName(workspaceName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepArchiveScript, StatusFailed, err.Error())
		return
	}
	if err := validateName(worktreeName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepArchiveScript, StatusFailed, err.Error())
		return
	}
	go s.removeWorktreeAsync(workspaceName, worktreeName)
}

// ForceRemoveWorktree skips archive and force-deletes.
func (s *WorkspaceService) ForceRemoveWorktree(workspaceName string, worktreeName string) {
	if err := validateName(workspaceName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusFailed, err.Error())
		return
	}
	if err := validateName(worktreeName); err != nil {
		s.emitTask(workspaceName, worktreeName, StepGitRemove, StatusFailed, err.Error())
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

// OpenFolderDialog opens a native folder picker.
func (s *WorkspaceService) OpenFolderDialog() string {
	path, _ := application.Get().Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		SetTitle("Select Git Repository").
		PromptForSingleSelection()
	return path
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

	// Step 1: git worktree add
	s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusInProgress, "")

	baseBranch := config.BaseBranch
	if baseBranch == "" {
		baseBranch = "origin/main"
	}

	// Try creating a new branch; if it already exists, reuse it
	cmd := exec.Command("git", "-C", config.RepoPath, "worktree", "add", "-b", worktreeName, worktreePath, baseBranch) // #nosec G204

	// Track command so it can be cancelled
	key := workspaceName + "/" + worktreeName
	s.mu.Lock()
	s.runningCmds[key] = cmd
	s.mu.Unlock()

	out, err := cmd.CombinedOutput()

	if err != nil && strings.Contains(string(out), "already exists") {
		// Branch exists — reuse it
		_ = os.RemoveAll(worktreePath)
		cmd = exec.Command("git", "-C", config.RepoPath, "worktree", "add", worktreePath, worktreeName) // #nosec G204
		s.mu.Lock()
		s.runningCmds[key] = cmd
		s.mu.Unlock()
		out, err = cmd.CombinedOutput()
	}

	s.mu.Lock()
	delete(s.runningCmds, key)
	s.mu.Unlock()

	if err != nil {
		_ = os.RemoveAll(worktreePath)
		s.emitTask(workspaceName, worktreeName, StepGitWorktree, StatusFailed, strings.TrimSpace(string(out)))
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
	if config.ArchiveScript != "" {
		s.emitTask(workspaceName, worktreeName, StepArchiveScript, StatusInProgress, "")
		err := s.runScriptTracked(workspaceName, worktreeName, config.ArchiveScript, worktreePath)
		if err != nil {
			s.emitTask(workspaceName, worktreeName, StepArchiveScript, StatusFailed, err.Error())
			return
		}
		s.emitTask(workspaceName, worktreeName, StepArchiveScript, StatusSuccess, "")
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

	cmd := exec.Command("git", "-C", config.RepoPath, "worktree", "remove", "--force", worktreePath) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback: direct removal + prune stale worktree index entries
		if removeErr := os.RemoveAll(worktreePath); removeErr != nil {
			return fmt.Errorf("git worktree remove failed (%s) and cleanup failed: %w", strings.TrimSpace(string(out)), removeErr)
		}
		_ = exec.Command("git", "-C", config.RepoPath, "worktree", "prune").Run() // #nosec G204
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
		wt.FilesChanged, wt.Insertions, wt.Deletions = getGitDiffStats(dirPath)
		worktrees = append(worktrees, wt)
	}
	return worktrees
}

// scanWorktreeStructure returns worktrees with name/path/branch only (no git diff).
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
			Branch:       getGitBranch(dirPath),
			ClaudeStatus: ClaudeStatusIdle,
		})
	}
	return worktrees
}

// getGitBranch returns the current branch for a directory.
func getGitBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// getGitDiffStats returns diff statistics for a directory.
func getGitDiffStats(dir string) (files, insertions, deletions int) {
	cmd := exec.Command("git", "-C", dir, "--no-optional-locks", "diff", "--shortstat") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}
	return parseGitDiffStat(string(out))
}

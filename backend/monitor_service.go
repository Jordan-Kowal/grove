package backend

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	claudePollInterval = 2 * time.Second
	gitPollInterval    = 10 * time.Second
)

// MonitorService polls workspace/worktree status and emits events on changes.
type MonitorService struct {
	workspaceSvc *WorkspaceService
	soundSvc     *SoundService
	traySvc      *TrayService
	groveDir     string
	mu           sync.RWMutex
	workspaces   []Workspace
	stopCh       chan struct{}
	stopOnce     sync.Once
	prevStatuses map[string]ClaudeStatus // track previous status per worktree path
	doneTimers   map[string]*time.Timer  // per-path timers for done → idle transition
	gitBusy      sync.Mutex              // prevents overlapping git diff scans
}

// NewMonitorService creates a new MonitorService.
func NewMonitorService(workspaceSvc *WorkspaceService, soundSvc *SoundService, traySvc *TrayService) *MonitorService {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	return &MonitorService{
		workspaceSvc: workspaceSvc,
		soundSvc:     soundSvc,
		traySvc:      traySvc,
		groveDir:     filepath.Join(homeDir, ".grove", "sessions"),
		stopCh:       make(chan struct{}),
		prevStatuses: make(map[string]ClaudeStatus),
		doneTimers:   make(map[string]*time.Timer),
	}
}

// ServiceStartup installs the hook script and starts background polling.
func (s *MonitorService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.installHook()
	application.Get().Event.On("refresh-requested", func(_ *application.CustomEvent) {
		s.RefreshNow()
	})
	// Populate workspaces before launching pollers so pollGit sees the worktree list.
	s.refreshWorkspaces()
	go s.pollClaude()
	go s.pollGit()
	return nil
}

const hookScript = `#!/bin/sh
# Grove status hook — writes Claude session state to ~/.grove/sessions/
# Usage: hook.sh <state>  (working|permission|question|idle)
# Called by Claude Code hooks. Uses PPID (the Claude Code process) as the stable identifier.
mkdir -p "$HOME/.grove/sessions"
escaped_cwd=$(printf '%s' "$PWD" | sed 's/\\/\\\\/g; s/"/\\"/g')
printf '{"state":"%s","cwd":"%s","pid":%d}\n' "$1" "$escaped_cwd" "$PPID" > "$HOME/.grove/sessions/$PPID.json"
`

// installHook ensures ~/.grove/sessions/ exists, ~/.grove/hook.sh is up to date,
// and Claude Code settings.json has the required hook entries.
func (s *MonitorService) installHook() {
	homeDir, _ := os.UserHomeDir()
	groveDir := filepath.Join(homeDir, ".grove")
	sessionsDir := filepath.Join(groveDir, "sessions")
	hookPath := filepath.Join(groveDir, "hook.sh")

	if err := os.MkdirAll(sessionsDir, 0o750); err != nil { // #nosec G301 -- user-only dir
		log.Printf("grove: failed to create sessions dir: %v", err)
		return
	}
	if err := os.WriteFile(hookPath, []byte(hookScript), 0o700); err != nil { // #nosec G306 -- needs execute permission
		log.Printf("grove: failed to write hook script: %v", err)
	}

	ensureClaudeSettings(hookPath)
}

// ServiceShutdown stops the background goroutines.
func (s *MonitorService) ServiceShutdown() error {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.mu.Lock()
	for path, timer := range s.doneTimers {
		timer.Stop()
		delete(s.doneTimers, path)
	}
	s.mu.Unlock()
	return nil
}

// GetWorkspaces returns the current workspace list with status.
func (s *MonitorService) GetWorkspaces() []Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Workspace, len(s.workspaces))
	for i, ws := range s.workspaces {
		ws.Worktrees = append([]WorktreeInfo(nil), ws.Worktrees...)
		result[i] = ws
	}
	return result
}

// RefreshNow triggers an immediate refresh and event emission.
func (s *MonitorService) RefreshNow() {
	s.refreshWorkspaces()
	s.refreshGit()
	s.refreshClaude()
	application.Get().Event.Emit("workspaces-updated", s.GetWorkspaces())
}

// --- Background polling ---

// pollClaude polls Claude session status every 2s and detects new/removed worktrees.
func (s *MonitorService) pollClaude() {
	s.refreshWorkspaces()
	s.refreshClaude()
	ticker := time.NewTicker(claudePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			prev := s.GetWorkspaces()
			s.refreshWorkspaces()
			s.refreshClaude()
			if curr := s.GetWorkspaces(); !reflect.DeepEqual(prev, curr) {
				application.Get().Event.Emit("workspaces-updated", curr)
			}
		}
	}
}

// pollGit runs git diff/status on all worktrees every 10s.
// Skips the tick if the previous scan is still running (slow monorepo guard).
func (s *MonitorService) pollGit() {
	s.refreshGit()
	ticker := time.NewTicker(gitPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if !s.gitBusy.TryLock() {
				continue // previous scan still running, skip
			}
			prev := s.GetWorkspaces()
			s.refreshGit()
			s.gitBusy.Unlock()
			if curr := s.GetWorkspaces(); !reflect.DeepEqual(prev, curr) {
				application.Get().Event.Emit("workspaces-updated", curr)
			}
		}
	}
}

// --- Workspace structure refresh (no git commands) ---

// refreshWorkspaces scans the filesystem for workspaces and worktrees.
// Updates the cached workspace list structure without running git diff/status.
func (s *MonitorService) refreshWorkspaces() {
	entries, err := os.ReadDir(s.workspaceSvc.GroveDir())
	if err != nil {
		return
	}

	var workspaces []Workspace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		config := s.workspaceSvc.readConfig(name)
		if config.RepoPath == "" {
			continue
		}
		if _, err := os.Stat(config.RepoPath); err != nil {
			continue
		}
		worktrees := s.workspaceSvc.scanWorktreeStructure(name)
		workspaces = append(workspaces, Workspace{
			Name:      name,
			Config:    config,
			Worktrees: worktrees,
		})
	}

	// Preserve git diff data from previous state
	s.mu.Lock()
	prevByPath := make(map[string]WorktreeInfo)
	for _, ws := range s.workspaces {
		for _, wt := range ws.Worktrees {
			prevByPath[wt.Path] = wt
		}
	}
	for i := range workspaces {
		for j := range workspaces[i].Worktrees {
			wt := &workspaces[i].Worktrees[j]
			if old, ok := prevByPath[wt.Path]; ok {
				wt.Branch = old.Branch
				wt.FilesChanged = old.FilesChanged
				wt.Insertions = old.Insertions
				wt.Deletions = old.Deletions
				wt.ClaudeStatus = old.ClaudeStatus
			}
		}
	}
	s.workspaces = workspaces
	s.mu.Unlock()
}

// --- Git diff refresh ---

// refreshGit updates branch names and git diff stats on all cached worktrees.
// Branch names are read from the filesystem (no process spawn).
// Diff stats are fetched concurrently via git commands.
func (s *MonitorService) refreshGit() {
	s.mu.RLock()
	var paths []string
	for _, ws := range s.workspaces {
		for _, wt := range ws.Worktrees {
			paths = append(paths, wt.Path)
		}
	}
	s.mu.RUnlock()

	type gitData struct {
		branch           string
		files, ins, dels int
	}
	results := make([]gitData, len(paths))

	// Diff stats: concurrent git commands (the expensive part)
	var wg sync.WaitGroup
	wg.Add(len(paths))
	for i, p := range paths {
		go func(idx int, dir string) {
			defer wg.Done()
			f, ins, d := getGitDiffStats(dir)
			results[idx] = gitData{
				branch: getGitBranch(dir),
				files:  f, ins: ins, dels: d,
			}
		}(i, p)
	}
	wg.Wait()

	// Build map from results
	dataByPath := make(map[string]gitData, len(paths))
	for i, p := range paths {
		dataByPath[p] = results[i]
	}

	// Apply results under lock
	s.mu.Lock()
	for i := range s.workspaces {
		for j := range s.workspaces[i].Worktrees {
			wt := &s.workspaces[i].Worktrees[j]
			if data, ok := dataByPath[wt.Path]; ok {
				wt.Branch = data.branch
				wt.FilesChanged = data.files
				wt.Insertions = data.ins
				wt.Deletions = data.dels
			}
		}
	}
	s.mu.Unlock()
}

// --- Claude status detection via ~/.grove/sessions/ ---

type groveSession struct {
	State string `json:"state"`
	CWD   string `json:"cwd"`
	PID   int    `json:"pid"`
}

func (s *MonitorService) refreshClaude() {
	sessions := s.readGroveSessions()

	// Build map: worktree path → highest priority status
	statusByPath := make(map[string]ClaudeStatus)
	for _, sess := range sessions {
		newStatus := groveStateToClaudeStatus(sess.State)
		if existing, ok := statusByPath[sess.CWD]; ok {
			if claudeStatusPriority(newStatus) > claudeStatusPriority(existing) {
				statusByPath[sess.CWD] = newStatus
			}
		} else {
			statusByPath[sess.CWD] = newStatus
		}
	}

	s.mu.Lock()
	doneTransition := false
	attentionTransition := false
	needsAttention := false
	for i := range s.workspaces {
		for j := range s.workspaces[i].Worktrees {
			wt := &s.workspaces[i].Worktrees[j]
			newStatus := ClaudeStatusIdle
			if status, ok := statusByPath[wt.Path]; ok {
				newStatus = status
			}
			prev := s.prevStatuses[wt.Path]

			// Cancel any existing done→idle timer when a new real status arrives
			if timer, ok := s.doneTimers[wt.Path]; ok && newStatus != ClaudeStatusIdle {
				timer.Stop()
				delete(s.doneTimers, wt.Path)
			}

			// Detect working → non-working transition: show "done" for 10s
			if prev == ClaudeStatusWorking && newStatus != ClaudeStatusWorking {
				doneTransition = true
				if newStatus == ClaudeStatusIdle {
					newStatus = ClaudeStatusDone
					s.scheduleDoneExpiry(wt.Path)
				}
			}

			// If this path is currently "done" and the new status is idle, keep showing done
			if prev == ClaudeStatusDone && newStatus == ClaudeStatusIdle {
				if _, hasTimer := s.doneTimers[wt.Path]; hasTimer {
					newStatus = ClaudeStatusDone
				}
			}

			if (newStatus == ClaudeStatusPermission || newStatus == ClaudeStatusQuestion) &&
				prev != ClaudeStatusPermission && prev != ClaudeStatusQuestion {
				attentionTransition = true
			}
			if newStatus == ClaudeStatusPermission || newStatus == ClaudeStatusQuestion {
				needsAttention = true
			}
			s.prevStatuses[wt.Path] = newStatus
			wt.ClaudeStatus = newStatus
		}
	}
	s.mu.Unlock()

	if attentionTransition {
		s.soundSvc.PlayIfNeeded(true)
	} else if doneTransition {
		s.soundSvc.PlayIfNeeded(false)
	}

	if needsAttention {
		s.traySvc.SetBadge()
	} else {
		s.traySvc.RemoveBadge()
	}
}

// DismissDone immediately transitions a worktree from "done" to "idle",
// cancelling any pending done-expiry timer. No-op if the path is not in "done" state.
func (s *MonitorService) DismissDone(path string) {
	s.mu.Lock()
	if timer, ok := s.doneTimers[path]; ok {
		timer.Stop()
		delete(s.doneTimers, path)
	}
	if s.prevStatuses[path] != ClaudeStatusDone {
		s.mu.Unlock()
		return
	}
	s.prevStatuses[path] = ClaudeStatusIdle
	for i := range s.workspaces {
		for j := range s.workspaces[i].Worktrees {
			if s.workspaces[i].Worktrees[j].Path == path {
				s.workspaces[i].Worktrees[j].ClaudeStatus = ClaudeStatusIdle
			}
		}
	}
	s.mu.Unlock()
	application.Get().Event.Emit("workspaces-updated", s.GetWorkspaces())
}

const doneDuration = 30 * time.Minute

// scheduleDoneExpiry starts a timer that transitions a worktree from "done" to "idle"
// after doneDuration and emits a workspaces-updated event. Must be called with s.mu held.
func (s *MonitorService) scheduleDoneExpiry(path string) {
	if timer, ok := s.doneTimers[path]; ok {
		timer.Stop()
	}
	s.doneTimers[path] = time.AfterFunc(doneDuration, func() {
		s.mu.Lock()
		delete(s.doneTimers, path)
		if s.prevStatuses[path] == ClaudeStatusDone {
			s.prevStatuses[path] = ClaudeStatusIdle
			for i := range s.workspaces {
				for j := range s.workspaces[i].Worktrees {
					if s.workspaces[i].Worktrees[j].Path == path {
						s.workspaces[i].Worktrees[j].ClaudeStatus = ClaudeStatusIdle
					}
				}
			}
		}
		s.mu.Unlock()
		application.Get().Event.Emit("workspaces-updated", s.GetWorkspaces())
	})
}

func (s *MonitorService) readGroveSessions() []groveSession {
	entries, err := os.ReadDir(s.groveDir)
	if err != nil {
		return nil
	}

	var sessions []groveSession
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.groveDir, entry.Name())
		data, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			continue
		}

		var sess groveSession
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}

		// Remove session file if Claude process is dead.
		if !isProcessAlive(sess.PID) {
			_ = os.Remove(path)
			continue
		}

		sessions = append(sessions, sess)
	}
	return sessions
}

func groveStateToClaudeStatus(state string) ClaudeStatus {
	switch state {
	case "working":
		return ClaudeStatusWorking
	case "permission":
		return ClaudeStatusPermission
	case "question":
		return ClaudeStatusQuestion
	case "idle":
		return ClaudeStatusIdle
	default:
		return ClaudeStatusIdle
	}
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

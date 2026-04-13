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

// soundPlayer is the subset of SoundService used by MonitorService.
type soundPlayer interface {
	PlayIfNeeded(isPermission bool)
}

// trayBadger is the subset of TrayService used by MonitorService.
type trayBadger interface {
	SetBadge()
	RemoveBadge()
}

// MonitorService polls workspace/worktree status and emits events on changes.
type MonitorService struct {
	workspaceSvc   *WorkspaceService
	soundSvc       soundPlayer
	traySvc        trayBadger
	groveDir       string
	mu             sync.RWMutex
	workspaces     []Workspace
	stopCh         chan struct{}
	stopOnce       sync.Once
	bootTime       time.Time               // app start time — done before this is treated as idle
	dismissTimes   map[string]time.Time    // last card click per worktree path
	prevAggregated map[string]ClaudeStatus // track previous aggregated status per worktree path
	gitBusy        sync.Mutex              // prevents overlapping git diff scans
	readSessions   func() []groveSession   // injectable for testing; defaults to readGroveSessions
}

// NewMonitorService creates a new MonitorService.
func NewMonitorService(workspaceSvc *WorkspaceService, soundSvc *SoundService, traySvc *TrayService) *MonitorService {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	svc := &MonitorService{
		workspaceSvc:   workspaceSvc,
		soundSvc:       soundSvc,
		traySvc:        traySvc,
		groveDir:       filepath.Join(homeDir, ".grove", "sessions"),
		stopCh:         make(chan struct{}),
		bootTime:       time.Now(),
		dismissTimes:   make(map[string]time.Time),
		prevAggregated: make(map[string]ClaudeStatus),
	}
	svc.readSessions = svc.readGroveSessions
	return svc
}

// ServiceStartup installs the hook script and starts background polling.
func (s *MonitorService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.installHook()
	application.Get().Event.On("refresh-requested", func(_ *application.CustomEvent) {
		s.RefreshNow()
	})
	// Fully populate workspaces (structure + git + claude) before launching pollers
	// so the frontend's initial GetWorkspaces() call returns complete data.
	s.refreshWorkspaces()
	s.refreshGit()
	s.refreshClaude()
	go s.pollClaude()
	go s.pollGit()
	return nil
}

const hookScript = `#!/bin/sh
# Grove status hook — writes Claude session state to ~/.grove/sessions/
# Usage: hook.sh <state>  (working|permission|question|done)
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
	State   string    `json:"state"`
	CWD     string    `json:"cwd"`
	PID     int       `json:"pid"`
	ModTime time.Time // file modification time (not serialized)
}

func (s *MonitorService) refreshClaude() {
	sessions := s.readSessions()
	now := time.Now()

	// Collect all known worktree paths for subdirectory matching.
	s.mu.RLock()
	var worktreePaths []string
	for _, ws := range s.workspaces {
		for _, wt := range ws.Worktrees {
			worktreePaths = append(worktreePaths, wt.Path)
		}
	}
	s.mu.RUnlock()

	// Derive effective status per session.
	// "done" is downgraded to "idle" if: before boot, after dismiss, or expired (>30min).
	type sessionResult struct {
		path   string
		status ClaudeStatus
	}
	results := make([]sessionResult, 0, len(sessions))

	s.mu.RLock()
	for _, sess := range sessions {
		resolvedPath := resolveWorktreePath(sess.CWD, worktreePaths)
		status := groveStateToClaudeStatus(sess.State)

		if status == ClaudeStatusDone {
			dismissTime := s.dismissTimes[resolvedPath]
			if sess.ModTime.Before(s.bootTime) ||
				dismissTime.After(sess.ModTime) ||
				now.Sub(sess.ModTime) > doneDuration {
				status = ClaudeStatusIdle
			}
		}

		results = append(results, sessionResult{path: resolvedPath, status: status})
	}
	s.mu.RUnlock()

	// Aggregate per-worktree using priority: Blocked > Done > Working > Idle.
	statusByPath := make(map[string]ClaudeStatus)
	for _, r := range results {
		if existing, ok := statusByPath[r.path]; ok {
			if claudeStatusPriority(r.status) > claudeStatusPriority(existing) {
				statusByPath[r.path] = r.status
			}
		} else {
			statusByPath[r.path] = r.status
		}
	}

	// Apply aggregated status to worktrees and detect transitions for sounds/tray.
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
			prevAgg := s.prevAggregated[wt.Path]

			if newStatus == ClaudeStatusDone && prevAgg != ClaudeStatusDone {
				doneTransition = true
			}
			if (newStatus == ClaudeStatusPermission || newStatus == ClaudeStatusQuestion) &&
				prevAgg != ClaudeStatusPermission && prevAgg != ClaudeStatusQuestion {
				attentionTransition = true
			}
			if newStatus == ClaudeStatusPermission || newStatus == ClaudeStatusQuestion {
				needsAttention = true
			}
			s.prevAggregated[wt.Path] = newStatus
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

const doneDuration = 30 * time.Minute

// DismissDone records a dismiss click for the given worktree path,
// causing all "done" sessions in that path to be treated as "idle".
func (s *MonitorService) DismissDone(path string) {
	s.mu.Lock()
	s.dismissTimes[path] = time.Now()
	s.mu.Unlock()
	s.refreshClaude()
	application.Get().Event.Emit("workspaces-updated", s.GetWorkspaces())
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

		filePath := filepath.Join(s.groveDir, entry.Name())
		data, err := os.ReadFile(filePath) // #nosec G304
		if err != nil {
			continue
		}

		var sess groveSession
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}

		// Remove session file if Claude process is dead.
		if !isProcessAlive(sess.PID) {
			_ = os.Remove(filePath)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		sess.ModTime = info.ModTime()

		sessions = append(sessions, sess)
	}
	return sessions
}

// resolveWorktreePath returns the matching worktree path if cwd is equal to
// or a subdirectory of a known worktree. Falls back to cwd itself.
func resolveWorktreePath(cwd string, worktreePaths []string) string {
	for _, wtPath := range worktreePaths {
		if cwd == wtPath || strings.HasPrefix(cwd, wtPath+string(filepath.Separator)) {
			return wtPath
		}
	}
	return cwd
}

func groveStateToClaudeStatus(state string) ClaudeStatus {
	switch state {
	case "working":
		return ClaudeStatusWorking
	case "permission":
		return ClaudeStatusPermission
	case "question":
		return ClaudeStatusQuestion
	case "done":
		return ClaudeStatusDone
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

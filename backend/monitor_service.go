package backend

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	claudePollInterval = 2 * time.Second
	editorPollInterval = 5 * time.Second
	gitPollInterval    = 10 * time.Second
	livenessCheckEvery = 10 * time.Second
	refreshDebounce    = 200 * time.Millisecond
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
	workspaceSvc      *WorkspaceService
	editorSvc         *EditorService
	soundSvc          soundPlayer
	traySvc           trayBadger
	groveDir          string
	editorApp         string // cached editor app name for window detection
	editorTracking    bool   // when false, skip editor window polling and clear EditorOpen flags
	mu                sync.RWMutex
	workspaces        []Workspace
	stopCh            chan struct{}
	stopOnce          sync.Once
	bootTime          time.Time               // app start time — done before this is treated as idle
	dismissTimes      map[string]time.Time    // last card click per worktree path
	prevAggregated    map[string]ClaudeStatus // track previous aggregated status per worktree path
	doneDuration      time.Duration           // how long "done" persists; 0 = instant, <0 = forever
	gitBusy           sync.Mutex              // prevents overlapping git diff scans
	untrackedLines    *untrackedCache         // memoizes untracked-file line counts by (path, mtime, size)
	readSessions      func() []groveSession   // injectable for testing; defaults to readGroveSessions
	stateVersion      uint64                  // bumped on any observable state change; protected by mu
	lastLivenessCheck time.Time               // last time session PIDs were probed
	refreshTimer      *time.Timer             // trailing-edge debounce for RefreshNow; protected by refreshMu
	refreshMu         sync.Mutex              // guards refreshTimer
}

// NewMonitorService creates a new MonitorService.
func NewMonitorService(workspaceSvc *WorkspaceService, editorSvc *EditorService, soundSvc *SoundService, traySvc *TrayService) *MonitorService {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	svc := &MonitorService{
		workspaceSvc:   workspaceSvc,
		editorSvc:      editorSvc,
		soundSvc:       soundSvc,
		traySvc:        traySvc,
		groveDir:       filepath.Join(homeDir, ".grove", "sessions"),
		stopCh:         make(chan struct{}),
		bootTime:       time.Now(),
		dismissTimes:   make(map[string]time.Time),
		prevAggregated: make(map[string]ClaudeStatus),
		doneDuration:   30 * time.Minute,
		untrackedLines: newUntrackedCache(),
		editorTracking: true,
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
	// Fully populate workspaces (structure + git + claude + editor) before launching pollers
	// so the frontend's initial GetWorkspaces() call returns complete data.
	s.refreshWorkspaces()
	s.refreshGit()
	s.refreshClaude()
	s.refreshEditorOpen()
	go s.pollClaude()
	go s.pollEditor()
	go s.pollGit()
	return nil
}

// ServiceShutdown stops the background goroutines.
func (s *MonitorService) ServiceShutdown() error {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.refreshMu.Lock()
	if s.refreshTimer != nil {
		s.refreshTimer.Stop()
	}
	s.refreshMu.Unlock()
	return nil
}

// Snapshot returns the current workspace list with status. It is a read-only
// clone of the MonitorService's cache — distinct from WorkspaceService.GetWorkspaces,
// which scans the filesystem.
func (s *MonitorService) Snapshot() []Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Workspace, len(s.workspaces))
	for i, ws := range s.workspaces {
		ws.Worktrees = append([]WorktreeInfo(nil), ws.Worktrees...)
		result[i] = ws
	}
	return result
}

// RefreshNow coalesces rapid mutations (e.g. a burst of CloseEditorWindow
// calls) into a single trailing refresh. The refresh runs refreshDebounce
// after the last call, sparing the git fan-out from thrash.
func (s *MonitorService) RefreshNow() {
	s.refreshMu.Lock()
	if s.refreshTimer != nil {
		s.refreshTimer.Stop()
	}
	s.refreshTimer = time.AfterFunc(refreshDebounce, s.refreshNowImmediate)
	s.refreshMu.Unlock()
}

// refreshNowImmediate runs the full refresh suite without debounce.
// Exposed only so the debounce timer can fire it.
func (s *MonitorService) refreshNowImmediate() {
	s.refreshWorkspaces()
	s.refreshGit()
	s.refreshClaude()
	s.refreshEditorOpen()
	application.Get().Event.Emit("workspaces-updated", s.Snapshot())
}

// --- Background polling ---

// bumpVersion increments the state version. Caller must hold s.mu (write lock).
func (s *MonitorService) bumpVersion() {
	s.stateVersion++
}

// emitIfChanged emits "workspaces-updated" only when state version changed since prev.
// Returns the current version for the caller to track.
func (s *MonitorService) emitIfChanged(prev uint64) uint64 {
	s.mu.RLock()
	curr := s.stateVersion
	s.mu.RUnlock()
	if curr == prev {
		return curr
	}
	application.Get().Event.Emit("workspaces-updated", s.Snapshot())
	return curr
}

// pollClaude polls Claude session status every 2s and detects new/removed worktrees.
func (s *MonitorService) pollClaude() {
	ticker := time.NewTicker(claudePollInterval)
	defer ticker.Stop()
	var prev uint64
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.refreshWorkspaces()
			s.refreshClaude()
			prev = s.emitIfChanged(prev)
		}
	}
}

// pollEditor queries the editor for open windows on a slower cadence than pollClaude.
// Editor state rarely changes between ticks, and osascript spawns are expensive.
func (s *MonitorService) pollEditor() {
	ticker := time.NewTicker(editorPollInterval)
	defer ticker.Stop()
	var prev uint64
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.refreshEditorOpen()
			prev = s.emitIfChanged(prev)
		}
	}
}

// pollGit runs git diff/status on all worktrees every 10s.
// refreshGit acquires s.gitBusy itself, so overlapping scans are serialized
// across all entry points (pollGit + RefreshNow + startup).
func (s *MonitorService) pollGit() {
	ticker := time.NewTicker(gitPollInterval)
	defer ticker.Stop()
	var prev uint64
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.refreshGit()
			prev = s.emitIfChanged(prev)
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
			Name:         name,
			Config:       config,
			MainWorktree: WorktreeInfo{Name: MainWorktreeName, Path: config.RepoPath},
			Worktrees:    worktrees,
		})
	}

	// Preserve git diff data from previous state
	s.mu.Lock()
	prevByPath := make(map[string]WorktreeInfo)
	for _, ws := range s.workspaces {
		prevByPath[ws.MainWorktree.Path] = ws.MainWorktree
		for _, wt := range ws.Worktrees {
			prevByPath[wt.Path] = wt
		}
	}
	restorePrev := func(wt *WorktreeInfo) {
		if old, ok := prevByPath[wt.Path]; ok {
			wt.Branch = old.Branch
			wt.FilesChanged = old.FilesChanged
			wt.Insertions = old.Insertions
			wt.Deletions = old.Deletions
			wt.ClaudeStatus = old.ClaudeStatus
			wt.ClaudeSessionCounts = old.ClaudeSessionCounts
			wt.EditorOpen = old.EditorOpen
		}
	}
	for i := range workspaces {
		restorePrev(&workspaces[i].MainWorktree)
		for j := range workspaces[i].Worktrees {
			restorePrev(&workspaces[i].Worktrees[j])
		}
	}
	if workspaceStructureChanged(s.workspaces, workspaces) {
		s.bumpVersion()
	}
	s.workspaces = workspaces

	// Evict per-worktree state for paths that no longer exist. dismissTimes +
	// prevAggregated are keyed by worktree path and would otherwise grow
	// unbounded over a long-lived session as worktrees are created and removed.
	currentPaths := make(map[string]struct{}, len(workspaces)*4)
	for _, ws := range workspaces {
		currentPaths[ws.MainWorktree.Path] = struct{}{}
		for _, wt := range ws.Worktrees {
			currentPaths[wt.Path] = struct{}{}
		}
	}
	for p := range s.dismissTimes {
		if _, ok := currentPaths[p]; !ok {
			delete(s.dismissTimes, p)
		}
	}
	for p := range s.prevAggregated {
		if _, ok := currentPaths[p]; !ok {
			delete(s.prevAggregated, p)
		}
	}
	s.mu.Unlock()
}

// workspaceStructureChanged returns true if the set of workspace/worktree paths differs
// between old and new. Does not compare fields that other refresh methods own
// (Branch, FilesChanged, ClaudeStatus, EditorOpen) — those bump version themselves.
func workspaceStructureChanged(oldWs, newWs []Workspace) bool {
	if len(oldWs) != len(newWs) {
		return true
	}
	for i := range oldWs {
		if oldWs[i].Name != newWs[i].Name ||
			oldWs[i].MainWorktree.Path != newWs[i].MainWorktree.Path ||
			len(oldWs[i].Worktrees) != len(newWs[i].Worktrees) {
			return true
		}
		for j := range oldWs[i].Worktrees {
			if oldWs[i].Worktrees[j].Path != newWs[i].Worktrees[j].Path ||
				oldWs[i].Worktrees[j].Name != newWs[i].Worktrees[j].Name {
				return true
			}
		}
	}
	return false
}

// --- Git diff refresh ---

// gitRefreshConcurrency caps how many `git` subprocesses run in parallel
// during a single refreshGit pass. Prevents CPU + index-lock thrash when a
// workspace has many worktrees.
const gitRefreshConcurrency = 8

// refreshGit updates branch names and git diff stats on all cached worktrees.
// Branch names are read from the filesystem (no process spawn).
// Diff stats are fetched concurrently via git commands, capped at
// gitRefreshConcurrency to bound CPU load. s.gitBusy serializes overlapping
// calls so two scans never race on s.workspaces.
func (s *MonitorService) refreshGit() {
	s.gitBusy.Lock()
	defer s.gitBusy.Unlock()

	// Snapshot paths under RLock, then release so refreshWorkspaces can still
	// mutate s.workspaces while git commands run. s.workspaces may be rebuilt
	// between this read and the applyGit reacquire below, but applyGit is
	// keyed by path — a worktree that disappears simply drops its update,
	// and a new worktree gets no data this tick (picked up next refreshGit).
	s.mu.RLock()
	var paths []string
	for _, ws := range s.workspaces {
		paths = append(paths, ws.MainWorktree.Path)
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

	// seen collects every untracked file path visited across all worktrees in this
	// pass. After the scan we sweep the cache so deleted files are evicted.
	// Guarded by seenMu because goroutines write concurrently.
	seen := make(map[string]struct{})
	var seenMu sync.Mutex

	// Bounded concurrency: at most gitRefreshConcurrency git subprocesses at once.
	// Acquire the slot on the dispatching goroutine so we also throttle goroutine
	// creation, not just the git subprocess inside each worker.
	sem := make(chan struct{}, gitRefreshConcurrency)
	var wg sync.WaitGroup
	wg.Add(len(paths))
	for i, p := range paths {
		sem <- struct{}{}
		go func(idx int, dir string) {
			defer wg.Done()
			defer func() { <-sem }()
			local := make(map[string]struct{})
			f, ins, d := getGitDiffStats(dir, s.untrackedLines, local)
			results[idx] = gitData{
				branch: getGitBranch(dir),
				files:  f, ins: ins, dels: d,
			}
			seenMu.Lock()
			for k := range local {
				seen[k] = struct{}{}
			}
			seenMu.Unlock()
		}(i, p)
	}
	wg.Wait()

	s.untrackedLines.sweepUnseen(seen)

	// Build map from results
	dataByPath := make(map[string]gitData, len(paths))
	for i, p := range paths {
		dataByPath[p] = results[i]
	}

	changed := false
	applyGit := func(wt *WorktreeInfo) {
		data, ok := dataByPath[wt.Path]
		if !ok {
			return
		}
		if wt.Branch != data.branch ||
			wt.FilesChanged != data.files ||
			wt.Insertions != data.ins ||
			wt.Deletions != data.dels {
			changed = true
		}
		wt.Branch = data.branch
		wt.FilesChanged = data.files
		wt.Insertions = data.ins
		wt.Deletions = data.dels
	}

	// Apply results under lock
	s.mu.Lock()
	for i := range s.workspaces {
		applyGit(&s.workspaces[i].MainWorktree)
		for j := range s.workspaces[i].Worktrees {
			applyGit(&s.workspaces[i].Worktrees[j])
		}
	}
	if changed {
		s.bumpVersion()
	}
	s.mu.Unlock()
}

// SetDoneDuration configures how long "done" persists.
// 0 = instant dismiss, negative = persist until clicked.
func (s *MonitorService) SetDoneDuration(minutes int) {
	s.mu.Lock()
	if minutes < 0 {
		s.doneDuration = -1
	} else {
		s.doneDuration = time.Duration(minutes) * time.Minute
	}
	s.mu.Unlock()
}

// SetEditorApp updates the cached editor app name used for window detection.
func (s *MonitorService) SetEditorApp(appName string) {
	s.mu.Lock()
	s.editorApp = appName
	s.mu.Unlock()
}

// SetEditorTrackingEnabled toggles polling for open editor windows. When
// disabled, refreshEditorOpen short-circuits and any cached EditorOpen flags
// are cleared so stale "active" badges do not linger on the dashboard.
func (s *MonitorService) SetEditorTrackingEnabled(enabled bool) {
	changed, toggled := s.applyEditorTracking(enabled)
	if !toggled {
		return
	}
	if changed {
		application.Get().Event.Emit("workspaces-updated", s.Snapshot())
	}
	if enabled {
		s.refreshEditorOpen()
		application.Get().Event.Emit("workspaces-updated", s.Snapshot())
	}
}

// applyEditorTracking updates the editorTracking flag and, when disabling,
// clears cached EditorOpen flags. Returns (changed, toggled): changed=true
// if any EditorOpen flag flipped, toggled=true if the tracking flag itself
// changed. Split out from SetEditorTrackingEnabled for testability — emits
// stay in the public method so tests can exercise state transitions without
// a Wails application instance.
func (s *MonitorService) applyEditorTracking(enabled bool) (changed, toggled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.editorTracking == enabled {
		return false, false
	}
	s.editorTracking = enabled
	if enabled {
		return false, true
	}
	clearEditor := func(wt *WorktreeInfo) {
		if wt.EditorOpen {
			wt.EditorOpen = false
			changed = true
		}
	}
	for i := range s.workspaces {
		clearEditor(&s.workspaces[i].MainWorktree)
		for j := range s.workspaces[i].Worktrees {
			clearEditor(&s.workspaces[i].Worktrees[j])
		}
	}
	if changed {
		s.bumpVersion()
	}
	return changed, true
}

// refreshEditorOpen queries the editor for open windows and marks matching worktrees.
func (s *MonitorService) refreshEditorOpen() {
	s.mu.RLock()
	appName := s.editorApp
	tracking := s.editorTracking
	var allPaths []string
	for _, ws := range s.workspaces {
		allPaths = append(allPaths, ws.MainWorktree.Path)
		for _, wt := range ws.Worktrees {
			allPaths = append(allPaths, wt.Path)
		}
	}
	s.mu.RUnlock()

	if !tracking || appName == "" {
		return
	}

	titles := s.editorSvc.GetOpenEditorPaths(appName)
	openSet := s.editorSvc.MatchOpenPaths(titles, allPaths)

	s.mu.Lock()
	changed := false
	applyEditor := func(wt *WorktreeInfo) {
		newOpen := openSet[wt.Path]
		if wt.EditorOpen != newOpen {
			changed = true
			wt.EditorOpen = newOpen
		}
	}
	for i := range s.workspaces {
		applyEditor(&s.workspaces[i].MainWorktree)
		for j := range s.workspaces[i].Worktrees {
			applyEditor(&s.workspaces[i].Worktrees[j])
		}
	}
	if changed {
		s.bumpVersion()
	}
	s.mu.Unlock()
}

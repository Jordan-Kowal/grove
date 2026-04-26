package backend

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// hookScript is a defensive-only allowlist: Grove itself always passes one of
// the four known states, but the script lives in $HOME and is referenced from
// user-editable ~/.claude/settings.json. If a future refactor starts shelling
// out with $1, or a user hand-edits the settings file to pass an unusual
// value, the allowlist keeps garbage out of the session JSON.
const hookScript = `#!/bin/sh
# Grove status hook — writes Claude session state to ~/.grove/sessions/
# Usage: hook.sh <state>  (working|permission|question|done)
# Called by Claude Code hooks. Uses PPID (the Claude Code process) as the stable identifier.
case "$1" in
  working|permission|question|done) ;;
  *) exit 1 ;;
esac
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

	if err := os.MkdirAll(sessionsDir, 0o700); err != nil { // #nosec G301 -- user-only dir
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

	// Collect all known paths (worktrees + main repos) for subdirectory matching.
	// Worktree paths are listed first so they match before the parent main repo.
	s.mu.RLock()
	var worktreePaths []string
	for _, ws := range s.workspaces {
		for _, wt := range ws.Worktrees {
			worktreePaths = append(worktreePaths, wt.Path)
		}
		worktreePaths = append(worktreePaths, ws.MainWorktree.Path)
	}
	s.mu.RUnlock()

	// Derive effective status per session.
	// "done" is downgraded to "idle" if: before boot, after dismiss, or expired.
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
			expired := s.doneDuration >= 0 && now.Sub(sess.ModTime) > s.doneDuration
			if sess.ModTime.Before(s.bootTime) ||
				dismissTime.After(sess.ModTime) ||
				expired {
				status = ClaudeStatusIdle
			}
		}

		results = append(results, sessionResult{path: resolvedPath, status: status})
	}
	s.mu.RUnlock()

	// Aggregate per-worktree: highest-priority status + per-status session counts.
	statusByPath := make(map[string]ClaudeStatus)
	countsByPath := make(map[string]map[ClaudeStatus]int)
	for _, r := range results {
		if existing, ok := statusByPath[r.path]; ok {
			if claudeStatusPriority(r.status) > claudeStatusPriority(existing) {
				statusByPath[r.path] = r.status
			}
		} else {
			statusByPath[r.path] = r.status
		}
		if countsByPath[r.path] == nil {
			countsByPath[r.path] = make(map[ClaudeStatus]int)
		}
		countsByPath[r.path][r.status]++
	}

	// Apply aggregated status to worktrees (and main worktree) and detect transitions for sounds/tray.
	s.mu.Lock()
	doneTransition := false
	attentionTransition := false
	needsAttention := false
	changed := false

	applyStatus := func(wt *WorktreeInfo) {
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
		newCounts := countsByPath[wt.Path]
		if wt.ClaudeStatus != newStatus || !sessionCountsEqual(wt.ClaudeSessionCounts, newCounts) {
			changed = true
		}
		s.prevAggregated[wt.Path] = newStatus
		wt.ClaudeStatus = newStatus
		wt.ClaudeSessionCounts = newCounts
	}

	for i := range s.workspaces {
		applyStatus(&s.workspaces[i].MainWorktree)
		for j := range s.workspaces[i].Worktrees {
			applyStatus(&s.workspaces[i].Worktrees[j])
		}
	}
	if changed {
		s.bumpVersion()
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

// sessionCountsEqual compares two ClaudeSessionCounts maps for equality.
func sessionCountsEqual(a, b map[ClaudeStatus]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// DismissDone records a dismiss click for the given worktree path,
// causing all "done" sessions in that path to be treated as "idle".
func (s *MonitorService) DismissDone(path string) {
	s.mu.Lock()
	s.dismissTimes[path] = time.Now()
	s.mu.Unlock()
	s.refreshClaude()
	application.Get().Event.Emit("workspaces-updated", s.Snapshot())
}

func (s *MonitorService) readGroveSessions() []groveSession {
	entries, err := os.ReadDir(s.groveDir)
	if err != nil {
		return nil
	}

	// Liveness probe (kill syscall) is staggered: runs at most once per livenessCheckEvery
	// window. Between probes, session files are trusted based on mtime. Dead PIDs linger
	// up to livenessCheckEvery seconds — cosmetic only.
	s.mu.Lock()
	checkLiveness := time.Since(s.lastLivenessCheck) >= livenessCheckEvery
	if checkLiveness {
		s.lastLivenessCheck = time.Now()
	}
	s.mu.Unlock()

	var sessions []groveSession
	var toRemove []string
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

		if checkLiveness && !isProcessAlive(sess.PID) {
			toRemove = append(toRemove, filePath)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		sess.ModTime = info.ModTime()

		sessions = append(sessions, sess)
	}

	// Cleanup off the hot path — readers never block on os.Remove.
	if len(toRemove) > 0 {
		go func(paths []string) {
			for _, p := range paths {
				_ = os.Remove(p)
			}
		}(toRemove)
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

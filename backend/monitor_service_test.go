package backend

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Test doubles for MonitorService dependencies ---

type mockSound struct {
	calls []bool // records isPermission arg for each PlayIfNeeded call
}

func (m *mockSound) PlayIfNeeded(isPermission bool) {
	m.calls = append(m.calls, isPermission)
}

type mockTray struct {
	badge bool
}

func (m *mockTray) SetBadge()    { m.badge = true }
func (m *mockTray) RemoveBadge() { m.badge = false }

// newTestMonitor creates a MonitorService wired with mocks and the given workspaces.
func newTestMonitor(workspaces []Workspace, sessions func() []groveSession) (*MonitorService, *mockSound, *mockTray) {
	sound := &mockSound{}
	tray := &mockTray{}
	svc := &MonitorService{
		soundSvc:       sound,
		traySvc:        tray,
		bootTime:       time.Now().Add(-time.Hour), // boot was 1h ago so "done" isn't pre-boot
		dismissTimes:   make(map[string]time.Time),
		prevAggregated: make(map[string]ClaudeStatus),
		workspaces:     workspaces,
		doneDuration:   30 * time.Minute,
		stopCh:         make(chan struct{}),
		readSessions:   sessions,
	}
	return svc, sound, tray
}

// --- refreshClaude state machine tests ---

func TestRefreshClaude_DoneSession_SetsStatusAndPlaysSound(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: time.Now()}}
	}
	svc, sound, tray := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusDone {
		t.Errorf("status = %q, want %q", status, ClaudeStatusDone)
	}
	if len(sound.calls) != 1 || sound.calls[0] != false {
		t.Errorf("sound calls = %v, want [false] (done sound)", sound.calls)
	}
	if tray.badge {
		t.Error("tray badge should not be set for done status")
	}
}

func TestRefreshClaude_PermissionSession_SetsBadgeAndPlaysSound(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "permission", CWD: "/wt", ModTime: time.Now()}}
	}
	svc, sound, tray := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusPermission {
		t.Errorf("status = %q, want %q", status, ClaudeStatusPermission)
	}
	if len(sound.calls) != 1 || sound.calls[0] != true {
		t.Errorf("sound calls = %v, want [true] (permission sound)", sound.calls)
	}
	if !tray.badge {
		t.Error("tray badge should be set for permission status")
	}
}

func TestRefreshClaude_DoneDowngrade_BeforeBoot(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	bootTime := time.Now()
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: bootTime.Add(-time.Minute)}}
	}
	svc, sound, _ := newTestMonitor(ws, sessions)
	svc.bootTime = bootTime

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusIdle {
		t.Errorf("done before boot should be idle, got %q", status)
	}
	if len(sound.calls) != 0 {
		t.Errorf("no sound expected for downgraded done, got %v", sound.calls)
	}
}

func TestRefreshClaude_DoneDowngrade_AfterDismiss(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessionTime := time.Now().Add(-5 * time.Minute)
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: sessionTime}}
	}
	svc, sound, _ := newTestMonitor(ws, sessions)
	// Dismiss happened after the session's ModTime
	svc.dismissTimes["/wt"] = sessionTime.Add(time.Minute)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusIdle {
		t.Errorf("done after dismiss should be idle, got %q", status)
	}
	if len(sound.calls) != 0 {
		t.Errorf("no sound expected for dismissed done, got %v", sound.calls)
	}
}

func TestRefreshClaude_DoneDowngrade_Expired(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: time.Now().Add(-31 * time.Minute)}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusIdle {
		t.Errorf("expired done should be idle, got %q", status)
	}
}

func TestRefreshClaude_Aggregation_HighestPriorityWins(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		now := time.Now()
		return []groveSession{
			{State: "working", CWD: "/wt", ModTime: now},
			{State: "permission", CWD: "/wt", ModTime: now},
			{State: "idle", CWD: "/wt", ModTime: now},
		}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusPermission {
		t.Errorf("aggregated status = %q, want %q (highest priority)", status, ClaudeStatusPermission)
	}
}

func TestRefreshClaude_SessionCounts_PopulatedPerStatus(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		now := time.Now()
		return []groveSession{
			{State: "working", CWD: "/wt", ModTime: now},
			{State: "working", CWD: "/wt", ModTime: now},
			{State: "permission", CWD: "/wt", ModTime: now},
		}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	counts := svc.workspaces[0].Worktrees[0].ClaudeSessionCounts
	svc.mu.RUnlock()

	if counts[ClaudeStatusWorking] != 2 {
		t.Errorf("working count = %d, want 2", counts[ClaudeStatusWorking])
	}
	if counts[ClaudeStatusPermission] != 1 {
		t.Errorf("permission count = %d, want 1", counts[ClaudeStatusPermission])
	}
	if counts[ClaudeStatusIdle] != 0 {
		t.Errorf("idle count = %d, want 0", counts[ClaudeStatusIdle])
	}
}

func TestRefreshClaude_SessionCounts_NilWhenNoSessions(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession { return nil }
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	counts := svc.workspaces[0].Worktrees[0].ClaudeSessionCounts
	svc.mu.RUnlock()

	if counts != nil {
		t.Errorf("expected nil session counts when no sessions, got %v", counts)
	}
}

func TestRefreshClaude_NoTransitionSound_OnRepeatStatus(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: time.Now()}}
	}
	svc, sound, _ := newTestMonitor(ws, sessions)

	// First refresh: transition idle→done — plays sound
	svc.refreshClaude()
	// Second refresh: done→done — no new sound
	svc.refreshClaude()

	if len(sound.calls) != 1 {
		t.Errorf("sound should play once on transition, got %d calls", len(sound.calls))
	}
}

func TestRefreshClaude_AttentionTakesPriority_OverDoneSound(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{
		{Path: "/wt1"},
		{Path: "/wt2"},
	}}}
	sessions := func() []groveSession {
		now := time.Now()
		return []groveSession{
			{State: "done", CWD: "/wt1", ModTime: now},
			{State: "question", CWD: "/wt2", ModTime: now},
		}
	}
	svc, sound, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	// When both attention and done transitions happen, only attention sound plays
	if len(sound.calls) != 1 || sound.calls[0] != true {
		t.Errorf("sound calls = %v, want [true] (attention takes priority)", sound.calls)
	}
}

func TestRefreshClaude_SubdirectorySession_ResolvesToWorktree(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "working", CWD: "/wt/src/deep", ModTime: time.Now()}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusWorking {
		t.Errorf("subdirectory session should resolve to parent worktree, got %q", status)
	}
}

func TestRefreshClaude_DismissLogic_DowngradesDoneToIdle(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessionTime := time.Now()
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: sessionTime}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	// First refresh: status becomes "done"
	svc.refreshClaude()
	svc.mu.RLock()
	before := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()
	if before != ClaudeStatusDone {
		t.Fatalf("precondition: status = %q, want done", before)
	}

	// Simulate what DismissDone does: record dismiss time, then refresh
	svc.mu.Lock()
	svc.dismissTimes["/wt"] = time.Now()
	svc.mu.Unlock()
	svc.refreshClaude()

	svc.mu.RLock()
	after := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()
	if after != ClaudeStatusIdle {
		t.Errorf("after dismiss: status = %q, want idle", after)
	}
}

func TestRefreshClaude_DoneDuration_InstantDismiss(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: time.Now().Add(-time.Millisecond)}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)
	svc.doneDuration = 0

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()
	if status != ClaudeStatusIdle {
		t.Errorf("instant dismiss: status = %q, want idle", status)
	}
}

func TestRefreshClaude_DoneDuration_PersistForever(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt"}}}}
	sessions := func() []groveSession {
		return []groveSession{{State: "done", CWD: "/wt", ModTime: time.Now().Add(-30 * time.Minute)}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)
	svc.doneDuration = -1

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].Worktrees[0].ClaudeStatus
	svc.mu.RUnlock()
	if status != ClaudeStatusDone {
		t.Errorf("persist forever: status = %q, want done", status)
	}
}

func TestRefreshClaude_NoSessions_AllIdle(t *testing.T) {
	ws := []Workspace{{Worktrees: []WorktreeInfo{{Path: "/wt1"}, {Path: "/wt2"}}}}
	sessions := func() []groveSession { return nil }
	svc, sound, tray := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	for _, wt := range svc.workspaces[0].Worktrees {
		if wt.ClaudeStatus != ClaudeStatusIdle {
			t.Errorf("worktree %s: status = %q, want idle", wt.Path, wt.ClaudeStatus)
		}
	}
	svc.mu.RUnlock()
	if len(sound.calls) != 0 {
		t.Errorf("no sound expected with no sessions, got %v", sound.calls)
	}
	if tray.badge {
		t.Error("no badge expected with no sessions")
	}
}

func TestResolveWorktreePath(t *testing.T) {
	sep := string(filepath.Separator)
	tests := []struct {
		name          string
		cwd           string
		worktreePaths []string
		want          string
	}{
		{
			name:          "exact match",
			cwd:           "/projects/grove",
			worktreePaths: []string{"/projects/grove"},
			want:          "/projects/grove",
		},
		{
			name:          "subdirectory match",
			cwd:           "/projects/grove" + sep + "src" + sep + "components",
			worktreePaths: []string{"/projects/grove"},
			want:          "/projects/grove",
		},
		{
			name:          "no match falls back to cwd",
			cwd:           "/other/path",
			worktreePaths: []string{"/projects/grove", "/projects/app"},
			want:          "/other/path",
		},
		{
			name:          "empty worktree paths falls back to cwd",
			cwd:           "/projects/grove",
			worktreePaths: nil,
			want:          "/projects/grove",
		},
		{
			name:          "prefix not on separator boundary is not a match",
			cwd:           "/projects/grove-extra",
			worktreePaths: []string{"/projects/grove"},
			want:          "/projects/grove-extra",
		},
		{
			name:          "first match wins among multiple candidates",
			cwd:           "/projects/grove" + sep + "sub",
			worktreePaths: []string{"/projects/grove", "/projects"},
			want:          "/projects/grove",
		},
		{
			name:          "deeper worktree matches over shallower when listed first",
			cwd:           "/projects/grove" + sep + "sub",
			worktreePaths: []string{"/projects", "/projects/grove"},
			want:          "/projects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveWorktreePath(tt.cwd, tt.worktreePaths); got != tt.want {
				t.Errorf("resolveWorktreePath(%q, %v) = %q, want %q", tt.cwd, tt.worktreePaths, got, tt.want)
			}
		})
	}
}

func TestIsProcessAlive(t *testing.T) {
	if !isProcessAlive(os.Getpid()) {
		t.Error("isProcessAlive(own pid) = false, want true")
	}
	if isProcessAlive(0) {
		t.Error("isProcessAlive(0) = true, want false")
	}
	if isProcessAlive(4999999) {
		t.Error("isProcessAlive(4999999) = true, want false")
	}
}

// --- MainWorktree monitoring tests ---

func TestRefreshClaude_MainWorktree_GetsClaudeStatus(t *testing.T) {
	ws := []Workspace{{
		Config:       WorkspaceConfig{RepoPath: "/repo"},
		MainWorktree: WorktreeInfo{Name: MainWorktreeName, Path: "/repo"},
		Worktrees:    []WorktreeInfo{{Path: "/wt"}},
	}}
	sessions := func() []groveSession {
		return []groveSession{{State: "working", CWD: "/repo", ModTime: time.Now()}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].MainWorktree.ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusWorking {
		t.Errorf("main worktree status = %q, want %q", status, ClaudeStatusWorking)
	}
}

func TestRefreshClaude_MainWorktree_SubdirectoryResolvesToMainRepo(t *testing.T) {
	ws := []Workspace{{
		Config:       WorkspaceConfig{RepoPath: "/repo"},
		MainWorktree: WorktreeInfo{Name: MainWorktreeName, Path: "/repo"},
		Worktrees:    []WorktreeInfo{{Path: "/wt"}},
	}}
	sessions := func() []groveSession {
		return []groveSession{{State: "permission", CWD: "/repo/src/deep", ModTime: time.Now()}}
	}
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].MainWorktree.ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusPermission {
		t.Errorf("main worktree status = %q, want %q", status, ClaudeStatusPermission)
	}
}

func TestRefreshClaude_MainWorktree_NoSession_StaysIdle(t *testing.T) {
	ws := []Workspace{{
		Config:       WorkspaceConfig{RepoPath: "/repo"},
		MainWorktree: WorktreeInfo{Name: MainWorktreeName, Path: "/repo"},
		Worktrees:    []WorktreeInfo{{Path: "/wt"}},
	}}
	sessions := func() []groveSession { return nil }
	svc, _, _ := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	svc.mu.RLock()
	status := svc.workspaces[0].MainWorktree.ClaudeStatus
	svc.mu.RUnlock()

	if status != ClaudeStatusIdle {
		t.Errorf("main worktree status = %q, want %q", status, ClaudeStatusIdle)
	}
}

func TestRefreshClaude_MainWorktree_SoundsAndTray(t *testing.T) {
	ws := []Workspace{{
		Config:       WorkspaceConfig{RepoPath: "/repo"},
		MainWorktree: WorktreeInfo{Name: MainWorktreeName, Path: "/repo"},
	}}
	sessions := func() []groveSession {
		return []groveSession{{State: "question", CWD: "/repo", ModTime: time.Now()}}
	}
	svc, sound, tray := newTestMonitor(ws, sessions)

	svc.refreshClaude()

	if len(sound.calls) != 1 || sound.calls[0] != true {
		t.Errorf("sound calls = %v, want [true] (attention sound for main worktree)", sound.calls)
	}
	if !tray.badge {
		t.Error("tray badge should be set for main worktree needing attention")
	}
}

func TestGroveStateToClaudeStatus(t *testing.T) {
	tests := []struct {
		state string
		want  ClaudeStatus
	}{
		{"working", ClaudeStatusWorking},
		{"permission", ClaudeStatusPermission},
		{"question", ClaudeStatusQuestion},
		{"done", ClaudeStatusDone},
		{"idle", ClaudeStatusIdle},
		{"", ClaudeStatusIdle},
		{"unknown", ClaudeStatusIdle},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := groveStateToClaudeStatus(tt.state); got != tt.want {
				t.Errorf("groveStateToClaudeStatus(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

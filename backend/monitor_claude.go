package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

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
	currentPaths := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(s.groveDir, entry.Name())
		currentPaths[filePath] = struct{}{}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		mtime := info.ModTime()

		// Cache hit: reuse parsed session when mtime is unchanged. Skips
		// the ReadFile + Unmarshal — the dominant cost in this loop.
		s.mu.RLock()
		cached, hit := s.sessionCache[filePath]
		s.mu.RUnlock()

		var sess groveSession
		if hit && cached.mtime.Equal(mtime) {
			sess = cached.parsed
		} else {
			data, err := os.ReadFile(filePath) // #nosec G304
			if err != nil {
				continue
			}
			if err := json.Unmarshal(data, &sess); err != nil {
				continue
			}
			s.mu.Lock()
			s.sessionCache[filePath] = sessionCacheEntry{mtime: mtime, parsed: sess}
			s.mu.Unlock()
		}

		if checkLiveness && !isProcessAlive(sess.PID) {
			toRemove = append(toRemove, filePath)
			continue
		}

		sess.ModTime = mtime
		sessions = append(sessions, sess)
	}

	// Evict cache entries for files that no longer exist (session ended +
	// file removed). Without this, the map grows unbounded across the
	// long-lived monitor session.
	s.mu.Lock()
	for p := range s.sessionCache {
		if _, ok := currentPaths[p]; !ok {
			delete(s.sessionCache, p)
		}
	}
	s.mu.Unlock()

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

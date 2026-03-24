package backend

// ClaudeStatus represents the current state of a Claude Code session.
type ClaudeStatus string

const (
	ClaudeStatusWorking    ClaudeStatus = "working"
	ClaudeStatusIdle       ClaudeStatus = "idle"
	ClaudeStatusDone       ClaudeStatus = "done"
	ClaudeStatusPermission ClaudeStatus = "permission"
	ClaudeStatusQuestion   ClaudeStatus = "question"
)

// claudeStatusPriority returns a numeric priority for aggregation (higher = more important).
func claudeStatusPriority(s ClaudeStatus) int {
	switch s {
	case ClaudeStatusPermission, ClaudeStatusQuestion:
		return 3
	case ClaudeStatusWorking:
		return 2
	case ClaudeStatusIdle, ClaudeStatusDone:
		return 1
	default:
		return 0
	}
}

// WorktreeInfo contains the status of a single git worktree.
type WorktreeInfo struct {
	Name         string       `json:"name"`
	Path         string       `json:"path"`
	Branch       string       `json:"branch"`
	FilesChanged int          `json:"filesChanged"`
	Insertions   int          `json:"insertions"`
	Deletions    int          `json:"deletions"`
	ClaudeStatus ClaudeStatus `json:"claudeStatus"`
}

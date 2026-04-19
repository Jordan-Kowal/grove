package backend

// WorkspaceConfig is the per-workspace configuration stored in config.json.
type WorkspaceConfig struct {
	RepoPath       string `json:"repoPath"`
	BaseBranch     string `json:"baseBranch"`
	SetupScript    string `json:"setupScript"`
	TeardownScript string `json:"archiveScript"`
	DeleteBranch   bool   `json:"deleteBranch"`
}

// Workspace represents a registered git repository with its worktrees.
type Workspace struct {
	Name         string          `json:"name"`
	Config       WorkspaceConfig `json:"config"`
	MainWorktree WorktreeInfo    `json:"mainWorktree"`
	Worktrees    []WorktreeInfo  `json:"worktrees"`
}

// BranchInfo describes a git branch (local or remote).
type BranchInfo struct {
	Name     string `json:"name"`
	IsRemote bool   `json:"isRemote"`
}

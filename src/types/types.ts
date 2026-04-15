export enum ClaudeStatus {
  WORKING = "working",
  IDLE = "idle",
  DONE = "done",
  PERMISSION = "permission",
  QUESTION = "question",
}

export type WorkspaceConfig = {
  repoPath: string;
  baseBranch: string;
  setupScript: string;
  archiveScript: string;
  deleteBranch: boolean;
};

export type WorktreeInfo = {
  name: string;
  path: string;
  branch: string;
  filesChanged: number;
  insertions: number;
  deletions: number;
  claudeStatus: ClaudeStatus;
  claudeSessionCounts: Partial<Record<ClaudeStatus, number>>;
  editorOpen: boolean;
};

export type Workspace = {
  name: string;
  config: WorkspaceConfig;
  mainWorktree: WorktreeInfo;
  worktrees: WorktreeInfo[];
};

export enum TaskStep {
  GIT_WORKTREE = "git_worktree",
  SETUP_SCRIPT = "setup_script",
  ARCHIVE_SCRIPT = "archive_script",
  GIT_REMOVE = "git_remove",
  REBASE = "rebase",
  CHECKOUT = "checkout",
  NEW_BRANCH = "new_branch",
}

export enum TaskStatus {
  IN_PROGRESS = "in_progress",
  SUCCESS = "success",
  FAILED = "failed",
}

export type WorktreeTaskEvent = {
  workspaceName: string;
  worktreeName: string;
  step: TaskStep;
  status: TaskStatus;
  error?: string;
};

export type LogLine = {
  text: string;
  timestamp: number; // Unix milliseconds
};

export type WorktreeLogEvent = {
  workspaceName: string;
  worktreeName: string;
  lines: string[];
  timestamp: number; // Unix milliseconds
};

export type BranchInfo = {
  name: string;
  isRemote: boolean;
};

export type WailsEvent<T = unknown> = {
  data: T;
};

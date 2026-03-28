import { createContext } from "solid-js";
import type {
  LogLine,
  Workspace,
  WorkspaceConfig,
  WorktreeTaskEvent,
} from "@/types/types";

export type DashboardContextProps = {
  workspaces: () => Workspace[];
  taskStatuses: () => Record<string, WorktreeTaskEvent>;
  taskStartedAt: () => Record<string, number>;
  pendingDeletes: () => Record<string, boolean>;
  addWorkspace: () => void;
  removeWorkspace: (name: string) => void;
  createWorktree: (workspaceName: string, worktreeName: string) => void;
  removeWorktree: (workspaceName: string, worktreeName: string) => void;
  confirmDelete: (workspaceName: string, worktreeName: string) => void;
  cancelDelete: (workspaceName: string, worktreeName: string) => void;
  forceRemoveWorktree: (workspaceName: string, worktreeName: string) => void;
  cancelTask: (workspaceName: string, worktreeName: string) => void;
  retrySetup: (workspaceName: string, worktreeName: string) => void;
  retryArchive: (workspaceName: string, worktreeName: string) => void;
  clearTaskStatus: (workspaceName: string, worktreeName: string) => void;
  getScriptLogs: (workspaceName: string, worktreeName: string) => LogLine[];
  clearScriptLogs: (workspaceName: string, worktreeName: string) => void;
  rebaseWorktree: (
    workspaceName: string,
    worktreeName: string,
    targetBranch: string,
  ) => void;
  checkoutBranch: (
    workspaceName: string,
    worktreeName: string,
    branch: string,
  ) => void;
  newBranchOnWorktree: (
    workspaceName: string,
    worktreeName: string,
    branchName: string,
  ) => void;
  focusEditor: (worktreePath: string) => void;
  updateWorkspaceConfig: (name: string, config: WorkspaceConfig) => void;
};

export const DashboardContext = createContext<DashboardContextProps>();

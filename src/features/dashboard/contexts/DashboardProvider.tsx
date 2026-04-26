import { MonitorService, WorkspaceService } from "@backend";
import { createSignal, type JSX, useContext } from "solid-js";
import { createStore, reconcile } from "solid-js/store";
import { useSettingsContext } from "@/contexts";
import {
  ClaudeStatus,
  type LogLine,
  type Workspace,
  type WorkspaceConfig,
  type WorktreeTaskEvent,
} from "@/types/types";
import { useEditorActions, useTaskEvents } from "../hooks";
import {
  DashboardContext,
  type DashboardContextProps,
} from "./DashboardContext";

export const useDashboardContext = (): DashboardContextProps => {
  const context = useContext(DashboardContext);
  if (!context) {
    throw new Error(
      "useDashboardContext must be used within a DashboardProvider",
    );
  }
  return context;
};

export type DashboardProviderProps = {
  children: JSX.Element;
};

export const DashboardProvider = (props: DashboardProviderProps) => {
  const { settings } = useSettingsContext();
  const [workspaces, setWorkspaces] = createStore<Workspace[]>([]);
  const [taskStatuses, setTaskStatuses] = createSignal<
    Record<string, WorktreeTaskEvent>
  >({});
  const [scriptLogs, setScriptLogs] = createSignal<Record<string, LogLine[]>>(
    {},
  );
  const [taskStartedAt, setTaskStartedAt] = createSignal<
    Record<string, number>
  >({});
  const [pendingDeletes, setPendingDeletes] = createSignal<
    Record<string, boolean>
  >({});

  useTaskEvents({
    workspaces,
    setWorkspaces,
    taskStatuses,
    setTaskStatuses,
    setTaskStartedAt,
    setScriptLogs,
  });

  const { focusEditor, closeEditor, closeAllEditors } = useEditorActions({
    workspaces,
    settings,
  });

  const addWorkspace = async () => {
    const path = await WorkspaceService.OpenFolderDialog();
    if (!path) return;
    try {
      await WorkspaceService.AddWorkspace(path);
      MonitorService.RefreshNow();
    } catch (e) {
      console.error("[grove] addWorkspace failed:", e);
    }
  };

  const removeWorkspace = async (name: string) => {
    try {
      await WorkspaceService.RemoveWorkspace(name);
      MonitorService.RefreshNow();
    } catch (e) {
      console.error("[grove] removeWorkspace failed:", e);
    }
  };

  const createWorktree = (workspaceName: string, worktreeName: string) => {
    // Optimistically add placeholder card (path="" signals it's a placeholder)
    setWorkspaces(
      reconcile(
        workspaces.map((ws) => {
          if (ws.name !== workspaceName) return ws;
          return {
            ...ws,
            worktrees: [
              ...(ws.worktrees ?? []),
              {
                name: worktreeName,
                path: "",
                branch: "",
                filesChanged: 0,
                insertions: 0,
                deletions: 0,
                claudeStatus: ClaudeStatus.IDLE,
                claudeSessionCounts: {},
                editorOpen: false,
              },
            ],
          };
        }),
        { key: "name" },
      ),
    );
    WorkspaceService.CreateWorktree(workspaceName, worktreeName).catch((e) =>
      console.error("[grove] createWorktree failed:", e),
    );
  };

  const removeWorktree = (workspaceName: string, worktreeName: string) => {
    const key = `${workspaceName}/${worktreeName}`;
    setPendingDeletes((prev) => ({ ...prev, [key]: true }));
  };

  const confirmDelete = (workspaceName: string, worktreeName: string) => {
    const key = `${workspaceName}/${worktreeName}`;
    setPendingDeletes((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
    WorkspaceService.RemoveWorktree(workspaceName, worktreeName).catch((e) =>
      console.error("[grove] removeWorktree failed:", e),
    );
  };

  const cancelDelete = (workspaceName: string, worktreeName: string) => {
    const key = `${workspaceName}/${worktreeName}`;
    setPendingDeletes((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const forceRemoveWorktree = (workspaceName: string, worktreeName: string) => {
    // Remove optimistic placeholder immediately
    setWorkspaces(
      reconcile(
        workspaces.map((ws) => {
          if (ws.name !== workspaceName) return ws;
          return {
            ...ws,
            worktrees: (ws.worktrees ?? []).filter(
              (wt) => wt.name !== worktreeName,
            ),
          };
        }),
        { key: "name" },
      ),
    );
    clearTaskStatus(workspaceName, worktreeName);
    WorkspaceService.ForceRemoveWorktree(workspaceName, worktreeName).catch(
      (e) => console.error("[grove] forceRemoveWorktree failed:", e),
    );
  };

  const cancelTask = (workspaceName: string, worktreeName: string) => {
    WorkspaceService.CancelTask(workspaceName, worktreeName).catch((e) =>
      console.error("[grove] cancelTask failed:", e),
    );
  };

  const retrySetup = (workspaceName: string, worktreeName: string) => {
    clearScriptLogs(workspaceName, worktreeName);
    WorkspaceService.RetrySetup(workspaceName, worktreeName).catch((e) =>
      console.error("[grove] retrySetup failed:", e),
    );
  };

  const retryArchive = (workspaceName: string, worktreeName: string) => {
    clearScriptLogs(workspaceName, worktreeName);
    WorkspaceService.RetryArchive(workspaceName, worktreeName).catch((e) =>
      console.error("[grove] retryArchive failed:", e),
    );
  };

  const clearTaskStatus = (workspaceName: string, worktreeName: string) => {
    const key = `${workspaceName}/${worktreeName}`;
    setTaskStatuses((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
    setTaskStartedAt((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const getScriptLogs = (workspaceName: string, worktreeName: string) => {
    const key = `${workspaceName}/${worktreeName}`;
    return scriptLogs()[key] ?? [];
  };

  const clearScriptLogs = (workspaceName: string, worktreeName: string) => {
    const key = `${workspaceName}/${worktreeName}`;
    setScriptLogs((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const rebaseWorktree = (
    workspaceName: string,
    worktreeName: string,
    targetBranch: string,
  ) => {
    WorkspaceService.RebaseWorktree(
      workspaceName,
      worktreeName,
      targetBranch,
    ).catch((e) => console.error("[grove] rebaseWorktree failed:", e));
  };

  const checkoutBranch = (
    workspaceName: string,
    worktreeName: string,
    branch: string,
  ) => {
    WorkspaceService.CheckoutBranch(workspaceName, worktreeName, branch).catch(
      (e) => console.error("[grove] checkoutBranch failed:", e),
    );
  };

  const newBranchOnWorktree = (
    workspaceName: string,
    worktreeName: string,
    branchName: string,
  ) => {
    WorkspaceService.NewBranchOnWorktree(
      workspaceName,
      worktreeName,
      branchName,
    ).catch((e) => console.error("[grove] newBranchOnWorktree failed:", e));
  };

  const updateWorkspaceConfig = async (
    name: string,
    config: WorkspaceConfig,
  ) => {
    try {
      await WorkspaceService.UpdateWorkspaceConfig(name, config);
      MonitorService.RefreshNow();
    } catch (e) {
      console.error("[grove] updateWorkspaceConfig failed:", e);
    }
  };

  const syncMainCheckout = async (workspaceName: string) => {
    try {
      await WorkspaceService.SyncMainCheckout(workspaceName);
    } catch (e) {
      console.error("[grove] syncMainCheckout failed:", e);
    }
  };

  const removeAllWorktrees = (workspaceName: string) => {
    const ws = workspaces.find((w) => w.name === workspaceName);
    if (!ws) return;
    for (const wt of ws.worktrees ?? []) {
      confirmDelete(workspaceName, wt.name);
    }
  };

  const contextValue: DashboardContextProps = {
    workspaces,
    taskStatuses,
    taskStartedAt,
    pendingDeletes,
    addWorkspace,
    removeWorkspace,
    createWorktree,
    removeWorktree,
    confirmDelete,
    cancelDelete,
    forceRemoveWorktree,
    cancelTask,
    retrySetup,
    retryArchive,
    clearTaskStatus,
    getScriptLogs,
    clearScriptLogs,
    rebaseWorktree,
    checkoutBranch,
    newBranchOnWorktree,
    focusEditor,
    closeEditor,
    closeAllEditors,
    updateWorkspaceConfig,
    syncMainCheckout,
    removeAllWorktrees,
  };

  return (
    <DashboardContext.Provider value={contextValue}>
      {props.children}
    </DashboardContext.Provider>
  );
};

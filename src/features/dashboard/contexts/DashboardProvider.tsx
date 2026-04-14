import {
  EditorService,
  MonitorService,
  SnapService,
  WorkspaceService,
} from "@backend";
import { Events } from "@wailsio/runtime";
import {
  createSignal,
  type JSX,
  onCleanup,
  onMount,
  useContext,
} from "solid-js";
import { createStore, reconcile } from "solid-js/store";
import { useSettingsContext } from "@/contexts";
import {
  ClaudeStatus,
  type LogLine,
  TaskStatus,
  type WailsEvent,
  type Workspace,
  type WorkspaceConfig,
  type WorktreeLogEvent,
  type WorktreeTaskEvent,
} from "@/types/types";
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
  const taskTimers = new Map<string, ReturnType<typeof setTimeout>>();

  // Merge real workspace data with optimistic worktrees (path="" placeholders).
  // Preserves placeholders until real data includes a worktree with the same name.
  const mergeWorkspaces = (real: Workspace[]) => {
    const prev = workspaces;
    return real.map((ws) => {
      const prevWs = prev.find((p) => p.name === ws.name);
      if (!prevWs) return ws;
      const realNames = new Set((ws.worktrees ?? []).map((wt) => wt.name));
      const optimistic = (prevWs.worktrees ?? []).filter(
        (wt) => wt.path === "" && !realNames.has(wt.name),
      );
      if (optimistic.length === 0) return ws;
      return { ...ws, worktrees: [...(ws.worktrees ?? []), ...optimistic] };
    });
  };

  onMount(() => {
    MonitorService.GetWorkspaces()
      .then((ws) => setWorkspaces(reconcile(ws, { key: "name" })))
      .catch((e) => console.error("[grove] initial GetWorkspaces failed:", e));
    const unsubWorkspaces = Events.On(
      "workspaces-updated",
      (event: WailsEvent<Workspace[]>) => {
        setWorkspaces(reconcile(mergeWorkspaces(event.data), { key: "name" }));
      },
    );
    const unsubTask = Events.On(
      "worktree-task",
      (event: WailsEvent<WorktreeTaskEvent>) => {
        const e = event.data;
        const key = `${e.workspaceName}/${e.worktreeName}`;
        const existing = taskTimers.get(key);
        if (existing) clearTimeout(existing);
        // Clear logs and record start time when a new step begins
        if (e.status === TaskStatus.IN_PROGRESS) {
          const prev = taskStatuses()[key];
          const isNewStep =
            !prev ||
            prev.step !== e.step ||
            prev.status !== TaskStatus.IN_PROGRESS;
          if (isNewStep) {
            setTaskStartedAt((prev) => ({ ...prev, [key]: Date.now() }));
          }
          setScriptLogs((prev) => {
            const next = { ...prev };
            delete next[key];
            return next;
          });
        }
        setTaskStatuses((prev) => ({ ...prev, [key]: e }));
        if (e.status === TaskStatus.SUCCESS) {
          const timer = setTimeout(() => {
            taskTimers.delete(key);
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
          }, 3000);
          taskTimers.set(key, timer);
        }
      },
    );
    // "refresh-requested" is handled by MonitorService (backend), which rescans
    // and emits "workspaces-updated" — no frontend listener needed.
    const unsubLog = Events.On(
      "worktree-log",
      (event: WailsEvent<WorktreeLogEvent>) => {
        const e = event.data;
        const key = `${e.workspaceName}/${e.worktreeName}`;
        const newLines: LogLine[] = e.lines.map((text) => ({
          text,
          timestamp: e.timestamp,
        }));
        setScriptLogs((prev) => ({
          ...prev,
          [key]: [...(prev[key] ?? []), ...newLines],
        }));
      },
    );

    onCleanup(() => {
      unsubWorkspaces();
      unsubTask();
      unsubLog();
      for (const timer of taskTimers.values()) clearTimeout(timer);
      taskTimers.clear();
    });
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

  const [pendingDeletes, setPendingDeletes] = createSignal<
    Record<string, boolean>
  >({});

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

  const focusEditor = async (worktreePath: string) => {
    MonitorService.DismissDone(worktreePath);
    const editorApp = settings().editorApp;
    try {
      await EditorService.FocusEditor(worktreePath, editorApp);
      const side = await SnapService.GetSnapSide();
      if (side) {
        const bounds = await SnapService.GetEditorBounds();
        if (bounds.width > 0) {
          await EditorService.PositionWindow(
            editorApp,
            bounds.x,
            bounds.y,
            bounds.width,
            bounds.height,
          );
        }
      }
    } catch (e) {
      console.error("[grove] focusEditor failed:", e);
    }
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

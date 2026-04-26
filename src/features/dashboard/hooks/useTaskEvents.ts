import { MonitorService } from "@backend";
import { Events } from "@wailsio/runtime";
import { onCleanup, onMount, type Setter } from "solid-js";
import { reconcile, type SetStoreFunction } from "solid-js/store";
import {
  type LogLine,
  TaskStatus,
  type WailsEvent,
  type Workspace,
  type WorktreeLogEvent,
  type WorktreeTaskEvent,
} from "@/types/types";

const SCRIPT_LOGS_MAX = 5000;

type Params = {
  workspaces: Workspace[];
  setWorkspaces: SetStoreFunction<Workspace[]>;
  taskStatuses: () => Record<string, WorktreeTaskEvent>;
  setTaskStatuses: Setter<Record<string, WorktreeTaskEvent>>;
  setTaskStartedAt: Setter<Record<string, number>>;
  setScriptLogs: Setter<Record<string, LogLine[]>>;
};

// Merge real workspace data with optimistic worktrees (path="" placeholders).
// Preserves placeholders until real data includes a worktree with the same name.
const mergeWorkspaces = (prev: Workspace[], real: Workspace[]) =>
  real.map((ws) => {
    const prevWs = prev.find((p) => p.name === ws.name);
    if (!prevWs) return ws;
    const realNames = new Set((ws.worktrees ?? []).map((wt) => wt.name));
    const optimistic = (prevWs.worktrees ?? []).filter(
      (wt) => wt.path === "" && !realNames.has(wt.name),
    );
    if (optimistic.length === 0) return ws;
    return { ...ws, worktrees: [...(ws.worktrees ?? []), ...optimistic] };
  });

export const useTaskEvents = ({
  workspaces,
  setWorkspaces,
  taskStatuses,
  setTaskStatuses,
  setTaskStartedAt,
  setScriptLogs,
}: Params) => {
  const taskTimers = new Map<string, ReturnType<typeof setTimeout>>();

  onMount(() => {
    MonitorService.Snapshot()
      .then((ws) => setWorkspaces(reconcile(ws, { key: "name" })))
      .catch((e) => console.error("[grove] initial Snapshot failed:", e));
    const unsubWorkspaces = Events.On(
      "workspaces-updated",
      (event: WailsEvent<Workspace[]>) => {
        setWorkspaces(
          reconcile(mergeWorkspaces(workspaces, event.data), { key: "name" }),
        );
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
        setScriptLogs((prev) => {
          const combined = [...(prev[key] ?? []), ...newLines];
          // Cap at 5k lines to keep append cost + <For> re-render bounded on
          // chatty scripts (e.g. npm install can emit 10k+ lines).
          const capped =
            combined.length > SCRIPT_LOGS_MAX
              ? combined.slice(-SCRIPT_LOGS_MAX)
              : combined;
          return { ...prev, [key]: capped };
        });
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
};

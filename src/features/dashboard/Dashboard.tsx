import { Plus, Settings } from "lucide-solid";
import { type Component, For, Show } from "solid-js";
import { WorkspaceSection } from "./components";
import { useDashboardContext } from "./contexts";

type DashboardProps = {
  onOpenSettings: () => void;
  onOpenLogs: (key: string) => void;
};

export const Dashboard: Component<DashboardProps> = (props) => {
  const ctx = useDashboardContext();

  return (
    <div class="flex flex-col h-screen">
      {/* Title bar */}
      <div class="drag-region flex items-center justify-between pl-18 pr-2 h-10 shrink-0 border-b border-base-300">
        <span class="text-[10px] font-semibold uppercase tracking-wider opacity-40 no-drag">
          Grove
        </span>
        <div class="flex items-center gap-0.5 no-drag">
          <button
            type="button"
            class="btn btn-ghost btn-xs p-1 h-auto min-h-0 opacity-40 hover:opacity-100"
            onClick={props.onOpenSettings}
            title="Settings"
          >
            <Settings size={14} />
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-1 h-auto min-h-0 opacity-40 hover:opacity-100"
            onClick={ctx.addWorkspace}
            title="Add workspace"
          >
            <Plus size={14} />
          </button>
        </div>
      </div>

      {/* Workspace sections */}
      <div class="flex-1 overflow-y-auto py-1">
        <Show
          when={ctx.workspaces().length > 0}
          fallback={
            <div class="flex flex-col items-center justify-center h-full gap-2 px-4">
              <p class="text-xs opacity-30 text-center">No workspaces</p>
              <button
                type="button"
                class="btn btn-ghost btn-xs opacity-50"
                onClick={ctx.addWorkspace}
              >
                <Plus size={12} />
                Add workspace
              </button>
            </div>
          }
        >
          <For each={ctx.workspaces()}>
            {(ws) => (
              <WorkspaceSection
                workspace={ws}
                taskStatuses={ctx.taskStatuses()}
                taskStartedAt={ctx.taskStartedAt()}
                pendingDeletes={ctx.pendingDeletes()}
                onCreateWorktree={(n) => ctx.createWorktree(ws.name, n)}
                onRemoveWorktree={(n) => ctx.removeWorktree(ws.name, n)}
                onConfirmDelete={(n) => ctx.confirmDelete(ws.name, n)}
                onCancelDelete={(n) => ctx.cancelDelete(ws.name, n)}
                onForceRemoveWorktree={(n) =>
                  ctx.forceRemoveWorktree(ws.name, n)
                }
                onRemoveWorkspace={() => ctx.removeWorkspace(ws.name)}
                onOpenWorkspace={() => ctx.focusEditor(ws.config.repoPath)}
                onClickWorktree={ctx.focusEditor}
                onCancelTask={(n) => ctx.cancelTask(ws.name, n)}
                onRetrySetup={(n) => ctx.retrySetup(ws.name, n)}
                onRetryArchive={(n) => ctx.retryArchive(ws.name, n)}
                onClearTaskStatus={(n) => ctx.clearTaskStatus(ws.name, n)}
                onOpenLogs={(n) => props.onOpenLogs(`${ws.name}/${n}`)}
                hasLogs={(n) => ctx.getScriptLogs(ws.name, n).length > 0}
                onRebase={(n, branch) => ctx.rebaseWorktree(ws.name, n, branch)}
                onCheckout={(n, branch) =>
                  ctx.checkoutBranch(ws.name, n, branch)
                }
                onNewBranch={(n, branchName) =>
                  ctx.newBranchOnWorktree(ws.name, n, branchName)
                }
              />
            )}
          </For>
        </Show>
      </div>
    </div>
  );
};

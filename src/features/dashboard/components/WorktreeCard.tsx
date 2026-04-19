import { type Component, createMemo, createSignal, Show } from "solid-js";
import { StatusBadge } from "@/components/ui";
import { useOutsideClick } from "@/hooks";
import {
  ClaudeStatus,
  TaskStatus,
  TaskStep,
  type WorktreeInfo,
} from "@/types/types";
import { useDashboardContext } from "../contexts";
import { type Action, QuickActionPanel } from "./QuickActionPanel";
import { TaskStatusBar } from "./TaskStatusBar";
import { WorktreeCardMenu } from "./WorktreeCardMenu";

type WorktreeCardProps = {
  workspaceName: string;
  worktree: WorktreeInfo;
  existingBranches: string[];
  baseBranch: string;
  hasSetupScript: boolean;
  isMainRepo?: boolean;
  onOpenLogs: () => void;
};

export const WorktreeCard: Component<WorktreeCardProps> = (props) => {
  const ctx = useDashboardContext();
  const [showMenu, setShowMenu] = createSignal(false);
  const [activeAction, setActiveAction] = createSignal<Action | null>(null);

  const key = () => `${props.workspaceName}/${props.worktree.name}`;
  const task = () => ctx.taskStatuses()[key()];
  const startedAt = () => ctx.taskStartedAt()[key()];
  const deletePending = () =>
    !props.isMainRepo && (ctx.pendingDeletes()[key()] ?? false);
  const step = () => task()?.step;
  const status = () => task()?.status;
  const hasDiff = () => props.worktree.filesChanged > 0;
  const sessionCount = createMemo(() => {
    const counts = props.worktree.claudeSessionCounts;
    if (!counts) return 0;
    return Object.values(counts).reduce((sum, n) => sum + (n ?? 0), 0);
  });
  const isActive = () => props.worktree.claudeStatus !== ClaudeStatus.IDLE;

  const isInProgress = () => status() === TaskStatus.IN_PROGRESS;
  const isFailed = () => status() === TaskStatus.FAILED;

  const isCardDisabled = () =>
    deletePending() ||
    activeAction() !== null ||
    ((step() === TaskStep.GIT_WORKTREE ||
      step() === TaskStep.GIT_REMOVE ||
      step() === TaskStep.REBASE ||
      step() === TaskStep.CHECKOUT ||
      step() === TaskStep.NEW_BRANCH) &&
      isInProgress());

  const showMenuAllowed = () =>
    !deletePending() &&
    !activeAction() &&
    (!task() || isFailed() || step() !== TaskStep.GIT_WORKTREE);

  useOutsideClick(showMenu, () => setShowMenu(false));

  const handleClick = () => {
    if (isCardDisabled()) return;
    ctx.focusEditor(props.worktree.path);
  };

  const openAction = (action: Action) => {
    setShowMenu(false);
    setActiveAction(action);
  };

  return (
    <div class="flex flex-col px-1 py-1 rounded hover:bg-base-200 transition-colors group cursor-pointer">
      <div class="flex items-start gap-1">
        <button
          type="button"
          class="flex-1 min-w-0 text-left"
          onClick={handleClick}
          disabled={isCardDisabled()}
        >
          <div class="flex gap-1.5">
            {/* Left column: status dot + session count */}
            <div class="flex flex-col items-center shrink-0 w-3">
              <StatusBadge
                status={props.worktree.claudeStatus}
                sessionCounts={props.worktree.claudeSessionCounts}
              />
              <Show when={isActive() && sessionCount() > 1}>
                <span class="text-[8px] font-bold opacity-50 leading-tight mt-0.5">
                  {sessionCount()}
                </span>
              </Show>
            </div>
            {/* Right column: name + branch */}
            <div class="flex flex-col min-w-0 flex-1">
              <div class="flex items-center gap-1">
                <span class="text-xs font-medium truncate">
                  {props.isMainRepo ? props.workspaceName : props.worktree.name}
                </span>
                <Show when={props.isMainRepo}>
                  <span class="badge badge-xs badge-primary text-[9px] shrink-0">
                    root
                  </span>
                </Show>
                <Show when={props.worktree.editorOpen}>
                  <span class="badge badge-xs badge-accent text-[9px] shrink-0">
                    active
                  </span>
                </Show>
              </div>
              <Show when={props.worktree.branch}>
                <div class="flex items-center justify-between">
                  <span class="text-[10px] opacity-40 truncate font-mono">
                    {props.worktree.branch}
                  </span>
                  <Show when={hasDiff()}>
                    <span class="text-[10px] opacity-40 shrink-0 ml-1 font-mono">
                      <span class="text-success">
                        +{props.worktree.insertions}
                      </span>{" "}
                      <span class="text-error">
                        -{props.worktree.deletions}
                      </span>
                    </span>
                  </Show>
                </div>
              </Show>
            </div>
          </div>
        </button>

        <Show when={showMenuAllowed()}>
          <WorktreeCardMenu
            workspaceName={props.workspaceName}
            worktree={props.worktree}
            isMainRepo={props.isMainRepo ?? false}
            hasSetupScript={props.hasSetupScript}
            showMenu={showMenu}
            setShowMenu={setShowMenu}
            onOpenAction={openAction}
          />
        </Show>
      </div>

      {/* Delete confirmation */}
      <Show when={!props.isMainRepo && deletePending()}>
        <div class="flex flex-col gap-0.5 pl-4.5 mt-0.5">
          <div class="flex items-center gap-1">
            <span class="text-[10px] text-warning flex-1">Delete?</span>
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-error opacity-70 hover:opacity-100"
              onClick={(e) => {
                e.stopPropagation();
                ctx.confirmDelete(props.workspaceName, props.worktree.name);
              }}
            >
              Yes
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
              onClick={(e) => {
                e.stopPropagation();
                ctx.cancelDelete(props.workspaceName, props.worktree.name);
              }}
            >
              No
            </button>
          </div>
        </div>
      </Show>

      {/* Quick action panel */}
      <Show when={activeAction()}>
        <QuickActionPanel
          workspaceName={props.workspaceName}
          worktreeName={props.worktree.name}
          baseBranch={props.baseBranch}
          existingBranches={props.existingBranches}
          action={activeAction()!}
          onDone={() => setActiveAction(null)}
        />
      </Show>

      {/* Task status bar */}
      <Show when={task() && !deletePending() && !activeAction()}>
        <TaskStatusBar
          workspaceName={props.workspaceName}
          worktreeName={props.worktree.name}
          taskEvent={task()!}
          startedAt={startedAt()}
          onOpenLogs={props.onOpenLogs}
        />
      </Show>
    </div>
  );
};

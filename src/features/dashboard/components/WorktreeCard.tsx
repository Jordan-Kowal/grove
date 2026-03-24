import { EllipsisVertical } from "lucide-solid";
import {
  type Component,
  createEffect,
  createSignal,
  onCleanup,
  Show,
} from "solid-js";
import { StatusBadge } from "@/components/ui";
import {
  TaskStatus,
  TaskStep,
  type WorktreeInfo,
  type WorktreeTaskEvent,
} from "@/types/types";
import { TaskStatusBar } from "./TaskStatusBar";

type WorktreeCardProps = {
  worktree: WorktreeInfo;
  deletePending: boolean;
  hasLogs: boolean;
  taskEvent?: WorktreeTaskEvent;
  taskStartedAt?: number;
  onClick: () => void;
  onRemove: () => void;
  onConfirmDelete: () => void;
  onCancelDelete: () => void;
  onForceRemove: () => void;
  onCancelTask: () => void;
  onRetrySetup: () => void;
  onRetryArchive: () => void;
  onClearTaskStatus: () => void;
  onOpenLogs: () => void;
};

export const WorktreeCard: Component<WorktreeCardProps> = (props) => {
  const [showMenu, setShowMenu] = createSignal(false);

  const task = () => props.taskEvent;
  const step = () => task()?.step;
  const status = () => task()?.status;
  const hasDiff = () => props.worktree.filesChanged > 0;

  const isInProgress = () => status() === TaskStatus.IN_PROGRESS;
  const isFailed = () => status() === TaskStatus.FAILED;

  // Card is non-actionable during git worktree creation, removal, or delete confirmation
  const isCardDisabled = () =>
    props.deletePending ||
    ((step() === TaskStep.GIT_WORKTREE || step() === TaskStep.GIT_REMOVE) &&
      isInProgress());

  // Show menu when idle, or during setup/archive (card is actionable)
  const showMenuAllowed = () =>
    !props.deletePending &&
    (!task() || isFailed() || step() !== TaskStep.GIT_WORKTREE);

  // Close menu on outside click
  createEffect(() => {
    if (!showMenu()) return;
    const handler = () => setShowMenu(false);
    document.addEventListener("click", handler);
    onCleanup(() => document.removeEventListener("click", handler));
  });

  const handleClick = () => {
    if (isCardDisabled()) return;
    props.onClick();
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
          <div class="flex items-center gap-1.5">
            <StatusBadge status={props.worktree.claudeStatus} />
            <span class="text-xs font-medium truncate">
              {props.worktree.name}
            </span>
          </div>
          <Show when={props.worktree.branch}>
            <div class="flex items-center justify-between pl-4.5">
              <span class="text-[10px] opacity-40 truncate font-mono">
                {props.worktree.branch}
              </span>
              <Show when={hasDiff()}>
                <span class="text-[10px] opacity-40 shrink-0 ml-1 font-mono">
                  <span class="text-success">+{props.worktree.insertions}</span>{" "}
                  <span class="text-error">-{props.worktree.deletions}</span>
                </span>
              </Show>
            </div>
          </Show>
        </button>

        {/* Menu */}
        <Show when={showMenuAllowed()}>
          <div class="relative shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0"
              onClick={(e) => {
                e.stopPropagation();
                setShowMenu(!showMenu());
              }}
            >
              <EllipsisVertical size={12} />
            </button>
            <Show when={showMenu()}>
              <div class="absolute right-0 top-full z-50 mt-1">
                <ul class="menu menu-xs bg-base-300 rounded-box shadow-lg w-36 p-1">
                  <li>
                    <button
                      type="button"
                      class="text-error text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowMenu(false);
                        props.onRemove();
                      }}
                    >
                      Remove worktree
                    </button>
                  </li>
                </ul>
              </div>
            </Show>
          </div>
        </Show>
      </div>

      {/* Delete confirmation */}
      <Show when={props.deletePending}>
        <div class="flex flex-col gap-0.5 pl-4.5 mt-0.5">
          <div class="flex items-center gap-1">
            <span class="text-[10px] text-warning flex-1">Delete?</span>
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-error opacity-70 hover:opacity-100"
              onClick={(e) => {
                e.stopPropagation();
                props.onConfirmDelete();
              }}
            >
              Yes
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
              onClick={(e) => {
                e.stopPropagation();
                props.onCancelDelete();
              }}
            >
              No
            </button>
          </div>
        </div>
      </Show>

      {/* Task status bar */}
      <Show when={task() && !props.deletePending}>
        <TaskStatusBar
          taskEvent={task()!}
          startedAt={props.taskStartedAt}
          hasLogs={props.hasLogs}
          onOpenLogs={props.onOpenLogs}
          onCancelTask={props.onCancelTask}
          onRetrySetup={props.onRetrySetup}
          onRetryArchive={props.onRetryArchive}
          onClearTaskStatus={props.onClearTaskStatus}
          onForceRemove={props.onForceRemove}
        />
      </Show>
    </div>
  );
};

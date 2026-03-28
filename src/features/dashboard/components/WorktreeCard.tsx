import { EllipsisVertical } from "lucide-solid";
import {
  type Component,
  createEffect,
  createSignal,
  onCleanup,
  Show,
} from "solid-js";
import { BranchNameInput, BranchSelect, StatusBadge } from "@/components/ui";
import {
  TaskStatus,
  TaskStep,
  type WorktreeInfo,
  type WorktreeTaskEvent,
} from "@/types/types";
import { TaskStatusBar } from "./TaskStatusBar";

enum Action {
  REBASE = "rebase",
  CHECKOUT = "checkout",
  NEW_BRANCH = "newBranch",
}

const ACTION_LABELS: Record<Action, string> = {
  [Action.REBASE]: "Rebase on branch",
  [Action.CHECKOUT]: "Checkout branch",
  [Action.NEW_BRANCH]: "New branch",
};

type WorktreeCardProps = {
  worktree: WorktreeInfo;
  workspaceName: string;
  baseBranch: string;
  existingBranches: string[];
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
  onRebase: (targetBranch: string) => void;
  onCheckout: (branch: string) => void;
  onNewBranch: (branchName: string) => void;
};

export const WorktreeCard: Component<WorktreeCardProps> = (props) => {
  const [showMenu, setShowMenu] = createSignal(false);
  const [activeAction, setActiveAction] = createSignal<Action | null>(null);
  const [selectedBranch, setSelectedBranch] = createSignal("");

  const task = () => props.taskEvent;
  const step = () => task()?.step;
  const status = () => task()?.status;
  const hasDiff = () => props.worktree.filesChanged > 0;

  const isInProgress = () => status() === TaskStatus.IN_PROGRESS;
  const isFailed = () => status() === TaskStatus.FAILED;

  // Card is non-actionable during git worktree creation, removal, delete confirmation, or active action
  const isCardDisabled = () =>
    props.deletePending ||
    activeAction() !== null ||
    ((step() === TaskStep.GIT_WORKTREE ||
      step() === TaskStep.GIT_REMOVE ||
      step() === TaskStep.REBASE ||
      step() === TaskStep.CHECKOUT ||
      step() === TaskStep.NEW_BRANCH) &&
      isInProgress());

  // Show menu when idle, or during setup/archive (card is actionable)
  const showMenuAllowed = () =>
    !props.deletePending &&
    !activeAction() &&
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

  const openAction = (action: Action) => {
    setShowMenu(false);
    setSelectedBranch(
      action === Action.REBASE ? props.baseBranch || "origin/main" : "",
    );
    setActiveAction(action);
  };

  const cancelAction = () => {
    setActiveAction(null);
    setSelectedBranch("");
  };

  // Only handles rebase/checkout — newBranch submits via BranchNameInput.onSubmit directly
  const confirmAction = () => {
    const action = activeAction();
    const branch = selectedBranch();
    setActiveAction(null);
    setSelectedBranch("");
    if (action === Action.REBASE && branch) props.onRebase(branch);
    if (action === Action.CHECKOUT && branch) props.onCheckout(branch);
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
                <ul class="menu menu-xs bg-base-300 rounded-box shadow-lg w-40 p-1">
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        openAction(Action.REBASE);
                      }}
                    >
                      Rebase on branch...
                    </button>
                  </li>
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        openAction(Action.CHECKOUT);
                      }}
                    >
                      Checkout branch...
                    </button>
                  </li>
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        openAction(Action.NEW_BRANCH);
                      }}
                    >
                      New branch...
                    </button>
                  </li>
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

      {/* Action panel (rebase / checkout / new branch) */}
      <Show when={activeAction()}>
        <div class="pl-4.5 mt-0.5 space-y-1">
          <span class="text-[10px] opacity-60">
            {ACTION_LABELS[activeAction()!]}
          </span>
          <Show
            when={
              activeAction() === Action.REBASE ||
              activeAction() === Action.CHECKOUT
            }
          >
            <BranchSelect
              workspaceName={props.workspaceName}
              value={selectedBranch()}
              onSelect={setSelectedBranch}
            />
          </Show>
          <Show when={activeAction() === Action.NEW_BRANCH}>
            <BranchNameInput
              placeholder="Branch name..."
              forbiddenNames={props.existingBranches}
              onSubmit={(name) => {
                setActiveAction(null);
                props.onNewBranch(name);
              }}
              onCancel={cancelAction}
            />
          </Show>
          <Show when={activeAction() !== "newBranch"}>
            <div class="flex justify-end gap-1">
              <button
                type="button"
                class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
                onClick={(e) => {
                  e.stopPropagation();
                  cancelAction();
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-info opacity-70 hover:opacity-100"
                disabled={!selectedBranch()}
                onClick={(e) => {
                  e.stopPropagation();
                  confirmAction();
                }}
              >
                OK
              </button>
            </div>
          </Show>
        </div>
      </Show>

      {/* Task status bar */}
      <Show when={task() && !props.deletePending && !activeAction()}>
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

import {
  AppWindowMac,
  Check,
  ClipboardCopy,
  EllipsisVertical,
  GitBranch,
  GitBranchPlus,
  GitMerge,
  Play,
  Trash2,
} from "lucide-solid";
import {
  type Component,
  createMemo,
  createSignal,
  onCleanup,
  Show,
} from "solid-js";
import { StatusBadge } from "@/components/ui";
import { useOutsideClick } from "@/hooks";
import {
  ClaudeStatus,
  TaskStatus,
  TaskStep,
  type WorktreeInfo,
} from "@/types/types";
import { useDashboardContext } from "../contexts";
import { Action, QuickActionPanel } from "./QuickActionPanel";
import { TaskStatusBar } from "./TaskStatusBar";

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
  const [copied, setCopied] = createSignal(false);
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(copiedTimer));

  const copyBranch = () => {
    navigator.clipboard.writeText(props.worktree.branch);
    setCopied(true);
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => {
      setCopied(false);
      setShowMenu(false);
    }, 1500);
  };

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
                <ul class="menu menu-xs bg-base-300 rounded-box shadow-lg w-48 p-1">
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        openAction(Action.REBASE);
                      }}
                    >
                      <GitMerge size={12} />
                      Rebase current branch
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
                      <GitBranch size={12} />
                      Checkout existing branch
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
                      <GitBranchPlus size={12} />
                      Move to new branch
                    </button>
                  </li>
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        copyBranch();
                      }}
                    >
                      <Show
                        when={copied()}
                        fallback={<ClipboardCopy size={12} />}
                      >
                        <Check size={12} class="text-success" />
                      </Show>
                      {copied() ? "Copied!" : "Copy branch name"}
                    </button>
                  </li>
                  <Show when={!props.isMainRepo && props.hasSetupScript}>
                    <li>
                      <button
                        type="button"
                        class="text-xs"
                        onClick={(e) => {
                          e.stopPropagation();
                          setShowMenu(false);
                          ctx.retrySetup(
                            props.workspaceName,
                            props.worktree.name,
                          );
                        }}
                      >
                        <Play size={12} />
                        Rerun setup script
                      </button>
                    </li>
                  </Show>
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      classList={{
                        "text-warning": props.worktree.editorOpen,
                        "opacity-30 pointer-events-none":
                          !props.worktree.editorOpen,
                      }}
                      disabled={!props.worktree.editorOpen}
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowMenu(false);
                        ctx.closeEditor(props.worktree.path);
                      }}
                    >
                      <AppWindowMac size={12} />
                      Close editor window
                    </button>
                  </li>
                  <Show when={!props.isMainRepo}>
                    <li>
                      <button
                        type="button"
                        class="text-error text-xs"
                        onClick={(e) => {
                          e.stopPropagation();
                          setShowMenu(false);
                          ctx.removeWorktree(
                            props.workspaceName,
                            props.worktree.name,
                          );
                        }}
                      >
                        <Trash2 size={12} />
                        Remove worktree
                      </button>
                    </li>
                  </Show>
                </ul>
              </div>
            </Show>
          </div>
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

import { Check, RefreshCw, ScrollText, Square, Trash2, X } from "lucide-solid";
import {
  type Component,
  createEffect,
  createSignal,
  onCleanup,
  Show,
} from "solid-js";
import { TaskStatus, TaskStep, type WorktreeTaskEvent } from "@/types/types";
import { useDashboardContext } from "../contexts";
import { ActionButton } from "./ActionButton";

const STEP_LABELS: Record<TaskStep, string> = {
  [TaskStep.GIT_WORKTREE]: "Git worktree",
  [TaskStep.SETUP_SCRIPT]: "Setup script",
  [TaskStep.ARCHIVE_SCRIPT]: "Archive script",
  [TaskStep.GIT_REMOVE]: "Removing worktree",
  [TaskStep.REBASE]: "Rebase",
  [TaskStep.CHECKOUT]: "Checkout",
  [TaskStep.NEW_BRANCH]: "New branch",
};

type TaskStatusBarProps = {
  workspaceName: string;
  worktreeName: string;
  taskEvent: WorktreeTaskEvent;
  startedAt?: number;
  onOpenLogs: () => void;
};

export const TaskStatusBar: Component<TaskStatusBarProps> = (props) => {
  const ctx = useDashboardContext();
  const [elapsed, setElapsed] = createSignal(0);

  const step = () => props.taskEvent.step;
  const status = () => props.taskEvent.status;
  const stepLabel = () => STEP_LABELS[step()];
  const hasLogs = () =>
    ctx.getScriptLogs(props.workspaceName, props.worktreeName).length > 0;

  const isInProgress = () => status() === TaskStatus.IN_PROGRESS;
  const isFailed = () => status() === TaskStatus.FAILED;
  const isSuccess = () => status() === TaskStatus.SUCCESS;

  const cancelTask = () =>
    ctx.cancelTask(props.workspaceName, props.worktreeName);
  const retrySetup = () =>
    ctx.retrySetup(props.workspaceName, props.worktreeName);
  const retryArchive = () =>
    ctx.retryArchive(props.workspaceName, props.worktreeName);
  const clearTaskStatus = () =>
    ctx.clearTaskStatus(props.workspaceName, props.worktreeName);
  const forceRemove = () =>
    ctx.forceRemoveWorktree(props.workspaceName, props.worktreeName);

  // Timestamp-based elapsed timer: immune to parent re-renders
  createEffect(() => {
    if (!isInProgress() || !props.startedAt) return;
    const startedAt = props.startedAt;
    const tick = () => setElapsed(Math.floor((Date.now() - startedAt) / 1000));
    tick();
    const interval = setInterval(tick, 1000);
    onCleanup(() => clearInterval(interval));
  });

  const LogsButton = () => (
    <Show when={hasLogs()}>
      <ActionButton
        tip="View logs"
        icon={<ScrollText size={10} />}
        opacity="opacity-50"
        onClick={props.onOpenLogs}
      />
    </Show>
  );

  return (
    <div class="flex items-center gap-1 pl-4.5 mt-0.5">
      {/* In progress */}
      <Show when={isInProgress()}>
        <span class="loading loading-spinner loading-xs text-info" />
        <span class="text-[10px] text-info flex-1">
          {stepLabel()} in progress... {elapsed()}s
        </span>
        <LogsButton />
        <ActionButton
          tip="Stop"
          icon={<Square size={10} />}
          opacity="opacity-50"
          onClick={cancelTask}
        />
      </Show>

      {/* Success */}
      <Show when={isSuccess()}>
        <Check size={10} class="text-success" />
        <span class="text-[10px] text-success flex-1">
          {stepLabel()} successful
        </span>
        <LogsButton />
      </Show>

      {/* Failed: setup script */}
      <Show when={isFailed() && step() === TaskStep.SETUP_SCRIPT}>
        <X size={10} class="text-error shrink-0" />
        <span class="text-[10px] text-error flex-1">{stepLabel()} failed</span>
        <LogsButton />
        <ActionButton
          tip="Dismiss"
          icon={<X size={10} />}
          onClick={clearTaskStatus}
        />
        <ActionButton
          tip="Retry"
          icon={<RefreshCw size={10} />}
          onClick={retrySetup}
        />
      </Show>

      {/* Failed: archive script */}
      <Show when={isFailed() && step() === TaskStep.ARCHIVE_SCRIPT}>
        <X size={10} class="text-error shrink-0" />
        <span class="text-[10px] text-error flex-1">{stepLabel()} failed</span>
        <LogsButton />
        <ActionButton
          tip="Dismiss"
          icon={<X size={10} />}
          onClick={clearTaskStatus}
        />
        <ActionButton
          tip="Force delete"
          icon={<Trash2 size={10} />}
          onClick={forceRemove}
        />
        <ActionButton
          tip="Retry"
          icon={<RefreshCw size={10} />}
          onClick={retryArchive}
        />
      </Show>

      {/* Failed: git worktree */}
      <Show when={isFailed() && step() === TaskStep.GIT_WORKTREE}>
        <X size={10} class="text-error shrink-0" />
        <span class="text-[10px] text-error flex-1">{stepLabel()} failed</span>
        <ActionButton
          tip="Dismiss"
          icon={<X size={10} />}
          onClick={() => {
            clearTaskStatus();
            forceRemove();
          }}
        />
      </Show>

      {/* Failed: rebase / checkout / new branch */}
      <Show
        when={
          isFailed() &&
          (step() === TaskStep.REBASE ||
            step() === TaskStep.CHECKOUT ||
            step() === TaskStep.NEW_BRANCH)
        }
      >
        <X size={10} class="text-error shrink-0" />
        <span class="text-[10px] text-error flex-1">{stepLabel()} failed</span>
        <LogsButton />
        <ActionButton
          tip="Dismiss"
          icon={<X size={10} />}
          onClick={clearTaskStatus}
        />
      </Show>
    </div>
  );
};

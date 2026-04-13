import { ArrowLeft, Check, RefreshCw, Square, X } from "lucide-solid";
import {
  type Component,
  createEffect,
  createSignal,
  For,
  Show,
} from "solid-js";
import { useElapsedTimer } from "@/hooks";
import { type LogLine, TaskStatus, TaskStep } from "@/types/types";
import { parseAnsiToSegments } from "@/utils";
import { useDashboardContext } from "../contexts";

type ErrorLogProps = {
  logKey: string;
  onBack: () => void;
};

const STEP_LABELS: Record<string, string> = {
  [TaskStep.SETUP_SCRIPT]: "Setup script",
  [TaskStep.ARCHIVE_SCRIPT]: "Archive script",
};

const formatTime = (ms: number) => {
  const d = new Date(ms);
  return d.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
};

const AnsiLine: Component<{ line: LogLine; showTimestamp: boolean }> = (
  props,
) => {
  const segments = () => parseAnsiToSegments(props.line.text);

  return (
    <div class="flex">
      <Show
        when={props.showTimestamp}
        fallback={<span class="w-18 shrink-0" />}
      >
        <span class="w-18 shrink-0 opacity-30 select-none">
          {formatTime(props.line.timestamp)}
        </span>
      </Show>
      <span class="flex-1">
        <For each={segments()}>
          {(segment) => (
            <span class={segment.classes.join(" ")}>{segment.text}</span>
          )}
        </For>
      </span>
    </div>
  );
};

export const ErrorLog: Component<ErrorLogProps> = (props) => {
  const ctx = useDashboardContext();
  let scrollRef: HTMLDivElement | undefined;
  const [autoScroll, setAutoScroll] = createSignal(true);

  const [workspaceName, worktreeName] = props.logKey.split("/", 2);

  const lines = () => ctx.getScriptLogs(workspaceName, worktreeName);
  const task = () => ctx.taskStatuses()[props.logKey];
  const startedAt = () => ctx.taskStartedAt()[props.logKey];

  const status = () => task()?.status;
  const step = () => task()?.step;
  const stepLabel = () => STEP_LABELS[step()!] ?? step();
  const isInProgress = () => status() === TaskStatus.IN_PROGRESS;
  const isFailed = () => status() === TaskStatus.FAILED;
  const isSuccess = () => status() === TaskStatus.SUCCESS;

  const elapsed = useElapsedTimer(isInProgress, startedAt);

  // Auto-scroll to bottom when new lines arrive (after DOM paint)
  createEffect(() => {
    lines();
    if (autoScroll() && scrollRef) {
      queueMicrotask(() => {
        if (scrollRef) scrollRef.scrollTop = scrollRef.scrollHeight;
      });
    }
  });

  const handleScroll = () => {
    if (!scrollRef) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 40);
  };

  return (
    <div class="flex flex-col h-screen">
      {/* Title bar */}
      <div class="drag-region flex items-center gap-2 pl-18 pr-3 h-10 shrink-0 border-b border-base-300">
        <button
          type="button"
          class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 no-drag opacity-50 hover:opacity-100"
          onClick={props.onBack}
        >
          <ArrowLeft size={14} />
        </button>
        <span class="text-[10px] font-semibold uppercase tracking-wider opacity-50 no-drag truncate flex-1">
          {props.logKey}
        </span>
      </div>

      {/* Task status bar */}
      <Show when={task()}>
        <div class="flex items-center gap-1.5 px-4 py-1.5 border-b border-base-300">
          {/* In progress */}
          <Show when={isInProgress()}>
            <span class="loading loading-spinner loading-xs text-info" />
            <span class="text-xs text-info flex-1">
              {stepLabel()} in progress... {elapsed()}s
            </span>
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 opacity-50 hover:opacity-100"
              onClick={() => ctx.cancelTask(workspaceName, worktreeName)}
              title="Stop"
            >
              <Square size={12} />
            </button>
          </Show>

          {/* Success */}
          <Show when={isSuccess()}>
            <Check size={12} class="text-success" />
            <span class="text-xs text-success flex-1">
              {stepLabel()} successful
            </span>
          </Show>

          {/* Failed */}
          <Show when={isFailed()}>
            <X size={12} class="text-error" />
            <span class="text-xs text-error flex-1">{stepLabel()} failed</span>
            <Show when={step() === TaskStep.SETUP_SCRIPT}>
              <button
                type="button"
                class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 opacity-50 hover:opacity-100"
                onClick={() => ctx.retrySetup(workspaceName, worktreeName)}
                title="Retry"
              >
                <RefreshCw size={12} />
              </button>
            </Show>
            <Show when={step() === TaskStep.ARCHIVE_SCRIPT}>
              <button
                type="button"
                class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 opacity-50 hover:opacity-100"
                onClick={() => ctx.retryArchive(workspaceName, worktreeName)}
                title="Retry"
              >
                <RefreshCw size={12} />
              </button>
            </Show>
          </Show>
        </div>
      </Show>

      {/* Log content */}
      <div
        ref={scrollRef}
        class="flex-1 overflow-auto p-4"
        onScroll={handleScroll}
      >
        <pre class="text-[11px] font-mono whitespace-pre-wrap wrap-break-word opacity-70 leading-relaxed select-text">
          <For each={lines()}>
            {(line, i) => (
              <AnsiLine
                line={line}
                showTimestamp={
                  i() === 0 || lines()[i() - 1].timestamp !== line.timestamp
                }
              />
            )}
          </For>
        </pre>
      </div>
    </div>
  );
};

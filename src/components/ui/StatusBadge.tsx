import { type Component, createMemo, For, Match, Show, Switch } from "solid-js";
import { ClaudeStatus } from "@/types/types";

type StatusBadgeProps = {
  status: ClaudeStatus;
  sessionCounts?: Partial<Record<ClaudeStatus, number>>;
};

const STATUS_ORDER: ClaudeStatus[] = [
  ClaudeStatus.PERMISSION,
  ClaudeStatus.QUESTION,
  ClaudeStatus.DONE,
  ClaudeStatus.WORKING,
  ClaudeStatus.IDLE,
];

const StatusDot: Component<{ status: ClaudeStatus; class?: string }> = (
  props,
) => {
  const needsAttention = () =>
    props.status === ClaudeStatus.PERMISSION ||
    props.status === ClaudeStatus.QUESTION;

  return (
    <Switch
      fallback={
        <span
          class={`inline-block rounded-full border-2 border-info ${props.class ?? "size-2.5"}`}
        />
      }
    >
      <Match when={props.status === ClaudeStatus.WORKING}>
        <span
          class={`loading loading-spinner text-primary ${props.class ?? "loading-xs"}`}
        />
      </Match>
      <Match when={props.status === ClaudeStatus.DONE}>
        <span
          class={`inline-block rounded-full bg-success ${props.class ?? "size-2.5"}`}
        />
      </Match>
      <Match when={props.status === ClaudeStatus.IDLE}>
        <span
          class={`inline-block rounded-full border-2 border-info ${props.class ?? "size-2.5"}`}
        />
      </Match>
      <Match when={needsAttention()}>
        <span
          class={`inline-block rounded-full bg-error ${props.class ?? "size-2.5"}`}
        />
      </Match>
    </Switch>
  );
};

export const StatusBadge: Component<StatusBadgeProps> = (props) => {
  const allDots = createMemo(() => {
    const counts = props.sessionCounts;
    if (!counts) return [];
    const dots: ClaudeStatus[] = [];
    for (const s of STATUS_ORDER) {
      const n = counts[s] ?? 0;
      for (let i = 0; i < n; i++) dots.push(s);
    }
    return dots;
  });

  const hasMultiple = () => allDots().length > 1;

  return (
    <span class="relative inline-flex shrink-0 items-center justify-center group/badge">
      <span class="inline-flex size-3 items-center justify-center">
        <StatusDot status={props.status} />
      </span>

      {/* Hover tooltip: row of status dots */}
      <Show when={hasMultiple()}>
        <div class="pointer-events-none absolute left-1/2 -translate-x-1/2 top-full mt-1 z-50 opacity-0 group-hover/badge:opacity-100 transition-opacity duration-150">
          <div class="bg-base-300 rounded-md shadow-lg px-1.5 py-1 flex items-center gap-1 ring-1 ring-base-content/10">
            <For each={allDots()}>
              {(status) => <StatusDot status={status} class="size-2" />}
            </For>
          </div>
        </div>
      </Show>
    </span>
  );
};

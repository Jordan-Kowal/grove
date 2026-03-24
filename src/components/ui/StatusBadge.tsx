import { type Component, Match, Switch } from "solid-js";
import { ClaudeStatus } from "@/types/types";

type StatusBadgeProps = {
  status: ClaudeStatus;
};

export const StatusBadge: Component<StatusBadgeProps> = (props) => {
  const needsAttention = () =>
    props.status === ClaudeStatus.PERMISSION ||
    props.status === ClaudeStatus.QUESTION;

  return (
    <span class="inline-flex size-3 shrink-0 items-center justify-center">
      <Switch
        fallback={
          <span
            class="inline-block size-2.5 rounded-full border-2 border-info"
            title="Idle"
          />
        }
      >
        <Match when={props.status === ClaudeStatus.WORKING}>
          <span
            class="loading loading-spinner loading-xs text-primary"
            title="Working"
          />
        </Match>
        <Match when={props.status === ClaudeStatus.DONE}>
          <span
            class="inline-block size-2.5 rounded-full bg-success"
            title="Done"
          />
        </Match>
        <Match when={props.status === ClaudeStatus.IDLE}>
          <span
            class="inline-block size-2.5 rounded-full border-2 border-info"
            title="Idle"
          />
        </Match>
        <Match when={needsAttention()}>
          <span
            class="inline-block size-2.5 rounded-full bg-error"
            title="Blocked"
          />
        </Match>
      </Switch>
    </span>
  );
};

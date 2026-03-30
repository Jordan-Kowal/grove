import { type Component, createSignal, Show } from "solid-js";
import { BranchNameInput, BranchSelect } from "@/components/ui";
import { useDashboardContext } from "../contexts";

export enum Action {
  REBASE = "rebase",
  CHECKOUT = "checkout",
  NEW_BRANCH = "newBranch",
}

const ACTION_LABELS: Record<Action, string> = {
  [Action.REBASE]: "Rebase current branch",
  [Action.CHECKOUT]: "Checkout existing branch",
  [Action.NEW_BRANCH]: "Move to new branch",
};

type QuickActionPanelProps = {
  workspaceName: string;
  worktreeName: string;
  baseBranch: string;
  existingBranches: string[];
  action: Action;
  onDone: () => void;
};

export const QuickActionPanel: Component<QuickActionPanelProps> = (props) => {
  const ctx = useDashboardContext();
  const [selectedBranch, setSelectedBranch] = createSignal(
    props.action === Action.REBASE ? props.baseBranch || "origin/main" : "",
  );

  const confirm = () => {
    const branch = selectedBranch();
    if (!branch) return;
    if (props.action === Action.REBASE) {
      ctx.rebaseWorktree(props.workspaceName, props.worktreeName, branch);
    }
    if (props.action === Action.CHECKOUT) {
      ctx.checkoutBranch(props.workspaceName, props.worktreeName, branch);
    }
    props.onDone();
  };

  return (
    <div class="pl-4.5 mt-0.5 space-y-1">
      <span class="text-[10px] opacity-60">{ACTION_LABELS[props.action]}</span>
      <Show
        when={
          props.action === Action.REBASE || props.action === Action.CHECKOUT
        }
      >
        <BranchSelect
          workspaceName={props.workspaceName}
          value={selectedBranch()}
          onSelect={setSelectedBranch}
        />
      </Show>
      <Show when={props.action === Action.NEW_BRANCH}>
        <BranchNameInput
          placeholder="Branch name..."
          forbiddenNames={props.existingBranches}
          allowSlash
          onSubmit={(name) => {
            ctx.newBranchOnWorktree(
              props.workspaceName,
              props.worktreeName,
              name,
            );
            props.onDone();
          }}
          onCancel={props.onDone}
        />
      </Show>
      <Show when={props.action !== Action.NEW_BRANCH}>
        <div class="flex justify-end gap-1">
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
            onClick={(e) => {
              e.stopPropagation();
              props.onDone();
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
              confirm();
            }}
          >
            OK
          </button>
        </div>
      </Show>
    </div>
  );
};

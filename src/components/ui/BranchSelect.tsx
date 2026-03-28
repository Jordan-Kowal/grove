import { WorkspaceService } from "@backend";
import {
  type Component,
  createMemo,
  createResource,
  For,
  Show,
} from "solid-js";
import type { BranchInfo } from "@/types/types";

type BranchSelectProps = {
  workspaceName: string;
  value?: string;
  onSelect: (branch: string) => void;
  class?: string;
};

const fetchBranches = async (name: string): Promise<BranchInfo[]> => {
  return WorkspaceService.ListBranches(name);
};

export const BranchSelect: Component<BranchSelectProps> = (props) => {
  const [branches] = createResource(() => props.workspaceName, fetchBranches);

  const localBranches = createMemo(
    () => branches()?.filter((b) => !b.isRemote) ?? [],
  );
  const remoteBranches = createMemo(
    () => branches()?.filter((b) => b.isRemote) ?? [],
  );

  const isLoading = () => branches.loading;

  return (
    <div class="relative">
      <Show when={isLoading()}>
        <span class="loading loading-spinner loading-xs text-info absolute right-6 top-1/2 -translate-y-1/2 z-10" />
      </Show>
      <select
        class={props.class ?? "select select-bordered select-xs w-full text-xs"}
        value={props.value ?? ""}
        disabled={isLoading()}
        onChange={(e) => props.onSelect(e.currentTarget.value)}
      >
        <Show when={isLoading()}>
          <option value="">Loading...</option>
        </Show>
        <Show when={!isLoading()}>
          <Show when={!props.value}>
            <option value="" disabled>
              Select a branch...
            </option>
          </Show>
          <Show when={localBranches().length > 0}>
            <optgroup label="Local">
              <For each={localBranches()}>
                {(b) => <option value={b.name}>{b.name}</option>}
              </For>
            </optgroup>
          </Show>
          <Show when={remoteBranches().length > 0}>
            <optgroup label="Remote">
              <For each={remoteBranches()}>
                {(b) => <option value={b.name}>{b.name}</option>}
              </For>
            </optgroup>
          </Show>
        </Show>
      </select>
    </div>
  );
};

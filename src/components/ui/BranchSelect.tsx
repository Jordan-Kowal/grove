import { WorkspaceService } from "@backend";
import {
  type Component,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  For,
  onCleanup,
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
  const [open, setOpen] = createSignal(false);
  const [query, setQuery] = createSignal("");
  let inputRef: HTMLInputElement | undefined;

  const isLoading = () => branches.loading;

  const filtered = createMemo(() => {
    const all = branches() ?? [];
    const q = query().toLowerCase();
    if (!q) return all;
    return all.filter((b) => b.name.toLowerCase().includes(q));
  });

  const localBranches = createMemo(() => filtered().filter((b) => !b.isRemote));
  const remoteBranches = createMemo(() => filtered().filter((b) => b.isRemote));

  const select = (name: string) => {
    props.onSelect(name);
    setQuery("");
    setOpen(false);
  };

  // Close on outside click
  createEffect(() => {
    if (!open()) return;
    const handler = (e: MouseEvent) => {
      if (
        inputRef &&
        !inputRef.closest(".branch-select")?.contains(e.target as Node)
      ) {
        setOpen(false);
        setQuery("");
      }
    };
    document.addEventListener("mousedown", handler);
    onCleanup(() => document.removeEventListener("mousedown", handler));
  });

  return (
    <div class="branch-select relative">
      <div class="relative">
        <input
          ref={inputRef}
          type="text"
          class={
            props.class ??
            "input input-bordered input-xs w-full text-xs font-mono"
          }
          placeholder={isLoading() ? "Loading..." : "Search branches..."}
          disabled={isLoading()}
          value={open() ? query() : (props.value ?? "")}
          onFocus={() => {
            setOpen(true);
            setQuery("");
          }}
          onInput={(e) => setQuery(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              setOpen(false);
              setQuery("");
              inputRef?.blur();
            }
          }}
        />
        <Show when={isLoading()}>
          <span class="loading loading-spinner loading-xs text-info absolute right-2 top-1/2 -translate-y-1/2" />
        </Show>
      </div>

      <Show when={open() && !isLoading()}>
        <div class="absolute z-50 mt-1 w-full max-h-48 overflow-y-auto bg-base-300 rounded-box shadow-lg p-1">
          <Show
            when={filtered().length > 0}
            fallback={
              <div class="px-2 py-1 text-[10px] opacity-40">
                No matching branches
              </div>
            }
          >
            <Show when={localBranches().length > 0}>
              <div class="px-2 py-0.5 text-[9px] font-semibold uppercase tracking-wider opacity-40">
                Local
              </div>
              <For each={localBranches()}>
                {(b) => (
                  <button
                    type="button"
                    class="w-full text-left px-2 py-1 text-xs font-mono rounded hover:bg-base-200 transition-colors"
                    classList={{
                      "text-info": b.name === props.value,
                    }}
                    onClick={() => select(b.name)}
                  >
                    {b.name}
                  </button>
                )}
              </For>
            </Show>
            <Show when={remoteBranches().length > 0}>
              <div class="px-2 py-0.5 text-[9px] font-semibold uppercase tracking-wider opacity-40 mt-1">
                Remote
              </div>
              <For each={remoteBranches()}>
                {(b) => (
                  <button
                    type="button"
                    class="w-full text-left px-2 py-1 text-xs font-mono rounded hover:bg-base-200 transition-colors"
                    classList={{
                      "text-info": b.name === props.value,
                    }}
                    onClick={() => select(b.name)}
                  >
                    {b.name}
                  </button>
                )}
              </For>
            </Show>
          </Show>
        </div>
      </Show>
    </div>
  );
};

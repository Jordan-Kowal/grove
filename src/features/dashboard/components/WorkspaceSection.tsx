import {
  ChevronDown,
  ChevronRight,
  EllipsisVertical,
  ExternalLink,
  Plus,
  RefreshCw,
  Trash2,
} from "lucide-solid";
import {
  type Component,
  createEffect,
  createSignal,
  For,
  onCleanup,
  Show,
} from "solid-js";
import { BranchNameInput } from "@/components/ui";
import type { Workspace } from "@/types/types";
import { useDashboardContext } from "../contexts";
import { WorktreeCard } from "./WorktreeCard";

type WorkspaceSectionProps = {
  workspace: Workspace;
  onOpenLogs: (worktreeName: string) => void;
};

export const WorkspaceSection: Component<WorkspaceSectionProps> = (props) => {
  const ctx = useDashboardContext();
  const [collapsed, setCollapsed] = createSignal(false);
  const [showMenu, setShowMenu] = createSignal(false);
  const [showAddInput, setShowAddInput] = createSignal(false);
  const [confirmRemove, setConfirmRemove] = createSignal(false);
  const [confirmSync, setConfirmSync] = createSignal(false);

  const name = () => props.workspace.name;

  // Close menu on outside click
  createEffect(() => {
    if (!showMenu()) return;
    const handler = () => setShowMenu(false);
    document.addEventListener("click", handler);
    onCleanup(() => document.removeEventListener("click", handler));
  });

  const worktreeNames = () =>
    (props.workspace.worktrees ?? []).map((wt) => wt.name);

  const existingBranches = () =>
    (props.workspace.worktrees ?? []).map((wt) => wt.branch).filter(Boolean);

  const baseBranch = () => props.workspace.config.baseBranch || "origin/main";

  return (
    <div class="mb-1">
      {/* Section header */}
      <div class="flex items-center justify-between px-2 py-1.5 group">
        <button
          type="button"
          class="flex items-center gap-1 min-w-0"
          onClick={() => setCollapsed(!collapsed())}
        >
          <Show
            when={collapsed()}
            fallback={<ChevronDown size={10} class="opacity-40 shrink-0" />}
          >
            <ChevronRight size={10} class="opacity-40 shrink-0" />
          </Show>
          <span class="text-[10px] font-semibold uppercase tracking-wider opacity-50">
            {name()}
          </span>
        </button>
        <div class="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0"
            onClick={() => ctx.focusEditor(props.workspace.config.repoPath)}
            title="Open in editor"
          >
            <ExternalLink size={12} />
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0"
            onClick={() => setShowAddInput(true)}
            title="Add worktree"
          >
            <Plus size={12} />
          </button>
          <div class="relative">
            <button
              type="button"
              class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0"
              onClick={() => setShowMenu(!showMenu())}
              title="Options"
            >
              <EllipsisVertical size={12} />
            </button>
            <Show when={showMenu()}>
              <div class="absolute right-0 top-full z-50 mt-1">
                <ul class="menu menu-xs bg-base-300 rounded-box shadow-lg w-44 p-1">
                  <li>
                    <button
                      type="button"
                      class="text-xs"
                      onClick={() => {
                        setShowMenu(false);
                        setConfirmSync(true);
                      }}
                    >
                      <RefreshCw size={12} />
                      Sync main checkout
                    </button>
                  </li>
                  <li>
                    <button
                      type="button"
                      class="text-error text-xs"
                      onClick={() => {
                        setShowMenu(false);
                        setConfirmRemove(true);
                      }}
                    >
                      <Trash2 size={12} />
                      Remove workspace
                    </button>
                  </li>
                </ul>
              </div>
            </Show>
          </div>
        </div>
      </div>

      {/* Sync main checkout confirmation */}
      <Show when={confirmSync()}>
        <div class="flex items-center gap-1 px-2 py-1">
          <span class="text-[10px] text-warning flex-1">
            Discard all changes in main checkout?
          </span>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-info opacity-70 hover:opacity-100"
            onClick={() => {
              setConfirmSync(false);
              ctx.syncMainCheckout(name());
            }}
          >
            Yes
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
            onClick={() => setConfirmSync(false)}
          >
            No
          </button>
        </div>
      </Show>

      {/* Remove workspace confirmation */}
      <Show when={confirmRemove()}>
        <div class="flex items-center gap-1 px-2 py-1">
          <span class="text-[10px] text-warning flex-1">Remove workspace?</span>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-error opacity-70 hover:opacity-100"
            onClick={() => {
              setConfirmRemove(false);
              ctx.removeWorkspace(name());
            }}
          >
            Yes
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
            onClick={() => setConfirmRemove(false)}
          >
            No
          </button>
        </div>
      </Show>

      <Show when={!collapsed()}>
        {/* Add worktree input */}
        <Show when={showAddInput()}>
          <div class="px-2 pb-1">
            <BranchNameInput
              placeholder="Worktree name..."
              forbiddenNames={worktreeNames()}
              onSubmit={(wtName) => {
                ctx.createWorktree(name(), wtName);
                setShowAddInput(false);
              }}
              onCancel={() => setShowAddInput(false)}
            />
          </div>
        </Show>

        {/* Worktree list */}
        <div class="space-y-0.5 px-1">
          <Show
            when={props.workspace.worktrees?.length > 0}
            fallback={
              <div class="flex flex-col items-center gap-1 py-3">
                <p class="text-[10px] opacity-30">No worktrees</p>
                <button
                  type="button"
                  class="btn btn-ghost btn-xs opacity-40 text-[10px]"
                  onClick={() => setShowAddInput(true)}
                >
                  <Plus size={10} />
                  Create one
                </button>
              </div>
            }
          >
            <For each={props.workspace.worktrees ?? []}>
              {(wt) => (
                <WorktreeCard
                  workspaceName={name()}
                  worktree={wt}
                  baseBranch={baseBranch()}
                  existingBranches={existingBranches()}
                  hasSetupScript={!!props.workspace.config.setupScript}
                  onOpenLogs={() => props.onOpenLogs(wt.name)}
                />
              )}
            </For>
          </Show>
        </div>
      </Show>
    </div>
  );
};

import {
  ChevronDown,
  ChevronRight,
  EllipsisVertical,
  Plus,
} from "lucide-solid";
import {
  type Component,
  createEffect,
  createSignal,
  For,
  onCleanup,
  Show,
} from "solid-js";
import type { Workspace, WorktreeTaskEvent } from "@/types/types";
import { WorktreeCard } from "./WorktreeCard";

type WorkspaceSectionProps = {
  workspace: Workspace;
  taskStatuses: Record<string, WorktreeTaskEvent>;
  taskStartedAt: Record<string, number>;
  pendingDeletes: Record<string, boolean>;
  onCreateWorktree: (name: string) => void;
  onRemoveWorktree: (name: string) => void;
  onConfirmDelete: (name: string) => void;
  onCancelDelete: (name: string) => void;
  onForceRemoveWorktree: (name: string) => void;
  onRemoveWorkspace: () => void;
  onClickWorktree: (path: string) => void;
  onCancelTask: (worktreeName: string) => void;
  onRetrySetup: (worktreeName: string) => void;
  onRetryArchive: (worktreeName: string) => void;
  onClearTaskStatus: (worktreeName: string) => void;
  onOpenLogs: (worktreeName: string) => void;
  hasLogs: (worktreeName: string) => boolean;
};

export const WorkspaceSection: Component<WorkspaceSectionProps> = (props) => {
  const [collapsed, setCollapsed] = createSignal(false);
  const [showMenu, setShowMenu] = createSignal(false);
  const [showAddInput, setShowAddInput] = createSignal(false);
  const [confirmRemove, setConfirmRemove] = createSignal(false);
  const [newName, setNewName] = createSignal("");

  // Close menu on outside click
  createEffect(() => {
    if (!showMenu()) return;
    const handler = () => setShowMenu(false);
    document.addEventListener("click", handler);
    onCleanup(() => document.removeEventListener("click", handler));
  });

  const isNameTaken = () => {
    const name = newName().trim();
    if (!name) return false;
    const lower = name.toLowerCase();
    return (props.workspace.worktrees ?? []).some(
      (wt) => wt.name.toLowerCase() === lower,
    );
  };

  const handleAdd = () => {
    const name = newName().trim();
    if (name && !isNameTaken()) {
      props.onCreateWorktree(name);
      setNewName("");
      setShowAddInput(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") handleAdd();
    if (e.key === "Escape") {
      setShowAddInput(false);
      setNewName("");
    }
  };

  const getTaskStatus = (worktreeName: string) => {
    const key = `${props.workspace.name}/${worktreeName}`;
    return props.taskStatuses[key];
  };

  const getTaskStartedAt = (worktreeName: string) => {
    const key = `${props.workspace.name}/${worktreeName}`;
    return props.taskStartedAt[key];
  };

  const isDeletePending = (worktreeName: string) => {
    const key = `${props.workspace.name}/${worktreeName}`;
    return props.pendingDeletes[key] ?? false;
  };

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
            {props.workspace.name}
          </span>
        </button>
        <div class="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
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
                <ul class="menu menu-xs bg-base-300 rounded-box shadow-lg w-36 p-1">
                  <li>
                    <button
                      type="button"
                      class="text-error text-xs"
                      onClick={() => {
                        setShowMenu(false);
                        setConfirmRemove(true);
                      }}
                    >
                      Remove workspace
                    </button>
                  </li>
                </ul>
              </div>
            </Show>
          </div>
        </div>
      </div>

      {/* Remove workspace confirmation */}
      <Show when={confirmRemove()}>
        <div class="flex items-center gap-1 px-2 py-1">
          <span class="text-[10px] text-warning flex-1">Remove workspace?</span>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-error opacity-70 hover:opacity-100"
            onClick={() => {
              setConfirmRemove(false);
              props.onRemoveWorkspace();
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
            <input
              ref={(el) => setTimeout(() => el.focus(), 0)}
              type="text"
              class="input input-xs input-bordered w-full text-xs"
              classList={{ "input-error": isNameTaken() }}
              placeholder="Worktree name..."
              value={newName()}
              onInput={(e) => {
                const sanitized = e.currentTarget.value.replace(
                  /[^a-zA-Z0-9\-_]/g,
                  "",
                );
                e.currentTarget.value = sanitized;
                setNewName(sanitized);
              }}
              onKeyDown={handleKeyDown}
            />
            <Show when={isNameTaken()}>
              <p class="text-[9px] text-error mt-0.5">Name already taken</p>
            </Show>
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
                  worktree={wt}
                  deletePending={isDeletePending(wt.name)}
                  hasLogs={props.hasLogs(wt.name)}
                  taskEvent={getTaskStatus(wt.name)}
                  taskStartedAt={getTaskStartedAt(wt.name)}
                  onClick={() => props.onClickWorktree(wt.path)}
                  onRemove={() => props.onRemoveWorktree(wt.name)}
                  onConfirmDelete={() => props.onConfirmDelete(wt.name)}
                  onCancelDelete={() => props.onCancelDelete(wt.name)}
                  onForceRemove={() => props.onForceRemoveWorktree(wt.name)}
                  onCancelTask={() => props.onCancelTask(wt.name)}
                  onRetrySetup={() => props.onRetrySetup(wt.name)}
                  onRetryArchive={() => props.onRetryArchive(wt.name)}
                  onClearTaskStatus={() => props.onClearTaskStatus(wt.name)}
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

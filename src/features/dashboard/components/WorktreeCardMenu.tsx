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
  type Accessor,
  type Component,
  createSignal,
  onCleanup,
  type Setter,
  Show,
} from "solid-js";
import type { WorktreeInfo } from "@/types/types";
import { useDashboardContext } from "../contexts";
import { Action } from "./QuickActionPanel";

type WorktreeCardMenuProps = {
  workspaceName: string;
  worktree: WorktreeInfo;
  isMainRepo: boolean;
  hasSetupScript: boolean;
  showMenu: Accessor<boolean>;
  setShowMenu: Setter<boolean>;
  onOpenAction: (action: Action) => void;
};

export const WorktreeCardMenu: Component<WorktreeCardMenuProps> = (props) => {
  const ctx = useDashboardContext();
  const [copied, setCopied] = createSignal(false);
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(copiedTimer));

  const copyBranch = () => {
    navigator.clipboard.writeText(props.worktree.branch);
    setCopied(true);
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => {
      setCopied(false);
      props.setShowMenu(false);
    }, 1500);
  };

  return (
    <div class="relative shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
      <button
        type="button"
        class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0"
        onClick={(e) => {
          e.stopPropagation();
          props.setShowMenu(!props.showMenu());
        }}
      >
        <EllipsisVertical size={12} />
      </button>
      <Show when={props.showMenu()}>
        <div class="absolute right-0 top-full z-50 mt-1">
          <ul class="menu menu-xs bg-base-300 rounded-box shadow-lg w-48 p-1">
            <li>
              <button
                type="button"
                class="text-xs"
                onClick={(e) => {
                  e.stopPropagation();
                  props.onOpenAction(Action.REBASE);
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
                  props.onOpenAction(Action.CHECKOUT);
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
                  props.onOpenAction(Action.NEW_BRANCH);
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
                <Show when={copied()} fallback={<ClipboardCopy size={12} />}>
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
                    props.setShowMenu(false);
                    ctx.retrySetup(props.workspaceName, props.worktree.name);
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
                  "opacity-30 pointer-events-none": !props.worktree.editorOpen,
                }}
                disabled={!props.worktree.editorOpen}
                onClick={(e) => {
                  e.stopPropagation();
                  props.setShowMenu(false);
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
                    props.setShowMenu(false);
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
  );
};

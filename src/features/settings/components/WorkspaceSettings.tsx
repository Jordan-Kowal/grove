import { WorkspaceService } from "@backend";
import { FolderOpen, GitBranch, Terminal } from "lucide-solid";
import {
  type Component,
  createEffect,
  createResource,
  createSignal,
  on,
  onCleanup,
} from "solid-js";
import { BranchSelect, Section } from "@/components/ui";
import { useDashboardContext } from "@/features/dashboard/contexts";
import type { WorkspaceConfig } from "@/types/types";

type WorkspaceSettingsProps = {
  name: string;
};

const DEBOUNCE_MS = 500;

const applyDefaults = (c: WorkspaceConfig): WorkspaceConfig => ({
  ...c,
  deleteBranch: c.deleteBranch ?? true,
});

const fetchConfig = async (name: string): Promise<WorkspaceConfig> => {
  const raw = await WorkspaceService.GetWorkspaceConfig(name);
  return applyDefaults(raw);
};

export const WorkspaceSettings: Component<WorkspaceSettingsProps> = (props) => {
  const ctx = useDashboardContext();
  const [remoteConfig] = createResource(() => props.name, fetchConfig);
  const [localOverride, setLocalOverride] =
    createSignal<WorkspaceConfig | null>(null);

  // Reset local edits when switching workspaces
  createEffect(
    on(
      () => props.name,
      () => setLocalOverride(null),
    ),
  );

  const config = () =>
    localOverride() ??
    remoteConfig() ?? {
      repoPath: "",
      baseBranch: "",
      setupScript: "",
      archiveScript: "",
      deleteBranch: true,
    };

  const loaded = () => remoteConfig.state === "ready";

  // Auto-save with debounce
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(saveTimer));

  const updateConfig = (patch: Partial<WorkspaceConfig>) => {
    const updated = { ...config(), ...patch };
    setLocalOverride(updated);
    clearTimeout(saveTimer);
    const name = props.name;
    saveTimer = setTimeout(() => {
      ctx.updateWorkspaceConfig(name, updated);
    }, DEBOUNCE_MS);
  };

  const worktreesPath = () => `~/.grove/projects/${props.name}/worktrees`;

  return (
    <div class="space-y-8">
      <h2 class="text-sm font-semibold">{props.name}</h2>

      {/* Paths */}
      <Section title="Paths" icon={<FolderOpen size={12} />}>
        <div class="space-y-1">
          <span class="text-xs font-medium opacity-60">Repository</span>
          <div class="text-xs font-mono opacity-40 bg-base-200 rounded px-2 py-1.5">
            {config().repoPath}
          </div>
        </div>

        <div class="space-y-1">
          <span class="text-xs font-medium opacity-60">
            Worktrees directory
          </span>
          <div class="text-xs font-mono opacity-40 bg-base-200 rounded px-2 py-1.5">
            {worktreesPath()}
          </div>
        </div>
      </Section>

      {/* Git */}
      <Section title="Git" icon={<GitBranch size={12} />}>
        <div class="block space-y-1">
          <span class="text-xs font-medium opacity-60">
            Branch new worktrees from
          </span>
          <BranchSelect
            workspaceName={props.name}
            value={config().baseBranch || "origin/main"}
            onSelect={(branch) => updateConfig({ baseBranch: branch })}
            class="select select-bordered w-full text-xs font-mono"
          />
          <p class="text-[10px] opacity-30">Default: origin/main</p>
        </div>

        <label class="flex items-center justify-between">
          <span class="text-xs font-medium opacity-60">
            Delete local branch when removing worktree
          </span>
          <input
            type="checkbox"
            class="toggle toggle-xs"
            checked={config().deleteBranch}
            disabled={!loaded()}
            onChange={(e) =>
              updateConfig({ deleteBranch: e.currentTarget.checked })
            }
          />
        </label>
      </Section>

      {/* Scripts */}
      <Section title="Scripts" icon={<Terminal size={12} />}>
        <label class="block space-y-1">
          <span class="text-xs font-medium opacity-60">Setup script</span>
          <textarea
            class="textarea textarea-bordered w-full text-xs font-mono leading-relaxed"
            rows={3}
            placeholder="e.g. bun install && go mod tidy"
            value={config().setupScript}
            disabled={!loaded()}
            onInput={(e) =>
              updateConfig({ setupScript: e.currentTarget.value })
            }
          />
          <p class="text-[10px] opacity-30">Runs after creating a worktree</p>
        </label>

        <label class="block space-y-1">
          <span class="text-xs font-medium opacity-60">Teardown script</span>
          <textarea
            class="textarea textarea-bordered w-full text-xs font-mono leading-relaxed"
            rows={3}
            placeholder="e.g. rm -rf node_modules dist bin"
            value={config().archiveScript}
            disabled={!loaded()}
            onInput={(e) =>
              updateConfig({ archiveScript: e.currentTarget.value })
            }
          />
          <p class="text-[10px] opacity-30">Runs before removing a worktree</p>
        </label>
      </Section>
    </div>
  );
};

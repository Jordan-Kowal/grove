import { MonitorService } from "@backend";
import { ArrowLeft } from "lucide-solid";
import {
  type Component,
  createSignal,
  For,
  Match,
  onMount,
  Switch,
} from "solid-js";
import type { Workspace } from "@/types/types";
import { GeneralSettings } from "./components/GeneralSettings";
import { WorkspaceSettings } from "./components/WorkspaceSettings";

type ActiveTab = { kind: "general" } | { kind: "workspace"; name: string };

const GENERAL_TAB: ActiveTab = { kind: "general" };

type TabButtonProps = {
  active: boolean;
  onClick: () => void;
  children: string;
};

const tabClass = (active: boolean) =>
  `w-full text-left px-3 py-1.5 text-xs transition-colors ${
    active ? "bg-base-200 font-medium" : "opacity-50 hover:opacity-80"
  }`;

const TabButton: Component<TabButtonProps> = (props) => (
  <button type="button" class={tabClass(props.active)} onClick={props.onClick}>
    {props.children}
  </button>
);

type SettingsProps = {
  onBack: () => void;
};

export const Settings: Component<SettingsProps> = (props) => {
  const [workspaces, setWorkspaces] = createSignal<Workspace[]>([]);
  const [activeTab, setActiveTab] = createSignal<ActiveTab>(GENERAL_TAB);

  const isGeneral = () => activeTab().kind === "general";
  const workspaceName = () => {
    const tab = activeTab();
    return tab.kind === "workspace" ? tab.name : "";
  };

  onMount(async () => {
    const ws = await MonitorService.GetWorkspaces();
    setWorkspaces(ws);
  });

  return (
    <div class="flex h-screen">
      {/* Sidebar */}
      <div class="w-44 shrink-0 border-r border-base-300 flex flex-col">
        {/* Back button + drag region */}
        <div class="drag-region h-10 flex items-center pl-18 pr-3 border-b border-base-300 shrink-0">
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 no-drag opacity-50 hover:opacity-100"
            onClick={props.onBack}
          >
            <ArrowLeft size={14} />
          </button>
          <span class="text-[10px] font-semibold uppercase tracking-wider opacity-40 ml-2 no-drag">
            Settings
          </span>
        </div>

        {/* Tabs */}
        <div class="flex-1 overflow-y-auto py-2">
          <TabButton
            active={isGeneral()}
            onClick={() => setActiveTab(GENERAL_TAB)}
          >
            General
          </TabButton>

          <div class="mt-3 px-3">
            <span class="text-[10px] font-semibold uppercase tracking-wider opacity-30">
              Workspaces
            </span>
          </div>

          <For each={workspaces()}>
            {(ws) => (
              <TabButton
                active={workspaceName() === ws.name}
                onClick={() =>
                  setActiveTab({ kind: "workspace", name: ws.name })
                }
              >
                {ws.name}
              </TabButton>
            )}
          </For>
        </div>
      </div>

      {/* Content */}
      <div class="flex-1 overflow-y-auto p-6">
        <Switch fallback={<WorkspaceSettings name={workspaceName()} />}>
          <Match when={isGeneral()}>
            <GeneralSettings />
          </Match>
        </Switch>
      </div>
    </div>
  );
};

import { EditorService, SoundService } from "@backend";
import { Bell, Check, Code, Info, Monitor, Play, X } from "lucide-solid";
import {
  type Component,
  createSignal,
  For,
  type JSX,
  onCleanup,
  onMount,
  Show,
} from "solid-js";
import { Theme, useSettingsContext } from "@/contexts";
import { VersionStatus } from "./VersionStatus";

type SectionProps = {
  title: string;
  icon: JSX.Element;
  children: JSX.Element;
};

const Section: Component<SectionProps> = (props) => (
  <div class="space-y-3">
    <h3 class="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider opacity-50">
      {props.icon}
      {props.title}
    </h3>
    <div class="space-y-3">{props.children}</div>
  </div>
);

export const GeneralSettings: Component = () => {
  const { settings, updateSetting } = useSettingsContext();
  const [sounds, setSounds] = createSignal<string[]>([]);
  const [editorValid, setEditorValid] = createSignal<boolean | null>(null);
  let editorTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(editorTimer));

  const checkEditor = async (name: string) => {
    if (!name) return;
    const valid = await EditorService.IsValidApp(name);
    setEditorValid(valid);
  };

  const validateEditorDebounced = (name: string) => {
    setEditorValid(null);
    clearTimeout(editorTimer);
    if (!name) return;
    editorTimer = setTimeout(() => checkEditor(name), 500);
  };

  onMount(async () => {
    const list = await SoundService.GetSounds();
    setSounds(list);
    checkEditor(settings().editorApp);
  });

  const handlePlayPreview = () => {
    SoundService.PlayPreview(settings().soundName);
  };

  return (
    <div class="space-y-8">
      <h2 class="text-sm font-semibold">General</h2>

      {/* Display */}
      <Section title="Display" icon={<Monitor size={12} />}>
        <label class="block space-y-1">
          <span class="text-xs font-medium opacity-60">Theme</span>
          <select
            class="select select-bordered select-sm w-full text-xs"
            value={settings().theme}
            onChange={(e) =>
              updateSetting("theme", e.currentTarget.value as Theme)
            }
          >
            <option value={Theme.NORD}>Nord</option>
            <option value={Theme.FOREST}>Forest</option>
          </select>
        </label>

        <label class="flex items-center justify-between cursor-pointer">
          <span class="text-xs font-medium opacity-60">Keep window on top</span>
          <input
            type="checkbox"
            class="toggle toggle-sm toggle-primary"
            checked={settings().alwaysOnTop}
            onChange={(e) =>
              updateSetting("alwaysOnTop", e.currentTarget.checked)
            }
          />
        </label>

        <label class="flex items-center justify-between cursor-pointer">
          <div>
            <span class="text-xs font-medium opacity-60">Dock to edge</span>
            <p class="text-[10px] opacity-40">
              Snap to screen edge at full height. Opens editors in remaining
              space.
            </p>
          </div>
          <input
            type="checkbox"
            class="toggle toggle-sm toggle-primary"
            checked={settings().snapToEdges}
            onChange={(e) =>
              updateSetting("snapToEdges", e.currentTarget.checked)
            }
          />
        </label>
      </Section>

      {/* Notifications */}
      <Section title="Notifications" icon={<Bell size={12} />}>
        <label class="block space-y-1">
          <span class="text-xs font-medium opacity-60">Play sound</span>
          <select
            class="select select-bordered select-sm w-full text-xs"
            value={settings().soundMode}
            onChange={(e) =>
              updateSetting(
                "soundMode",
                e.currentTarget.value as "never" | "permission" | "all",
              )
            }
          >
            <option value="never">Never</option>
            <option value="all">When done or needs input</option>
            <option value="permission">Only when needs input</option>
          </select>
        </label>

        <Show when={settings().soundMode !== "never"}>
          <div class="flex items-center gap-2">
            <select
              class="select select-bordered select-sm flex-1 text-xs"
              value={settings().soundName}
              onChange={(e) =>
                updateSetting("soundName", e.currentTarget.value)
              }
            >
              <For each={sounds()}>
                {(sound) => <option value={sound}>{sound}</option>}
              </For>
            </select>
            <button
              type="button"
              class="btn btn-ghost btn-sm btn-square"
              onClick={handlePlayPreview}
            >
              <Play size={14} />
            </button>
          </div>
        </Show>

        <label class="flex items-center justify-between cursor-pointer">
          <span class="text-xs font-medium opacity-60">Show menu bar icon</span>
          <input
            type="checkbox"
            class="toggle toggle-sm toggle-primary"
            checked={settings().systemTrayEnabled}
            onChange={(e) =>
              updateSetting("systemTrayEnabled", e.currentTarget.checked)
            }
          />
        </label>
      </Section>

      {/* Editor */}
      <Section title="Editor" icon={<Code size={12} />}>
        <div class="space-y-1">
          <span class="text-xs font-medium opacity-60">Default editor</span>
          <div class="flex items-center gap-2">
            <input
              type="text"
              class="input input-bordered input-sm flex-1 text-xs font-mono"
              value={settings().editorApp}
              onInput={(e) => {
                const value = e.currentTarget.value;
                updateSetting("editorApp", value);
                validateEditorDebounced(value);
              }}
              placeholder="Zed"
            />
            <Show when={editorValid() === null && settings().editorApp}>
              <span class="loading loading-spinner loading-xs shrink-0 opacity-40" />
            </Show>
            <Show when={editorValid() === true}>
              <Check size={14} class="text-success shrink-0" />
            </Show>
            <Show when={editorValid() === false}>
              <X size={14} class="text-error shrink-0" />
            </Show>
          </div>
          <p class="text-[10px] opacity-30">
            macOS app name (e.g. Zed, Visual Studio Code, Cursor)
          </p>
        </div>
      </Section>

      {/* About */}
      <Section title="About" icon={<Info size={12} />}>
        <VersionStatus />
      </Section>
    </div>
  );
};

import { SnapService, SoundService } from "@backend";
import {
  Bell,
  Check,
  Code,
  Info,
  Monitor,
  Play,
  RotateCw,
  X,
} from "lucide-solid";
import {
  type Component,
  createSignal,
  For,
  Match,
  onCleanup,
  onMount,
  Show,
  Switch,
} from "solid-js";
import { Section } from "@/components/ui";
import {
  SoundMode,
  THEMES,
  type Theme,
  useSettingsContext,
  useWarningsContext,
  WarningKey,
} from "@/contexts";
import { VersionStatus } from "./VersionStatus";

const EDITOR_PENDING_DELAY_MS = 500;

export const GeneralSettings: Component = () => {
  const { settings, updateSetting } = useSettingsContext();
  const warnings = useWarningsContext();
  const [sounds, setSounds] = createSignal<string[]>([]);
  const [editorPending, setEditorPending] = createSignal(false);
  let editorPendingTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(editorPendingTimer));

  const markEditorPending = () => {
    clearTimeout(editorPendingTimer);
    setEditorPending(true);
    editorPendingTimer = setTimeout(
      () => setEditorPending(false),
      EDITOR_PENDING_DELAY_MS,
    );
  };

  const editorInvalid = () => warnings.has(WarningKey.EDITOR_NOT_FOUND);
  const editorValid = () =>
    !editorPending() && !!settings().editorApp && !editorInvalid();

  onMount(async () => {
    const list = await SoundService.GetSounds();
    setSounds(list);
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
            <For each={THEMES}>
              {(theme) => (
                <option value={theme}>
                  {theme.charAt(0).toUpperCase() + theme.slice(1)}
                </option>
              )}
            </For>
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

        <div class="relative">
          <Show when={warnings.has(WarningKey.ACCESSIBILITY)}>
            <span class="absolute -left-3 top-1.5 size-1.5 rounded-full bg-error" />
          </Show>
          <label class="flex items-center justify-between cursor-pointer">
            <div class="space-y-0.5">
              <span class="text-xs font-medium opacity-60">Dock to edge</span>
              <p class="text-[10px] opacity-40">
                Snap to screen edge at full height. Opens editors in remaining
                space.
              </p>
              <p class="text-[10px] opacity-40">
                Requires Accessibility permission.{" "}
                <button
                  type="button"
                  class="underline cursor-pointer hover:opacity-100"
                  onClick={() => SnapService.OpenAccessibilitySettings()}
                >
                  Open Settings
                </button>
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

          <div class="mt-1 flex items-center justify-between gap-2 text-[10px]">
            <span class="flex items-center gap-1 opacity-60">
              Permission:
              <Switch>
                <Match when={warnings.isAccessibilityTrusted() === true}>
                  <span class="text-success inline-flex items-center gap-0.5">
                    <Check size={10} /> granted
                  </span>
                </Match>
                <Match when={warnings.isAccessibilityTrusted() === false}>
                  <span class="text-error inline-flex items-center gap-0.5">
                    <X size={10} /> not granted
                  </span>
                </Match>
                <Match when={warnings.isAccessibilityTrusted() === null}>
                  <span class="opacity-60">checking…</span>
                </Match>
              </Switch>
            </span>
            <button
              type="button"
              class="btn btn-ghost btn-xs gap-1 text-[10px] opacity-60 hover:opacity-100"
              onClick={() => warnings.recheckAccessibility()}
              title="Auto-checked every 60 seconds"
            >
              <RotateCw size={10} />
              Recheck
            </button>
          </div>
        </div>
      </Section>

      {/* Notifications */}
      <Section title="Notifications" icon={<Bell size={12} />}>
        <label class="block space-y-1">
          <span class="text-xs font-medium opacity-60">Play sound</span>
          <select
            class="select select-bordered select-sm w-full text-xs"
            value={settings().soundMode}
            onChange={(e) =>
              updateSetting("soundMode", e.currentTarget.value as SoundMode)
            }
          >
            <option value={SoundMode.NEVER}>Never</option>
            <option value={SoundMode.ALL}>When done or needs input</option>
            <option value={SoundMode.PERMISSION}>Only when needs input</option>
          </select>
        </label>

        <Show when={settings().soundMode !== SoundMode.NEVER}>
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

        <label class="block space-y-1">
          <span class="text-xs font-medium opacity-60">
            "Done" badge duration
          </span>
          <select
            class="select select-bordered select-sm w-full text-xs"
            value={settings().doneDuration}
            onChange={(e) =>
              updateSetting("doneDuration", Number(e.currentTarget.value))
            }
          >
            <option value={0}>Instant dismiss</option>
            <option value={1}>1 minute</option>
            <option value={2}>2 minutes</option>
            <option value={3}>3 minutes</option>
            <option value={5}>5 minutes</option>
            <option value={10}>10 minutes</option>
            <option value={15}>15 minutes</option>
            <option value={30}>30 minutes</option>
            <option value={60}>60 minutes</option>
            <option value={-1}>Until clicked</option>
          </select>
        </label>

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
        <div class="relative space-y-1">
          <Show when={warnings.has(WarningKey.EDITOR_NOT_FOUND)}>
            <span class="absolute -left-3 top-1.5 size-1.5 rounded-full bg-error" />
          </Show>
          <span class="text-xs font-medium opacity-60">Default editor</span>
          <div class="flex items-center gap-2">
            <input
              type="text"
              class="input input-bordered input-sm flex-1 text-xs font-mono"
              value={settings().editorApp}
              onInput={(e) => {
                const value = e.currentTarget.value;
                updateSetting("editorApp", value);
                warnings.validateEditor(value);
                markEditorPending();
              }}
              placeholder="Zed"
            />
            <Show when={editorPending() && settings().editorApp}>
              <span class="loading loading-spinner loading-xs shrink-0 opacity-40" />
            </Show>
            <Show when={editorValid()}>
              <Check size={14} class="text-success shrink-0" />
            </Show>
            <Show when={!editorPending() && editorInvalid()}>
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

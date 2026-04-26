import {
  MonitorService,
  SnapService,
  SoundService,
  TrayService,
} from "@backend";
import { Window } from "@wailsio/runtime";
import { createMemo, type JSX, useContext } from "solid-js";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import {
  DEFAULT_SETTINGS,
  type Settings,
  SettingsContext,
  type SettingsContextProps,
} from "./SettingsContext";

const SETTINGS_STORAGE_KEY = "grove-settings";

export const useSettingsContext = (): SettingsContextProps => {
  const context = useContext(SettingsContext);
  if (!context) {
    throw new Error(
      "useSettingsContext must be used within a SettingsProvider",
    );
  }
  return context;
};

export type SettingsProviderProps = {
  children: JSX.Element;
};

/** Push settings to backend services and DOM. */
const syncToBackend = (s: Settings) => {
  document.documentElement.setAttribute("data-theme", s.theme);
  Window.SetAlwaysOnTop(s.alwaysOnTop);
  SoundService.SetPreferences(s.soundMode, s.soundName);
  MonitorService.SetDoneDuration(s.doneDuration);
  MonitorService.SetEditorApp(s.editorApp);
  MonitorService.SetEditorTrackingEnabled(s.editorTrackingEnabled);
  SnapService.SetEnabled(s.snapToEdges);
  if (s.snapToEdges) SnapService.SnapNow();
  TrayService.SetEnabled(s.systemTrayEnabled);
};

const parseSettings = (raw: string | undefined): Settings => {
  if (!raw) return { ...DEFAULT_SETTINGS };
  try {
    return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) };
  } catch {
    return { ...DEFAULT_SETTINGS };
  }
};

export const SettingsProvider = (props: SettingsProviderProps) => {
  const [storageValue, setStorageValue] = useLocalStorage(SETTINGS_STORAGE_KEY);

  const settings = createMemo((): Settings => parseSettings(storageValue()));

  // Sync initial settings to backend on mount
  syncToBackend(settings());

  const updateSetting = <K extends keyof Settings>(
    key: K,
    value: Settings[K],
  ) => {
    const updated = { ...settings(), [key]: value };
    setStorageValue(JSON.stringify(updated));
    syncToBackend(updated);
  };

  const resetSettings = () => {
    setStorageValue(JSON.stringify(DEFAULT_SETTINGS));
    syncToBackend(DEFAULT_SETTINGS);
  };

  const contextValue: SettingsContextProps = {
    settings,
    updateSetting,
    resetSettings,
  };

  return (
    <SettingsContext.Provider value={contextValue}>
      {props.children}
    </SettingsContext.Provider>
  );
};

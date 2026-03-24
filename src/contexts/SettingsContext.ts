import { createContext } from "solid-js";

export enum Theme {
  NORD = "nord",
  FOREST = "forest",
}

export type Settings = {
  theme: Theme;
  alwaysOnTop: boolean;
  snapToEdges: boolean;
  soundMode: "never" | "permission" | "all";
  soundName: string;
  systemTrayEnabled: boolean;
  editorApp: string;
};

export const DEFAULT_SETTINGS: Settings = {
  theme: Theme.NORD,
  alwaysOnTop: true,
  snapToEdges: true,
  soundMode: "all",
  soundName: "Glass",
  systemTrayEnabled: false,
  editorApp: "Zed",
};

export type SettingsContextProps = {
  settings: () => Settings;
  updateSetting: <K extends keyof Settings>(key: K, value: Settings[K]) => void;
  resetSettings: () => void;
};

export const SettingsContext = createContext<SettingsContextProps>();

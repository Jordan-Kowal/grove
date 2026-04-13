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
  doneDuration: number; // minutes; 0 = instant, -1 = persist until clicked
  systemTrayEnabled: boolean;
  editorApp: string;
};

export const DEFAULT_SETTINGS: Settings = {
  theme: Theme.FOREST,
  alwaysOnTop: true,
  snapToEdges: true,
  soundMode: "all",
  soundName: "Glass",
  doneDuration: 30,
  systemTrayEnabled: false,
  editorApp: "Zed",
};

export type SettingsContextProps = {
  settings: () => Settings;
  updateSetting: <K extends keyof Settings>(key: K, value: Settings[K]) => void;
  resetSettings: () => void;
};

export const SettingsContext = createContext<SettingsContextProps>();

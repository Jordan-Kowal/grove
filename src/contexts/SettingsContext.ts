import { createContext } from "solid-js";

export const THEMES = [
  "abyss",
  "acid",
  "aqua",
  "autumn",
  "black",
  "bumblebee",
  "business",
  "caramellatte",
  "cmyk",
  "coffee",
  "corporate",
  "cupcake",
  "cyberpunk",
  "dark",
  "dim",
  "dracula",
  "emerald",
  "fantasy",
  "forest",
  "garden",
  "halloween",
  "lemonade",
  "light",
  "lofi",
  "luxury",
  "night",
  "nord",
  "pastel",
  "retro",
  "silk",
  "sunset",
  "synthwave",
  "valentine",
  "winter",
  "wireframe",
] as const;

export type Theme = (typeof THEMES)[number];

export enum SoundMode {
  NEVER = "never",
  PERMISSION = "permission",
  ALL = "all",
}

export type Settings = {
  theme: Theme;
  alwaysOnTop: boolean;
  snapToEdges: boolean;
  soundMode: SoundMode;
  soundName: string;
  doneDuration: number; // minutes; 0 = instant, -1 = persist until clicked
  systemTrayEnabled: boolean;
  editorApp: string;
};

export const DEFAULT_SETTINGS: Settings = {
  theme: "forest",
  alwaysOnTop: true,
  snapToEdges: true,
  soundMode: SoundMode.ALL,
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

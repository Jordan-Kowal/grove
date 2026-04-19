import { createContext } from "solid-js";

export enum WarningKey {
  ACCESSIBILITY = "accessibility",
  EDITOR_NOT_FOUND = "editorNotFound",
}

export enum WarningScope {
  GENERAL = "general",
}

const WARNING_SCOPES: Record<WarningKey, WarningScope> = {
  [WarningKey.ACCESSIBILITY]: WarningScope.GENERAL,
  [WarningKey.EDITOR_NOT_FOUND]: WarningScope.GENERAL,
};

export const scopeFor = (key: WarningKey): WarningScope => WARNING_SCOPES[key];

export type WarningsContextProps = {
  hasAny: () => boolean;
  hasForScope: (scope: WarningScope) => boolean;
  has: (key: WarningKey) => boolean;
  setWarning: (key: WarningKey, active: boolean) => void;
  recheckAccessibility: () => void;
  validateEditor: (name: string) => void;
  // Latest known permission state: null = not checked yet, true/false = confirmed.
  isAccessibilityTrusted: () => boolean | null;
};

export const WarningsContext = createContext<WarningsContextProps>();

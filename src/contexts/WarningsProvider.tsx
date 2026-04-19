import { AppService, EditorService } from "@backend";
import {
  createMemo,
  createSignal,
  type JSX,
  onCleanup,
  useContext,
} from "solid-js";
import { useSettingsContext } from "./SettingsProvider";
import {
  scopeFor,
  WarningKey,
  type WarningScope,
  WarningsContext,
  type WarningsContextProps,
} from "./WarningsContext";

const ACCESSIBILITY_POLL_INTERVAL_MS = 60_000;
const EDITOR_DEBOUNCE_MS = 500;

export const useWarningsContext = (): WarningsContextProps => {
  const context = useContext(WarningsContext);
  if (!context) {
    throw new Error(
      "useWarningsContext must be used within a WarningsProvider",
    );
  }
  return context;
};

export type WarningsProviderProps = {
  children: JSX.Element;
};

export const WarningsProvider = (props: WarningsProviderProps) => {
  const { settings } = useSettingsContext();
  const [warnings, setWarnings] = createSignal<Set<WarningKey>>(new Set());
  const [accessibilityTrusted, setAccessibilityTrusted] = createSignal<
    boolean | null
  >(null);

  // Accessibility warning is derived from (trusted, snapToEdges): warn only
  // when Dock-to-edge is enabled AND the permission is explicitly denied.
  const accessibilityWarning = createMemo(
    () => settings().snapToEdges && accessibilityTrusted() === false,
  );

  const setWarning = (key: WarningKey, active: boolean) => {
    setWarnings((prev) => {
      const has = prev.has(key);
      if (active === has) return prev;
      const next = new Set(prev);
      if (active) next.add(key);
      else next.delete(key);
      return next;
    });
  };

  const has = (key: WarningKey) => {
    if (key === WarningKey.ACCESSIBILITY) return accessibilityWarning();
    return warnings().has(key);
  };
  const hasAny = () => accessibilityWarning() || warnings().size > 0;
  const hasForScope = (scope: WarningScope) => {
    if (
      accessibilityWarning() &&
      scopeFor(WarningKey.ACCESSIBILITY) === scope
    ) {
      return true;
    }
    for (const key of warnings()) {
      if (scopeFor(key) === scope) return true;
    }
    return false;
  };

  // Poll AXIsProcessTrusted() continuously so the settings UI can always show
  // the current permission state. The ACCESSIBILITY warning only fires when
  // Dock-to-edge is enabled — that's the only feature that needs the permission.
  let accessibilityInterval: ReturnType<typeof setInterval> | undefined;

  const runAccessibilityCheck = async () => {
    try {
      const trusted = await AppService.IsAccessibilityTrusted();
      setAccessibilityTrusted(trusted);
    } catch (e) {
      console.error("[grove] IsAccessibilityTrusted failed:", e);
      setAccessibilityTrusted(false);
    }
  };

  const recheckAccessibility = () => {
    runAccessibilityCheck();
  };

  runAccessibilityCheck();
  accessibilityInterval = setInterval(
    runAccessibilityCheck,
    ACCESSIBILITY_POLL_INTERVAL_MS,
  );

  onCleanup(() => {
    if (accessibilityInterval !== undefined) {
      clearInterval(accessibilityInterval);
    }
  });

  // Editor producer: caller invokes `validateEditor` from the input handler;
  // we debounce and guard against out-of-order resolutions via a request token.
  let editorTimer: ReturnType<typeof setTimeout> | undefined;
  let editorRequestId = 0;

  const runEditorCheck = async (requestId: number, name: string) => {
    try {
      const valid = await EditorService.IsValidApp(name);
      if (requestId !== editorRequestId) return;
      setWarning(WarningKey.EDITOR_NOT_FOUND, !valid);
    } catch (e) {
      console.error("[grove] IsValidApp failed:", e);
      if (requestId !== editorRequestId) return;
      setWarning(WarningKey.EDITOR_NOT_FOUND, true);
    }
  };

  const validateEditor = (name: string) => {
    clearTimeout(editorTimer);
    editorRequestId += 1;
    const requestId = editorRequestId;
    if (!name) {
      setWarning(WarningKey.EDITOR_NOT_FOUND, false);
      return;
    }
    editorTimer = setTimeout(
      () => runEditorCheck(requestId, name),
      EDITOR_DEBOUNCE_MS,
    );
  };

  // Initial check on mount for the persisted editorApp value.
  validateEditor(settings().editorApp);

  onCleanup(() => clearTimeout(editorTimer));

  const contextValue: WarningsContextProps = {
    hasAny,
    hasForScope,
    has,
    setWarning,
    recheckAccessibility,
    validateEditor,
    isAccessibilityTrusted: accessibilityTrusted,
  };

  return (
    <WarningsContext.Provider value={contextValue}>
      {props.children}
    </WarningsContext.Provider>
  );
};

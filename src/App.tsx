import { Window } from "@wailsio/runtime";
import {
  type Component,
  createSignal,
  ErrorBoundary,
  Match,
  Switch,
} from "solid-js";
import {
  SettingsProvider,
  VersionProvider,
  WarningsProvider,
} from "./contexts";
import { Dashboard, ErrorLog } from "./features/dashboard";
import { DashboardProvider } from "./features/dashboard/contexts";
import { Settings } from "./features/settings";

enum View {
  DASHBOARD = "dashboard",
  SETTINGS = "settings",
  LOGS = "logs",
}

const DASHBOARD_WIDTH = 250;
const DASHBOARD_HEIGHT = 800;
const EXPANDED_WIDTH = 700;

const AppContent: Component = () => {
  const [view, setView] = createSignal<View>(View.DASHBOARD);
  const [logKey, setLogKey] = createSignal("");
  let dashboardSize = { width: DASHBOARD_WIDTH, height: DASHBOARD_HEIGHT };

  const navigate = async (target: View) => {
    if (view() === View.DASHBOARD) {
      const size = await Window.Size();
      dashboardSize = { width: size.width, height: size.height };
    }
    setView(target);
    if (target === View.DASHBOARD) {
      Window.SetSize(dashboardSize.width, dashboardSize.height);
    } else {
      Window.SetSize(EXPANDED_WIDTH, dashboardSize.height);
    }
  };

  const openLogs = (key: string) => {
    setLogKey(key);
    navigate(View.LOGS);
  };

  return (
    <DashboardProvider>
      <Switch>
        <Match when={view() === View.DASHBOARD}>
          <Dashboard
            onOpenSettings={() => navigate(View.SETTINGS)}
            onOpenLogs={openLogs}
          />
        </Match>
        <Match when={view() === View.SETTINGS}>
          <Settings onBack={() => navigate(View.DASHBOARD)} />
        </Match>
        <Match when={view() === View.LOGS}>
          <ErrorLog logKey={logKey()} onBack={() => navigate(View.DASHBOARD)} />
        </Match>
      </Switch>
    </DashboardProvider>
  );
};

export const App: Component = () => {
  return (
    <main class="min-h-screen bg-base-100 text-base-content">
      <SettingsProvider>
        <WarningsProvider>
          <VersionProvider>
            <ErrorBoundary
              fallback={(error) => (
                <div class="p-4 text-error">
                  <p>Something went wrong:</p>
                  <pre class="text-sm">{error.message}</pre>
                </div>
              )}
            >
              <AppContent />
            </ErrorBoundary>
          </VersionProvider>
        </WarningsProvider>
      </SettingsProvider>
    </main>
  );
};

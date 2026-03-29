import { AppService } from "@backend";
import { AlertCircle, Check, Download } from "lucide-solid";
import { type Component, Match, Switch } from "solid-js";
import { useVersionContext } from "@/contexts";
import { getCurrentVersion } from "@/utils/versionCheck";

export const VersionStatus: Component = () => {
  const { latestVersion, checkFailed, checked, isUpdateAvailable } =
    useVersionContext();

  return (
    <Switch>
      <Match when={!checked()}>
        <div class="flex items-center gap-3 rounded bg-base-200 px-3 py-2">
          <span class="loading loading-spinner loading-xs" />
          <span class="text-xs opacity-60">Checking for updates...</span>
        </div>
      </Match>
      <Match when={checkFailed()}>
        <div class="flex items-center gap-3 rounded bg-base-200 px-3 py-2">
          <AlertCircle size={14} class="shrink-0 text-warning" />
          <span class="text-xs opacity-60">
            v{getCurrentVersion()} — update check failed
          </span>
        </div>
      </Match>
      <Match when={isUpdateAvailable()}>
        <div class="flex items-center gap-3 rounded bg-base-200 px-3 py-2">
          <Download size={14} class="shrink-0 text-info" />
          <span class="text-xs flex-1">v{latestVersion()} available</span>
          <button
            type="button"
            class="btn btn-primary btn-xs"
            onClick={() => AppService.InstallUpdate(latestVersion()!)}
          >
            Update
          </button>
        </div>
      </Match>
      <Match when={checked()}>
        <div class="flex items-center gap-3 rounded bg-base-200 px-3 py-2">
          <Check size={14} class="shrink-0 text-success" />
          <span class="text-xs opacity-60">
            v{getCurrentVersion()} — up to date
          </span>
        </div>
      </Match>
    </Switch>
  );
};

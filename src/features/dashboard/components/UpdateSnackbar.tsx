import { AppService } from "@backend";
import { type Component, createSignal, Show } from "solid-js";
import { useVersionContext } from "@/contexts";

export const UpdateSnackbar: Component = () => {
  const { latestVersion, isUpdateAvailable } = useVersionContext();
  const [dismissed, setDismissed] = createSignal(false);

  const visible = () => isUpdateAvailable() && !dismissed();

  return (
    <Show when={visible()}>
      <div class="fixed bottom-3 left-3 right-3 z-50">
        <div class="rounded-box bg-base-200 border border-base-300 py-2 px-3 flex flex-col gap-2 shadow-lg">
          <span class="text-xs">
            New version <span class="font-bold">{latestVersion()}</span>{" "}
            available
          </span>
          <div class="flex gap-2 justify-end">
            <button
              type="button"
              class="btn btn-ghost btn-xs"
              onClick={() => setDismissed(true)}
            >
              Dismiss
            </button>
            <button
              type="button"
              class="btn btn-primary btn-xs"
              onClick={() => AppService.InstallUpdate(latestVersion()!)}
            >
              Update
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
};

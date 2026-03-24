import { Trash2 } from "lucide-solid";
import { type Component, createSignal, Show } from "solid-js";
import type { TmpUsage } from "@/types/types";

type TmpMonitorProps = {
  tmpUsage: TmpUsage;
  onNuke: () => void;
};

export const TmpMonitor: Component<TmpMonitorProps> = (props) => {
  const [confirming, setConfirming] = createSignal(false);

  const handleConfirm = () => {
    setConfirming(false);
    props.onNuke();
  };

  return (
    <div class="flex items-center justify-between px-3 py-2 bg-base-200 rounded-box">
      <div class="text-xs">
        <span class="opacity-60">/tmp</span>{" "}
        <span class="font-mono">{props.tmpUsage.sizeFormatted}</span>
      </div>
      <Show
        when={confirming()}
        fallback={
          <button
            type="button"
            class="btn btn-ghost btn-xs"
            onClick={() => setConfirming(true)}
            title="Delete /tmp Claude files"
          >
            <Trash2 size={14} />
          </button>
        }
      >
        <div class="flex items-center gap-1">
          <span class="text-[10px] text-warning">Clean?</span>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] text-error opacity-70 hover:opacity-100"
            onClick={handleConfirm}
          >
            Yes
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-xs p-0.5 h-auto min-h-0 text-[10px] opacity-50 hover:opacity-100"
            onClick={() => setConfirming(false)}
          >
            No
          </button>
        </div>
      </Show>
    </div>
  );
};

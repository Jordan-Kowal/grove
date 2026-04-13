import { type Accessor, createEffect, onCleanup } from "solid-js";

export const useOutsideClick = (
  active: Accessor<boolean>,
  onClose: () => void,
  event: "click" | "mousedown" = "click",
): void => {
  createEffect(() => {
    if (!active()) return;
    const handler = () => onClose();
    document.addEventListener(event, handler);
    onCleanup(() => document.removeEventListener(event, handler));
  });
};

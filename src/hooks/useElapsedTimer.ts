import { type Accessor, createEffect, createSignal, onCleanup } from "solid-js";

export const useElapsedTimer = (
  active: Accessor<boolean>,
  startedAt: Accessor<number | undefined>,
): Accessor<number> => {
  const [elapsed, setElapsed] = createSignal(0);

  createEffect(() => {
    if (!active() || !startedAt()) return;
    const start = startedAt()!;
    const tick = () => setElapsed(Math.floor((Date.now() - start) / 1000));
    tick();
    const interval = setInterval(tick, 1000);
    onCleanup(() => clearInterval(interval));
  });

  return elapsed;
};

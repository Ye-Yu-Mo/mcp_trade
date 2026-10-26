import { useEffect, useRef, useCallback } from "react";

export function usePolling(
  fn: () => void | Promise<void>,
  intervalMs: number,
  enabled = true,
) {
  const savedFn = useRef(fn);
  savedFn.current = fn;
  const running = useRef(false);

  const tick = useCallback(async () => {
    if (running.current) return;
    running.current = true;
    try {
      await savedFn.current();
    } finally {
      running.current = false;
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;
    void tick(); // initial fetch
    const id = setInterval(() => void tick(), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, tick, enabled]);
}

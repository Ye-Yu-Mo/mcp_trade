import { useEffect, useRef, useCallback } from "react";

export function usePolling(fn: () => void, intervalMs: number) {
  const savedFn = useRef(fn);
  savedFn.current = fn;

  const tick = useCallback(() => savedFn.current(), []);

  useEffect(() => {
    tick(); // initial fetch
    const id = setInterval(tick, intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, tick]);
}

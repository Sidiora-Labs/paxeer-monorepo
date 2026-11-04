"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/** A selection and request generation own all data, error and loading writes. */
export function usePerfdbSelection<T>(key: string | null, empty: T) {
  const selection = useRef({ key, generation: 0, request: 0, mounted: true });
  if (selection.current.key !== key) {
    selection.current = {
      key,
      generation: selection.current.generation + 1,
      request: 0,
      mounted: true,
    };
  }
  const [state, setState] = useState({ key, value: empty, isLoading: true, error: null as Error | null });
  useEffect(() => {
    selection.current.mounted = true;
    return () => {
      selection.current.mounted = false;
      selection.current.generation++;
    };
  }, [key]);

  const capture = useCallback(() => {
    const { key: selected, generation } = selection.current;
    return () => key === selected && selection.current.mounted && selection.current.key === selected && selection.current.generation === generation;
  }, [key]);

  const run = useCallback(async (load: () => Promise<T>, initial: T) => {
    if (selection.current.key !== key || !selection.current.mounted) return;
    const selected = key;
    const generation = selection.current.generation;
    const request = ++selection.current.request;
    const current = () => request === selection.current.request && selection.current.mounted && selection.current.key === selected && selection.current.generation === generation;
    setState({ key: selected, value: initial, isLoading: true, error: null });
    try {
      const value = await load();
      if (current()) setState({ key: selected, value, isLoading: false, error: null });
    } catch (cause) {
      if (current()) setState({ key: selected, value: initial, isLoading: false, error: cause instanceof Error ? cause : new Error(String(cause)) });
    }
  }, [key]);

  return {
    ...(state.key === key ? state : { key, value: empty, isLoading: true, error: null }),
    run,
    capture,
  };
}

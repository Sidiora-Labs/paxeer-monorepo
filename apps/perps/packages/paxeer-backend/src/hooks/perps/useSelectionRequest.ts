"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

/** A read belongs to one selection and one request, including its error/finally. */
export function useSelectionRequest<T>(
  key: string | undefined,
  load: () => Promise<T>,
  intervalMs?: number,
) {
  const generation = useMemo(() => ({}), [key, load]);
  const scope = useRef<{
    generation: object;
    active: boolean;
    request: number;
  }>();
  const [state, setState] = useState<{
    generation: object;
    data: T | undefined;
    isLoading: boolean;
    error: Error | null;
  }>({ generation, data: undefined, isLoading: Boolean(key), error: null });

  const refetch = useCallback(async () => {
    const current = scope.current;
    if (!key || !current?.active || current.generation !== generation) return;
    const request = ++current.request;
    const matches = () => current.active && current.request === request;
    setState((previous) => ({
      generation,
      data: previous.generation === generation ? previous.data : undefined,
      isLoading: true,
      error: null,
    }));
    try {
      const data = await load();
      if (matches()) {
        setState({ generation, data, isLoading: false, error: null });
      }
    } catch (error) {
      if (matches()) {
        setState((previous) => ({
          ...previous,
          isLoading: false,
          error: error instanceof Error ? error : new Error(String(error)),
        }));
      }
    }
  }, [key, generation, load]);

  useEffect(() => {
    const current = { generation, active: true, request: 0 };
    scope.current = current;
    setState({
      generation,
      data: undefined,
      isLoading: Boolean(key),
      error: null,
    });
    void refetch();
    const timer =
      key && intervalMs
        ? setInterval(() => void refetch(), intervalMs)
        : undefined;
    return () => {
      current.active = false;
      clearInterval(timer);
    };
  }, [generation, key, refetch, intervalMs]);

  const visible = key && state.generation === generation;
  return {
    data: visible ? state.data : undefined,
    isLoading: Boolean(key) && (!visible || state.isLoading),
    error: visible ? state.error : null,
    refetch,
  };
}

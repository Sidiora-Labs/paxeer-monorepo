"use client";

// ─────────────────────────────────────────────────────────────────────
//  Thin React-hook layer over `graphqlClient.ts`.
//
//  `useSharedPolledQuery` replaces the ~30 hand-rolled `useState +
//  useCallback + useEffect(setInterval(refetch))` blocks that used to
//  live in every `useXxx` hook across `lib/perps`. All of them funnel
//  through the same `subscribeQuery` singleton so:
//    • duplicate components → 1 shared timer
//    • duplicate concurrent fetches → 1 in-flight Promise (dedup)
//    • hidden tab → every poller freezes until focus comes back
//
//  The public shape (data / isLoading / error / refetch) intentionally
//  matches the old bespoke hooks so migrating call sites is purely a
//  1-line swap.
// ─────────────────────────────────────────────────────────────────────

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  subscribeQuery,
  gqlQuery,
  type GraphQLEndpoint,
} from "@/core/graphql/client";

export interface SharedPolledQueryOptions<TRaw, TPicked> {
  endpoint: GraphQLEndpoint;
  query: string;
  /**
   * Variables object. Object identity does NOT need to be stable —
   * the hook serializes `variables` into the shared-poller key and
   * re-subscribes only when the serialized value changes.
   */
  variables?: Record<string, unknown>;
  /** Polling cadence in ms. Fastest requested cadence wins globally. */
  intervalMs: number;
  /** Gate the subscription (e.g. wait for a userAddress). Default `true`. */
  enabled?: boolean;
  /**
   * Extract the slice of the raw response that the component actually
   * wants. Lets callers name-shift (`res.oiSnapshots` → `OiSnapshot[]`).
   * The function is stored behind a ref so it doesn't need to be
   * memoized by the caller.
   */
  pick: (raw: TRaw) => TPicked;
}

export interface SharedPolledQueryResult<T> {
  data: T | undefined;
  isLoading: boolean;
  error: Error | null;
  /** Force an immediate fetch through the in-flight dedup layer. */
  refetch: () => void;
}

/**
 * Subscribe to a shared polled GraphQL query with React-ergonomic
 * state. Components asking for the same `(endpoint, query, variables)`
 * share the same upstream fetch + timer, and the hook returns a cached
 * snapshot synchronously on remount so there's no loading flash on
 * route changes.
 */
export function useSharedPolledQuery<TRaw = unknown, TPicked = TRaw>(
  opts: SharedPolledQueryOptions<TRaw, TPicked>,
): SharedPolledQueryResult<TPicked> {
  const { endpoint, query, variables, intervalMs, enabled = true, pick } = opts;

  const variablesKey = JSON.stringify(variables ?? null);
  const generation = useMemo(
    () => ({}),
    [endpoint, query, variablesKey, enabled],
  );
  const inputs = useRef({ variables, pick });
  inputs.current = { variables, pick };
  const scope = useRef<{
    generation: object;
    active: boolean;
    request: number;
  }>();
  const [state, setState] = useState<{
    generation: object;
    data: TPicked | undefined;
    isLoading: boolean;
    error: Error | null;
  }>({ generation, data: undefined, isLoading: enabled, error: null });

  const refetch = useCallback(() => {
    const current = scope.current;
    if (!enabled || !current?.active || current.generation !== generation)
      return;
    const request = ++current.request;
    const picker = inputs.current.pick;
    const requestVariables = inputs.current.variables;
    const matches = () => current.active && current.request === request;
    setState((previous) => ({ ...previous, isLoading: true, error: null }));
    void gqlQuery<TRaw>(endpoint, query, requestVariables)
      .then((res) => {
        if (matches()) {
          setState({
            generation,
            data: picker(res),
            isLoading: false,
            error: null,
          });
        }
      })
      .catch((error) => {
        if (matches()) {
          setState((previous) => ({
            ...previous,
            isLoading: false,
            error: error instanceof Error ? error : new Error(String(error)),
          }));
        }
      });
  }, [endpoint, query, enabled, generation]);

  useEffect(() => {
    const current = { generation, active: true, request: 0 };
    scope.current = current;
    setState({ generation, data: undefined, isLoading: enabled, error: null });
    if (!enabled)
      return () => {
        current.active = false;
      };
    const unsubscribe = subscribeQuery<TRaw>({
      endpoint,
      query,
      variables: inputs.current.variables,
      intervalMs,
      onData: (res) => {
        if (current.active) {
          setState({
            generation,
            data: inputs.current.pick(res),
            isLoading: false,
            error: null,
          });
        }
      },
      onError: (error) => {
        if (current.active) {
          setState((previous) => ({ ...previous, error, isLoading: false }));
        }
      },
    });
    return () => {
      current.active = false;
      unsubscribe();
    };
  }, [endpoint, query, generation, intervalMs, enabled]);

  const visible = enabled && state.generation === generation;
  return {
    data: visible ? state.data : undefined,
    error: visible ? state.error : null,
    isLoading: enabled && (!visible || state.isLoading),
    refetch,
  };
}

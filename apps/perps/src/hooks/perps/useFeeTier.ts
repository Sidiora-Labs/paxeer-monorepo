"use client";

import { useEffect, useState } from "react";
import { isAddress } from "viem";

import { buildGetFeeTier } from "@/core/perps/calls";
import { feeTierView, type FeeTierView } from "@/core/perps/feeTier";
import { readContract } from "./useContract";

interface FeeTierResult {
  address: string | undefined;
  view: FeeTierView | undefined;
  error: Error | null;
}

export function useFeeTier(address: string | undefined): {
  view: FeeTierView | undefined;
  isLoading: boolean;
  error: Error | null;
} {
  const [result, setResult] = useState<FeeTierResult>({
    address: undefined,
    view: undefined,
    error: null,
  });

  useEffect(() => {
    if (!address) {
      setResult({ address: undefined, view: undefined, error: null });
      return;
    }
    let cancelled = false;
    setResult({ address, view: undefined, error: null });

    async function load() {
      try {
        if (!address || !isAddress(address)) throw new Error("Invalid account address");
        const decoded = (await readContract(buildGetFeeTier(address))) as Parameters<
          typeof feeTierView
        >[0];
        const view = feeTierView(decoded);
        if (!cancelled) setResult({ address, view, error: null });
      } catch (err: unknown) {
        if (!cancelled) {
          setResult({
            address,
            view: undefined,
            error: err instanceof Error ? err : new Error(String(err)),
          });
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [address]);

  const current = address && result.address === address ? result : undefined;
  return {
    view: current?.view,
    isLoading: Boolean(address && !current?.view && !current?.error),
    error: current?.error ?? null,
  };
}

"use client";

import { useEffect, useState } from "react";
import type { Address } from "viem";

import { classifyError } from "@/core/errors/classify";
import { reportError } from "@/core/errors/report";
import { activePoll } from "@/core/net/realtime";
import { buildGetTokenInfo, buildIsTokenPaused } from "@/core/spot/calls";
import {
  spotTokenState,
  type SpotTokenState,
  type VaultTokenInfo,
} from "@/core/spot/tokenState";
import { readContract } from "@/hooks/perps/useContract";
import { getPaxPointer } from "./useSpotOrders";

export function useSpotTokenState(token: Address, native = false) {
  const [snapshot, setSnapshot] = useState<{
    token: Address;
    native: boolean;
    state: SpotTokenState | null;
    error: string | null;
  } | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSnapshot(null);
    const stop = activePoll(async () => {
      try {
        const address = native ? await getPaxPointer() : token;
        const [paused, info] = await Promise.all([
          readContract(buildIsTokenPaused(address)),
          readContract(buildGetTokenInfo(address)),
        ]);
        const state = spotTokenState(paused as boolean, info as VaultTokenInfo);
        if (!cancelled) setSnapshot({ token, native, state, error: null });
      } catch (err: unknown) {
        if (cancelled) return;
        const error = classifyError(err, "spot-order/token-state");
        setSnapshot({ token, native, state: null, error: error.message });
        void reportError(error);
        throw err;
      }
    }, 8_000);
    return () => {
      cancelled = true;
      stop();
    };
  }, [token, native]);

  const current =
    snapshot?.token === token && snapshot.native === native ? snapshot : null;
  return { state: current?.state ?? null, error: current?.error ?? null };
}

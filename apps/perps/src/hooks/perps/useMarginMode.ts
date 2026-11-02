"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { buildGetAccountSummary, buildGetPositionMargin, buildTopUpPosition } from "@/core/perps/calls";
import { PERPS_SPENDER } from "@/core/network/approvals";
import { COLLATERAL_TOKENS } from "@/core/perps/config";
import { ensureAllowance } from "@/hooks/spot/useSpotApprovals";
import { readContract, writeContract, waitForTx } from "./useContract";
import type { PositionMargin, AccountSummary, TopUpPositionArgs } from "@/types/perps";

export function usePositionMargin(
  userAddress: `0x${string}` | undefined,
  positionId: bigint | undefined,
): {
  data: PositionMargin | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
} {
  const [data, setData] = useState<PositionMargin | undefined>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const mountedRef = useRef(true);

  const refetch = useCallback(async () => {
    if (!userAddress || positionId === undefined) return;
    setIsLoading(true);
    setError(null);
    try {
      const result = (await readContract(buildGetPositionMargin(positionId))) as PositionMargin;

      if (mountedRef.current) {
        setData({
          token: result.token,
          lockedAmount: result.lockedAmount,
          marketId: result.marketId,
          isLong: result.isLong,
        });
      }
    } catch (e) {
      if (mountedRef.current) setError(e instanceof Error ? e : new Error(String(e)));
    } finally {
      if (mountedRef.current) setIsLoading(false);
    }
  }, [userAddress, positionId]);

  useEffect(() => {
    mountedRef.current = true;
    refetch();
    return () => {
      mountedRef.current = false;
    };
  }, [refetch]);

  return { data, isLoading, error, refetch };
}

export function useTopUpPosition(userAddress: `0x${string}` | undefined): {
  execute: (args: TopUpPositionArgs) => Promise<void>;
  isPending: boolean;
  error: Error | null;
} {
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const pendingRef = useRef(false);

  const execute = useCallback(
    async (args: TopUpPositionArgs) => {
      if (!userAddress) throw new Error("Wallet not connected");
      if (pendingRef.current) throw new Error("Position top-up already pending");
      pendingRef.current = true;
      args = { ...args };
      setIsPending(true);
      setError(null);
      try {
        buildTopUpPosition(args.positionId, args.amount);
        await ensureAllowance({
          token: COLLATERAL_TOKENS.usdl.address,
          owner: userAddress,
          spender: PERPS_SPENDER,
          amount: args.amount,
        });
        const hash = await writeContract(buildTopUpPosition(args.positionId, args.amount));
        await waitForTx(hash);
      } catch (e) {
        const err = e instanceof Error ? e : new Error(String(e));
        setError(err);
        throw err;
      } finally {
        pendingRef.current = false;
        setIsPending(false);
      }
    },
    [userAddress],
  );

  return { execute, isPending, error };
}

export function useAccountSummary(
  userAddress: `0x${string}` | undefined,
  tokenAddress: `0x${string}` | undefined,
): {
  data: AccountSummary | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
} {
  const [data, setData] = useState<AccountSummary | undefined>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const mountedRef = useRef(true);

  const refetch = useCallback(async () => {
    if (!userAddress || !tokenAddress) return;
    setIsLoading(true);
    setError(null);
    try {
      const result = (await readContract(buildGetAccountSummary(userAddress, tokenAddress))) as AccountSummary;

      if (mountedRef.current) {
        setData({
          idle: result.idle,
          totalLocked: result.totalLocked,
          positionCount: result.positionCount,
        });
      }
    } catch (e) {
      if (mountedRef.current) setError(e instanceof Error ? e : new Error(String(e)));
    } finally {
      if (mountedRef.current) setIsLoading(false);
    }
  }, [userAddress, tokenAddress]);

  useEffect(() => {
    mountedRef.current = true;
    refetch();
    return () => {
      mountedRef.current = false;
    };
  }, [refetch]);

  return { data, isLoading, error, refetch };
}

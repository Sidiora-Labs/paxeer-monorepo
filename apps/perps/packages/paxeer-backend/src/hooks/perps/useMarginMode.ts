"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import {
  buildGetAccountSummary,
  buildGetPositionMargin,
  buildTopUpPosition,
} from "@paxeer-network/paxeer-backend/core/perps/calls";
import { PERPS_SPENDER } from "@paxeer-network/paxeer-backend/core/network/approvals";
import { COLLATERAL_TOKENS } from "@paxeer-network/paxeer-backend/core/perps/config";
import { ensureAllowance } from "@paxeer-network/paxeer-backend/hooks/spot/useSpotApprovals";
import { readContract, writeContract, waitForTx } from "./useContract";
import { useSelectionRequest } from "./useSelectionRequest";
import type {
  PositionMargin,
  AccountSummary,
  TopUpPositionArgs,
} from "@paxeer-network/paxeer-backend/types/perps";

/** Native perps use one account margin mode; expose it through the shared snapshot. */
export function useMarginMode(userAddress: `0x${string}` | undefined): {
  data:
    | { marginMode: number; accountSummary: AccountSummary | null }
    | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
} {
  const [data, setData] = useState<
    { marginMode: number; accountSummary: AccountSummary | null } | undefined
  >();
  const refetch = useCallback(() => {
    if (!userAddress) {
      setData(undefined);
      return;
    }
    setData({ marginMode: 0, accountSummary: null });
  }, [userAddress]);
  useEffect(() => {
    refetch();
  }, [refetch]);
  return { data, isLoading: false, error: null, refetch };
}

export function usePositionMargin(
  userAddress: `0x${string}` | undefined,
  positionId: bigint | undefined,
): {
  data: PositionMargin | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
} {
  const account = userAddress?.toLowerCase();
  const key =
    account && positionId !== undefined
      ? `${account}:${positionId}`
      : undefined;
  const load = useCallback(async () => {
    if (!account || positionId === undefined) {
      throw new Error("Position selection unavailable");
    }
    const result = (await readContract(
      buildGetPositionMargin(positionId),
    )) as PositionMargin;
    return {
      token: result.token,
      lockedAmount: result.lockedAmount,
      marketId: result.marketId,
      isLong: result.isLong,
    };
  }, [account, positionId]);
  return useSelectionRequest(key, load);
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
      if (pendingRef.current)
        throw new Error("Position top-up already pending");
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
        const hash = await writeContract(
          buildTopUpPosition(args.positionId, args.amount),
        );
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
  const account = userAddress?.toLowerCase() as `0x${string}` | undefined;
  const token = tokenAddress?.toLowerCase() as `0x${string}` | undefined;
  const key = account && token ? `${account}:${token}` : undefined;
  const load = useCallback(async () => {
    if (!account || !token) throw new Error("Account selection unavailable");
    const result = (await readContract(
      buildGetAccountSummary(account, token),
    )) as AccountSummary;
    return {
      idle: result.idle,
      totalLocked: result.totalLocked,
      positionCount: result.positionCount,
    };
  }, [account, token]);
  return useSelectionRequest(key, load);
}

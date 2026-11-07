"use client";

import { useState, useCallback, useRef } from "react";
import {
  buildGetAccountSummary,
  buildGetPositionMargin,
  buildTopUpPosition,
} from "@/core/perps/calls";
import { PERPS_SPENDER } from "@/core/network/approvals";
import { COLLATERAL_TOKENS } from "@/core/perps/config";
import { ensureAllowance } from "@/hooks/spot/useSpotApprovals";
import { readContract, writeContract, waitForTx } from "./useContract";
import { useSelectionRequest } from "./useSelectionRequest";
import type {
  PositionMargin,
  AccountSummary,
  TopUpPositionArgs,
} from "@/types/perps";

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

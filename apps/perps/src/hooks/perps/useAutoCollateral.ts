"use client";

import { useCallback } from "react";
import type { Address } from "viem";

import { pickCollateral } from "@/core/perps/accountView";
import { useAccount } from "./useAccount";
import { usePerpsWallet } from "./usePerpsWallet";
import { useSelectionRequest } from "./useSelectionRequest";

export interface AutoCollateralResult {
  selectedToken: Address;
  selectedSymbol: string;
  selectedDecimals: number;
  /** USDL base units (6 decimals) of getAccount(account).collateral. */
  collateral: bigint;
  isLoading: boolean;
}

/**
 * Selects USDL, the single collateral of the cross-margin account at 0x1015, with the
 * account collateral read through getAccount(account). Falls back to USDL at 6 decimals
 * with zero collateral when no wallet is connected.
 */
export function useAutoCollateral(): AutoCollateralResult {
  const { isConnected, address } = usePerpsWallet();
  const { getAccount } = useAccount();
  const account =
    isConnected && address ? (address.toLowerCase() as Address) : undefined;
  const load = useCallback(async () => {
    if (!account) throw new Error("Wallet not connected");
    return pickCollateral(await getAccount(account)).collateral;
  }, [account, getAccount]);
  const { data, isLoading } = useSelectionRequest(account, load, 30_000);
  const collateral = data ?? 0n;

  const picked = pickCollateral({ collateral });

  return {
    selectedToken: picked.token,
    selectedSymbol: picked.symbol,
    selectedDecimals: picked.decimals,
    collateral: picked.collateral,
    isLoading,
  };
}

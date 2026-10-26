import type { ContractFunctionReturnType } from "viem";

import type { pecorVaultAbi } from "@/core/network/spot_abi/PECORVault";

export type VaultTokenInfo = ContractFunctionReturnType<typeof pecorVaultAbi, "view", "getTokenInfo">;

export interface SpotTokenState {
  tradeable: boolean;
  paused: boolean;
  reason: "TokenPaused" | "TokenNotRegistered" | null;
}

export function spotTokenState(paused: boolean, info: VaultTokenInfo): SpotTokenState {
  if (paused) return { tradeable: false, paused: true, reason: "TokenPaused" };
  if (!info[0]) return { tradeable: false, paused: false, reason: "TokenNotRegistered" };
  return { tradeable: true, paused: false, reason: null };
}

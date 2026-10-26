import type { ContractFunctionReturnType } from "viem";

import type { PerpsPrecompileABI } from "../network/abi/PerpsPrecompile";

export interface FeeTierView {
  tier: number;
  takerBps: number;
  makerBps: number;
  volume14dUsd: bigint;
}

export function feeTierView(
  result: ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getFeeTier">,
): FeeTierView {
  const [tier, takerFeePpm, makerFeePpm, volume14dUsd] = result;
  return {
    tier,
    takerBps: Number(takerFeePpm) / 100,
    makerBps: Number(makerFeePpm) / 100,
    volume14dUsd,
  };
}

"use client";

import { useQuery } from "@tanstack/react-query";
import type { ContractFunctionReturnType } from "viem";
import type { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";
import { buildGetMarketConfig, buildGetMarginTiers } from "@/core/perps/calls";
import { decodeMarketConfig, type MarginTier } from "@/core/perps/marketConfig";
import { readContract } from "./useContract";

export function useMarketConfig(marketId: bigint) {
  return useQuery({
    queryKey: ["perps", "marketConfig", marketId.toString()],
    queryFn: async () => {
      const [rawConfig, rawTiers] = await Promise.all([
        readContract(buildGetMarketConfig(marketId)),
        readContract(buildGetMarginTiers(marketId)),
      ]);
      const config = decodeMarketConfig(
        rawConfig as ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getMarketConfig">,
      );
      const tiers = rawTiers as readonly MarginTier[];
      if (
        config.marketId !== marketId ||
        config.tickSize <= 0n ||
        config.lotSize <= 0n ||
        config.maxLeverage < 10n ** 18n ||
        !tiers.some((tier) => tier.lowerBoundUsd === 0n) ||
        tiers.some((tier) => tier.maxLeverage < 10n ** 18n)
      ) {
        throw new Error("Market order limits are invalid.");
      }
      return { config, tiers };
    },
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

"use client";

import { queryOptions, useQuery } from "@tanstack/react-query";
import type { ContractFunctionReturnType } from "viem";
import type { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";
import { buildGetMarketConfig, buildGetMarginTiers } from "@/core/perps/calls";
import { decodeMarketConfig, type MarginTier } from "@/core/perps/marketConfig";
import { readContract } from "./useContract";

export function marketConfigQueryOptions(marketId: bigint) {
  return queryOptions({
    queryKey: ["perps", "marketConfig", marketId.toString()],
    queryFn: async () => {
      const [rawConfig, rawTiers] = await Promise.all([
        readContract(buildGetMarketConfig(marketId)),
        readContract(buildGetMarginTiers(marketId)),
      ]);
      const config = decodeMarketConfig(
        rawConfig as ContractFunctionReturnType<
          typeof PerpsPrecompileABI,
          "view",
          "getMarketConfig"
        >,
      );
      const tiers = rawTiers as readonly MarginTier[];
      if (
        config.marketId !== marketId ||
        config.tickSize <= 0n ||
        config.lotSize <= 0n ||
        config.maxLeverage < 1n ||
        !tiers.some((tier) => tier.lowerBoundUsd === 0n) ||
        tiers.some(
          (tier, index) =>
            tier.maxLeverage < 1n ||
            tier.maintenanceBps > 10_000n ||
            (index > 0 && tier.lowerBoundUsd <= tiers[index - 1].lowerBoundUsd),
        )
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

export function useMarketConfig(marketId: bigint) {
  return useQuery(marketConfigQueryOptions(marketId));
}

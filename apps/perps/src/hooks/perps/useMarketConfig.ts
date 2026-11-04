"use client";

import { useEffect } from "react";
import { queryOptions, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ContractFunctionReturnType } from "viem";
import type { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";
import { buildGetMarketConfig, buildGetMarginTiers } from "@/core/perps/calls";
import { decodeMarketConfig, type MarginTier } from "@/core/perps/marketConfig";
import { onStreamResync } from "@/core/net/realtime";
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
    staleTime: 30_000,
    gcTime: 5 * 60_000,
    refetchInterval: 30_000,
    retry: false,
    refetchOnMount: true,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  });
}

export function useMarketConfig(marketId: bigint) {
  const client = useQueryClient();
  const marketKey = marketId.toString();
  useEffect(
    () =>
      onStreamResync(() => {
        void client.invalidateQueries({
          queryKey: ["perps", "marketConfig", marketKey],
          exact: true,
        });
      }),
    [client, marketKey],
  );
  return useQuery(marketConfigQueryOptions(marketId));
}

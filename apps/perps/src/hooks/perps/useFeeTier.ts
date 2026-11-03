"use client";

import { queryOptions, useQuery } from "@tanstack/react-query";
import { isAddress } from "viem";
import { z } from "zod";
import { buildGetFeeTier } from "@/core/perps/calls";
import { feeTierView } from "@/core/perps/feeTier";
import { readContract } from "./useContract";

const uint256 = z
  .bigint()
  .nonnegative()
  .max((1n << 256n) - 1n);
const feeTierResult = z.tuple([
  z.number().int().min(0).max(255),
  uint256,
  z
    .bigint()
    .min(-(1n << 255n))
    .max((1n << 255n) - 1n),
  uint256,
]);

export function feeTierQueryOptions(address: string | undefined) {
  return queryOptions({
    queryKey: ["perps", "feeTier", address?.toLowerCase()],
    enabled: Boolean(address),
    retry: false,
    retryOnMount: false,
    queryFn: async () => {
      if (!address || !isAddress(address))
        throw new Error("Invalid account address");
      return feeTierView(
        feeTierResult.parse(await readContract(buildGetFeeTier(address))),
      );
    },
  });
}

export function useFeeTier(address: string | undefined) {
  const query = useQuery(feeTierQueryOptions(address));
  return {
    view: query.isError ? undefined : query.data,
    isLoading: Boolean(address && query.isPending),
    error: query.error,
  };
}

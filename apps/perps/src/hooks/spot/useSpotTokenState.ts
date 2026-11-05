"use client";

import { useEffect } from "react";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { isAddress, type Address } from "viem";
import { z } from "zod";
import { classifyError } from "@/core/errors/classify";
import { reportError } from "@/core/errors/report";
import { buildGetTokenInfo, buildIsTokenPaused } from "@/core/spot/calls";
import { spotTokenState, type SpotTokenState } from "@/core/spot/tokenState";
import { readContract } from "@/hooks/perps/useContract";
import { getPaxPointer } from "./useSpotOrders";

const uint256 = z
  .bigint()
  .nonnegative()
  .max((1n << 256n) - 1n);
const tokenInfo = z.tuple([
  z.boolean(),
  z.boolean(),
  z.number().int().min(0).max(255),
  uint256,
  uint256,
  uint256,
]);

export function spotTokenStateQueryOptions(token: Address, native = false) {
  return queryOptions({
    queryKey: ["spot", "tokenState", token.toLowerCase(), native],
    retry: false,
    retryOnMount: false,
    refetchInterval: 8_000,
    queryFn: async () => {
      if (!isAddress(token)) throw new Error("Invalid token address");
      const address = native ? await getPaxPointer() : token;
      const [paused, info] = await Promise.all([
        readContract(buildIsTokenPaused(address)),
        readContract(buildGetTokenInfo(address)),
      ]);
      return spotTokenState(z.boolean().parse(paused), tokenInfo.parse(info));
    },
  });
}

export interface SpotTokenStateResult {
  /** Null while loading or after a failed read; never implies tradeability. */
  state: SpotTokenState | null;
  /** Classified display message on failure, otherwise null. */
  error: string | null;
}

/** Read pause/registration state; native tokens resolve the PAX pointer first. */
export function useSpotTokenState(
  token: Address,
  native = false,
): SpotTokenStateResult {
  const query = useQuery(spotTokenStateQueryOptions(token, native));
  useEffect(() => {
    if (query.error)
      void reportError(classifyError(query.error, "spot-order/token-state"));
  }, [query.error]);
  return {
    state: query.isError ? null : (query.data ?? null),
    error: query.error
      ? classifyError(query.error, "spot-order/token-state").message
      : null,
  };
}

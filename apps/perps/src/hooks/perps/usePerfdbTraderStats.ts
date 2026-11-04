"use client";

import { useEffect } from "react";
import { usePerfdbSelection } from "./usePerfdbSelection";
import { perfdbQuery } from "@/core/perfdb/client";

// ── Types ────────────────────────────────────────────────────────────
export interface PerfdbTraderStats {
  address: string;
  totalPnl: string;
  totalVolume: string;
  totalFeesPaid: string;
  totalTrades: number;
  winCount: number;
  lossCount: number;
  liquidationCount: number;
  openPositions: number;
}

// ── Query (schema confirms all fields on GqlTraderStats) ────────────
const USER_STATS_QUERY = /* GraphQL */ `
  query UserStats($address: String!) {
    userStats(address: $address) {
      address
      totalPnl
      totalVolume
      totalFeesPaid
      totalTrades
      winCount
      lossCount
      liquidationCount
      openPositions
    }
  }
`;

interface UserStatsResponse {
  userStats: PerfdbTraderStats | null;
}

/**
 * Fetches enhanced trader stats from PerfDB.
 * Fetch once when address changes. No polling.
 */
export function usePerfdbTraderStats(address: string | undefined): {
  stats: PerfdbTraderStats | null;
  isLoading: boolean;
  error: Error | null;
} {
  const account = address?.trim().toLowerCase() || null;
  const {
    value: stats,
    isLoading,
    error,
    run,
  } = usePerfdbSelection<PerfdbTraderStats | null>(account, null);
  useEffect(() => {
    void run(async () => {
      if (!account) return null;
      const res = await perfdbQuery<UserStatsResponse>(USER_STATS_QUERY, {
        address: account,
      });
      return res.userStats ?? null;
    }, null);
  }, [account, run]);
  return { stats, isLoading: Boolean(account) && isLoading, error };
}

"use client";

import { useEffect } from "react";
import { usePerfdbSelection } from "./usePerfdbSelection";
import { perfdbQuery } from "@/core/perfdb/client";

// ── Types ────────────────────────────────────────────────────────────
export interface PerfdbBalance {
  token: string;
  balance: string;
  totalDeposited: string;
  totalWithdrawn: string;
  depositCount: number;
  withdrawCount: number;
}

// ── Query (schema confirms all fields on GqlUserBalance) ────────────
const USER_BALANCES_QUERY = /* GraphQL */ `
  query UserBalances($address: String!) {
    userBalances(address: $address) {
      token
      balance
      totalDeposited
      totalWithdrawn
      depositCount
      withdrawCount
    }
  }
`;

interface UserBalancesResponse {
  userBalances: PerfdbBalance[];
}

/**
 * Fetches PerfDB aggregated balances (per-token: balance, totals, counts).
 * Fetch once when address changes.
 */
export function usePerfdbBalances(address: string | undefined): {
  balances: PerfdbBalance[];
  isLoading: boolean;
  error: Error | null;
} {
  const account = address?.toLowerCase() || null;
  const { value: balances, isLoading, error, run } = usePerfdbSelection<PerfdbBalance[]>(account, []);
  useEffect(() => {
    void run(async () => {
      if (!account) return [];
      const res = await perfdbQuery<UserBalancesResponse>(USER_BALANCES_QUERY, { address: account });
      return res.userBalances ?? [];
    }, []);
  }, [account, run]);
  return { balances, isLoading: Boolean(account) && isLoading, error };
}

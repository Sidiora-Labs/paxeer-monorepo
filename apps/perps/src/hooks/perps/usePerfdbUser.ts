"use client";

import { useEffect } from "react";
import { usePerfdbSelection } from "./usePerfdbSelection";
import { perfdbQuery } from "@/core/perfdb/client";

// ── Types ────────────────────────────────────────────────────────────
export interface PerfdbUser {
  address: string;
  vaultAddress: string;
  referralCode: string | null;
  tier: string | null; // e.g. "bronze", "silver", "gold", "platinum", "diamond"
  feeDiscountBps: number; // basis points, e.g. 50 = 0.5%
}

// ── Query (schema confirms GqlUser fields) ──────────────────────────
const USER_QUERY = /* GraphQL */ `
  query User($address: String!) {
    user(address: $address) {
      address
      vaultAddress
      referralCode
      tier
      feeDiscountBps
    }
  }
`;

interface UserResponse {
  user: PerfdbUser | null;
}

/**
 * Fetches PerfDB user profile (vault, referral code, tier, fee discount).
 * Fetch once when address changes.
 */
export function usePerfdbUser(address: string | undefined): {
  user: PerfdbUser | null;
  isLoading: boolean;
  error: Error | null;
} {
  const account = address?.trim().toLowerCase() || null;
  const {
    value: user,
    isLoading,
    error,
    run,
  } = usePerfdbSelection<PerfdbUser | null>(account, null);
  useEffect(() => {
    void run(async () => {
      if (!account) return null;
      const res = await perfdbQuery<UserResponse>(USER_QUERY, {
        address: account,
      });
      return res.user ?? null;
    }, null);
  }, [account, run]);
  return { user, isLoading: Boolean(account) && isLoading, error };
}

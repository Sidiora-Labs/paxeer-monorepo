"use client";

import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import {
  perfdbQuery,
  perfdbRestAuth,
  getAuthToken,
} from "@/core/perfdb/client";

// ── Types ────────────────────────────────────────────────────────────
// Spec defined ReferralTier with { tier:number, name, requiredReferrals, rewardBps }.
// Schema actually exposes GqlReferralTier = { tier:String!, minReferrals:Int!, minVolume:String!, rewardBps:Int!, feeDiscountBps:Int! }.
// We expose BOTH shapes (spec + extra schema fields) so consumers can use
// `name` (alias of tier) or `requiredReferrals` (alias of minReferrals).
export interface ReferralTier {
  tier: string; // lowercase name e.g. "bronze"
  name: string; // alias of tier (capitalized)
  requiredReferrals: number; // alias of minReferrals
  minReferrals: number;
  minVolume: string;
  rewardBps: number;
  feeDiscountBps: number;
}

// Spec defined UserReferralData with { referralCode, tier:number, referralCount, totalEarned }.
// The referral tier is nullable when no tier qualifies in the server's LEFT JOIN.
// We make totalEarned optional (null when server doesn't provide).
export interface UserReferralData {
  referralCode: string | null;
  tier: string | null;
  referralCount: number;
  totalEarned: string | null;
}

interface RawReferralTier {
  tier: string;
  minReferrals: number;
  minVolume: string;
  rewardBps: number;
  feeDiscountBps: number;
}

interface RawUserReferrals {
  referralCode: string | null;
  tier: string | null;
  referralCount: number;
  totalEarned?: string | null;
}

interface ReferralsResponse {
  userReferrals: RawUserReferrals | null;
  referralTiers: RawReferralTier[];
}

// ── Queries ──────────────────────────────────────────────────────────
const USER_REFERRALS_QUERY = /* GraphQL */ `
  query UserReferrals($address: String!) {
    userReferrals(address: $address) {
      referralCode
      tier
      referralCount
    }
    referralTiers {
      tier
      minReferrals
      minVolume
      rewardBps
      feeDiscountBps
    }
  }
`;

// Tiers-only query (no address required)
const REFERRAL_TIERS_QUERY = /* GraphQL */ `
  query ReferralTiers {
    referralTiers {
      tier
      minReferrals
      minVolume
      rewardBps
      feeDiscountBps
    }
  }
`;

function capitalize(s: string): string {
  if (!s) return s;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function normalizeTier(raw: RawReferralTier): ReferralTier {
  return {
    tier: raw.tier,
    name: capitalize(raw.tier),
    requiredReferrals: raw.minReferrals,
    minReferrals: raw.minReferrals,
    minVolume: raw.minVolume,
    rewardBps: raw.rewardBps,
    feeDiscountBps: raw.feeDiscountBps,
  };
}

function normalizeUser(raw: RawUserReferrals): UserReferralData {
  return {
    referralCode: raw.referralCode,
    tier: raw.tier,
    referralCount: raw.referralCount,
    totalEarned: raw.totalEarned ?? null,
  };
}

export interface ReferralTierView {
  status: "unavailable" | "unranked" | "ranked";
  label: string;
  currentTierIndex: number;
  nextTier: ReferralTier | null;
  progressPct: number | null;
  isMaxTier: boolean;
}

export function referralTierView(
  data: UserReferralData | null,
  tiers: readonly ReferralTier[],
): ReferralTierView {
  const tierName = data?.tier;
  const index =
    tierName == null
      ? -1
      : tiers.findIndex(
          (tier) => tier.tier.toLowerCase() === tierName.toLowerCase(),
        );
  const status =
    !data || (data.tier !== null && index < 0)
      ? "unavailable"
      : data.tier === null
        ? "unranked"
        : "ranked";
  const nextTier = status === "unavailable" ? null : (tiers[index + 1] ?? null);
  const isMaxTier = status === "ranked" && nextTier === null;
  let progressPct: number | null = null;
  if (data && status !== "unavailable") {
    if (nextTier) {
      progressPct =
        nextTier.requiredReferrals === 0
          ? 100
          : Math.min(
              100,
              Math.max(
                0,
                (data.referralCount / nextTier.requiredReferrals) * 100,
              ),
            );
    } else if (isMaxTier) {
      progressPct = 100;
    }
  }
  return {
    status,
    label:
      status === "unranked"
        ? "Unranked"
        : status === "ranked"
          ? tiers[index].name
          : "Tier unavailable",
    currentTierIndex: index,
    nextTier,
    progressPct,
    isMaxTier,
  };
}

export async function fetchPerfdbReferrals(
  address: string | undefined,
): Promise<{
  data: UserReferralData | null;
  tiers: ReferralTier[];
}> {
  if (address) {
    const res = await perfdbQuery<ReferralsResponse>(USER_REFERRALS_QUERY, {
      address,
    });
    return {
      data: res.userReferrals ? normalizeUser(res.userReferrals) : null,
      tiers: (res.referralTiers ?? []).map(normalizeTier),
    };
  }
  const res = await perfdbQuery<{ referralTiers: RawReferralTier[] }>(
    REFERRAL_TIERS_QUERY,
  );
  return { data: null, tiers: (res.referralTiers ?? []).map(normalizeTier) };
}

/**
 * Fetches a user's referral status and the global tier ladder.
 * When `address` is undefined, only the tiers list is populated.
 */
export function usePerfdbReferrals(address: string | undefined): {
  data: UserReferralData | null;
  tiers: ReferralTier[];
  tierView: ReferralTierView;
  isLoading: boolean;
  error: Error | null;
  generateCode: () => Promise<string>;
  applyCode: (code: string) => Promise<void>;
} {
  const [data, setData] = useState<UserReferralData | null>(null);
  const [tiers, setTiers] = useState<ReferralTier[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);
  const mountedRef = useRef<boolean>(true);
  const addressRef = useRef<string | undefined>(address);

  useEffect(() => {
    addressRef.current = address;
  }, [address]);

  const fetchAll = useCallback(async (): Promise<void> => {
    try {
      const res = await fetchPerfdbReferrals(addressRef.current);
      if (!mountedRef.current) return;
      setData(res.data);
      setTiers(res.tiers);
      setError(null);
    } catch (err) {
      if (!mountedRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
      setData(null);
    } finally {
      if (mountedRef.current) setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    setIsLoading(true);
    fetchAll();
    return () => {
      mountedRef.current = false;
    };
  }, [address, fetchAll]);

  const generateCode = useCallback(async (): Promise<string> => {
    const token = getAuthToken();
    if (!token) {
      throw new Error("Not authenticated. Sign in to generate a code.");
    }
    const res = await perfdbRestAuth<{ referralCode: string }>(
      "/v2/referrals/generate",
      { method: "POST" },
    );
    await fetchAll();
    return res.referralCode;
  }, [fetchAll]);

  const applyCode = useCallback(
    async (code: string): Promise<void> => {
      const token = getAuthToken();
      if (!token) {
        throw new Error("Not authenticated. Sign in to apply a code.");
      }
      const trimmed = code.trim();
      if (!trimmed) throw new Error("Referral code is required.");
      await perfdbRestAuth<{ ok: boolean }>("/v2/referrals/apply", {
        method: "POST",
        body: JSON.stringify({ code: trimmed }),
      });
      await fetchAll();
    },
    [fetchAll],
  );

  const tierView = useMemo(() => referralTierView(data, tiers), [data, tiers]);
  return { data, tiers, tierView, isLoading, error, generateCode, applyCode };
}

"use client";

import { useCallback, useRef } from "react";
import { useSelectionRequest } from "./useSelectionRequest";
import { requirePerfdbIdentity } from "./usePerfdbAuth";
import {
  perfdbQueryAuth,
  perfdbRestAuth,
} from "@/core/perfdb/client";

// ── Types ────────────────────────────────────────────────────────────
export type AlertType =
  "price_above" | "price_below" | "liquidation_warning" | "funding_threshold";

/**
 * Schema on server is `GqlAlert { id:Int!, alertType:String!, marketId:Int, threshold:String, enabled:Boolean!, triggeredAt:String }`.
 * We normalize `id` to string (Int → string) and expose `isActive` alias of `enabled`.
 */
export interface PerfdbAlert {
  id: string;
  alertType: AlertType;
  marketId: number | null;
  threshold: string | null;
  createdAt: string | null; // schema has no createdAt; triggeredAt used when available
  isActive: boolean;
}

interface RawAlert {
  id: number | string;
  alertType: string;
  marketId: number | null;
  threshold: string | null;
  enabled: boolean;
  triggeredAt: string | null;
}

// ── Query ────────────────────────────────────────────────────────────
const USER_ALERTS_QUERY = /* GraphQL */ `
  query UserAlerts($address: String!) {
    userAlerts(address: $address) {
      id
      alertType
      marketId
      threshold
      enabled
      triggeredAt
    }
  }
`;

interface UserAlertsResponse {
  userAlerts: RawAlert[];
}

function normalize(raw: RawAlert): PerfdbAlert {
  return {
    id: String(raw.id),
    alertType: raw.alertType as AlertType,
    marketId: raw.marketId,
    threshold: raw.threshold,
    createdAt: raw.triggeredAt,
    isActive: Boolean(raw.enabled),
  };
}

/**
 * Fetches & manages a user's alerts.
 * Reads require an address only (auth header is sent if present but server may allow read without it).
 * Mutations require JWT via `perfdbRestAuth`.
 */
export function usePerfdbAlerts(address: string | undefined): {
  alerts: PerfdbAlert[];
  isLoading: boolean;
  error: Error | null;
  createAlert: (
    alertType: AlertType,
    marketId?: number,
    threshold?: string,
  ) => Promise<void>;
  deleteAlert: (id: string) => Promise<void>;
} {
  const account = address?.toLowerCase() || undefined;
  const addressRef = useRef(account);
  addressRef.current = account;
  const load = useCallback(async () => {
    if (!account) throw new Error("Account selection unavailable");
    const res = await perfdbQueryAuth<UserAlertsResponse>(USER_ALERTS_QUERY, {
      address: account,
    });
    return (res.userAlerts ?? []).map(normalize);
  }, [account]);
  const { data, isLoading, error, refetch } = useSelectionRequest(
    account,
    load,
  );
  const alerts = error ? [] : (data ?? []);

  const createAlert = useCallback(
    async (
      alertType: AlertType,
      marketId?: number,
      threshold?: string,
    ): Promise<void> => {
      requirePerfdbIdentity(addressRef.current);
      const body: Record<string, unknown> = { alertType };
      if (typeof marketId === "number") body.marketId = marketId;
      if (threshold !== undefined && threshold !== "")
        body.threshold = threshold;
      await perfdbRestAuth<{ ok: boolean }>("/v2/alerts", {
        method: "POST",
        body: JSON.stringify(body),
      });
      // Refresh list
      await refetch();
    },
    [refetch],
  );

  const deleteAlert = useCallback(
    async (id: string): Promise<void> => {
      requirePerfdbIdentity(addressRef.current);
      await perfdbRestAuth<{ ok: boolean; }>(
        `/v2/alerts/${encodeURIComponent(id)}`,
        { method: "DELETE" },
      );
      await refetch();
    },
    [refetch],
  );

  return { alerts, isLoading, error, createAlert, deleteAlert };
}

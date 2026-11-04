"use client";

import { useEffect, useState } from "react";
import type { MarketSnapshot } from "./useMarketsStream";

export const PRICE_FRESHNESS_MS = 30_000;

/** Wake at the next row's expiry even if the market socket remains silent. */
export function usePriceFreshness(rows: readonly MarketSnapshot[]): number {
  const [, tick] = useState(0);
  const now = Date.now();
  useEffect(() => {
    let deadline = Infinity;
    for (const row of rows) {
      const expires = row.price_updated_at + PRICE_FRESHNESS_MS;
      if (row.price > 0 && Number.isFinite(expires) && expires > now) {
        deadline = Math.min(deadline, expires);
      }
    }
    if (!Number.isFinite(deadline)) return;
    const timer = setTimeout(
      () => tick((value) => value + 1),
      Math.min(PRICE_FRESHNESS_MS, Math.max(1, deadline - Date.now())),
    );
    return () => clearTimeout(timer);
  }, [rows, now]);
  return now;
}

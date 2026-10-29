"use client";

import { useMemo } from "react";
import { getMarketCategory, type TradeableMarket } from "@/core/perps/config";
import type { MarketCatalogEntry } from "@/core/perps/marketCatalog";
import { usePerpsMarkets } from "@/hooks/perps/usePerpsMarkets";
import {
  useMarketsSnapshot,
  type MarketSnapshot,
} from "@/hooks/perps/useMarketsStream";
import type { MarketRow } from "./types";

/**
 * True if the category filter should include this market.
 *
 * - `rwa` bundles stocks + indices + commodities (as labelled in the
 *   reference design's filter chip).
 * - `crypto` maps 1:1 to the canonical crypto market list.
 * - `new` uses the catalog listing category.
 * - `all` (and anything unrecognised) includes every market.
 * - `favorites` is resolved upstream in the page — it's out of scope here.
 */
function matchesCategory(category: string, row: MarketRow): boolean {
  if (category === "all" || category === "favorites") return true;
  if (category === "new") return row.isNew;
  const assetCategory =
    row.category === "new" ? getMarketCategory(row.marketId) : row.category;
  if (category === "rwa") {
    return ["stocks", "indices", "commodities"].includes(assetCategory);
  }
  if (category === "crypto") return assetCategory === "crypto";
  return true;
}

export function buildMarketsRows(
  markets: readonly (MarketCatalogEntry & TradeableMarket)[],
  snapshots: readonly MarketSnapshot[],
): MarketRow[] {
  const snapBySymbol = new Map<string, MarketSnapshot>();
  for (const snapshot of snapshots) {
    snapBySymbol.set(snapshot.symbol.toUpperCase(), snapshot);
  }

  return markets.map((market) => {
    const snapshot = snapBySymbol.get(market.symbol.toUpperCase());
    return {
      ...market,
      price: snapshot?.price ?? 0,
      change24h: snapshot?.change_24h ?? 0,
      volume24h: snapshot?.volume_24h ?? 0,
      openInterest: snapshot?.open_interest ?? 0,
      funding8h: snapshot?.funding_8h ?? null,
      fundingHourly: snapshot?.last_funding_rate_hourly ?? null,
      isNew: market.category === "new",
      hasLiveData: Boolean(snapshot),
      snapshot,
    };
  });
}

export function useMarketsRows(): {
  rows: MarketRow[];
  bySymbol: Map<string, MarketRow>;
  hasAnyLive: boolean;
} {
  const { tradeable } = usePerpsMarkets();
  const snapshots = useMarketsSnapshot();

  return useMemo(() => {
    const rows = buildMarketsRows(tradeable, snapshots);
    const bySymbol = new Map(rows.map((row) => [row.symbol, row]));
    return { rows, bySymbol, hasAnyLive: rows.some((row) => row.hasLiveData) };
  }, [tradeable, snapshots]);
}

/**
 * Pure filter + sort utility, extracted so tests and view-level memos
 * can share the same logic without creating a hook dependency cycle.
 */
export function filterAndSort(
  rows: readonly MarketRow[],
  opts: {
    search?: string;
    category?: string;
    favorites?: ReadonlySet<string>;
    sortKey?: import("./types").SortKey;
    sortDir?: import("./types").SortDir;
  },
): MarketRow[] {
  const { search, category = "all", favorites, sortKey = "volume24h", sortDir = "desc" } = opts;

  let list = rows.filter((r) => {
    if (category === "favorites") {
      if (!favorites || !favorites.has(r.symbol.toUpperCase())) return false;
    } else if (!matchesCategory(category, r)) {
      return false;
    }
    if (search) {
      const q = search.toLowerCase();
      if (
        !r.symbol.toLowerCase().includes(q) &&
        !r.name.toLowerCase().includes(q)
      ) {
        return false;
      }
    }
    return true;
  });

  list = [...list].sort((a, b) => {
    let cmp = 0;
    switch (sortKey) {
      case "symbol":
        cmp = a.symbol.localeCompare(b.symbol);
        break;
      case "price":
        cmp = a.price - b.price;
        break;
      case "change24h":
        cmp = a.change24h - b.change24h;
        break;
      case "volume24h":
        cmp = a.volume24h - b.volume24h;
        break;
      case "openInterest":
        cmp = a.openInterest - b.openInterest;
        break;
      case "funding8h":
        cmp = a.funding8h === b.funding8h ? 0 : a.funding8h === null ? -1 : b.funding8h === null ? 1 : a.funding8h < b.funding8h ? -1 : 1;
        break;
    }
    return sortDir === "asc" ? cmp : -cmp;
  });

  return list;
}

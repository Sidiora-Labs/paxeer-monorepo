"use client";

import { queryOptions, useQuery } from "@tanstack/react-query";
import { useSyncExternalStore } from "react";
import { isPageActive } from "@paxeer-network/paxeer-backend/core/net/realtime";
import { gqlQuery } from "@paxeer-network/paxeer-backend/core/graphql/client";
import { registerVenueSymbols } from "@paxeer-network/paxeer-backend/core/net/venueStream";
import {
  INDEXER_GRAPHQL,
  type TradeableMarket,
} from "@paxeer-network/paxeer-backend/core/perps/config";
import {
  catalogFromGateway,
  type GatewayMarketRow,
  type MarketCatalogEntry,
} from "@paxeer-network/paxeer-backend/core/perps/marketCatalog";
import { decodeGatewayMarketsResult, MARKETS_QUERY } from "./useSubgraph";

export type GatewayCatalogEntry = MarketCatalogEntry &
  Pick<
    GatewayMarketRow,
    "oracleSymbol" | "tickSize" | "lotSize" | "tierClass"
  > & {
    baseToken: string;
  };

export interface PerpsMarketsCatalog {
  catalog: GatewayCatalogEntry[];
  tradeable: (GatewayCatalogEntry & TradeableMarket)[];
}

export async function loadPerpsMarkets(): Promise<PerpsMarketsCatalog> {
  const answer = decodeGatewayMarketsResult(
    await gqlQuery<unknown>(INDEXER_GRAPHQL, MARKETS_QUERY),
  );
  const mapped = catalogFromGateway(answer);
  const rows = new Map(answer.markets.map((row) => [row.marketId, row]));
  const catalog = mapped.catalog.map((entry): GatewayCatalogEntry => {
    const row = rows.get(entry.marketId);
    if (!row) throw new Error(`Missing gateway market ${entry.marketId}`);
    return {
      ...entry,
      baseToken: row.symbol,
      oracleSymbol: row.oracleSymbol,
      tickSize: row.tickSize,
      lotSize: row.lotSize,
      tierClass: row.tierClass,
    };
  });
  const tradeable = catalog
    .filter((entry) => entry.enabled)
    .map((entry) => {
      if (entry.maxLeverage === null) {
        throw new Error(
          `Missing max leverage for enabled market ${entry.marketId}`,
        );
      }
      return { ...entry, maxLeverage: entry.maxLeverage };
    });
  registerVenueSymbols(catalog.map((entry) => entry.symbol));
  return { catalog, tradeable };
}

export function catalogRetryDelay(failureCount: number) {
  return [2000, 5000, 15000][failureCount] ?? 30000;
}

function subscribePageActivity(onChange: () => void) {
  window.addEventListener("online", onChange);
  window.addEventListener("offline", onChange);
  document.addEventListener("visibilitychange", onChange);
  return () => {
    window.removeEventListener("online", onChange);
    window.removeEventListener("offline", onChange);
    document.removeEventListener("visibilitychange", onChange);
  };
}

export function perpsMarketsQueryOptions(active = isPageActive()) {
  return queryOptions({
    queryKey: ["perps", "markets"],
    queryFn: loadPerpsMarkets,
    enabled: active,
    retry: true,
    retryDelay: catalogRetryDelay,
  });
}

export function usePerpsMarkets() {
  const active = useSyncExternalStore(
    subscribePageActivity,
    isPageActive,
    () => false,
  );
  const query = useQuery(perpsMarketsQueryOptions(active));
  const data = query.data;
  return {
    catalog: data?.catalog ?? [],
    tradeable: data?.tradeable ?? [],
    isLoading: query.isPending,
    error: query.error,
    refetch: query.refetch,
  };
}

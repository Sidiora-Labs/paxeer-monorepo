"use client";

import { queryOptions, useQuery } from "@tanstack/react-query";
import { gqlQuery } from "@/core/graphql/client";
import { registerVenueSymbols } from "@/core/net/venueStream";
import { INDEXER_GRAPHQL, type TradeableMarket } from "@/core/perps/config";
import {
  catalogFromGateway,
  type GatewayMarketRow,
  type MarketCatalogEntry,
} from "@/core/perps/marketCatalog";
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

export function perpsMarketsQueryOptions() {
  return queryOptions({
    queryKey: ["perps", "markets"],
    queryFn: loadPerpsMarkets,
    retry: false,
  });
}

export function usePerpsMarkets() {
  const query = useQuery(perpsMarketsQueryOptions());
  const data = query.isError ? undefined : query.data;
  return {
    catalog: data?.catalog ?? [],
    tradeable: data?.tradeable ?? [],
    isLoading: query.isPending,
    error: query.error,
    refetch: query.refetch,
  };
}

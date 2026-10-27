"use client";

import { useCallback, useEffect, useState } from "react";
import { gqlQuery } from "@/core/graphql/client";
import { registerVenueSymbols } from "@/core/net/venueStream";
import { INDEXER_GRAPHQL, type PerpsMarket } from "@/core/perps/config";
import {
  catalogFromGateway,
  type GatewayMarketRow,
  type MarketCatalogEntry,
} from "@/core/perps/marketCatalog";
import { MARKETS_QUERY } from "./useSubgraph";

export type GatewayCatalogEntry = MarketCatalogEntry &
  Pick<GatewayMarketRow, "oracleSymbol" | "tickSize" | "lotSize" | "tierClass"> & {
    baseToken: string;
  };

export interface PerpsMarketsCatalog {
  catalog: GatewayCatalogEntry[];
  tradeable: (GatewayCatalogEntry & PerpsMarket)[];
}

export async function loadPerpsMarkets(): Promise<PerpsMarketsCatalog> {
  const answer = await gqlQuery<{ markets: GatewayMarketRow[] }>(
    INDEXER_GRAPHQL,
    MARKETS_QUERY,
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
        throw new Error(`Missing max leverage for enabled market ${entry.marketId}`);
      }
      return { ...entry, maxLeverage: entry.maxLeverage };
    });
  registerVenueSymbols(catalog.map((entry) => entry.symbol));
  return { catalog, tradeable };
}

export function usePerpsMarkets() {
  const [data, setData] = useState<PerpsMarketsCatalog>();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [generation, setGeneration] = useState(0);
  const refetch = useCallback(() => setGeneration((value) => value + 1), []);

  useEffect(() => {
    let active = true;
    setIsLoading(true);
    loadPerpsMarkets()
      .then((catalog) => {
        if (!active) return;
        setData(catalog);
        setError(null);
      })
      .catch((cause: unknown) => {
        if (active) {
          setError(cause instanceof Error ? cause : new Error(String(cause)));
        }
      })
      .finally(() => {
        if (active) setIsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [generation]);

  return {
    catalog: data?.catalog ?? [],
    tradeable: data?.tradeable ?? [],
    isLoading,
    error,
    refetch,
  };
}

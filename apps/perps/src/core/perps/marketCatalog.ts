import { getMarketCategory } from "./config";

export interface GatewayMarketRow {
  marketId: number;
  symbol: string;
  name: string;
  oracleSymbol: string;
  szDecimals: number | null;
  pxDecimals: number | null;
  tickSize: string | null;
  lotSize: string | null;
  tierClass: number | null;
  maxLeverage: string | null;
  enabled: boolean;
  createdAt: string | null;
  latestPrice: { price: string; onchainTimestamp: number | null } | null;
  fundingRate: { ratePerSecond: string; rate24h: string } | null;
}

export interface MarketCatalogEntry {
  marketId: number;
  symbol: string;
  name: string;
  szDecimals: number | null;
  pxDecimals: number | null;
  maxLeverage: number | null;
  enabled: boolean;
  listedAt: string | null;
  category: string;
}

export interface MarketCatalog {
  catalog: MarketCatalogEntry[];
  tradeable: MarketCatalogEntry[];
}

const NEW_LISTING_WINDOW_MS = 14 * 24 * 60 * 60 * 1000;

export function catalogFromGateway(
  answer: { markets: readonly GatewayMarketRow[] },
  now: number = Date.now(),
): MarketCatalog {
  if (!Number.isFinite(now)) throw new Error("Invalid catalog time");

  const catalog = answer.markets.map((row): MarketCatalogEntry => {
    const listedAtMs = row.createdAt === null ? null : Date.parse(row.createdAt);
    if (listedAtMs !== null && !Number.isFinite(listedAtMs)) {
      throw new Error(`Invalid listing time for market ${row.marketId}`);
    }
    const maxLeverage = row.maxLeverage === null ? null : Number(row.maxLeverage);
    if (
      maxLeverage !== null &&
      (!Number.isFinite(maxLeverage) || maxLeverage <= 0)
    ) {
      throw new Error(`Invalid max leverage for market ${row.marketId}`);
    }
    const isNew =
      listedAtMs !== null &&
      listedAtMs <= now &&
      now - listedAtMs <= NEW_LISTING_WINDOW_MS;

    return {
      marketId: row.marketId,
      symbol: row.symbol,
      name: row.name,
      szDecimals: row.szDecimals,
      pxDecimals: row.pxDecimals,
      maxLeverage,
      enabled: row.enabled,
      listedAt: row.createdAt,
      category: isNew ? "new" : getMarketCategory(row.marketId),
    };
  }).sort((a, b) => a.marketId - b.marketId);

  return { catalog, tradeable: catalog.filter((entry) => entry.enabled) };
}

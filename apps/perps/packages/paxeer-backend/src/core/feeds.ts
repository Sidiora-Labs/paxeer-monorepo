/** Approved production origins use the public market feed directly. */
export const PUBLIC_FEED_ORIGIN =
  process.env.PRICE_FEED_ORIGIN || "https://data.feed.chainflowtrading.com";
const DIRECT_FEED_ORIGINS = (
  process.env.NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS ||
  "https://app.hyperpax.xyz"
)
  .split(",")
  .map((origin) => origin.trim());
export function usesDirectMarketFeeds(origin?: string): boolean {
  const currentOrigin =
    origin ?? (typeof window === "undefined" ? "" : window.location.origin);
  return DIRECT_FEED_ORIGINS.includes(currentOrigin);
}
export function getFeedOrigin(origin?: string): string {
  if (usesDirectMarketFeeds(origin)) return `${PUBLIC_FEED_ORIGIN}/api`;
  return "/api/price-feed";
}
export const FEED_ORIGIN = getFeedOrigin();
export const MARKET_DATA_URL = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL
  ? `${new URL(process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL).origin}/api`
  : "";
export const FEED_SYMBOLS = [
  "BTC",
  "ETH",
  "SOL",
  "AVAX",
  "LINK",
  "HYPE",
  "XRP",
  "ASTER",
  "TRUMP",
  "BNB",
  "SID",
  "PAX",
  "TSLA",
  "NVDA",
  "GOOGL",
  "XAU",
  "NAS100",
  "SPX500",
] as const;
let catalogFeedSymbols: readonly string[] = FEED_SYMBOLS;

/** Replace the bootstrap set atomically when the validated gateway catalog loads. */
export function registerCandleFeedSymbols(symbols: readonly string[]): void {
  const next = symbols.map((symbol) => {
    if (!/^[A-Za-z0-9]+$/.test(symbol))
      throw new TypeError(`Invalid candle feed symbol: ${symbol}`);
    return symbol.toUpperCase();
  });
  if (new Set(next).size !== next.length)
    throw new TypeError("Duplicate candle feed symbol");
  catalogFeedSymbols = next;
}

export function getCandleFeedSymbols(): readonly string[] {
  return [...catalogFeedSymbols];
}
export function normalizeFeedSymbol(symbol: string): string {
  return symbol
    .toUpperCase()
    .replace(/^PERP_/, "")
    .replace(/_USDC$/, "")
    .replace(/\/USD$/, "");
}
export function candleFeedUrl(
  endpoint: "config" | "symbols" | "history",
  symbol?: string,
  params?: URLSearchParams,
): string {
  const selected =
    symbol ??
    (catalogFeedSymbols.includes("SID") ? "SID" : catalogFeedSymbols[0]);
  if (!selected)
    throw new Error("Candle feed unavailable: market catalog is empty");
  const normalized = normalizeFeedSymbol(selected);
  if (
    !/^[A-Z0-9]+$/.test(normalized) ||
    !catalogFeedSymbols.includes(normalized)
  )
    throw new Error(`Candle feed unavailable for unlisted symbol: ${symbol}`);
  if (!["config", "symbols", "history"].includes(endpoint))
    throw new Error(`Unsupported candle feed endpoint: ${endpoint}`);
  const query = new URLSearchParams(params);
  if (endpoint !== "config") query.set("symbol", normalized);
  return `${getFeedOrigin()}/${normalized.toLowerCase()}/${endpoint}${query.size ? `?${query}` : ""}`;
}

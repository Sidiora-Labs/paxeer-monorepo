/** Public, rate-limited read feeds. No API keys belong in the browser. */
const configuredFeed =
  process.env.NEXT_PUBLIC_MARKET_PROXY_HOST || "data.feed.chainflowtrading.com";
const feedOrigin = new URL(
  configuredFeed.includes("://") ? configuredFeed : `https://${configuredFeed}`,
);
if (
  !["http:", "https:"].includes(feedOrigin.protocol) ||
  feedOrigin.username ||
  feedOrigin.password ||
  feedOrigin.pathname !== "/" ||
  feedOrigin.search ||
  feedOrigin.hash
) {
  throw new Error(
    "NEXT_PUBLIC_MARKET_PROXY_HOST must be a host or bare HTTP(S) origin",
  );
}
export const FEED_ORIGIN = feedOrigin.origin;
export const MARKET_DATA_URL = `${FEED_ORIGIN}/api/markets`;
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
  return `${FEED_ORIGIN}/api/${normalized.toLowerCase()}/${endpoint}${query.size ? `?${query}` : ""}`;
}

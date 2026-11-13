import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer } from "node:http";
import { once } from "node:events";
import type { AddressInfo } from "node:net";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";
import type { PerpsDatafeed } from "../src/core/datafeed/perps";
import type {
  Bar,
  HistoryMetadata,
  LibrarySymbolInfo,
  ResolutionString,
  SearchSymbolResultItem,
} from "../public/static/charting_library/charting_library";

// Committed in-memory peers for the gateway GraphQL and Chainflow UDF contracts.
// All requests use the production HTTP transport against these loopback routes.
const markets: GatewayMarketRow[] = [
  {
    marketId: 101,
    symbol: "DOGE",
    name: "Dogecoin",
    oracleSymbol: "DOGE/USD",
    szDecimals: 0,
    pxDecimals: 5,
    tickSize: "1",
    lotSize: "1",
    tierClass: 0,
    maxLeverage: "20",
    enabled: true,
    createdAt: "2026-01-01T00:00:00.000Z",
    latestPrice: { price: "0.125", onchainTimestamp: 1767225600 },
    fundingRate: null,
  },
];
const missingMarkets = ["ADA", "APT", "SUI"];
for (const [index, symbol] of missingMarkets.entries()) {
  markets.push({
    ...markets[0]!,
    marketId: 102 + index,
    symbol,
    name: symbol,
    oracleSymbol: `${symbol}/USD`,
  });
}
const history = {
  s: "ok",
  t: [1767225600, 1767225660],
  o: [0.12, 0.125],
  h: [0.13, 0.135],
  l: [0.115, 0.12],
  c: [0.125, 0.13],
  v: [12000, 15000],
};
const requests: URL[] = [];
let catalogAnswer: unknown = { markets };
let origin: string;
const envNames = [
  "NEXT_PUBLIC_PERPS_GATEWAY_URL",
  "NEXT_PUBLIC_PAXSCAN_BASE_URL",
  "NEXT_PUBLIC_PAXSCAN_API_URL",
  "NEXT_PUBLIC_PAXSCAN_WS_URL",
  "NEXT_PUBLIC_MARKET_PROXY_HOST",
  "NEXT_PUBLIC_PAXEER_RPC_URLS",
  "NEXT_PUBLIC_PAXEER_RPC_URL",
] as const;
const previousEnv = new Map(envNames.map((name) => [name, process.env[name]]));

const producer = createServer(async (request, response) => {
  const url = new URL(request.url!, origin);
  requests.push(url);
  response.setHeader("Content-Type", "application/json");
  if (request.method === "POST" && url.pathname === "/graphql") {
    let body = "";
    for await (const chunk of request) body += chunk;
    const operation = JSON.parse(body);
    if (!operation.query.includes("query GetMarkets")) {
      response
        .writeHead(400)
        .end(JSON.stringify({ errors: [{ message: "Unknown operation" }] }));
      return;
    }
    response.end(JSON.stringify({ data: catalogAnswer }));
    return;
  }
  if (url.pathname === "/api/doge/config") {
    response.end(
      JSON.stringify({
        supported_resolutions: ["1", "5"],
        supports_time: true,
      }),
    );
    return;
  }
  if (
    url.pathname === "/api/doge/symbols" &&
    url.searchParams.get("symbol") === "DOGE"
  ) {
    response.end(
      JSON.stringify({
        name: "DOGE",
        ticker: "DOGE",
        description: "Dogecoin",
        pricescale: 100000,
        supported_resolutions: ["1", "5"],
      }),
    );
    return;
  }
  if (
    url.pathname === "/api/doge/history" &&
    url.searchParams.get("symbol") === "DOGE"
  ) {
    response.end(
      JSON.stringify(
        url.searchParams.get("from") === "1"
          ? { s: "no_data", nextTime: 0 }
          : history,
      ),
    );
    return;
  }
  if (
    url.pathname === "/api/apt/symbols" ||
    url.pathname === "/api/apt/history"
  ) {
    response.end(
      JSON.stringify({ s: "error", errmsg: "No candle producer for APT" }),
    );
    return;
  }
  if (url.pathname === "/api/sui/symbols") {
    response.end(JSON.stringify({ name: "BTC", ticker: "BTC" }));
    return;
  }
  if (url.pathname === "/api/sui/history") {
    response.end(JSON.stringify({ error: "No candle producer for SUI" }));
    return;
  }
  response.writeHead(404).end(JSON.stringify({ error: "No candle feed" }));
});

before(async () => {
  producer.listen(0, "127.0.0.1");
  await once(producer, "listening");
  origin = `http://127.0.0.1:${(producer.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_MARKET_PROXY_HOST = origin;
  delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
  delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
});

after(async () => {
  producer.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    producer.close((error) => (error ? reject(error) : resolve())),
  );
  for (const name of envNames) {
    const previous = previousEnv.get(name);
    if (previous === undefined) delete process.env[name];
    else process.env[name] = previous;
  }
});

function resolveSymbol(
  feed: PerpsDatafeed,
  symbol: string,
): Promise<LibrarySymbolInfo> {
  return new Promise((resolve, reject) =>
    feed.resolveSymbol(symbol, resolve, (message) =>
      reject(new Error(message)),
    ),
  );
}

function bars(
  feed: PerpsDatafeed,
  symbol: LibrarySymbolInfo,
  from = 1767225600,
): Promise<{ bars: Bar[]; meta: HistoryMetadata }> {
  return new Promise((resolve, reject) =>
    feed.getBars(
      symbol,
      "1" as ResolutionString,
      { from, to: 1767225720, countBack: 2, firstDataRequest: true },
      (bars, meta) => resolve({ bars, meta: meta ?? {} }),
      (message) => reject(new Error(message)),
    ),
  );
}

async function dynamicCatalogReachesChartHistory(): Promise<void> {
  const { loadPerpsMarkets } = await import(
    "../src/hooks/perps/usePerpsMarkets"
  );
  const { isVenueSymbol } = await import("../src/core/net/venueStream");
  const { PerpsDatafeed } = await import("../src/core/datafeed/perps");
  const { FEED_ORIGIN } = await import("../src/core/feeds");
  const loaded = await loadPerpsMarkets();
  assert.deepEqual(
    loaded.catalog.map((market) => market.symbol),
    markets.map((market) => market.symbol),
  );
  assert.ok(loaded.tradeable.some((market) => market.symbol === "DOGE"));
  assert.ok(isVenueSymbol("DOGE"));
  assert.equal(FEED_ORIGIN, origin);

  const chart = new PerpsDatafeed();
  const config = await new Promise<
    Parameters<Parameters<PerpsDatafeed["onReady"]>[0]>[0]
  >((resolve) => chart.onReady(resolve));
  assert.deepEqual(config.supported_resolutions, ["1", "5"]);
  const found = await new Promise<SearchSymbolResultItem[]>((resolve) =>
    chart.searchSymbols("dog", "", "", resolve),
  );
  assert.equal(found.length, 1);
  assert.equal(found[0]?.symbol, "DOGE");
  const symbol = await resolveSymbol(chart, "PERP_DOGE_USDC");
  assert.equal(symbol.ticker, "DOGE");
  assert.equal(symbol.pricescale, 100000);
  const result = await bars(chart, symbol);
  assert.deepEqual(result, {
    bars: history.t.map((time, index) => ({
      time: time * 1000,
      open: history.o[index],
      high: history.h[index],
      low: history.l[index],
      close: history.c[index],
      volume: history.v[index],
    })),
    meta: { noData: false },
  });
  const historyRequest = requests.find(
    (url) => url.pathname === "/api/doge/history",
  );
  assert.ok(historyRequest);
  assert.equal(historyRequest.searchParams.get("symbol"), "DOGE");
  assert.equal(historyRequest.searchParams.get("resolution"), "1");
  assert.equal(historyRequest.searchParams.get("countback"), "2");
  assert.deepEqual(await bars(chart, symbol, 1), {
    bars: [],
    meta: { noData: true, nextTime: 0 },
  });
}

async function missingProducerFeedIsExplicit(): Promise<void> {
  const { PerpsDatafeed } = await import("../src/core/datafeed/perps");
  const { loadPerpsMarkets } = await import(
    "../src/hooks/perps/usePerpsMarkets"
  );
  await loadPerpsMarkets();
  const chart = new PerpsDatafeed();
  const doge = await resolveSymbol(chart, "DOGE/USD");
  for (const symbol of missingMarkets) {
    await assert.rejects(
      resolveSymbol(chart, symbol),
      new RegExp(`Candle feed unavailable for ${symbol}`),
    );
  }
  for (const symbol of missingMarkets) {
    await assert.rejects(
      bars(chart, { ...doge, name: symbol, ticker: symbol }),
      new RegExp(`Candle feed unavailable for ${symbol}`),
    );
  }
  assert.ok(requests.some((url) => url.pathname === "/api/ada/symbols"));
  assert.ok(requests.some((url) => url.pathname === "/api/apt/symbols"));
  assert.ok(requests.some((url) => url.pathname === "/api/sui/symbols"));
}

async function catalogRegistrationPreservesRouteValidation(): Promise<void> {
  const { loadPerpsMarkets } = await import(
    "../src/hooks/perps/usePerpsMarkets"
  );
  const { candleFeedUrl, getCandleFeedSymbols } = await import(
    "../src/core/feeds"
  );
  const { registerVenueSymbols, isVenueSymbol } = await import(
    "../src/core/net/venueStream"
  );
  await loadPerpsMarkets();
  for (const symbol of [
    "../admin",
    "DOGE/../../admin",
    "%2fadmin",
    "DOGE?admin",
    "DOGE#admin",
    "DOGE@kline",
    " DOGE",
    "DOGE\\admin",
  ]) {
    assert.throws(() => candleFeedUrl("history", symbol));
    assert.throws(() => registerVenueSymbols(["ETH", symbol]));
    assert.ok(
      isVenueSymbol("DOGE"),
      "failed registration preserves the venue catalog",
    );
    assert.ok(
      getCandleFeedSymbols().includes("DOGE"),
      "failed registration preserves chart routes",
    );
  }
  assert.throws(() => candleFeedUrl("history", "UNLISTED"));
  assert.throws(() => registerVenueSymbols(["DOGE", "doge"]), /Duplicate/);
  const query = new URLSearchParams({ symbol: "../admin", resolution: "1" });
  assert.equal(
    new URL(candleFeedUrl("history", "DOGE/USD", query)).searchParams.get(
      "symbol",
    ),
    "DOGE",
  );
  assert.equal(query.get("symbol"), "../admin", "caller query is not mutated");

  catalogAnswer = {
    markets: [{ ...markets[0]!, marketId: -1, symbol: "INVALID" }],
  };
  await assert.rejects(loadPerpsMarkets());
  assert.ok(getCandleFeedSymbols().includes("DOGE"));
  catalogAnswer = {
    markets: markets.filter((market) => market.symbol !== "DOGE"),
  };
  await loadPerpsMarkets();
  assert.equal(isVenueSymbol("DOGE"), false);
  assert.throws(() => candleFeedUrl("history", "DOGE"), /unlisted/);
  catalogAnswer = { markets: [] };
  await loadPerpsMarkets();
  assert.deepEqual(getCandleFeedSymbols(), []);
  assert.throws(() => candleFeedUrl("config"), /catalog is empty/);
  catalogAnswer = { markets };
  await loadPerpsMarkets();
}

async function publicRpcDefaultsReachChainConsumer(): Promise<void> {
  const { getPaxeerRpcUrls } = await import("../src/core/rpc");
  const { paxeerChain } = await import("../src/core/chain");
  const expected = [
    ...Array.from(
      { length: 14 },
      (_, index) => `https://api${index + 1}.mainnet-beta.paxeer.network`,
    ),
    "https://api-mainnet-beta.paxeer.network",
  ];
  assert.deepEqual(getPaxeerRpcUrls(), expected);
  assert.deepEqual(paxeerChain.rpcUrls.default.http, expected);
  assert.equal(paxeerChain.id, 125);
  const returned = getPaxeerRpcUrls();
  returned.pop();
  assert.deepEqual(
    getPaxeerRpcUrls(),
    expected,
    "callers cannot mutate defaults",
  );
  for (const endpoint of expected) {
    const url = new URL(endpoint);
    assert.equal(url.protocol, "https:");
    assert.equal(url.pathname, "/");
    assert.equal(url.search, "");
  }
}

async function operatorOverridesPreserveLegacyFiltering(): Promise<void> {
  const { getPaxeerRpcUrls } = await import("../src/core/rpc");
  const defaults = getPaxeerRpcUrls();
  const first = `${origin}/operator-one`;
  const second = `${origin}/operator-two`;
  const retired = [
    "https://rpcpaxeer.online",
    "https://old.rpcpaxeer.online",
    "https://api.hyperpax.xyz",
    "https://stats.paxscan.io",
  ];
  try {
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = second;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = ` ${first},,${retired.join(",")},${first}, ${second} `;
    assert.deepEqual(getPaxeerRpcUrls(), [first, second]);
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    assert.deepEqual(getPaxeerRpcUrls(), [second, ...defaults]);
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = retired.join(",");
    assert.deepEqual(getPaxeerRpcUrls(), [second, ...defaults]);
    for (const endpoint of retired) {
      process.env.NEXT_PUBLIC_PAXEER_RPC_URL = endpoint;
      assert.deepEqual(getPaxeerRpcUrls(), defaults);
    }
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
    assert.deepEqual(getPaxeerRpcUrls(), defaults);
  } finally {
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
  }
  assert.equal(
    requests.some((url) => url.pathname.startsWith("/operator-")),
    false,
    "configuration checks never probe RPC availability",
  );
}

test(dynamicCatalogReachesChartHistory);
test(missingProducerFeedIsExplicit);
test(catalogRegistrationPreservesRouteValidation);
test(publicRpcDefaultsReachChainConsumer);
test(operatorOverridesPreserveLegacyFiltering);

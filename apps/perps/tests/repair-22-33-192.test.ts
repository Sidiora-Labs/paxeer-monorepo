import assert from "node:assert/strict";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import { after, before, test } from "node:test";
import React, {
  createElement,
  isValidElement,
  type ReactElement,
  type ReactNode,
} from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  Area,
  AreaChart,
  DefaultTooltipContent,
  Line,
  LineChart,
  Tooltip,
  type TooltipProps,
} from "recharts";
import type {
  EnrichedGlobalStats,
  MarkPriceRecord,
  OiSnapshot,
  ProtocolSnapshotRecord,
} from "../src/types/perps";
import type { TickerWire } from "../src/hooks/perps/useRealtimePrices";

const { getTooltipItem } = createRequire(import.meta.url)(
  "recharts/lib/util/ChartUtils",
) as typeof import("recharts/types/util/ChartUtils");
const DAY_MS = 86_400_000;
const latestTime = Date.parse("2026-01-03T12:00:00Z");
const enriched: EnrichedGlobalStats = {
  onchain: {
    totalMarkets: 3,
    totalPositions: 5,
    openPositions: 2,
    totalTrades: 10,
    totalLiquidations: 0,
    totalVolume: "5000000000000000000000",
    totalUsers: 4,
    indexerBlock: 200,
  },
  orderly: {
    perp_volume_ytd: 5000,
    perp_volume_ltd: 5000,
    perp_volume_today: 100,
    perp_volume_last_1_day: 100,
    perp_volume_last_7_days: 700,
    perp_volume_last_30_days: 3000,
  },
};
const ticker: TickerWire = {
  symbol: "PERP_BTC_USDC",
  paxeerSymbol: "BTC",
  mark_price: 110,
  index_price: 109,
  open_interest: 70,
  last_funding_rate_hourly: null,
  "24h_open": 100,
  "24h_close": 110,
  "24h_high": 111,
  "24h_low": 99,
  "24h_amount": 100,
  "24h_volume": 1,
};
const markPriceHistory: MarkPriceRecord[] = [
  {
    id: 1,
    marketId: 0,
    markPrice: "110000000000000000000",
    indexPrice: "109000000000000000000",
    baseReserve: "1000000000000000000",
    quoteReserve: "110000000000000000000",
    blockNumber: 200,
    txHash: "0x" + "01".repeat(32),
    blockTimestamp: new Date(latestTime).toISOString(),
  },
];
const oiSnapshots: OiSnapshot[] = [
  {
    id: 2,
    marketId: 0,
    longOi: "40000000000000000000",
    shortOi: "30000000000000000000",
    deltaUsd: "1000000000000000000",
    isIncrease: true,
    blockNumber: 200,
    txHash: "0x" + "02".repeat(32),
    blockTimestamp: new Date(latestTime).toISOString(),
  },
];
function snapshot(id: number, ts: number, usd: bigint): ProtocolSnapshotRecord {
  return {
    id,
    totalPositions: "5",
    totalOpenPositions: "2",
    totalMarkets: "3",
    tvlUsd: (usd * 10n ** 6n).toString(),
    insuranceTotalUsd: "0",
    onchainTimestamp: ts / 1000,
    blockNumber: id,
    blockTimestamp: new Date(ts).toISOString(),
  };
}
let protocolSnapshots: ProtocolSnapshotRecord[] = [];
let origin: string;
let modules: Awaited<ReturnType<typeof loadModules>>;
const exchanges: { query: string; variables?: Record<string, unknown> }[] = [];
const previousReact = Object.getOwnPropertyDescriptor(globalThis, "React");
const previousEnv = new Map<string, string | undefined>();

async function loadModules() {
  return {
    client: await import("../src/core/graphql/client"),
    queries: await import("../src/hooks/perps/useSubgraph"),
    config: await import("../src/core/perps/config"),
    liquid: await import("../src/widgets/analytics/AnalyticsLiquidStat"),
    tvl: await import("../src/widgets/analytics/ProtocolTvlChart"),
    mark: await import("../src/widgets/analytics/MarkPriceChart"),
    oi: await import("../src/widgets/analytics/OiChart"),
    prices: await import("../src/hooks/perps/useRealtimePrices"),
    transport: await import("../src/core/net/fetch"),
  };
}
const server = createServer((req, res) => {
  void (async () => {
    res.setHeader("Content-Type", "application/json");
    if (req.method === "GET" && req.url === "/tickers") {
      res.end(JSON.stringify({ data: [ticker] }));
      return;
    }
    assert.equal(req.method, "POST");
    assert.equal(req.url, "/graphql");
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(
      Buffer.concat(chunks).toString("utf8"),
    ) as (typeof exchanges)[number];
    exchanges.push(body);
    const { queries } = modules;
    let data: object;
    if (body.query === queries.PROTOCOL_SNAPSHOTS_QUERY) {
      assert.deepEqual(body.variables, { limit: 100 });
      data = { protocolSnapshots };
    } else if (body.query === queries.ENRICHED_GLOBAL_STATS_QUERY) {
      data = { enrichedGlobalStats: enriched };
    } else if (body.query === queries.MARK_PRICE_HISTORY_QUERY) {
      assert.deepEqual(body.variables, { marketId: 0, limit: 100 });
      data = { markPriceHistory };
    } else if (body.query === queries.OI_SNAPSHOTS_QUERY) {
      assert.deepEqual(body.variables, { marketId: 0, limit: 100 });
      data = { oiSnapshots };
    } else {
      throw new Error("Unexpected GraphQL query in analytics fixture");
    }
    res.end(JSON.stringify({ data }));
  })().catch((error: unknown) => {
    res.statusCode = 400;
    res.end(JSON.stringify({ errors: [{ message: String(error) }] }));
  });
});

before(async () => {
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  for (const [key, value] of Object.entries({
    NEXT_PUBLIC_PERPS_GATEWAY_URL: origin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: origin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${origin}/api/v2`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: origin.replace("http:", "ws:"),
  })) {
    previousEnv.set(key, process.env[key]);
    process.env[key] = value;
  }
  // Match Next's JSX runtime for shared UI modules under the declared tsx gate.
  Object.defineProperty(globalThis, "React", {
    value: React,
    configurable: true,
  });
  modules = await loadModules();
});
after(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
  if (previousReact) Object.defineProperty(globalThis, "React", previousReact);
  else Reflect.deleteProperty(globalThis, "React");
  for (const [key, value] of previousEnv) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

async function loadLiquidProps(records: ProtocolSnapshotRecord[]) {
  protocolSnapshots = records;
  const { client, queries, config, transport, prices } = modules;
  const [history, stats, tickers] = await Promise.all([
    client.gqlQuery<{ protocolSnapshots: ProtocolSnapshotRecord[] }>(
      config.INDEXER_GRAPHQL,
      queries.PROTOCOL_SNAPSHOTS_QUERY,
      { limit: 100 },
    ),
    client.gqlQuery<{ enrichedGlobalStats: EnrichedGlobalStats }>(
      config.INDEXER_GRAPHQL,
      queries.ENRICHED_GLOBAL_STATS_QUERY,
    ),
    transport.fetchJsonWithPolicy<{ data: TickerWire[] }>(`${origin}/tickers`),
  ]);
  return {
    snapshots: history.protocolSnapshots,
    enriched: stats.enrichedGlobalStats,
    tickers: prices.tickersFromWire(tickers.data),
  };
}
async function renderLiquid(records: ProtocolSnapshotRecord[]) {
  return renderToStaticMarkup(
    createElement(
      modules.liquid.AnalyticsLiquidStatContent,
      await loadLiquidProps(records),
    ),
  );
}
function text(html: string) {
  return html.replace(/<[^>]*>/g, "");
}

async function protocolSnapshotsSixDecimalTvl() {
  // Match the custody transfers and raw snapshot asserted by projections_test.go.
  const inflow = 5_000_000n;
  const outflow = 1_500_000n;
  const rawTvl = (inflow - outflow).toString();
  assert.equal(rawTvl, "3500000");
  const props = await loadLiquidProps([
    { ...snapshot(2, latestTime, 3n), tvlUsd: rawTvl },
    { ...snapshot(1, latestTime - DAY_MS, 2n), tvlUsd: "2500000" },
  ]);
  assert.equal(props.snapshots[0].tvlUsd, "3500000");
  assert.equal(props.snapshots[1].tvlUsd, "2500000");
  for (const isMobile of [false, true]) {
    const html = renderToStaticMarkup(
      createElement(modules.liquid.AnalyticsLiquidStatContent, props),
    );
    assert.match(text(html), /TVL · live/);
    assert.match(text(html), /\$3\.50/);
    assert.match(text(html), /▲ 40\.00%/);
    assert.doesNotMatch(html, /24h change unavailable/);
    const tree = modules.tvl.ProtocolTvlChartContent({
      data: props.snapshots,
      isLoading: false,
      isMobile,
    });
    const charts = elements<{ data: { tvl: number }[] }>(tree, AreaChart);
    assert.equal(charts.length, 1);
    assert.deepEqual(
      charts[0].props.data.map((point) => point.tvl),
      [3.5, 2.5],
    );
    const series = elements<{ dataKey: string; name: string }>(tree, Area);
    assert.equal(series.length, 1);
    assert.equal(series[0].props.dataKey, "tvl");
    assert.equal(series[0].props.name, "TVL");
    assert.ok(
      elements<{ style?: React.CSSProperties }>(tree, "div").some(
        (element) => element.props.style?.height === (isMobile ? 200 : 300),
      ),
    );
  }
}

async function f008AggregateTvlIncludesCollateralAbsentFromRecentEvents() {
  // Aggregate snapshots include unchanged collateral (300 + 200) at both endpoints;
  // only the first token changed (500 -> 600) during the recent event window.
  const baseline = snapshot(1, latestTime - DAY_MS, 500n + 300n + 200n);
  const current = snapshot(3, latestTime, 600n + 300n + 200n);
  const html = await renderLiquid([
    current,
    baseline,
    snapshot(2, latestTime - DAY_MS / 2, 1050n),
  ]);
  assert.match(text(html), /TVL · live/);
  assert.match(text(html), /1,100/);
  assert.match(text(html), /▲ 10\.00%/);
  assert.doesNotMatch(html, /24h change unavailable/);
  assert.ok(
    exchanges.some(
      (request) => request.query === modules.queries.PROTOCOL_SNAPSHOTS_QUERY,
    ),
  );
  assert.equal(
    exchanges.some(
      (request) =>
        request.query === modules.queries.VAULT_BALANCE_HISTORY_QUERY,
    ),
    false,
  );

  const decreased = await renderLiquid([
    snapshot(4, latestTime - DAY_MS, 2000n),
    snapshot(5, latestTime, 1000n),
  ]);
  assert.match(text(decreased), /▼ 50\.00%/);
  const zero = await renderLiquid([baseline, snapshot(6, latestTime, 0n)]);
  assert.match(text(zero), /TVL · live/);
  assert.match(text(zero), /▼ 100\.00%/);
}

async function f008MissingDailyHistoryNeverInventsAPercentage() {
  const current = snapshot(3, latestTime, 1100n);
  const cases: { records: ProtocolSnapshotRecord[]; label: string }[] = [
    { records: [], label: "TVL unavailable" },
    { records: [current], label: "TVL: $1,100" },
    {
      records: [snapshot(1, latestTime - DAY_MS / 2, 1000n), current],
      label: "TVL: $1,100",
    },
    {
      records: [snapshot(1, latestTime - 2 * DAY_MS, 1000n), current],
      label: "TVL: $1,100",
    },
    {
      records: [snapshot(1, latestTime - DAY_MS + 1, 1000n), current],
      label: "TVL: $1,100",
    },
    {
      records: [snapshot(1, latestTime - DAY_MS - 1, 1000n), current],
      label: "TVL: $1,100",
    },
    {
      records: [snapshot(1, latestTime - DAY_MS, 0n), current],
      label: "TVL: $1,100",
    },
    {
      records: [current, { ...current, id: 4, tvlUsd: "invalid" }],
      label: "TVL unavailable",
    },
    {
      records: [{ ...current, blockTimestamp: "invalid" }],
      label: "TVL unavailable",
    },
  ];
  for (const { records, label } of cases) {
    const html = await renderLiquid(records);
    const status = html.match(/<p role="status"[^>]*>(.*?)<\/p>/s);
    assert.ok(
      status,
      `Missing history must be labeled for ${JSON.stringify(records)}`,
    );
    assert.equal(text(status[1]), `${label} · 24h change unavailable`);
    assert.doesNotMatch(status[1], /%|NaN|Infinity|▲|▼/);
    assert.doesNotMatch(text(html), /TVL · live/);
    assert.match(text(html), /24h Volume · live/);
  }
  const fractional = await renderLiquid([
    { ...snapshot(1, latestTime - DAY_MS, 1n), tvlUsd: "1250000" },
    { ...snapshot(2, latestTime, 1n), tvlUsd: "1875000" },
  ]);
  assert.match(text(fractional), /TVL · live/);
  assert.match(text(fractional), /\$1\.88/);
  assert.match(text(fractional), /▲ 50\.00%/);
  assert.doesNotMatch(fractional, /24h change unavailable/);
}

function elements<P>(node: ReactNode, type: unknown): ReactElement<P>[] {
  if (Array.isArray(node))
    return node.flatMap((child) => elements<P>(child, type));
  if (!isValidElement<P & { children?: ReactNode }>(node)) return [];
  return [
    ...(node.type === type ? [node] : []),
    ...elements<P>(node.props.children, type),
  ];
}
function tooltipText(tree: ReactNode, chartType: unknown, seriesType: unknown) {
  const charts = elements<{ data: Record<string, string | number>[] }>(
    tree,
    chartType,
  );
  const tooltips = elements<TooltipProps<number, string>>(tree, Tooltip);
  const series = elements<{ dataKey: string; name: string }>(tree, seriesType);
  assert.equal(charts.length, 1);
  assert.equal(tooltips.length, 1);
  assert.equal(series.length, 2);
  const point = charts[0].props.data[0];
  const payload = series.map((item) => getTooltipItem(item, point));
  const html = renderToStaticMarkup(
    createElement(DefaultTooltipContent<number, string>, {
      ...tooltips[0].props,
      label: point.date,
      payload,
    }),
  );
  const labels = [
    ...html.matchAll(
      /<span class="recharts-tooltip-item-name"[^>]*>(.*?)<\/span>/g,
    ),
  ].map((match) => text(match[1]));
  const values = [
    ...html.matchAll(
      /<span class="recharts-tooltip-item-value"[^>]*>(.*?)<\/span>/g,
    ),
  ].map((match) => text(match[1]));
  return { labels, values, point, series };
}

async function f009MarkAndIndexTooltipsMatchTheirPlottedSeries() {
  const { client, queries, config, mark } = modules;
  const data = await client.gqlQuery<{ markPriceHistory: MarkPriceRecord[] }>(
    config.INDEXER_GRAPHQL,
    queries.MARK_PRICE_HISTORY_QUERY,
    { marketId: 0, limit: 100 },
  );
  for (const isMobile of [false, true]) {
    const result = tooltipText(
      mark.MarkPriceChartContent({
        data: data.markPriceHistory,
        isLoading: false,
        isMobile,
      }),
      LineChart,
      Line,
    );
    assert.deepEqual(
      result.series.map((item) => [item.props.dataKey, item.props.name]),
      [
        ["markPrice", "Mark Price"],
        ["indexPrice", "Index Price"],
      ],
    );
    assert.deepEqual(result.labels, ["Mark Price", "Index Price"]);
    assert.equal(result.point.markPrice, 110);
    assert.equal(result.point.indexPrice, 109);
    assert.deepEqual(
      result.values,
      [110, 109].map(config.ANALYTICS_AXIS_FORMATTERS.compactUsd),
    );
  }
}

async function f010LongAndShortOiTooltipsMatchTheirPlottedSeries() {
  const { client, queries, config, oi } = modules;
  const data = await client.gqlQuery<{ oiSnapshots: OiSnapshot[] }>(
    config.INDEXER_GRAPHQL,
    queries.OI_SNAPSHOTS_QUERY,
    { marketId: 0, limit: 100 },
  );
  for (const isMobile of [false, true]) {
    const result = tooltipText(
      oi.OiChartContent({ data: data.oiSnapshots, isLoading: false, isMobile }),
      AreaChart,
      Area,
    );
    assert.deepEqual(
      result.series.map((item) => [item.props.dataKey, item.props.name]),
      [
        ["longOi", "Long OI"],
        ["shortOi", "Short OI"],
      ],
    );
    assert.deepEqual(result.labels, ["Long OI", "Short OI"]);
    assert.equal(result.point.longOi, 40);
    assert.equal(result.point.shortOi, 30);
    assert.deepEqual(
      result.values,
      [40, 30].map(config.ANALYTICS_AXIS_FORMATTERS.compactUsd),
    );
  }
}

test(protocolSnapshotsSixDecimalTvl);
test(f008AggregateTvlIncludesCollateralAbsentFromRecentEvents);
test(f008MissingDailyHistoryNeverInventsAPercentage);
test(f009MarkAndIndexTooltipsMatchTheirPlottedSeries);
test(f010LongAndShortOiTooltipsMatchTheirPlottedSeries);

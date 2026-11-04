import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { before, after, test } from "node:test";
import React, { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PositionRowInput } from "../src/core/perps/pnlHelpers";

const usdl = "0x85FcD13735F4309833A503EE804ea32395851479";
const token18 = "0x1111111111111111111111111111111111111111";
const unknownToken = "0x2222222222222222222222222222222222222222";
const account = "0x3333333333333333333333333333333333333333";
const wad = 10n ** 18n;
const large = 10n ** 40n;
const dailyTranscript = Array.from({ length: 90 }, (_, index) => ({
  date: new Date(Date.UTC(2026, 0, index + 1)).toISOString().slice(0, 10),
  pnl: index === 45 ? null : [large.toString(), "1e18", (-large).toString(), (-2n * wad).toString()][index] ?? "0",
  volume: (100n * wad).toString(),
  trades: 1,
}));
const vaultTranscript = [
  { tokenAddress: usdl.toLowerCase(), amount: "123456789", eventType: "deposit" },
  { tokenAddress: usdl.toUpperCase(), amount: "1000001", eventType: "withdraw" },
  { tokenAddress: token18, amount: "1234567890123456789", eventType: "deposit" },
  { tokenAddress: usdl, amount: "0", eventType: "withdraw" },
  { tokenAddress: unknownToken, amount: "1000000", eventType: "deposit" },
].map((event, index) => ({
  ...event, id: String(index), blockTimestamp: "2026-01-01T00:00:00Z",
}));
const position: PositionRowInput = {
  positionId: "42", marketId: "0", isLong: true,
  sizeUsd: (1000n * wad).toString(), collateralUsd: (100n * wad).toString(),
  leverage: (10n * wad).toString(), entryPrice: (100n * wad).toString(),
  collateralToken: usdl, collateralAmount: "100000000",
};
const holdings = [{
  symbol: "ETH", address: token18 as `0x${string}`, decimals: 18,
  vaultBalance: 1234567890123456789n, lockedBalance: 0n,
  walletBalance: 0n, available: 1234567890123456789n,
}];
const requests: string[] = [];
const failures: unknown[] = [];

// Committed wire fixtures, served over real HTTP to the production clients.
const server = createServer(async (request, response) => {
  try {
    assert.equal(request.url, "/graphql");
    assert.equal(request.method, "POST");
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString()) as {
      query: string; variables: Record<string, unknown>;
    };
    let data: unknown;
    if (body.query.includes("query UserDailyPnl")) {
      assert.deepEqual(body.variables, { address: account, limit: 90 });
      requests.push("dailyPnl");
      data = { userDailyPnl: dailyTranscript };
    } else if (body.query.includes("query GetVaultEvents")) {
      assert.deepEqual(body.variables, { userAddress: account, limit: 5, offset: 0 });
      requests.push("vaultEvents");
      data = { vaultEvents: vaultTranscript };
    } else if (body.query.includes("query GetPositions")) {
      assert.deepEqual(body.variables, { userAddress: account });
      requests.push("positions");
      data = { positions: [position] };
    } else {
      assert.fail(`Unexpected operation: ${body.query}`);
    }
    response.setHeader("Content-Type", "application/json");
    response.end(JSON.stringify({ data }));
  } catch (error) {
    failures.push(error);
    response.statusCode = 500;
    response.end(JSON.stringify({ errors: [{ message: String(error) }] }));
  }
});

before(async function fixtureOriginsBeforeProductionImports() {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  const origin = `http://127.0.0.1:${address.port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
});

after(async function closeFixtureTransport() {
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  assert.deepEqual(failures, []);
});

function unexpectedAction() {
  assert.fail("Server rendering must not dispatch a portfolio action");
}

async function readDailyHistory() {
  const { fetchDailyPnl } = await import("../src/hooks/perps/useDailyPnl");
  return fetchDailyPnl(account);
}

async function f019_exactWeiReachesCumulativeChart() {
  const result = await readDailyHistory();
  assert.equal(result.rows.length, 89);
  const { PnlChart } = await import("../src/widgets/portfolio/PnlChart");
  // Inspect the actual data passed to Recharts; ResponsiveContainer has no
  // server-side dimensions and does not serialize its chart children.
  const chart = PnlChart({ dailyPnl: result.rows });
  const data = chart.props.children.props.children.props.data as { cumPnl: number }[];
  assert.equal(data[0].cumPnl, 1e22);
  assert.equal(data[2].cumPnl, 1, "sum exact wei before converting to dollars");
  assert.equal(data[3].cumPnl, -1);
  assert.equal(data.at(-1)?.cumPnl, -1);
  assert.ok(requests.includes("dailyPnl"));
}
test(f019_exactWeiReachesCumulativeChart);

async function f022_vaultTransportPreservesNativeUnitsAndOptionalQuotes() {
  const { subgraphQuery, VAULT_EVENTS_QUERY } = await import("../src/hooks/perps/useSubgraph");
  const { vaultEvents } = await subgraphQuery<{ vaultEvents: typeof vaultTranscript }>(
    VAULT_EVENTS_QUERY, { userAddress: account, limit: 5, offset: 0 },
  );
  const { MobilePortfolioOverview } = await import("../src/widgets/portfolio/MobilePortfolioOverview");
  const props: React.ComponentProps<typeof MobilePortfolioOverview> = {
    address: null, totalVaultUsd: 0, totalPnl: 0, userStats: null,
    balances: holdings, trades: [], vaultEvents, isLoading: false,
    onRefresh: unexpectedAction, action: "deposit", onActionChange: unexpectedAction,
    selectedToken: usdl, onSelectedTokenChange: unexpectedAction,
    tokenOptions: [{ label: "USDL", value: usdl }], amount: "",
    onAmountChange: unexpectedAction, maxAmount: "0", isProcessing: false,
    txStep: "idle", error: null, success: null, onSubmit: unexpectedAction,
    isConnected: true, hasVault: true, isCreatingVault: false,
    onConnect: unexpectedAction, onCreateVault: unexpectedAction,
  };
  const activity = (prices?: Record<string, number>) => {
    const html = renderToStaticMarkup(createElement(MobilePortfolioOverview, {
      ...props, collateralPricesUsd: prices,
    }));
    return html.slice(html.indexOf("Deposits &amp; Withdrawals"));
  };
  const native = activity();
  assert.match(native, /123\.456789 USDL/);
  assert.match(native, /1\.000001 USDL/);
  assert.match(native, /1\.234567890123456789 ETH/);
  assert.match(native, />0 USDL</);
  assert.match(native, /Token metadata unavailable/);
  assert.ok(native.includes(unknownToken));
  assert.doesNotMatch(native, /\$/);
  const priced = activity({ [usdl]: 2 });
  assert.match(priced, /123\.456789 USDL/);
  assert.match(priced, /\$246\.91 USD/);
  assert.match(priced, /\$0\.00 USD/);
  assert.equal((priced.match(/ USD</g) ?? []).length, 3, "only quoted token events get USD values");
  assert.doesNotMatch(activity({ [usdl]: Number.NaN }), /\$/);
  assert.doesNotMatch(activity({ [usdl]: -1 }), /\$/);
  assert.ok(requests.includes("vaultEvents"));
}
test(f022_vaultTransportPreservesNativeUnitsAndOptionalQuotes);

async function overviewProps() {
  const history = await readDailyHistory();
  return {
    isConnected: true, hasVault: true, address: account,
    totalVaultUsd: 2468.12, unrealizedPnl: -123.45, openPositions: 2,
    realizedPnl: -61.37, totalTrades: 3,
    balances: [{
      ...holdings[0], symbol: "USDL", address: usdl as `0x${string}`,
      decimals: 6, vaultBalance: 987650000n
    }],
    dailyPnl: history.rows, dailyPnlLoading: false, dailyPnlError: null,
    dailyPnlSkippedDays: history.skippedDays, dailyPnlHistoryCapped: history.historyCapped,
    isLoading: false, privacy: false, onTogglePrivacy: unexpectedAction,
    onRefresh: unexpectedAction, onConnect: unexpectedAction,
    onDeposit: unexpectedAction, onWithdraw: unexpectedAction,
  };
}

function sections(html: string) {
  const sections = html.match(/<section\b[\s\S]*?<\/section>/g);
  assert.ok(sections);
  return sections;
}

async function f020_overviewDisclosesEstimateAndHistoryLimits() {
  const { PortfolioOverviewPanel } = await import("../src/widgets/portfolio/PortfolioOverviewPanel");
  const { PortfolioAssetsChart } = await import("../src/widgets/portfolio/PortfolioAssetsChart");
  const props = await overviewProps();
  assert.equal(props.dailyPnlSkippedDays, 1);
  assert.equal(props.dailyPnlHistoryCapped, true);
  const assets = sections(renderToStaticMarkup(createElement(PortfolioOverviewPanel, props)))[1];
  assert.match(assets, /PnL-only estimate/);
  assert.match(assets, /excludes deposits and withdrawals/);
  assert.match(assets, /History limited to the last 90 active days/);
  assert.match(assets, /1 day skipped: incomplete values/);
  const all = renderToStaticMarkup(createElement(PortfolioAssetsChart, {
    dailyPnl: props.dailyPnl, totalVaultUsd: props.totalVaultUsd,
    historyCapped: true, skippedDays: 2, range: "ALL",
  }));
  assert.match(all, /<option value="ALL" selected="">Last 90 active days<\/option>/);
  assert.match(all, /2 days skipped/);
  const uncapped = renderToStaticMarkup(createElement(PortfolioAssetsChart, {
    dailyPnl: props.dailyPnl, totalVaultUsd: props.totalVaultUsd, range: "ALL",
  }));
  assert.match(uncapped, /<option value="ALL" selected="">ALL<\/option>/);
  assert.doesNotMatch(uncapped, /History limited|days skipped/);
}
test(f020_overviewDisclosesEstimateAndHistoryLimits);

async function f021_privacyConcealsHoldingsAndDerivedPercentages() {
  const { PortfolioOverviewPanel } = await import("../src/widgets/portfolio/PortfolioOverviewPanel");
  const props = await overviewProps();
  const visible = sections(renderToStaticMarkup(createElement(PortfolioOverviewPanel, props)))[0];
  assert.match(visible, /987\.65/);
  assert.match(visible, /-5\.00%/);
  assert.match(visible, /aria-label="Hide balances"/);
  const hidden = sections(renderToStaticMarkup(createElement(PortfolioOverviewPanel, {
    ...props, privacy: true,
  })))[0];
  assert.doesNotMatch(hidden, /987\.65|5\.00%|123\.45|61\.37|2\.47K/);
  assert.doesNotMatch(hidden, /text-emerald|text-red/);
  assert.match(hidden, /aria-label="Show balances"/);
  assert.equal((hidden.match(/aria-label="Balance hidden"/g) ?? []).length, 5);
  assert.equal((hidden.match(/aria-hidden="true">••••••/g) ?? []).length, 5);
  const revealed = sections(renderToStaticMarkup(createElement(PortfolioOverviewPanel, props)))[0];
  assert.equal(revealed, visible, "showing balances restores the displayed amounts");
}
test(f021_privacyConcealsHoldingsAndDerivedPercentages);

async function f040_positionValuationRequiresValidPriceAndTiers() {
  const { subgraphQuery, POSITIONS_QUERY } = await import("../src/hooks/perps/useSubgraph");
  const { positions } = await subgraphQuery<{ positions: PositionRowInput[] }>(
    POSITIONS_QUERY, { userAddress: account },
  );
  const { nodeMarginTiers } = await import("./node-leverage-output");
  const { PortfolioPositionsPanel, buildPortfolioPositionRow, PositionRowItem } = await import(
    "../src/widgets/portfolio/PortfolioPositionsPanel"
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const renderPanel = () => renderToStaticMarkup(createElement(QueryClientProvider, { client },
    createElement(PortfolioPositionsPanel, {
      positions, isConnected: true, hasVault: true, onConnect: unexpectedAction,
    }),
  ));
  try {
    assert.match(renderPanel(), /Loading market margin tiers/);
    client.setQueryData(["perps", "marginTiers", 0], nodeMarginTiers);
    const missingMark = renderPanel();
    assert.match(missingMark, /Valuation unavailable/);
    assert.doesNotMatch(missingMark, /\+0\.00%/);
    assert.ok(!missingMark.slice(missingMark.indexOf('class="flex flex-col items-end"')).includes("text-emerald-400"));
    for (const mark of [undefined, 0, -1, Number.NaN, Number.POSITIVE_INFINITY]) {
      const row = buildPortfolioPositionRow(positions[0], "BTC", mark, nodeMarginTiers);
      assert.equal(row.valuation, null);
      assert.equal(row.markPrice, null);
      const html = renderToStaticMarkup(createElement(PositionRowItem, { row }));
      assert.match(html, /role="status"/);
      assert.match(html, /Valuation unavailable/);
      assert.doesNotMatch(html.slice(html.indexOf('class="flex flex-col items-end"')), /text-emerald-400|\+0\.00%/);
    }
    const withoutTiers = buildPortfolioPositionRow(positions[0], "BTC", 110, undefined);
    assert.equal(withoutTiers.valuation, null);
    assert.match(renderToStaticMarkup(createElement(PositionRowItem, { row: withoutTiers })), /Valuation unavailable/);
    const { tickerFromWire } = await import("../src/hooks/perps/useRealtimePrices");
    for (const [mark, pnl, pct] of [[110, 100n * wad, "100.00%"], [90, -100n * wad, "-100.00%"], [100, 0n, "0.00%"]] as const) {
      const ticker = tickerFromWire({
        symbol: "PERP_BTC_USDC", paxeerSymbol: "BTC", mark_price: mark,
        index_price: mark, open_interest: 0, last_funding_rate_hourly: null,
        "24h_open": 100, "24h_close": mark, "24h_high": 110,
        "24h_low": 90, "24h_amount": 0, "24h_volume": 0,
      });
      const row = buildPortfolioPositionRow(positions[0], "BTC", ticker.mark_price, nodeMarginTiers);
      assert.equal(row.valuation?.unrealizedPnl, pnl);
      const html = renderToStaticMarkup(createElement(PositionRowItem, { row }));
      assert.ok(html.includes(pct));
      assert.doesNotMatch(html, /Valuation unavailable/);
    }
    client.setQueryData(["perps", "marginTiers", 0], []);
    assert.match(renderPanel(), /Market margin tiers unavailable/);
    assert.ok(requests.includes("positions"));
  } finally {
    client.clear();
  }
}
test(f040_positionValuationRequiresValidPriceAndTiers);

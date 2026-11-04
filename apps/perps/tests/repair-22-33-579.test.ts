/**
 * Task 22.33.579: perps-default-disabled is proved by the named tests below.
 * Gateway fixture store -> HTTP -> production catalog query/codec -> real
 * QueryClient provider, trading page and recovery navigation. No wiring requests.
 * Baseline verification was not run: the ordered gate instructions require the
 * complete implementation before hf-build and prohibit a manual verify run.
 */
import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { after, before, test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "next/app";
import Router from "next/dist/shared/lib/router/router";
import { RouterContext } from "next/dist/shared/lib/router-context.shared-runtime";
import { createConfig, http, WagmiProvider } from "wagmi";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

type PageModule = typeof import("../src/page-modules/perps");
type MarketsModule = typeof import("../src/hooks/perps/usePerpsMarkets");
let page: PageModule;
let markets: MarketsModule;
let origin: string;
let wallet: ReturnType<typeof createConfig>;
let rows: GatewayMarketRow[] = [];
const queries: string[] = [];
const client = new QueryClient({ defaultOptions: { queries: { gcTime: Infinity } } });

// Committed in-memory gateway catalog, served through the real HTTP transport.
function gatewayRow(marketId: number, symbol: string, enabled: boolean): GatewayMarketRow {
  return {
    marketId, symbol, name: symbol, oracleSymbol: symbol,
    enabled, maxLeverage: "20", szDecimals: 3, pxDecimals: 4,
    tickSize: "100000000000000", lotSize: "1000000000000000",
    tierClass: 0, createdAt: null, latestPrice: null, fundingRate: null,
  };
}

const server = createServer((request, response) => {
  void (async () => {
    if (request.method !== "POST" || request.url !== "/graphql") {
      response.writeHead(404).end();
      return;
    }
    let body = "";
    for await (const chunk of request) body += chunk;
    const { query } = JSON.parse(body) as { query: string };
    queries.push(query);
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ data: { markets: rows } }));
  })().catch((error: unknown) => {
    response.writeHead(500).end(String(error));
  });
});

before(async () => {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object");
  origin = `http://127.0.0.1:${address.port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  Object.assign(globalThis, { React });
  // Next's real router reads its server-rendered bootstrap state at construction.
  Object.defineProperty(globalThis, "self", {
    configurable: true,
    value: { __NEXT_DATA__: { gssp: true } },
  });
  page = await import("../src/page-modules/perps");
  markets = await import("../src/hooks/perps/usePerpsMarkets");
  const { paxeerChain } = await import("../src/core/chain");
  wallet = createConfig({
    chains: [paxeerChain],
    transports: { [paxeerChain.id]: http(origin) },
    ssr: true,
  });
});

after(async () => {
  client.clear();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
  });
});

async function publishCatalog(next: GatewayMarketRow[]) {
  rows = next;
  const before = queries.length;
  const catalog = await client.fetchQuery({
    ...markets.perpsMarketsQueryOptions(), staleTime: 0,
  });
  assert.equal(queries.length, before + 1, "catalog travels through the gateway HTTP query");
  assert.match(queries.at(-1)!, /query GetMarkets/);
  return catalog;
}

function renderPage(market?: string | string[]) {
  const query = market === undefined ? {} : { market };
  const router = new Router("/perps", query, "/perps", {
    initialProps: { pageProps: {}, __N_SSP: true },
    pageLoader: undefined,
    App,
    Component: page.default,
    wrapApp: (component) => component,
    subscription: async () => { throw new Error("Server rendering must not navigate"); },
    isFallback: false,
  });
  const html = renderToStaticMarkup(
    React.createElement(RouterContext.Provider, { value: router },
      React.createElement(WagmiProvider, { config: wallet, reconnectOnMount: false },
        React.createElement(QueryClientProvider, { client }, React.createElement(page.default)),
      ),
    ),
  );
  assert.deepEqual(router.query, query, "rendering never rewrites explicit trade intent");
  return html;
}

async function disabledPaxUsesLiveCatalogDefault() {
  const catalog = await publishCatalog([
    gatewayRow(11, "PAX", false), gatewayRow(1, "ETH", true), gatewayRow(0, "BTC", true),
  ]);
  assert.equal(page.tradingMarket(undefined, catalog.tradeable)?.symbol, "BTC");
  assert.equal(page.tradingMarket(undefined, catalog.catalog)?.symbol, "BTC");
  const html = renderPage();
  assert.doesNotMatch(html, /The selected market is unavailable/);
  assert.match(html, /BTC\/USD/);
}

async function enabledPaxRemainsPreferredWithoutSelection() {
  const catalog = await publishCatalog([gatewayRow(0, "BTC", true), gatewayRow(11, "PAX", true)]);
  assert.equal(page.tradingMarket(undefined, catalog.tradeable)?.symbol, "PAX");
  assert.equal(page.tradingMarket("btc", catalog.tradeable)?.symbol, "BTC");
  assert.match(renderPage(), /PAX\/USD/);
  assert.match(renderPage("btc"), /BTC\/USD/);
}

async function unavailableBookmarkKeepsWorkingMarketNavigation() {
  const catalog = await publishCatalog([gatewayRow(11, "PAX", false), gatewayRow(1, "ETH", true)]);
  for (const bookmark of ["PAX", "MISSING", "", ["PAX", "ETH"]]) {
    assert.equal(page.tradingMarket(bookmark, catalog.tradeable), undefined);
    const html = renderPage(bookmark);
    assert.match(html, /role="alert"/);
    assert.match(html, /The selected market is unavailable/);
    assert.match(html, /aria-label="Select an available market"/);
    assert.doesNotMatch(html, /href="\/perps\?market=PAX"/);
    const href = html.match(/href="([^" ]+)"[^>]*>ETH\/USD<\/a>/)?.[1];
    assert.ok(href, "available market has a real navigation target");
    const target = new URL(href, origin);
    assert.equal(target.pathname, "/perps");
    const selected = target.searchParams.get("market");
    assert.equal(selected, "ETH");
    assert.equal(page.tradingMarket(selected!, catalog.tradeable)?.marketId, 1);
    assert.doesNotMatch(renderPage(selected!), /The selected market is unavailable/);
  }
}

async function emptyAndDisabledCatalogsHaveHonestUnavailableState() {
  for (const next of [[], [gatewayRow(11, "PAX", false)]]) {
    const catalog = await publishCatalog(next);
    for (const selected of [undefined, "PAX", "BTC"]) {
      assert.equal(page.tradingMarket(selected, catalog.catalog), undefined);
      const html = renderPage(selected);
      assert.match(html, /The selected market is unavailable/);
      assert.match(html, /No markets are currently available to trade/);
      assert.doesNotMatch(html, /href="\/perps\?market=/);
    }
  }
}

async function catalogChangesKeepExplicitIntentAndRefreshRecoveryChoices() {
  await publishCatalog([gatewayRow(11, "PAX", true), gatewayRow(1, "ETH", true)]);
  assert.doesNotMatch(renderPage("PAX"), /The selected market is unavailable/);
  await publishCatalog([gatewayRow(11, "PAX", false), gatewayRow(1, "ETH", true)]);
  assert.match(renderPage("PAX"), /href="\/perps\?market=ETH"/);
  assert.match(renderPage(), /ETH\/USD/);
  await publishCatalog([gatewayRow(11, "PAX", false), gatewayRow(1, "ETH", false), gatewayRow(18, "DOGE", true)]);
  const html = renderPage("PAX");
  assert.match(html, /href="\/perps\?market=DOGE"/);
  assert.doesNotMatch(html, /href="\/perps\?market=ETH"/);
  await publishCatalog([gatewayRow(11, "PAX", true)]);
  assert.doesNotMatch(renderPage("PAX"), /The selected market is unavailable/);
}

test(disabledPaxUsesLiveCatalogDefault);
test(enabledPaxRemainsPreferredWithoutSelection);
test(unavailableBookmarkKeepsWorkingMarketNavigation);
test(emptyAndDisabledCatalogsHaveHonestUnavailableState);
test(catalogChangesKeepExplicitIntentAndRefreshRecoveryChoices);

import assert from "node:assert/strict";
import { existsSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import "fake-indexeddb/auto";
import { Window } from "happy-dom";
import React, { act, useEffect } from "react";
import type { Root } from "react-dom/client";
const require = createRequire(import.meta.url);
let DefaultConfigStore: typeof import("@paxeer-network/core").DefaultConfigStore;
let PaxeerNetworkProvider: typeof import("@paxeer-network/hooks").PaxeerNetworkProvider;
let useMarketStore: typeof import("@paxeer-network/hooks/paxeer-network/useMarket/market.store").useMarketStore;
let useAppStore: typeof import("@paxeer-network/hooks").useAppStore;
let LocaleProvider: typeof import("@paxeer-network/i18n").LocaleProvider;
let MarketsListWidget: typeof import("@paxeer-network/markets").MarketsListWidget;
let MarketsProvider: typeof import("@paxeer-network/markets").MarketsProvider;
let MarketsTabName: typeof import("@paxeer-network/markets").MarketsTabName;
let PaxeerNetworkThemeProvider: typeof import("@paxeer-network/ui").PaxeerNetworkThemeProvider;
let nativeMarketRows: typeof import("@paxeer-network/paxeer-backend/nativeMetadata").nativeMarketRows;
let nativeSymbols: typeof import("@paxeer-network/paxeer-backend/nativeMetadata").nativeSymbols;
let useMarketsSnapshot: typeof import("@paxeer-network/paxeer-backend/hooks/perps/useMarketsStream").useMarketsSnapshot;
let updateNativeSnapshot: typeof import("@paxeer-network/paxeer-backend/runtime").updateNativeSnapshot;

function loadWorkspace() {
  ({ DefaultConfigStore } = require("@paxeer-network/core"));
  ({ PaxeerNetworkProvider, useAppStore } = require("@paxeer-network/hooks"));
  ({
    useMarketStore,
  } = require("@paxeer-network/hooks/paxeer-network/useMarket/market.store"));
  ({ LocaleProvider } = require("@paxeer-network/i18n"));
  ({
    MarketsListWidget,
    MarketsProvider,
    MarketsTabName,
  } = require("@paxeer-network/markets"));
  ({ PaxeerNetworkThemeProvider } = require("@paxeer-network/ui"));
  ({
    nativeMarketRows,
    nativeSymbols,
  } = require("@paxeer-network/paxeer-backend/nativeMetadata"));
  ({
    useMarketsSnapshot,
  } = require("@paxeer-network/paxeer-backend/hooks/perps/useMarketsStream"));
  ({
    updateNativeSnapshot,
  } = require("@paxeer-network/paxeer-backend/runtime"));
}

const gatewayOrigin = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
const phaseDirectory = process.env.PROOF_PHASE_DIR;
const phaseProofEnabled = Boolean(gatewayOrigin);

const pause = (milliseconds: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, milliseconds));

function installDocumentRuntime() {
  const runtime = new Window({ url: gatewayOrigin || "http://127.0.0.1/" });
  const previous = new Map<string, PropertyDescriptor | undefined>();
  for (const key of [
    "React",
    "IS_REACT_ACT_ENVIRONMENT",
    "window",
    "document",
    "navigator",
    "location",
    "localStorage",
    "sessionStorage",
    "HTMLElement",
    "Node",
    "Element",
    "Event",
    "CustomEvent",
    "MouseEvent",
    "EventTarget",
    "StorageEvent",
    "DocumentFragment",
    "SVGElement",
    "MutationObserver",
    "ResizeObserver",
    "getComputedStyle",
  ]) {
    previous.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, {
      configurable: true,
      writable: true,
      value:
        key === "React"
          ? React
          : key === "IS_REACT_ACT_ENVIRONMENT"
            ? true
            : (runtime as unknown as Record<string, unknown>)[key],
    });
  }
  (globalThis as Record<string, unknown>).window = runtime;
  (runtime as unknown as Record<string, unknown>).WebSocket =
    globalThis.WebSocket;
  return async () => {
    await runtime.happyDOM.close();
    for (const [key, value] of previous) {
      if (value === undefined)
        delete (globalThis as Record<string, unknown>)[key];
      else Object.defineProperty(globalThis, key, value);
    }
  };
}

function gatewayStateBridge({ onSnapshot }: { onSnapshot: () => void }) {
  const snapshots = useMarketsSnapshot();

  useEffect(() => {
    if (!snapshots.length) return;

    updateNativeSnapshot({
      markets: snapshots.map((snapshot) => ({
        symbol: snapshot.symbol,
        symbol_full: snapshot.symbol_full,
        price: snapshot.price ?? undefined,
        change_24h: snapshot.change_24h ?? undefined,
        volume_24h: snapshot.volume_24h,
        open_interest: snapshot.open_interest,
        funding_8h: snapshot.funding_8h ?? undefined,
        perp_funding_rate: snapshot.perp_funding_rate ?? undefined,
        last_funding_rate_hourly:
          snapshot.last_funding_rate_hourly ?? undefined,
        price_updated_at: snapshot.price_updated_at,
        last_updated: snapshot.last_updated ?? undefined,
        price_24h: snapshot.price_24h,
        high_24h: snapshot.high_24h,
        low_24h: snapshot.low_24h,
        amount_24h: snapshot.amount_24h,
      })),
    });

    useMarketStore.getState().actions.updateMarket(nativeMarketRows() as any);
    const symbols = Object.fromEntries(
      nativeSymbols().map((symbol) => [
        symbol.symbol,
        {
          ...symbol,
          base: symbol.symbol.split("_")[1],
          quote: "USDC",
          type: "PERP",
          name: symbol.symbol,
          base_dp: 8,
          quote_dp: 6,
        },
      ]),
    );
    useAppStore.getState().actions.setSymbolsInfo(symbols as any);
    onSnapshot();
  }, [snapshots, onSnapshot]);

  return null;
}

function configuredTree(onSnapshot: () => void) {
  const configStore = new DefaultConfigStore({
    brokerId: "paxeer-network",
    networkId: "mainnet",
  });
  configStore.set("apiBaseUrl", gatewayOrigin ?? "");
  const context = {
    configStore,
    keyStore: {} as any,
    walletAdapters: [],
    networkId: "mainnet",
  } as any;

  return React.createElement(
    PaxeerNetworkProvider,
    { value: context },
    React.createElement(
      LocaleProvider,
      { locale: "en" },
      React.createElement(
        PaxeerNetworkThemeProvider,
        null,
        React.createElement(
          MarketsProvider,
          null,
          React.createElement(gatewayStateBridge, { onSnapshot }),
          React.createElement(MarketsListWidget, {
            type: MarketsTabName.All,
            panelSize: "large",
          }),
        ),
      ),
    ),
  );
}

async function renderMarkets() {
  const restoreDocument = installDocumentRuntime();
  try {
    loadWorkspace();
    const {
      MARKETS_WS_URL,
    } = require("@paxeer-network/paxeer-backend/hooks/perps/useMarketsStream");
    assert.equal(
      MARKETS_WS_URL,
      new URL("/ws/markets", gatewayOrigin).href.replace(/^http/, "ws"),
      "mounted markets client uses the configured gateway socket",
    );
  } catch (error) {
    await restoreDocument();
    throw error;
  }
  const {
    createRoot,
  }: typeof import("react-dom/client") = require("react-dom/client");
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  let receivedSnapshot = false;
  await act(async () => {
    root.render(
      configuredTree(() => {
        receivedSnapshot = true;
      }),
    );
  });
  return {
    container,
    root,
    restoreDocument,
    receivedSnapshot: () => receivedSnapshot,
  };
}

async function waitForMarketRows(
  mounted: Awaited<ReturnType<typeof renderMarkets>>,
  count: number,
) {
  for (let attempt = 0; attempt < 400; attempt += 1) {
    await act(async () => {
      await pause(10);
    });
    if (
      mounted.receivedSnapshot() &&
      mounted.container.querySelectorAll("tbody tr").length >= count
    )
      return;
  }
  assert.fail(`mounted markets view did not render ${count} rows`);
}

function assertSymbols(container: HTMLElement, symbols: readonly string[]) {
  const rows = Array.from(container.querySelectorAll("tbody tr"));
  assert.equal(rows.length, symbols.length);
  const renderedSymbols = rows.map((row) => {
    const labels = Array.from(row.querySelectorAll("td:first-child *"))
      .filter((element) => element.children.length === 0)
      .map((element) => element.textContent?.trim())
      .filter(
        (label): label is string =>
          label !== undefined && symbols.includes(label),
      );
    assert.equal(labels.length, 1, row.textContent ?? "");
    return labels[0];
  });
  assert.deepEqual(renderedSymbols.sort(), [...symbols].sort());
}

async function unmountMarkets(mounted: {
  container: HTMLElement;
  root: Root;
  restoreDocument: () => Promise<void>;
}) {
  await act(async () => {
    mounted.root.unmount();
    for (let attempt = 0; attempt < 400; attempt += 1) {
      await pause(10);
      const response = await fetch(new URL("/metrics", gatewayOrigin));
      assert.equal(response.status, 200);
      if (/^perps_gateway_sockets 0$/m.test(await response.text())) return;
    }
    assert.fail("mounted markets stream did not close after unmount");
  });
  await mounted.restoreDocument();
}

async function remountMarkets(
  mounted: Awaited<ReturnType<typeof renderMarkets>>,
) {
  await unmountMarkets(mounted);
  return renderMarkets();
}

async function advancePhase(phase: number) {
  assert.ok(phaseDirectory);
  process.stdout.write(`PROOF_ADVANCE=${phase}\n`);
  for (let attempt = 0; attempt < 500; attempt += 1) {
    if (existsSync(join(phaseDirectory, String(phase)))) return;
    await pause(10);
  }
  assert.fail(`gateway phase ${phase} was not acknowledged`);
}

async function marketsNullableSnapshotsReachMountedView() {
  assert.ok(gatewayOrigin);
  assert.ok(phaseDirectory);
  mkdirSync(phaseDirectory, { recursive: true });
  const mounted = await renderMarkets();
  try {
    await waitForMarketRows(mounted, 18);
    const symbols = nativeSymbols().map(
      (symbol) => symbol.symbol.split("_")[1],
    );
    assertSymbols(mounted.container, symbols);
    assert.ok((mounted.container.textContent || "").includes("--"));
    assert.doesNotMatch(mounted.container.textContent || "", /NaN|undefined/);

    await advancePhase(1);
    Object.assign(mounted, await remountMarkets(mounted));
    await waitForMarketRows(mounted, 18);
    assertSymbols(mounted.container, symbols);
    assert.doesNotMatch(mounted.container.textContent || "", /NaN|undefined/);
  } finally {
    await unmountMarkets(mounted);
    rmSync(join(phaseDirectory, "1"), { force: true });
  }
}

async function marketsRowsCrossExportsIntoTheRealTableWithoutStaleResults() {
  assert.ok(gatewayOrigin);
  assert.ok(phaseDirectory);
  const mounted = await renderMarkets();
  const symbols = nativeSymbols().map((symbol) => symbol.symbol.split("_")[1]);
  try {
    await waitForMarketRows(mounted, 18);
    assertSymbols(mounted.container, symbols);
    for (const phase of [2, 3]) {
      await advancePhase(phase);
      const next = await remountMarkets(mounted);
      await waitForMarketRows(next, 18);
      assertSymbols(next.container, symbols);
      assert.doesNotMatch(next.container.textContent || "", /NaN|undefined/);
      Object.assign(mounted, next);
    }
  } finally {
    await unmountMarkets(mounted);
  }
}

test(marketsNullableSnapshotsReachMountedView, {
  skip: !phaseProofEnabled,
});
test(marketsRowsCrossExportsIntoTheRealTableWithoutStaleResults, {
  skip: !phaseProofEnabled,
});

function streamAddresses(gateway: string | undefined, pageOrigin: string) {
  const env = { ...process.env };
  delete env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  delete env.PRICE_FEED_ORIGIN;
  delete env.NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS;
  const result = spawnSync(
    process.execPath,
    [
      "--import",
      "tsx",
      "-e",
      `
        process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.example.test";
        require("@paxeer-network/paxeer-backend/core/network/gateway");
        delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
        const gateway = ${JSON.stringify(gateway)};
        if (gateway !== undefined) process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = gateway;
        globalThis.window = { location: { origin: ${JSON.stringify(pageOrigin)} } };
        const feeds = require("@paxeer-network/paxeer-backend/core/feeds");
        const markets = require("@paxeer-network/paxeer-backend/hooks/perps/useMarketsStream");
        const explorer = require("@paxeer-network/paxeer-backend/hooks/chain/useExplorerStream");
        const spot = require("@paxeer-network/paxeer-backend/hooks/spot/useSpotMarketStream");
        const perps = require("@paxeer-network/paxeer-backend/hooks/perps/useSidPaxMarketStream");
        process.stdout.write(JSON.stringify({
          root: feeds.getStreamSocketOrigin() + "/ws",
          markets: markets.MARKETS_WS_URL,
          explorer: explorer.EXPLORER_WS_URL,
          spot: [spot.buildSymbolWsUrl("SID"), spot.buildSymbolWsUrl("PAX")],
          perps: [perps.buildSymbolWsUrl("SID"), perps.buildSymbolWsUrl("PAX")],
        }));
      `,
    ],
    {
      cwd: fileURLToPath(new URL("..", import.meta.url)),
      env,
      encoding: "utf8",
      timeout: 10_000,
    },
  );
  assert.equal(result.status, 0, result.stderr || result.error?.message);
  return JSON.parse(result.stdout);
}

function httpsGatewayProvidesEveryStreamOrigin() {
  assert.deepEqual(
    streamAddresses(
      "https://gateway.example.test:8443/nested/api?market=SID#stream",
      "https://app.hyperpax.xyz",
    ),
    {
      root: "wss://gateway.example.test:8443/ws",
      markets: "wss://gateway.example.test:8443/ws/markets",
      explorer: "wss://gateway.example.test:8443/ws/explorer",
      spot: [
        "wss://gateway.example.test:8443/ws/sid",
        "wss://gateway.example.test:8443/ws/pax",
      ],
      perps: [
        "wss://gateway.example.test:8443/ws/sid",
        "wss://gateway.example.test:8443/ws/pax",
      ],
    },
  );
}

function httpGatewayProvidesEveryStreamOrigin() {
  assert.deepEqual(
    streamAddresses(
      "http://gateway.example.test:8080/nested/api",
      "https://preview.example.test",
    ),
    {
      root: "ws://gateway.example.test:8080/ws",
      markets: "ws://gateway.example.test:8080/ws/markets",
      explorer: "ws://gateway.example.test:8080/ws/explorer",
      spot: [
        "ws://gateway.example.test:8080/ws/sid",
        "ws://gateway.example.test:8080/ws/pax",
      ],
      perps: [
        "ws://gateway.example.test:8080/ws/sid",
        "ws://gateway.example.test:8080/ws/pax",
      ],
    },
  );
}

function streamsRetainTheirOriginsWithoutAGateway() {
  for (const gateway of [undefined, ""]) {
    for (const [pageOrigin, feed] of [
      ["https://preview.example.test", "/api/price-feed"],
      ["https://app.hyperpax.xyz", "wss://data.feed.chainflowtrading.com/api"],
    ]) {
      assert.deepEqual(streamAddresses(gateway, pageOrigin), {
        root: `${feed}/ws`,
        markets: `${feed}/ws/markets`,
        explorer: `${feed}/ws/explorer`,
        spot: [`${feed}/ws/sid`, `${feed}/ws/pax`],
        perps: ["/ws/sid", "/ws/pax"],
      });
    }
  }
}

test(httpsGatewayProvidesEveryStreamOrigin);
test(httpGatewayProvidesEveryStreamOrigin);
test(streamsRetainTheirOriginsWithoutAGateway);

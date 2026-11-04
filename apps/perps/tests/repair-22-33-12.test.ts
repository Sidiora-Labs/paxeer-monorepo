import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { once } from "node:events";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { setTimeout as delay } from "node:timers/promises";
import React, { createElement, isValidElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider } from "wagmi";
import { Router as NextPagesRouter } from "next/router";
import NextApp from "next/app";
import { RouterContext } from "next/dist/shared/lib/router-context.shared-runtime";
import * as Dialog from "@radix-ui/react-dialog";
import type { FooterHealth } from "../src/app/shell/Footer";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

// Committed gateway transcripts exercise the production GraphQL decoder and catalog.
const marketRows: GatewayMarketRow[] = [
  { marketId: 0, symbol: "BTC", name: "Bitcoin", oracleSymbol: "BTC", enabled: true, createdAt: null, szDecimals: 3, pxDecimals: 2, tickSize: "1", lotSize: "1", tierClass: 0, maxLeverage: "20", latestPrice: null, fundingRate: null },
  { marketId: 1, symbol: "ETH", name: "Ether", oracleSymbol: "ETH", enabled: false, createdAt: null, szDecimals: 3, pxDecimals: 2, tickSize: "1", lotSize: "1", tierClass: 0, maxLeverage: "20", latestPrice: null, fundingRate: null },
  { marketId: 5, symbol: "TSLA", name: "Tesla", oracleSymbol: "TSLA", enabled: true, createdAt: null, szDecimals: 3, pxDecimals: 2, tickSize: "1", lotSize: "1", tierClass: 0, maxLeverage: "20", latestPrice: null, fundingRate: null },
  { marketId: 7, symbol: "NAS100", name: "Nasdaq 100", oracleSymbol: "NAS100", enabled: true, createdAt: null, szDecimals: 3, pxDecimals: 2, tickSize: "1", lotSize: "1", tierClass: 0, maxLeverage: "20", latestPrice: null, fundingRate: null },
  { marketId: 8, symbol: "XAU", name: "Gold", oracleSymbol: "XAU", enabled: true, createdAt: null, szDecimals: 3, pxDecimals: 2, tickSize: "1", lotSize: "1", tierClass: 0, maxLeverage: "20", latestPrice: null, fundingRate: null },
];
let statusMode: "healthy" | "incident" | "failure" | "invalid" | "hang" = "healthy";
let gatewayReads = 0;
const requests: string[] = [];
const server = createServer(async (request, response) => {
  requests.push(request.url ?? "");
  if (request.url === "/graphql" && request.method === "POST") {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString()) as { query: string };
    if (!body.query.includes("query GetMarkets")) {
      response.writeHead(400).end();
      return;
    }
    gatewayReads += 1;
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ data: { markets: marketRows } }));
    return;
  }
  if (request.url === "/api/v2/status.json") {
    if (statusMode === "hang") return;
    response.writeHead(statusMode === "failure" ? 503 : 200, { "content-type": "application/json" });
    response.end(JSON.stringify(statusMode === "invalid" ? { status: { indicator: "none" } } : {
      status: statusMode === "incident"
        ? { indicator: "major", description: "Partial system outage" }
        : { indicator: "none", description: "All Systems Operational" },
    }));
    return;
  }
  response.writeHead(404).end();
});
let origin: string;
const priorReact = Object.getOwnPropertyDescriptor(globalThis, "React");

before(async () => {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  // Set all configuration before the first runtime import from src.
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  process.env.NEXT_PUBLIC_STATUS_PAGE_URL = "https://status.fixture.test";
  Object.defineProperty(globalThis, "React", { value: React, configurable: true });
});

after(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  if (priorReact) Object.defineProperty(globalThis, "React", priorReact);
  else Reflect.deleteProperty(globalThis, "React");
});

function elements(node: ReactNode): { type: unknown; props: Record<string, unknown> }[] {
  if (Array.isArray(node)) return node.flatMap(elements);
  if (!isValidElement<{ children?: ReactNode }>(node)) return [];
  return [{ type: node.type, props: node.props as Record<string, unknown> }, ...elements(node.props.children)];
}

function requiredElement(node: ReactNode, predicate: (element: ReturnType<typeof elements>[number]) => boolean) {
  const found = elements(node).find(predicate);
  assert.ok(found, "the production tree exposes the expected element");
  return found;
}

async function renderer() {
  const { wagmiConfig } = await import("../src/core/wagmi");
  const { Header } = await import("../src/app/shell/Header");
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const savedSelf = Object.getOwnPropertyDescriptor(globalThis, "self");
  let pagesRouter: NextPagesRouter;
  try {
    // Bootstrap the real pages router from an in-memory Next page transcript.
    // SSR reads route state without a browser page loader or navigation.
    Object.defineProperty(globalThis, "self", { configurable: true, value: {
      __NEXT_DATA__: { props: { pageProps: {} }, page: "/perps", query: {}, buildId: "repair-22-33-12", gssp: true },
      location: new URL(`${origin}/perps`),
    } });
    pagesRouter = new NextPagesRouter("/perps", {}, "/perps", {
      initialProps: { pageProps: {} },
      pageLoader: undefined,
      Component: Header,
      App: NextApp,
      wrapApp: (App) => App,
      subscription: async (route, App) => {
        renderToStaticMarkup(createElement(App, { Component: route.Component, pageProps: route.props?.pageProps, router: pagesRouter }));
      },
      isFallback: false,
    });
  } finally {
    if (savedSelf) Object.defineProperty(globalThis, "self", savedSelf);
    else Reflect.deleteProperty(globalThis, "self");
  }
  assert.equal(pagesRouter.pathname, "/perps");
  function render(node: ReactNode) {
    return renderToStaticMarkup(createElement(WagmiProvider, { config: wagmiConfig, reconnectOnMount: false },
      createElement(QueryClientProvider, { client },
        createElement(RouterContext.Provider, { value: pagesRouter }, node))));
  }
  return { render, client, wagmiConfig };
}

async function waitUntil(predicate: () => boolean) {
  const deadline = Date.now() + 2000;
  while (!predicate()) {
    assert.ok(Date.now() < deadline, "the real transport delivers the expected state within two seconds");
    await delay(5);
  }
}

async function statusAssumesHealth() {
  const { Footer, FooterStatus, watchFooterStatus } = await import("../src/app/shell/Footer");
  const initial = renderToStaticMarkup(createElement(Footer));
  assert.match(initial, /Status unknown/);
  assert.doesNotMatch(initial, /Operational|bg-emerald-500|embed\/script/);
  assert.match(initial, /href="https:\/\/status.fixture.test"/);

  for (const mode of ["healthy", "incident", "failure", "invalid", "hang"] as const) {
    statusMode = mode;
    const received: FooterHealth[] = [];
    const stop = watchFooterStatus(origin, (health) => received.push(health), { timeoutMs: 100, refreshMs: 10000 });
    try {
      assert.equal(received[0].state, "unknown");
      await waitUntil(() => received.length > 1);
      const health = received.at(-1)!;
      const html = renderToStaticMarkup(createElement(FooterStatus, { statusPage: origin, health }));
      assert.ok(requests.includes("/api/v2/status.json"));
      assert.ok(html.includes(`href="${origin}"`), "the status link survives failure");
      if (mode === "healthy") {
        assert.equal(health.state, "confirmed");
        assert.match(html, /All Systems Operational/);
        assert.match(html, /bg-emerald-500/);
      } else if (mode === "incident") {
        assert.equal(health.state, "confirmed");
        assert.match(html, /Partial system outage/);
        assert.doesNotMatch(html, /bg-emerald-500/);
      } else {
        assert.equal(health.state, "unavailable");
        assert.match(html, /Status unavailable/);
        assert.doesNotMatch(html, /Operational|bg-emerald-500/);
      }
    } finally {
      stop();
    }
  }

  statusMode = "healthy";
  const received: FooterHealth[] = [];
  const stop = watchFooterStatus(origin, (health) => {
    received.push(health);
    if (health.state === "confirmed") statusMode = "failure";
  }, { timeoutMs: 100, refreshMs: 10 });
  try {
    await waitUntil(() => received.some((health) => health.state === "stale"));
    const html = renderToStaticMarkup(createElement(FooterStatus, { statusPage: origin, health: received.at(-1)! }));
    assert.match(html, /Status stale/);
    assert.doesNotMatch(html, /Operational|bg-emerald-500/);
  } finally {
    stop();
  }
  const stoppedCount = received.length;
  await delay(30);
  assert.equal(received.length, stoppedCount, "unsubscribed footers receive no further updates");
}

test(statusAssumesHealth);

async function withdrawEntryRetainsRequestedAction() {
  const { Header } = await import("../src/app/shell/Header");
  const { WalletControlPanel } = await import("../src/app/modals/WalletControlPanel");
  const { DepositWithdrawModal } = await import("../src/app/modals/DepositWithdrawModal");
  const { privateKeyToAccount } = await import("viem/accounts");
  const { render, client, wagmiConfig } = await renderer();
  const saved = wagmiConfig.state;
  const connector = wagmiConfig.connectors[0];
  assert.ok(connector);
  const account = privateKeyToAccount(`0x${"11".repeat(32)}`);
  // The wallet provider consumes its real in-memory connection store. No signing
  // or chain mutation is requested by these navigation-only callbacks.
  wagmiConfig.setState((state) => ({ ...state, status: "connected", current: connector.uid,
    connections: new Map([[connector.uid, { accounts: [account.address], chainId: 125, connector }]]) }));
  let pass = 0;
  const actions: unknown[] = [];
  function ModalProbe(props: React.ComponentProps<typeof DepositWithdrawModal>) {
    const tree = DepositWithdrawModal(props);
    const root = requiredElement(tree, (element) => element.type === Dialog.Root);
    assert.equal(root.props.open, props.isOpen);
    const title = requiredElement(tree, (element) => element.type === Dialog.Title);
    // Portals have no server HTML. Inspect the real consumer's title alongside
    // its unmodified tree so the content assertion still executes under SSR.
    return createElement(React.Fragment, null, tree, createElement("output", null, title.props.children as ReactNode));
  }
  function Harness() {
    const tree = Header();
    const panel = requiredElement(tree, (element) => element.type === WalletControlPanel).props;
    const modal = requiredElement(tree, (element) => element.type === DepositWithdrawModal).props;
    actions.push({ isOpen: modal.isOpen, initialAction: modal.initialAction });
    switch (pass++) {
      case 0: (panel.onWithdraw as () => void)(); break;
      case 1: (modal.onClose as () => void)(); break;
      case 2: (panel.onDeposit as () => void)(); break;
      case 3: (modal.onClose as () => void)(); break;
      case 4: (panel.onWithdraw as () => void)(); break;
    }
    // Render the real consumer as well. Its initialAction consumption is an
    // explicitly recorded out-of-scope wiring request, not claimed by this proof.
    return createElement(ModalProbe, modal as unknown as React.ComponentProps<typeof DepositWithdrawModal>);
  }
  try {
    const html = render(createElement(Harness));
    assert.equal(pass, 6);
    assert.deepEqual(actions, [
      { isOpen: false, initialAction: "deposit" },
      { isOpen: true, initialAction: "withdraw" },
      { isOpen: false, initialAction: "withdraw" },
      { isOpen: true, initialAction: "deposit" },
      { isOpen: false, initialAction: "deposit" },
      { isOpen: true, initialAction: "withdraw" },
    ]);
    assert.match(html, /Deposit|Withdraw/);
  } finally {
    wagmiConfig.setState(saved);
    client.clear();
  }
}

test(withdrawEntryRetainsRequestedAction);

async function mobileNavigationUsesDialogFocusLifecycle() {
  const { MobileNavDrawer, MobileNavTrigger } = await import("../src/app/shell/MobileNav");
  let closes = 0;
  let rootProps: Record<string, unknown> = {};
  function Harness() {
    const tree = MobileNavDrawer({ open: true, onClose: () => { closes += 1; } });
    rootProps = requiredElement(tree, (element) => element.type === Dialog.Root).props;
    assert.equal(rootProps.open, true);
    assert.notEqual(rootProps.modal, false, "the Radix modal default traps focus and locks body scroll");
    requiredElement(tree, (element) => element.type === Dialog.Portal);
    requiredElement(tree, (element) => element.type === Dialog.Overlay);
    const content = requiredElement(tree, (element) => element.type === Dialog.Content);
    assert.equal(typeof content.props.onOpenAutoFocus, "function");
    assert.equal(typeof content.props.onCloseAutoFocus, "function");
    requiredElement(tree, (element) => element.type === "nav" && element.props["aria-label"] === "Main navigation");
    requiredElement(tree, (element) => element.type === Dialog.Title);
    requiredElement(tree, (element) => element.type === Dialog.Close && element.props["aria-label"] === "Close navigation menu");
    return tree;
  }
  const { render, client } = await renderer();
  try {
    render(createElement(Harness));
    (rootProps.onOpenChange as (open: boolean) => void)(false);
    assert.equal(closes, 1, "Radix Escape/outside/close dismissal reaches the owner");
    const trigger = renderToStaticMarkup(createElement(MobileNavTrigger, { onClick: () => {} }));
    assert.match(trigger, /aria-label="Open navigation menu"/);
    assert.match(trigger, /aria-haspopup="dialog"/);
  } finally {
    client.clear();
  }
}

test(mobileNavigationUsesDialogFocusLifecycle);

async function mobileDisabledMarketSelection() {
  const { loadPerpsMarkets } = await import("../src/hooks/perps/usePerpsMarkets");
  const { MobileTradeHeader, mobileTradeMarket } = await import("../src/app/shell/MobileTradeHeader");
  const { catalog } = await loadPerpsMarkets();
  assert.ok(gatewayReads > 0);
  assert.equal(catalog.find((market) => market.symbol === "ETH")?.enabled, false);
  const selected: string[] = [];
  let pass = 0;
  function Harness() {
    const tree = MobileTradeHeader({ selectedPair: "BTC/USD", catalog, onOpenMarketPanel: () => {},
      onSelectMarket: (symbol) => {
        selected.push(symbol);
        const market = mobileTradeMarket(`${symbol}/USD`, catalog);
        assert.ok(market?.enabled, "every emitted selection resolves in the receiving header");
      } });
    if (pass++ === 0) {
      (requiredElement(tree, (element) => element.props["aria-label"] === "Select market").props.onClick as () => void)();
    } else if (pass === 2) {
      const rows = elements(tree).filter((element) => element.type === "button" && elements(element.props.children as ReactNode).some((child) => child.type === "img" && child.props.alt === ""));
      assert.equal(rows.length, 4, "disabled Ether never becomes a selectable row");
      assert.equal(rows.some((row) => renderToStaticMarkup(createElement("div", null, row.props.children as ReactNode)).includes("ETH")), false);
      const tesla = rows.find((row) => renderToStaticMarkup(createElement("div", null, row.props.children as ReactNode)).includes("TSLA"));
      assert.ok(tesla);
      if (pass === 2) (tesla.props.onClick as () => void)();
    }
    return tree;
  }
  const { render, client } = await renderer();
  try {
    render(createElement(Harness));
    assert.deepEqual(selected, ["TSLA"]);
    assert.equal(mobileTradeMarket("ETH/USD", catalog), undefined);
  } finally {
    client.clear();
  }
}

test(mobileDisabledMarketSelection);

async function mobileRwaCategoryFromGateway() {
  const { loadPerpsMarkets } = await import("../src/hooks/perps/usePerpsMarkets");
  const { MobileTradeHeader } = await import("../src/app/shell/MobileTradeHeader");
  const { catalog } = await loadPerpsMarkets();
  assert.deepEqual(catalog.map((market) => market.category), ["crypto", "crypto", "stocks", "indices", "commodities"]);
  let pass = 0;
  function Harness() {
    const tree = MobileTradeHeader({ selectedPair: "BTC/USD", catalog, onOpenMarketPanel: () => {} });
    if (pass++ === 0) {
      (requiredElement(tree, (element) => element.props["aria-label"] === "Select market").props.onClick as () => void)();
    } else if (pass === 2) {
      (requiredElement(tree, (element) => element.type === "button" && element.props.children === "rwa").props.onClick as () => void)();
    }
    return tree;
  }
  const { render, client } = await renderer();
  try {
    const html = render(createElement(Harness));
    assert.equal(pass, 3);
    for (const symbol of ["TSLA", "NAS100", "XAU"]) assert.ok(html.includes(`>${symbol}</span>`), symbol);
    assert.doesNotMatch(html, />Bitcoin<|>Ether<|No markets/);
  } finally {
    client.clear();
  }
}

test(mobileRwaCategoryFromGateway);

async function splashHydrationAndSessionRecovery() {
  const { SplashScreen, startSplashSession } = await import("../src/app/shell/SplashScreen");
  const serverHtml = renderToStaticMarkup(createElement(SplashScreen));
  assert.equal(serverHtml, "");
  const savedWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  let reads = 0;
  try {
    Object.defineProperty(globalThis, "window", { configurable: true, value: {
      get sessionStorage() { reads += 1; throw new Error("Storage denied"); },
    } });
    assert.equal(renderToStaticMarkup(createElement(SplashScreen)), serverHtml, "first client render matches server even when storage is denied");
    assert.equal(reads, 0, "rendering never reads browser storage");
  } finally {
    if (savedWindow) Object.defineProperty(globalThis, "window", savedWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
  const session = new Map<string, string>();
  const storage: Pick<Storage, "getItem" | "setItem"> = {
    getItem: (key) => session.get(key) ?? null,
    setItem: (key, value) => { session.set(key, value); },
  };
  const first: boolean[] = [];
  const stop = startSplashSession(storage, (shown) => first.push(shown), () => {});
  assert.deepEqual(first, [true]);
  stop();
  const reopened: boolean[] = [];
  const stopReopened = startSplashSession(storage, (shown) => reopened.push(shown), () => {});
  assert.deepEqual(reopened, [], "a reopened component recovers the session marker and never replays");
  stopReopened();
  const visible: boolean[] = [];
  const rendered: boolean[] = [];
  session.clear();
  const stopTimers = startSplashSession(storage, (shown) => rendered.push(shown), (shown) => visible.push(shown));
  try {
    await delay(3450);
    assert.deepEqual(first, [true], "cleanup cancels timers on the stopped session");
    assert.deepEqual(visible, [true, false], "the splash fades after its visible duration");
    assert.deepEqual(rendered, [true, false], "the overlay unmounts after fading");
  } finally {
    stopTimers();
  }
}

test(splashHydrationAndSessionRecovery);

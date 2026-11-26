import assert from "node:assert/strict";
import { before, after, afterEach, test } from "node:test";
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import { Window } from "happy-dom";
import React from "react";
import "fake-indexeddb/auto";
import { parseUnits } from "viem";
import type { GatewayCatalogEntry } from "../packages/paxeer-backend/src/hooks/perps/usePerpsMarkets";

const require = createRequire(import.meta.url);
const globals = new Map<string, PropertyDescriptor | undefined>();
const browser = new Window({ url: "https://app.hyperpax.xyz" });
const originalGateway = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://app.hyperpax.xyz";

let testing: typeof import("@testing-library/react");
let hooks: typeof import("@paxeer-network/hooks");
let core: typeof import("@paxeer-network/core");
let ui: typeof import("@paxeer-network/ui");
let metadata: typeof import("../packages/paxeer-backend/src/nativeMetadata");
let runtime: typeof import("../packages/paxeer-backend/src/runtime");
let normalizeMarket: typeof import("../packages/paxeer-backend/src/hooks/perps/useMarketsStream").normalizeMarket;
let marketStore: typeof import("../packages/hooks/src/paxeer-network/useMarket/market.store").useMarketStore;
let LocaleProvider: typeof import("@paxeer-network/i18n").LocaleProvider;
let context: import("../packages/hooks/src/paxeerNetworkContext").PaxeerNetworkConfigContextState;

before(async () => {
  for (const [name, value] of Object.entries({
    window: browser,
    document: browser.document,
    navigator: browser.navigator,
    location: browser.location,
    SVGElement: browser.SVGElement,
    StorageEvent: browser.StorageEvent,
    EventTarget: browser.EventTarget,
    localStorage: browser.localStorage,
    sessionStorage: browser.sessionStorage,
    HTMLElement: browser.HTMLElement,
    HTMLInputElement: browser.HTMLInputElement,
    HTMLFormElement: browser.HTMLFormElement,
    Element: browser.Element,
    Node: browser.Node,
    Event: browser.Event,
    MouseEvent: browser.MouseEvent,
    CustomEvent: browser.CustomEvent,
    DocumentFragment: browser.DocumentFragment,
    MutationObserver: browser.MutationObserver,
    ResizeObserver: browser.ResizeObserver,
    IntersectionObserver: browser.IntersectionObserver,
    getComputedStyle: browser.getComputedStyle.bind(browser),
    requestAnimationFrame: browser.requestAnimationFrame.bind(browser),
    cancelAnimationFrame: browser.cancelAnimationFrame.bind(browser),
    React,
    IS_REACT_ACT_ENVIRONMENT: true,
  })) {
    globals.set(name, Object.getOwnPropertyDescriptor(globalThis, name));
    Object.defineProperty(globalThis, name, {
      value,
      configurable: true,
      writable: true,
    });
  }
  testing = await import("@testing-library/react");
  core = require("@paxeer-network/core");
  hooks = require("@paxeer-network/hooks");
  ui = require("@paxeer-network/ui");
  ({ LocaleProvider } = require("@paxeer-network/i18n"));
  metadata = require("../packages/paxeer-backend/src/nativeMetadata");
  runtime = require("../packages/paxeer-backend/src/runtime");
  ({
    normalizeMarket,
  } = require("../packages/paxeer-backend/src/hooks/perps/useMarketsStream"));
  ({
    useMarketStore: marketStore,
  } = require("../packages/hooks/src/paxeer-network/useMarket/market.store"));
  const configStore = new core.DefaultConfigStore({ networkId: "mainnet" });
  const keyStore = new core.LocalStorageStore("mainnet");
  const {
    DefaultEVMWalletAdapter,
  } = require("../packages/default-evm-adapter/src");
  const { EthersProvider } = require("../packages/web3-provider-ethers/src");
  const walletAdapters = [new DefaultEVMWalletAdapter(new EthersProvider())];
  context = { configStore, keyStore, walletAdapters, networkId: "mainnet" };
  const account = new core.Account(configStore, keyStore, walletAdapters);
  core.SimpleDI.registerByName("account", account);
});

afterEach(() => testing.cleanup());
after(async () => {
  await browser.happyDOM.close();
  for (const [name, descriptor] of globals) {
    if (descriptor) Object.defineProperty(globalThis, name, descriptor);
    else Reflect.deleteProperty(globalThis, name);
  }
  if (originalGateway === undefined)
    delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  else process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = originalGateway;
});

function Workspace({ children }: { children: React.ReactNode }) {
  return React.createElement(
    hooks.PaxeerNetworkProvider,
    { value: context },
    React.createElement(
      LocaleProvider,
      { locale: "en" },
      React.createElement(
        ui.PaxeerNetworkThemeProvider,
        null,
        React.createElement(ui.TooltipProvider, null, children),
      ),
    ),
  );
}

function catalogEntry(
  symbol: string,
  marketId: number,
  tick = "0.01",
): GatewayCatalogEntry {
  return {
    marketId,
    symbol,
    name: symbol,
    baseToken: symbol,
    oracleSymbol: symbol,
    szDecimals: 8,
    pxDecimals: tick.split(".")[1]?.length ?? 0,
    tickSize: parseUnits(tick, 18).toString(),
    lotSize: "1",
    tierClass: 0,
    maxLeverage: 20,
    enabled: true,
    listedAt: null,
    category: "crypto",
  };
}

function streamRow(symbol: string, change: number | null, price = 100) {
  const row = normalizeMarket({
    market_id: symbol,
    symbol,
    symbol_full: symbol,
    price,
    price_24h: change === null ? null : price / (1 + change),
    volume_24h: 1000000,
    open_interest: 1000000,
    last_updated: Date.now(),
    perp_index_price: price,
    perp_mark_price: price,
  });
  assert.ok(row, symbol);
  return row;
}

function publish(
  markets: ReturnType<typeof streamRow>[],
  catalog: GatewayCatalogEntry[],
) {
  runtime.updateNativeSnapshot({ markets, marketCatalog: catalog });
  const symbols = Object.fromEntries(
    metadata.nativeSymbols().map((symbol) => [
      symbol.symbol,
      {
        ...symbol,
        base: symbol.display_symbol_name,
        quote: "USDC",
        type: "PERP",
        name: symbol.symbol,
        base_dp: 8,
      },
    ]),
  );
  hooks.useAppStore.getState().actions.setSymbolsInfo(symbols);
  // NativeRuntime publishes these same native rows to the workspace market store.
  marketStore.getState().actions.updateMarket(metadata.nativeMarketRows());
}

async function topListsHoldOnlyMarketsOfTheirDirection() {
  const {
    useDataSource,
  } = require("../packages/markets/src/pages/home/marketsHeader/marketsHeader.script");
  const {
    MarketsHeader,
  } = require("../packages/markets/src/pages/home/marketsHeader/marketsHeader.ui");
  const {
    MobileMarketsHeader,
  } = require("../packages/markets/src/pages/home/marketsHeader/marketsHeader.mobile.ui");
  const symbols = Array.from({ length: 19 }, (_, i) => `M${i}`);
  publish(
    symbols.map((symbol, i) => streamRow(symbol, (i + 1) / 100)),
    symbols.map((symbol, i) => catalogEntry(symbol, i)),
  );
  let lists: ReturnType<typeof useDataSource>;
  function Header({ mobile }: { mobile: boolean }) {
    lists = useDataSource();
    return React.createElement(
      mobile ? MobileMarketsHeader : MarketsHeader,
      lists,
    );
  }
  for (const mobile of [false, true]) {
    const view = testing.render(React.createElement(Header, { mobile }), {
      wrapper: Workspace,
    });
    await testing.waitFor(() => assert.equal(lists.gainers.length, 5));
    assert.deepEqual(
      lists.gainers.map((row: { symbol: string }) => row.symbol),
      [18, 17, 16, 15, 14].map((i) => `PERP_M${i}_USDC`),
    );
    assert.deepEqual(lists.losers, []);
    if (mobile) {
      testing.fireEvent.mouseDown(
        view.getByRole("tab", { name: "Top losers" }),
        { button: 0, ctrlKey: false },
      );
      await testing.waitFor(() =>
        assert.match(
          view.getByRole("tabpanel").textContent!,
          /No results found\./,
        ),
      );
      testing.fireEvent.mouseDown(
        view.getByRole("tab", { name: "Top gainers" }),
        { button: 0, ctrlKey: false },
      );
      await testing.waitFor(() =>
        assert.match(view.getByRole("tabpanel").textContent!, /M18/),
      );
      const panel = view.getByRole("tabpanel");
      assert.equal(
        panel.querySelectorAll('[class*="oui-grid-cols-"]').length,
        5,
      );
    } else {
      const card = view
        .getByText("Top losers")
        .closest('[class*="oui-rounded"]')!;
      assert.match(card.textContent!, /No results found\./);
      const gainers = view
        .getByText("Top gainers")
        .closest('[class*="oui-rounded"]')!;
      assert.equal(
        gainers.querySelectorAll('[class*="oui-grid-cols-"]').length,
        5,
      );
    }
    view.unmount();
  }
  const changes = [
    0.02,
    -0.07,
    0,
    -0.01,
    0.12,
    null,
    -0.15,
    0.05,
    -0.03,
    0.08,
    -0.09,
    0.01,
    -0.11,
    0.03,
  ];
  publish(
    changes.map((change, i) => streamRow(`M${i}`, change)),
    changes.map((_, i) => catalogEntry(`M${i}`, i)),
  );
  for (const mobile of [false, true]) {
    const view = testing.render(React.createElement(Header, { mobile }), {
      wrapper: Workspace,
    });
    await testing.waitFor(() => assert.equal(lists.losers.length, 5));
    assert.deepEqual(
      lists.gainers.map((row: { symbol: string }) => row.symbol),
      [4, 9, 7, 13, 0].map((i) => `PERP_M${i}_USDC`),
    );
    assert.deepEqual(
      lists.losers.map((row: { symbol: string }) => row.symbol),
      [6, 12, 10, 1, 8].map((i) => `PERP_M${i}_USDC`),
    );
    assert.ok(lists.gainers.every((row: { change: number }) => row.change > 0));
    assert.ok(lists.losers.every((row: { change: number }) => row.change < 0));
    if (mobile) {
      for (const [name, indices] of [
        ["Top gainers", [4, 9, 7, 13, 0]],
        ["Top losers", [6, 12, 10, 1, 8]],
      ] as const) {
        testing.fireEvent.mouseDown(view.getByRole("tab", { name }), {
          button: 0,
          ctrlKey: false,
        });
        await testing.waitFor(() =>
          assert.match(
            view.getByRole("tabpanel").textContent!,
            new RegExp(`M${indices[0]}`),
          ),
        );
        const rows = Array.from(
          view
            .getByRole("tabpanel")
            .querySelectorAll<HTMLElement>('[class*="oui-grid-cols-"]'),
        );
        assert.equal(rows.length, indices.length);
        const positions = indices.map((i) =>
          rows.findIndex(
            (row) =>
              testing.within(row).queryByText(`M${i}`, { exact: true }) !==
              null,
          ),
        );
        assert.ok(
          positions.every(
            (position, i) =>
              position >= 0 && (i === 0 || position > positions[i - 1]),
          ),
        );
      }
    }
    view.unmount();
  }
}

async function pricesPrintAtTheTickOfTheirMarket() {
  const { parseNumber } = require("../packages/ui/src/typography/utils");
  const {
    SymbolInfoBarFull,
  } = require("../packages/markets/src/components/symbolInfoBarFull/symbolInfoBarFull.ui");
  const {
    useSymbolInfoBarFullScript,
  } = require("../packages/markets/src/components/symbolInfoBarFull/symbolInfoBarFull.script");
  const {
    MarketsListWidget,
  } = require("../packages/markets/src/components/marketsList/marketsList.widget");
  const {
    MarketsProvider,
  } = require("../packages/markets/src/components/marketsProvider");
  const {
    useMarketsListFullColumns,
  } = require("../packages/markets/src/components/marketsListFull/column");
  const {
    getMarkColumn,
    getIndexColumn,
  } = require("../packages/markets/src/components/shared/column");
  const {
    useDataSource,
  } = require("../packages/markets/src/pages/home/marketsHeader/marketsHeader.script");
  const {
    MarketsHeader,
  } = require("../packages/markets/src/pages/home/marketsHeader/marketsHeader.ui");
  const { MarketsTabName } = require("../packages/markets/src/type");
  const cases = [
    { symbol: "ETH", price: 1704.109999, tick: "0.01", text: "1,704.11" },
    { symbol: "SOL", price: 61.310999, tick: "0.01", text: "61.31" },
    { symbol: "LINK", price: 6.617999, tick: "0.001", text: "6.618" },
  ];
  publish(
    cases.map(({ symbol, price }) => streamRow(symbol, 0.1, price)),
    cases.map(({ symbol, tick }, i) => catalogEntry(symbol, i, tick)),
  );
  function MarketRows() {
    const data = useDataSource();
    const columns = useMarketsListFullColumns(data.favorite);
    return React.createElement(
      React.Fragment,
      null,
      React.createElement(ui.DataTable, {
        columns: [...columns, getMarkColumn(), getIndexColumn()],
        dataSource: data.markets,
      }),
      React.createElement(MarketsHeader, data),
    );
  }
  function TradeHeader({ symbol }: { symbol: string }) {
    return React.createElement(
      SymbolInfoBarFull,
      useSymbolInfoBarFullScript({ symbol }),
    );
  }
  for (const example of cases) {
    assert.equal(
      parseNumber(example.price, { rule: "price", tick: Number(example.tick) }),
      example.text,
    );
    const symbol = `PERP_${example.symbol}_USDC`;
    const view = testing.render(
      React.createElement(
        MarketsProvider,
        { symbol },
        React.createElement(
          "section",
          { "aria-label": "Trade header" },
          React.createElement(TradeHeader, { symbol }),
        ),
        React.createElement(
          "section",
          { "aria-label": "Trade markets" },
          React.createElement(MarketsListWidget, {
            type: MarketsTabName.All,
            panelSize: "large",
          }),
        ),
        React.createElement(
          "section",
          { "aria-label": "Markets" },
          React.createElement(MarketRows),
        ),
      ),
      { wrapper: Workspace },
    );
    const header = testing.within(
      view.getByRole("region", { name: "Trade header" }),
    );
    await testing.waitFor(() =>
      assert.equal(header.queryAllByText(example.text).length, 3),
    );
    assert.ok(
      header.getByText(
        (_, element) =>
          element?.classList.contains("oui-data-value") === true &&
          element.textContent === `$${example.text}`,
      ),
    );
    const sideRow = Array.from(
      view
        .getByRole("region", { name: "Trade markets" })
        .querySelectorAll("tbody tr"),
    ).find((row) => row.textContent?.includes(example.symbol))!;
    assert.ok(sideRow, example.symbol);
    assert.ok(sideRow.lastElementChild!.textContent!.includes(example.text));
    assert.equal((sideRow.lastElementChild as HTMLElement).style.width, "80px");
    const fullRow = Array.from(
      view
        .getByRole("region", { name: "Markets" })
        .querySelectorAll("tbody tr"),
    ).find((row) => row.textContent?.includes(example.symbol))!;
    assert.ok(fullRow);
    assert.equal(
      Array.from(fullRow.querySelectorAll("td")).filter((cell) =>
        cell.textContent?.includes(example.text),
      ).length,
      3,
    );
    view.unmount();
  }
  assert.equal(parseNumber("1.234999", { rule: "price", tick: 0.05 }), "1.25");
  assert.equal(parseNumber("12.9999", { rule: "price", tick: 1 }), "13");
}

async function headerRendersNoControlWithoutContent() {
  const {
    ChainMenu,
  } = require("../packages/ui-scaffold/src/components/chainMenu/chainMenu.ui");
  const {
    ChainMenuUiMobile,
  } = require("../packages/ui-scaffold/src/components/chainMenu/chainMenu.ui.mobile");
  const {
    useChainMenuScript,
  } = require("../packages/ui-scaffold/src/components/chainMenu/chainMenu.script");
  function Selector({ mobile }: { mobile: boolean }) {
    const state = useChainMenuScript();
    return React.createElement(mobile ? ChainMenuUiMobile : ChainMenu, state);
  }
  for (const mobile of [false, true]) {
    const view = testing.render(React.createElement(Selector, { mobile }), {
      wrapper: Workspace,
    });
    assert.equal(view.container.innerHTML, "");
    assert.equal(view.queryByRole("button"), null);
    view.unmount();
  }
  const icon = testing.renderHook(() => ui.ChainIcon({ chainId: 125 }));
  assert.equal(icon.result.current.props.src, "/paxeer-network-symbol.svg");
  icon.unmount();
  const { renderToStaticMarkup } = require("react-dom/server");
  const markup = renderToStaticMarkup(
    React.createElement(ui.ChainIcon, { chainId: 125 }),
  );
  assert.ok(!markup.includes("/network_logo/125.png"));
  const iconSource = await readFile(
    new URL("../apps/web/public/paxeer-network-symbol.svg", import.meta.url),
    "utf8",
  );
  assert.match(iconSource, /<svg/);
}

async function signedOutPortfolioShowsNoEmptyChartFrames() {
  const {
    OverviewProvider,
  } = require("../packages/portfolio/src/pages/overview/provider/overviewProvider");
  const {
    AssetsChartWidget,
  } = require("../packages/portfolio/src/pages/overview/assetChart/assetsChart.widget");
  const {
    PerformanceWidget,
  } = require("../packages/portfolio/src/pages/overview/performance/performance.widget");
  const {
    PortfolioChartsMobileWidget,
  } = require("../packages/portfolio/src/pages/overview/portfolioChartsMobile/portfolioChartsMobile.widget");
  const {
    AgentsAccess,
  } = require("../packages/portfolio/src/pages/agents/agents.ui");
  const { AccountStatusEnum } = require("../packages/types/src");
  assert.equal(
    core.SimpleDI.get<import("@paxeer-network/core").Account>("account")!
      .stateValue.status,
    AccountStatusEnum.NotConnected,
  );
  const view = testing.render(
    React.createElement(
      OverviewProvider,
      null,
      React.createElement(AssetsChartWidget),
      React.createElement(PerformanceWidget),
      React.createElement(PortfolioChartsMobileWidget),
      React.createElement(AgentsAccess, { state: "signed_out" }),
    ),
    { wrapper: Workspace },
  );
  await testing.waitFor(() =>
    assert.equal(view.getAllByText(/No results found\./).length, 6),
  );
  assert.ok(view.container.querySelector("#portfolio-overview-assets-chart"));
  assert.ok(view.container.querySelector("#portfolio-overview-performance"));
  assert.equal(
    view.container.querySelectorAll(
      ".recharts-responsive-container, .recharts-cartesian-axis, .chart-invisible",
    ).length,
    0,
  );
  assert.match(
    view.container.textContent!,
    /Agents belong to your embedded wallet, and a wallet connected in the header does not replace this sign-in\./,
  );
}

test(topListsHoldOnlyMarketsOfTheirDirection);
test(pricesPrintAtTheTickOfTheirMarket);
test(headerRendersNoControlWithoutContent);
test(signedOutPortfolioShowsNoEmptyChartFrames);

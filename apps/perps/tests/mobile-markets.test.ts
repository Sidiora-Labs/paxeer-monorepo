import assert from "node:assert/strict";
import { test } from "node:test";
import * as React from "react";
import { createElement, isValidElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider } from "wagmi";
import type { MarketCatalogEntry } from "../src/core/perps/marketCatalog";

function elements(
  node: ReactNode,
): { type: unknown; props: Record<string, unknown> }[] {
  if (Array.isArray(node)) return node.flatMap(elements);
  if (!isValidElement<{ children?: ReactNode }>(node)) return [];
  return [
    { type: node.type, props: node.props as Record<string, unknown> },
    ...elements(node.props.children),
  ];
}

async function mobileHeaderListsCatalogMarket() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  const { MobileTradeHeader } = await import("../src/app/shell/MobileTradeHeader");
  const { PERPS_MARKETS } = await import("../src/core/perps/config");
  const { wagmiConfig } = await import("../src/core/wagmi");
  assert.equal(PERPS_MARKETS.some((market) => market.symbol === "DOGE"), false);

  const catalog: MarketCatalogEntry[] = [
    {
      marketId: 18,
      symbol: "DOGE",
      name: "Dogecoin",
      szDecimals: 0,
      pxDecimals: 5,
      maxLeverage: 20,
      enabled: true,
      listedAt: null,
      category: "crypto",
    },
  ];
  const client = new QueryClient();
  let renders = 0;
  let opens = 0;

  function Harness() {
    const tree = MobileTradeHeader({
      selectedPair: "DOGE/USD",
      catalog,
      onOpenMarketPanel: () => {
        opens += 1;
      },
    });
    renders += 1;
    if (renders === 1) {
      const selector = elements(tree).find(
        (element) =>
          element.type === "button" &&
          element.props["aria-label"] === "Select market",
      );
      assert.ok(selector, "the catalog market renders the selector");
      (selector.props.onClick as () => void)();
    }
    return tree;
  }

  const reactGlobal = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", { value: React, configurable: true });
  try {
    const html = renderToStaticMarkup(
      createElement(
        WagmiProvider,
        { config: wagmiConfig, reconnectOnMount: false },
        createElement(QueryClientProvider, { client }, createElement(Harness)),
      ),
    );
    assert.equal(renders, 2, "opening the selector rerenders the header");
    assert.equal(opens, 1);
    assert.match(html, />Markets<\/span>/);
    assert.match(html, />DOGE<\/span>/);
    assert.match(html, />Dogecoin<\/span>/, "the market list contains the catalog entry");
    assert.match(html, />20x<\/span>/);
    assert.doesNotMatch(html, /No markets|Market DOGE unavailable|Loading markets/);

    const empty = renderToStaticMarkup(
      createElement(
        WagmiProvider,
        { config: wagmiConfig, reconnectOnMount: false },
        createElement(
          QueryClientProvider,
          { client },
          createElement(MobileTradeHeader, {
            selectedPair: "DOGE/USD",
            catalog: [],
            onOpenMarketPanel: () => {
              opens += 1;
            },
          }),
        ),
      ),
    );
    assert.match(empty, /Market DOGE unavailable/);
    assert.doesNotMatch(empty, />BTC<\/span>/, "an empty catalog never selects a static market");
  } finally {
    client.clear();
    if (reactGlobal) Object.defineProperty(globalThis, "React", reactGlobal);
    else Reflect.deleteProperty(globalThis, "React");
  }
}

test(mobileHeaderListsCatalogMarket);

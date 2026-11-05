import { strict as assert } from "node:assert";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { spotTokenState, type VaultTokenInfo } from "../src/core/spot/tokenState";

async function watchlistMarksPausedToken() {
  const gatewayUrl = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://perps-gateway.invalid";
  const client = new QueryClient();
  try {
    const { SPOT_MARKETS } = await import("../src/core/spot/config");
    const { SpotMarketWatchlist, watchlistRowState } = await import(
      "../src/widgets/spot/SpotMarketWatchlist"
    );
    for (const registered of [true, false]) {
      const info: VaultTokenInfo = [
        registered, false, 6, 10_000_000n, 12_000_000n, 2_000_000n,
      ];
      const live = spotTokenState(false, info);
      const paused = spotTokenState(true, info);
      assert.equal(watchlistRowState(paused), "paused");
      assert.equal(watchlistRowState(live), "live");

      for (const pausedMarket of [...SPOT_MARKETS, null]) {
        for (const market of SPOT_MARKETS) {
          client.setQueryData(
            ["spot-token-state", market.baseAddress],
            market === pausedMarket ? paused : live,
          );
        }
        const html = renderToStaticMarkup(
          createElement(
            QueryClientProvider,
            { client },
            createElement(SpotMarketWatchlist, {
              selectedSymbol: SPOT_MARKETS[0].symbol,
              onSelect: (symbol: string) => {
                throw new Error(`Unexpected selection: ${symbol}`);
              },
            }),
          ),
        );
        assert.equal(
          (html.match(/>Paused<\/span>/g) ?? []).length,
          pausedMarket ? 1 : 0,
        );
        for (const market of SPOT_MARKETS) {
          const row = html.match(
            new RegExp(`alt="${market.symbol}"[\\s\\S]*?</button>`),
          );
          assert.ok(row, `watchlist renders ${market.symbol}`);
          assert.equal(
            row[0].includes(">Paused</span>"),
            market === pausedMarket,
          );
          assert.ok(row[0].includes(market.pair));
        }
      }
    }
  } finally {
    client.clear();
    if (gatewayUrl === undefined) {
      delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
    } else {
      process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = gatewayUrl;
    }
  }
}

test(watchlistMarksPausedToken);

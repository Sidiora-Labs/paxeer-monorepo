import { test } from "node:test";
import { strict as assert } from "node:assert";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { TradeSettlement } from "../src/types/perps";

async function settlementRowNamesChainMarket() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  const { catalogFromGateway } = await import(
    "../src/core/perps/marketCatalog"
  );
  const { PERPS_MARKETS } = await import("../src/core/perps/config");
  const { TradeSettlementRows } = await import(
    "../src/widgets/portfolio/TradeSettlementHistory"
  );
  const { catalog } = catalogFromGateway({
    markets: [
      {
        marketId: 18,
        symbol: "DOGE",
        name: "Dogecoin",
        oracleSymbol: "DOGEUSD",
        szDecimals: 0,
        pxDecimals: 5,
        tickSize: "0.00001",
        lotSize: "1",
        tierClass: 1,
        maxLeverage: "20",
        enabled: false,
        createdAt: null,
        latestPrice: null,
        fundingRate: null,
      },
    ],
  });
  const settlement: TradeSettlement = {
    id: 1,
    positionId: "18",
    userAddress: "0x0000000000000000000000000000000000000001",
    marketId: 18,
    tradeType: 1,
    tradeTypeName: null,
    sizeUsd: "100000000000000000000",
    executionPrice: "100000000000000000",
    grossPnl: "10000000000000000000",
    totalFeesUsd: "1000000000000000000",
    borrowingFeeUsd: "0",
    fundingPaidUsd: "0",
    netPayoutTokens: "9000000000000000000",
    blockNumber: 1,
    txHash: "0x" + "01".repeat(32),
    blockTimestamp: "2026-01-01T00:00:00Z",
  };
  assert.equal(
    PERPS_MARKETS.find((market) => market.marketId === 18),
    undefined,
  );
  const labels = catalog.map((market) => ({
    ...market,
    marketId: BigInt(market.marketId),
  }));
  const renderRows = (rowLabels: typeof labels) =>
    renderToStaticMarkup(
      createElement(
        "table",
        null,
        createElement(
          "tbody",
          null,
          createElement(TradeSettlementRows, {
            settlements: [settlement],
            labels: rowLabels,
          }),
        ),
      ),
    );
  const html = renderRows(labels);
  assert.match(html, /<td[^>]*>DOGE\/USD<\/td>/);
  assert.doesNotMatch(html, /MKT-18|Market 18/);
  assert.match(
    renderRows([{ ...labels[0], symbol: "RENAMED" }]),
    /<td[^>]*>RENAMED\/USD<\/td>/,
  );
  assert.match(renderRows([]), /<td[^>]*>18\/USD<\/td>/);
}

test(settlementRowNamesChainMarket);

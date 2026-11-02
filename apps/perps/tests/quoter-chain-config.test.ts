import { strict as assert } from "node:assert";
import test from "node:test";
import { decodeFunctionResult, encodeFunctionResult } from "viem";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { decodeMarketConfig, type MarketConfig } from "../src/core/perps/marketConfig";
import { tickerQuoteWithConfig } from "../src/core/perps/quoteFallback";

const E18 = 10n ** 18n;
const ticker = { index_price: 25, mark_price: 26, last_funding_rate_hourly: "-0.0001" };

function chainConfig(enabled: boolean): MarketConfig {
  const result = {
    marketId: 901n,
    symbol: "CHAIN",
    name: "Chain market",
    oracleSymbol: "CHAIN/USD",
    szDecimals: 3,
    pxDecimals: 4,
    tickSize: 10n ** 14n,
    lotSize: 10n ** 15n,
    maxLeverage: 20n,
    tierClass: 2,
    impactNotionalUsd: 10_000n * E18,
    oiCapUsd: 1_000_000n * E18,
    fundingCapPpmPerHour: 1000n,
    enabled,
    listedAt: 1_750_000_000n,
  } satisfies MarketConfig;
  return decodeMarketConfig(
    decodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarketConfig",
      data: encodeFunctionResult({ abi: PerpsPrecompileABI, functionName: "getMarketConfig", result }),
    }),
  );
}

function chainTiers() {
  return decodeFunctionResult({
    abi: PerpsPrecompileABI,
    functionName: "getMarginTiers",
    data: encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarginTiers",
      result: [
        { lowerBoundUsd: 0n, maxLeverage: 15n, maintenanceBps: 250n, deductionUsd: 0n },
        { lowerBoundUsd: 100_000n * E18, maxLeverage: 10n, maintenanceBps: 500n, deductionUsd: 0n },
      ],
    }),
  });
}

function chainMarkDetails(fresh: boolean) {
  const [markPrice, indexPrice, midPrice, decodedFresh] = decodeFunctionResult({
    abi: PerpsPrecompileABI,
    functionName: "getMarkPriceDetails",
    data: encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarkPriceDetails",
      result: [27n * E18, 24n * E18, 26n * E18, fresh],
    }),
  });
  return { markPrice, indexPrice, midPrice, fresh: decodedFresh };
}

function tickerQuoteUsesChainConfig() {
  const config = chainConfig(true);
  const tiers = chainTiers();
  const details = chainMarkDetails(true);
  assert.deepEqual(tickerQuoteWithConfig(ticker, config, tiers, details, 1200, 800), {
    indexPrice: 25n * E18,
    markPrice: 26n * E18,
    oracleTWAP: 25n * E18,
    fundingRatePerSecond: 0n,
    fundingRate24h: -2_400_000_000_000_000n,
    longOI: 1200n * E18,
    shortOI: 800n * E18,
    maxLeverage: 20n,
    maintenanceMarginBps: 250n,
    enabled: true,
    priceStale: false,
  });
  assert.throws(
    () => tickerQuoteWithConfig(ticker, config, [], details, 1200, 800),
    /Missing margin tiers for market 901/,
  );
}

function tickerQuoteReportsStale() {
  for (const enabled of [true, false]) {
    for (const fresh of [true, false]) {
      const quote = tickerQuoteWithConfig(ticker, chainConfig(enabled), chainTiers(), chainMarkDetails(fresh), 0, 0);
      assert.equal(quote.enabled, enabled, `enabled=${enabled}, fresh=${fresh}`);
      assert.equal(quote.priceStale, !fresh, `enabled=${enabled}, fresh=${fresh}`);
      assert.equal(quote.maxLeverage, 20n);
      assert.equal(quote.maintenanceMarginBps, 250n);
    }
  }
}

test(tickerQuoteUsesChainConfig);
test(tickerQuoteReportsStale);

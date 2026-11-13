import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { decodeV2Contract } from '@kindle/sdk';
import { canonicalCandleToDisplayBar, confirmedChartMarkers } from './udf-datafeed';

const recorded = JSON.parse(
  readFileSync(new URL('../../core/test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8')
);
const execution = decodeV2Contract('Candle', recorded.candles.body.data.items[0]);
const prices = ['open', 'high', 'low', 'close'] as const;

function developmentCandle() {
  const candle = structuredClone(execution);
  candle.seriesModelVersion = 'development-positioned-candles-v1';
  for (const name of prices) {
    candle[name].source.kind = 'post_swap_spot';
    candle[name].source.amountBasis = 'effective_reserves';
    candle[name].source.sourceReference = null;
  }
  return candle;
}

function executionCandleRetainsRecordedValues() {
  const candle = structuredClone(execution);
  const bar = canonicalCandleToDisplayBar(candle);
  expect(bar.time).toBe(Date.parse(candle.start));
  for (const name of prices) {
    expect(bar[name]).toBe(Number(candle[name].numerator) / Number(candle[name].denominator));
  }
  expect(bar.volume).toBe(Number(candle.volumeUsdlRaw) / 1_000_000);
}
test(executionCandleRetainsRecordedValues, executionCandleRetainsRecordedValues);

function developmentCandleAcceptsPostSwapSpot() {
  expect(canonicalCandleToDisplayBar(developmentCandle())).toEqual(canonicalCandleToDisplayBar(execution));
}
test(developmentCandleAcceptsPostSwapSpot, developmentCandleAcceptsPostSwapSpot);

function developmentCandleRefusesExecutionPrices() {
  expect(() => canonicalCandleToDisplayBar({ ...execution, seriesModelVersion: 'development-positioned-candles-v1' }))
    .toThrow('Canonical candle price provenance mismatch');
}
test(developmentCandleRefusesExecutionPrices, developmentCandleRefusesExecutionPrices);

function otherSeriesRefusesDevelopmentPrices() {
  expect(() => canonicalCandleToDisplayBar({ ...developmentCandle(), seriesModelVersion: execution.seriesModelVersion }))
    .toThrow('Canonical candle price provenance mismatch');
}
test(otherSeriesRefusesDevelopmentPrices, otherSeriesRefusesDevelopmentPrices);

function candleRefusesCrossedPriceSources() {
  for (const name of prices) {
    const development = developmentCandle();
    development[name] = structuredClone(execution[name]);
    expect(() => canonicalCandleToDisplayBar(development)).toThrow('Canonical candle price provenance mismatch');
    const other = structuredClone(execution);
    other[name] = developmentCandle()[name];
    expect(() => canonicalCandleToDisplayBar(other)).toThrow('Canonical candle price provenance mismatch');
    const crossedBasis = developmentCandle();
    crossedBasis[name].source.amountBasis = 'paid_input';
    expect(() => canonicalCandleToDisplayBar(crossedBasis)).toThrow('Canonical candle price provenance mismatch');
    const crossedKind = developmentCandle();
    crossedKind[name].source.kind = 'gross_execution';
    expect(() => canonicalCandleToDisplayBar(crossedKind)).toThrow('Canonical candle price provenance mismatch');
  }
}
test(candleRefusesCrossedPriceSources, candleRefusesCrossedPriceSources);

function developmentCandleMarkersFollowCanonicalTrades() {
  const trades = recorded.trades.body.data.items;
  const development = developmentCandle();
  const expected = confirmedChartMarkers([execution], trades);
  const actual = confirmedChartMarkers([development], trades);
  expect(expected.markers).toHaveLength(0);
  expect(expected.partial).toBe(true);
  expect(actual.markers).toEqual(expected.markers);
  expect(actual.partial).toBe(expected.partial);
}
test(developmentCandleMarkersFollowCanonicalTrades, developmentCandleMarkersFollowCanonicalTrades);

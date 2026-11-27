import { expect, test, vi } from 'vitest';
import { decodeV2Contract } from '@kindle/sdk';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import type { CandleHistoryOutcome, fetchCandleHistoryOutcome } from '@/core/clients/candle-stats';
import { allTimeHighMarketCap, allTimeHighWindow, loadAllTimeHighHistory } from './all-time-high';
import type { PoolStats } from './use-stats-batch';

const day = 86_400;
const now = Date.parse('2026-11-26T12:34:56Z') / 1000;
const createdAt = new Date((now - 40 * day) * 1000).toISOString();
const from = Math.floor((now - 40 * day) / day) * day;
const pool = '0x' + 'a'.repeat(40);
const stats: PoolStats = {
  ...decodeV2Contract('StatsBatchResponse', v2Reads.responses.statsBatchReal.body).data.items[0],
  poolAddress: pool, totalSupplyRaw: '1000000000000000', totalSupplyValuationUsdlRaw: '100000000000',
};
const candles: CandleHistoryOutcome = { status: 'ok', history: {
  t: [from, from + 20 * day, Math.floor(now / day) * day],
  o: [0.0001, 0.00012, 0.0001], h: [0.00012, 0.0002, 0.00015],
  l: [0.00009, 0.0001, 0.00009], c: [0.0001, 0.00015, 0.0001], v: [10, 20, 30],
} };

async function allTimeHighComesFromTheCandlesOfTheWholeLife() {
  const read = vi.fn<typeof fetchCandleHistoryOutcome>().mockResolvedValue(candles);
  const history = await loadAllTimeHighHistory(pool.toUpperCase().replace('0X', '0x'), createdAt, { now, read });
  expect(read).toHaveBeenCalledExactlyOnceWith(pool, '1D', from, now, { signal: undefined });
  expect(history).toEqual({ status: 'candles', highestPrice: 0.0002, window: { from, to: now } });
  expect(allTimeHighMarketCap(history, stats, 6)).toBe(200_000);
}
test(allTimeHighComesFromTheCandlesOfTheWholeLife.name, allTimeHighComesFromTheCandlesOfTheWholeLife);

async function liveMarketCapSetsANewAllTimeHigh() {
  const read = vi.fn<typeof fetchCandleHistoryOutcome>().mockResolvedValue(candles);
  const history = await loadAllTimeHighHistory(pool, createdAt, { now, read });
  expect(allTimeHighMarketCap(history, { ...stats, totalSupplyValuationUsdlRaw: '300000000000' }, 6)).toBe(300_000);
  expect(read).toHaveBeenCalledTimes(1);
}
test(liveMarketCapSetsANewAllTimeHigh.name, liveMarketCapSetsANewAllTimeHigh);

async function onlyWholeLifeCoverageIssuesARead() {
  const read = vi.fn<typeof fetchCandleHistoryOutcome>().mockResolvedValue(candles);
  for (const created of [null, undefined, '', 'invalid', new Date((now + day) * 1000).toISOString(),
    new Date((now - 365 * day) * 1000).toISOString(), new Date((now - 400 * day) * 1000).toISOString()]) {
    expect(await loadAllTimeHighHistory(pool, created, { now, read })).toEqual({ status: 'not_covered' });
  }
  expect(read).not.toHaveBeenCalled();
  const youngestCovered = new Date((now - 365 * day + 1) * 1000).toISOString();
  expect(allTimeHighWindow(youngestCovered, now)).toEqual({ from: Math.floor((now - 365 * day + 1) / day) * day, to: now });
  await loadAllTimeHighHistory(pool, youngestCovered, { now, read });
  expect(read).toHaveBeenCalledTimes(1);
}
test(onlyWholeLifeCoverageIssuesARead.name, onlyWholeLifeCoverageIssuesARead);

async function anEmptyHistoryUsesTheCurrentMarketCap() {
  const read = vi.fn<typeof fetchCandleHistoryOutcome>().mockResolvedValue({ status: 'no_data', nextTime: null });
  const history = await loadAllTimeHighHistory(pool, createdAt, { now, read });
  expect(history).toEqual({ status: 'no_candles', window: { from, to: now } });
  expect(allTimeHighMarketCap(history, stats, 6)).toBe(100_000);
  expect(allTimeHighMarketCap(history, { ...stats, totalSupplyValuationUsdlRaw: '80000000000' }, 6)).toBe(80_000);
}
test(anEmptyHistoryUsesTheCurrentMarketCap.name, anEmptyHistoryUsesTheCurrentMarketCap);

async function failedHistoryDoesNotBecomeAnEmptyHistory() {
  const outcome: CandleHistoryOutcome = { status: 'unavailable', httpStatus: 503, code: 'DEPENDENCY_UNAVAILABLE', message: 'Candle history unavailable', retryable: false, retryAfterMs: null, requestId: null };
  const read = vi.fn<typeof fetchCandleHistoryOutcome>().mockResolvedValue(outcome);
  const history = await loadAllTimeHighHistory(pool, createdAt, { now, read });
  expect(history).toEqual({ status: 'failed', outcome });
  expect(allTimeHighMarketCap(history, stats, 6)).toBeNull();
  const controller = new AbortController();
  controller.abort();
  read.mockResolvedValue({ status: 'aborted', reason: controller.signal.reason });
  expect(await loadAllTimeHighHistory(pool, createdAt, { now, read, signal: controller.signal })).toEqual({ status: 'failed', outcome: { status: 'aborted', reason: controller.signal.reason } });
  expect(read).toHaveBeenLastCalledWith(pool, '1D', from, now, { signal: controller.signal });
}
test(failedHistoryDoesNotBecomeAnEmptyHistory.name, failedHistoryDoesNotBecomeAnEmptyHistory);

async function valuationUsesStatisticsSupplyAndVerifiedTokenDecimals() {
  const history = await loadAllTimeHighHistory(pool, createdAt, { now, read: vi.fn<typeof fetchCandleHistoryOutcome>().mockResolvedValue(candles) });
  expect(allTimeHighMarketCap(history, { ...stats, totalSupplyRaw: '1000000000000000000000000000' }, 18)).toBe(200_000);
  expect(allTimeHighMarketCap(history, { ...stats, totalSupplyRaw: null }, 6)).toBeNull();
  expect(allTimeHighMarketCap(history, stats, null)).toBeNull();
  expect(allTimeHighMarketCap(history, { ...stats, supplyValuationQuality: 'unavailable' }, 6)).toBeNull();
  expect(allTimeHighMarketCap(history, null, 6)).toBeNull();
}
test(valuationUsesStatisticsSupplyAndVerifiedTokenDecimals.name, valuationUsesStatisticsSupplyAndVerifiedTokenDecimals);

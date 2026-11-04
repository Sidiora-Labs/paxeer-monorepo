import { describe, expect, it, test } from 'vitest';
import type { Candle, PoolStats, Price, Trade } from '../../../../../../../sdk/launchpad/packages/production-contracts/src/types.gen';
import { comparePositions, deriveWindowStats, windowRange } from './window-stats';

const pool = '0x' + 'ab'.repeat(20);
const token = '0x' + 'cd'.repeat(20);
const usdl = '0x' + 'ef'.repeat(20);

function position(height: number, transactionIndex = 0, logIndex = 0): Trade['position'] {
  return { blockHash: '0x' + height.toString(16).padStart(64, '0'), chainId: '125', height: String(height), logIndex: String(logIndex), transactionHash: '0x' + (height * 100 + logIndex).toString(16).padStart(64, '0'), transactionIndex: String(transactionIndex) };
}

function price(numerator: string, denominator = '1000000'): Price {
  return {
    baseAsset: { address: token, chainId: '125', currencyCode: null, decimals: 6, kind: 'erc20' },
    quoteAsset: { address: usdl, chainId: '125', currencyCode: null, decimals: 6, kind: 'erc20' },
    numerator, denominator,
    source: { amountBasis: 'paid_input', kind: 'gross_execution', observedAt: '2026-10-13T12:00:00.000Z', position: null, providerId: null, sourceEventId: null, sourceReference: null },
  };
}

function candle(start: string, first: number, last: number, buy: string, sell: string, trades: number, open: string, close: string, coverage: Candle['coverage'] = 'complete'): Candle {
  return {
    poolAddress: pool, interval: '1m', start, coverage, synthetic: false, seriesModelVersion: 'v1', snapshotId: 's', revision: '1',
    firstPosition: position(first), lastPosition: position(last),
    open: price(open), close: price(close), high: price(close), low: price(open),
    buyVolumeUsdlRaw: buy, sellVolumeUsdlRaw: sell, volumeUsdlRaw: (BigInt(buy) + BigInt(sell)).toString(), volumeTokenRaw: '0',
    trades: String(trades), uniqueTraderCount: null, largeTradeCount: null, largeTradePolicyId: null, largeTradeThresholdUsdlRaw: null,
  };
}

function fill(height: number, side: Trade['side'], usdlAmountRaw: string): Trade {
  return {
    ammInputAfterFeeRaw: null, codeEpoch: '1', emittingContract: pool, eventId: `e${height}`, feeAssetAddress: usdl, feeAssetDecimals: 6, feeDisposition: 'distributed_usdl', feeRaw: '0',
    inputDerivationQuality: 'exact', paidInputRaw: null, poolAddress: pool, position: position(height), pricedInputRaw: null, recipient: null, recipientAttribution: 'unavailable', recipientEvidenceRefs: [],
    sender: '0x' + '11'.repeat(20), side, tokenAmountRaw: '1', usdlAmountRaw,
  };
}

const stats = { buys24h: '120', sells24h: '80' } as Pick<PoolStats, 'buys24h' | 'sells24h'> as PoolStats;

// Two one-minute buckets: heights 100..102 (2 buys, 1 sell) and 105..106 (1 buy, 1 sell).
const candles = [
  candle('2026-10-13T12:01:00.000Z', 105, 106, '3000000', '1000000', 2, '1100000', '1210000'),
  candle('2026-10-13T12:00:00.000Z', 100, 102, '5000000', '2000000', 3, '1000000', '1100000'),
];
const allFills = [fill(106, 'sell', '1000000'), fill(105, 'buy', '3000000'), fill(102, 'sell', '2000000'), fill(101, 'buy', '2000000'), fill(100, 'buy', '3000000'), fill(97, 'buy', '9000000')];
const from = Date.parse('2026-10-13T12:00:00.000Z');

export function windowSnapshotExcludesNewerLiveFills() {
  const laterInLastBlock = { ...fill(106, 'buy', '9000000'), eventId: 'later-in-last-block', position: position(106, 0, 1) };
  const earlierInFirstBlock = { ...fill(100, 'buy', '9000000'), eventId: 'earlier-in-first-block', position: position(100, 0, 0) };
  const boundedCandles = [candles[0], { ...candles[1], firstPosition: position(100, 0, 1) }];
  const boundedFills = allFills.map(row => row.position.height === '100' ? { ...row, position: position(100, 0, 1) } : row);
  const atPositionBoundaries = deriveWindowStats('5m', boundedCandles, [laterInLastBlock, earlierInFirstBlock, ...boundedFills], null, from);
  expect(atPositionBoundaries.counts).toMatchObject({ buys: 3, sells: 2, retained: 5, complete: true });
  expect(atPositionBoundaries.volumeRaw).toBe(11_000_000n);
  expect(atPositionBoundaries.trades).toBe(5n);
  const result = deriveWindowStats('5m', candles, [fill(107, 'buy', '9000000'), laterInLastBlock, ...allFills], null, from);
  expect(result.counts).toMatchObject({ buys: 3, sells: 2, retained: 5, complete: true });
  expect(result.volumeRaw).toBe(11_000_000n);
  expect(result.trades).toBe(5n);
  const missing = deriveWindowStats('5m', candles, [fill(107, 'buy', '9000000'), ...allFills.filter(row => row.position.height !== '100' && row.position.height !== '97')], null, from);
  expect(missing.counts).toMatchObject({ buys: 2, sells: 2, retained: 4, complete: false, oldestHeight: '101' });
  expect(missing.volumeRaw).toBe(11_000_000n);
}

export function windowSnapshotDeduplicatesEvents() {
  const result = deriveWindowStats('5m', candles, [...allFills, { ...allFills[0] }, { ...allFills[1] }], null, from);
  expect(result.counts).toMatchObject({ buys: 3, sells: 2, retained: 5, complete: true });
  const missing = deriveWindowStats('5m', candles, [allFills[0], { ...allFills[0] }, allFills[1], allFills[2], allFills[4]], null, from);
  expect(missing.counts).toMatchObject({ buys: 2, sells: 2, retained: 4, complete: false });
  expect(missing.trades).toBe(5n);
}

export function windowSnapshotMissingInRangeHistoryStaysPartial() {
  const result = deriveWindowStats('5m', candles, allFills.filter(row => row.position.height !== '101'), null, from);
  expect(result.counts).toMatchObject({ buys: 2, sells: 2, retained: 4, complete: false, oldestHeight: '100', coveredSince: '2026-10-13T12:00:00.000Z' });
  expect(result.volumeRaw).toBe(11_000_000n);
  expect(result.buyUsdlRaw).toBe(8_000_000n);
  expect(result.sellUsdlRaw).toBe(3_000_000n);
  expect(result.netRaw).toBe(5_000_000n);
  expect(result.trades).toBe(5n);
}

export function windowSnapshotExcludesGapsAndPartialCoverage() {
  const gap = deriveWindowStats('5m', candles, [fill(103, 'buy', '9000000'), ...allFills.filter(row => row.position.height !== '101')], null, from);
  expect(gap.counts).toMatchObject({ buys: 2, sells: 2, retained: 4, complete: false });
  const partial = deriveWindowStats('5m', [{ ...candles[0], coverage: 'partial' }, candles[1]], allFills, null, from);
  expect(partial.counts).toMatchObject({ buys: 3, sells: 2, retained: 5, complete: false });
  expect(partial.partialCandles).toBe(1);
}

test(windowSnapshotExcludesNewerLiveFills.name, windowSnapshotExcludesNewerLiveFills);
test(windowSnapshotDeduplicatesEvents.name, windowSnapshotDeduplicatesEvents);
test(windowSnapshotMissingInRangeHistoryStaysPartial.name, windowSnapshotMissingInRangeHistoryStaysPartial);
test(windowSnapshotExcludesGapsAndPartialCoverage.name, windowSnapshotExcludesGapsAndPartialCoverage);

describe('windowRange', () => {
  it('is bucket aligned and includes the current bucket', () => {
    const now = Date.parse('2026-10-13T12:04:30.000Z');
    expect(windowRange('5m', now)).toEqual({ from: Date.parse('2026-10-13T12:00:00.000Z'), to: Date.parse('2026-10-13T12:05:00.000Z') });
    expect(windowRange('1h', now)).toEqual({ from: Date.parse('2026-10-13T11:05:00.000Z'), to: Date.parse('2026-10-13T12:05:00.000Z') });
    expect(windowRange('24h', now).to - windowRange('24h', now).from).toBe(24 * 3_600_000);
  });
});

describe('comparePositions', () => {
  it('orders by height, then transaction index, then log index', () => {
    expect(comparePositions(position(1), position(2))).toBe(-1);
    expect(comparePositions(position(2, 1), position(2, 0))).toBe(1);
    expect(comparePositions(position(2, 0, 3), position(2, 0, 3))).toBe(0);
  });
});

describe('deriveWindowStats', () => {
  it('sums candle USDL figures and counts retained fills inside the window', () => {
    const result = deriveWindowStats('5m', candles, allFills, null, from);
    expect(result.volumeRaw).toBe(11_000_000n);
    expect(result.buyUsdlRaw).toBe(8_000_000n);
    expect(result.sellUsdlRaw).toBe(3_000_000n);
    expect(result.netRaw).toBe(5_000_000n);
    expect(result.trades).toBe(5n);
    expect(result.changePct).toBeCloseTo(21, 6);
    expect(result.counts).toMatchObject({ source: 'fills', buys: 3, sells: 2, complete: true, retained: 5 });
    expect(result.since).toBe('2026-10-13T12:00:00.000Z');
    expect(result.partialCandles).toBe(0);
  });

  it('states coverage when retained fills do not reach the window start and never extrapolates', () => {
    const retained = allFills.filter(row => Number(row.position.height) >= 101);
    const result = deriveWindowStats('5m', candles, retained, null, from);
    expect(result.counts).toMatchObject({ buys: 2, sells: 2, complete: false, retained: 4, oldestHeight: '101', coveredSince: '2026-10-13T12:00:00.000Z' });
    const newest = deriveWindowStats('5m', candles, allFills.slice(0, 2), null, from);
    expect(newest.counts).toMatchObject({ buys: 1, sells: 1, complete: false, retained: 2, coveredSince: '2026-10-13T12:01:00.000Z' });
    expect(newest.volumeRaw).toBe(11_000_000n);
  });

  it('treats an empty window as zero activity with complete coverage', () => {
    const result = deriveWindowStats('1h', [], allFills, null, from);
    expect(result.volumeRaw).toBe(0n);
    expect(result.changePct).toBeNull();
    expect(result.counts).toMatchObject({ buys: 0, sells: 0, complete: true });
  });

  it('takes 24h side counts from the pool stats projection', () => {
    const result = deriveWindowStats('24h', candles, [], stats, from);
    expect(result.counts).toMatchObject({ source: 'pool-stats', buys: 120, sells: 80, complete: true });
    expect(result.buyUsdlRaw).toBe(8_000_000n);
  });

  it('flags partial candle buckets', () => {
    const result = deriveWindowStats('5m', [candles[0], { ...candles[1], coverage: 'partial' }], allFills, null, from);
    expect(result.partialCandles).toBe(1);
  });
});

import { decodeV2Contract } from '@kindle/sdk';
import { expect, test } from 'vitest';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { formatShare, noStatsReason, sharesNotComputedReason, statsShare } from './pool-stats-display';
import type { PoolStats } from './use-stats-batch';

const recorded = decodeV2Contract('StatsBatchResponse', v2Reads.responses.statsBatchReal.body).data.items[0];
const holders = ['top10', 'creator'] as const;
const newCoin: PoolStats = {
  ...recorded,
  circulationModel: 'supply_minus_pool_reserve_v1',
  totalSupplyRaw: '1000000000000000',
  circulatingSupplyRaw: '398481149474',
  top10HolderShareBps: 10_000,
  creatorHoldingShareBps: 10_000,
};

function holderSharesUseWholeSupplyForNewCoin() {
  for (const holder of holders) {
    const share = statsShare(newCoin, holder);
    expect(share).toEqual({ value: 0.0398, reason: null });
    expect(formatShare(share.value!)).toBe('0.04%');
  }
}

function holderSharesReadRecordedSupply() {
  for (const holder of holders) {
    const share = statsShare(recorded, holder);
    expect(share).toEqual({ value: 0.4803, reason: null });
    expect(formatShare(share.value!)).toBe('0.48%');
  }
}

function holderSharesScaleWithCirculation() {
  for (const holder of holders) {
    const row: PoolStats = { ...newCoin, top10HolderShareBps: 2500, creatorHoldingShareBps: 2500 };
    expect(statsShare({ ...row, circulatingSupplyRaw: row.totalSupplyRaw }, holder)).toEqual({ value: 25, reason: null });
    expect(statsShare({ ...row, circulatingSupplyRaw: '500000000000000' }, holder)).toEqual({ value: 12.5, reason: null });
  }
}

function holderSharesRequireValidSupplyAndModel() {
  const invalid: Partial<PoolStats>[] = [
    { circulationModel: 'another_model' },
    { circulationModel: undefined },
    { circulationModel: null },
    { circulatingSupplyRaw: undefined },
    { circulatingSupplyRaw: null },
    { totalSupplyRaw: undefined },
    { totalSupplyRaw: null },
    { totalSupplyRaw: '0' },
    { circulatingSupplyRaw: '1000000000000001' },
  ];
  for (const raw of ['', '-1', '1.5', '01', '1e3', ' 1', '1\n', '1'.repeat(79)]) {
    invalid.push({ circulatingSupplyRaw: raw }, { totalSupplyRaw: raw });
  }
  for (const bps of [undefined, null, -1, 10_001, 0.5, NaN, Infinity]) {
    invalid.push({ top10HolderShareBps: bps, creatorHoldingShareBps: bps });
  }
  for (const patch of invalid) {
    for (const holder of holders) {
      expect(statsShare({ ...newCoin, ...patch }, holder)).toEqual({ value: null, reason: sharesNotComputedReason });
    }
  }
  for (const holder of holders) {
    expect(statsShare(null, holder)).toEqual({ value: null, reason: noStatsReason });
    expect(statsShare(undefined, holder)).toEqual({ value: null, reason: noStatsReason });
    expect(statsShare({ ...newCoin, circulatingSupplyRaw: '0' }, holder)).toEqual({ value: 0, reason: null });
    expect(statsShare({ ...newCoin, top10HolderShareBps: 0, creatorHoldingShareBps: 0 }, holder)).toEqual({ value: 0, reason: null });
  }
}

function holderSharesPreserveLargeSupplyDigits() {
  const row: PoolStats = {
    ...newCoin,
    totalSupplyRaw: '1234567890123456789012345678',
    circulatingSupplyRaw: '1234567890123456789012345678',
    top10HolderShareBps: 3333,
    creatorHoldingShareBps: 3333,
  };
  for (const holder of holders) {
    expect(statsShare(row, holder)).toEqual({ value: 33.33, reason: null });
    expect(statsShare({ ...row, circulatingSupplyRaw: '1234567890123456789012345677' }, holder)).toEqual({ value: 33.3299, reason: null });
  }
}

function holderShareFormattingKeepsSmallSharesVisible() {
  const cases: Array<[number, string]> = [
    [100, '100%'],
    [25, '25%'],
    [12.34, '12.3%'],
    [0.3, '0.3%'],
    [0.48, '0.48%'],
    [0.0398, '0.04%'],
    [0.004, '<0.01%'],
    [0, '0%'],
  ];
  for (const [percent, text] of cases) expect(formatShare(percent)).toBe(text);
}

test(holderSharesUseWholeSupplyForNewCoin, holderSharesUseWholeSupplyForNewCoin);
test(holderSharesReadRecordedSupply, holderSharesReadRecordedSupply);
test(holderSharesScaleWithCirculation, holderSharesScaleWithCirculation);
test(holderSharesRequireValidSupplyAndModel, holderSharesRequireValidSupplyAndModel);
test(holderSharesPreserveLargeSupplyDigits, holderSharesPreserveLargeSupplyDigits);
test(holderShareFormattingKeepsSmallSharesVisible, holderShareFormattingKeepsSmallSharesVisible);

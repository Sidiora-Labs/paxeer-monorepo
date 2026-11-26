import { expect, test } from 'vitest';
import { decodeV2Contract } from '@kindle/sdk';
import type { HoldersPage } from '@/core/market/use-holders';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { buildHolderRows, holderValuationPrice } from './holders-table';

const detail = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body).data;

function holderValuesComeFromTheSpotPriceOfTheStats() {
  const stats = { ...detail.stats, spotPriceWad: '10009702352249' };
  const livePrice = 0.0000103143;
  const price = holderValuationPrice(stats, livePrice);
  expect(price).toBe(0.000010009702352249);
  expect(holderValuationPrice(stats, null)).toBe(price);
  expect(holderValuationPrice({ ...stats, spotPriceWad: null }, livePrice)).toBe(livePrice);
  expect(holderValuationPrice({ ...stats, spotPriceWad: null }, null)).toBeNull();
  expect(holderValuationPrice(null, livePrice)).toBe(livePrice);
  expect(holderValuationPrice(null, null)).toBeNull();
  expect(holderValuationPrice(null, Number.NaN)).toBeNull();

  const page: HoldersPage = {
    rows: [{ wallet: detail.token.address, tokenAddress: detail.token.address, balanceRaw: '1000000000000', share: null }],
    response: null,
    canonical: null,
    legacyShare: null,
    legacySource: null,
    nextCursor: null,
  };
  const token = { decimals: 6, decimalsQuality: 'verified' as const, totalSupplyRaw: '1000000000000000', supplyQuality: 'verified' as const };
  const rows = buildHolderRows([page], token, price);
  expect(rows).toHaveLength(1);
  expect(rows[0].positionUsdlRaw).toBe(10009702n);
  expect(buildHolderRows([page], token, holderValuationPrice(null, livePrice))[0].positionUsdlRaw).toBe(10314300n);
  expect(buildHolderRows([page], token, holderValuationPrice(null, null))[0].positionUsdlRaw).toBeNull();
}

test(holderValuesComeFromTheSpotPriceOfTheStats, holderValuesComeFromTheSpotPriceOfTheStats);

import { expect, test } from 'vitest';
import { decodeV2Contract } from '@kindle/sdk';
import { formatSpotPrice, statsSpotPrice } from '@/core/market/pool-stats-display';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { formatPrice } from '@/utils/format';
import { tokenHeaderPrice } from './token-header';

const stats = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body).data.stats;
const livePrice = 0.0000103143;

function headerPriceComesFromTheSpotPriceOfTheStats() {
  const input = { ...stats, spotPriceWad: '10009702352249' };
  const spot = statsSpotPrice(input);
  const price = tokenHeaderPrice(input, livePrice);
  expect(price).toEqual({ value: formatSpotPrice(spot.value), reason: spot.reason });
  expect(price.value).not.toBe(formatPrice(livePrice));
}

function headerPriceFallsBackToTheLiveQuote() {
  for (const input of [{ ...stats, spotPriceWad: null }, { ...stats, spotPriceWad: undefined }, null]) {
    expect(tokenHeaderPrice(input, livePrice)).toEqual({ value: formatPrice(livePrice), reason: null });
    expect(tokenHeaderPrice(input, null)).toEqual({ value: null, reason: statsSpotPrice(input).reason });
    expect(tokenHeaderPrice(input, Number.NaN)).toEqual({ value: null, reason: statsSpotPrice(input).reason });
  }
}

test(headerPriceComesFromTheSpotPriceOfTheStats, headerPriceComesFromTheSpotPriceOfTheStats);
test(headerPriceFallsBackToTheLiveQuote, headerPriceFallsBackToTheLiveQuote);

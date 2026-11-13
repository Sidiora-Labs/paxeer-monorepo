// @vitest-environment jsdom

import config from 'configs/app';
import { expect, test, vi } from 'vitest';

import { getChartData } from './chart';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL: 'PAX',
    NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL: 'USDC',
    NEXT_PUBLIC_MULTICHAIN_ENABLED: 'false',
  };
});

function secondaryPriceLabelUsesConfiguredSecondarySymbol() {
  expect(config.chain.currency.symbol).toBe('PAX');
  expect(config.chain.secondaryCoin.symbol).toBe('USDC');

  const prices = [ { date: new Date('2026-01-01T00:00:00Z'), value: 1.25 } ];
  const primary = getChartData('coin_price', prices);
  const secondary = getChartData('secondary_coin_price', prices);

  expect(primary).toHaveLength(1);
  expect(primary[0].name).toBe('PAX price');
  expect(secondary).toHaveLength(1);
  expect(secondary[0].id).toBe('secondary_coin_price');
  expect(secondary[0].name).toBe('USDC price');
  expect(secondary[0].items).toEqual(prices);
  expect(secondary[0].valueFormatter?.(1.25)).toBe('$1.25');
}

test('secondary-price-label: secondaryPriceLabelUsesConfiguredSecondarySymbol', secondaryPriceLabelUsesConfiguredSecondarySymbol);

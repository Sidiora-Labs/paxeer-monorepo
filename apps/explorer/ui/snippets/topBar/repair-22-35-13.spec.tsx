// @vitest-environment jsdom

import { QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

import { createMarketQueryClient, marketStats, marketStyleRules, renderMarketData } from 'ui/home/marketData.test-utils';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test, vi } from 'vitest';
import { within } from 'vitest/lib';

import TopBar from './TopBar';
import TopBarStats from './TopBarStats';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL: 'PAX',
    NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL: 'SID',
    NEXT_PUBLIC_HIDE_NATIVE_COIN_PRICE: 'false',
    NEXT_PUBLIC_FEATURED_NETWORKS: 'https://localhost:3000/featured-networks.json',
    NEXT_PUBLIC_STATS_API_HOST: '',
  };
});

function topBarShowsBothMarketPricesAndDayChanges() {
  const client = createMarketQueryClient();
  const { container, unmount } = render(<QueryClientProvider client={ client }><TopBarStats/></QueryClientProvider>);
  try {
    const pax = within(container).getByRole('group', { name: 'PAX market price' });
    const sid = within(container).getByRole('group', { name: 'SID market price' });
    expect(pax.querySelector('[data-price]')?.textContent).toBe('$2.125');
    expect(sid.querySelector('[data-price]')?.textContent).toBe('$0.075');
    expect(within(pax).getByLabelText('PAX price change in 24 hours').textContent).toBe('5.25%');
    expect(within(sid).getByLabelText('SID price change in 24 hours').textContent).toBe('-3.50%');
    expect(within(pax).getByLabelText('PAX price change in 24 hours').getAttribute('data-trend')).toBe('up');
    expect(within(sid).getByLabelText('SID price change in 24 hours').getAttribute('data-trend')).toBe('down');
    expect(marketStyleRules(pax.parentElement!)).toContain('flex-wrap:wrap');
    for (const group of [ pax, sid ]) {
      expect(group.hasAttribute('hidden')).toBe(false);
      expect(group.querySelector('[data-price]')?.getAttribute('class')).toBeTruthy();
    }
  } finally {
    unmount();
    client.clear();
  }
}

test(topBarShowsBothMarketPricesAndDayChanges.name, topBarShowsBothMarketPricesAndDayChanges);

function absentMarketFieldsRenderDashes() {
  for (const stats of [
    { ...marketStats, coin_price: null, coin_price_change_percentage: null, secondary_coin_price: null, secondary_coin_price_change_percentage: null },
    { ...marketStats, coin_price: null, coin_price_change_percentage: null, secondary_coin_price: undefined, secondary_coin_price_change_percentage: undefined },
  ]) {
    const container = renderMarketData(<TopBarStats/>, createMarketQueryClient({ stats }));
    for (const symbol of [ 'PAX', 'SID' ]) {
      const group = within(container).getByRole('group', { name: `${ symbol } market price` });
      expect(group.querySelector('[data-price]')?.textContent).toBe('—');
      expect(group.querySelector('[data-change]')?.textContent).toBe('—');
      expect(group.querySelector('[data-change]')?.getAttribute('data-trend')).toBe('flat');
      expect(group.textContent).not.toContain('$0');
      expect(group.textContent).not.toContain('0.00%');
    }
  }
}

test(absentMarketFieldsRenderDashes.name, absentMarketFieldsRenderDashes);

function zeroMarketValuesRemainVisible() {
  const stats = { ...marketStats, coin_price: '0', coin_price_change_percentage: 0, secondary_coin_price: '0', secondary_coin_price_change_percentage: 0 };
  const container = renderMarketData(<TopBarStats/>, createMarketQueryClient({ stats }));
  for (const symbol of [ 'PAX', 'SID' ]) {
    const group = within(container).getByRole('group', { name: `${ symbol } market price` });
    expect(group.querySelector('[data-price]')?.textContent).toBe('$0.00');
    expect(group.querySelector('[data-change]')?.textContent).toBe('0.00%');
  }
}

test(zeroMarketValuesRemainVisible.name, zeroMarketValuesRemainVisible);

function eachDayChangeIsIndependentOfItsPrice() {
  const stats = { ...marketStats, coin_price_change_percentage: null, secondary_coin_price: null };
  const container = renderMarketData(<TopBarStats/>, createMarketQueryClient({ stats }));
  const pax = within(container).getByRole('group', { name: 'PAX market price' });
  const sid = within(container).getByRole('group', { name: 'SID market price' });
  expect(pax.querySelector('[data-price]')?.textContent).toBe('$2.125');
  expect(pax.querySelector('[data-change]')?.textContent).toBe('—');
  expect(sid.querySelector('[data-price]')?.textContent).toBe('—');
  expect(sid.querySelector('[data-change]')?.textContent).toBe('-3.50%');
}

test(eachDayChangeIsIndependentOfItsPrice.name, eachDayChangeIsIndependentOfItsPrice);

function topBarCarriesMarketPricesOutsideSettings() {
  const container = renderMarketData(<TopBar/>);
  const bar = container.querySelector('[data-label="header-utilities"]') as HTMLElement;
  const pax = within(bar).getByRole('group', { name: 'PAX market price' });
  const sid = within(bar).getByRole('group', { name: 'SID market price' });
  expect(pax.querySelector('[data-price]')?.textContent).toBe('$2.125');
  expect(sid.querySelector('[data-price]')?.textContent).toBe('$0.075');
  expect(pax.closest('[role="dialog"]')).toBeNull();
  expect(sid.closest('[role="dialog"]')).toBeNull();
  expect(within(bar).getByLabelText('User settings')).toBeTruthy();
}

test(topBarCarriesMarketPricesOutsideSettings.name, topBarCarriesMarketPricesOutsideSettings);

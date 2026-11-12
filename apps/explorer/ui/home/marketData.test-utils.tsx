import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { ChartMarketResponse, ChartSecondaryCoinMarketResponse, ChartTransactionResponse } from 'types/api/charts';
import type { HomeStats } from 'types/api/stats';

import { getResourceKey } from 'lib/api/useApiQuery';
import { Wrapper } from 'ui/shared/layout/testWrapper';

export const marketStats: HomeStats = {
  average_block_time: 2000,
  coin_price: '2.125',
  coin_price_change_percentage: 5.25,
  secondary_coin_price: '0.075',
  secondary_coin_price_change_percentage: -3.5,
  secondary_coin_market_cap: '75000000',
  market_cap: '212500000',
  total_blocks: '1000',
  total_addresses: '300',
  total_transactions: '5000',
  transactions_today: '50',
  total_gas_used: '100000',
  gas_used_today: '1000',
  gas_prices: null,
  gas_price_updated_at: null,
  gas_prices_update_in: 0,
  static_gas_price: null,
  network_utilization_percentage: 10,
  tvl: null,
};

export const paxMarket: ChartMarketResponse = {
  available_supply: '100000000',
  chart_data: [
    { date: '2026-01-01', closing_price: '2', market_cap: '200000000' },
    { date: '2026-01-02', closing_price: null, market_cap: null },
    { date: '2026-01-03', closing_price: '2.125', market_cap: '212500000' },
  ],
};

export const sidMarket: ChartSecondaryCoinMarketResponse = {
  available_supply: '1000000000',
  chart_data: [
    { date: '2026-01-01', closing_price: '0.08', market_cap: '80000000' },
    { date: '2026-01-02', closing_price: null, market_cap: null },
    { date: '2026-01-03', closing_price: '0.075', market_cap: '75000000' },
  ],
};

export const dailyTransactions: ChartTransactionResponse = {
  chart_data: [
    { date: '2026-01-01', transactions_count: 40 },
    { date: '2026-01-02', transactions_count: null },
    { date: '2026-01-03', transactions_count: 50 },
  ],
};

export function createMarketQueryClient({
  stats = marketStats,
  pax = paxMarket,
  sid = sidMarket,
  transactions = dailyTransactions,
}: {
  stats?: HomeStats;
  pax?: ChartMarketResponse;
  sid?: ChartSecondaryCoinMarketResponse;
  transactions?: ChartTransactionResponse;
} = {}) {
  const client = new QueryClient();
  client.setQueryData(getResourceKey('general:stats'), stats);
  client.setQueryData(getResourceKey('general:stats_charts_market'), pax);
  client.setQueryData(getResourceKey('general:stats_charts_secondary_coin_market'), sid);
  client.setQueryData(getResourceKey('general:stats_charts_txs'), transactions);
  return client;
}

export function renderMarketData(children: React.ReactNode, client = createMarketQueryClient()) {
  const container = document.createElement('div');
  try {
    container.innerHTML = renderToStaticMarkup(
      <Wrapper>
        <QueryClientProvider client={ client }>{ children }</QueryClientProvider>
      </Wrapper>,
    );
    return container;
  } finally {
    client.clear();
  }
}

export function marketStyleRules(element: Element): string {
  function rulesFrom(list: CSSRuleList): Array<string> {
    return Array.from(list).flatMap((rule) => (
      'cssRules' in rule ? rulesFrom((rule as CSSGroupingRule).cssRules) : [ rule.cssText ]
    ));
  }
  return Array.from(document.querySelectorAll('style')).flatMap((tag) => {
    if (tag.textContent) {
      return tag.textContent.match(/[^{}]+\{[^{}]*\}/g) ?? [];
    }
    return tag.sheet ? rulesFrom(tag.sheet.cssRules) : [];
  })
    .filter((rule) => Array.from(element.classList).some((name) => rule.includes(`.${ name }`)))
    .join('').replace(/\s+/g, '');
}

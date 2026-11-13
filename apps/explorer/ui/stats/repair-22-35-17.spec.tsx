// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, waitFor } from '@testing-library/react';
import { once } from 'node:events';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { GeneralApiMiscResourcePayload } from 'lib/api/services/general/misc';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

const originalEnvs = window.__envs;
const requests: Array<string> = [];
const secondaryMarket: GeneralApiMiscResourcePayload<'general:stats_charts_secondary_coin_market'> = {
  available_supply: '200000000',
  chart_data: [
    { date: '2026-01-03', closing_price: '4', market_cap: '800000000' },
    { date: '2026-01-01', closing_price: '2', market_cap: '400000000' },
    { date: '2026-01-02', closing_price: '3', market_cap: '600000000' },
  ],
};
let client: QueryClient;
const server = createServer((request, response) => {
  const path = request.url;
  if (path) requests.push(path);
  if (request.method !== 'GET' || path !== '/api/v2/stats/charts/secondary-coin-market') {
    response.writeHead(404).end();
    return;
  }
  response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(secondaryMarket));
});

fetchMock.disableMocks();
server.listen(0, '127.0.0.1');
await once(server, 'listening');
window.__envs = {
  ...originalEnvs,
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String((server.address() as AddressInfo).port),
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_BASE_PATH: '',
  NEXT_PUBLIC_STATS_API_HOST: '',
};

const { render } = await import('ui/shared/layout/testWrapper');
const { GENERAL_API_RESOURCES } = await import('lib/api/services/general');
const { GENERAL_API_MISC_RESOURCES } = await import('lib/api/services/general/misc');
const { getResourceKey } = await import('lib/api/useApiQuery');
const { getResourceStaleTime, STALE_TIME } = await import('lib/api/useQueryClientConfig');
const { EXPLORER_CHARTS, explorerChartValue } = await import('./explorerCharts');
const { 'default': ExplorerChartWidget } = await import('./ExplorerChartWidget');

beforeEach(() => {
  requests.length = 0;
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
});

afterEach(() => {
  cleanup();
  client.clear();
});

afterAll(async() => {
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
});

test('everyExplorerChartNamesARegisteredGeneralResource', function everyExplorerChartNamesARegisteredGeneralResource() {
  const names = Object.keys(GENERAL_API_RESOURCES).map((name) => `general:${ name }`);
  expect(EXPLORER_CHARTS.map(({ resource }) => resource)).toEqual([
    'general:stats_charts_txs',
    'general:stats_charts_market',
    'general:stats_charts_market',
    'general:stats_charts_secondary_coin_market',
    'general:stats_charts_secondary_coin_market',
  ]);
  for (const chart of EXPLORER_CHARTS) {
    expect(names).toContain(chart.resource);
    expect(getResourceStaleTime(chart.resource)).toBe(STALE_TIME.stats);
  }
  expect(GENERAL_API_MISC_RESOURCES.stats_charts_txs.path).toBe('/api/v2/stats/charts/transactions');
  expect(GENERAL_API_MISC_RESOURCES.stats_charts_market.path).toBe('/api/v2/stats/charts/market');
  expect(GENERAL_API_MISC_RESOURCES.stats_charts_secondary_coin_market.path).toBe('/api/v2/stats/charts/secondary-coin-market');
});

test('sidChartsRenderTheSecondaryMarketRouteResponse', async function sidChartsRenderTheSecondaryMarketRouteResponse() {
  const charts = EXPLORER_CHARTS.filter(({ id }) => id === 'secondaryCoinPrice' || id === 'secondaryCoinMarketCap');
  expect(charts.map(({ field }) => field)).toEqual([ 'closing_price', 'market_cap' ]);
  const { container } = render(
    <QueryClientProvider client={ client }>
      { charts.map((chart) => <ExplorerChartWidget key={ chart.id } chart={ chart } interval="all"/>) }
    </QueryClientProvider>,
  );
  await waitFor(() => {
    expect(container.querySelector('[data-chart-card="secondaryCoinPrice"] [data-chart-value]')?.textContent).toBe('4 USD');
    expect(container.querySelector('[data-chart-card="secondaryCoinMarketCap"] [data-chart-value]')?.textContent).toBe('800,000,000 USD');
    for (const chart of charts) {
      expect(container.querySelector(`[data-chart-card="${ chart.id }"] [data-chart-surface] svg`)).not.toBeNull();
    }
  });
  expect(requests).toEqual([ '/api/v2/stats/charts/secondary-coin-market' ]);
  expect(client.getQueryData(getResourceKey('general:stats_charts_secondary_coin_market'))).toEqual(secondaryMarket);
});

test('secondaryMarketFieldsKeepZeroAndMissingValuesIndependent', function secondaryMarketFieldsKeepZeroAndMissingValuesIndependent() {
  const zero: GeneralApiMiscResourcePayload<'general:stats_charts_secondary_coin_market'>['chart_data'][number] = {
    date: '2026-01-01', closing_price: '0', market_cap: '0',
  };
  expect(explorerChartValue(zero, 'closing_price')).toBe(0);
  expect(explorerChartValue(zero, 'market_cap')).toBe(0);
  expect(explorerChartValue({ ...zero, closing_price: null }, 'closing_price')).toBeUndefined();
  expect(explorerChartValue({ ...zero, closing_price: null }, 'market_cap')).toBe(0);
  expect(explorerChartValue({ date: zero.date, closing_price: '4' }, 'market_cap')).toBeUndefined();
});

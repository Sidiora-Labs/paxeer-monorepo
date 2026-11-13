// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AppProps } from 'next/dist/shared/lib/router/router';
import { createServer } from 'node:http';
import React from 'react';

import type { TransactionsStats } from 'types/api/transaction';

import * as statsFixtures from 'mocks/stats/index';
import * as transactionStatsFixtures from 'mocks/txs/stats';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

let responseStats: TransactionsStats = transactionStatsFixtures.base;
const requests: Array<string> = [];
const server = createServer((request, response) => {
  const path = new URL(request.url ?? '/', 'http://localhost').pathname;
  requests.push(path);
  response.setHeader('content-type', 'application/json');

  if (path === '/api/v2/transactions/stats') {
    response.end(JSON.stringify(responseStats));
  } else if (path === '/api/v2/stats') {
    response.end(JSON.stringify(statsFixtures.base));
  } else {
    response.statusCode = 404;
    response.end(JSON.stringify({ error: 'unexpected_request' }));
  }
});

fetchMock.disableMocks();
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
const address = server.address();
if (!address || typeof address === 'string') {
  throw new Error('Expected a loopback TCP address');
}

const previousEnvs = window.__envs;
window.__envs = {
  ...previousEnvs,
  NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String(address.port),
  NEXT_PUBLIC_API_BASE_PATH: '',
  NEXT_PUBLIC_STATS_API_HOST: '',
};

const { render } = await import('ui/shared/layout/testWrapper');
const { waitFor } = await import('@testing-library/react');
const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': TxsStats } = await import('./TxsStats');

const previousNextData = window.__NEXT_DATA__;
const previousMatchers = window.__DEV_MIDDLEWARE_MATCHERS;
window.__NEXT_DATA__ = { props: {}, page: '/txs', query: {}, buildId: 'transaction-statistics' };
window.__DEV_MIDDLEWARE_MATCHERS = [];
const App = ({ Component, pageProps }: AppProps) => React.createElement(Component, pageProps);
const clients: Array<QueryClient> = [];
const routers: Array<InstanceType<typeof Router>> = [];

beforeEach(() => {
  responseStats = { ...transactionStatsFixtures.base };
  requests.length = 0;
});

afterEach(async() => {
  for (const client of clients.splice(0)) {
    await client.cancelQueries();
    client.clear();
  }
  for (const router of routers.splice(0)) {
    window.removeEventListener('popstate', router.onPopState);
  }
  expect(requests).toContain('/api/v2/transactions/stats');
  expect(requests).toContain('/api/v2/stats');
  expect(requests.filter((path) => ![ '/api/v2/transactions/stats', '/api/v2/stats' ].includes(path))).toEqual([]);
});

afterAll(async() => {
  window.__envs = previousEnvs;
  window.__NEXT_DATA__ = previousNextData;
  window.__DEV_MIDDLEWARE_MATCHERS = previousMatchers;
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

function StatisticsPage() {
  return <TxsStats/>;
}

async function renderStatistics() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  clients.push(client);
  const router = new Router('/txs', {}, '/txs', {
    initialProps: {},
    pageLoader: new PageLoader('transaction-statistics', ''),
    App,
    Component: StatisticsPage,
    subscription: async() => {},
    wrapApp: (component) => component,
    isFallback: false,
  });
  routers.push(router);
  const view = render(
    <RouterContext.Provider value={ router }>
      <QueryClientProvider client={ client }><StatisticsPage/></QueryClientProvider>
    </RouterContext.Provider>,
  );

  await waitFor(() => {
    expect(client.getQueryState([ 'general:txs_stats' ])?.status).toBe('success');
    expect(client.getQueryState([ 'general:stats' ])?.status).toBe('success');
    expect(client.getQueryData([ 'general:txs_stats' ])).toEqual(responseStats);
    expect(view.container.querySelector('[data-scan-stat="Total transactions"] [data-value]')?.getAttribute('data-loading'))
      .not.toBe('true');
    expect(valueOf(view.container, 'Transactions (24h)')).toBe(Number(responseStats.transactions_count_24h).toLocaleString());
  });
  return view;
}

function cardOf(container: HTMLElement, label: string) {
  return container.querySelector(`[data-scan-stat="${ label }"]`);
}

function valueOf(container: HTMLElement, label: string) {
  return cardOf(container, label)?.querySelector('[data-value]')?.textContent;
}

async function transactionStatisticsShowPriceUpdateTotalsAndInclusiveDayCount() {
  const { container } = await renderStatistics();

  expect(valueOf(container, 'Transactions (24h)')).toBe('992,890');
  expect(valueOf(container, 'Total transactions')).toBe('100,000,000');
  expect(valueOf(container, 'Price updates (24h)')).toBe('250,000');
  expect(valueOf(container, 'Total price updates')).toBe('20,000,000');
  expect(cardOf(container, 'Transactions (24h)')?.querySelector('[data-delta]')?.textContent).toBe('(0.99%)');
  expect(valueOf(container, 'Pending transactions (last 1h)')).toBe('4,200');
  expect(valueOf(container, 'Total transaction fee (24h)')).toContain('22.18');
  expect(valueOf(container, 'Avg. transaction fee (24h)')).toMatch(/^\$/);
}

async function transactionStatisticsShowDashesForOmittedTotals() {
  delete responseStats.total_transactions_count;
  delete responseStats.price_updates_count_24h;
  delete responseStats.total_price_updates_count;
  const { container } = await renderStatistics();

  expect(valueOf(container, 'Total transactions')).toBe('-');
  expect(valueOf(container, 'Price updates (24h)')).toBe('-');
  expect(valueOf(container, 'Total price updates')).toBe('-');
  expect(valueOf(container, 'Transactions (24h)')).toBe('992,890');
  expect(cardOf(container, 'Transactions (24h)')?.querySelector('[data-delta]')?.textContent).toBe('(1.21%)');
}

async function transactionStatisticsKeepPresentFieldsWhenOneIsOmitted() {
  delete responseStats.price_updates_count_24h;
  const { container } = await renderStatistics();

  expect(valueOf(container, 'Total transactions')).toBe('100,000,000');
  expect(valueOf(container, 'Price updates (24h)')).toBe('-');
  expect(valueOf(container, 'Total price updates')).toBe('20,000,000');
}

async function transactionStatisticsShowZeroCountsWithoutUsingHomepageTotal() {
  responseStats = {
    ...responseStats,
    transactions_count_24h: '0',
    total_transactions_count: '0',
    price_updates_count_24h: '0',
    total_price_updates_count: '0',
  };
  const { container } = await renderStatistics();

  expect(valueOf(container, 'Transactions (24h)')).toBe('0');
  expect(valueOf(container, 'Total transactions')).toBe('0');
  expect(valueOf(container, 'Price updates (24h)')).toBe('0');
  expect(valueOf(container, 'Total price updates')).toBe('0');
  expect(cardOf(container, 'Transactions (24h)')?.querySelector('[data-delta]')).toBeNull();
}

async function transactionStatisticsPreserveLargeStringTotals() {
  responseStats = {
    ...responseStats,
    total_transactions_count: '9007199254740993',
    total_price_updates_count: '9007199254740991',
  };
  const { container } = await renderStatistics();

  expect(valueOf(container, 'Total transactions')).toBe('9,007,199,254,740,993');
  expect(valueOf(container, 'Total price updates')).toBe('9,007,199,254,740,991');
  expect(valueOf(container, 'Price updates (24h)')).toBe('250,000');
}

test('transactionStatisticsShowPriceUpdateTotalsAndInclusiveDayCount', transactionStatisticsShowPriceUpdateTotalsAndInclusiveDayCount);
test('transactionStatisticsShowDashesForOmittedTotals', transactionStatisticsShowDashesForOmittedTotals);
test('transactionStatisticsKeepPresentFieldsWhenOneIsOmitted', transactionStatisticsKeepPresentFieldsWhenOneIsOmitted);
test('transactionStatisticsShowZeroCountsWithoutUsingHomepageTotal', transactionStatisticsShowZeroCountsWithoutUsingHomepageTotal);
test('transactionStatisticsPreserveLargeStringTotals', transactionStatisticsPreserveLargeStringTotals);

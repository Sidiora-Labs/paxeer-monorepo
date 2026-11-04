// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AppProps } from 'next/dist/shared/lib/router/router';
import { createServer } from 'node:http';
import React from 'react';

import type { PaginationParams } from 'ui/shared/pagination/types';

import * as statsFixtures from 'mocks/stats/index';
import * as transactionFixtures from 'mocks/txs/tx';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

const minedTotal = '82258122';
const pendingRows = [
  transactionFixtures.pending,
  { ...transactionFixtures.pending, hash: transactionFixtures.base2.hash },
  { ...transactionFixtures.pending, hash: transactionFixtures.base3.hash },
];
let visiblePendingRows = pendingRows;
const requests: Array<string> = [];
const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost');
  requests.push(url.pathname + url.search);
  response.setHeader('content-type', 'application/json');

  if (url.pathname === '/api/v2/stats') {
    response.end(JSON.stringify({ ...statsFixtures.base, total_transactions: minedTotal }));
  } else if (url.pathname === '/api/v2/transactions' && url.searchParams.get('filter') === 'pending') {
    response.end(JSON.stringify({ items: visiblePendingRows, next_page_params: null }));
  } else if (url.pathname === '/api/v2/transactions' && url.searchParams.get('filter') === 'validated') {
    response.end(JSON.stringify({ items: [ transactionFixtures.base, transactionFixtures.base2 ], next_page_params: null }));
  } else {
    response.statusCode = 404;
    response.end(JSON.stringify({ error: 'unexpected_fixture_request' }));
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
  NEXT_PUBLIC_APP_ENV: 'testing',
  NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String(address.port),
  NEXT_PUBLIC_API_BASE_PATH: '',
};

const { 'default': useApiQuery } = await import('lib/api/useApiQuery');
const { render } = await import('ui/shared/layout/testWrapper');
const { waitFor } = await import('@testing-library/react');
const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': TxsContent } = await import('./TxsContent');

const initialNextData = window.__NEXT_DATA__;
const initialMatchers = window.__DEV_MIDDLEWARE_MATCHERS;
window.__NEXT_DATA__ = { props: {}, page: '/txs', query: {}, buildId: 'repair-22-33-532' };
window.__DEV_MIDDLEWARE_MATCHERS = [];
const App = ({ Component, pageProps }: AppProps) => React.createElement(Component, pageProps);

const clients: Array<QueryClient> = [];
const routers: Array<InstanceType<typeof Router>> = [];

beforeEach(() => {
  visiblePendingRows = pendingRows;
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
  expect(requests.filter((target) => ![
    '/api/v2/stats', '/api/v2/transactions?filter=pending', '/api/v2/transactions?filter=validated',
  ].includes(target))).toEqual([]);
});

afterAll(async() => {
  window.__NEXT_DATA__ = initialNextData;
  window.__DEV_MIDDLEWARE_MATCHERS = initialMatchers;
  window.__envs = previousEnvs;
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

function renderTransactionPage(filter: 'pending' | 'validated', overrides: Partial<PaginationParams> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  clients.push(client);
  const pagination: PaginationParams = {
    page: 1,
    onNextPageClick: () => {},
    onPrevPageClick: () => {},
    resetPage: () => {},
    hasPages: false,
    hasNextPage: false,
    canGoBackwards: false,
    isLoading: false,
    isVisible: false,
    ...overrides,
  };

  function TransactionPage() {
    const query = useApiQuery(filter === 'pending' ? 'general:txs_pending' : 'general:txs_validated', {
      queryParams: { filter },
    });
    return React.createElement(TxsContent, {
      pagination,
      items: query.data?.items,
      socketType: filter === 'pending' ? 'txs_pending' : 'txs_validated',
      showBlockInfo: filter === 'validated',
      isPlaceholderData: query.isPending,
      isError: query.isError,
      sort: 'default',
      stickyHeader: false,
    });
  }

  const router = new Router('/txs', {}, '/txs', {
    initialProps: {},
    pageLoader: new PageLoader('repair-22-33-532', ''),
    App,
    Component: TransactionPage,
    subscription: async() => {},
    wrapApp: (component) => component,
    isFallback: false,
  });
  routers.push(router);

  const view = render(React.createElement(RouterContext.Provider, { value: router },
    React.createElement(QueryClientProvider, { client }, React.createElement(TransactionPage))));
  return { ...view, client };
}

function transactionLinks(container: HTMLElement) {
  return Array.from(container.querySelectorAll('tbody tr a[href^="/tx/"]')).map((link) => link.getAttribute('href'));
}

async function pendingTitleCountsCurrentPageInsteadOfMinedHistory() {
  const { container, client } = renderTransactionPage('pending', { hasPages: true, hasNextPage: true });
  await waitFor(() => {
    expect(client.getQueryData([ 'general:stats' ])).toEqual({ ...statsFixtures.base, total_transactions: minedTotal });
    expect(container.querySelector('[data-scan-table-card] [data-title]')?.textContent)
      .toBe('Pending transactions on this page: 3');
  });
  expect(requests).toContain('/api/v2/transactions?filter=pending');
  expect(requests).toContain('/api/v2/stats');
  expect(transactionLinks(container)).toEqual(pendingRows.map((row) => `/tx/${ row.hash }`));
  expect(container.querySelector('[data-note]')?.textContent).toBe('Transactions waiting to be included in a block');
}

async function pendingTitleUsesShortLaterPageWithoutEstimatingATotal() {
  visiblePendingRows = pendingRows.slice(0, 1);
  const { container, client } = renderTransactionPage('pending', { page: 2, hasPages: true, canGoBackwards: true });
  await waitFor(() => {
    expect(client.getQueryState([ 'general:stats' ])?.status).toBe('success');
    expect(container.querySelector('[data-scan-table-card] [data-title]')?.textContent)
      .toBe('Pending transactions on this page: 1');
  });
  expect(transactionLinks(container)).toEqual([ `/tx/${ pendingRows[0].hash }` ]);
}

async function validatedTitleRetainsMatchingMinedTotal() {
  const { container } = renderTransactionPage('validated');
  await waitFor(() => {
    expect(container.querySelector('[data-scan-table-card] [data-title]')?.textContent)
      .toBe('Latest 2 from a total of 82,258,122 transactions');
  });
  expect(requests).toContain('/api/v2/transactions?filter=validated');
  expect(transactionLinks(container)).toEqual([ `/tx/${ transactionFixtures.base.hash }`, `/tx/${ transactionFixtures.base2.hash }` ]);
}

async function emptyPendingPageDoesNotAdvertiseMinedHistory() {
  visiblePendingRows = [];
  const { container, client } = renderTransactionPage('pending');
  await waitFor(() => {
    expect(client.getQueryState([ 'general:stats' ])?.status).toBe('success');
    expect(client.getQueryState([ 'general:txs_pending', { filter: 'pending' } ])?.status).toBe('success');
    expect(container.textContent).toContain('There are no transactions.');
  });
  expect(requests).toContain('/api/v2/transactions?filter=pending');
  expect(container.querySelector('[data-scan-table-card]')).toBeNull();
}

test('pendingTitleCountsCurrentPageInsteadOfMinedHistory', pendingTitleCountsCurrentPageInsteadOfMinedHistory);
test('pendingTitleUsesShortLaterPageWithoutEstimatingATotal', pendingTitleUsesShortLaterPageWithoutEstimatingATotal);
test('validatedTitleRetainsMatchingMinedTotal', validatedTitleRetainsMatchingMinedTotal);
test('emptyPendingPageDoesNotAdvertiseMinedHistory', emptyPendingPageDoesNotAdvertiseMinedHistory);

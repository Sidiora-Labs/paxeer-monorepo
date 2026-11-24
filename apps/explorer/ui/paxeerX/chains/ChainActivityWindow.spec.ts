// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, waitFor, within } from '@testing-library/react';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import { getRouteMatcher } from 'next/dist/shared/lib/router/utils/route-matcher';
import { getRouteRegex } from 'next/dist/shared/lib/router/utils/route-regex';
import { once } from 'node:events';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { ClusterChainConfig } from 'types/multichain';

import type { MultichainContext as ChainContext } from 'lib/contexts/multichain';
import type ChainDetailPage from 'pages/chains/[id]';
import type { Provider as ChakraProvider } from 'toolkit/chakra/provider';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import type ChainDetail from './ChainDetail';
import type ChainsList from './ChainsList';
import type { ChainEvent, ChainTransfer } from './detail';
import type { ChainRecord } from './model';

const records: Array<ChainRecord> = [ 7, 19 ].map((id): ChainRecord => ({
  id,
  name: `tenant-${ id }`,
  operator: '0x1111111111111111111111111111111111111111',
  status: 'active',
  chain_id: 125000000 + id,
  created_height: 42,
  bond: '100000000000000000000',
  gas_allowance: 500000,
  byte_allowance: 65536,
  fee_share_bps: 100,
}));

// An in-memory activity store is the HTTP peer; production hooks, codecs and UI consume it.
// Row IDs cross Number.MAX_SAFE_INTEGER and heights deliberately do not define row order.
const transfers: Array<ChainTransfer> = records.flatMap((record) => Array.from({ length: 45 }, (_, index): ChainTransfer => ({
  id: (BigInt('9007199254740990') + BigInt(index)).toString(),
  chain: `pax:${ record.id }`,
  height_or_seq: String(100 - index),
  tx_id: `transfer-${ record.id }-${ index }`,
  ordinal: '0',
  decoded: {},
  kind: 'native',
  direction: 'out',
  account: record.operator,
  counterparty: null,
  asset: 'PAX',
  amount: '1',
  tx_index: '0',
  log_index: '0',
  'final': false,
  final_basis: 'block_commit',
  stability: { level: 'reversible', source: 'block_commit', finalized_boundary: null },
  settlement: { level: 'unverified', source: null, reason: 'no_verified_checkpoint_evidence' },
})));
const events: Array<ChainEvent> = transfers.map((transfer): ChainEvent => ({
  id: transfer.id,
  chain: transfer.chain,
  height_or_seq: transfer.height_or_seq,
  tx_id: transfer.tx_id.replace('transfer', 'event'),
  ordinal: transfer.ordinal,
  decoded: {},
  source: 'chain-transaction',
  name: 'transaction',
  contract: null,
  account: transfer.account,
}));

type ResponseMode = 'descending' | 'ascending' | 'oversized' | 'unsupported' | 'wrong-tenant' | 'empty';
let responseMode: ResponseMode = 'descending';
const requests: Array<URL> = [];

function activityPage<T extends ChainTransfer | ChainEvent>(source: Array<T>, tenant: number, url: URL) {
  let rows = source.filter((item) => item.chain === `pax:${ tenant }`);
  const descending = url.searchParams.get('order') === 'desc' && responseMode !== 'ascending';
  if (descending) {
    rows = rows.toReversed();
  }
  const cursor = url.searchParams.get('cursor');
  if (cursor) {
    rows = rows.filter((row) => descending ? BigInt(row.id) < BigInt(cursor) : BigInt(row.id) > BigInt(cursor));
  }
  const limit = responseMode === 'oversized' ? 21 : Number(url.searchParams.get('limit') || 50);
  const items = responseMode === 'empty' ? [] : rows.slice(0, limit);
  return {
    version: 1,
    items,
    next_cursor: rows.length > limit && items.length ? items[items.length - 1].id : null,
  };
}

const server = createServer((request, response) => {
  const url = new URL(request.url || '/', 'http://localhost');
  requests.push(url);
  response.setHeader('content-type', 'application/json');
  const send = (status: number, body: unknown) => {
    response.writeHead(status);
    response.end(JSON.stringify(body));
  };
  if (url.pathname === '/v1/chains') {
    send(200, { chains: records });
    return;
  }
  const match = url.pathname.match(/^\/v1\/chains\/(\d+)(?:\/(summary|transfers|events))?$/);
  const record = records.find((item) => String(item.id) === match?.[1]);
  if (!record) {
    send(404, { error: 'unknown_chain' });
    return;
  }
  if (!match?.[2]) {
    send(200, record);
  } else if (match[2] === 'summary') {
    send(200, { id: record.id, tx_count: 45, gas_used: 945000, last_height: 100 });
  } else if (responseMode === 'unsupported') {
    // Current indexer parse_query refusal, until the platform wiring request lands.
    send(400, { error: 'unknown_query_parameter' });
  } else {
    const tenant = responseMode === 'wrong-tenant' ? 19 : record.id;
    send(200, match[2] === 'transfers' ? activityPage(transfers, tenant, url) : activityPage(events, tenant, url));
  }
});

const initialEnvs = window.__envs;
const initialNextData = window.__NEXT_DATA__;
const initialMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
let List: typeof ChainsList;
let Detail: typeof ChainDetail;
let DetailPage: typeof ChainDetailPage;
let Provider: typeof ChakraProvider;
let MultichainContext: typeof ChainContext;
let chain: ClusterChainConfig;
let client: QueryClient;
const routers: Array<Router> = [];

beforeAll(async() => {
  fetchMock.disableMocks();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  window.__envs = { ...initialEnvs, NEXT_PUBLIC_PAXEER_X_ENABLED: 'true' };
  window.__NEXT_DATA__ = { props: {}, page: '/chains', query: {}, buildId: 'chains-repair' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });
  await import('ui/paxeerX/account/testWrapper');
  ({ Provider } = await import('toolkit/chakra/provider'));
  ({ MultichainContext } = await import('lib/contexts/multichain'));
  const { 'default': config } = await import('configs/app');
  chain = {
    id: '125', name: 'Root network', slug: 'root',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        paxeerXIndexer: { endpoint: `http://127.0.0.1:${ (server.address() as AddressInfo).port }` },
      },
    },
  };
  ({ 'default': List } = await import('./ChainsList'));
  ({ 'default': Detail } = await import('./ChainDetail'));
  ({ 'default': DetailPage } = await import('pages/chains/[id]'));
});

afterEach(() => {
  cleanup();
  client?.clear();
  for (const router of routers.splice(0)) {
    window.removeEventListener('popstate', router.onPopState);
  }
  requests.length = 0;
  responseMode = 'descending';
});

afterAll(async() => {
  window.__envs = initialEnvs;
  window.__NEXT_DATA__ = initialNextData;
  if (initialMatchers) {
    Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', initialMatchers);
  } else {
    Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
  }
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

async function openView(component: React.ReactElement, href = '/chains') {
  const route = '/chains/[id]';
  const params = getRouteMatcher(getRouteRegex(route))(href);
  const router = new Router(params ? route : '/chains', params || {}, href, {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('chains-repair', ''),
    App,
    Component: List,
    wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => React.createElement(WrappedApp, props),
    subscription: async(data, CurrentApp) => {
      renderToStaticMarkup(React.createElement(CurrentApp, { ...data.props, Component: data.Component, router }));
    },
    isFallback: false,
  });
  routers.push(router);
  await router._initialMatchesMiddlewarePromise;
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(React.createElement(Provider, {},
    React.createElement(QueryClientProvider, { client },
      React.createElement(MultichainContext.Provider, { value: { chain } },
        React.createElement(RouterContext.Provider, { value: router }, component)))));
}

function activityRequests() {
  return requests.filter((url) => /\/(?:transfers|events)$/.test(url.pathname));
}

async function chainsFullHistoryPaginationUsesOneLatestWindow() {
  const view = await openView(React.createElement(Detail, { id: 7 }));
  for (const kind of [ 'transfers', 'events' ]) {
    const table = await view.findByRole('table', { name: kind });
    const rows = within(table).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(20);
    expect(rows.map((row) => row.textContent?.match(new RegExp(`${ kind === 'transfers' ? 'transfer' : 'event' }-7-\\d+`))?.[0]))
      .toEqual(Array.from({ length: 20 }, (_, index) => `${ kind === 'transfers' ? 'transfer' : 'event' }-7-${ 44 - index }`));
  }
  expect(activityRequests()).toHaveLength(2);
  for (const url of activityRequests()) {
    expect(Object.fromEntries(url.searchParams)).toEqual({ limit: '20', order: 'desc' });
    expect(url.pathname).toMatch(/^\/v1\/chains\/7\/(transfers|events)$/);
  }
}

async function chainListDetailLinkKeepsSelectedTenantInEveryQuery() {
  for (const record of records) {
    const list = await openView(React.createElement(List));
    const link = await list.findByRole('link', { name: record.name });
    const href = link.getAttribute('href');
    expect(href).toBe(`/chains/${ record.id }`);
    if (!href) {
      throw new Error('Chain detail link is missing its route');
    }
    list.unmount();
    client.clear();
    requests.length = 0;
    const detail = await openView(React.createElement(DetailPage), href);
    await detail.findByRole('heading', { name: record.name });
    await detail.findByRole('table', { name: 'transfers' });
    await detail.findByRole('table', { name: 'events' });
    await waitFor(() => expect(requests.some((url) => url.pathname.endsWith('/summary'))).toBe(true));
    expect(requests.map((url) => url.pathname).sort()).toEqual([
      `/v1/chains/${ record.id }`, `/v1/chains/${ record.id }/events`,
      `/v1/chains/${ record.id }/summary`, `/v1/chains/${ record.id }/transfers`,
    ]);
    detail.unmount();
    client.clear();
    requests.length = 0;
  }
}

async function recentActivityRejectsBrokenOrderingAndUnsupportedProducer() {
  for (const mode of [ 'ascending', 'oversized', 'unsupported', 'wrong-tenant' ] as const) {
    responseMode = mode;
    const view = await openView(React.createElement(Detail, { id: 7 }));
    await waitFor(() => expect(view.getAllByRole('alert')).toHaveLength(2));
    expect(view.queryByRole('table', { name: 'transfers' })).toBeNull();
    expect(view.queryByRole('table', { name: 'events' })).toBeNull();
    expect(activityRequests()).toHaveLength(2);
    for (const url of activityRequests()) {
      expect(Object.fromEntries(url.searchParams)).toEqual({ limit: '20', order: 'desc' });
    }
    view.unmount();
    client.clear();
    requests.length = 0;
  }
}

async function recentActivityRendersEmptyBoundedWindows() {
  responseMode = 'empty';
  const view = await openView(React.createElement(Detail, { id: 7 }));
  await view.findByText('No transfers yet.');
  await view.findByText('No events yet.');
  expect(activityRequests()).toHaveLength(2);
}

test(chainsFullHistoryPaginationUsesOneLatestWindow);
test(chainListDetailLinkKeepsSelectedTenantInEveryQuery);
test(recentActivityRejectsBrokenOrderingAndUnsupportedProducer);
test(recentActivityRendersEmptyBoundedWindows);

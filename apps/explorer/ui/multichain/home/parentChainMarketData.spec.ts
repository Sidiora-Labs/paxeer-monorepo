// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import { getFeaturePayload } from 'configs/app/features/types';
import type { ChartMarketResponse } from 'types/api/charts';
import type { HomeStats } from 'types/api/stats';
import type { ClusterChainConfig } from 'types/multichain';

import { mainPageStats } from 'mocks/multichain/stats';
import { base as nativeStats } from 'mocks/stats';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

fetchMock.disableMocks();

// jsdom has no SVG geometry API. These histories produce M/L/Z paths, whose
// length can be measured exactly without replacing the chart or its animation.
function svgPolylineLength(this: SVGElement) {
  const path = this.getAttribute('d') ?? '';
  const tokenPattern = /[MLZ]|[-+]?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?/g;
  if (path.replace(tokenPattern, '').replace(/[\s,]/g, '') !== '') {
    throw new Error(`Unsupported SVG path geometry: ${ path }`);
  }
  const tokens = path.match(tokenPattern) ?? [];
  let point = [ 0, 0 ];
  let start = [ 0, 0 ];
  let length = 0;
  for (let index = 0; index < tokens.length;) {
    const command = tokens[index++];
    if (command === 'Z') {
      length += Math.hypot(point[0] - start[0], point[1] - start[1]);
      point = start;
      continue;
    }
    if (command !== 'M' && command !== 'L') {
      throw new Error(`Unsupported SVG path command: ${ command }`);
    }
    const next = [ Number(tokens[index++]), Number(tokens[index++]) ];
    if (!next.every(Number.isFinite)) {
      throw new Error(`Invalid SVG path coordinates: ${ path }`);
    }
    if (command === 'M') {
      start = next;
    } else {
      length += Math.hypot(next[0] - point[0], next[1] - point[1]);
    }
    point = next;
  }
  return length;
}

// jsdom represents <path> with SVGElement where SVGPathElement is unavailable.
const svgPathPrototype = typeof SVGPathElement === 'undefined' ? SVGElement.prototype : SVGPathElement.prototype;
Object.defineProperties(svgPathPrototype, {
  getTotalLength: { configurable: true, value: svgPolylineLength },
  getPointAtLength: { configurable: true, value: () => ({ x: 0, y: 0 }) },
});

let samples: Array<string> = [];
let marketStats: HomeStats = nativeStats;
let marketStatus = 200;
const requests: Array<string> = [];
const marketChart: ChartMarketResponse = {
  available_supply: '100',
  chart_data: [
    { date: '2026-10-01', closing_price: '2', market_cap: '200' },
    { date: '2026-10-02', closing_price: '3', market_cap: '300' },
  ],
};

const server = createServer((request, response) => {
  const path = new URL(request.url!, 'http://localhost').pathname;
  requests.push(path);
  response.setHeader('Content-Type', 'application/json');
  if (path === '/api/v1/pages/multichain/main') {
    response.end(JSON.stringify({
      ...mainPageStats,
      yesterday_txns_multichain: { ...mainPageStats.yesterday_txns_multichain, value: samples.at(-1) ?? '0' },
      new_txns_multichain_window: {
        ...mainPageStats.new_txns_multichain_window,
        chart: samples.map((value, index) => ({ date: `2026-10-0${ index + 1 }`, value })),
      },
    }));
    return;
  }
  if (/^\/tenant-[ab]\/api\/v2\/stats$/.test(path)) {
    response.statusCode = marketStatus;
    response.end(JSON.stringify(marketStatus === 200 ? {
      ...marketStats,
      coin_image: `http://127.0.0.1:${ (server.address() as AddressInfo).port }${ path }/logo.png`,
      ...(path.startsWith('/tenant-b/') && { coin_price: '42', market_cap: '4200' }),
    } : { error: 'Market data unavailable' }));
    return;
  }
  if (/^\/tenant-[ab]\/api\/v2\/stats\/charts\/market$/.test(path)) {
    response.end(JSON.stringify(marketChart));
    return;
  }
  response.statusCode = 404;
  response.end(JSON.stringify({ error: `Unexpected fixture request: ${ path }` }));
});
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
afterAll(async() => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

const port = String((server.address() as AddressInfo).port);
const origin = `http://127.0.0.1:${ port }`;
window.__envs = {
  ...window.__envs,
  NEXT_PUBLIC_APP_INSTANCE: 'vitest',
  NEXT_PUBLIC_APP_ENV: 'testing',
  NEXT_PUBLIC_APP_PROTOCOL: 'http',
  NEXT_PUBLIC_APP_HOST: '127.0.0.1',
  NEXT_PUBLIC_APP_PORT: port,
  NEXT_PUBLIC_NETWORK_ID: '125',
  NEXT_PUBLIC_NETWORK_NAME: 'Paxeer',
  NEXT_PUBLIC_NETWORK_CURRENCY_NAME: 'Paxeer',
  NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL: 'PAX',
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: port,
  NEXT_PUBLIC_API_BASE_PATH: '/tenant-a',
  NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  NEXT_PUBLIC_MULTICHAIN_ENABLED: 'true',
  NEXT_PUBLIC_MULTICHAIN_CLUSTER: 'tenant-a',
  NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST: origin,
  NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST: origin,
  NEXT_PUBLIC_HOMEPAGE_CHARTS: '["daily_txs","coin_price","market_cap"]',
  NEXT_PUBLIC_ROLLBAR_CLIENT_TOKEN: '',
};

const { 'default': config } = await import('configs/app');
await import('ui/shared/layout/testWrapper');
const { Provider } = await import('toolkit/chakra/provider');
const { MultichainContext } = await import('lib/contexts/multichain');
const { 'default': ChainIndicators } = await import('./ChainIndicators');
const { 'default': useChartDataQuery } = await import('./useChartDataQuery');

const tenantA: ClusterChainConfig = { id: '125', name: 'Paxeer', slug: 'paxeer', app_config: config };
const tenantB: ClusterChainConfig = {
  id: '126', name: 'Tenant B', slug: 'tenant-b',
  app_config: {
    ...config,
    app: { ...config.app, baseUrl: `${ origin }/tenant-b/explorer` },
    chain: { ...config.chain, id: '126', currency: { ...config.chain.currency, name: 'Tenant B coin', symbol: 'TEN' } },
    apis: { ...config.apis, general: { ...config.apis.general!, basePath: '/tenant-b' } },
    features: { ...config.features, multichain: { title: 'Multichain explorer', isEnabled: true, cluster: 'tenant-b' } },
  },
};

const clients: Array<QueryClient> = [];
function queryClient() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: 60_000 } } });
  clients.push(client);
  return client;
}

function tree(chain: ClusterChainConfig, child: React.ReactElement, client: QueryClient) {
  return React.createElement(QueryClientProvider, { client },
    React.createElement(Provider, { enableSystem: false, defaultTheme: 'light' },
      React.createElement(MultichainContext.Provider, { value: { chain } }, child),
    ),
  );
}

function mount(chain = tenantA, client = queryClient()) {
  return { ...render(tree(chain, React.createElement(ChainIndicators), client)), client };
}

beforeEach(() => {
  samples = [];
  marketStats = nativeStats;
  marketStatus = 200;
  requests.length = 0;
});
afterEach(async() => {
  cleanup();
  await Promise.all(clients.map((client) => client.cancelQueries()));
  clients.splice(0).forEach((client) => client.clear());
  expect(requests.filter((path) => ![
    '/api/v1/pages/multichain/main',
    '/tenant-a/api/v2/stats', '/tenant-b/api/v2/stats',
    '/tenant-a/api/v2/stats/charts/market', '/tenant-b/api/v2/stats/charts/market',
  ].includes(path))).toEqual([]);
});

async function assertHistory(values: Array<string>, percentage?: number) {
  samples = values;
  const { container, unmount } = mount();
  await expect.poll(() => container.querySelector('[data-title]')?.textContent).toBe('New transactions');
  const chart = container.querySelector('[data-label="chain-indicator-chart"]')!;
  expect(chart.getAttribute('data-points')).toBe(String(values.length));
  const formatted = Number(values.at(-1) ?? 0).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' });
  expect(chart.textContent).toContain(formatted);
  expect(chart.querySelector('p')?.textContent).toBe(formatted);
  expect(chart.textContent).not.toContain('N/A');
  if (percentage === undefined) {
    expect(chart.textContent).not.toContain('%');
  } else {
    expect(chart.textContent).toContain(`${ percentage }%`);
  }
  expect(requests).toContain('/api/v1/pages/multichain/main');
  unmount();
}

test('multichainShortChartEmptyHistory', async function multichainShortChartEmptyHistory() {
  expect.hasAssertions();
  await assertHistory([]);
});

test('multichainChartGeometryMeasuresProducerPaths', async function multichainChartGeometryMeasuresProducerPaths() {
  const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  path.setAttribute('d', 'M0,0L3,4');
  expect(path.getTotalLength()).toBe(5);
  path.setAttribute('d', 'M0,0L3,4Z');
  expect(path.getTotalLength()).toBe(10);
  path.setAttribute('d', 'M-3,-4L0,0');
  expect(path.getTotalLength()).toBe(5);
  path.setAttribute('d', 'M0,0Z');
  expect(path.getTotalLength()).toBe(0);
  path.setAttribute('d', 'M0,0C1,2,3,4,5,6');
  expect(() => path.getTotalLength()).toThrow('Unsupported SVG path geometry');
});

test('multichainShortChartSingleZeroSample', async function multichainShortChartSingleZeroSample() {
  expect.hasAssertions();
  await assertHistory([ '0' ]);
});

test('multichainShortChartTwoSamples', async function multichainShortChartTwoSamples() {
  expect.hasAssertions();
  await assertHistory([ '5', '0' ], -100);
  await assertHistory([ '0', '5' ]);
  await assertHistory([ '0', '0' ]);
  await assertHistory([ '4', '6' ], 50);
});

test('multichainMarketLabelMatchesNativeFeed', async function multichainMarketLabelMatchesNativeFeed() {
  const { container, getByText } = mount();
  await expect.poll(() => container.textContent).toContain('PAX price');
  expect(container.textContent).toContain('$0.001997');
  expect(container.textContent).toContain('PAX market cap');
  expect(container.textContent).not.toContain('ETH');
  expect(container.querySelector('img[alt="PAX token logo"]')?.getAttribute('src')).toBe(`${ origin }/tenant-a/api/v2/stats/logo.png`);
  fireEvent.click(getByText('PAX price'));
  await expect.poll(() => container.querySelector('[data-title]')?.textContent).toBe('PAX price');
  await expect.poll(() => container.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points')).toBe('2');
  expect(requests).toContain('/tenant-a/api/v2/stats');
  expect(requests).toContain('/tenant-a/api/v2/stats/charts/market');
});

function MarketChart() {
  const chart = useChartDataQuery({ indicatorId: 'coin_price' });
  return React.createElement('output', {
    'data-points': chart.data[0]?.items.length,
    'data-values': JSON.stringify(chart.data[0]?.items.map((item) => item.value)),
  }, JSON.stringify(chart));
}

test('multichainParentFeedSeparatesTenantCaches', async function multichainParentFeedSeparatesTenantCaches() {
  const client = queryClient();
  const first = mount(tenantA, client);
  await expect.poll(() => first.container.textContent).toContain('PAX price');
  fireEvent.click(first.getByText('PAX price'));
  await expect.poll(() => first.container.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points')).toBe('2');
  first.rerender(tree(tenantB, React.createElement(ChainIndicators), client));
  await expect.poll(() => first.container.textContent).toContain('TEN price');
  await expect.poll(() => first.container.textContent).toContain('$42.00');
  expect(first.container.textContent).not.toContain('PAX');
  expect(first.container.querySelector('img[alt="TEN token logo"]')?.getAttribute('src')).toBe(`${ origin }/tenant-b/api/v2/stats/logo.png`);
  await expect.poll(() => requests.includes('/tenant-b/api/v2/stats/charts/market')).toBe(true);
  const marketQueries = client.getQueryCache().getAll().filter((query) => query.queryKey[0] === 'parent_chain');
  expect(marketQueries).toHaveLength(4);
  for (const chain of [ tenantA, tenantB ]) {
    const keys = marketQueries.filter((query) => JSON.stringify(query.queryKey).includes(`"chainId":"${ chain.id }"`));
    expect(keys).toHaveLength(2);
    for (const query of keys) {
      expect(query.queryKey).toContainEqual(expect.objectContaining({
        symbol: chain.app_config.chain.currency.symbol,
        tenant: chain.app_config.app.baseUrl,
        basePath: chain.app_config.apis.general!.basePath,
        cluster: getFeaturePayload(chain.app_config.features.multichain)?.cluster,
      }));
    }
  }
  first.unmount();
  const chart = render(tree(tenantB, React.createElement(MarketChart), client));
  await expect.poll(() => chart.container.textContent).toContain('TEN price');
  await expect.poll(() => chart.container.querySelector('output')?.getAttribute('data-points')).toBe('2');
  expect(chart.container.querySelector('output')?.getAttribute('data-values')).toBe(JSON.stringify([ 2, 3 ]));
});

test('multichainParentFeedOmitsUnavailableStatistics', async function multichainParentFeedOmitsUnavailableStatistics() {
  marketStats = { ...nativeStats, coin_price: null, market_cap: null };
  const result = mount();
  await expect.poll(() => result.container.textContent).toContain('PAX market statistics unavailable.');
  expect(result.container.textContent).not.toContain('PAX price');
  expect(result.container.textContent).not.toContain('PAX market cap');
  expect(result.container.querySelectorAll('[data-loading]')).toHaveLength(0);
  await expect.poll(() => result.container.querySelector('[data-title]')?.textContent).toBe('New transactions');
  result.unmount();
  marketStats = { ...nativeStats, coin_price: '0', market_cap: null };
  const partial = mount();
  await expect.poll(() => partial.container.textContent).toContain('PAX price');
  expect(partial.container.textContent).toContain('$0.00');
  expect(partial.container.textContent).not.toContain('PAX market cap');
  expect(partial.getByRole('status').textContent).toBe('PAX market statistics unavailable.');
});

test('multichainParentFeedReportsFailedFeed', async function multichainParentFeedReportsFailedFeed() {
  marketStatus = 503;
  const result = mount();
  await expect.poll(() => result.container.textContent).toContain('PAX market statistics unavailable.');
  expect(result.container.textContent).not.toContain('PAX price');
  expect(result.container.textContent).not.toContain('PAX market cap');
  await expect.poll(() => result.container.querySelector('[data-title]')?.textContent).toBe('New transactions');
});

test('multichainParentFeedRequiresConfiguredIdentity', async function multichainParentFeedRequiresConfiguredIdentity() {
  const unavailable: ClusterChainConfig = {
    ...tenantA,
    app_config: { ...config, apis: { ...config.apis, general: undefined } },
  };
  const result = mount(unavailable);
  await expect.poll(() => result.container.querySelector('[data-title]')?.textContent).toBe('New transactions');
  expect(result.getByRole('status').textContent).toBe('Native asset market statistics unavailable.');
  expect(result.client.isFetching()).toBe(0);
  expect(requests).toEqual([ '/api/v1/pages/multichain/main' ]);
});

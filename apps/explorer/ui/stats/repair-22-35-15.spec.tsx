// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, waitFor, within } from '@testing-library/react';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import { makePublicRouterInstance } from 'next/router';
import { once } from 'node:events';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { ChartMarketResponse, ChartSecondaryCoinMarketResponse, ChartTransactionResponse } from 'types/api/charts';
import type { FeedMarketsResponse } from 'types/api/paxeerXFeed';

import type { ExplorerStats, ExplorerTransactionsStats } from 'lib/api/services/general/stats';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

const originalEnvs = window.__envs;
const requests: Array<URL> = [];
const responses = new Map<string, unknown>();
const failures = new Set<string>();
let endpoint: string;
let client: QueryClient;
let router: Router;
const server = createServer((request, response) => {
  const url = new URL(request.url || '/', endpoint);
  requests.push(url);
  if (failures.has(url.pathname)) {
    response.writeHead(503, { 'Content-Type': 'application/json' }).end(JSON.stringify({ message: 'Service unavailable' }));
    return;
  }
  if (request.method !== 'GET' || !responses.has(url.pathname)) {
    response.writeHead(404).end();
    return;
  }
  response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(responses.get(url.pathname)));
});

fetchMock.disableMocks();
server.listen(0, '127.0.0.1');
await once(server, 'listening');
const port = String((server.address() as AddressInfo).port);
endpoint = `http://127.0.0.1:${ port }`;
window.__envs = {
  ...originalEnvs,
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: port,
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_BASE_PATH: '',
  NEXT_PUBLIC_STATS_API_HOST: '',
  NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST: '',
  NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST: '',
  NEXT_PUBLIC_MULTICHAIN_CLUSTER: '',
  NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL: 'PAX',
  NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS: '18',
};
Object.assign(window, {
  __NEXT_DATA__: { props: { pageProps: {} }, page: '/stats', query: {}, buildId: 'statistics' },
  __BUILD_MANIFEST: { sortedPages: [ '/stats' ], __rewrites: { beforeFiles: [], afterFiles: [], fallback: [] }, '/stats': [] },
  __DEV_PAGES_MANIFEST: { pages: [ '/stats' ] },
  __DEV_MIDDLEWARE_MATCHERS: [],
  __MIDDLEWARE_MATCHERS: [],
  __SSG_MANIFEST: new Set(),
});

await import('ui/shared/layout/testWrapper');
const { 'default': config } = await import('configs/app');
const { StatsPageContent } = await import('pages/stats');
const { 'default': useNavItems, isInternalItem } = await import('lib/hooks/useNavItems');
const { MultichainContext } = await import('lib/contexts/multichain');
const { Provider } = await import('toolkit/chakra/provider');
const { wrapper: TestApp } = await import('vitest/lib');
const { stats: statsGuard } = await import('nextjs/getServerSideProps/guards');
const { 'default': colors } = await import('toolkit/theme/foundations/colors');
const { 'default': semanticTokens } = await import('toolkit/theme/foundations/semanticTokens');
const { CHART_COLOR_TOKENS } = await import('ui/shared/chart/utils');

function dateDaysAgo(days: number): string {
  const date = new Date();
  date.setUTCDate(date.getUTCDate() - days);
  return date.toISOString().slice(0, 10);
}

const statistics: ExplorerStats = {
  total_blocks: '1234567',
  total_addresses: '7654321',
  total_transactions: '123456789',
  transactions_today: '1234',
  average_block_time: 5250,
  total_gas_used: '123000000000',
  gas_used_today: '45600000',
  network_utilization_percentage: 65.5,
  gas_prices: {
    slow: { price: 0, fiat_price: '0', time: 0, base_fee: 0, priority_fee: 0 },
    average: { price: 1.25, fiat_price: '0.1', time: 1, base_fee: 1, priority_fee: 0.25 },
    fast: { price: 2.5, fiat_price: '0.2', time: 0.5, base_fee: 1, priority_fee: 1.5 },
  },
  coin_price: '0.125',
  coin_price_change_percentage: 2,
  market_cap: '12500000',
  secondary_coin_price: '2.75',
  secondary_coin_market_cap: '550000000',
  secondary_coin_price_change_percentage: -3,
  gas_price_updated_at: null,
  gas_prices_update_in: 0,
  static_gas_price: null,
  tvl: null,
};
const transactions: ExplorerTransactionsStats = {
  transactions_count_24h: '1399',
  total_transactions_count: '123456789',
  pending_transactions_count: '17',
  transaction_fees_sum_24h: '1500000000000000000',
  transaction_fees_avg_24h: '25000000000000000',
  price_updates_count_24h: '165',
  total_price_updates_count: '750000',
};
const markets: FeedMarketsResponse = {
  items: [
    { market_id: 1, symbol: 'PAX', chainflow_symbol: 'PAX', latest: null },
    { market_id: 2, symbol: 'SID', chainflow_symbol: 'SID', latest: null },
  ],
  chain_head: 1234566,
};
const transactionChart: ChartTransactionResponse = {
  chart_data: [
    { date: dateDaysAgo(0), transactions_count: 1444 },
    { date: dateDaysAgo(1), transactions_count: 1333 },
    { date: dateDaysAgo(2), transactions_count: 1222 },
  ],
};
const paxChart: ChartMarketResponse = {
  available_supply: '100000000',
  chart_data: [
    { date: dateDaysAgo(0), closing_price: '0.35', market_cap: '35000000' },
    { date: dateDaysAgo(1), closing_price: '0.25', market_cap: '25000000' },
    { date: dateDaysAgo(2), closing_price: '0.15', market_cap: '15000000' },
  ],
};
const sidChart: ChartSecondaryCoinMarketResponse = {
  available_supply: '200000000',
  chart_data: [
    { date: dateDaysAgo(0), closing_price: '4', market_cap: '800000000' },
    { date: dateDaysAgo(1), closing_price: '3', market_cap: '600000000' },
    { date: dateDaysAgo(2), closing_price: '2', market_cap: '400000000' },
  ],
};

beforeEach(() => {
  requests.length = 0;
  failures.clear();
  responses.clear();
  responses.set('/api/v2/stats', statistics);
  responses.set('/api/v2/transactions/stats', transactions);
  responses.set('/api/v2/paxeer-x/feed/markets', markets);
  responses.set('/api/v2/stats/charts/transactions', transactionChart);
  responses.set('/api/v2/stats/charts/market', paxChart);
  responses.set('/api/v2/stats/charts/secondary-coin-market', sidChart);
  responses.set('/api/v2/main-page/blocks', []);
  responses.set('/api/v2/paxeer-x/capabilities', { price_feed: true });
});

afterEach(() => {
  cleanup();
  client?.clear();
  if (router) {
    window.removeEventListener('popstate', router.onPopState);
  }
});

afterAll(async() => {
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
});

function StatsNavigation() {
  const { shellNavItems } = useNavItems();
  return (
    <nav aria-label="Explorer navigation">
      { shellNavItems.flatMap((group) => group.subItems.flat()).filter(isInternalItem).map((item) => (
        <a key={ item.text } href={ item.nextRoute.pathname }>{ item.text }</a>
      )) }
    </nav>
  );
}

function renderStatistics(mode: 'light' | 'dark' = 'light', statsService = false) {
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false } } });
  router = new Router('/stats', {}, '/stats', {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('statistics', ''),
    App: ({ Component, pageProps }) => <Component { ...pageProps }/>,
    Component: StatsPageContent,
    wrapApp: App => App,
    isFallback: false,
    subscription: async() => undefined,
  });
  const chain = {
    id: '125', name: 'Paxeer', slug: 'paxeer',
    app_config: { ...config, apis: { ...config.apis, stats: { endpoint, basePath: '' } } },
  };
  return render(
    <Provider forcedTheme={ mode }>
      <TestApp>
        <QueryClientProvider client={ client }>
          <RouterContext.Provider value={ makePublicRouterInstance(router) }>
            <MultichainContext.Provider value={ statsService ? { chain } : null }>
              <StatsNavigation/>
              <StatsPageContent/>
            </MultichainContext.Provider>
          </RouterContext.Provider>
        </QueryClientProvider>
      </TestApp>
    </Provider>,
  );
}

function counterValue(container: HTMLElement, label: string) {
  return container.querySelector(`[data-stats-number-widgets] [data-scan-stat="${ label }"] [data-value]`)?.textContent;
}

function chartValue(container: HTMLElement, id: string) {
  return container.querySelector(`[data-chart-card="${ id }"] [data-chart-value]`)?.textContent;
}

const counterValues: Record<string, string> = {
  'Total blocks': '1.235M',
  'Total addresses': '7.654M',
  'Total transactions': '123.457M',
  'Transactions today': '1.234K',
  'Average block time': '5.25s',
  'Total gas used': '123B',
  'Gas used today': '45.6M',
  'Network utilisation': '65.5%',
  'Gas price (slow)': '0 Gwei',
  'Gas price (average)': '1.25 Gwei',
  'Gas price (fast)': '2.5 Gwei',
  'PAX price': '0.125 USD',
  'PAX market value': '12.5M USD',
  'SID price': '2.75 USD',
  'SID market value': '550M USD',
  'Transactions (24h)': '1.399K',
  'Total transactions (transaction statistics)': '123.457M',
  'Pending transactions': '17',
  'Total transaction fees (24h)': '1.5 PAX',
  'Average transaction fee (24h)': '0.025 PAX',
  'Price updates (24h)': '165',
  'Total price updates': '750K',
  'Price feed markets': '2',
};
const chartValues: Record<string, string> = {
  newTxns: '1,444', coinPrice: '0.35 USD', marketCap: '35,000,000 USD', secondaryCoinPrice: '4 USD', secondaryCoinMarketCap: '800,000,000 USD',
};

async function assertExplorerStatistics(mode: 'light' | 'dark') {
  expect(config.apis.stats).toBeUndefined();
  expect(config.features.stats.isEnabled).toBe(true);
  expect(await statsGuard(config)({} as Parameters<ReturnType<typeof statsGuard>>[0])).toBeUndefined();
  const { container } = renderStatistics(mode);
  await waitFor(() => {
    for (const [ label, value ] of Object.entries(counterValues)) {
      expect(counterValue(container, label), label).toBe(value);
    }
    for (const [ id, value ] of Object.entries(chartValues)) {
      expect(chartValue(container, id), id).toBe(value);
    }
  }, { timeout: 15_000 });
  expect(container.querySelector('[data-stats-source="explorer"]')).not.toBeNull();
  expect(container.querySelectorAll('[data-stats-number-widgets] [data-scan-stat]')).toHaveLength(23);
  expect(container.querySelectorAll('[data-chart-card]')).toHaveLength(5);
  expect(Array.from(container.querySelectorAll('[data-chart-title]')).map(node => node.textContent)).toEqual([
    'Daily transactions (1M)', 'PAX price (1M)', 'PAX market value (1M)', 'SID price (1M)', 'SID market value (1M)',
  ]);
  for (const id of Object.keys(chartValues)) {
    expect(container.querySelector(`[data-chart-card="${ id }"] [data-chart-surface] svg`), id).not.toBeNull();
  }
  expect(Array.from(container.querySelectorAll('[data-stats-section]')).map(node => node.getAttribute('data-stats-section')))
    .toEqual([ 'overview', 'transactions', 'market' ]);
  expect(Array.from(container.querySelectorAll('[data-stats-section-nav] [data-section-id]')).map(node => node.textContent))
    .toEqual([ 'Overview stats', 'Transactions', 'Market' ]);
  expect(container.querySelector('[data-stats-filters]')).not.toBeNull();
  expect(container.querySelectorAll('[data-network-stats-hero] [data-value]')).toHaveLength(4);
  for (const node of container.querySelectorAll('[data-network-stats-hero] [data-value]')) {
    expect(node.textContent).toBe('—');
  }
  const nav = within(within(container).getByRole('navigation', { name: 'Explorer navigation' }));
  expect(nav.getByRole('link', { name: 'Network stats' }).getAttribute('href')).toBe('/stats');
  expect(nav.getByRole('link', { name: 'Charts' }).getAttribute('href')).toBe('/stats');
  const paths = requests.map(url => url.pathname);
  for (const path of responses.keys()) {
    if (path !== '/api/v2/paxeer-x/capabilities') {
      expect(paths, path).toContain(path);
    }
  }
  expect(paths.some(path => path.startsWith('/api/v1/'))).toBe(false);
  expect(container.textContent).not.toContain('NaN');
}

async function explorerStatisticsRenderInLightMode() {
  await assertExplorerStatistics('light');
}

async function explorerStatisticsRenderInDarkMode() {
  await assertExplorerStatistics('dark');
}

async function absentStatisticsRenderDashesAndPreserveZero() {
  const { secondary_coin_market_cap, ...withoutSidCap } = statistics;
  const { total_price_updates_count, price_updates_count_24h, total_transactions_count, ...withoutTotals } = transactions;
  responses.set('/api/v2/stats', { ...withoutSidCap, coin_price: null, transactions_today: null, gas_prices: null });
  responses.set('/api/v2/transactions/stats', { ...withoutTotals, pending_transactions_count: '0', transaction_fees_avg_24h: '0' });
  responses.set('/api/v2/paxeer-x/feed/markets', { items: [], chain_head: 1234566 });
  responses.set('/api/v2/stats/charts/secondary-coin-market', {
    chart_data: sidChart.chart_data.map(item => ({ ...item, market_cap: null })),
  });
  responses.set('/api/v2/stats/charts/market', {
    ...paxChart,
    chart_data: paxChart.chart_data.map((item, index) => index === 0 ? { ...item, closing_price: null } : item),
  });
  const { container } = renderStatistics();
  await waitFor(() => expect(counterValue(container, 'Pending transactions')).toBe('0'));
  for (const label of [
    'SID market value', 'PAX price', 'Transactions today', 'Gas price (slow)', 'Gas price (average)', 'Gas price (fast)',
    'Total price updates', 'Price updates (24h)', 'Total transactions (transaction statistics)',
  ]) {
    expect(counterValue(container, label), label).toBe('—');
  }
  expect(counterValue(container, 'Average transaction fee (24h)')).toBe('0 PAX');
  await waitFor(() => {
    expect(counterValue(container, 'Price feed markets')).toBe('0');
    expect(chartValue(container, 'secondaryCoinMarketCap')).toBe('—');
    expect(chartValue(container, 'coinPrice')).toBe('—');
    expect(container.querySelector('[data-chart-card="secondaryCoinMarketCap"] [data-chart-surface]')?.textContent).toBe('No data');
  });
  expect(container.querySelector('[data-chart-card="secondaryCoinMarketCap"] [data-chart-surface] svg')).toBeNull();
  expect(chartValue(container, 'secondaryCoinPrice')).toBe('4 USD');
  expect(container.textContent).not.toMatch(/NaN|undefined/);
}

async function failedTransactionStatisticsStayInTheirSection() {
  failures.add('/api/v2/transactions/stats');
  const { container } = renderStatistics();
  const transactionSection = container.querySelector('[data-stats-counter-source="transactions"]') as HTMLElement;
  await waitFor(() => expect(within(transactionSection).getByText('Something went wrong. Try refreshing the page or come back later.')).toBeDefined());
  await waitFor(() => expect(chartValue(container, 'newTxns')).toBe('1,444'));
  expect(counterValue(container, 'Total blocks')).toBe('1.235M');
  expect(counterValue(container, 'Price feed markets')).toBe('2');
  expect(container.querySelectorAll('[role="alert"]')).toHaveLength(1);
  expect(container.querySelector('[data-stats-counter-source="stats"] [role="alert"]')).toBeNull();
  expect(container.querySelector('[data-stats-counter-source="markets"] [role="alert"]')).toBeNull();
}

async function failedMarketChartsKeepOtherChartsAndCounters() {
  failures.add('/api/v2/stats/charts/secondary-coin-market');
  const { container } = renderStatistics();
  await waitFor(() => {
    for (const id of [ 'secondaryCoinPrice', 'secondaryCoinMarketCap' ]) {
      const card = container.querySelector(`[data-chart-card="${ id }"]`) as HTMLElement;
      expect(within(card).getByText(/The data didn.t load/)).toBeDefined();
      expect(chartValue(container, id)).toBe('—');
    }
    expect(chartValue(container, 'coinPrice')).toBe('0.35 USD');
    expect(chartValue(container, 'marketCap')).toBe('35,000,000 USD');
    expect(chartValue(container, 'newTxns')).toBe('1,444');
  });
  expect(counterValue(container, 'SID price')).toBe('2.75 USD');
  expect(container.querySelector('[data-charts-error]')).toBeNull();
  expect(container.querySelectorAll('[role="alert"]')).toHaveLength(0);
}

async function statisticsFiltersSelectSectionsNamesAndIntervals() {
  responses.set('/api/v2/stats/charts/transactions', { chart_data: [
    { date: dateDaysAgo(400), transactions_count: 9999 },
    { date: dateDaysAgo(401), transactions_count: 8888 },
  ] });
  const { container } = renderStatistics();
  await waitFor(() => expect(chartValue(container, 'coinPrice')).toBe('0.35 USD'));
  expect(chartValue(container, 'newTxns')).toBe('—');
  fireEvent.click(container.querySelector('[data-stats-filter-interval] [data-id="all"]') as HTMLElement);
  await waitFor(() => expect(chartValue(container, 'newTxns')).toBe('9,999'));
  expect(container.querySelector('[data-chart-card="newTxns"] [data-chart-title]')?.textContent).toBe('Daily transactions (All time)');
  fireEvent.change(within(container).getByPlaceholderText('Find chart, metric...'), { target: { value: 'SID' } });
  await waitFor(() => expect(Array.from(container.querySelectorAll('[data-chart-card]')).map(node => node.getAttribute('data-chart-card')))
    .toEqual([ 'secondaryCoinPrice', 'secondaryCoinMarketCap' ]));
  fireEvent.change(within(container).getByPlaceholderText('Find chart, metric...'), { target: { value: '' } });
  await waitFor(() => expect(container.querySelectorAll('[data-chart-card]')).toHaveLength(5));
  const chooser = container.querySelector('[data-stats-filter-section]') as HTMLElement;
  expect(within(chooser).getByRole('combobox').textContent).toContain('All stats');
  const nativeSelect = chooser.querySelector('select') as HTMLSelectElement;
  expect(nativeSelect).not.toBeNull();
  fireEvent.change(nativeSelect, { target: { value: 'transactions' } });
  await waitFor(() => expect(Array.from(container.querySelectorAll('[data-chart-card]')).map(node => node.getAttribute('data-chart-card'))).toEqual([ 'newTxns' ]));
  expect(within(chooser).getByRole('combobox').textContent).toContain('Transactions');
  expect(counterValue(container, 'Total price updates')).toBe('750K');
}

async function configuredStatisticsServiceKeepsItsCountersAndCharts() {
  const info = { id: 'averageGasPrice', title: 'Average gas price', description: 'Daily average gas price', units: 'Gwei', resolutions: [ 'DAY' ] };
  responses.set('/api/v1/counters', { counters: [ { id: 'totalBlocks', title: 'Total blocks', value: '9876543', description: 'Indexed blocks' } ] });
  responses.set('/api/v1/lines', { sections: [ { id: 'gas', title: 'Gas', charts: [ info ] } ] });
  responses.set('/api/v1/lines/averageGasPrice', { info, chart: [
    { date: dateDaysAgo(2), date_to: dateDaysAgo(2), value: '7.5', is_approximate: false },
    { date: dateDaysAgo(1), date_to: dateDaysAgo(1), value: '8.5', is_approximate: false },
    { date: dateDaysAgo(0), date_to: dateDaysAgo(0), value: '9.5', is_approximate: false },
  ] });
  const { container } = renderStatistics('light', true);
  await waitFor(() => {
    expect(counterValue(container, 'Total blocks')).toBe('9.877M');
    expect(chartValue(container, 'averageGasPrice')).toBe('9.5 Gwei');
  }, { timeout: 15_000 });
  expect(container.querySelector('[data-stats-source="stats"]')).not.toBeNull();
  expect(container.querySelectorAll('[data-chart-card]')).toHaveLength(1);
  expect(container.querySelector('[data-chart-view]')?.getAttribute('href')).toBe('/stats/averageGasPrice?chain_id=125');
  const paths = requests.map(url => url.pathname);
  expect(paths).toContain('/api/v1/counters');
  expect(paths).toContain('/api/v1/lines');
  const lineRequest = requests.find(url => url.pathname === '/api/v1/lines/averageGasPrice');
  expect(lineRequest?.searchParams.get('resolution')).toBe('DAY');
  expect(lineRequest?.searchParams.get('from')).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  expect(paths).not.toContain('/api/v2/transactions/stats');
  expect(paths).not.toContain('/api/v2/paxeer-x/feed/markets');
  expect(paths.some(path => path.startsWith('/api/v2/stats/charts/'))).toBe(false);
}

function atPath(root: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => node !== null && typeof node === 'object' ? Reflect.get(node, key) : undefined, root);
}

function resolveColour(path: string, mode: '_light' | '_dark', seen = new Set<string>()): string {
  expect(seen.has(path), path).toBe(false);
  seen.add(path);
  const node = atPath(semanticTokens, path) ?? atPath({ colors }, path);
  const value = atPath(atPath(node, 'DEFAULT') ?? node, 'value');
  const resolved = typeof value === 'string' ? value : atPath(value, mode);
  expect(resolved, `${ path } ${ mode }`).toBeTypeOf('string');
  if (typeof resolved !== 'string') {
    throw new Error(`Missing colour ${ path } in ${ mode }`);
  }
  const reference = resolved.match(/^\{(.+)\}$/);
  return reference ? resolveColour(reference[1], mode, seen) : resolved;
}

function statisticsColoursResolveInBothModes() {
  for (const mode of [ '_light', '_dark' ] as const) {
    for (const token of [ 'canvas', 'surface', 'hairline', 'text.primary', 'text.secondary', 'text.muted', 'bg.surface', 'border.divider' ]) {
      expect(resolveColour(`colors.${ token }`, mode)).toMatch(/^#|^rgba?\(/);
    }
    for (const tokens of Object.values(CHART_COLOR_TOKENS)) {
      expect(resolveColour(`colors.${ mode === '_light' ? tokens.light : tokens.dark }`, mode)).toMatch(/^#|^rgba?\(/);
    }
  }
}

test(explorerStatisticsRenderInLightMode.name, explorerStatisticsRenderInLightMode, 30_000);
test(explorerStatisticsRenderInDarkMode.name, explorerStatisticsRenderInDarkMode, 30_000);
test(absentStatisticsRenderDashesAndPreserveZero.name, absentStatisticsRenderDashesAndPreserveZero, 30_000);
test(failedTransactionStatisticsStayInTheirSection.name, failedTransactionStatisticsStayInTheirSection, 30_000);
test(failedMarketChartsKeepOtherChartsAndCounters.name, failedMarketChartsKeepOtherChartsAndCounters, 30_000);
test(statisticsFiltersSelectSectionsNamesAndIntervals.name, statisticsFiltersSelectSectionsNamesAndIntervals, 30_000);
test(configuredStatisticsServiceKeepsItsCountersAndCharts.name, configuredStatisticsServiceKeepsItsCountersAndCharts, 30_000);
test(statisticsColoursResolveInBothModes.name, statisticsColoursResolveInBothModes);

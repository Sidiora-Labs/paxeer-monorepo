// @vitest-environment jsdom

import type { AppProps } from 'next/dist/shared/lib/router/router';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { ChartMarketResponse } from 'types/api/charts';
import type { HomeStatsWidgetId } from 'types/homepage';

import * as blockFixtures from 'mocks/blocks/block';
import { outputRootsData } from 'mocks/optimism/outputRoots';
import * as dailyTxsFixtures from 'mocks/stats/daily_txs';
import * as statsFixtures from 'mocks/stats/index';
import * as mainStatsFixtures from 'mocks/stats/main';
import { afterAll, afterEach, beforeEach, expect, test as registerTest, vi } from 'vitest';

import { HIGHLIGHTS_CONFIG, HIGHLIGHTS_CONFIG_URL, IMAGE_URL_1, IMAGE_URL_2, IMAGE_URL_3 } from './homepageConfiguration.fixtures';

fetchMock.disableMocks();

let banners = HIGHLIGHTS_CONFIG;
let utilization: number | null = statsFixtures.base.network_utilization_percentage;
let gasPrices = statsFixtures.base.gas_prices;
const requests: Array<string> = [];
const latestBatch = 17384;
const marketChart: ChartMarketResponse = {
  available_supply: '165672954',
  chart_data: [ '2026-01-01', '2026-01-02' ].map((date) => ({
    date,
    closing_price: statsFixtures.base.coin_price,
    market_cap: statsFixtures.base.market_cap,
    tvl: statsFixtures.base.tvl,
  })),
};
const server = createServer((request, response) => {
  const path = new URL(request.url!, 'http://localhost').pathname;
  requests.push(path);
  response.setHeader('Content-Type', 'application/json');
  const body = (() => {
    switch (path) {
      case '/explorer/assets/configs/homepage_highlights_config.json': return banners;
      case '/api/v2/stats': return {
        ...statsFixtures.withBtcLocked, network_utilization_percentage: utilization, gas_prices: gasPrices, celo: { epoch_number: 42 },
      };
      case '/api/v2/main-page/blocks': return [ blockFixtures.base, blockFixtures.base2 ];
      case '/api/v1/pages/main': return mainStatsFixtures.base;
      case '/api/v2/stats/charts/transactions': return dailyTxsFixtures.base;
      case '/api/v2/stats/charts/market': return marketChart;
      case '/api/v2/main-page/arbitrum/batches/latest-number': return latestBatch;
      case '/api/v2/optimism/output-roots': return outputRootsData;
      default: response.statusCode = 404; return { error: `Unexpected fixture request: ${ path }` };
    }
  })();
  response.end(JSON.stringify(body));
});
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
afterAll(async() => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

const port = String((server.address() as AddressInfo).port);
const origin = `http://127.0.0.1:${ port }`;
const allWidgetIds: Array<HomeStatsWidgetId> = [
  'total_blocks', 'average_block_time', 'total_txs', 'wallet_addresses', 'gas_tracker', 'btc_locked',
  'current_epoch', 'total_operational_txs', 'latest_batch', 'latest_l1_state_batch',
];
const heroConfig = {
  background: [ 'lightpink', 'navy' ],
  text_color: [ 'deepskyblue', 'white' ],
  border: [ '3px solid green', '3px dashed yellow' ],
};
window.__envs = {
  ...window.__envs,
  NEXT_PUBLIC_APP_INSTANCE: 'vitest',
  NEXT_PUBLIC_APP_ENV: 'testing',
  NEXT_PUBLIC_NETWORK_NAME: 'Blockscout',
  NEXT_PUBLIC_NETWORK_ID: '1',
  NEXT_PUBLIC_NETWORK_CURRENCY_NAME: 'Ether',
  NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL: 'ETH',
  NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS: '18',
  NEXT_PUBLIC_NETWORK_CURRENCY_GWEI_NAME: 'Gwei',
  NEXT_PUBLIC_NETWORK_RPC_URL: '',
  NEXT_PUBLIC_IS_TESTNET: 'true',
  NEXT_PUBLIC_GAS_TRACKER_UNITS: '["gwei"]',
  NEXT_PUBLIC_GAS_TRACKER_ENABLED: 'true',
  NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  NEXT_PUBLIC_API_BASE_PATH: '/',
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: port,
  NEXT_PUBLIC_APP_PROTOCOL: 'http',
  NEXT_PUBLIC_APP_HOST: '127.0.0.1',
  NEXT_PUBLIC_APP_PORT: port,
  NEXT_PUBLIC_STATS_API_HOST: origin,
  NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
  NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
  NEXT_PUBLIC_HOMEPAGE_CHARTS: '[]',
  NEXT_PUBLIC_HOMEPAGE_STATS: JSON.stringify(allWidgetIds),
  NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG: JSON.stringify(heroConfig),
  NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG: HIGHLIGHTS_CONFIG_URL,
  NEXT_PUBLIC_ROLLUP_TYPE: 'arbitrum',
  NEXT_PUBLIC_ROLLUP_PARENT_CHAIN: JSON.stringify({ name: 'Fixture L1', baseUrl: origin }),
};

const { 'default': config } = await import('configs/app');
await import('ui/shared/layout/testWrapper');
const { render: renderWithProviders } = await import('vitest/lib');
const { Provider } = await import('toolkit/chakra/provider');
const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': HeroBanner } = await import('./HeroBanner');
const { 'default': PromotionalBanners } = await import('./PromotionalBanners');
const { 'default': Highlights } = await import('./Highlights');
const { 'default': Stats } = await import('./Stats');
const { 'default': LatestBlocks } = await import('./LatestBlocks');
const { HomeRpcDataContextProvider } = await import('./fallbacks/rpcDataContext');

window.__NEXT_DATA__ = { props: { pageProps: {} }, page: '/', query: {}, buildId: 'homepage-configuration' };
window.__DEV_MIDDLEWARE_MATCHERS = [];
window.__SSG_MANIFEST = new Set();

const FixtureApp = ({ Component, pageProps }: AppProps) => React.createElement(Component, pageProps);
const router = new Router('/', {}, '/', {
  initialProps: {},
  pageLoader: new PageLoader('homepage-configuration', ''),
  App: FixtureApp,
  Component: HeroBanner,
  subscription: async() => undefined,
  wrapApp: (App) => App,
  isFallback: false,
});

function render(component: React.ReactElement, mode: 'light' | 'dark' = 'light') {
  localStorage.setItem('theme', mode);
  return renderWithProviders(React.createElement(RouterContext.Provider, { value: router },
    React.createElement(Provider, { defaultTheme: mode, enableSystem: false }, component),
  ));
}

beforeEach(() => {
  requests.length = 0;
  banners = HIGHLIGHTS_CONFIG;
  utilization = statsFixtures.base.network_utilization_percentage;
  gasPrices = statsFixtures.base.gas_prices;
  config.UI.homepage.stats.splice(0, config.UI.homepage.stats.length, ...allWidgetIds);
  config.UI.homepage.charts.splice(0);
});

afterEach(() => {
  expect(requests.filter((path) => ![
    '/explorer/assets/configs/homepage_highlights_config.json', '/api/v2/stats', '/api/v2/main-page/blocks',
    '/api/v1/pages/main', '/api/v2/main-page/arbitrum/batches/latest-number', '/api/v2/optimism/output-roots',
    '/api/v2/stats/charts/transactions', '/api/v2/stats/charts/market',
  ].includes(path))).toEqual([]);
});

registerTest('heroContainerConfigLight', async function heroContainerConfigLight() {
  expect(config.UI.homepage.heroBanner).toEqual(heroConfig);
  const { container } = render(React.createElement(HeroBanner));
  const hero = container.querySelector('[data-label="hero"]') as HTMLElement;
  expect(hero.style.background).toBe('lightpink');
  expect(hero.style.border).toBe('3px solid green');
  expect((container.querySelector('[data-label="hero-title"]') as HTMLElement).style.color).toBe('deepskyblue');
  expect(container.querySelector('[data-label="hero-search"] input')).not.toBeNull();
});

registerTest('heroCustomizationDark', async function heroCustomizationDark() {
  const { container } = render(React.createElement(HeroBanner), 'dark');
  await expect.poll(() => (container.querySelector('[data-label="hero"]') as HTMLElement).style.background).toBe('navy');
  expect((container.querySelector('[data-label="hero"]') as HTMLElement).style.border).toBe('3px dashed yellow');
  expect((container.querySelector('[data-label="hero-title"]') as HTMLElement).style.color).toBe('white');
});

async function assertBanners(container: HTMLElement, count: number, dark: boolean) {
  await expect.poll(() => container.querySelectorAll('[data-promotional-banner]').length).toBe(count);
  const rows = Array.from(container.querySelectorAll('[data-promotional-banner]'));
  expect(rows.map((row) => row.querySelector('[data-banner-title]')?.textContent)).toEqual(HIGHLIGHTS_CONFIG.slice(0, count).map((banner) => banner.title));
  expect(rows.map((row) => row.querySelector('[data-banner-description]')?.textContent))
    .toEqual(HIGHLIGHTS_CONFIG.slice(0, count).map((banner) => banner.description));
  expect(rows[0].closest('a')?.getAttribute('href')).toBe('https://example.com');
  expect(rows[1].closest('a')?.getAttribute('href')).toBe('/essential-dapps/swap');
  expect(rows[0].querySelector('img')?.getAttribute('src')).toBe(IMAGE_URL_1);
  await expect.poll(() => rows[1].querySelector('img')?.getAttribute('src')).toBe(dark ? IMAGE_URL_3 : IMAGE_URL_2);
  expect((rows[0] as HTMLElement).style.background).toBe('rgb(6, 51, 27)');
  expect(requests).toContain('/explorer/assets/configs/homepage_highlights_config.json');
}

registerTest('highlightsConfiguredHomepageEntry', async function highlightsConfiguredHomepageEntry() {
  const { container } = render(React.createElement(HeroBanner));
  await assertBanners(container, 3, false);
  expect(container.querySelector('[data-label="hero"]')).not.toBeNull();
});

registerTest('highlightsTwoAndThreeAdaptiveBanners', async function highlightsTwoAndThreeAdaptiveBanners() {
  banners = HIGHLIGHTS_CONFIG.slice(0, 2);
  const first = render(React.createElement(PromotionalBanners));
  await assertBanners(first.container, 2, false);
  expect(first.container.textContent).not.toContain('Duckling Smart Contracts');
  first.unmount();
  banners = HIGHLIGHTS_CONFIG;
  const reopened = render(React.createElement(PromotionalBanners), 'dark');
  await assertBanners(reopened.container, 3, true);
  expect(requests.filter((path) => path === '/explorer/assets/configs/homepage_highlights_config.json')).toHaveLength(2);
});

registerTest('highlightsStatisticsRemainSeparate', async function highlightsStatisticsRemainSeparate() {
  const empty = render(React.createElement(Highlights));
  expect(empty.container.querySelector('[data-label="home-highlights"]')).toBeNull();
  empty.unmount();
  const { container } = render(React.createElement(Highlights, { items: [ { id: 'coin_price', label: 'ETH price', value: '$1.00' } ] }));
  expect(container.querySelector('[data-highlight="coin_price"]')?.textContent).toContain('$1.00');
  expect(container.querySelector('[data-promotional-banner]')).toBeNull();
  expect(requests).toEqual([]);
});

registerTest.each([
  [ null, undefined ],
  [ 0, 'positive' ],
  [ 55.1234, 'warning' ],
  [ 91.2345, 'negative' ],
] as const)('nullableUtilizationAndNumericBehavior %s', async function nullableUtilizationAndNumericBehavior(value, expectedColor) {
  utilization = value;
  const { container, unmount } = render(React.createElement(HomeRpcDataContextProvider, null, React.createElement(LatestBlocks)));
  await expect.poll(() => container.querySelectorAll('[data-latest-block]').length).toBe(2);
  const note = container.querySelector('[data-note]') as HTMLElement;
  await expect.poll(() => note.textContent).toContain('Current epoch: #42');
  if (value === null) {
    expect(note.textContent).not.toContain('Network utilization:');
    expect(note.textContent).not.toContain('%');
  } else {
    await expect.poll(() => note.textContent).toContain(`${ value.toFixed(2) }%`);
    const number = Array.from(note.querySelectorAll('span')).find((span) => span.textContent === `${ value.toFixed(2) }%`)!;
    expect(number.className).toBeTruthy();
    const rules = Array.from(document.querySelectorAll('style')).flatMap((style) => Array.from(style.sheet?.cssRules ?? []));
    const colorClass = Array.from(number.classList).find((className) => className.startsWith('css-'));
    const colorRule = rules.find((rule) => rule.cssText.includes(`.${ colorClass }`));
    expect(colorRule?.cssText).toContain(`var(--chakra-colors-${ expectedColor })`);
  }
  expect(requests).toContain('/api/v2/stats');
  unmount();
});

function widgetValues(container: HTMLElement) {
  return Object.fromEntries(Array.from(container.querySelectorAll('[data-stat]')).map((row) => [
    row.getAttribute('data-stat'), row.querySelector('[data-value]')?.textContent,
  ]));
}

const serviceTileValues = {
  total_blocks: Number(mainStatsFixtures.base.total_blocks!.value).toLocaleString(),
  average_block_time: `${ Number(mainStatsFixtures.base.average_block_time!.value).toFixed(1) }s`,
  total_txs: Number(mainStatsFixtures.base.total_transactions!.value).toLocaleString(),
  wallet_addresses: Number(mainStatsFixtures.base.total_addresses!.value).toLocaleString(),
  gas_tracker: '23.75 Gwei',
};
const backendTileValues = {
  total_blocks: Number(statsFixtures.base.total_blocks).toLocaleString(),
  average_block_time: '6.2s',
  total_txs: Number(statsFixtures.base.total_transactions).toLocaleString(),
  wallet_addresses: Number(statsFixtures.base.total_addresses).toLocaleString(),
  gas_tracker: '23.75 Gwei',
};

registerTest('statsCoreTilesAndProducerValues', async function statsCoreTilesAndProducerValues() {
  expect(config.UI.homepage.stats).toEqual(allWidgetIds);
  const { container } = render(React.createElement(Stats));
  await expect.poll(() => widgetValues(container).wallet_addresses).toBe(serviceTileValues.wallet_addresses);
  await expect.poll(() => widgetValues(container).total_blocks).toBe(serviceTileValues.total_blocks);
  await expect.poll(() => widgetValues(container)).toEqual(serviceTileValues);
  expect(Array.from(container.querySelectorAll('[data-stat] h2'), (label) => label.textContent))
    .toEqual([ 'Total blocks', 'Average block time', 'Total transactions', 'Wallet addresses', 'Gas price' ]);
  for (const id of [ 'btc_locked', 'current_epoch', 'total_operational_txs', 'latest_batch', 'latest_l1_state_batch' ]) {
    expect(container.querySelector(`[data-stat="${ id }"]`)).toBeNull();
  }
  expect(requests).toContain('/api/v1/pages/main');
  expect(requests).toContain('/api/v2/stats');
  expect(requests).not.toContain('/api/v2/main-page/blocks');
  expect(requests).not.toContain('/api/v2/main-page/arbitrum/batches/latest-number');
  expect(requests).not.toContain('/api/v2/optimism/output-roots');
});

registerTest('statsMissingGasKeepsCoreTiles', async function statsMissingGasKeepsCoreTiles() {
  config.UI.homepage.stats.splice(0, config.UI.homepage.stats.length, 'wallet_addresses', 'gas_tracker');
  gasPrices = null;
  const { container } = render(React.createElement(Stats));
  await expect.poll(() => widgetValues(container)).toEqual({ ...serviceTileValues, gas_tracker: '—' });
  for (const id of allWidgetIds) {
    const tile = container.querySelector(`[data-stat="${ id }"]`);
    if (id in serviceTileValues) {
      expect(tile).not.toBeNull();
    } else {
      expect(tile).toBeNull();
    }
  }
  expect(requests).not.toContain('/api/v2/main-page/arbitrum/batches/latest-number');
});

registerTest('statsConfiguredMarketChartsAndCoreTiles', async function statsConfiguredMarketChartsAndCoreTiles() {
  config.UI.homepage.stats.splice(0);
  const empty = render(React.createElement(Stats));
  await expect.poll(() => requests.includes('/api/v2/stats')).toBe(true);
  expect(empty.container.querySelector('[data-label="home-stats"]')).not.toBeNull();
  await expect.poll(() => widgetValues(empty.container)).toEqual(serviceTileValues);
  expect(empty.container.querySelector('[data-label="home-market-history"]')).toBeNull();
  empty.unmount();
  config.UI.homepage.charts.push('coin_price', 'market_cap');
  const { container } = render(React.createElement(Stats));
  await expect.poll(() => widgetValues(container)).toEqual(serviceTileValues);
  for (const [ id, field, title ] of [ [ 'coin_price', 'closing_price', 'ETH price' ], [ 'market_cap', 'market_cap', 'ETH market value' ] ] as const) {
    const chart = container.querySelector(`[data-label="home-market-history"][data-indicator="${ id }"]`)!;
    await expect.poll(() => chart.querySelector('[data-points]')?.getAttribute('data-points')).toBe('2');
    expect(chart.querySelector('[data-title]')?.textContent).toBe(title);
    expect(chart.querySelector('svg')?.getAttribute('aria-label')).toBe(title);
    expect(chart.querySelector('desc')?.textContent?.trim()).toBe(marketChart.chart_data.map((sample) => (
      `${ new Date(sample.date).toLocaleDateString() }: ${ Number(sample[field]).toLocaleString(undefined, { maximumSignificantDigits: 21 }) }`
    )).join('; '));
  }
  expect(requests).toContain('/api/v2/stats/charts/market');
});

registerTest('statsDefaultBackendTiles', async function statsDefaultBackendTiles() {
  vi.resetModules();
  window.__envs = { ...window.__envs, NEXT_PUBLIC_STATS_API_HOST: '', NEXT_PUBLIC_HOMEPAGE_STATS: '', NEXT_PUBLIC_ROLLUP_TYPE: '' };
  const { 'default': BackendStats } = await import('./Stats');
  const { container } = render(React.createElement(BackendStats));
  await expect.poll(() => widgetValues(container).wallet_addresses).toBe(backendTileValues.wallet_addresses);
  await expect.poll(() => widgetValues(container)).toEqual(backendTileValues);
  expect(requests).not.toContain('/api/v1/pages/main');
  await expect.poll(() => requests).toContain('/api/v2/stats/charts/transactions');
  expect(container.querySelector('[data-label="home-stats-history"] [data-title]')?.textContent)
    .toBe('Blockscout transaction history in 14 days');
});

registerTest('statsOptimisticCoreTiles', async function statsOptimisticCoreTiles() {
  vi.resetModules();
  window.__envs = { ...window.__envs, NEXT_PUBLIC_ROLLUP_TYPE: 'optimistic', NEXT_PUBLIC_HOMEPAGE_STATS: '["latest_l1_state_batch"]' };
  const { 'default': OptimisticStats } = await import('./Stats');
  const { container } = render(React.createElement(OptimisticStats));
  await expect.poll(() => widgetValues(container)).toEqual(backendTileValues);
  expect(container.querySelector('[data-stat="latest_l1_state_batch"]')).toBeNull();
  expect(requests).not.toContain('/api/v2/optimism/output-roots');
  await expect.poll(() => requests).toContain('/api/v2/stats/charts/transactions');
});

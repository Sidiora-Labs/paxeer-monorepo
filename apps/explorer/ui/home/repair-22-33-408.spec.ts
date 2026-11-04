// @vitest-environment jsdom

import type { AppProps } from 'next/dist/shared/lib/router/router';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { HomeStatsWidgetId } from 'types/homepage';

import * as blockFixtures from 'mocks/blocks/block';
import { outputRootsData } from 'mocks/optimism/outputRoots';
import * as statsFixtures from 'mocks/stats/index';
import * as mainStatsFixtures from 'mocks/stats/main';
import { afterAll, afterEach, beforeEach, expect, test as registerTest, vi } from 'vitest';

import { HIGHLIGHTS_CONFIG, HIGHLIGHTS_CONFIG_URL, IMAGE_URL_1, IMAGE_URL_2, IMAGE_URL_3 } from './repair-22-33-408.fixtures';

fetchMock.disableMocks();

let banners = HIGHLIGHTS_CONFIG;
let utilization: number | null = statsFixtures.base.network_utilization_percentage;
let gasPrices = statsFixtures.base.gas_prices;
const requests: Array<string> = [];
const latestBatch = 17384;
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

window.__NEXT_DATA__ = { props: { pageProps: {} }, page: '/', query: {}, buildId: 'repair-22-33-408' };
window.__DEV_MIDDLEWARE_MATCHERS = [];
window.__SSG_MANIFEST = new Set();

const FixtureApp = ({ Component, pageProps }: AppProps) => React.createElement(Component, pageProps);
const router = new Router('/', {}, '/', {
  initialProps: {},
  pageLoader: new PageLoader('repair-22-33-408', ''),
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
  ].includes(path))).toEqual([]);
});

async function heroContainerConfigLight() {
  expect(config.UI.homepage.heroBanner).toEqual(heroConfig);
  const { container } = render(React.createElement(HeroBanner));
  const hero = container.querySelector('[data-label="hero"]') as HTMLElement;
  expect(hero.style.background).toBe('lightpink');
  expect(hero.style.border).toBe('3px solid green');
  expect((container.querySelector('[data-label="hero-title"]') as HTMLElement).style.color).toBe('deepskyblue');
  expect(container.querySelector('[data-label="hero-search"] input')).not.toBeNull();
}

async function heroCustomizationDark() {
  const { container } = render(React.createElement(HeroBanner), 'dark');
  await expect.poll(() => (container.querySelector('[data-label="hero"]') as HTMLElement).style.background).toBe('navy');
  expect((container.querySelector('[data-label="hero"]') as HTMLElement).style.border).toBe('3px dashed yellow');
  expect((container.querySelector('[data-label="hero-title"]') as HTMLElement).style.color).toBe('white');
}

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

async function highlightsConfiguredHomepageEntry() {
  const { container } = render(React.createElement(HeroBanner));
  await assertBanners(container, 3, false);
  expect(container.querySelector('[data-label="hero"]')).not.toBeNull();
}

async function highlightsTwoAndThreeAdaptiveBanners() {
  banners = HIGHLIGHTS_CONFIG.slice(0, 2);
  const first = render(React.createElement(PromotionalBanners));
  await assertBanners(first.container, 2, false);
  expect(first.container.textContent).not.toContain('Duckling Smart Contracts');
  first.unmount();
  banners = HIGHLIGHTS_CONFIG;
  const reopened = render(React.createElement(PromotionalBanners), 'dark');
  await assertBanners(reopened.container, 3, true);
  expect(requests.filter((path) => path === '/explorer/assets/configs/homepage_highlights_config.json')).toHaveLength(2);
}

async function highlightsStatisticsRemainSeparate() {
  const empty = render(React.createElement(Highlights));
  expect(empty.container.querySelector('[data-label="home-highlights"]')).toBeNull();
  empty.unmount();
  const { container } = render(React.createElement(Highlights, { items: [ { id: 'coin_price', label: 'ETH price', value: '$1.00' } ] }));
  expect(container.querySelector('[data-highlight="coin_price"]')?.textContent).toContain('$1.00');
  expect(container.querySelector('[data-promotional-banner]')).toBeNull();
  expect(requests).toEqual([]);
}

async function nullableUtilizationAndNumericBehavior() {
  for (const [ value, expectedColor ] of [ [ null, undefined ], [ 0, 'green' ], [ 55.1234, 'orange' ], [ 91.2345, 'red' ] ] as const) {
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
      expect(colorRule?.cssText).toContain(`var(--chakra-colors-${ expectedColor }-600)`);
    }
    expect(requests).toContain('/api/v2/stats');
    unmount();
  }
}

function widgetValues(container: HTMLElement) {
  return Object.fromEntries(Array.from(container.querySelectorAll('[data-highlight]')).map((row) => [
    row.getAttribute('data-highlight'), row.querySelector('[data-value]')?.textContent,
  ]));
}

async function statsWidgetSelectionAndProducerValues() {
  expect(config.UI.homepage.stats).toEqual(allWidgetIds);
  const { container } = render(React.createElement(Stats));
  await expect.poll(() => widgetValues(container).wallet_addresses).toBe(Number(mainStatsFixtures.base.total_addresses!.value).toLocaleString());
  await expect.poll(() => widgetValues(container).latest_batch).toBe(latestBatch.toLocaleString());
  await expect.poll(() => widgetValues(container)).toEqual({
    total_blocks: blockFixtures.base.height.toLocaleString(),
    average_block_time: `${ Number(mainStatsFixtures.base.average_block_time!.value).toFixed(1) }s`,
    total_txs: Number(mainStatsFixtures.base.total_transactions!.value).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' }),
    wallet_addresses: Number(mainStatsFixtures.base.total_addresses!.value).toLocaleString(),
    gas_tracker: '23.8 Gwei',
    btc_locked: '3,337.49',
    current_epoch: '42',
    total_operational_txs: Number(mainStatsFixtures.base.total_operational_transactions!.value).toLocaleString(),
    latest_batch: latestBatch.toLocaleString(),
  });
  expect(container.querySelector('[data-highlight="latest_l1_state_batch"]')).toBeNull();
  expect(requests).toContain('/api/v1/pages/main');
  expect(requests).toContain('/api/v2/stats');
  expect(requests).toContain('/api/v2/main-page/blocks');
}

async function statsDisabledWidgetsAndMissingGas() {
  config.UI.homepage.stats.splice(0, config.UI.homepage.stats.length, 'wallet_addresses', 'gas_tracker');
  gasPrices = null;
  const { container } = render(React.createElement(Stats));
  await expect.poll(() => widgetValues(container)).toEqual({ wallet_addresses: Number(mainStatsFixtures.base.total_addresses!.value).toLocaleString() });
  for (const id of allWidgetIds.filter((id) => id !== 'wallet_addresses')) {
    expect(container.querySelector(`[data-highlight="${ id }"]`)).toBeNull();
  }
  expect(requests).not.toContain('/api/v2/main-page/arbitrum/batches/latest-number');
}

async function statsCoinSelectionAndEmptySelection() {
  config.UI.homepage.stats.splice(0);
  const empty = render(React.createElement(Stats));
  await expect.poll(() => requests.includes('/api/v2/stats')).toBe(true);
  expect(empty.container.querySelector('[data-label="home-stats"]')).toBeNull();
  empty.unmount();
  config.UI.homepage.charts.push('coin_price', 'market_cap');
  const { container } = render(React.createElement(Stats));
  await expect.poll(() => widgetValues(container).coin_price).toBe('$0.001997');
  await expect.poll(() => widgetValues(container)).toEqual({ coin_price: '$0.001997', market_cap: '$330,809.96' });
}

async function statsDefaultBackendWidgets() {
  vi.resetModules();
  window.__envs = { ...window.__envs, NEXT_PUBLIC_STATS_API_HOST: '', NEXT_PUBLIC_HOMEPAGE_STATS: '', NEXT_PUBLIC_ROLLUP_TYPE: '' };
  const { 'default': BackendStats } = await import('./Stats');
  const { container } = render(React.createElement(BackendStats));
  await expect.poll(() => widgetValues(container).wallet_addresses).toBe(Number(statsFixtures.base.total_addresses).toLocaleString());
  await expect.poll(() => widgetValues(container)).toEqual({
    total_blocks: blockFixtures.base.height.toLocaleString(),
    average_block_time: '6.2s',
    total_txs: Number(statsFixtures.base.total_transactions).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' }),
    wallet_addresses: Number(statsFixtures.base.total_addresses).toLocaleString(),
    gas_tracker: '23.8 Gwei',
  });
  expect(requests).not.toContain('/api/v1/pages/main');
}

async function statsOptimisticStateBatch() {
  vi.resetModules();
  window.__envs = { ...window.__envs, NEXT_PUBLIC_ROLLUP_TYPE: 'optimistic', NEXT_PUBLIC_HOMEPAGE_STATS: '["latest_l1_state_batch"]' };
  const { 'default': OptimisticStats } = await import('./Stats');
  const { container } = render(React.createElement(OptimisticStats));
  await expect.poll(() => widgetValues(container)).toEqual({ latest_l1_state_batch: outputRootsData.items[0].l2_output_index.toLocaleString() });
  expect(requests).toContain('/api/v2/optimism/output-roots');
}

function test(proof: () => Promise<void>) {
  registerTest(proof.name, proof);
}

test(heroContainerConfigLight);
test(heroCustomizationDark);
test(highlightsConfiguredHomepageEntry);
test(highlightsTwoAndThreeAdaptiveBanners);
test(highlightsStatisticsRemainSeparate);
test(nullableUtilizationAndNumericBehavior);
test(statsWidgetSelectionAndProducerValues);
test(statsDisabledWidgetsAndMissingGas);
test(statsCoinSelectionAndEmptySelection);
test(statsDefaultBackendWidgets);
test(statsOptimisticStateBatch);

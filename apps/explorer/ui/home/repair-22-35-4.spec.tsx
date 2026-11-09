// @vitest-environment jsdom

import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { TimeChartData, TimeChartItemRaw } from 'toolkit/components/charts/types';
import type { TokenInfo } from 'types/api/token';

import useApiQuery from 'lib/api/useApiQuery';
import * as blockMock from 'mocks/blocks/block';
import * as statsMock from 'mocks/stats/index';
import { tokenInfoERC20b } from 'mocks/tokens/tokenInfo';
import * as txMock from 'mocks/txs/tx';
import colors from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import Home from 'ui/pages/Home';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import StatsWidget from 'ui/shared/stats/StatsWidget';
import { beforeEach, expect, test, vi } from 'vitest';
import { screen, waitFor, within } from 'vitest/lib';

import ChainIndicatorChartContainer from './indicators/ChainIndicatorChartContainer';
import { getChartData, prepareChartItems } from './indicators/utils/chart';
import Stats from './Stats';
import TickerStrip from './TickerStrip';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_STATS_API_HOST: '',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

vi.setConfig({ testTimeout: 60_000 });

if (!('getTotalLength' in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, 'getTotalLength', { configurable: true, value: () => 0 });
}

const pricedTokens = [
  { ...tokenInfoERC20b, symbol: 'UP', exchange_rate: '1.25', price_change_percentage_24h: '4.2' },
  { ...tokenInfoERC20b, address_hash: '0x0000000000000000000000000000000000000002', symbol: 'DOWN', exchange_rate: '2.50', price_change_percentage_24h: -3.1 },
  { ...tokenInfoERC20b, address_hash: '0x0000000000000000000000000000000000000003', symbol: 'NO_RATE', exchange_rate: null },
];
let tokenItems: Array<TokenInfo> = pricedTokens;
let statsResponse: object = statsMock.base;
let chartResponse: object = { chart_data: [] };

beforeEach(() => {
  routerState.pathname = '/';
  routerState.query = {};
  tokenItems = pricedTokens;
  statsResponse = statsMock.base;
  chartResponse = { chart_data: [] };
  fetchMock.resetMocks();
  fetchMock.mockResponse((request) => {
    let body: unknown = {};
    if (request.url.includes('/api/v2/tokens')) body = { items: tokenItems, next_page_params: null };
    else if (request.url.includes('/api/v2/stats/charts/transactions')) body = chartResponse;
    else if (request.url.includes('/api/v2/stats')) body = statsResponse;
    else if (request.url.includes('/api/v2/main-page/blocks')) body = [ blockMock.base, blockMock.base2 ];
    else if (request.url.includes('/api/v2/main-page/transactions')) body = [ txMock.base, txMock.base2 ];
    return { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
  });
});

function cssFor(element: Element) {
  return Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText))
    .filter((rule) => Array.from(element.classList).some((name) => rule.includes(`.${ name }`))).join('').replace(/\s+/g, '');
}

function TokenResponseState() {
  const query = useApiQuery('general:tokens');
  return query.isSuccess ? <output>Tokens loaded</output> : null;
}

async function tickerUsesOnlyReturnedRatesAndSignedChanges() {
  const { container } = render(<TickerStrip/>);
  await screen.findByText('UP');
  const chips = container.querySelectorAll('[data-ticker-token]');
  expect(chips).toHaveLength(2);
  expect(chips[0].querySelector('[data-price]')?.textContent).toBe('$1.25');
  expect(chips[1].querySelector('[data-price]')?.textContent).toBe('$2.50');
  expect(chips[0].querySelector('[data-change]')?.textContent).toBe('+4.2%');
  expect(chips[1].querySelector('[data-change]')?.textContent).toBe('-3.1%');
  expect(cssFor(chips[0].querySelector('[data-change]')!)).toContain('color:var(--chakra-colors-positive)');
  expect(cssFor(chips[1].querySelector('[data-change]')!)).toContain('color:var(--chakra-colors-negative)');
  expect(screen.queryByText('NO_RATE')).toBeNull();
  expect(fetchMock.mock.calls.some(([ request ]) => String(request).includes('/api/v2/tokens'))).toBe(true);
}

test(tickerUsesOnlyReturnedRatesAndSignedChanges.name, tickerUsesOnlyReturnedRatesAndSignedChanges);

async function tickerIsAbsentWithoutPricedTokens() {
  for (const items of [ [ pricedTokens[2] ], [] ]) {
    tokenItems = items;
    const { container, unmount } = render(<><TickerStrip/><TokenResponseState/></>);
    await screen.findByText('Tokens loaded');
    expect(container.querySelector('[data-label="ticker-strip"]')).toBeNull();
    expect(screen.queryByRole('region', { name: 'Token prices' })).toBeNull();
    unmount();
  }
}

test(tickerIsAbsentWithoutPricedTokens.name, tickerIsAbsentWithoutPricedTokens);

async function tickerDoesNotInventMissingChangesOrRoundSmallPricesToZero() {
  tokenItems = [ { ...tokenInfoERC20b, symbol: 'UP', exchange_rate: '0.00000125' } ];
  const { container } = render(<TickerStrip/>);
  await screen.findByText('UP');
  expect(container.querySelector('[data-price]')?.textContent).toBe('$0.00000125');
  const change = container.querySelector('[data-change]')!;
  expect(change.textContent).toBe('—');
  expect(cssFor(change)).toContain('color:var(--chakra-colors-text-muted)');
}

test(tickerDoesNotInventMissingChangesOrRoundSmallPricesToZero.name, tickerDoesNotInventMissingChangesOrRoundSmallPricesToZero);

async function fiveTilesUseTheStatsResponse() {
  const { container } = render(<Stats/>);
  await waitFor(() => expect(container.querySelector('[data-stat="total_blocks"] [data-value]')?.textContent).toBe('30,215,608'));
  expect(Array.from(container.querySelectorAll('[data-stat] h2')).map((label) => label.textContent))
    .toEqual([ 'Total blocks', 'Average block time', 'Total transactions', 'Wallet addresses', 'Gas price' ]);
  expect(Array.from(container.querySelectorAll('[data-stat] [data-value]')).map((value) => value.textContent))
    .toEqual([ '30,215,608', '6.2s', '82,258,122', '19,667,249', '23.75 Gwei' ]);
  const label = container.querySelector('[data-stat] h2')!.parentElement!;
  expect(cssFor(label)).toContain('text-transform:uppercase');
  expect(container.querySelector('[data-label="home-stats-history"]')).not.toBeNull();
  expect(container.querySelector('[data-label="sparkline"]')).toBeNull();
  await screen.findByLabelText('Network chart unavailable');
}

test(fiveTilesUseTheStatsResponse.name, fiveTilesUseTheStatsResponse);

async function absentStatsStayMutedDashesAndRealZeroStaysZero() {
  statsResponse = { ...statsMock.base, total_addresses: undefined, total_transactions: '0', average_block_time: null, gas_prices: null };
  const { container } = render(<Stats/>);
  await waitFor(() => expect(container.querySelector('[data-stat="total_txs"] [data-value]')?.textContent).toBe('0'));
  for (const id of [ 'wallet_addresses', 'average_block_time', 'gas_tracker' ]) {
    const value = container.querySelector(`[data-stat="${ id }"] [data-value]`)!;
    expect(value.textContent).toBe('—');
    expect(cssFor(value)).toContain('color:var(--chakra-colors-text-muted)');
  }
  expect(container.querySelectorAll('[data-stat]')).toHaveLength(5);
}

test(absentStatsStayMutedDashesAndRealZeroStaysZero.name, absentStatsStayMutedDashesAndRealZeroStaysZero);

async function homeOrdersTickerHeroTilesChartAndTables() {
  const { container } = render(<Home/>);
  await screen.findByText('UP');
  await waitFor(() => {
    const row = container.querySelector('table[aria-label="Latest transactions"] [data-latest-tx]');
    expect(row?.getAttribute('data-latest-tx')).toBe(txMock.base.hash);
  });
  const groups = [ 'ticker-strip', 'hero', 'home-stat-tiles', 'home-stats-history', 'home-lists' ];
  expect(Array.from(container.querySelectorAll(groups.map((group) => `[data-label="${ group }"]`).join(', ')))
    .map((item) => item.getAttribute('data-label')))
    .toEqual([ 'ticker-strip', 'hero', 'home-stat-tiles', 'home-stats-history', 'home-lists' ]);
  expect(container.querySelector('[data-label="hero-search"] input')?.getAttribute('placeholder')).toBe(
    'Search or ask: a transaction, address, block or token',
  );
  const sections = container.querySelectorAll('[data-home-table]');
  expect(sections).toHaveLength(2);
  for (const [ index, name ] of [ 'View all blocks', 'View all transactions' ].entries()) {
    const link = within(sections[index] as HTMLElement).getByRole('link', { name });
    expect(link.closest('[data-header]')).not.toBeNull();
    expect(link.getAttribute('href')).toBe(index === 0 ? '/blocks' : '/txs');
    expect(sections[index].querySelector('thead th')).not.toBeNull();
    expect(sections[index].querySelector('tbody tr')).not.toBeNull();
  }
}

test(homeOrdersTickerHeroTilesChartAndTables.name, homeOrdersTickerHeroTilesChartAndTables);

async function networkChartUsesReturnedSamples() {
  chartResponse = { chart_data: [
    { date: '2026-01-01', transactions_count: 5 },
    { date: '2026-01-02', transactions_count: null },
    { date: '2026-01-03', transactions_count: 0 },
  ] };
  const { container } = render(<Stats/>);
  await waitFor(() => expect(container.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points')).toBe('2'));
  expect(container.querySelector('[data-label="sparkline"]')).not.toBeNull();
  expect(prepareChartItems([
    { date: new Date('2026-01-01'), value: 5 },
    { date: new Date('2026-01-02'), value: null },
    { date: new Date('2026-01-03'), value: 0 },
  ]).map(({ value }) => value)).toEqual([ 5, 0 ]);
}

test(networkChartUsesReturnedSamples.name, networkChartUsesReturnedSamples);

async function homeChartCalendarDayWindow() {
  type Sample = [ string, TimeChartItemRaw['value'] ];
  type Observation = [ string, number ];
  const january = Array.from({ length: 30 }, (_, index): Sample => [
    `2026-01-${ String(index + 1).padStart(2, '0') }`, index === 28 ? null : index === 19 ? 0 : index + 1,
  ]);
  const januaryExpected: Array<Observation> = [
    [ '2026-01-17', 17 ], [ '2026-01-18', 18 ], [ '2026-01-19', 19 ], [ '2026-01-20', 0 ],
    [ '2026-01-21', 21 ], [ '2026-01-22', 22 ], [ '2026-01-23', 23 ], [ '2026-01-24', 24 ],
    [ '2026-01-25', 25 ], [ '2026-01-26', 26 ], [ '2026-01-27', 27 ], [ '2026-01-28', 28 ], [ '2026-01-30', 30 ],
  ];
  const cases: Array<{ name: string; samples: Array<Sample>; expected: Array<Observation> }> = [
    { name: 'January with a missing observation', samples: january, expected: januaryExpected },
    {
      name: 'sparse and unsorted with exact inclusive bounds',
      samples: [ [ '2026-01-30', 30 ], [ '2026-01-02', 2 ], [ '2026-01-25', '25' ], [ '2026-01-16', 16 ], [ '2026-01-17', 0 ] ],
      expected: [ [ '2026-01-17', 0 ], [ '2026-01-25', 25 ], [ '2026-01-30', 30 ] ],
    },
    {
      name: 'leading and trailing nulls preserve the raw-date anchor',
      samples: [
        [ '2026-01-01', null ], [ '2026-01-16', 16 ], [ '2026-01-17', 17 ],
        [ '2026-01-18', 18 ], [ '2026-01-29', null ], [ '2026-01-30', null ],
      ],
      expected: [ [ '2026-01-17', 17 ], [ '2026-01-18', 18 ] ],
    },
    {
      name: 'trailing null leaves only older observations outside the window',
      samples: [ [ '2026-01-01', 1 ], [ '2026-01-02', 2 ], [ '2026-01-30', null ] ],
      expected: [],
    },
    { name: 'all null', samples: [ [ '2026-01-01', null ], [ '2026-01-30', null ] ], expected: [] },
    { name: 'empty', samples: [], expected: [] },
    {
      name: 'month rollover',
      samples: [ [ '2026-02-19', 19 ], [ '2026-02-20', 20 ], [ '2026-02-28', 28 ], [ '2026-03-05', 5 ] ],
      expected: [ [ '2026-02-20', 20 ], [ '2026-02-28', 28 ], [ '2026-03-05', 5 ] ],
    },
    {
      name: 'year rollover',
      samples: [ [ '2025-12-22', 22 ], [ '2025-12-23', 23 ], [ '2025-12-31', 31 ], [ '2026-01-05', 5 ] ],
      expected: [ [ '2025-12-23', 23 ], [ '2025-12-31', 31 ], [ '2026-01-05', 5 ] ],
    },
    {
      name: 'leap day',
      samples: [ [ '2024-03-05', 5 ], [ '2024-02-20', 20 ], [ '2024-02-21', 21 ], [ '2024-02-29', 29 ] ],
      expected: [ [ '2024-02-21', 21 ], [ '2024-02-29', 29 ], [ '2024-03-05', 5 ] ],
    },
    {
      name: 'UTC day boundaries',
      samples: [
        [ '2026-01-30T23:59:59.999Z', 30 ], [ '2026-01-16T23:59:59.999Z', 16 ],
        [ '2026-01-17T01:00:00+01:00', 0 ], [ '2026-01-16T23:30:00-01:00', 17 ], [ '2026-01-31T00:30:00+01:00', 31 ],
      ],
      expected: [
        [ '2026-01-17T00:00:00.000Z', 0 ], [ '2026-01-17T00:30:00.000Z', 17 ],
        [ '2026-01-30T23:30:00.000Z', 31 ], [ '2026-01-30T23:59:59.999Z', 30 ],
      ],
    },
    {
      name: 'invalid dates and missing or invalid values never become zero',
      samples: [
        [ 'invalid', 99 ], [ '2026-01-17', 0 ], [ '2026-01-18', '' ], [ '2026-01-19', 'NaN' ],
        [ '2026-01-20', 'Infinity' ], [ '2026-01-21', 'unavailable' ], [ '2026-01-30', 30 ],
      ],
      expected: [ [ '2026-01-17', 0 ], [ '2026-01-30', 30 ] ],
    },
  ];
  const observations = (items: TimeChartData[0]['items']): Array<Observation> =>
    items.map(({ date, value }) => [ date.toISOString(), value ]);
  const expectedObservations = (items: Array<Observation>): Array<Observation> =>
    items.map(([ date, value ]) => [ new Date(date).toISOString(), value ]);

  for (const { name, samples, expected } of cases) {
    const raw = samples.map(([ date, value ]) => ({ date: new Date(date), value }));
    const original = raw.slice();
    expect(observations(prepareChartItems(raw, 14)), name).toEqual(expectedObservations(expected));
    expect(observations(getChartData('daily_txs', raw, 14)[0].items), name).toEqual(expectedObservations(expected));
    expect(raw, `${ name }: raw order is preserved`).toEqual(original);
  }
  expect(prepareChartItems([ { date: new Date('invalid'), value: 1 } ], 14)).toEqual([]);
  const fullJanuary = january.filter(([ , value ]) => value !== null).map(([ date, value ]): Observation => [ date, Number(value) ]);
  const rawJanuary = january.map(([ date, value ]) => ({ date: new Date(date), value }));
  expect(observations(prepareChartItems(rawJanuary))).toEqual(expectedObservations(fullJanuary));
  expect(observations(getChartData('coin_price', rawJanuary)[0].items)).toEqual(expectedObservations(fullJanuary));

  const originalEnvs = window.__envs;
  let samples: Array<Sample> = [];
  const requests: Array<string> = [];
  const server = createServer((request, response) => {
    requests.push(request.url || '');
    response.setHeader('Content-Type', 'application/json');
    if (request.url === '/api/v2/stats/charts/transactions') {
      response.end(JSON.stringify({ chart_data: samples.map(([ date, value ]) => ({ date, transactions_count: value })) }));
    } else if (request.url === '/api/v1/pages/main') {
      response.end(JSON.stringify({ daily_new_transactions: { chart: samples.map(([ date, value ]) => ({ date, value })) } }));
    } else if (request.url === '/api/v2/stats') {
      response.end(JSON.stringify(statsMock.base));
    } else {
      response.statusCode = 404;
      response.end(JSON.stringify({ error: 'Unexpected resource' }));
    }
  });
  fetchMock.disableMocks();
  try {
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const port = String((server.address() as AddressInfo).port);
    for (const statsEnabled of [ false, true ]) {
      window.__envs = {
        ...originalEnvs,
        NEXT_PUBLIC_API_HOST: '127.0.0.1',
        NEXT_PUBLIC_API_PORT: port,
        NEXT_PUBLIC_API_PROTOCOL: 'http',
        NEXT_PUBLIC_STATS_API_HOST: statsEnabled ? `http://127.0.0.1:${ port }` : '',
      };
      vi.resetModules();
      const { 'default': HomeStats } = await import('./Stats');
      const { 'default': useHomeChartDataQuery } = await import('./indicators/useChartDataQuery');
      const { 'default': HomeChart } = await import('./indicators/ChainIndicatorsChart');
      const { render: renderHome } = await import('ui/shared/layout/testWrapper');
      function QueryToChart() {
        const chartQuery = useHomeChartDataQuery('daily_txs', 14);
        const unboundedQuery = useHomeChartDataQuery('daily_txs');
        return (
          <>
            <HomeStats/>
            <section aria-label="Query to chart">
              <HomeChart title="Calendar window" isLoading={ false } chartQuery={ chartQuery }/>
              <output data-window>{ JSON.stringify(observations(chartQuery.data[0].items)) }</output>
              <output data-unbounded>{ JSON.stringify(observations(unboundedQuery.data[0].items)) }</output>
            </section>
          </>
        );
      }
      for (const scenario of cases) {
        samples = scenario.samples;
        requests.length = 0;
        const { container, unmount } = renderHome(<QueryToChart/>);
        try {
          const queryChart = within(container).getByRole('region', { name: 'Query to chart' });
          const homeChart = container.querySelector('[data-label="home-stats-history"]')!;
          await waitFor(() => {
            expect(requests).toContain(statsEnabled ? '/api/v1/pages/main' : '/api/v2/stats/charts/transactions');
            expect(queryChart.querySelector('output[data-window]')?.textContent, scenario.name)
              .toBe(JSON.stringify(expectedObservations(scenario.expected)));
            for (const chart of [ homeChart, queryChart ]) {
              if (scenario.expected.length < 2) {
                expect(within(chart as HTMLElement).getByLabelText('Network chart unavailable').textContent).toBe('—');
                expect(chart.querySelector('[data-label="sparkline"]')).toBeNull();
              } else {
                expect(chart.querySelector('[data-label="sparkline"]')).not.toBeNull();
                expect(chart.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points'))
                  .toBe(String(scenario.expected.length));
              }
            }
          });
          expect(homeChart.textContent).toContain('transaction history in 14 days');
          if (scenario.samples === january) {
            expect(queryChart.querySelector('output[data-unbounded]')?.textContent).toBe(JSON.stringify(expectedObservations(fullJanuary)));
          }
          expect(requests).not.toContain(statsEnabled ? '/api/v2/stats/charts/transactions' : '/api/v1/pages/main');
        } finally {
          unmount();
        }
      }
    }
  } finally {
    window.__envs = originalEnvs;
    fetchMock.enableMocks();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    vi.resetModules();
  }
}

test(homeChartCalendarDayWindow.name, homeChartCalendarDayWindow);

function missingSeriesNeverBecomesZeroOrAnInventedCurve() {
  const date = new Date('2026-01-01T00:00:00Z');
  expect(prepareChartItems([ { date, value: null }, { date, value: 0 }, { date, value: 5 } ]).map(({ value }) => value)).toEqual([ 0, 5 ]);
  const data: TimeChartData = [ { id: 'daily_txs', name: 'Transactions', charts: [], items: [ { date, value: 5 } ] } ];
  const { container } = render(<ChainIndicatorChartContainer data={ data } isError={ false } isPending={ false }/>);
  expect(screen.getByLabelText('Network chart unavailable').textContent).toBe('—');
  expect(container.querySelector('svg')).toBeNull();
}

test(missingSeriesNeverBecomesZeroOrAnInventedCurve.name, missingSeriesNeverBecomesZeroOrAnInventedCurve);

function statDeltaHandlesBothSignsAndZero() {
  const { container } = render(<>
    <StatsWidget label="Increase" value="10" diff={ 2 }/>
    <StatsWidget label="Decrease" value="10" diff={ -2 }/>
    <StatsWidget label="Unchanged" value="10" diff={ 0 }/>
  </>);
  expect(cssFor(within(container).getByText('+2'))).toContain('color:var(--chakra-colors-positive)');
  expect(cssFor(within(container).getByText('-2'))).toContain('color:var(--chakra-colors-negative)');
  expect(cssFor(within(container).getByText('0'))).toContain('color:var(--chakra-colors-text-muted)');
}

test(statDeltaHandlesBothSignsAndZero.name, statDeltaHandlesBothSignsAndZero);

function homeColoursResolveInBothModes() {
  function atPath(source: unknown, path: string): unknown {
    return path.split('.').reduce<unknown>((node, key) => node && typeof node === 'object' ? (node as Record<string, unknown>)[key] : undefined, source);
  }
  function resolve(path: string, mode: '_light' | '_dark'): unknown {
    const node = atPath(semanticTokens, path) ?? atPath({ colors }, path);
    const raw = atPath(node, 'value');
    const value = typeof raw === 'string' ? raw : atPath(raw, mode);
    const ref = typeof value === 'string' ? /^\{(.+)\}$/.exec(value) : null;
    return ref ? resolve(ref[1], mode) : value;
  }
  for (const mode of [ '_light', '_dark' ] as const) {
    for (const token of [ 'positive', 'negative', 'canvas', 'raised', 'bg.surface', 'text.primary', 'text.secondary', 'text.muted', 'border.divider' ]) {
      expect(resolve(`colors.${ token }`, mode), `${ token } ${ mode }`).toMatch(/^(#[0-9A-Fa-f]{6}|rgba\(.+\))$/);
    }
  }
}

test(homeColoursResolveInBothModes.name, homeColoursResolveInBothModes);

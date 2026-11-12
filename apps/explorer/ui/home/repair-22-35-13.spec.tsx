// @vitest-environment jsdom

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { parse } from 'dotenv';
import React from 'react';

import type { ChainIndicatorId } from 'types/homepage';

import config from 'configs/app';
import { hasHomepageChartDependencies, isHomepageChartId } from 'configs/app/homepageCharts';
import { parseEnvJson } from 'configs/app/utils';
import buildUrl from 'lib/api/buildUrl';
import { GENERAL_API_MISC_RESOURCES } from 'lib/api/services/general/misc';
import colors from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import { expect, test, vi } from 'vitest';
import { within } from 'vitest/lib';

import ChainIndicators from './indicators/ChainIndicators';
import useChartDataQuery from './indicators/useChartDataQuery';
import { createMarketQueryClient, paxMarket, renderMarketData, sidMarket } from './marketData.test-utils';
import Stats from './Stats';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL: 'PAX',
    NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL: 'SID',
    NEXT_PUBLIC_HOMEPAGE_CHARTS: '[\'daily_txs\',\'coin_price\',\'market_cap\',\'secondary_coin_price\',\'secondary_coin_market_cap\']',
    NEXT_PUBLIC_STATS_API_HOST: '',
  };
});

const marketIndicators = [ 'coin_price', 'market_cap', 'secondary_coin_price', 'secondary_coin_market_cap' ] as const;

function ChartResponseValues({ id }: { id: ChainIndicatorId }) {
  const query = useChartDataQuery(id, 14);
  return (
    <output data-series={ id }>
      { JSON.stringify(query.data[0].items.map(({ date, value }) => [ date.toISOString().slice(0, 10), value ])) }
    </output>
  );
}

function homeShowsFourMarketChartsBesideTransactions() {
  const container = renderMarketData(<>
    <Stats/>
    { marketIndicators.map((id) => <ChartResponseValues key={ id } id={ id }/>) }
  </>);
  const panels = container.querySelectorAll('[data-indicator]');
  expect(Array.from(panels, (panel) => panel.getAttribute('data-indicator'))).toEqual([ 'daily_txs', ...marketIndicators ]);
  const titles = [ 'Blockscout transaction history in 14 days', 'PAX price', 'PAX market value', 'SID price', 'SID market value' ];
  for (const [ index, panel ] of Array.from(panels).entries()) {
    expect(panel.querySelector('[data-title]')?.textContent).toBe(titles[index]);
    expect(panel.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points')).toBe('2');
    expect(panel.querySelector('[data-label="sparkline"]')).not.toBeNull();
    expect(panel.getAttribute('class')).toBe(panels[0].getAttribute('class'));
    expect(panel.parentElement).toBe(panels[0].parentElement);
  }
  const expected = {
    coin_price: [ [ '2026-01-01', 2 ], [ '2026-01-03', 2.125 ] ],
    market_cap: [ [ '2026-01-01', 200000000 ], [ '2026-01-03', 212500000 ] ],
    secondary_coin_price: [ [ '2026-01-01', 0.08 ], [ '2026-01-03', 0.075 ] ],
    secondary_coin_market_cap: [ [ '2026-01-01', 80000000 ], [ '2026-01-03', 75000000 ] ],
  };
  for (const id of marketIndicators) {
    expect(container.querySelector(`[data-series="${ id }"]`)?.textContent).toBe(JSON.stringify(expected[id]));
  }
}

test(homeShowsFourMarketChartsBesideTransactions.name, homeShowsFourMarketChartsBesideTransactions);

function emptyMarketSeriesShareTheTransactionChartEmptyState() {
  for (const chart_data of [ [], [ { date: '2026-01-01', closing_price: null, market_cap: null } ] ]) {
    const client = createMarketQueryClient({
      pax: { available_supply: paxMarket.available_supply, chart_data },
      sid: { available_supply: sidMarket.available_supply, chart_data },
      transactions: { chart_data: [] },
    });
    const container = renderMarketData(<Stats/>, client);
    const panels = container.querySelectorAll('[data-indicator]');
    expect(panels).toHaveLength(5);
    for (const panel of panels) {
      expect(within(panel as HTMLElement).getByLabelText('Network chart unavailable').textContent).toBe('—');
      expect(panel.querySelector('[data-label="sparkline"]')).toBeNull();
      expect(panel.querySelector('[data-label="chain-indicator-chart"]')?.getAttribute('data-points')).toBe('0');
    }
  }
}

test(emptyMarketSeriesShareTheTransactionChartEmptyState.name, emptyMarketSeriesShareTheTransactionChartEmptyState);

function secondaryMarketValueDoesNotBorrowThePriceOrNativeMarketValue() {
  const sid = {
    ...sidMarket,
    chart_data: [
      { date: '2026-01-01', closing_price: '0', market_cap: '0' },
      { date: '2026-01-02', closing_price: '0.075' },
      { date: '2026-01-03', closing_price: null, market_cap: '75000000' },
    ],
  };
  const container = renderMarketData(<>
    <ChartResponseValues id="secondary_coin_price"/>
    <ChartResponseValues id="secondary_coin_market_cap"/>
  </>, createMarketQueryClient({ sid }));
  expect(container.querySelector('[data-series="secondary_coin_price"]')?.textContent)
    .toBe(JSON.stringify([ [ '2026-01-01', 0 ], [ '2026-01-02', 0.075 ] ]));
  expect(container.querySelector('[data-series="secondary_coin_market_cap"]')?.textContent)
    .toBe(JSON.stringify([ [ '2026-01-01', 0 ], [ '2026-01-03', 75000000 ] ]));
}

test(secondaryMarketValueDoesNotBorrowThePriceOrNativeMarketValue.name, secondaryMarketValueDoesNotBorrowThePriceOrNativeMarketValue);

function secondaryMarketIndicatorUsesTheStatisticsFields() {
  const container = renderMarketData(<ChainIndicators/>);
  expect(container.textContent).toContain('SID market value');
  expect(container.textContent).toContain('$75M');
  expect(container.textContent).toContain('3.5%');
}

test(secondaryMarketIndicatorUsesTheStatisticsFields.name, secondaryMarketIndicatorUsesTheStatisticsFields);

function paxeerPresetEnablesBothCoinsAndMarketCharts() {
  const preset = parse(readFileSync(path.resolve(__dirname, '../../configs/envs/paxeer-x.env')));
  expect(preset.NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL).toBe('PAX');
  expect(preset.NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL).toBe('SID');
  expect(preset.NEXT_PUBLIC_HIDE_NATIVE_COIN_PRICE).toBe('false');
  const charts = parseEnvJson<Array<string>>(preset.NEXT_PUBLIC_HOMEPAGE_CHARTS.replaceAll('\'', '"'));
  expect(charts).toEqual([ 'daily_txs', ...marketIndicators ]);
  expect(charts?.every(isHomepageChartId)).toBe(true);
  expect(config.UI.homepage.charts).toEqual(charts);
  expect(isHomepageChartId('unrecognized_chart')).toBe(false);
  expect(hasHomepageChartDependencies([ 'daily_operational_txs' ])).toBe(false);
  expect(hasHomepageChartDependencies([ 'daily_operational_txs' ], 'https://localhost:3004')).toBe(true);
  expect(hasHomepageChartDependencies([ 'secondary_coin_market_cap' ])).toBe(true);
}

test(paxeerPresetEnablesBothCoinsAndMarketCharts.name, paxeerPresetEnablesBothCoinsAndMarketCharts);

function marketIndicatorsUseThePublishedMarketRoutes() {
  expect(GENERAL_API_MISC_RESOURCES.stats_charts_market.path).toBe('/api/v2/stats/charts/market');
  expect(GENERAL_API_MISC_RESOURCES.stats_charts_secondary_coin_market.path).toBe('/api/v2/stats/charts/secondary-coin-market');
  expect(new URL(buildUrl('general:stats_charts_market')).pathname).toBe('/api/v2/stats/charts/market');
  expect(new URL(buildUrl('general:stats_charts_secondary_coin_market')).pathname).toBe('/api/v2/stats/charts/secondary-coin-market');
}

test(marketIndicatorsUseThePublishedMarketRoutes.name, marketIndicatorsUseThePublishedMarketRoutes);

function marketColoursResolveInBothModes() {
  function atPath(source: unknown, key: string): unknown {
    return key.split('.').reduce<unknown>((node, part) => (
      node && typeof node === 'object' ? (node as Record<string, unknown>)[part] : undefined
    ), source);
  }
  function resolve(key: string, mode: '_light' | '_dark'): unknown {
    const node = atPath(semanticTokens, key) ?? atPath({ colors }, key);
    const raw = atPath(node, 'value') ?? atPath(node, 'DEFAULT.value');
    const value = typeof raw === 'string' ? raw : atPath(raw, mode);
    const reference = typeof value === 'string' ? /^\{(.+)\}$/.exec(value) : null;
    return reference ? resolve(reference[1], mode) : value;
  }
  for (const mode of [ '_light', '_dark' ] as const) {
    for (const token of [
      'accent', 'positive', 'negative', 'bg.surface', 'border.divider', 'text.muted', 'text.secondary',
      'chart.grid', 'chart.area.start', 'chart.area.stop',
    ]) {
      expect(resolve(`colors.${ token }`, mode), `${ token } ${ mode }`).toMatch(/^(#[0-9A-Fa-f]{6}|rgba\(.+\))$/);
    }
  }
}

test(marketColoursResolveInBothModes.name, marketColoursResolveInBothModes);

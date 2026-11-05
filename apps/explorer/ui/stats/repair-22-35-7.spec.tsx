// @vitest-environment jsdom

import React from 'react';

import type * as stats from '@blockscout/stats-types';

import { base as block } from 'mocks/blocks/block';
import { epochUnfinalized, list as epochs } from 'mocks/epochs/celo';
import { base as homeStats } from 'mocks/stats';
import * as blackfort from 'mocks/validators/blackfort';
import * as stability from 'mocks/validators/stability';
import * as zilliqa from 'mocks/validators/zilliqa';
import colors from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import EpochDetails from 'ui/epochs/EpochDetails';
import EpochsListItem from 'ui/epochs/EpochsListItem';
import EpochsTable from 'ui/epochs/EpochsTable';
import GasTrackerPrices from 'ui/gasTracker/GasTrackerPrices';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { CHART_COLOR_TOKENS } from 'ui/shared/chart/utils';
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { fireEvent, waitFor, within } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_VALIDATORS_CHAIN_TYPE: 'stability',
  };
});
vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import Epoch from 'ui/pages/Epoch';
import Stats from 'ui/pages/Stats';
import ValidatorsBlackfort from 'ui/pages/ValidatorsBlackfort';
import ValidatorsStability from 'ui/pages/ValidatorsStability';
import ValidatorsZilliqa from 'ui/pages/ValidatorsZilliqa';

import { getNetworkCounter, NETWORK_HERO_LABELS } from './NetworkStatsHero';

const headers = { 'Content-Type': 'application/json' };
const heroCounters: stats.Counters = {
  counters: [
    { id: 'tps', title: 'Transactions per second', value: '12.345', description: 'Measured transaction rate' },
    { id: 'active', title: 'Active validators', value: '42', description: 'Current active validators' },
    { id: 'height', title: 'Latest block', value: '30146364', description: 'Latest indexed block height' },
  ],
};
let counters = heroCounters;
let blackfortResponse: unknown = blackfort.validatorsResponse;
const requests: Array<string> = [];

beforeEach(() => {
  routerState.pathname = '/stats';
  routerState.query = {};
  counters = heroCounters;
  blackfortResponse = blackfort.validatorsResponse;
  requests.length = 0;
  fetchMock.resetMocks();
  fetchMock.mockResponse((request) => {
    const path = new URL(request.url).pathname;
    requests.push(request.url);
    const payloads: Record<string, unknown> = {
      '/api/v1/counters': counters,
      '/api/v1/lines': { sections: [] },
      '/api/v2/celo/epochs/1740': epochUnfinalized,
      '/api/v2/stats': { ...homeStats, celo: { epoch_number: 1739 } },
      '/api/v2/main-page/blocks': [ block ],
      '/api/v2/validators/blackfort': blackfortResponse,
      '/api/v2/validators/blackfort/counters': blackfort.validatorsCountersResponse,
      '/api/v2/validators/stability': stability.validatorsResponse,
      '/api/v2/validators/stability/counters': stability.validatorsCountersResponse,
      '/api/v2/validators/zilliqa': zilliqa.validatorsResponse,
    };
    return Promise.resolve({ body: JSON.stringify(payloads[path] ?? {}), headers });
  });
});

function heroValue(container: HTMLElement, label: string): string | null | undefined {
  return container.querySelector(`[data-network-stats-hero] [data-scan-stat="${ label }"] [data-value]`)?.textContent;
}

async function networkHeroUsesReturnedMeasurements() {
  const { container } = render(<Stats/>);
  await waitFor(() => {
    expect(heroValue(container, 'Transactions per second')).toBe('12.345');
    expect(heroValue(container, 'Current epoch')).toBe('1,739');
  });
  expect(Array.from(container.querySelectorAll('[data-network-stats-hero] [data-label]')).map((node) => node.textContent)).toEqual(NETWORK_HERO_LABELS);
  expect(heroValue(container, 'Active validators')).toBe('42');
  expect(heroValue(container, 'Current epoch')).toBe('1,739');
  expect(heroValue(container, 'Latest block')).toBe('30,146,364');
  expect(requests.some((url) => url.includes('/api/v1/counters'))).toBe(true);
  expect(requests.some((url) => url.includes('/api/v2/stats'))).toBe(true);
}

async function missingHeroMeasurementsRenderDash() {
  counters = { counters: [
    { id: 'totalBlocks', title: 'Total blocks', value: '40000000', description: 'Count is not a height' },
    { id: 'yesterdayTxns', title: 'Yesterday transactions', value: '86400', description: 'Daily count is not a current rate' },
  ] };
  const { container } = render(<Stats/>);
  await waitFor(() => expect(heroValue(container, 'Current epoch')).toBe('1,739'));
  await waitFor(() => expect(container.querySelector('[data-stats-number-widgets] [data-scan-stat="Total blocks"] [data-value]')?.textContent).toBe('40M'));
  expect(heroValue(container, 'Transactions per second')).toBe('—');
  expect(heroValue(container, 'Active validators')).toBe('—');
  expect(heroValue(container, 'Latest block')).toBe('—');
  expect(container.querySelector('[data-network-stats-hero] svg')).toBeNull();
  expect(getNetworkCounter([ { ...heroCounters.counters[0], value: '0' } ], 'Transactions per second')).toBe('0');
  expect(getNetworkCounter([ { ...heroCounters.counters[0], value: '' } ], 'Transactions per second')).toBeUndefined();
  expect(getNetworkCounter([ { ...heroCounters.counters[0], value: 'Infinity' } ], 'Transactions per second')).toBeUndefined();
}

async function recentBlocksUsesRealBlockFields() {
  const { container } = render(<Stats/>);
  const recent = container.querySelector('[data-network-recent-blocks]') as HTMLElement;
  await waitFor(() => expect(within(recent).getAllByRole('row')).toHaveLength(2));
  expect(within(recent).getByRole('heading', { name: 'Recent blocks' })).toBeTruthy();
  expect(within(recent).getByRole('link', { name: 'View all' }).getAttribute('href')).toBe('/blocks');
  expect(Array.from(recent.querySelectorAll('tbody td')).map((cell) => cell.textContent)).toEqual([
    expect.stringContaining(String(block.height)), expect.any(String), block.transactions_count.toLocaleString(), Number(block.gas_used).toLocaleString(),
  ]);
  expect(requests.some((url) => url.includes('/api/v2/main-page/blocks'))).toBe(true);
}

function tableHeaders(container: HTMLElement): Array<string | null> {
  return Array.from(container.querySelectorAll('thead th')).map((node) => node.textContent);
}

async function blackfortRendersAvailableStakeAndCommission() {
  routerState.pathname = '/validators';
  const { container } = render(<ValidatorsBlackfort/>);
  await waitFor(() => expect(container.querySelectorAll('tbody tr')).toHaveLength(blackfort.validatorsResponse.items.length));
  expect(tableHeaders(container)).toEqual([ 'Validator', 'Stake', 'Delegated amount', 'Commission' ]);
  const firstRow = container.querySelector('tbody tr') as HTMLElement;
  expect(firstRow.textContent).toContain(blackfort.validator1.name);
  expect(firstRow.textContent).toContain('0.1%');
  expect(requests.some((url) => url.includes('/api/v2/validators/blackfort'))).toBe(true);
  fireEvent.click(within(container.querySelector('thead') as HTMLElement).getByRole('button', { name: 'Validator' }));
  await waitFor(() => expect(requests.some((url) => url.includes('sort=address_hash'))).toBe(true));
}

async function missingValidatorFieldRemovesItsColumn() {
  blackfortResponse = {
    ...blackfort.validatorsResponse,
    items: blackfort.validatorsResponse.items.map(({ commission, ...item }) => item),
  };
  const { container } = render(<ValidatorsBlackfort/>);
  await waitFor(() => expect(container.querySelectorAll('tbody tr')).toHaveLength(blackfort.validatorsResponse.items.length));
  expect(tableHeaders(container)).toEqual([ 'Validator', 'Stake', 'Delegated amount' ]);
  expect(container.textContent).not.toContain('NaN');
  expect(container.querySelectorAll('tbody tr:first-child td')).toHaveLength(3);
}

async function stabilityDoesNotInventStakeRankOrCommission() {
  const { container } = render(<ValidatorsStability/>);
  await waitFor(() => expect(container.querySelectorAll('tbody tr')).toHaveLength(stability.validatorsResponse.items.length));
  expect(tableHeaders(container)).toEqual([ 'Validator', 'Status', 'Blocks' ]);
  expect(container.querySelector('tbody tr')?.textContent).toContain(stability.validator1.blocks_validated_count.toLocaleString());
  expect(container.querySelector('[data-scan-stat="Active validators"] [data-value]')?.textContent).toBe('42');
  expect(requests.some((url) => url.includes('/api/v2/validators/stability'))).toBe(true);
}

async function zilliqaUsesIndexWithoutInventingRankOrCommission() {
  const { container } = render(<ValidatorsZilliqa/>);
  await waitFor(() => expect(container.querySelectorAll('tbody tr')).toHaveLength(zilliqa.validatorsResponse.items.length));
  expect(tableHeaders(container)).toEqual([ 'Validator', 'Index', 'Stake' ]);
  expect(container.querySelector('tbody td:nth-child(2)')?.textContent).toBe(String(zilliqa.validator1.index));
  expect(container.querySelector('tbody td:nth-child(3)')?.textContent).toBe('1');
  expect(requests.some((url) => url.includes('/api/v2/validators/zilliqa'))).toBe(true);
}

function epochsKeepUnavailableTimingHonest() {
  const { container } = render(<EpochsTable items={ epochs.items } top={ 0 }/>);
  expect(tableHeaders(container)).toEqual([ 'Epoch', 'Status', 'Started', 'First block', 'Last block', 'Duration', 'Community ETH', 'Carbon offset ETH', 'Total ETH' ]);
  const cells = Array.from(container.querySelectorAll('tbody tr:first-child td'));
  expect(cells[2].textContent).toBe('—');
  expect(cells[3].textContent).toContain(String(epochs.items[0].start_block_number));
  expect(cells[4].textContent).toBe('—');
  expect(cells[5].textContent).toBe('—');
  expect(container.querySelector('tbody tr:nth-child(2) td:nth-child(5) a')?.getAttribute('href')).toBe(`/block/${ epochs.items[1].end_block_number }`);
}

function epochDetailsAndMobileUseLabelledGroups() {
  const { container } = render(<><EpochDetails data={ epochUnfinalized }/><EpochsListItem item={ epochs.items[0] }/></>);
  const group = container.querySelector('[data-section-group="Epoch info"]') as HTMLElement;
  expect(within(group).getByRole('heading', { name: 'Epoch info' })).toBeTruthy();
  expect(group.textContent).toContain('First block');
  expect(group.textContent).toContain('Duration');
  expect(group.querySelector('[data-detailed-info-value]')?.textContent).toContain('In progress');
  expect(container.textContent).not.toContain('NaN');
}

async function epochPageUsesEntityHeader() {
  routerState.pathname = '/epoch/[number]';
  routerState.query = { number: '1740' };
  const { container } = render(<Epoch/>);
  await waitFor(() => expect(container.querySelector('[data-section-group="Epoch info"]')?.textContent).toContain('Epochs are finalized approximately once a day'));
  expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('EPOCH');
  expect(container.querySelector('h1')?.textContent).toBe('Epoch #1740');
  expect(container.querySelector('[data-entity-identifier]')?.textContent).toContain('1740');
  expect(container.querySelector('[data-entity-tags]')?.textContent).toContain('In progress');
  expect(container.querySelector('[data-entity-tags]')?.textContent).toContain('L2');
}

function gasTilesAndTablePreserveZeroAndMissingValues() {
  const prices = {
    fast: { price: 0, fiat_price: '0', time: 0, base_fee: 0, priority_fee: 0 },
    average: { price: null, fiat_price: null, time: null, base_fee: null, priority_fee: null },
    slow: null,
  };
  const { container } = render(<GasTrackerPrices prices={ prices } isLoading={ false }/>);
  expect(Array.from(container.querySelectorAll('[data-scan-stat]')).map((node) => node.getAttribute('data-scan-stat'))).toEqual([ 'Fast', 'Normal' ]);
  expect(container.querySelector('[data-scan-stat="Fast"] [data-value]')?.textContent).toContain('0');
  expect(container.querySelector('[data-scan-stat="Normal"] [data-value]')?.textContent).toBe('—');
  expect(tableHeaders(container)).toEqual([ 'Speed', 'Base fee (Gwei)', 'Priority fee (Gwei)', 'Confirmation time (s)' ]);
  expect(Array.from(container.querySelectorAll('tbody tr:first-child td')).map((node) => node.textContent)).toEqual([ 'Fast', '0', '0', '0' ]);
  expect(Array.from(container.querySelectorAll('tbody tr:nth-child(2) td')).map((node) => node.textContent)).toEqual([ 'Normal', '—', '—', '—' ]);
}

function atPath(root: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => node !== null && typeof node === 'object' ? Reflect.get(node, key) : undefined, root);
}

function resolveColor(path: string, mode: '_light' | '_dark', seen = new Set<string>()): string {
  expect(seen.has(path), `Cyclic token ${ path }`).toBe(false);
  seen.add(path);
  const node = atPath(semanticTokens, path) ?? atPath({ colors }, path);
  expect(node, path).toBeTypeOf('object');
  const value = atPath(atPath(node, 'DEFAULT') ?? node, 'value');
  const resolved = typeof value === 'string' ? value : atPath(value, mode);
  expect(resolved, `${ path } ${ mode }`).toBeTypeOf('string');
  if (typeof resolved !== 'string') {
    throw new Error(`Missing colour ${ path } in ${ mode }`);
  }
  const reference = resolved.match(/^\{(.+)\}$/);
  return reference ? resolveColor(reference[1], mode, seen) : resolved;
}

function networkTokensResolveInBothModes() {
  for (const mode of [ '_light', '_dark' ] as const) {
    for (const token of [ 'canvas', 'surface', 'raised', 'hairline', 'text.primary', 'text.secondary', 'text.muted', 'accent' ]) {
      expect(resolveColor(`colors.${ token }`, mode)).toMatch(/^#|^rgba?\(/);
    }
    for (const tokens of Object.values(CHART_COLOR_TOKENS)) {
      const token = mode === '_light' ? tokens.light : tokens.dark;
      expect(resolveColor(`colors.${ token }`, mode)).toMatch(/^#|^rgba?\(/);
    }
    expect(resolveColor('colors.chart.grid', mode)).toBe(resolveColor('colors.hairline', mode));
    expect(resolveColor('colors.chart.area.start', mode)).toBe(mode === '_light' ? 'rgba(43,79,218,0.30)' : 'rgba(79,125,255,0.30)');
    expect(resolveColor('colors.chart.area.stop', mode)).toBe(mode === '_light' ? 'rgba(43,79,218,0)' : 'rgba(79,125,255,0)');
  }
}

describe('repair 22.35.7 — decision 2587 network pages', () => {
  test(networkHeroUsesReturnedMeasurements.name, networkHeroUsesReturnedMeasurements, 30_000);
  test(missingHeroMeasurementsRenderDash.name, missingHeroMeasurementsRenderDash, 30_000);
  test(recentBlocksUsesRealBlockFields.name, recentBlocksUsesRealBlockFields, 30_000);
  test(blackfortRendersAvailableStakeAndCommission.name, blackfortRendersAvailableStakeAndCommission, 30_000);
  test(missingValidatorFieldRemovesItsColumn.name, missingValidatorFieldRemovesItsColumn, 30_000);
  test(stabilityDoesNotInventStakeRankOrCommission.name, stabilityDoesNotInventStakeRankOrCommission, 30_000);
  test(zilliqaUsesIndexWithoutInventingRankOrCommission.name, zilliqaUsesIndexWithoutInventingRankOrCommission, 30_000);
  test(epochsKeepUnavailableTimingHonest.name, epochsKeepUnavailableTimingHonest);
  test(epochDetailsAndMobileUseLabelledGroups.name, epochDetailsAndMobileUseLabelledGroups);
  test(epochPageUsesEntityHeader.name, epochPageUsesEntityHeader, 30_000);
  test(gasTilesAndTablePreserveZeroAndMissingValues.name, gasTilesAndTablePreserveZeroAndMissingValues);
  test(networkTokensResolveInBothModes.name, networkTokensResolveInBothModes);
});

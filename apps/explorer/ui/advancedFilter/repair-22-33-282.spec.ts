// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { once } from 'node:events';
import { createServer } from 'node:http';
import React from 'react';

import type { AdvancedFilterParams, AdvancedFiltersSearchParams } from 'types/api/advancedFilter';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import dayjs from 'lib/date/dayjs';
import { baseResponse } from 'mocks/advancedFilter/advancedFilter';
import { base as stats } from 'mocks/stats';
import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { ADVANCED_FILTER_ITEM } from 'stubs/advancedFilter';
import { Provider } from 'toolkit/chakra/provider';
import 'ui/shared/layout/testWrapper';
import { afterAll, afterEach, beforeAll, expect, test, vi } from 'vitest';

import { getNativeToken } from './constants';
import ItemByColumn from './ItemByColumn';
import { AdvancedFilterInteractionFixture } from './repair-22-33-282.fixture';

vi.hoisted(async() => {
  const { 'default': dotenv } = await import('dotenv');
  const { parsed, error } = dotenv.config({ path: './configs/envs/.env.vitest' });
  if (error || !parsed) throw error || new Error('Explorer test environment is missing');
  window.__envs = parsed;
});

const selectedConfig: ClusterChainConfig['app_config'] = {
  ...config,
  chain: {
    ...config.chain,
    currency: { ...config.chain.currency, name: 'Selected currency', symbol: 'SEL', decimals: 6 },
  },
};
const otherConfig: ClusterChainConfig['app_config'] = {
  ...selectedConfig,
  chain: {
    ...selectedConfig.chain,
    currency: { ...selectedConfig.chain.currency, name: 'Other currency', symbol: 'OTHER', decimals: 3 },
  },
};

beforeAll(() => {
  fetchMock.disableMocks();
});
afterAll(() => {
  fetchMock.enableMocks();
});
afterEach(cleanup);

async function openTransport() {
  const requests: Array<URL> = [];
  const unexpected: Array<string> = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url || '/', 'http://localhost');
    requests.push(url);
    const document = (() => {
      switch (url.pathname) {
        case '/selected/api/v2/tokens':
        case '/other/api/v2/tokens':
          return { items: [ tokenInfo ], next_page_params: null };
        case '/selected/api/v2/advanced-filters/methods':
        case '/other/api/v2/advanced-filters/methods':
          return [ { method_id: '0x095ea7b3', name: 'approve' } ];
        case '/selected/api/v2/stats':
        case '/other/api/v2/stats':
          return stats;
        case '/selected/api/v2/advanced-filters':
        case '/other/api/v2/advanced-filters':
          return baseResponse;
        default:
          unexpected.push(url.pathname);
          response.statusCode = 404;
          return { error: 'Unexpected fixture path' };
      }
    })();
    response.setHeader('content-type', 'application/json');
    response.end(JSON.stringify(document));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a loopback TCP address');
  const { address: host, port } = address;
  const endpoint = `http://${ host }:${ port }`;
  function chain(appConfig = selectedConfig, slug = 'selected'): ClusterChainConfig {
    return {
      id: slug === 'selected' ? '125000001' : '125000002',
      name: slug === 'selected' ? 'Selected chain' : 'Other chain',
      slug,
      app_config: {
        ...appConfig,
        apis: {
          ...appConfig.apis,
          general: {
            endpoint,
            basePath: `/${ slug }`,
            host,
            port: String(port),
            protocol: 'http',
            socketEndpoint: `ws://${ host }:${ port }`,
          },
        },
      },
    };
  }
  return {
    requests,
    chain,
    close: async() => {
      await new Promise<void>((resolve, reject) => {
        server.close(error => error ? reject(error) : resolve());
        server.closeAllConnections();
      });
      expect(unexpected).toEqual([]);
    },
  };
}

function mountInteraction(
  transport: Awaited<ReturnType<typeof openTransport>>,
  column: React.ComponentProps<typeof AdvancedFilterInteractionFixture>['column'],
  initialFilters: AdvancedFilterParams = {},
  searchParams?: AdvancedFiltersSearchParams,
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function tree(chain: ClusterChainConfig) {
    return React.createElement(Provider, {},
      React.createElement(QueryClientProvider, { client },
        React.createElement(MultichainContext.Provider, { value: { chain } },
          React.createElement(AdvancedFilterInteractionFixture, { column, initialFilters, searchParams, chain }))));
  }
  const mounted = render(tree(transport.chain()));
  return {
    ...mounted,
    switchChain: () => mounted.rerender(tree(transport.chain(otherConfig, 'other'))),
    stop: () => {
      mounted.unmount();
      client.clear();
    },
  };
}

function appliedFilters(): AdvancedFilterParams {
  return JSON.parse(screen.getByTestId('applied-filters').textContent || '{}') as AdvancedFilterParams;
}

function filterButton() {
  const button = screen.getByRole('button', { name: /^close$/ });
  expect(button.textContent).toBe('Filter');
  return button;
}

async function applyFilter() {
  const button = filterButton();
  expect(button.hasAttribute('disabled')).toBe(false);
  fireEvent.click(button);
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
}

async function receivedFilter(transport: Awaited<ReturnType<typeof openTransport>>, expected: Record<string, string>, slug = 'selected') {
  await waitFor(() => {
    const requests = transport.requests.filter(url => url.pathname === `/${ slug }/api/v2/advanced-filters`);
    expect(requests.length).toBeGreaterThan(0);
    expect(Object.fromEntries(requests[requests.length - 1].searchParams)).toEqual(expected);
    expect(JSON.parse(screen.getByTestId('filter-response').textContent || 'null')).toEqual(baseResponse);
  });
}

async function customDatesSurviveClosingAndReopeningAndReachTheRequest() {
  const transport = await openTransport();
  const mounted = mountInteraction(transport, 'age');
  try {
    fireEvent.click(screen.getByRole('button', { name: 'filter by Age' }));
    fireEvent.change(await screen.findByPlaceholderText('From'), { target: { value: '2024-01-02' } });
    fireEvent.change(screen.getByPlaceholderText('To'), { target: { value: '2024-01-03' } });
    const expected = {
      age_from: dayjs('2024-01-02').startOf('day').toISOString(),
      age_to: dayjs('2024-01-03').endOf('day').toISOString(),
    };
    await applyFilter();
    expect(appliedFilters()).toEqual(expected);
    await receivedFilter(transport, expected);
    fireEvent.click(screen.getByRole('button', { name: 'filter by Age' }));
    expect((await screen.findByPlaceholderText('From') as HTMLInputElement).value).toBe('2024-01-02');
    expect((screen.getByPlaceholderText('To') as HTMLInputElement).value).toBe('2024-01-03');
    expect(filterButton().hasAttribute('disabled')).toBe(true);
    fireEvent.change(screen.getByPlaceholderText('To'), { target: { value: '2024-01-04' } });
    await applyFilter();
    const updated = { ...expected, age_to: dayjs('2024-01-04').endOf('day').toISOString() };
    expect(appliedFilters()).toEqual(updated);
    await receivedFilter(transport, updated);
  } finally {
    mounted.stop();
    await transport.close();
  }
}

async function assetSelectionAndDeselectionReachTheRequestAndReopenEmpty() {
  const transport = await openTransport();
  const mounted = mountInteraction(transport, 'asset', {}, { methods: {}, tokens: { [tokenInfo.address_hash]: tokenInfo } });
  try {
    fireEvent.click(screen.getByRole('button', { name: 'filter by Asset' }));
    const input = await screen.findByPlaceholderText('Token name or symbol');
    fireEvent.change(input, { target: { value: 'ARIA' } });
    const checkbox = await screen.findByRole('checkbox', { name: /ARIANEE/ });
    fireEvent.click(checkbox);
    expect((checkbox as HTMLInputElement).checked).toBe(true);
    await applyFilter();
    expect(appliedFilters()).toEqual({
      token_contract_address_hashes_to_include: [ tokenInfo.address_hash ],
      token_contract_symbols_to_include: [ 'ARIA' ],
      token_contract_address_hashes_to_exclude: [],
      token_contract_symbols_to_exclude: [],
    });
    await receivedFilter(transport, {
      token_contract_address_hashes_to_include: tokenInfo.address_hash,
      token_contract_symbols_to_include: 'ARIA',
      token_contract_address_hashes_to_exclude: '',
      token_contract_symbols_to_exclude: '',
    });
    fireEvent.click(screen.getByRole('button', { name: 'filter by Asset' }));
    fireEvent.change(await screen.findByPlaceholderText('Token name or symbol'), { target: { value: 'ARIA' } });
    const selected = await screen.findByRole('checkbox', { name: /ARIANEE/ });
    expect((selected as HTMLInputElement).checked).toBe(true);
    fireEvent.click(selected);
    expect((selected as HTMLInputElement).checked).toBe(false);
    await applyFilter();
    expect(appliedFilters()).toEqual({
      token_contract_address_hashes_to_include: [], token_contract_symbols_to_include: [],
      token_contract_address_hashes_to_exclude: [], token_contract_symbols_to_exclude: [],
    });
    await receivedFilter(transport, {
      token_contract_address_hashes_to_include: '', token_contract_symbols_to_include: '',
      token_contract_address_hashes_to_exclude: '', token_contract_symbols_to_exclude: '',
    });
    fireEvent.click(screen.getByRole('button', { name: 'filter by Asset' }));
    fireEvent.change(await screen.findByPlaceholderText('Token name or symbol'), { target: { value: 'ARIA' } });
    expect((await screen.findByRole('checkbox', { name: /ARIANEE/ }) as HTMLInputElement).checked).toBe(false);
  } finally {
    mounted.stop();
    await transport.close();
  }
}

async function savedMethodAbsentFromSearchResultsCanBeRemoved() {
  const transport = await openTransport();
  const mounted = mountInteraction(transport, 'method', {
    methods: [ '0xa9059cbb' ], methods_names: [ 'transfer' ],
  }, { methods: { '0xa9059cbb': 'transfer' }, tokens: {} });
  try {
    fireEvent.click(screen.getByRole('button', { name: 'filter by Method' }));
    const saved = await screen.findByRole('checkbox', { name: /transfer/ });
    expect((saved as HTMLInputElement).checked).toBe(true);
    expect(await screen.findByRole('checkbox', { name: /approve/ })).toBeDefined();
    expect(transport.requests.some(url => url.pathname.endsWith('/methods'))).toBe(true);
    fireEvent.click(saved);
    expect((saved as HTMLInputElement).checked).toBe(false);
    await applyFilter();
    expect(appliedFilters()).toEqual({ methods: [], methods_names: [] });
    await receivedFilter(transport, { methods: '', methods_names: '' });
    fireEvent.click(screen.getByRole('button', { name: 'filter by Method' }));
    await screen.findByRole('checkbox', { name: /approve/ });
    expect(screen.queryByRole('checkbox', { name: /transfer/ })).toBeNull();
  } finally {
    mounted.stop();
    await transport.close();
  }
}

function nativeAmountFeeAndAssetUseSelectedChainMetadata() {
  expect(selectedConfig.chain.currency.decimals).not.toBe(config.chain.currency.decimals);
  expect(selectedConfig.chain.currency.symbol).not.toBe(config.chain.currency.symbol);
  const item = { ...ADVANCED_FILTER_ITEM, value: '1234567', fee: '250000' };
  function cells(chainConfig?: ClusterChainConfig['app_config']) {
    return React.createElement(Provider, {},
      ...([ 'amount', 'fee', 'asset' ] as const).map(column => React.createElement('div', { key: column, 'data-testid': column },
        React.createElement(ItemByColumn, { item, column, chainConfig }))));
  }
  const mounted = render(cells(selectedConfig));
  expect(screen.getByTestId('amount').textContent).toBe('1.234567');
  expect(screen.getByTestId('fee').textContent).toBe('0.25');
  expect(screen.getByTestId('asset').textContent).toBe('SEL');
  mounted.rerender(cells(otherConfig));
  expect(screen.getByTestId('amount').textContent).toBe('1,234.567');
  expect(screen.getByTestId('fee').textContent).toBe('250');
  expect(screen.getByTestId('asset').textContent).toBe('OTHER');
  mounted.rerender(cells());
  expect(screen.getByTestId('asset').textContent).toBe(config.chain.currency.symbol);
}

async function switchingChainsRefreshesNativeChoicesAndPreservesIncludeExcludeSentinel() {
  for (const mode of [ 'include', 'exclude' ] as const) {
    const transport = await openTransport();
    const addressField = mode === 'include' ? 'token_contract_address_hashes_to_include' : 'token_contract_address_hashes_to_exclude';
    const symbolField = mode === 'include' ? 'token_contract_symbols_to_include' : 'token_contract_symbols_to_exclude';
    const mounted = mountInteraction(transport, 'asset', { [addressField]: [ 'native' ], [symbolField]: [ 'SEL' ] });
    try {
      fireEvent.click(screen.getByRole('button', { name: 'filter by Asset' }));
      await screen.findByText('Selected currency');
      expect(await screen.findByText('SEL', { selector: '[data-id="native"] *' })).toBeDefined();
      mounted.switchChain();
      await screen.findByText('Other currency');
      expect(screen.queryByText('Selected currency')).toBeNull();
      await screen.findByText('OTHER', { selector: '[data-id="native"] *' });
      fireEvent.click(screen.getByText('ARIA', { selector: '[data-id] *' }));
      await applyFilter();
      expect(appliedFilters()[addressField]).toEqual(mode === 'include' ? [ tokenInfo.address_hash, 'native' ] : [ 'native' ]);
      expect(appliedFilters()[symbolField]).toEqual(mode === 'include' ? [ 'ARIA', 'OTHER' ] : [ 'OTHER' ]);
      const expected = {
        token_contract_address_hashes_to_include: mode === 'include' ? `${ tokenInfo.address_hash },native` : tokenInfo.address_hash,
        token_contract_symbols_to_include: mode === 'include' ? 'ARIA,OTHER' : 'ARIA',
        token_contract_address_hashes_to_exclude: mode === 'exclude' ? 'native' : '',
        token_contract_symbols_to_exclude: mode === 'exclude' ? 'OTHER' : '',
      };
      await receivedFilter(transport, expected, 'other');
      fireEvent.click(screen.getByRole('button', { name: 'filter by Asset' }));
      await screen.findByText('Other currency');
      expect(appliedFilters()[addressField]).toContain('native');
    } finally {
      mounted.stop();
      await transport.close();
      cleanup();
    }
  }
  expect(getNativeToken(selectedConfig.chain.currency)).toMatchObject({ address_hash: 'native', name: 'Selected currency', symbol: 'SEL', decimals: '6' });
  expect(getNativeToken(otherConfig.chain.currency)).toMatchObject({ address_hash: 'native', name: 'Other currency', symbol: 'OTHER', decimals: '3' });
}

test('customDatesSurviveClosingAndReopeningAndReachTheRequest', async() => {
  expect.hasAssertions();
  await customDatesSurviveClosingAndReopeningAndReachTheRequest();
});
test('assetSelectionAndDeselectionReachTheRequestAndReopenEmpty', async() => {
  expect.hasAssertions();
  await assetSelectionAndDeselectionReachTheRequestAndReopenEmpty();
});
test('savedMethodAbsentFromSearchResultsCanBeRemoved', async() => {
  expect.hasAssertions();
  await savedMethodAbsentFromSearchResultsCanBeRemoved();
});
test('nativeAmountFeeAndAssetUseSelectedChainMetadata', async() => {
  expect.hasAssertions();
  await nativeAmountFeeAndAssetUseSelectedChainMetadata();
});
test('switchingChainsRefreshesNativeChoicesAndPreservesIncludeExcludeSentinel', async() => {
  expect.hasAssertions();
  await switchingChainsRefreshesNativeChoicesAndPreservesIncludeExcludeSentinel();
});

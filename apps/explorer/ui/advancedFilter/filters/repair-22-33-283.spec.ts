// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import React from 'react';

import type { AdvancedFilterMethodInfo, AdvancedFilterParams } from 'types/api/advancedFilter';

import buildUrl from 'lib/api/buildUrl';
import { getResourceKey } from 'lib/api/useApiQuery';
import dayjs from 'lib/date/dayjs';
import { tokenInfoERC20a, tokenInfoERC20b, tokenInfoERC20c } from 'mocks/tokens/tokenInfo';
import { PopoverContent, PopoverRoot } from 'toolkit/chakra/popover';
import { render } from 'ui/shared/layout/testWrapper';
import { test, expect } from 'vitest';

import AgeFilter from './AgeFilter';
import AssetFilter from './AssetFilter';
import MethodFilter from './MethodFilter';

function createQueryStore() {
  return new QueryClient({
    // These component peers are hydrated in-memory stores, with no remote data source.
    defaultOptions: { queries: { enabled: false, staleTime: Infinity, retry: false } },
  });
}

function renderFilter(filter: React.ReactElement, client = createQueryStore()) {
  return render(React.createElement(QueryClientProvider, { client },
    React.createElement(PopoverRoot, {
      defaultOpen: true,
      children: React.createElement(PopoverContent, { portalled: false }, filter),
    }),
  ));
}

async function applyFilter() {
  const button = screen.getByRole('button', { name: /^Filter$/ }) as HTMLButtonElement;
  expect(button.disabled).toBe(false);
  fireEvent.click(button);
  await waitFor(() => expect(screen.queryByRole('button', { name: /^Filter$/ })).toBeNull());
}

async function ageFilterRoundtrip() {
  const applied: Partial<Record<keyof AdvancedFilterParams, string | undefined>> = {};
  const handleFilterChange = (field: keyof AdvancedFilterParams, value?: string) => {
    applied[field] = value;
  };
  const first = renderFilter(React.createElement(AgeFilter, { columnName: 'Age', handleFilterChange }));

  fireEvent.change(screen.getByPlaceholderText('From'), { target: { value: '2024-03-09' } });
  fireEvent.change(screen.getByPlaceholderText('To'), { target: { value: '2024-03-11' } });
  await applyFilter();

  const request = new URL(buildUrl('general:advanced_filter', undefined, applied));
  const from = dayjs('2024-03-09').startOf('day').toISOString();
  const to = dayjs('2024-03-11').endOf('day').toISOString();
  expect(request.searchParams.get('age_from')).toBe(from);
  expect(request.searchParams.get('age_to')).toBe(to);
  expect(request.searchParams.has('age')).toBe(false);

  first.unmount();
  renderFilter(React.createElement(AgeFilter, {
    columnName: 'Age',
    handleFilterChange,
    value: { age: '', from: request.searchParams.get('age_from')!, to: request.searchParams.get('age_to')! },
  }));

  expect((screen.getByPlaceholderText('From') as HTMLInputElement).value).toBe('2024-03-09');
  expect((screen.getByPlaceholderText('To') as HTMLInputElement).value).toBe('2024-03-11');
  await applyFilter();
  const reapplied = new URL(buildUrl('general:advanced_filter', undefined, applied));
  expect(reapplied.search).toBe(request.search);
}

async function assetFilterUncheck() {
  const client = createQueryStore();
  const tokens = { items: [ tokenInfoERC20a, tokenInfoERC20b, tokenInfoERC20c ], next_page_params: null };
  client.setQueryData(getResourceKey('general:tokens', { queryParams: { limit: '7', q: '' } }), tokens);
  client.setQueryData(getResourceKey('general:tokens', { queryParams: { q: 'hyfi' } }), tokens);
  const applied: Record<string, Array<string>> = {};
  const handleFilterChange = (field: keyof AdvancedFilterParams, value: Array<string>) => {
    applied[field] = value;
  };
  renderFilter(React.createElement(AssetFilter, {
    columnName: 'Asset',
    value: [ { token: tokenInfoERC20b, mode: 'exclude' }, { token: tokenInfoERC20c, mode: 'include' } ],
    handleFilterChange,
  }), client);

  const search = screen.getByPlaceholderText('Token name or symbol');
  fireEvent.change(search, { target: { value: 'hyfi' } });
  const checkbox = await screen.findByRole('checkbox', { name: /hyfi.token/ }) as HTMLInputElement;
  expect((search as HTMLInputElement).value).toBe('hyfi');
  expect(checkbox.checked).toBe(false);
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox.checked).toBe(true));
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox.checked).toBe(false));
  // Leave another real selection changed so the production Apply control remains enabled.
  const included = screen.getByRole('checkbox', { name: /Ethereum/ }) as HTMLInputElement;
  expect(included.checked).toBe(true);
  fireEvent.click(included);
  await waitFor(() => expect(included.checked).toBe(false));
  await applyFilter();

  expect(applied.token_contract_address_hashes_to_include).toEqual([]);
  expect(applied.token_contract_symbols_to_include).toEqual([]);
  expect(applied.token_contract_address_hashes_to_exclude).toEqual([ tokenInfoERC20b.address_hash ]);
  expect(applied.token_contract_symbols_to_exclude).toEqual([ tokenInfoERC20b.symbol ]);
  const request = new URL(buildUrl('general:advanced_filter', undefined, applied));
  expect(request.searchParams.has('token_contract_address_hashes_to_include')).toBe(false);
  expect(request.searchParams.get('token_contract_address_hashes_to_exclude')).toBe(tokenInfoERC20b.address_hash);
}

async function assetFilterUncheckExcluded() {
  const client = createQueryStore();
  const tokens = { items: [ tokenInfoERC20b ], next_page_params: null };
  client.setQueryData(getResourceKey('general:tokens', { queryParams: { limit: '7', q: '' } }), tokens);
  client.setQueryData(getResourceKey('general:tokens', { queryParams: { q: 'USDC' } }), tokens);
  const applied: Record<string, Array<string>> = {};
  renderFilter(React.createElement(AssetFilter, {
    columnName: 'Asset',
    value: [ { token: tokenInfoERC20b, mode: 'exclude' } ],
    handleFilterChange: (field, value) => {
      applied[field] = value;
    },
  }), client);

  fireEvent.change(screen.getByPlaceholderText('Token name or symbol'), { target: { value: 'USDC' } });
  const checkbox = await screen.findByRole('checkbox', { name: /USD Coin/ }) as HTMLInputElement;
  expect(checkbox.checked).toBe(true);
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox.checked).toBe(false));
  await applyFilter();
  expect(applied.token_contract_address_hashes_to_include).toEqual([]);
  expect(applied.token_contract_address_hashes_to_exclude).toEqual([]);
}

async function methodFilterUncheck() {
  const client = createQueryStore();
  const saved: AdvancedFilterMethodInfo = { method_id: '0xa9059cbb', name: 'transfer' };
  const popular: AdvancedFilterMethodInfo = { method_id: '0x095ea7b3', name: 'approve' };
  const response = [ popular ];
  expect(response.some(method => method.method_id === saved.method_id)).toBe(false);
  client.setQueryData(getResourceKey('general:advanced_filter_methods', { queryParams: { q: '' } }), response);
  const applied: Record<string, Array<string>> = {};
  renderFilter(React.createElement(MethodFilter, {
    value: [ saved ],
    handleFilterChange: (field, value) => {
      applied[field] = value;
    },
  }), client);

  const checkbox = await screen.findByRole('checkbox', { name: /transfer/ }) as HTMLInputElement;
  expect(checkbox.checked).toBe(true);
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox.checked).toBe(false));
  // Addition must also work from the displayed saved list, independently of the query response.
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox.checked).toBe(true));
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox.checked).toBe(false));
  fireEvent.click(screen.getByRole('checkbox', { name: /approve/ }));
  await applyFilter();
  expect(applied.methods).toEqual([ popular.method_id ]);
  expect(applied.methods_names).toEqual([ popular.name ]);
  const request = new URL(buildUrl('general:advanced_filter', undefined, applied));
  expect(request.searchParams.get('methods')).toBe(popular.method_id);
  expect(request.searchParams.get('methods')).not.toContain(saved.method_id);
}

test(ageFilterRoundtrip);
test(assetFilterUncheck);
test(assetFilterUncheckExcluded);
test(methodFilterUncheck);

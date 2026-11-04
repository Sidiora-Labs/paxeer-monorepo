/* eslint-disable react/jsx-no-bind */
import React from 'react';

import dayjs from 'lib/date/dayjs';
import { baseResponse } from 'mocks/advancedFilter/advancedFilter';
import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { test, expect } from 'playwright/lib';
import type { ColumnsIds } from 'ui/advancedFilter/constants';

import FilterByColumn from './FilterByColumn';
import { AdvancedFilterInteractionFixture } from './repair-22-33-282.fixture';

const columns: Array<ColumnsIds> = [
  'type',
  'method',
  'age',
  'or_and',
  'from',
  'to',
  'amount',
  'asset',
];

const filters = {
  transaction_types: [ 'coin_transfer' as const ],
  methods: [ '0xa9059cbb' ],
  age: '7d' as const,
  address_relation: 'or' as const,
  from_address_hashes_to_include: [ '0x1230000000000000000000000000000000000000' ],
  to_address_hashes_to_include: [ '0x4560000000000000000000000000000000000000' ],
  amount_from: '100',
  token_contract_symbols_to_include: [ 'ETH' ],
  token_contract_address_hashes_to_include: [ 'native' ],
};

const searchParams = {
  methods: {
    '0xa9059cbb': 'transfer',
  },
  tokens: {},
};

for (const column of columns) {
  test(`${ column } filter +@dark-mode`, async({ page, render, mockApiResponse }) => {
    await mockApiResponse('general:tokens', {
      items: [],
      next_page_params: null,
    });
    await mockApiResponse('general:advanced_filter_methods', [], { queryParams: { q: '' } });
    await render(
      <FilterByColumn
        filters={ filters }
        searchParams={ searchParams }
        column={ column }
        columnName="Test"
        handleFilterChange={ () => {} }
      />,
    );

    const filterButton = page.locator('button');
    await filterButton.click();
    const popover = page.locator('.chakra-popover__content');
    await expect(popover).toBeVisible();
    await expect(popover).toHaveScreenshot();
  });
}

test('custom dates are applied, requested and recovered on reopening', async({ page, render, mockApiResponse }) => {
  await mockApiResponse('general:advanced_filter', baseResponse);
  const expected = {
    age_from: dayjs('2024-01-02').startOf('day').toISOString(),
    age_to: dayjs('2024-01-03').endOf('day').toISOString(),
  };
  const url = await mockApiResponse('general:advanced_filter', baseResponse, { queryParams: expected });
  await render(<AdvancedFilterInteractionFixture column="age"/>);
  await page.getByRole('button', { name: 'filter by Age' }).click();
  await page.getByPlaceholder('From', { exact: true }).fill('2024-01-02');
  await page.getByPlaceholder('To', { exact: true }).fill('2024-01-03');
  const request = page.waitForRequest(url);
  await page.getByRole('button', { name: 'close', exact: true }).filter({ hasText: /^Filter$/ }).click();
  expect(Object.fromEntries(new URL((await request).url()).searchParams)).toEqual(expected);
  await expect(page.getByTestId('applied-filters')).toHaveText(JSON.stringify(expected));
  await page.getByRole('button', { name: 'filter by Age' }).click();
  await expect(page.getByPlaceholder('From', { exact: true })).toHaveValue('2024-01-02');
  await expect(page.getByPlaceholder('To', { exact: true })).toHaveValue('2024-01-03');
  await expect(page.getByRole('button', { name: 'close', exact: true }).filter({ hasText: /^Filter$/ })).toBeDisabled();
});

test('asset deselection removes the applied address and request parameter value', async({ page, render, mockApiResponse }) => {
  const initialFilters = {
    token_contract_address_hashes_to_include: [ tokenInfo.address_hash ],
    token_contract_symbols_to_include: [ 'ARIA' ],
  };
  const emptyFilters = {
    token_contract_address_hashes_to_include: [],
    token_contract_symbols_to_include: [],
    token_contract_address_hashes_to_exclude: [],
    token_contract_symbols_to_exclude: [],
  };
  await mockApiResponse('general:advanced_filter', baseResponse, { queryParams: initialFilters });
  const url = await mockApiResponse('general:advanced_filter', baseResponse, { queryParams: emptyFilters });
  await mockApiResponse('general:tokens', { items: [ tokenInfo ], next_page_params: null }, { queryParams: { limit: '7', q: '' } });
  await mockApiResponse('general:tokens', { items: [ tokenInfo ], next_page_params: null }, { queryParams: { q: 'ARIA' } });
  await render(
    <AdvancedFilterInteractionFixture
      column="asset"
      initialFilters={ initialFilters }
      searchParams={{ methods: {}, tokens: { [tokenInfo.address_hash]: tokenInfo } }}
    />,
  );
  await page.getByRole('button', { name: 'filter by Asset' }).click();
  await page.getByPlaceholder('Token name or symbol').fill('ARIA');
  const checkbox = page.getByRole('checkbox', { name: /ARIANEE/ });
  await expect(checkbox).toBeChecked();
  await checkbox.uncheck();
  await expect(checkbox).not.toBeChecked();
  const request = page.waitForRequest(url);
  await page.getByRole('button', { name: 'close', exact: true }).filter({ hasText: /^Filter$/ }).click();
  expect(Object.fromEntries(new URL((await request).url()).searchParams)).toEqual({
    token_contract_address_hashes_to_include: '', token_contract_symbols_to_include: '',
    token_contract_address_hashes_to_exclude: '', token_contract_symbols_to_exclude: '',
  });
  await expect(page.getByTestId('applied-filters')).toHaveText(JSON.stringify(emptyFilters));
  await page.getByRole('button', { name: 'filter by Asset' }).click();
  await page.getByPlaceholder('Token name or symbol').fill('ARIA');
  await expect(checkbox).not.toBeChecked();
});

test('saved method absent from results is removed from applied filters and requests', async({ page, render, mockApiResponse }) => {
  const initialFilters = { methods: [ '0xa9059cbb' ], methods_names: [ 'transfer' ] };
  const emptyFilters = { methods: [], methods_names: [] };
  await mockApiResponse('general:advanced_filter', baseResponse, { queryParams: initialFilters });
  const url = await mockApiResponse('general:advanced_filter', baseResponse, { queryParams: emptyFilters });
  await mockApiResponse('general:advanced_filter_methods', [ { method_id: '0x095ea7b3', name: 'approve' } ], { queryParams: { q: '' } });
  await render(<AdvancedFilterInteractionFixture column="method" initialFilters={ initialFilters } searchParams={ searchParams }/>);
  await page.getByRole('button', { name: 'filter by Method' }).click();
  const checkbox = page.getByRole('checkbox', { name: /transfer/ });
  await expect(checkbox).toBeChecked();
  await checkbox.uncheck();
  await expect(checkbox).not.toBeChecked();
  const request = page.waitForRequest(url);
  await page.getByRole('button', { name: 'close', exact: true }).filter({ hasText: /^Filter$/ }).click();
  expect(Object.fromEntries(new URL((await request).url()).searchParams)).toEqual({ methods: '', methods_names: '' });
  await expect(page.getByTestId('applied-filters')).toHaveText(JSON.stringify(emptyFilters));
});

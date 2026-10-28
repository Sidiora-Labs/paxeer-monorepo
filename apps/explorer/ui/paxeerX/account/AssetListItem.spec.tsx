// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXBalance } from 'types/api/paxeerX';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { describe, expect, it, test } from 'vitest';
import { screen } from 'vitest/lib';

import AssetListItem from './AssetListItem';
import { render } from './testWrapper';

const tokenBalance = paxeerXMock.unifiedAccount.balances[1];
const custodyBalance = paxeerXMock.unifiedAccount.balances[0];

const renderItem = (item: typeof tokenBalance) => render(
  <TableRoot variant="scan">
    <TableBody>
      <AssetListItem item={ item }/>
    </TableBody>
  </TableRoot>,
);

function asset_row_renders_native_balance_without_parts() {
  const balance: PaxeerXBalance = {
    asset: paxeerXMock.tokenAsset,
    total: '2500000',
  };
  const { container } = renderItem(balance);

  expect(balance).not.toHaveProperty('parts');
  expect(container.querySelector('[data-label="total"]')?.textContent).toBe('2.5');
  expect(container.querySelector('td:first-child')?.textContent).toContain('USDX');
  expect(container.querySelectorAll('td')[1]?.textContent).toBe('USDX');
  expect(screen.queryByRole('button')).toBeNull();
  expect(container.querySelector('[data-parts-of]')).toBeNull();
  expect(container.querySelectorAll('tr')).toHaveLength(1);
}

describe('AssetListItem', () => {
  it('names the asset by its symbol and repeats the kernel asset id under it', () => {
    const { container } = renderItem(tokenBalance);

    const row = container.querySelector(`[data-asset="${ paxeerXMock.tokenAsset.id }"]`) as HTMLElement;

    expect(row).toBeTruthy();
    expect(row.querySelector('td:first-child')?.textContent).toContain(paxeerXMock.tokenAsset.symbol);
    expect(row.querySelector('[data-label="asset-id"]')?.textContent).toBe(paxeerXMock.tokenAsset.id);
  });

  it('leaves out the second line for an asset known by its id alone', () => {
    const { container } = renderItem(custodyBalance);

    expect(container.querySelector('[data-label="asset-id"]')).toBeNull();
    expect(container.querySelector('[data-label="total"]')?.textContent).toBe('1,200');
  });

  it('scales the total by the decimals of the asset', () => {
    const { container } = renderItem(tokenBalance);

    expect(container.querySelector('[data-label="total"]')?.textContent).toBe('2.5');
  });

  test(asset_row_renders_native_balance_without_parts);
});

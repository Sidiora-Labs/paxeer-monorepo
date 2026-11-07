// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXBalance } from 'types/api/paxeerX';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { describe, expect, it, test } from 'vitest';
import { screen } from 'vitest/lib';

import AssetListItem from './AssetListItem';
import { render } from './testWrapper';

const tokenBalance = paxeerXMock.unifiedAccount.balances.find(({ asset }) => asset.id === paxeerXMock.tokenAsset.id)!;
const nativeBalance = paxeerXMock.unifiedAccount.balances.find(({ asset }) => asset.id === paxeerXMock.nativeAsset.id)!;

const renderItem = (item: PaxeerXBalance) => render(
  <TableRoot variant="scan">
    <TableBody>
      <AssetListItem item={ item }/>
    </TableBody>
  </TableRoot>,
);

function assetItemFixtureMismatch() {
  const cases = [
    { balance: tokenBalance, symbol: 'USDX', total: '2.5' },
    { balance: nativeBalance, symbol: 'HPX', total: '1' },
  ];

  for (const { balance, symbol, total } of cases) {
    const { container, unmount } = renderItem(balance);
    const row = container.querySelector(`[data-asset="${ balance.asset.id }"]`);

    expect(row).toBeTruthy();
    expect(balance.asset.symbol).toBe(symbol);
    expect(row?.querySelector('td:first-child')?.textContent).toContain(symbol);
    expect(row?.querySelector('[data-label="asset-id"]')?.textContent).toBe(balance.asset.id);
    expect(row?.querySelectorAll('td')[1]?.textContent).toBe(balance.asset.denom);
    expect(row?.querySelector('[data-label="total"]')?.textContent).toBe(total);

    unmount();
  }
}

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

function asset_item_renders_total_without_parts() {
  const cases: Array<{ balance: PaxeerXBalance; expectedTotal: string }> = [
    { balance: { asset: paxeerXMock.tokenAsset, total: '1200' }, expectedTotal: '0.0012' },
    { balance: { asset: paxeerXMock.tokenAsset, total: '1234567890000' }, expectedTotal: '1,234,567.89' },
    { balance: { asset: paxeerXMock.nativeAsset, total: '1000000000000000000' }, expectedTotal: '1' },
  ];

  for (const { balance, expectedTotal } of cases) {
    const { container, unmount } = renderItem(balance);

    expect(container.querySelectorAll('[data-label="total"]')).toHaveLength(1);
    expect(container.querySelector('[data-label="total"]')?.textContent).toBe(expectedTotal);
    expect(container.querySelector('[data-parts-of]')).toBeNull();
    expect(container.querySelectorAll('tr')).toHaveLength(1);
    expect(container.querySelectorAll('td')).toHaveLength(3);
    expect(screen.queryByRole('button')).toBeNull();

    unmount();
  }
}

describe('AssetListItem', () => {
  it('names a native per-asset token balance by symbol and repeats its asset id under it', () => {
    const { container } = renderItem(tokenBalance);

    const row = container.querySelector(`[data-asset="${ paxeerXMock.tokenAsset.id }"]`) as HTMLElement;

    expect(row).toBeTruthy();
    expect(row.querySelector('td:first-child')?.textContent).toContain(paxeerXMock.tokenAsset.symbol);
    expect(row.querySelector('[data-label="asset-id"]')?.textContent).toBe(paxeerXMock.tokenAsset.id);
  });

  it('leaves out the second line for an asset known by its id alone', () => {
    const balance: PaxeerXBalance = {
      asset: { ...tokenBalance.asset, symbol: null, denom: tokenBalance.asset.id, decimals: null },
      total: '1200',
    };
    const { container } = renderItem(balance);

    expect(container.querySelector('[data-label="asset-id"]')).toBeNull();
    expect(container.querySelector('[data-label="total"]')?.textContent).toBe('1,200');
    expect(container.querySelector('td:first-child')?.textContent).toBe(balance.asset.id);
    expect(container.querySelectorAll('td')[1]?.textContent).toBe(balance.asset.denom);
  });

  it('scales a native per-asset token total by the selected asset decimals', () => {
    const { container } = renderItem(tokenBalance);

    expect(container.querySelector('[data-label="total"]')?.textContent).toBe('2.5');
  });

  test(asset_row_renders_native_balance_without_parts.name, asset_row_renders_native_balance_without_parts);
  test(asset_item_renders_total_without_parts.name, asset_item_renders_total_without_parts);
  test(assetItemFixtureMismatch.name, assetItemFixtureMismatch);
});

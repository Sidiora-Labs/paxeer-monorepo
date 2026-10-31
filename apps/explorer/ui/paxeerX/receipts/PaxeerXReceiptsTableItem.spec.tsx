// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveReceiptsItem } from 'types/api/paxeerXArchive';

import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXReceiptsTableItem from './PaxeerXReceiptsTableItem';

const receipt: PaxeerXArchiveReceiptsItem = {
  id: '0x1a2b3c4d5e6f7081928374655647382910ffeeddccbbaa99887766554433221100',
  account: '0x0000000000000000000000000000000000000000000000000000000000000000',
  frozen: true,
  block_number: 8341004,
};

const renderItem = (item: PaxeerXArchiveReceiptsItem = receipt) => render(
  <TableRoot variant="scan">
    <TableBody>
      <PaxeerXReceiptsTableItem item={ item }/>
    </TableBody>
  </TableRoot>,
);

function receipts_table_item_has_no_rung_badge() {
  const { container } = renderItem();
  const row = container.querySelector(`[data-receipt="${ receipt.id }"]`) as HTMLElement;

  expect(row).toBeTruthy();
  expect(row.querySelector(`a[href="/paxeer-x/receipts/${ receipt.id }"]`)).toBeTruthy();
  expect(row.querySelector(`a[href="/block/${ receipt.block_number }"]`)?.textContent).toBe(String(receipt.block_number));
  expect(row.querySelectorAll('td')).toHaveLength(3);
  expect(row.querySelector('[data-rung]')).toBeNull();
  expect(screen.queryByText('Settlement')).toBeNull();
}

describe('PaxeerXReceiptsTableItem', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it(receipts_table_item_has_no_rung_badge.name, receipts_table_item_has_no_rung_badge);

  it('marks the row with the receipt it carries and links it to its page', () => {
    const { container } = renderItem();

    const row = container.querySelector(`[data-receipt="${ receipt.id }"]`) as HTMLElement;

    expect(row).toBeTruthy();
    expect(row.querySelector(`a[href="/paxeer-x/receipts/${ receipt.id }"]`)).toBeTruthy();
  });

  it('offers the receipt and the account with a copy control each', () => {
    renderItem();

    expect(screen.getAllByLabelText('copy')).toHaveLength(2);
  });

  it('orders the cells as receipt, account and block', () => {
    const { container } = renderItem();

    const cells = container.querySelectorAll('td');

    expect(cells).toHaveLength(3);
    expect(cells[2]?.textContent).toBe(String(receipt.block_number));
  });

  it('marks an account the receipt log does not carry', () => {
    const { container } = renderItem({ ...receipt, account: null });

    expect(container.querySelectorAll('td')[1]?.textContent).toBe('—');
    expect(screen.getAllByLabelText('copy')).toHaveLength(1);
  });
});

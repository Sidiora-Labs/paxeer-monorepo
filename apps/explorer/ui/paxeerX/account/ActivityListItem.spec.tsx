// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXActivityItem } from 'types/api/paxeerX';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { describe, expect, it, test } from 'vitest';
import { screen } from 'vitest/lib';

import ActivityListItem from './ActivityListItem';
import { render } from './testWrapper';

const transfer = paxeerXMock.unifiedAccount.activity[1];

const renderItem = (item: typeof transfer) => render(
  <TableRoot variant="scan">
    <TableBody>
      <ActivityListItem item={ item }/>
    </TableBody>
  </TableRoot>,
);

function activity_item_renders_without_side() {
  const { container } = render(
    <TableRoot variant="scan">
      <TableBody>
        { paxeerXMock.unifiedAccount.activity.map((item) => <ActivityListItem key={ item.hash } item={ item }/>) }
      </TableBody>
    </TableRoot>,
  );

  expect(container.querySelectorAll('[data-activity]')).toHaveLength(2);

  for (const [ index, label ] of [ 'Token transfer', 'Transaction' ].entries()) {
    const item = paxeerXMock.unifiedAccount.activity[index];
    const row = container.querySelector(`[data-activity="${ item.hash }"]`);

    expect(item).not.toHaveProperty('side');
    expect(row).toBeTruthy();
    expect(row?.querySelectorAll('td')).toHaveLength(5);
    expect(row?.querySelector('td:first-child')?.textContent).toBe(label);
    expect(row?.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe(label);
  }

  expect(screen.queryByText('Chain')).toBeNull();
  expect(screen.queryByText('Kernel')).toBeNull();
}

function activity_row_renders_native_item_without_side() {
  const item: PaxeerXActivityItem = {
    kind: transfer.kind,
    hash: transfer.hash,
    block_number: transfer.block_number,
    ordinal: transfer.ordinal,
    status: transfer.status,
    timestamp: transfer.timestamp,
    asset: transfer.asset,
    amount: transfer.amount,
    counterparty: transfer.counterparty,
  };
  const { container } = renderItem(item);
  const row = container.querySelector(`[data-activity="${ item.hash }"]`) as HTMLElement;

  expect(item).not.toHaveProperty('side');
  expect(row).toBeTruthy();
  expect(row.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Token transfer');
  expect(row.querySelector('td:first-child')?.textContent).toBe('Token transfer');
  expect(screen.queryByText('Chain')).toBeNull();
  expect(screen.queryByText('Kernel')).toBeNull();
}

describe('ActivityListItem', () => {
  test(activity_item_renders_without_side.name, activity_item_renders_without_side);
  test(activity_row_renders_native_item_without_side);

  it('links the transaction and the block it sits in', () => {
    const { container } = renderItem(transfer);

    expect(container.querySelector(`a[href="/tx/${ transfer.hash }"]`)).toBeTruthy();
    expect(container.querySelector(`a[href="/block/${ transfer.block_number }"]`)?.textContent).toBe(String(transfer.block_number));
  });

  it('puts the row on its rung of the settlement ladder', () => {
    const { container } = renderItem(transfer);

    expect(container.querySelector(`[data-rung="${ transfer.status }"]`)).toBeTruthy();
  });

  it('scales the amount by the asset and marks an entry that carries none', () => {
    renderItem(transfer);

    expect(screen.getByText('2.5 USDX')).toBeTruthy();

    const { container } = renderItem({ ...transfer, amount: null, asset: null });

    expect(container.querySelectorAll('[data-activity] td')[3]?.textContent).toBe('—');
  });
});

// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXActivityItem } from 'types/api/paxeerX';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { describe, expect, it, test } from 'vitest';
import { screen } from 'vitest/lib';

import ActivityListItem from './ActivityListItem';
import { render } from './testWrapper';

const transaction = paxeerXMock.unifiedAccount.activity[1];

const renderItem = (item: PaxeerXActivityItem) => render(
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

  for (const [ index, label ] of [ 'Transaction', 'Transaction' ].entries()) {
    const item = paxeerXMock.unifiedAccount.activity[index];
    const row = container.querySelector(`[data-activity="${ item.hash }"]`);

    expect(item).not.toHaveProperty('side');
    expect(row).toBeTruthy();
    expect(row?.getAttribute('data-kind')).toBe(item.kind);
    expect(row?.querySelectorAll('td')).toHaveLength(5);
    expect(row?.querySelector('td:first-child')?.textContent).toBe(label);
    expect(row?.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe(label);
  }

  expect(screen.queryByText('Chain')).toBeNull();
  expect(screen.queryByText('Kernel')).toBeNull();
}

function activity_row_renders_native_item_without_side() {
  const item: PaxeerXActivityItem = {
    kind: transaction.kind,
    hash: transaction.hash,
    block_number: transaction.block_number,
    ordinal: transaction.ordinal,
    status: transaction.status,
    timestamp: transaction.timestamp,
    asset: transaction.asset,
    amount: transaction.amount,
    counterparty: transaction.counterparty,
  };
  const { container } = renderItem(item);
  const row = container.querySelector(`[data-activity="${ item.hash }"]`) as HTMLElement;

  expect(item).not.toHaveProperty('side');
  expect(row).toBeTruthy();
  expect(row.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Transaction');
  expect(row.querySelector('td:first-child')?.textContent).toBe('Transaction');
  expect(screen.queryByText('Chain')).toBeNull();
  expect(screen.queryByText('Kernel')).toBeNull();
}

describe('ActivityListItem', () => {
  test(activity_item_renders_without_side.name, activity_item_renders_without_side);
  test(activity_row_renders_native_item_without_side);

  it('preserves the label and amount of a token transfer entry', () => {
    const tokenTransfer: PaxeerXActivityItem = {
      ...transaction,
      kind: 'token_transfer',
      asset: paxeerXMock.tokenAsset,
      amount: '2500000',
    };
    const { container } = renderItem(tokenTransfer);
    const row = container.querySelector('[data-activity]');

    expect(row?.getAttribute('data-kind')).toBe('token_transfer');
    expect(row?.querySelector('td:first-child')?.textContent).toBe('Token transfer');
    expect(row?.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Token transfer');
    expect(screen.getByText('2.5 USDX')).toBeTruthy();
  });

  it('links the transaction and the block it sits in', () => {
    const { container } = renderItem(transaction);

    expect(container.querySelector(`a[href="/tx/${ transaction.hash }"]`)).toBeTruthy();
    expect(container.querySelector(`a[href="/block/${ transaction.block_number }"]`)?.textContent).toBe(String(transaction.block_number));
  });

  it('puts the row on its rung of the settlement ladder', () => {
    const { container } = renderItem(transaction);

    expect(container.querySelector(`[data-rung="${ transaction.status }"]`)?.textContent).toBe('Committed');
  });

  it('scales the amount by the asset and marks an entry that carries none', () => {
    renderItem(transaction);

    expect(screen.getByText('1.5 HPX')).toBeTruthy();

    const { container } = renderItem({ ...transaction, amount: null, asset: null });

    expect(container.querySelectorAll('[data-activity] td')[3]?.textContent).toBe('—');
  });
});

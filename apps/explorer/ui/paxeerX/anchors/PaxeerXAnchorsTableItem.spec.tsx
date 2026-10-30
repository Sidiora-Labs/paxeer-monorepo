// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveAnchor } from 'types/api/paxeerXArchive';

import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, within } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXAnchorsTableItem, { ANCHOR_SETTLEMENT_RUNG } from './PaxeerXAnchorsTableItem';

const anchor: PaxeerXArchiveAnchor = {
  frozen: true,
  batch_number: 1042,
  checkpoint_id: '0x1a2b3c4d5e6f7081928374655647382910ffeeddccbbaa99887766554433221100',
  checkpoint_height: 8340992,
  sealed_height: 8340960,
  state_root: '0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789',
  block_number: 8341004,
  timestamp: '2023-05-22T18:00:36.000000Z',
};

const renderItem = (item: PaxeerXArchiveAnchor = anchor) => render(
  <TableRoot variant="scan">
    <TableBody>
      <PaxeerXAnchorsTableItem item={ item }/>
    </TableBody>
  </TableRoot>,
);

function anchors_table_item_renders_archive_row() {
  const { container } = renderItem();
  const row = container.querySelector(`[data-anchor="${ anchor.checkpoint_id }"]`) as HTMLElement;

  expect(row).toBeTruthy();
  expect(row.getAttribute('data-anchor')).toBe(anchor.checkpoint_id);
  expect(within(row).getByText(String(anchor.batch_number))).toBeTruthy();
  expect(row.querySelector('[data-label="checkpoint-height"]')?.textContent).toBe(String(anchor.checkpoint_height));
  expect(row.querySelector('[data-label="sealed-height"]')?.textContent).toBe(String(anchor.sealed_height));
  expect(row.querySelector(`a[href="/block/${ anchor.block_number }"]`)?.textContent).toBe(String(anchor.block_number));
}

describe('PaxeerXAnchorsTableItem', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it(anchors_table_item_renders_archive_row.name, anchors_table_item_renders_archive_row);

  it('marks the row with the checkpoint it carries', () => {
    const { container } = renderItem();

    expect(container.querySelector(`[data-anchor="${ anchor.checkpoint_id }"]`)).toBeTruthy();
  });

  it('offers the state root with a copy control', () => {
    const { container } = renderItem();

    expect(screen.getAllByLabelText('copy')).toHaveLength(1);
    expect(container.querySelectorAll('td')[3]?.textContent).not.toBe('—');
  });

  it('settles the row on the rung the ladder spells for a submitted checkpoint', () => {
    const { container } = renderItem();

    expect(ANCHOR_SETTLEMENT_RUNG).toBe('receipted');
    expect(container.querySelector(`[data-rung="${ ANCHOR_SETTLEMENT_RUNG }"]`)).toBeTruthy();
  });

  it('drops the copy control when the log carries no state root', () => {
    const { container } = renderItem({ ...anchor, state_root: null });

    expect(screen.queryByLabelText('copy')).toBeNull();
    expect(container.querySelectorAll('td')[3]?.textContent).toBe('—');
  });
});

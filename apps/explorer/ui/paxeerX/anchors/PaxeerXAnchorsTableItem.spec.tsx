// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveAnchor } from 'types/api/paxeerXArchive';

import { PAXEER_X_ANCHORS_ITEM } from 'stubs/paxeerXLists';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, within } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXAnchorsTableItem, { ANCHOR_SETTLEMENT_RUNG } from './PaxeerXAnchorsTableItem';

const anchor = PAXEER_X_ANCHORS_ITEM;

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
  const checkpointCell = within(row).getAllByRole('cell')[1];
  const checkpointText = within(checkpointCell).getByText('0x3ed9d81e7c...6967');
  expect(window.getComputedStyle(checkpointText).visibility).toBe('visible');
  expect(within(checkpointCell).getByRole('button', { name: 'copy' })).toBeTruthy();
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

    expect(screen.getAllByLabelText('copy')).toHaveLength(2);
    const stateRootCell = container.querySelectorAll('td')[4];
    expect(within(stateRootCell).getByRole('button', { name: 'copy' })).toBeTruthy();
    expect(stateRootCell?.textContent).not.toBe('—');
  });

  it('settles the row on the rung the ladder spells for a submitted checkpoint', () => {
    const { container } = renderItem();

    expect(ANCHOR_SETTLEMENT_RUNG).toBe('receipted');
    expect(container.querySelector(`[data-rung="${ ANCHOR_SETTLEMENT_RUNG }"]`)).toBeTruthy();
  });

  it('drops the copy control when the log carries no state root', () => {
    const { container } = renderItem({ ...anchor, state_root: null });

    expect(screen.getAllByLabelText('copy')).toHaveLength(1);
    const stateRootCell = container.querySelectorAll('td')[4];
    expect(within(stateRootCell).queryByLabelText('copy')).toBeNull();
    expect(stateRootCell?.textContent).toBe('—');
  });
});

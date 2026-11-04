// @vitest-environment jsdom

import React from 'react';

import { PAXEER_X_ANCHORS_ITEM } from 'stubs/paxeerXLists';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import { render } from 'ui/shared/layout/testWrapper';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { fireEvent, waitFor, within } from 'vitest/lib';

import PaxeerXAnchorsTable from './PaxeerXAnchorsTable';
import PaxeerXAnchorsTableItem from './PaxeerXAnchorsTableItem';

class InMemoryClipboard implements Pick<Clipboard, 'writeText' | 'readText'> {
  private text = '';

  async writeText(text: string) {
    this.text = text;
  }

  async readText() {
    return this.text;
  }
}

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: new InMemoryClipboard() });
});

afterEach(() => {
  if (originalClipboard) {
    Object.defineProperty(navigator, 'clipboard', originalClipboard);
  } else {
    Reflect.deleteProperty(navigator, 'clipboard');
  }
});

async function anchor_table_id_test_visible_archive_checkpoint_and_copy() {
  const anchor = PAXEER_X_ANCHORS_ITEM;
  const { container } = render(React.createElement(TableRoot, { variant: 'scan' },
    React.createElement(TableBody, null, React.createElement(PaxeerXAnchorsTableItem, { item: anchor })),
  ));
  const row = within(container).getByRole('row');
  const cells = within(row).getAllByRole('cell');

  expect(cells).toHaveLength(8);
  expect(cells[0].textContent).toBe(String(anchor.batch_number));
  const identifier = within(cells[1]).getByText('0x3ed9d81e7c...6967');
  expect(window.getComputedStyle(identifier).visibility).toBe('visible');
  const copy = within(cells[1]).getByRole('button', { name: 'copy' });

  fireEvent.click(copy);

  await waitFor(async() => {
    expect(await navigator.clipboard.readText()).toBe(anchor.checkpoint_id);
  });
  expect(copy.querySelector('use')?.getAttribute('href')?.endsWith('#copy_check')).toBe(true);
}

async function anchor_table_id_invisible_table_wires_checkpoint_and_retains_batch() {
  const anchor = { ...PAXEER_X_ANCHORS_ITEM, state_root: null };
  const { container } = render(React.createElement(PaxeerXAnchorsTable, { items: [ anchor ] }));
  const table = within(container).getByRole('table');
  const headers = within(table).getAllByRole('columnheader');

  expect(headers.map((header) => header.textContent)).toEqual([
    'Batch', 'Checkpoint ID', 'Checkpoint height', 'Sealed height', 'State root', 'Block', 'Age', 'Settlement',
  ]);
  const row = within(table).getAllByRole('row')[1];
  const cells = within(row).getAllByRole('cell');

  expect(cells).toHaveLength(headers.length);
  expect(cells[0].textContent).toBe(String(anchor.batch_number));
  expect(within(cells[1]).getByText('0x3ed9d81e7c...6967')).toBeTruthy();
  expect(cells[4].textContent).toBe('—');
  expect(within(cells[4]).queryByRole('button', { name: 'copy' })).toBeNull();
  expect(within(cells[5]).getByRole('link').getAttribute('href')).toBe(`/block/${ anchor.block_number }`);
  expect(within(row).getAllByRole('button', { name: 'copy' })).toHaveLength(1);

  fireEvent.click(within(cells[1]).getByRole('button', { name: 'copy' }));

  await waitFor(async() => {
    expect(await navigator.clipboard.readText()).toBe(anchor.checkpoint_id);
  });
}

async function anchor_table_id_loading_keeps_identifier_and_copy_in_skeletons() {
  const { container, rerender } = render(React.createElement(PaxeerXAnchorsTable, {
    items: [ PAXEER_X_ANCHORS_ITEM ],
    isLoading: true,
  }));
  const cells = container.querySelectorAll('tbody td');

  expect(cells).toHaveLength(8);
  expect(cells[0].querySelector('[data-loading]')).toBeTruthy();
  expect(cells[1].querySelector('[data-loading]')).toBeTruthy();
  const copy = cells[1].querySelector('[data-loading-skeleton]');
  expect(copy?.getAttribute('aria-label')).toBe('copy');
  expect(copy?.getAttribute('data-loading')).toBe('true');
  expect(window.getComputedStyle(copy as Element).pointerEvents).toBe('none');
  expect(copy?.hasAttribute('disabled')).toBe(true);

  rerender(React.createElement(PaxeerXAnchorsTable, {
    items: [ PAXEER_X_ANCHORS_ITEM ],
    isLoading: false,
  }));

  const checkpointCell = container.querySelectorAll('tbody td')[1];
  expect(checkpointCell.querySelector('[data-loading-skeleton]')).toBeNull();
  const identifier = within(checkpointCell as HTMLElement).getByText('0x3ed9d81e7c...6967');
  expect(window.getComputedStyle(identifier).visibility).toBe('visible');
  const loadedCopy = within(checkpointCell as HTMLElement).getByRole('button', { name: 'copy' });
  expect(loadedCopy.hasAttribute('disabled')).toBe(false);

  fireEvent.click(loadedCopy);

  await waitFor(async() => {
    expect(await navigator.clipboard.readText()).toBe(PAXEER_X_ANCHORS_ITEM.checkpoint_id);
  });
}

test(anchor_table_id_test_visible_archive_checkpoint_and_copy.name, anchor_table_id_test_visible_archive_checkpoint_and_copy);
test(anchor_table_id_invisible_table_wires_checkpoint_and_retains_batch.name, anchor_table_id_invisible_table_wires_checkpoint_and_retains_batch);
test(anchor_table_id_loading_keeps_identifier_and_copy_in_skeletons.name, anchor_table_id_loading_keeps_identifier_and_copy_in_skeletons);

// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveReceiptsItem } from 'types/api/paxeerXArchive';

import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXReceiptsTable from './PaxeerXReceiptsTable';

const receipt: PaxeerXArchiveReceiptsItem = {
  id: '0x1a2b3c4d5e6f7081928374655647382910ffeeddccbbaa99887766554433221100',
  account: '0x0000000000000000000000000000000000000000000000000000000000000000',
  frozen: true,
  block_number: 8341004,
};

const accountless: PaxeerXArchiveReceiptsItem = {
  ...receipt,
  id: '0x11223344556677889900aabbccddeeff0f1e2d3c4b5a69788796a5b4c3d2e1f0',
  account: null,
  block_number: receipt.block_number - 1,
};

const items = [ receipt, accountless ];

const renderTable = () => render(<PaxeerXReceiptsTable items={ items }/>);

describe('PaxeerXReceiptsTable', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('heads the table with the receipt columns in order', () => {
    const { container } = renderTable();

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent))
      .toEqual([ 'Receipt ID', 'Account', 'Block' ]);
  });

  it('renders one row per receipt, each marked by its id', () => {
    const { container } = renderTable();

    expect(container.querySelector('[data-label="paxeer-x-receipts"]')).toBeTruthy();
    expect(Array.from(container.querySelectorAll('[data-receipt]')).map((row) => row.getAttribute('data-receipt')))
      .toEqual(items.map((item) => item.id));
  });

  it('links a receipt id to its own page and the block to the block page', () => {
    const { container } = renderTable();

    const row = container.querySelector(`[data-receipt="${ receipt.id }"]`) as HTMLElement;

    expect(row.querySelector(`a[href="/paxeer-x/receipts/${ receipt.id }"]`)).toBeTruthy();
    expect(row.querySelector(`a[href="/block/${ receipt.block_number }"]`)?.textContent)
      .toBe(String(receipt.block_number));
  });

  it('marks a receipt the log carries without an account', () => {
    const { container } = renderTable();

    const row = container.querySelector(`[data-receipt="${ accountless.id }"]`) as HTMLElement;

    expect(row.querySelectorAll('td')[1]?.textContent).toBe('—');
  });

  it('renders archive rows without settlement badges', () => {
    const { container } = renderTable();

    expect(container.querySelectorAll('[data-receipt] [data-rung]')).toHaveLength(0);
    expect(container.querySelectorAll('tbody tr td')).toHaveLength(items.length * 3);
  });
});

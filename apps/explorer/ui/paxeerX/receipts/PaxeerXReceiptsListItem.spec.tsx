// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveReceiptsItem } from 'types/api/paxeerXArchive';

import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXReceiptsListItem from './PaxeerXReceiptsListItem';

const receipt: PaxeerXArchiveReceiptsItem = {
  id: '0x1a2b3c4d5e6f7081928374655647382910ffeeddccbbaa99887766554433221100',
  account: '0x0000000000000000000000000000000000000000000000000000000000000000',
  frozen: true,
  block_number: 8341004,
};

const renderItem = (item: PaxeerXArchiveReceiptsItem = receipt) => render(<PaxeerXReceiptsListItem item={ item }/>);

describe('PaxeerXReceiptsListItem', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('carries the same fields as the table row, one label per line', () => {
    renderItem();

    expect(screen.getByText('Receipt ID')).toBeTruthy();
    expect(screen.getByText('Account')).toBeTruthy();
    expect(screen.getByText('Block')).toBeTruthy();
  });

  it('links the receipt id to its page and the block to the block page', () => {
    const { container } = renderItem();

    expect(container.querySelector(`a[href="/paxeer-x/receipts/${ receipt.id }"]`)).toBeTruthy();
    expect(container.querySelector(`a[href="/block/${ receipt.block_number }"]`)?.textContent)
      .toBe(String(receipt.block_number));
  });

  it('renders the archive entry without a settlement badge', () => {
    const { container } = renderItem();

    expect(container.querySelector('[data-rung]')).toBeNull();
    expect(screen.queryByText('Settlement')).toBeNull();
  });

  it('marks an account the receipt log does not carry', () => {
    renderItem({ ...receipt, account: null });

    expect(screen.getByText('—')).toBeTruthy();
    expect(screen.getAllByLabelText('copy')).toHaveLength(1);
  });
});

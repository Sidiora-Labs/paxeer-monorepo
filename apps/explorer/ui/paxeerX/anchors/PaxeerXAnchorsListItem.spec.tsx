// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveAnchor } from 'types/api/paxeerXArchive';

import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXAnchorsListItem from './PaxeerXAnchorsListItem';
import { ANCHOR_SETTLEMENT_RUNG } from './PaxeerXAnchorsTableItem';

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

const renderItem = (item: PaxeerXArchiveAnchor = anchor) => render(<PaxeerXAnchorsListItem item={ item }/>);

describe('PaxeerXAnchorsListItem', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('carries the same fields as the table row, one label per line', () => {
    renderItem();

    expect(screen.getByText('Checkpoint height')).toBeTruthy();
    expect(screen.getByText('Sealed height')).toBeTruthy();
    expect(screen.getByText('State root')).toBeTruthy();
    expect(screen.getByText('Block')).toBeTruthy();
    expect(screen.getByText('Age')).toBeTruthy();
    expect(screen.getByText('Settlement')).toBeTruthy();
  });

  it('spells the heights and links the block', () => {
    const { container } = renderItem();

    expect(container.querySelector('[data-label="checkpoint-height"]')?.textContent).toBe(String(anchor.checkpoint_height));
    expect(container.querySelector('[data-label="sealed-height"]')?.textContent).toBe(String(anchor.sealed_height));
    expect(container.querySelector(`a[href="/block/${ anchor.block_number }"]`)).toBeTruthy();
  });

  it('settles the entry on the same rung the table row uses', () => {
    const { container } = renderItem();

    expect(container.querySelector(`[data-rung="${ ANCHOR_SETTLEMENT_RUNG }"]`)).toBeTruthy();
  });

  it('marks a checkpoint the log left without heights or a state root', () => {
    const { container } = renderItem({
      ...anchor,
      checkpoint_height: null,
      sealed_height: null,
      state_root: null,
    });

    expect(container.querySelector('[data-label="checkpoint-height"]')?.textContent).toBe('—');
    expect(container.querySelector('[data-label="sealed-height"]')?.textContent).toBe('—');
    expect(screen.queryByLabelText('copy')).toBeNull();
  });
});

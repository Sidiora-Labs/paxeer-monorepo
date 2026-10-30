// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXArchiveAnchor } from 'types/api/paxeerXArchive';

import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXAnchorsTable from './PaxeerXAnchorsTable';
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

const sparse: PaxeerXArchiveAnchor = {
  ...anchor,
  batch_number: anchor.batch_number - 1,
  checkpoint_id: '0x4b3c2d1e0f9a8b7c6d5e4f30211223344556677889900aabbccddeeff00112233',
  checkpoint_height: null,
  sealed_height: null,
  state_root: null,
  block_number: anchor.block_number - 1,
};

const items = [ anchor, sparse ];

const renderTable = () => render(<PaxeerXAnchorsTable items={ items }/>);

describe('PaxeerXAnchorsTable', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('heads the table with the anchor columns in order', () => {
    const { container } = renderTable();

    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent))
      .toEqual([ 'Batch', 'Checkpoint height', 'Sealed height', 'State root', 'Block', 'Age', 'Settlement' ]);
  });

  it('renders one row per checkpoint, each marked by its checkpoint id', () => {
    const { container } = renderTable();

    expect(container.querySelector('[data-label="paxeer-x-anchors"]')).toBeTruthy();
    expect(Array.from(container.querySelectorAll('[data-anchor]')).map((row) => row.getAttribute('data-anchor')))
      .toEqual(items.map((item) => item.checkpoint_id));
  });

  it('spells the heights of a checkpoint and links its block', () => {
    const { container } = renderTable();

    const row = container.querySelector(`[data-anchor="${ anchor.checkpoint_id }"]`) as HTMLElement;

    expect(row.querySelector('[data-label="batch-number"]')?.textContent).toBe(String(anchor.batch_number));
    expect(row.querySelector('[data-label="checkpoint-height"]')?.textContent).toBe(String(anchor.checkpoint_height));
    expect(row.querySelector('[data-label="sealed-height"]')?.textContent).toBe(String(anchor.sealed_height));
    expect(row.querySelector(`a[href="/block/${ anchor.block_number }"]`)?.textContent)
      .toBe(String(anchor.block_number));
  });

  it('marks the columns the anchor log leaves empty', () => {
    const { container } = renderTable();

    const row = container.querySelector(`[data-anchor="${ sparse.checkpoint_id }"]`) as HTMLElement;

    expect(row.querySelector('[data-label="checkpoint-height"]')?.textContent).toBe('—');
    expect(row.querySelector('[data-label="sealed-height"]')?.textContent).toBe('—');
    expect(row.querySelectorAll('td')[3]?.textContent).toBe('—');
  });

  it('puts every accepted checkpoint on the same rung of the settlement ladder', () => {
    const { container } = renderTable();

    expect(container.querySelectorAll(`[data-anchor] [data-rung="${ ANCHOR_SETTLEMENT_RUNG }"]`)).toHaveLength(items.length);
  });
});

// @vitest-environment jsdom

import React from 'react';

import * as blockMock from 'mocks/blocks/block';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, expect, test } from 'vitest';
import { screen } from 'vitest/lib';

import RecentBlocks from './RecentBlocks';

beforeEach(() => {
  fetchMock.resetMocks();
  fetchMock.mockResponse(JSON.stringify([ blockMock.base, blockMock.base2 ]), { headers: { 'Content-Type': 'application/json' } });
});

test('keeps short column heads whole within the available width', async function keepsShortColumnHeadsWholeWithinTheAvailableWidth(): Promise<void> {
  render(<RecentBlocks/>);
  await screen.findByText(String(blockMock.base.height));
  const region = screen.getByRole('table').closest('[role="region"]') as HTMLElement;
  const headers = Array.from(region.querySelectorAll('th'));
  const table = region.querySelector('table')!;

  expect(headers.map(header => header.textContent)).toEqual([ 'Block', 'Age', 'Txn', 'Gas used' ]);
  for (const header of headers) {
    const style = window.getComputedStyle(header);
    expect(style.whiteSpace).toBe('nowrap');
    expect(style.wordBreak).toBe('normal');
    expect(style.overflowWrap).toBe('normal');
  }
  expect(window.getComputedStyle(table).tableLayout).toBe('auto');
  expect(window.getComputedStyle(table).minWidth).toBe('0');
  expect(window.getComputedStyle(table).width).toBe('100%');
  expect(region.querySelectorAll('tbody tr')).toHaveLength(2);
  expect(region.querySelector('tbody tr')?.textContent).toContain(blockMock.base.transactions_count.toLocaleString());
});

test('keeps the scrollable region focusable and the first column sticky', async function keepsRegionFocusAndFirstColumnPosition(): Promise<void> {
  render(<RecentBlocks/>);
  await screen.findByText(String(blockMock.base.height));
  const region = screen.getByRole('table').closest('[role="region"]') as HTMLElement;

  region.focus();
  expect(document.activeElement).toBe(region);
  expect(region.tabIndex).toBe(0);
  expect(region.getAttribute('aria-label')).toBe('Recent blocks');
  expect(window.getComputedStyle(region).overflowX).toBe('auto');
  for (const firstCell of region.querySelectorAll('th:first-child, td:first-child')) {
    expect(window.getComputedStyle(firstCell).position).toBe('sticky');
    expect(window.getComputedStyle(firstCell).left).toBe('var(--chakra-spacing-0)');
  }
  expect(region.querySelector('a')?.getAttribute('href')).toContain(`/block/${ blockMock.base.height }`);
});

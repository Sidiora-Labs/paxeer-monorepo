// @vitest-environment jsdom

import React from 'react';

import * as blockMock from 'mocks/blocks/block';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, expect, test } from 'vitest';
import { screen } from 'vitest/lib';

import RecentBlocks from './RecentBlocks';

function matchingDeclaration(element: Element, property: string, selectorEnding = ''): string {
  const declarations: Array<string> = [];
  function readRules(rules: CSSRuleList): void {
    for (const rule of Array.from(rules)) {
      if (rule instanceof CSSStyleRule) {
        const value = rule.style.getPropertyValue(property);
        if (value && rule.selectorText.endsWith(selectorEnding) && element.matches(rule.selectorText)) {
          declarations.push(value);
        }
      } else if ('cssRules' in rule) {
        readRules((rule as CSSGroupingRule).cssRules);
      }
    }
  }
  for (const sheet of Array.from(document.styleSheets)) {
    readRules(sheet.cssRules);
  }
  expect(declarations).toHaveLength(1);
  return declarations[0];
}

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

test('keeps body cells free of vertical padding', async function keepsBodyCellsFreeOfVerticalPadding(): Promise<void> {
  render(<RecentBlocks/>);
  await screen.findByText(String(blockMock.base.height));
  const table = screen.getByRole('table');

  for (const cell of table.querySelectorAll('tbody td')) {
    expect(window.getComputedStyle(cell).paddingBlock).toBe('var(--chakra-spacing-0)');
    expect(matchingDeclaration(cell, 'padding-inline')).toBe('var(--chakra-spacing-1\\.5)');
  }
  expect(matchingDeclaration(table.querySelector('tbody td:first-child')!, 'padding-left', ':first-of-type')).toBe('var(--chakra-spacing-3)');
  expect(matchingDeclaration(table.querySelector('tbody td:last-child')!, 'padding-right', ':last-of-type')).toBe('var(--chakra-spacing-3)');
  expect(matchingDeclaration(table.querySelector('thead')!, 'height')).toBe('var(--chakra-spacing-9)');
});

test('keeps block links at the theme touch target', async function keepsBlockLinksAtTheThemeTouchTarget(): Promise<void> {
  render(<RecentBlocks/>);
  await screen.findByText(String(blockMock.base.height));
  const links = screen.getByRole('table').querySelectorAll('tbody a');

  expect(links).toHaveLength(2);
  for (const link of links) {
    const style = window.getComputedStyle(link);
    expect(style.minHeight).toBe('var(--chakra-sizes-touch-target)');
    expect(matchingDeclaration(link, 'min-width', ' a')).toBe('var(--chakra-sizes-touch-target)');
    expect(style.marginBlock).toBe('calc(var(--chakra-border-widths-1) / -2)');
  }
});

test('keeps the section label and all blocks link on one line', function keepsTheSectionLabelAndAllBlocksLinkOnOneLine(): void {
  render(<RecentBlocks/>);
  const heading = screen.getByRole('heading', { name: 'Recent blocks', level: 2 });
  const link = screen.getByRole('link', { name: 'View all' });
  const line = heading.parentElement!;
  const style = window.getComputedStyle(line);

  expect(link.parentElement).toBe(line);
  expect(Array.from(line.children)).toEqual([ heading, link ]);
  expect(style.display).toBe('flex');
  expect(style.alignItems).toBe('center');
  expect(style.justifyContent).toBe('space-between');
  expect(style.flexWrap).toBe('nowrap');
  expect(window.getComputedStyle(heading).whiteSpace).toBe('nowrap');
  expect(window.getComputedStyle(heading).textTransform).toBe('uppercase');
  expect(window.getComputedStyle(link).flexShrink).toBe('0');
  expect(window.getComputedStyle(link).whiteSpace).toBe('nowrap');
  expect(window.getComputedStyle(link).minHeight).toBe('var(--chakra-sizes-touch-target)');
  expect(link.getAttribute('href')).toBe('/blocks');
  expect(heading.closest('section')?.getAttribute('aria-labelledby')).toBe(heading.id);
});

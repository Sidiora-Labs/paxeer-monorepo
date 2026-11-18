// @vitest-environment jsdom

import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import React from 'react';

import { marketStyleRules } from 'ui/home/marketData.test-utils';
import { nextRouterModule, render as renderWithProviders } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, vi } from 'vitest';
import { screen } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
  };
});

import HeaderDesktop from './HeaderDesktop';

const render = (element: React.ReactElement) => renderWithProviders(
  <RouterContext.Provider value={ nextRouterModule().useRouter() }>{ element }</RouterContext.Provider>,
);

const renderPageSearchBar = () => <div>page search</div>;

describe('desktop header', () => {
  it('renders the single shell header with a compact search field', function rendersCompactSearch() {
    const { container } = render(<HeaderDesktop/>);

    expect(container.querySelector('[data-label="content-search"]')).toBeNull();
    expect(container.querySelector('header')).toBeTruthy();
    expect(screen.getByPlaceholderText('Search or ask…')).toBeTruthy();
  });

  it('keeps a page supplied search box inside the shell header', function keepsPageSearch() {
    const { container } = render(<HeaderDesktop renderSearchBar={ renderPageSearchBar }/>);

    const row = container.querySelector('[data-label="desktop-header"]');

    expect(row).toBeTruthy();
    expect(row?.tagName).toBe('HEADER');
    expect(screen.getByText('page search')).toBeTruthy();
  });
});

it('wraps header groups when their content exceeds the row', function headerRowWrapsAtIntrinsicWidth() {
  const { container } = render(<HeaderDesktop/>);
  const row = container.querySelector('[data-label="header-row"]')!;
  const classes = Array.from(row.classList);
  const rules = Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules))
    .map((rule) => rule.cssText).filter((rule) => classes.some((name) => rule.includes(`.${ name }`)))
    .join('').replace(/\s+/g, '');

  expect(rules).toContain('flex-wrap:wrap');
  expect(rules).not.toContain('flex-wrap:nowrap');
  expect(Array.from(row.children).map((child) => child.getAttribute('data-label')))
    .toEqual([ 'header-brand', 'header-menu', 'header-search', 'header-controls' ]);
  for (const label of [ 'header-brand', 'header-menu', 'header-controls' ]) {
    expect(marketStyleRules(row.querySelector(`[data-label="${ label }"]`)!)).toContain('flex-shrink:0');
  }
  expect(marketStyleRules(row.querySelector('[data-label="header-controls"]')!)).toContain('flex-wrap:nowrap');
});

it('lets search fill a second row and flex between desktop bounds', function headerSearchUsesTheAvailableWidthWithinBounds() {
  const { container } = render(<HeaderDesktop/>);
  const search = container.querySelector('[data-label="header-search"]')!;
  const rules = marketStyleRules(search);

  expect(window.getComputedStyle(search).flexGrow).toBe('1');
  expect(window.getComputedStyle(search).flexShrink).toBe('1');
  expect(window.getComputedStyle(search).flexBasis).toBe('100%');
  expect(rules).toContain('order:1');
  expect(rules).toContain('max-width:100%');
  expect(rules).toContain('order:0');
  expect(rules).toContain('flex-basis:var(--chakra-spacing-80)');
  expect(rules).toContain('min-width:var(--chakra-spacing-62\\.5)');
  expect(rules).toContain('max-width:var(--chakra-spacing-80)');
});

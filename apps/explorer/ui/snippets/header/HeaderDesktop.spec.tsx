// @vitest-environment jsdom

import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import React from 'react';

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

describe('HeaderDesktop', () => {
  it('renders the single shell header with a compact search field', () => {
    const { container } = render(<HeaderDesktop/>);

    expect(container.querySelector('[data-label="content-search"]')).toBeNull();
    expect(container.querySelector('header')).toBeTruthy();
    expect(screen.getByPlaceholderText('Search or ask')).toBeTruthy();
  });

  it('keeps a page supplied search box inside the shell header', () => {
    const { container } = render(<HeaderDesktop renderSearchBar={ renderPageSearchBar }/>);

    const row = container.querySelector('[data-label="desktop-header"]');

    expect(row).toBeTruthy();
    expect(row?.tagName).toBe('HEADER');
    expect(screen.getByText('page search')).toBeTruthy();
  });
});

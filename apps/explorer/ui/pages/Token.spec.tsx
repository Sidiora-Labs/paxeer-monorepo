// @vitest-environment jsdom

import React from 'react';

import * as addressMock from 'mocks/address/address';
import { tokenCounters, tokenInfo } from 'mocks/tokens/tokenInfo';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

// The token lists render fifty placeholder rows of real table items, which jsdom lays out well past the
// default per-test budget.
vi.setConfig({ testTimeout: 60_000 });

import TokenPageContent from './Token';

Object.defineProperty(document, 'fonts', {
  configurable: true,
  value: { load: () => Promise.resolve([ {} ]) },
});

const HASH = tokenInfo.address_hash;

const json = (payload: unknown) => ({
  body: JSON.stringify(payload),
  headers: { 'Content-Type': 'application/json' },
});

const tabTitles = (container: HTMLElement) =>
  Array.from(container.querySelectorAll('[role="tab"]')).map((tab) => tab.textContent ?? '');

describe('TokenPageContent', () => {
  beforeEach(() => {
    routerState.pathname = '/token/[hash]';
    routerState.query = { hash: HASH };
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      const { pathname } = new URL(request.url);

      if (pathname === `/api/v2/tokens/${ HASH }`) {
        return Promise.resolve(json(tokenInfo));
      }

      if (pathname === `/api/v2/tokens/${ HASH }/counters`) {
        return Promise.resolve(json(tokenCounters));
      }

      if (pathname === `/api/v2/addresses/${ HASH }`) {
        return Promise.resolve(json(addressMock.contract));
      }

      return Promise.resolve(json({ items: [], next_page_params: null }));
    });
  });

  it('heads the page with its name and the TOKEN kind', async() => {
    const { container } = render(<TokenPageContent/>);

    await waitFor(() => expect(container.querySelector('h1')?.textContent).toBe(`${ tokenInfo.name } (${ tokenInfo.symbol })`));
    expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('TOKEN');
  });

  it('puts the Markets tiles below the tab strip', () => {
    const { container } = render(<TokenPageContent/>);

    const details = container.querySelector('[data-token-markets]') as HTMLElement;
    const tabList = container.querySelector('[role="tablist"]') as HTMLElement;

    expect(details).not.toBeNull();
    expect(tabList).not.toBeNull();
    expect(details.querySelectorAll('[data-stat-tile]')).toHaveLength(4);
    expect(tabList.compareDocumentPosition(details) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('opens on Markets and retains History, Holders and Metadata', () => {
    const { container } = render(<TokenPageContent/>);

    const titles = tabTitles(container);

    expect(titles).toEqual([ 'Markets', 'History', 'Holders', 'Metadata' ]);
  });

  it('adds the contract tab once the address answered as a contract', async() => {
    routerState.query = { hash: HASH, tab: 'metadata' };
    const { container } = render(<TokenPageContent/>);

    await waitFor(() => {
      expect(tabTitles(container).some((title) => title.startsWith('Contract'))).toBe(true);
    });
  });

  it('keeps the pagination inside the table cards rather than on the tab row', () => {
    const { container } = render(<TokenPageContent/>);

    const tabList = container.querySelector('[role="tablist"]') as HTMLElement;

    expect(tabList.querySelector('[data-scan-pagination]')).toBeNull();
  });

  it('asks for the address in the same round as the token instead of waiting for the socket', () => {
    render(<TokenPageContent/>);

    const paths = fetchMock.mock.calls.map((call) => new URL(String(call[0]), 'http://localhost').pathname);

    expect(paths).toContain(`/api/v2/tokens/${ HASH }`);
    expect(paths).toContain(`/api/v2/addresses/${ HASH }`);
  });

  it('keeps the chip row of the title above the Markets tiles', () => {
    const { container } = render(<TokenPageContent/>);

    const chipRow = container.querySelector('[data-token-chip-row]') as HTMLElement;
    const details = container.querySelector('[data-token-markets]') as HTMLElement;

    expect(chipRow).not.toBeNull();
    expect(chipRow.compareDocumentPosition(details) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});

// @vitest-environment jsdom

import React from 'react';

import * as addressMock from 'mocks/address/address';
import { base as addressTabCounters } from 'mocks/address/tabCounters';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

// The address lists render fifty placeholder rows of real table items, which jsdom lays out well past the
// default per-test budget.
vi.setConfig({ testTimeout: 60_000 });

import AddressPageContent from './Address';
import { addressDetailFixtures } from './repair-22-33-314.fixtures';

const HASH = addressMock.hash;

const tabTitles = (container: HTMLElement) =>
  Array.from(container.querySelectorAll('[role="tab"]')).map((item) => item.textContent ?? '');

async function renderLoadedAddress() {
  const result = render(<AddressPageContent/>);
  await waitFor(() => {
    expect(result.container.querySelector(`a[href="/tx/${ addressMock.contract.creation_transaction_hash }"]`)).not.toBeNull();
    expect(result.container.querySelector('[data-section-group="ADDRESS INFO"]')).not.toBeNull();
  });
  return result;
}

const unexpectedRequests: Array<string> = [];

describe('AddressPageContent', () => {
  beforeEach(() => {
    routerState.pathname = '/address/[hash]';
    routerState.query = { hash: HASH };
    fetchMock.resetMocks();
    unexpectedRequests.length = 0;
    fetchMock.mockResponse((request) => {
      const path = new URL(request.url).pathname;
      if (!addressDetailFixtures.has(path)) {
        unexpectedRequests.push(path);
        return Promise.reject(new Error(`Unexpected address request: ${ path }`));
      }
      return Promise.resolve({ body: JSON.stringify(addressDetailFixtures.get(path)), headers: { 'Content-Type': 'application/json' } });
    });
  });

  afterEach(() => {
    expect(unexpectedRequests).toEqual([]);
  });

  it('heads the page with its name or address under the contract kind', async() => {
    const { container } = await renderLoadedAddress();

    const heading = container.querySelector('h1')?.textContent ?? '';

    expect(heading).toBe(addressMock.contract.name || addressMock.contract.ens_domain_name || addressMock.contract.hash);
    expect(container.querySelector('[data-entity-kind-label]')?.textContent).toBe('CONTRACT');
    expect(heading.endsWith('details')).toBe(false);
  });

  it('puts the icon action row beneath the title', async() => {
    const { container } = await renderLoadedAddress();

    expect(container.querySelector('[data-address-actions]')).toBeTruthy();
  });

  it('renders the balance tiles and address information below the tab strip', async() => {
    const { container } = await renderLoadedAddress();

    const details = container.querySelector('[data-address-details]') as Element;
    const tabList = container.querySelector('[role="tablist"]') as Element;

    expect(details).toBeTruthy();
    expect(tabList).toBeTruthy();
    expect(details.querySelectorAll('[data-address-card]')).toHaveLength(2);
    expect(details.querySelector('[data-address-balances]')).not.toBeNull();
    expect(tabList.compareDocumentPosition(details) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('opens on Overview and groups all existing data views', async() => {
    const { container } = await renderLoadedAddress();

    await waitFor(() => {
      const titles = tabTitles(container);
      expect(titles).toEqual([
        'Overview', 'History', `Transfers(${ addressTabCounters.token_transfers_count })`, 'Portfolio', 'Analytics', 'Contract', 'More',
      ]);
      expect(titles.some((title) => title.startsWith('Details'))).toBe(false);
    });
  });

  it('keeps the coin balance history and the advanced filter on the tab row', async() => {
    const { container, rerender } = await renderLoadedAddress();
    routerState.query = { hash: HASH, tab: 'portfolio' };
    rerender(<AddressPageContent/>);

    await waitFor(() => expect(tabTitles(container).some((title) => title.startsWith('Coin balance history'))).toBe(true));
    expect(container.querySelector('a[href^="/advanced-filter"]')).toBeTruthy();
  });

  it('places the identifier and its controls on their own row beneath the title', async() => {
    const { container } = await renderLoadedAddress();

    const row = container.querySelector('[data-entity-identifier]') as HTMLElement;
    expect(row.querySelector('[data-entity-kind="address"]')).toBeTruthy();
    expect(row.querySelector('[aria-label="copy"]')).not.toBeNull();
    expect(row.querySelector('[aria-label="Show QR code"]')).not.toBeNull();
    expect(container.querySelector('[data-title-row]')?.contains(row)).toBe(false);

  });
});

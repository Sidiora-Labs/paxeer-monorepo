// @vitest-environment jsdom

import React from 'react';

import type { PriceUpdateItem } from 'types/api/paxeerXFeed';

import { watchlistName } from 'mocks/address/tag';
import * as txMock from 'mocks/txs/tx';
import { render } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, vi, test } from 'vitest';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import LatestTxsItemMobile from './LatestTxsItemMobile';

vi.setConfig({ testTimeout: 60_000 });

describe('LatestTxsItemMobile', () => {
  it('stacks the hash, the parties and the value of the transaction', () => {
    const { container } = render(<LatestTxsItemMobile tx={ txMock.base }/>);

    const row = container.querySelector(`[data-latest-tx="${ txMock.base.hash }"]`) as HTMLElement;
    const parts = Array.from(row.querySelectorAll('[data-label]'))
      .map((item) => item.getAttribute('data-label'))
      .filter((label) => label !== null);

    expect(row.querySelector(`a[href="/tx/${ txMock.base.hash }"]`)).not.toBeNull();
    expect(parts).toEqual([ 'tx-parties', 'tx-tags', 'tx-value' ]);
  });

  it('names the sender and the recipient of the transaction', () => {
    const { container } = render(<LatestTxsItemMobile tx={ txMock.base }/>);

    const parties = container.querySelector('[data-label="tx-parties"]') as HTMLElement;

    expect(parties.querySelector(`a[href="/address/${ txMock.base.from.hash }"]`)).not.toBeNull();
    expect(parties.querySelector(`a[href="/address/${ txMock.base.to?.hash }"]`)).not.toBeNull();
  });

  it('carries the value and the fee of the transaction', () => {
    const { container } = render(<LatestTxsItemMobile tx={ txMock.base }/>);

    const value = container.querySelector('[data-label="tx-value"]') as HTMLElement;

    expect(value.textContent).toContain('ETH');
    expect(value.textContent).toContain('Fee');
  });

  it('shows a price update with its feed link, market count, feeder, the price feed and dashes for value and fee', () => {
    const { container } = render(<LatestTxsItemMobile tx={ txMock.priceUpdateItem }/>);

    const row = container.querySelector(`[data-latest-tx="${ txMock.priceUpdateItem.hash }"]`) as HTMLElement;
    const tags = row.querySelector('[data-label="tx-tags"]') as HTMLElement;
    const parties = row.querySelector('[data-label="tx-parties"]') as HTMLElement;
    const value = row.querySelector('[data-label="tx-value"]') as HTMLElement;

    expect(row.querySelector(`a[href="/feed/update/${ txMock.priceUpdateItem.hash }"]`)).not.toBeNull();
    expect(row.querySelector(`a[href="/tx/${ txMock.priceUpdateItem.hash }"]`)).toBeNull();
    expect(tags.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Price update · 2 markets');
    expect(Array.from(parties.querySelectorAll('a[href^="/address/"]')).map((link) => link.getAttribute('href')))
      .toEqual([ `/address/${ txMock.priceUpdateItem.from.hash }` ]);
    expect(parties.querySelector('a[href="/feed"]')?.textContent).toBe('Price feed');
    expect(value.textContent).toBe('-Fee -');
    expect(row.querySelector('button[aria-label="Transaction info"]')).toBeNull();
  });

  function shows_a_failed_price_update_with_its_status() {
    const tx: PriceUpdateItem = { ...txMock.priceUpdateItem, status: 'error', result: 'failed' };
    const { container } = render(<LatestTxsItemMobile tx={ tx }/>);

    const row = container.querySelector(`[data-latest-tx="${ tx.hash }"]`) as HTMLElement;
    const tags = row.querySelector('[data-label="tx-tags"]') as HTMLElement;
    const status = tags.querySelector('[data-status="error"]');
    const parties = row.querySelector('[data-label="tx-parties"]') as HTMLElement;
    const lines = Array.from(parties.children);

    expect(status).not.toBeNull();
    expect(status?.textContent).toBe('Failed');
    expect(tags.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Price update · 2 markets');
    expect(lines).toHaveLength(2);
    expect(lines[0].firstElementChild?.textContent).toBe('From');
    expect(lines[0].querySelector(`a[href="/address/${ tx.from.hash }"]`)).not.toBeNull();
    expect(lines[1].firstElementChild?.textContent).toBe('To');
    expect(lines[1].querySelector('a[href="/feed"]')?.textContent).toBe('Price feed');
    expect(row.querySelector('button[aria-label="Transaction info"]')).toBeNull();
  }

  test(shows_a_failed_price_update_with_its_status);

  function shows_watchlist_tags_on_a_price_update() {
    const tx = {
      ...txMock.priceUpdateItem,
      from: { ...txMock.priceUpdateItem.from, watchlist_names: [ watchlistName ] },
    };
    const { container } = render(<LatestTxsItemMobile tx={ tx }/>);

    const row = container.querySelector(`[data-latest-tx="${ tx.hash }"]`) as HTMLElement;
    const tags = row.querySelector('[data-label="tx-tags"]') as HTMLElement;

    expect(tags.textContent).toContain(watchlistName.display_name);
    expect(tags.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Price update · 2 markets');
    expect(tags.querySelector('[data-status="error"]')).toBeNull();
    expect(row.querySelector(`a[href="/address/${ tx.from.hash }"]`)).not.toBeNull();
  }

  test(shows_watchlist_tags_on_a_price_update);

  function shows_the_info_popover_on_an_evm_row() {
    const { container } = render(<LatestTxsItemMobile tx={ txMock.base }/>);

    const row = container.querySelector(`[data-latest-tx="${ txMock.base.hash }"]`) as HTMLElement;

    expect(row.querySelector('button[aria-label="Transaction info"]')).not.toBeNull();
  }

  test(shows_the_info_popover_on_an_evm_row);
});

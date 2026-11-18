// @vitest-environment jsdom

import React from 'react';

import * as blockMock from 'mocks/blocks/block';
import * as statsMock from 'mocks/stats/index';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import Blocks from './Blocks';

const PAGE_TIMEOUT = 120_000;
const WAIT_TIMEOUT = 60_000;

describe('Blocks', () => {
  beforeEach(() => {
    routerState.pathname = '/blocks';
    routerState.query = {};
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      if (request.url.includes('/api/v2/blocks')) {
        return { body: JSON.stringify(blockMock.baseListResponse), headers: { 'Content-Type': 'application/json' } };
      }

      if (request.url.includes('/api/v2/stats')) {
        return { body: JSON.stringify(statsMock.base), headers: { 'Content-Type': 'application/json' } };
      }

      return { body: JSON.stringify({}), headers: { 'Content-Type': 'application/json' } };
    });
  });

  it('opens with the page title', () => {
    const { container } = render(<Blocks/>);

    expect(container.querySelector('h1')?.textContent).toBe('Blocks');
  }, PAGE_TIMEOUT);

  it('counts the blocks of the chain in the card header', async() => {
    const { container } = render(<Blocks/>);

    await waitFor(() => {
      expect(container.querySelector('[data-list-page] [data-title]')?.textContent)
        .toBe('Latest 2 from a total of 30,215,608 blocks');
    }, { timeout: WAIT_TIMEOUT });
  }, PAGE_TIMEOUT);

  it('leaves every pagination inside the table card', async() => {
    const { container } = render(<Blocks/>);

    await waitFor(() => {
      expect(container.querySelector('[data-list-page]')).not.toBeNull();
    }, { timeout: WAIT_TIMEOUT });

    const outside = Array.from(container.querySelectorAll('[data-scan-pagination]'))
      .filter((node) => !node.closest('[data-list-page]'));

    expect(outside).toHaveLength(0);
  }, PAGE_TIMEOUT);

  it('lists the blocks the node returns inside the card', async() => {
    const { container } = render(<Blocks/>);

    await waitFor(() => {
      const heights = Array.from(container.querySelectorAll('[data-list-page] [data-body] table tbody a[href^="/block/"]'))
        .map((link) => link.getAttribute('href'))
        .filter((href) => !href?.includes('?'));

      expect(heights).toEqual([ `/block/${ blockMock.base.height }`, `/block/${ blockMock.base2.height }` ]);
    }, { timeout: WAIT_TIMEOUT });
  }, PAGE_TIMEOUT);

  it('lists blocks without a tab strip or requests for forked blocks and uncles', async function listsBlocksWithoutTabs() {
    const { container, queryByRole } = render(<Blocks/>);

    await waitFor(() => {
      expect(container.querySelector('[data-list-page] [data-title]')?.textContent)
        .toBe('Latest 2 from a total of 30,215,608 blocks');
    }, { timeout: WAIT_TIMEOUT });

    expect(queryByRole('tablist')).toBeNull();
    expect(queryByRole('tab', { name: 'Forked', hidden: true })).toBeNull();
    expect(queryByRole('tab', { name: 'Uncles', hidden: true })).toBeNull();

    const blockTypes = fetchMock.mock.calls
      .map(([ input ]) => new URL(input instanceof Request ? input.url : String(input)))
      .filter((url) => url.pathname.endsWith('/api/v2/blocks'))
      .map((url) => url.searchParams.get('type'));

    expect(blockTypes).toEqual([ 'block' ]);
  }, PAGE_TIMEOUT);

  it.each([ 'reorgs', 'uncles' ])('lists blocks for the legacy %s tab', async function listsBlocksForLegacyTab(tab) {
    routerState.query = { tab };
    const { container, queryByRole } = render(<Blocks/>);

    await waitFor(() => {
      const heights = Array.from(container.querySelectorAll('[data-list-page] [data-body] table tbody a[href^="/block/"]'))
        .map((link) => link.getAttribute('href'))
        .filter((href) => !href?.includes('?'));

      expect(heights).toEqual([ `/block/${ blockMock.base.height }`, `/block/${ blockMock.base2.height }` ]);
    }, { timeout: WAIT_TIMEOUT });

    expect(queryByRole('tab', { name: 'Forked', hidden: true })).toBeNull();
    expect(queryByRole('tab', { name: 'Uncles', hidden: true })).toBeNull();

    const blockTypes = fetchMock.mock.calls
      .map(([ input ]) => new URL(input instanceof Request ? input.url : String(input)))
      .filter((url) => url.pathname.endsWith('/api/v2/blocks'))
      .map((url) => url.searchParams.get('type'));

    expect(blockTypes).toEqual([ 'block' ]);
  }, PAGE_TIMEOUT);

  it('shows network utilization and the block countdown above the list', async function showsBlockCountdown() {
    const { container, getByText } = render(<Blocks/>);

    await waitFor(() => {
      expect(container.querySelector('[data-list-page] [data-title]')?.textContent)
        .toBe('Latest 2 from a total of 30,215,608 blocks');
    }, { timeout: WAIT_TIMEOUT });

    const countdown = getByText('Block countdown').closest('a');
    const networkUtilization = getByText(/Network utilization \(last 50 blocks\):/);
    const list = container.querySelector('[data-list-page]');

    expect(countdown).not.toBeNull();
    expect(countdown!.textContent).toBe('Block countdown');
    expect(countdown!.getAttribute('href')).toBe('/block/countdown');
    expect(countdown!.compareDocumentPosition(list!)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(networkUtilization.compareDocumentPosition(list!)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  }, PAGE_TIMEOUT);
});

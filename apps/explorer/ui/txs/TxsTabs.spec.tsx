// @vitest-environment jsdom

import React from 'react';

import * as statsMock from 'mocks/stats/index';
import * as txMock from 'mocks/txs/tx';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

// The validated tab renders fifty placeholder rows of real table items, which jsdom lays out well past
// the default per-test budget.
vi.setConfig({ testTimeout: 60_000 });

import TxsTabs from './TxsTabs';

const paginationsOutsideTheCard = (container: HTMLElement) =>
  Array.from(container.querySelectorAll('[data-scan-pagination]')).filter((node) => !node.closest('[data-list-page]'));

describe('TxsTabs', () => {
  beforeEach(() => {
    routerState.pathname = '/txs';
    routerState.query = {};
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      if (request.url.includes('/api/v2/stats')) {
        return { body: JSON.stringify(statsMock.base), headers: { 'Content-Type': 'application/json' } };
      }

      if (request.url.includes('/api/v2/transactions')) {
        return {
          body: JSON.stringify({ items: [ txMock.base ], next_page_params: null }),
          headers: { 'Content-Type': 'application/json' },
        };
      }

      return { body: JSON.stringify({}), headers: { 'Content-Type': 'application/json' } };
    });
  });

  it('keeps the advanced filter in the tab strip', () => {
    const { container } = render(<TxsTabs/>);

    const link = container.querySelector('a[href^="/advanced-filter"]');

    expect(link).not.toBeNull();
    expect(link?.closest('[data-list-page]')).toBeNull();
  });

  it('leaves the pagination to the table card', () => {
    const { container } = render(<TxsTabs/>);

    expect(container.querySelector('[data-list-page]')).not.toBeNull();
    expect(paginationsOutsideTheCard(container)).toHaveLength(0);
  });

  it('leaves the pagination to the table card on the pending tab as well', () => {
    routerState.query = { tab: 'pending' };

    const { container } = render(<TxsTabs/>);

    expect(paginationsOutsideTheCard(container)).toHaveLength(0);
  });
});

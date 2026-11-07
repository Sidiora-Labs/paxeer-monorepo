// @vitest-environment jsdom

import React from 'react';

import * as blockMock from 'mocks/blocks/block';
import * as dailyTxsMock from 'mocks/stats/daily_txs';
import * as statsMock from 'mocks/stats/index';
import { tokenInfoERC20b } from 'mocks/tokens/tokenInfo';
import * as txMock from 'mocks/txs/tx';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { screen } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_STATS_API_HOST: '',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import Home from './Home';

// jsdom implements <path> as SVGElement and leaves the geometry methods of SVGGeometryElement out,
// so the chart line animation of the toolkit has nothing to measure without this shim.
if (!('getTotalLength' in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, 'getTotalLength', { configurable: true, value: () => 0 });
}

vi.setConfig({ testTimeout: 60_000 });

describe('Home', () => {
  beforeEach(() => {
    routerState.pathname = '/';
    routerState.query = {};
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      const body = (() => {
        if (request.url.includes('/api/v2/tokens')) {
          return { items: [ tokenInfoERC20b ], next_page_params: null };
        }

        if (request.url.includes('/api/v2/main-page/blocks')) {
          return [ blockMock.base, blockMock.base2 ];
        }

        if (request.url.includes('/api/v2/main-page/transactions')) {
          return [ txMock.base, txMock.base2 ];
        }

        if (request.url.includes('/api/v2/stats/charts/transactions')) {
          return dailyTxsMock.base;
        }

        if (request.url.includes('/api/v2/stats')) {
          return statsMock.base;
        }

        return {};
      })();

      return { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
    });
  });

  it('puts the ticker, hero, stat tiles and lists in that order', async() => {
    const { container } = render(<Home/>);

    await screen.findByText(tokenInfoERC20b.symbol!);

    expect(Array.from(container.querySelectorAll('[data-label="ticker-strip"], [data-label="hero"], [data-label="home-stats"], [data-label="home-lists"]'))
      .map((item) => item.getAttribute('data-label')))
      .toEqual([ 'ticker-strip', 'hero', 'home-stats', 'home-lists' ]);
  });

  it('sets the blocks table beside the transactions table', async() => {
    const { container } = render(<Home/>);

    await screen.findByText(tokenInfoERC20b.symbol!);

    const lists = container.querySelector('[data-label="home-lists"]') as HTMLElement;

    expect(Array.from(lists.querySelectorAll('[data-scan-table-card] [data-title]')).map((item) => item.textContent))
      .toEqual([ 'Latest blocks', 'Latest transactions' ]);
  });

  it('carries the search field of the page in the hero', async() => {
    const { container } = render(<Home/>);

    await screen.findByText(tokenInfoERC20b.symbol!);

    expect(container.querySelector('[data-label="hero"] [data-label="hero-search"] input')).not.toBeNull();
  });
});

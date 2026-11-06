// @vitest-environment jsdom

import React from 'react';

import type { Block } from 'types/api/block';

import { SocketProvider } from 'lib/socket/context';
import * as blockMock from 'mocks/blocks/block';
import * as dailyTxsMock from 'mocks/stats/daily_txs';
import * as statsMock from 'mocks/stats/index';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { screen } from 'vitest/lib';
import { createTestSocket } from 'vitest/utils/socketServer';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_STATS_API_HOST: '',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import { HomeRpcDataContextProvider } from './fallbacks/rpcDataContext';
import LatestBlocks from './LatestBlocks';
import Stats from './Stats';

const TOPIC = 'blocks:new_block';

const socketBlock = (offset: number): Block => ({
  ...blockMock.base,
  height: blockMock.base.height + offset,
  hash: `0x${ (1000 + offset).toString(16).padStart(64, '0') }`,
});

const totalBlocksCounter = (container: HTMLElement) => container
  .querySelector('[data-stat="total_blocks"] [data-value]')?.textContent;

const renderHomeStats = (socketUrl: string, onRender: React.ProfilerOnRenderCallback) => render(
  <SocketProvider url={ socketUrl }>
    <HomeRpcDataContextProvider>
      <React.Profiler id="home-stats" onRender={ onRender }>
        <Stats/>
      </React.Profiler>
      <LatestBlocks/>
    </HomeRpcDataContextProvider>
  </SocketProvider>,
);

// jsdom implements <path> as SVGElement and leaves the geometry methods of SVGGeometryElement out,
// so the chart line animation of the toolkit has nothing to measure without this shim.
if (!('getTotalLength' in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, 'getTotalLength', { configurable: true, value: () => 0 });
}

const mockHomeApi = () => {
  fetchMock.mockResponse((request) => {
    const body = (() => {
      if (request.url.includes('/api/v2/main-page/blocks')) {
        return [ blockMock.base, blockMock.base2 ];
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
};

vi.setConfig({ testTimeout: 60_000 });

describe('Stats', () => {
  beforeEach(() => {
    routerState.pathname = '/';
    routerState.query = {};
    fetchMock.resetMocks();
    mockHomeApi();
  });

  it('places five stat tiles above the history card', async() => {
    const { container } = render(<Stats/>);

    await screen.findByText('30,215,608');

    expect(Array.from(container.querySelectorAll('[data-stat]')).map((item) => item.getAttribute('data-stat')))
      .toEqual([ 'total_blocks', 'average_block_time', 'total_txs', 'wallet_addresses', 'gas_tracker' ]);
    const tiles = container.querySelector('[data-label="home-stat-tiles"]') as HTMLElement;
    const history = container.querySelector('[data-label="home-stats-history"]') as HTMLElement;
    expect(tiles.compareDocumentPosition(history) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('uses the stats fields for the block total, wallet count and gas price', async() => {
    const { container } = render(<Stats/>);

    await screen.findByText('30,215,608');

    expect(container.querySelector('[data-stat="total_blocks"] h2')?.textContent).toBe('Total blocks');
    expect(totalBlocksCounter(container)).toBe('30,215,608');
    expect(container.querySelector('[data-stat="wallet_addresses"] [data-value]')?.textContent).toBe('19,667,249');
    expect(container.querySelector('[data-stat="gas_tracker"] [data-value]')?.textContent).toBe('23.75 Gwei');
  });

  it('gives transaction count and average block time their own labelled tiles', async() => {
    const { container } = render(<Stats/>);

    await screen.findByText('82,258,122');

    expect(container.querySelector('[data-stat="total_txs"] h2')?.textContent).toBe('Total transactions');
    expect(container.querySelector('[data-stat="total_txs"] [data-value]')?.textContent).toBe('82,258,122');
    expect(container.querySelector('[data-stat="average_block_time"] h2')?.textContent).toBe('Average block time');
    expect(container.querySelector('[data-stat="average_block_time"] [data-value]')?.textContent).toBe('6.2s');
  });

  it('keeps API totals and history unchanged while the latest block list flushes', async() => {
    const socket = await createTestSocket();

    try {
      const commits: Array<string> = [];
      const { container } = renderHomeStats(socket.url, (id, phase) => {
        commits.push(phase);
      });

      await screen.findByText('30,215,608');

      await vi.waitFor(() => {
        expect(container.querySelector('[data-latest-block]')?.getAttribute('data-latest-block')).toBe(String(blockMock.base.height));
        expect(totalBlocksCounter(container)).toBe('30,215,608');
      }, { timeout: 30_000, interval: 100 });

      await socket.join(TOPIC);

      // One message whose flush lands on a cadence tick, so the burst that follows sits inside one
      // cadence instead of straddling two of them.
      socket.send(TOPIC, 'new_block', { average_block_time: '1000', block: socketBlock(1) });

      await vi.waitFor(() => {
        expect(container.querySelector('[data-latest-block]')?.getAttribute('data-latest-block')).toBe(String(blockMock.base.height + 1));
        expect(totalBlocksCounter(container)).toBe('30,215,608');
      }, { timeout: 30_000, interval: 50 });

      await new Promise((resolve) => {
        setTimeout(resolve, 300);
      });

      const untouched = Array.from(container.querySelectorAll('[data-stat]'));
      const mutations: Array<MutationRecord> = [];
      const observer = new MutationObserver((records) => {
        mutations.push(...records);
      });
      untouched.forEach((element) => observer.observe(element, { childList: true, characterData: true, subtree: true }));

      const historyTitle = container.querySelector('[data-label="home-stats-history"] [data-title]')?.textContent;
      const commitsBeforeBurst = commits.length;

      for (let offset = 2; offset <= 21; offset++) {
        socket.send(TOPIC, 'new_block', { average_block_time: '1000', block: socketBlock(offset) });
        await new Promise((resolve) => {
          setTimeout(resolve, 20);
        });
      }

      expect(commits.length - commitsBeforeBurst).toBe(0);
      expect(container.querySelector('[data-latest-block]')?.getAttribute('data-latest-block')).toBe(String(blockMock.base.height + 1));
      expect(totalBlocksCounter(container)).toBe('30,215,608');

      await vi.waitFor(() => {
        expect(container.querySelector('[data-latest-block]')?.getAttribute('data-latest-block')).toBe(String(blockMock.base.height + 21));
        expect(totalBlocksCounter(container)).toBe('30,215,608');
      }, { timeout: 30_000, interval: 50 });

      expect(mutations).toHaveLength(0);
      expect(container.querySelector('[data-label="home-stats-history"] [data-title]')?.textContent).toBe(historyTitle);

      observer.disconnect();
    } finally {
      await socket.close();
    }
  });

  it('titles the history card with the last fourteen days', async() => {
    const { container } = render(<Stats/>);

    await screen.findByText('30,215,608');

    const column = container.querySelector('[data-label="home-stats-history"]') as HTMLElement;

    expect(column.querySelector('[data-title]')?.textContent).toBe('Blockscout transaction history in 14 days');
  });
});

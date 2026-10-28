// @vitest-environment jsdom

import React from 'react';

import type { MarketUpdate } from 'types/api/paxeerXFeed';

import { SocketProvider } from 'lib/socket/context';
import { SOCKET_FLUSH_INTERVAL_MS } from 'lib/socket/useSocketBuffer';
import * as feedMock from 'mocks/paxeerX/feed';
import { FEED_HISTORY_BUFFER_LIMIT } from 'ui/paxeerX/feed/useFeedSocket';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, waitFor } from 'vitest/lib';
import { createTestSocket } from 'vitest/utils/socketServer';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import PaxeerXFeedMarket from './PaxeerXFeedMarket';

const TOPIC = 'paxeer_x_feed:market:0';

const historyItem = (blockNumber: number, hashByte: string, sequence: number): MarketUpdate => ({
  update_hash: `0x${ hashByte.repeat(32) }`,
  block_number: blockNumber,
  timestamp: '2026-01-02T03:04:10.000000Z',
  index_price: '67500000000000000000000',
  mark_price: '67505000000000000000000',
  source_timestamp_ms: 1767323050000,
  sequence,
});

const updateHashes = (container: HTMLElement) => Array.from(container.querySelectorAll('[data-update]')).map((item) => item.getAttribute('data-update'));

vi.setConfig({ testTimeout: 60_000 });

describe('PaxeerXFeedMarket live updates', () => {
  beforeEach(() => {
    routerState.pathname = '/feed/market/[id]';
    routerState.query = { id: '0' };
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => Promise.resolve({
      body: JSON.stringify(request.url.includes('/updates') ? feedMock.marketUpdatesPage : feedMock.markets),
      headers: { 'Content-Type': 'application/json' },
    }));
  });

  it('puts the submissions of its market topic on top of the first page', async() => {
    const socket = await createTestSocket();

    try {
      const { container } = render(
        <SocketProvider url={ socket.url }>
          <PaxeerXFeedMarket/>
        </SocketProvider>,
      );

      const initial = feedMock.marketUpdates.map((item) => item.update_hash);

      await waitFor(() => {
        expect(updateHashes(container)).toEqual(initial);
      });
      await socket.join(TOPIC);

      const first = historyItem(34800124, 'ab', 9200);
      const second = historyItem(34800125, 'cd', 9201);

      socket.send(TOPIC, 'new_update', first);
      socket.send(TOPIC, 'new_update', second);
      socket.send(TOPIC, 'new_update', feedMock.marketUpdates[0]);

      await waitFor(() => {
        expect(updateHashes(container)).toEqual([ second.update_hash, first.update_hash, initial[0] ]);
      }, { timeout: 10_000 });

      expect(container.querySelector(`[data-update="${ second.update_hash }"] [data-price]`)?.getAttribute('data-price')).toBe(second.index_price);
    } finally {
      await socket.close();
    }
  });

  it('keeps at most a page of submissions while the history is held', async function keepsAtMostAPageOfSubmissionsWhileTheHistoryIsHeld() {
    const socket = await createTestSocket();

    try {
      const { container, getByRole } = render(
        <SocketProvider url={ socket.url }>
          <PaxeerXFeedMarket/>
        </SocketProvider>,
      );

      const initial = feedMock.marketUpdates.map((item) => item.update_hash);

      await waitFor(() => {
        expect(updateHashes(container)).toEqual(initial);
      });
      await socket.join(TOPIC);

      const table = getByRole('table');
      const submissions = Array.from({ length: FEED_HISTORY_BUFFER_LIMIT + 5 }, (_, index) =>
        historyItem(34800124 + index, (index + 1).toString(16).padStart(2, '0'), 9200 + index),
      );
      const discarded = submissions.slice(0, 5).map((item) => item.update_hash);
      const retained = submissions.slice(5).reverse().map((item) => item.update_hash);
      const rendered = new Set(updateHashes(container));
      const observer = new MutationObserver(() => {
        updateHashes(container).forEach((hash) => rendered.add(hash));
      });

      observer.observe(container, { childList: true, subtree: true });

      try {
        fireEvent.mouseEnter(table);

        await act(async() => {
          submissions.forEach((item) => socket.send(TOPIC, 'new_update', item));
          await new Promise((resolve) => {
            setTimeout(resolve, SOCKET_FLUSH_INTERVAL_MS + 500);
          });
        });

        expect(updateHashes(container)).toEqual(initial);
        fireEvent.mouseLeave(table);

        await waitFor(() => {
          expect(updateHashes(container)).toEqual(retained);
        }, { timeout: 10_000 });

        expect(updateHashes(container)).toHaveLength(FEED_HISTORY_BUFFER_LIMIT);
        discarded.forEach((hash) => {
          expect(rendered.has(hash)).toBe(false);
          expect(container.querySelector(`[data-update="${ hash }"]`)).toBeNull();
        });
      } finally {
        observer.disconnect();
      }
    } finally {
      await socket.close();
    }
  });

  it('leaves a later page as the endpoint returned it', async() => {
    routerState.query = { id: '0', page: '2' };

    const socket = await createTestSocket();

    try {
      const { container } = render(
        <SocketProvider url={ socket.url }>
          <PaxeerXFeedMarket/>
        </SocketProvider>,
      );

      await waitFor(() => {
        expect(updateHashes(container)).toEqual(feedMock.marketUpdates.map((item) => item.update_hash));
      });

      await new Promise((resolve) => {
        setTimeout(resolve, 500);
      });

      expect(() => socket.send(TOPIC, 'new_update', historyItem(34800124, 'ab', 9200))).toThrow(`no client joined ${ TOPIC }`);
    } finally {
      await socket.close();
    }
  });
});

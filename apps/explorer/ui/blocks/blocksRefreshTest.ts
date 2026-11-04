import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { once } from 'node:events';
import React from 'react';
import type { WebSocket } from 'ws';
import { WebSocketServer } from 'ws';

import type { Block, BlocksResponse, BlockType, NewBlockSocketResponse } from 'types/api/block';

import { getResourceKey } from 'lib/api/useApiQuery';
import { SocketProvider, useSocket } from 'lib/socket/context';
import useSocketChannel from 'lib/socket/useSocketChannel';
import useSocketMessage from 'lib/socket/useSocketMessage';
import { base as blockFixture } from 'mocks/blocks/block';
import { render } from 'ui/shared/layout/testWrapper';
import { expect } from 'vitest';
import { act, waitFor } from 'vitest/lib';

import BlocksList from './BlocksList';
import BlocksTable from './BlocksTable';
import useBlocksNewBlock from './useBlocksNewBlock';

const TOPIC = 'blocks:new_block';

// This loopback peer replays the v2 channel envelope over a real WebSocket. The
// production Phoenix client performs the join and decodes every delivered event.
class PhoenixFixtureServer {
  private server = new WebSocketServer({ host: '127.0.0.1', port: 0 });
  private subscribers = new Set<WebSocket>();

  constructor() {
    this.server.on('connection', (socket) => {
      socket.on('message', (data) => {
        const frame = JSON.parse(data.toString());
        if (!Array.isArray(frame) || frame.length !== 5) {
          throw new Error('Expected a Phoenix v2 channel frame with five fields');
        }
        const [ joinRef, ref, topic, event ] = frame;
        if (event === 'phx_join') {
          expect(topic).toBe(TOPIC);
          this.subscribers.add(socket);
        }
        if (event === 'phx_join' || event === 'phx_leave' || event === 'heartbeat') {
          socket.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status: 'ok', response: {} } ]));
        }
        if (event === 'phx_leave') {
          this.subscribers.delete(socket);
        }
      });
      socket.on('close', () => this.subscribers.delete(socket));
    });
  }

  async start() {
    await once(this.server, 'listening');
    const address = this.server.address();
    if (!address || typeof address === 'string') {
      throw new Error('The loopback Phoenix fixture did not bind a TCP port');
    }
    return `ws://127.0.0.1:${ address.port }/socket/v2`;
  }

  async publish(payload: NewBlockSocketResponse) {
    expect(this.subscribers.size).toBe(1);
    const frame = JSON.stringify([ null, null, TOPIC, 'new_block', payload ]);
    await Promise.all(Array.from(this.subscribers, (socket) => new Promise<void>((resolve, reject) => {
      socket.send(frame, (error) => error ? reject(error) : resolve());
    })));
    return payload;
  }

  async close() {
    this.server.clients.forEach((socket) => socket.terminate());
    await new Promise<void>((resolve, reject) => {
      this.server.close((error) => error ? reject(error) : resolve());
    });
  }
}

function blockAt(height: number, type: BlockType = 'block', fork = 0): Block {
  return {
    ...blockFixture,
    height,
    hash: `0x${ (height * 100 + fork).toString(16).padStart(64, '0') }`,
    type,
    transactions_count: 0,
    price_update_count: 0,
  };
}

// Notifier.block_params_v2 supplies average_block_time as milliseconds in a
// string. BlockView.prepare_block adds native submissions to transactions_count
// and carries their own count in price_update_count, preserving the block hash.
function blockEvent(block: Block): NewBlockSocketResponse {
  return { average_block_time: '12000', block };
}

async function withBlocks(
  count: number,
  type: BlockType | undefined,
  run: (context: {
    producer: PhoenixFixtureServer;
    client: QueryClient;
    container: HTMLElement;
    key: ReturnType<typeof getResourceKey>;
    fetches: () => number;
    received: Array<NewBlockSocketResponse>;
  }) => Promise<void>,
) {
  const producer = new PhoenixFixtureServer();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  let unmount: (() => void) | undefined;

  try {
    const url = await producer.start();
    const storedPage: BlocksResponse = {
      items: Array.from({ length: Math.min(count, 50) }, (_, index) => blockAt(count - index, type)),
      next_page_params: count > 50 ? { block_number: count - 49, items_count: 50 } : null,
    };
    const key = getResourceKey('general:blocks', { queryParams: { type } });
    const socketContext: { current: ReturnType<typeof useSocket> } = { current: null };
    let fetchCount = 0;
    const received: Array<NewBlockSocketResponse> = [];

    const Harness = () => {
      socketContext.current = useSocket();
      const query = useQuery({
        queryKey: key,
        queryFn: () => {
          fetchCount++;
          return Promise.resolve(structuredClone(storedPage));
        },
      });
      const { newItemsCount } = useBlocksNewBlock({ type, isDisabled: query.isPending || query.isError });
      const channel = useSocketChannel({ topic: TOPIC, isDisabled: query.isPending || query.isError });
      useSocketMessage({ channel, event: 'new_block', handler: (payload) => received.push(payload) });

      if (!query.data) {
        return null;
      }

      return React.createElement(React.Fragment, null,
        React.createElement('output', { 'data-new-blocks': true }, String(newItemsCount)),
        React.createElement(BlocksList, { data: query.data.items, isLoading: false, page: 1 }),
        React.createElement(BlocksTable, { data: query.data.items, top: 0, page: 1 }),
      );
    };

    const rendered = render(React.createElement(QueryClientProvider, { client },
      React.createElement(SocketProvider, { url, children: React.createElement(Harness) }),
    ));
    unmount = rendered.unmount;

    await waitFor(() => {
      expect(socketContext.current?.channelRegistry.current[TOPIC]?.channel.state).toBe('joined');
      expect(client.getQueryData<BlocksResponse>(key)?.items.length).toBe(Math.min(count, 50));
    }, { timeout: 10_000 });

    await run({ producer, client, container: rendered.container, key, fetches: () => fetchCount, received });
  } finally {
    unmount?.();
    client.clear();
    await producer.close();
  }
}

export async function nativeBlockRefresh() {
  await withBlocks(2, 'block', async({ producer, client, container, key, fetches }) => {
    const before = client.getQueryData<BlocksResponse>(key)!;
    const initial = before.items[0];
    const transactionLinks = () => container.querySelectorAll(`a[href="/block/${ initial.height }?tab=txs"]`);
    expect(initial.transactions_count).toBe(0);
    expect(initial.price_update_count).toBe(0);
    expect(transactionLinks()).toHaveLength(0);
    const requests = fetches();

    const payload = await producer.publish(blockEvent({ ...initial, transactions_count: 2, price_update_count: 2 }));
    expect(payload.block.hash).toBe(initial.hash);
    expect(payload.block.transactions_count).toBe(2);
    expect(payload.block.price_update_count).toBe(2);

    await waitFor(() => {
      expect(Array.from(transactionLinks(), (link) => link.textContent)).toEqual([ '2', '2' ]);
    });
    const after = client.getQueryData<BlocksResponse>(key)!;
    expect(after.items).toEqual([ payload.block, before.items[1] ]);
    expect(after.next_page_params).toEqual(before.next_page_params);
    expect(fetches()).toBe(requests);
    expect(container.querySelector('[data-new-blocks]')?.textContent).toBe('0');
  });
}

export async function canonicalHashReplacement() {
  for (const type of [ 'block', undefined ] as const) {
    await withBlocks(2, type, async({ producer, client, key, fetches }) => {
      const before = client.getQueryData<BlocksResponse>(key)!;
      const payload = await producer.publish(blockEvent(blockAt(before.items[0].height, 'block', 1)));
      expect(payload.block.hash).not.toBe(before.items[0].hash);
      await waitFor(() => expect(client.getQueryData<BlocksResponse>(key)?.items).toEqual([ payload.block, before.items[1] ]));
      const next = await producer.publish(blockEvent(blockAt(4)));
      await waitFor(() => expect(client.getQueryData<BlocksResponse>(key)?.items).toEqual([ next.block, payload.block, before.items[1] ]));
      expect(client.getQueryData<BlocksResponse>(key)?.next_page_params).toEqual(before.next_page_params);
      expect(fetches()).toBe(1);
    });
  }
}

export async function distinctForkHashes() {
  await withBlocks(2, 'reorg', async({ producer, client, container, key, fetches, received }) => {
    const before = client.getQueryData<BlocksResponse>(key)!;
    const payload = await producer.publish(blockEvent(blockAt(before.items[0].height, 'reorg', 1)));
    expect(payload.block.type).toBe('reorg');
    expect(payload.block.hash).not.toBe(before.items[0].hash);
    await waitFor(() => {
      expect(client.getQueryData<BlocksResponse>(key)?.items).toEqual([ payload.block, ...before.items ]);
      expect(container.querySelector(`table a[href="/block/${ payload.block.hash }"]`)).not.toBeNull();
      expect(container.querySelector(`table a[href="/block/${ before.items[0].hash }"]`)).not.toBeNull();
    });

    const canonical = await producer.publish(blockEvent(blockAt(4)));
    expect(canonical.block.type).toBe('block');
    await waitFor(() => expect(received[received.length - 1]).toEqual(canonical));
    expect(client.getQueryData<BlocksResponse>(key)?.items).toEqual([ payload.block, ...before.items ]);
    expect(fetches()).toBe(1);
  });
}

export async function refreshAtOverloadLimit() {
  await withBlocks(51, 'block', async({ producer, client, container, key, fetches }) => {
    const cursor = client.getQueryData<BlocksResponse>(key)!.next_page_params;
    expect(cursor).not.toBeNull();

    for (let height = 52; height <= 76; height++) {
      const payload = await producer.publish(blockEvent(blockAt(height)));
      await waitFor(() => expect(client.getQueryData<BlocksResponse>(key)?.items[0]).toEqual(payload.block));
    }

    expect(client.getQueryData<BlocksResponse>(key)?.items).toHaveLength(75);
    const initial = client.getQueryData<BlocksResponse>(key)!.items[0];
    const refreshed = await producer.publish(blockEvent({ ...initial, transactions_count: 3, price_update_count: 3 }));
    await waitFor(() => expect(client.getQueryData<BlocksResponse>(key)?.items[0]).toEqual(refreshed.block));
    expect(container.querySelector('[data-new-blocks]')?.textContent).toBe('0');

    const replacement = await producer.publish(blockEvent(blockAt(76, 'block', 1)));
    await waitFor(() => expect(client.getQueryData<BlocksResponse>(key)?.items[0]).toEqual(replacement.block));
    const full = client.getQueryData<BlocksResponse>(key)!;
    await act(async() => {
      await producer.publish(blockEvent(blockAt(77)));
    });
    await waitFor(() => expect(container.querySelector('[data-new-blocks]')?.textContent).toBe('1'));
    expect(client.getQueryData<BlocksResponse>(key)).toEqual(full);
    expect(client.getQueryData<BlocksResponse>(key)?.next_page_params).toEqual(cursor);
    expect(fetches()).toBe(1);
  });
}

export async function mixedListCanonicalPromotion() {
  await withBlocks(2, undefined, async({ producer, client, container, key, fetches }) => {
    const before = client.getQueryData<BlocksResponse>(key)!;
    const fork = await producer.publish(blockEvent(blockAt(before.items[0].height, 'reorg', 1)));
    await waitFor(() => expect(client.getQueryData<BlocksResponse>(key)?.items).toEqual([ fork.block, ...before.items ]));
    const promoted = await producer.publish(blockEvent({ ...fork.block, type: 'block' }));
    await waitFor(() => {
      expect(client.getQueryData<BlocksResponse>(key)?.items).toEqual([ promoted.block, before.items[1] ]);
      expect(container.querySelector(`table a[href="/block/${ fork.block.hash }"]`)).toBeNull();
      expect(container.querySelectorAll(`table a[href="/block/${ promoted.block.height }"]`)).toHaveLength(1);
    });
    expect(client.getQueryData<BlocksResponse>(key)?.next_page_params).toEqual(before.next_page_params);
    expect(fetches()).toBe(1);
  });
}

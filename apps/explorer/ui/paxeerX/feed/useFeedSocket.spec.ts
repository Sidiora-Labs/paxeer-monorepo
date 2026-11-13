// @vitest-environment jsdom

import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react';
import { createServer, get } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';
import type { WebSocket } from 'ws';
import { WebSocketServer } from 'ws';

import { SocketProvider, useSocket } from 'lib/socket/context';
import { expect, test } from 'vitest';

import useFeedSocket, { FEED_FALLBACK_REFETCH_INTERVAL } from './useFeedSocket';

type Frame = [ string | null, string | null, string, string, unknown ];
type JoinMode = 'ok' | 'error' | 'hold';

async function createFeedPeer() {
  let polls = 0;
  const server = createServer((_request, response) => {
    response.setHeader('Content-Type', 'application/json');
    response.end(JSON.stringify(++polls));
  });
  const transport = new WebSocketServer({ server });
  const modes = new Map<string, JoinMode>();
  const joins: Array<{ socket: WebSocket; frame: Frame }> = [];

  function reply(join: { socket: WebSocket; frame: Frame }, status: 'ok' | 'error') {
    const [ joinRef, ref, topic ] = join.frame;
    join.socket.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status, response: {} } ]));
  }

  transport.on('connection', (socket) => {
    socket.on('message', (raw) => {
      const frame = JSON.parse(raw.toString()) as Frame;
      const [ , , topic, event ] = frame;
      const message = { socket, frame };
      if (event === 'phx_join') {
        joins.push(message);
        const mode = modes.get(topic) ?? 'ok';
        if (mode !== 'hold') {
          reply(message, mode);
        }
      } else if (event === 'phx_leave' || event === 'heartbeat') {
        reply(message, 'ok');
      }
    });
  });

  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;

  return {
    url: `ws://127.0.0.1:${ port }/socket`,
    modes,
    joins,
    get polls() {
      return polls;
    },
    read: () => new Promise<number>((resolve, reject) => {
      get(`http://127.0.0.1:${ port }/feed`, (response) => {
        let body = '';
        response.on('data', (chunk) => {
          body += chunk.toString();
        });
        response.on('end', () => resolve(JSON.parse(body) as number));
        response.on('error', reject);
      }).on('error', reject);
    }),
    accept: (topic: string) => {
      const join = joins.filter((item) => item.frame[2] === topic).at(-1);
      if (!join) {
        throw new Error(`No join request for ${ topic }`);
      }
      reply(join, 'ok');
    },
    disconnect: () => transport.clients.forEach((socket) => socket.terminate()),
    close: async() => {
      transport.clients.forEach((socket) => socket.terminate());
      await new Promise<void>((resolve, reject) => transport.close((error) => error ? reject(error) : resolve()));
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    },
  };
}

interface FeedProps {
  topic: string;
  isDisabled: boolean;
  isMounted: boolean;
  providerKey: number;
}

function mountFeed(peer: Awaited<ReturnType<typeof createFeedPeer>>, topic: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const options = { timeout: 250, rejoinAfterMs: () => 10_000, reconnectAfterMs: () => 25 };
  let latest: (ReturnType<typeof useFeedSocket> & { data: number | undefined; socket: ReturnType<typeof useSocket> }) | undefined;

  function FeedProbe({ topic, isDisabled }: FeedProps) {
    const feed = useFeedSocket({ topic, isDisabled });
    const query = useQuery({
      queryKey: [ 'feed', topic ],
      queryFn: peer.read,
      refetchInterval: feed.refetchInterval,
      refetchIntervalInBackground: true,
    });
    latest = { ...feed, data: query.data, socket: useSocket() };
    return null;
  }

  function FeedTree(props: FeedProps) {
    return React.createElement(QueryClientProvider, { client },
      React.createElement(SocketProvider, {
        key: props.providerKey,
        url: peer.url,
        options,
        children: props.isMounted ? React.createElement(FeedProbe, props) : null,
      }));
  }

  let props: FeedProps = { topic, isDisabled: false, isMounted: true, providerKey: 0 };
  const mounted = render(React.createElement(FeedTree, props));
  return {
    state: () => {
      if (!latest) {
        throw new Error('Feed has not rendered');
      }
      return latest;
    },
    update: (next: Partial<FeedProps>) => {
      props = { ...props, ...next };
      mounted.rerender(React.createElement(FeedTree, props));
    },
    close: () => {
      mounted.unmount();
      client.clear();
    },
  };
}

async function feedSocketReplacementTimeoutKeepsHttpPolling() {
  const peer = await createFeedPeer();
  const feed = mountFeed(peer, 'paxeer_x_feed:new_update');
  try {
    await waitFor(() => expect(feed.state().refetchInterval).toBe(false));
    const previous = feed.state().channel;
    peer.modes.set('paxeer_x_feed:market:1', 'hold');
    feed.update({ topic: 'paxeer_x_feed:market:1' });
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    expect(feed.state().isSocketDown).toBe(true);
    await waitFor(() => expect(feed.state().channel?.state).toBe('errored'));
    expect(previous?.state).toBe('closed');
    expect(feed.state().socket?.socket?.isConnected()).toBe(true);
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    const polls = peer.polls;
    await waitFor(() => {
      expect(peer.polls).toBeGreaterThan(polls);
      expect(feed.state().data).toBe(peer.polls);
    }, { timeout: FEED_FALLBACK_REFETCH_INTERVAL + 2_000 });
    await act(async() => peer.accept('paxeer_x_feed:market:1'));
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
  } finally {
    feed.close();
    await peer.close();
  }
}

async function feedSocketDisableAndRestartRecoverCurrentChannel() {
  const peer = await createFeedPeer();
  const topic = 'paxeer_x_feed:new_update';
  const feed = mountFeed(peer, topic);
  try {
    await waitFor(() => expect(feed.state().refetchInterval).toBe(false));
    const previous = feed.state().channel;
    feed.update({ isDisabled: true });
    expect(feed.state().refetchInterval).toBe(false);
    expect(feed.state().isSocketDown).toBe(false);
    await waitFor(() => expect(feed.state().channel).toBeUndefined());
    peer.modes.set(topic, 'hold');
    feed.update({ isDisabled: false });
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    await waitFor(() => expect(feed.state().channel?.state).toBe('joining'));
    await waitFor(() => expect(peer.joins).toHaveLength(2));
    expect(feed.state().channel).not.toBe(previous);
    await act(async() => peer.accept(topic));
    await waitFor(() => expect(feed.state().refetchInterval).toBe(false));

    const stopped = feed.state().channel;
    feed.update({ isMounted: false });
    await waitFor(() => expect(stopped?.state).toBe('closed'));
    feed.update({ isMounted: true });
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    await waitFor(() => expect(peer.joins).toHaveLength(3));
    expect(feed.state().channel).not.toBe(stopped);
    await act(async() => peer.accept(topic));
    await waitFor(() => {
      expect(feed.state().refetchInterval).toBe(false);
      expect(feed.state().isSocketDown).toBe(false);
    });

    feed.update({ providerKey: 1 });
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    await waitFor(() => expect(peer.joins).toHaveLength(4));
    await act(async() => peer.accept(topic));
    await waitFor(() => expect(feed.state().refetchInterval).toBe(false));
  } finally {
    feed.close();
    await peer.close();
  }
}

async function feedSocketReconnectFailureKeepsHttpPollingUntilRejoined() {
  const peer = await createFeedPeer();
  const topic = 'paxeer_x_feed:new_update';
  const feed = mountFeed(peer, topic);
  try {
    await waitFor(() => expect(feed.state().refetchInterval).toBe(false));
    peer.modes.set(topic, 'error');
    await act(async() => peer.disconnect());
    await waitFor(() => expect(peer.joins).toHaveLength(2));
    await waitFor(() => expect(feed.state().channel?.state).toBe('errored'));
    expect(feed.state().isSocketDown).toBe(true);
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    const polls = peer.polls;
    await waitFor(() => expect(peer.polls).toBeGreaterThan(polls), { timeout: FEED_FALLBACK_REFETCH_INTERVAL + 2_000 });
    peer.modes.set(topic, 'hold');
    await act(async() => peer.disconnect());
    await waitFor(() => expect(peer.joins).toHaveLength(3));
    expect(feed.state().refetchInterval).toBe(FEED_FALLBACK_REFETCH_INTERVAL);
    await act(async() => peer.accept(topic));
    await waitFor(() => {
      expect(feed.state().refetchInterval).toBe(false);
      expect(feed.state().isSocketDown).toBe(false);
    });
  } finally {
    feed.close();
    await peer.close();
  }
}

test('feedSocketReplacementTimeoutKeepsHttpPolling', { timeout: 15_000 }, async() => {
  await expect(feedSocketReplacementTimeoutKeepsHttpPolling()).resolves.toBeUndefined();
});
test('feedSocketDisableAndRestartRecoverCurrentChannel', { timeout: 10_000 }, async() => {
  await expect(feedSocketDisableAndRestartRecoverCurrentChannel()).resolves.toBeUndefined();
});
test('feedSocketReconnectFailureKeepsHttpPollingUntilRejoined', { timeout: 15_000 }, async() => {
  await expect(feedSocketReconnectFailureKeepsHttpPollingUntilRejoined()).resolves.toBeUndefined();
});

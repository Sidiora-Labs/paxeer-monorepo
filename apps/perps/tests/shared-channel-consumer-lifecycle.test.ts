import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import { setTimeout as delay } from 'node:timers/promises';
import { test, type TestContext } from 'node:test';
import React, { useEffect } from 'react';
import type { Channel, SocketConnectOption } from 'phoenix';

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = 'http://127.0.0.1';
process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = 'http://127.0.0.1';
process.env.NEXT_PUBLIC_PAXSCAN_API_URL = 'http://127.0.0.1/api';
process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = 'ws://127.0.0.1/socket';

const modules = Promise.all([
  import('../src/core/paxscan/socket/context'),
  import('../src/core/paxscan/socket/useSocketChannel'),
  import('../src/core/paxscan/socket/useSocketMessage'),
]);
const require = createRequire(import.meta.url);
const fiberRequire = createRequire(require.resolve('@react-three/fiber'));
const viemRequire = createRequire(require.resolve('viem'));

// An in-memory React host; production hooks and their effects run in the real reconciler.
interface MemoryNode {
  type: string;
  props: object;
  children: MemoryNode[];
}
function append(parent: MemoryNode, child: MemoryNode) {
  const previous = parent.children.indexOf(child);
  if (previous >= 0) parent.children.splice(previous, 1);
  parent.children.push(child);
}
function remove(parent: MemoryNode, child: MemoryNode) {
  const index = parent.children.indexOf(child);
  assert.ok(index >= 0);
  parent.children.splice(index, 1);
}
function insert(parent: MemoryNode, child: MemoryNode, before: MemoryNode) {
  const previous = parent.children.indexOf(child);
  if (previous >= 0) parent.children.splice(previous, 1);
  const index = parent.children.indexOf(before);
  assert.ok(index >= 0);
  parent.children.splice(index, 0, child);
}
const memoryHost = {
  now: performance.now,
  supportsMutation: true,
  supportsPersistence: false,
  supportsHydration: false,
  isPrimaryRenderer: false,
  noTimeout: -1,
  scheduleTimeout: setTimeout,
  cancelTimeout: clearTimeout,
  getRootHostContext: () => null,
  getChildHostContext: () => null,
  getPublicInstance: (node: MemoryNode) => node,
  prepareForCommit: () => null,
  resetAfterCommit() {},
  createInstance: (type: string, props: object): MemoryNode => ({ type, props, children: [] }),
  createTextInstance: (text: string): MemoryNode => ({ type: '#text', props: { text }, children: [] }),
  appendInitialChild: append,
  appendChild: append,
  appendChildToContainer: append,
  removeChild: remove,
  removeChildFromContainer: remove,
  insertBefore: insert,
  insertInContainerBefore: insert,
  finalizeInitialChildren: () => false,
  shouldSetTextContent: () => false,
  prepareUpdate: () => true,
  commitUpdate(node: MemoryNode, _update: boolean, _type: string, _oldProps: object, props: object) {
    node.props = props;
  },
  commitTextUpdate(node: MemoryNode, _oldText: string, text: string) { node.props = { text }; },
  clearContainer(container: MemoryNode) { container.children.length = 0; },
  detachDeletedInstance(node: MemoryNode) { node.children.length = 0; },
  getCurrentEventPriority: () => 16,
};
interface MemoryRenderer {
  createContainer(container: MemoryNode, tag: number, hydration: null, strict: boolean,
    concurrent: null, prefix: string, recover: (error: Error) => void, transitions: null): object;
  updateContainer(element: React.ReactNode, root: object, parent: null, callback: null): void;
  flushPassiveEffects(): boolean;
  flushSync(callback: () => void): void;
}
const createRenderer = fiberRequire('react-reconciler') as (host: typeof memoryHost) => MemoryRenderer;

interface WireMessage {
  join_ref: string | null;
  ref: string | null;
  topic: string;
  event: string;
  payload: Record<string, unknown>;
}
interface Peer {
  on(event: 'message', handler: (data: { toString(): string }) => void): void;
  on(event: 'close', handler: () => void): void;
  send(data: string): void;
  terminate(): void;
}
interface LoopbackServer extends NodeJS.EventEmitter {
  address(): AddressInfo;
  clients: Set<Peer>;
  close(callback: (error?: Error) => void): void;
}
const { WebSocketServer } = viemRequire('ws') as {
  WebSocketServer: new(options: { host: string; port: number }) => LoopbackServer;
};
const { Serializer } = require('phoenix') as {
  Serializer: {
    encode(message: WireMessage, callback: (data: string) => void): void;
    decode(data: string, callback: (message: WireMessage) => void): void;
  };
};

async function waitFor(predicate: () => boolean, description: string) {
  const deadline = Date.now() + 3000;
  while (!predicate()) {
    assert.ok(Date.now() < deadline, `Timed out: ${description}`);
    await delay(5);
  }
}

async function fixture(t: TestContext) {
  const server = new WebSocketServer({ host: '127.0.0.1', port: 0 });
  const frames: Array<WireMessage & { peer: Peer }> = [];
  const memberships = new Map<Peer, Map<string, WireMessage>>();
  const send = (peer: Peer, message: WireMessage) => Serializer.encode(message, data => peer.send(data));
  server.on('connection', (peer: Peer) => {
    const topics = new Map<string, WireMessage>();
    memberships.set(peer, topics);
    peer.on('close', () => { memberships.delete(peer); });
    peer.on('message', data => Serializer.decode(data.toString(), message => {
      frames.push({ ...message, peer });
      if (message.event === 'phx_join') {
        topics.set(message.topic, message);
      } else if (message.event === 'phx_leave') {
        topics.delete(message.topic);
      } else if (message.event === 'fixture_publish') {
        const source = topics.get(message.topic);
        assert.ok(source, 'producer must still own its joined topic');
        for (const [recipient, subscriptions] of memberships) {
          const subscription = subscriptions.get(message.topic);
          if (subscription && JSON.stringify(subscription.payload) === JSON.stringify(source.payload)) {
            send(recipient, {
              join_ref: subscription.join_ref, ref: null, topic: message.topic,
              event: 'balance', payload: message.payload,
            });
          }
        }
      }
      send(peer, {
        ...message, event: 'phx_reply',
        payload: { status: 'ok', response: message.event === 'phx_join' ? { accepted: message.payload } : {} },
      });
    }));
  });
  await once(server, 'listening');
  const origin = `http://127.0.0.1:${server.address().port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = `${origin.replace('http:', 'ws:')}/socket`;
  const [ { PaxscanSocketProvider, usePaxscanSocket }, { usePaxscanSocketChannel }, { usePaxscanSocketMessage } ] = await modules;
  const renderer = createRenderer(memoryHost);
  const container: MemoryNode = { type: 'root', props: {}, children: [] };
  const root = renderer.createContainer(container, 0, null, false, null, '', error => { throw error; }, null);
  function render(element: React.ReactNode) {
    renderer.flushSync(() => renderer.updateContainer(element, root, null, null));
    while (renderer.flushPassiveEffects()) { /* Flush updates from provider and subscription effects. */ }
  }
  t.after(async () => {
    render(null);
    for (const peer of server.clients) peer.terminate();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  });

  const channels = new Map<string, Channel | undefined>();
  const received = new Map<string, unknown[]>();
  const joined = new Map<string, unknown[]>();
  let registry: ReturnType<typeof usePaxscanSocket>;
  interface ConsumerProps { id: string; disabled?: boolean; params?: object; topic?: string }
  function Consumer({ id, disabled = false, params, topic = 'addresses:shared' }: ConsumerProps) {
    registry = usePaxscanSocket();
    const channel = usePaxscanSocketChannel({
      topic, params, isDisabled: disabled,
      onJoin: (_channel, response) => { joined.set(id, [...(joined.get(id) ?? []), response]); },
    });
    useEffect(() => { channels.set(id, channel); }, [ id, channel ]);
    usePaxscanSocketMessage({
      channel, event: 'balance',
      handler: payload => { received.set(id, [...(received.get(id) ?? []), payload]); },
    });
    return React.createElement('consumer', { id, disabled });
  }
  const options: Partial<SocketConnectOption> = { transport: WebSocket, timeout: 1500 };
  function mount(consumers: ConsumerProps[]) {
    render(React.createElement(PaxscanSocketProvider, {
      url: process.env.NEXT_PUBLIC_PAXSCAN_WS_URL, options,
      children: consumers.map(props => React.createElement(Consumer, { ...props, key: props.id })),
    }));
  }
  function channel(id: string) {
    const channel = channels.get(id);
    assert.ok(channel, `${id} must have a channel`);
    return channel;
  }
  async function publish(id: string, balance: string) {
    await new Promise<void>((resolve, reject) => channel(id).push('fixture_publish', { balance })
      .receive('ok', () => resolve())
      .receive('error', error => reject(new Error(JSON.stringify(error))))
      .receive('timeout', () => reject(new Error('publish timed out'))));
  }
  return {
    mount, channel, channels, received, joined, publish, frames, server, memberships,
    registry: () => { assert.ok(registry); return registry.channelRegistry.current; },
    joins: () => frames.filter(frame => frame.event === 'phx_join'),
    leaves: () => frames.filter(frame => frame.event === 'phx_leave'),
  };
}

async function sharedChannelLeavePreservesOtherConsumer(t: TestContext) {
  const f = await fixture(t);
  f.mount([{ id: 'first' }, { id: 'second' }]);
  await waitFor(() => f.channel('second').state === 'joined', 'shared join');
  assert.equal(f.channel('first'), f.channel('second'));
  assert.equal(f.joins().length, 1);
  assert.equal(Object.values(f.registry())[0].subscribers, 2);
  await f.publish('second', '10');
  await waitFor(() => f.received.get('first')?.length === 1 && f.received.get('second')?.length === 1, 'both consumers receive');

  f.mount([{ id: 'first', disabled: true }, { id: 'second' }]);
  assert.equal(f.channels.get('first'), undefined);
  assert.equal(f.channel('second').state, 'joined');
  assert.equal(Object.values(f.registry())[0].subscribers, 1);
  await f.publish('second', '20');
  await waitFor(() => f.received.get('second')?.length === 2, 'remaining consumer receives after first disables');
  assert.deepEqual(f.received.get('first'), [{ balance: '10' }]);
  assert.deepEqual(f.received.get('second'), [{ balance: '10' }, { balance: '20' }]);
  assert.equal(f.leaves().length, 0);
  assert.equal(f.joins().length, 1);

  f.mount([{ id: 'second' }]);
  assert.equal(f.channel('second').state, 'joined');
  f.mount([{ id: 'second', disabled: true }]);
  await waitFor(() => f.leaves().length === 1 && f.server.clients.size === 0, 'last owner leaves and disconnects');
  assert.equal(Object.keys(f.registry()).length, 0);
}

async function channelJoinParamsReachTransportAndSeparateIdentities(t: TestContext) {
  const f = await fixture(t);
  const alpha = { token: 'alpha', filter: { assets: ['PAX', 'USDC'], confirmed: true }, cursor: null };
  const equivalent = { cursor: null, filter: { confirmed: true, assets: ['PAX', 'USDC'] }, token: 'alpha' };
  const beta = { ...alpha, token: 'beta' };
  f.mount([{ id: 'alpha', params: alpha }, { id: 'equivalent', params: equivalent }]);
  await waitFor(() => f.channel('alpha').state === 'joined', 'alpha joins');
  const shared = f.channel('alpha');
  assert.equal(shared, f.channel('equivalent'));
  assert.deepEqual(f.joins().map(join => join.payload), [equivalent]);
  assert.deepEqual(f.joined.get('alpha'), [{ accepted: equivalent }]);

  // Join the incompatible subscription after the first is fully joined: Phoenix
  // would leave it if the two identities used the same socket connection.
  f.mount([{ id: 'alpha', params: equivalent }, { id: 'equivalent', params: alpha }, { id: 'beta', params: beta }]);
  await waitFor(() => f.channel('beta').state === 'joined', 'beta joins with its own params');
  assert.equal(f.channel('alpha'), shared, 'equivalent parameter reorder must not rejoin');
  assert.notEqual(f.channel('beta'), shared);
  assert.equal(shared.state, 'joined', 'different params must not evict the existing Phoenix channel');
  assert.equal(f.joins().length, 2);
  assert.deepEqual(f.joins().map(join => join.payload), [equivalent, beta]);
  assert.notEqual(f.joins()[0].peer, f.joins()[1].peer);
  assert.deepEqual(f.joined.get('beta'), [{ accepted: beta }]);
  assert.equal(Object.keys(f.registry()).length, 2);

  await f.publish('alpha', '11');
  await f.publish('beta', '22');
  await waitFor(() => f.received.get('equivalent')?.length === 1 && f.received.get('beta')?.length === 1, 'isolated deliveries');
  assert.deepEqual(f.received.get('alpha'), [{ balance: '11' }]);
  assert.deepEqual(f.received.get('equivalent'), [{ balance: '11' }]);
  assert.deepEqual(f.received.get('beta'), [{ balance: '22' }]);

  f.mount([{ id: 'alpha', params: beta }, { id: 'equivalent', params: equivalent, disabled: true }, { id: 'beta', params: beta }]);
  assert.equal(f.channel('alpha'), f.channel('beta'), 'changing params acquires the matching identity');
  await f.publish('beta', '33');
  await waitFor(() => f.received.get('alpha')?.length === 2, 'migrated consumer receives beta');
  assert.deepEqual(f.received.get('alpha'), [{ balance: '11' }, { balance: '33' }]);
  assert.deepEqual(f.received.get('equivalent'), [{ balance: '11' }]);
  assert.equal(f.joins().length, 2);
  assert.equal(Object.values(f.registry())[0].subscribers, 2);
  f.mount([]);
  await waitFor(() => f.leaves().length === 2 && f.server.clients.size === 0, 'both identities cleaned up');
  assert.equal(Object.keys(f.registry()).length, 0);
}

async function channelJoinParamsRejectLossyPayloads() {
  const [, { serializeJoinParams }] = await modules;
  const cycle: Record<string, unknown> = {};
  cycle.self = cycle;
  for (const params of [
    { value: undefined }, { value: NaN }, { value: Infinity }, { value: 1n },
    { value: () => 'secret' }, { value: Symbol('secret') }, { value: new Date() },
    { value: new Array(1) }, cycle, { get token() { return 'secret'; } },
    { [Symbol('token')]: 'secret' }, [],
  ]) assert.throws(() => serializeJoinParams(params), TypeError);
  assert.equal(serializeJoinParams(), '{}');
  assert.equal(serializeJoinParams({ b: 2, a: { z: false, a: null } }), '{"a":{"a":null,"z":false},"b":2}');
  assert.notEqual(serializeJoinParams({ assets: ['PAX', 'USDC'] }), serializeJoinParams({ assets: ['USDC', 'PAX'] }));
  assert.equal(serializeJoinParams(JSON.parse('{"__proto__":{"token":"alpha"}}')), '{"__proto__":{"token":"alpha"}}');
}

test(sharedChannelLeavePreservesOtherConsumer);
test(channelJoinParamsReachTransportAndSeparateIdentities);
test(channelJoinParamsRejectLossyPayloads);

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { createHash } from 'node:crypto';
import type { Duplex } from 'node:stream';
import type { AddressInfo } from 'node:net';

interface Frame {
  type: string;
  id?: string;
  payload?: unknown;
}

class GraphQLPeer {
  received: Frame[] = [];
  ended = false;
  private buffer = Buffer.alloc(0);

  constructor(private readonly socket: Duplex) {
    socket.on('data', chunk => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      this.drain();
    });
    socket.on('close', () => { this.ended = true; });
    socket.on('error', () => { this.ended = true; });
  }

  private drain() {
    for (;;) {
      if (this.buffer.length < 2) return;
      const opcode = this.buffer[0]! & 0x0f;
      let length = this.buffer[1]! & 0x7f;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      } else if (length === 127) {
        if (this.buffer.length < 10) return;
        length = Number(this.buffer.readBigUInt64BE(2));
        offset = 10;
      }
      if (this.buffer.length < offset + 4 + length) return;
      const mask = this.buffer.subarray(offset, offset + 4);
      const payload = Buffer.from(this.buffer.subarray(offset + 4, offset + 4 + length));
      for (let i = 0; i < payload.length; i++) payload[i] = payload[i]! ^ mask[i % 4]!;
      this.buffer = this.buffer.subarray(offset + 4 + length);
      if (opcode === 1) {
        const frame = JSON.parse(payload.toString('utf8')) as Frame;
        this.received.push(frame);
        if (frame.type === 'connection_init') this.send({ type: 'connection_ack' });
        if (frame.type === 'ping') this.send({ type: 'pong' });
      } else if (opcode === 8) {
        this.writeFrame(0x88, payload);
        this.socket.end();
      }
    }
  }

  private writeFrame(opcode: number, payload: Buffer) {
    const header = payload.length < 126
      ? Buffer.from([opcode, payload.length])
      : Buffer.from([opcode, 126, payload.length >> 8, payload.length & 0xff]);
    this.socket.write(Buffer.concat([header, payload]));
  }

  send(frame: Frame) { this.writeFrame(0x81, Buffer.from(JSON.stringify(frame))); }
  destroy() { this.socket.destroy(); }
  subscribes() { return this.received.filter(frame => frame.type === 'subscribe'); }
}

async function waitFor<T>(probe: () => T | undefined | false | null, label: string): Promise<T> {
  const deadline = Date.now() + 5000;
  for (;;) {
    const value = probe();
    if (value) return value;
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${label}`);
    await new Promise(resolve => setTimeout(resolve, 5));
  }
}

async function resubscribesAfterTerminalLossError() {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  const previousDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  const page = Object.assign(new EventTarget(), { hidden: false });
  Object.defineProperty(globalThis, 'window', { configurable: true, value: new EventTarget() });
  Object.defineProperty(globalThis, 'document', { configurable: true, value: page });
  const server = createServer();
  const peers: GraphQLPeer[] = [];
  const releases: (() => void)[] = [];
  server.on('upgrade', (req, socket) => {
    assert.equal(req.headers['sec-websocket-protocol'], 'graphql-transport-ws');
    const accept = createHash('sha1')
      .update(`${req.headers['sec-websocket-key']}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`)
      .digest('base64');
    socket.write(`HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\nSec-WebSocket-Protocol: graphql-transport-ws\r\n\r\n`);
    peers.push(new GraphQLPeer(socket));
  });

  try {
    await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
    const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}/graphql`;
    const { gqlEndpoint, subscribeStream } = await import('../src/core/graphql/client');
    const { onStreamResync } = await import('../src/core/net/realtime');
    const endpoint = gqlEndpoint(url, { wsUrl: url.replace('http:', 'ws:') });
    const query = 'subscription OrderChanged($account: String!) { orderChanged(account: $account) { id } }';
    const variables = { account: 'trader' };
    const values: { orderChanged: { id: string } }[] = [];
    const errors: Error[] = [];
    const resyncs: string[] = [];
    const events: string[] = [];
    releases.push(onStreamResync(wsUrl => { resyncs.push(wsUrl); events.push('resync'); }));
    const unsubscribe = subscribeStream<{ orderChanged: { id: string } }>({
      endpoint, query, variables,
      onData: value => { values.push(value); },
      onError: error => { errors.push(error); events.push('error'); },
    });
    releases.push(unsubscribe);
    const peer = await waitFor(() => peers[0], 'socket');
    await waitFor(() => peer.subscribes().length === 1, 'initial subscribe');
    const initial = peer.subscribes()[0]!;
    assert.ok(initial.id);
    assert.deepEqual(initial.payload, { query, variables });

    const message = 'subscription lost stream messages; subscribe again';
    peer.send({ id: initial.id, type: 'error', payload: [{ message }] });
    await waitFor(() => peer.subscribes().length === 2, 'resubscribe after loss');
    assert.deepEqual(peer.subscribes()[1], initial);
    assert.deepEqual(errors.map(error => error.message), [message]);
    assert.deepEqual(events, ['error', 'resync']);
    assert.deepEqual(resyncs, [endpoint.wsUrl]);
    assert.equal(peers.length, 1);
    assert.equal(peer.ended, false);
    const recovered = { orderChanged: { id: 'after-error' } };
    peer.send({ id: initial.id, type: 'next', payload: { data: recovered } });
    await waitFor(() => values.length === 1, 'data after loss');
    assert.deepEqual(values, [recovered]);

    peer.send({ id: initial.id, type: 'complete' });
    await waitFor(() => peer.subscribes().length === 3, 'resubscribe after complete');
    assert.deepEqual(peer.subscribes()[2], initial);
    assert.deepEqual(resyncs, [endpoint.wsUrl, endpoint.wsUrl]);
    assert.equal(errors.length, 1);
    const completed = { orderChanged: { id: 'after-complete' } };
    peer.send({ id: initial.id, type: 'next', payload: { data: completed } });
    await waitFor(() => values.length === 2, 'data after complete');
    assert.deepEqual(values, [recovered, completed]);
    assert.equal(peers.length, 1);
    assert.equal(peer.ended, false);

    const sentinel: unknown[] = [];
    releases.push(subscribeStream({ endpoint, query, variables: { account: 'other' }, onData: value => { sentinel.push(value); } }));
    await waitFor(() => peer.subscribes().length === 4, 'second active subscription');
    const otherId = peer.subscribes()[3]!.id;
    page.hidden = true;
    page.dispatchEvent(new Event('visibilitychange'));
    peer.send({ id: initial.id, type: 'complete' });
    await waitFor(() => resyncs.length === 3, 'resync before cleanup');
    assert.equal(peer.subscribes().length, 4);
    unsubscribe();
    releases.splice(releases.indexOf(unsubscribe), 1);
    await waitFor(() => peer.received.some(frame => frame.id === initial.id && frame.type === 'complete'), 'client complete');
    page.hidden = false;
    page.dispatchEvent(new Event('visibilitychange'));
    await new Promise(resolve => setTimeout(resolve, 4100));
    peer.send({ id: otherId, type: 'next', payload: { data: completed } });
    await waitFor(() => sentinel.length === 1, 'remaining subscription data');
    assert.equal(peer.subscribes().length, 4);
    assert.deepEqual(sentinel, [completed]);
    assert.equal(peers.length, 1);
    assert.equal(peer.ended, false);

    peer.send({ id: initial.id, type: 'error', payload: [{ message }] });
    peer.send({ id: initial.id, type: 'complete' });
    peer.send({ id: otherId, type: 'next', payload: { data: recovered } });
    await waitFor(() => sentinel.length === 2, 'data after stale terminal frames');
    assert.equal(resyncs.length, 3);
    assert.equal(errors.length, 1);
    assert.equal(peer.subscribes().length, 4);
  } finally {
    releases.reverse().forEach(release => release());
    peers.forEach(peer => peer.destroy());
    await new Promise<void>(resolve => server.close(() => resolve()));
    await waitFor(() => peers.every(peer => peer.ended), 'sockets to close');
    if (previousWindow) Object.defineProperty(globalThis, 'window', previousWindow);
    else Reflect.deleteProperty(globalThis, 'window');
    if (previousDocument) Object.defineProperty(globalThis, 'document', previousDocument);
    else Reflect.deleteProperty(globalThis, 'document');
  }
}

test(resubscribesAfterTerminalLossError);

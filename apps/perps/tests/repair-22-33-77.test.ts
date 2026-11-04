import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createServer, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import type { Duplex } from "node:stream";
import { test } from "node:test";

const query = "query Account($account: String!) { account(id: $account) { owner } }";
const variables = { account: "current" };
type Account = { account: { owner: string } };
const account = (owner: string): Account => ({ account: { owner } });

async function waitFor(probe: () => boolean, label: string) {
  const deadline = Date.now() + 5000;
  while (!probe()) {
    if (Date.now() >= deadline) throw new Error(`Timed out: ${label}`);
    await new Promise(resolve => setTimeout(resolve, 5));
  }
}

async function fixture() {
  const received: { owner: string; body: unknown; response: ServerResponse }[] = [];
  const held = new Set<string>();
  const server = createServer(async (request, response) => {
    let body = "";
    for await (const chunk of request) body += chunk;
    const owner = request.headers.authorization?.replace(/^Bearer /, "") ?? "anonymous";
    received.push({ owner, body: JSON.parse(body), response });
    if (!held.has(owner)) respond(response, owner);
  });
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  const client = await import("../src/core/graphql/client");
  return {
    server, origin, client, received, held,
    release(owner: string) {
      held.delete(owner);
      received.filter(row => row.owner === owner).forEach(row => respond(row.response, owner));
    },
    async close() {
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    },
  };
}

function respond(response: ServerResponse, owner: string) {
  response.writeHead(200, { "Content-Type": "application/json" });
  response.end(JSON.stringify({ data: account(owner) }));
}

async function graphqlAuthCacheIsolatesConcurrentSessions() {
  const f = await fixture();
  try {
    f.held.add("session-a");
    const a = f.client.gqlEndpoint(`${f.origin}/graphql`, {
      authGeneration: 1, authHeaders: () => ({ Authorization: "Bearer session-a" }),
    });
    const b = f.client.gqlEndpoint(`${f.origin}/graphql`, {
      authGeneration: 1, authHeaders: () => ({ Authorization: "Bearer session-b" }),
    });
    const pending = f.client.gqlQuery<Account>(a, query, variables);
    assert.equal(f.client.gqlQuery(a, query, { ...variables }), pending);
    await waitFor(() => f.received.length === 1, "first session request");
    const other = f.client.gqlQuery<Account>(b, query, variables);
    assert.notEqual(other, pending);
    assert.deepEqual(await other, account("session-b"));
    f.release("session-a");
    assert.deepEqual(await pending, account("session-a"));
    assert.deepEqual(f.received.map(row => row.owner), ["session-a", "session-b"]);
    f.received.forEach(row => assert.deepEqual(row.body, { query, variables }));
  } finally { await f.close(); }
}

async function graphqlAuthGenerationCancelsPreviousRequests() {
  const f = await fixture();
  try {
    let owner = "session-a";
    f.held.add(owner);
    const endpoint = f.client.gqlEndpoint(`${f.origin}/graphql`, {
      authGeneration: 1, authHeaders: () => ({ Authorization: `Bearer ${owner}` }),
    });
    const old = f.client.gqlQuery(endpoint, query, variables);
    const rejected = assert.rejects(old, /Request cancelled|GraphQL authentication changed/);
    await waitFor(() => f.received.length === 1, "held old request");
    owner = "session-b";
    endpoint.authGeneration = 2;
    assert.deepEqual(await f.client.gqlQuery(endpoint, query, variables), account(owner));
    await rejected;
    f.release("session-a");
    assert.deepEqual(f.received.map(row => row.owner), ["session-a", "session-b"]);
  } finally { await f.close(); }
}

async function graphqlAuthCacheClearsSnapshotsAndRestartsSubscribers() {
  const f = await fixture();
  const cleanups: (() => void)[] = [];
  try {
    let owner = "session-a";
    const endpoint = f.client.gqlEndpoint(`${f.origin}/graphql`, {
      authGeneration: 1, authHeaders: () => ({ Authorization: `Bearer ${owner}` }),
    });
    const active: Account[] = [], queued: Account[] = [], joined: Account[] = [], independent: Account[] = [];
    const errors: Error[] = [];
    const subscribe = (values: Account[]) => f.client.subscribeQuery<Account>({
      endpoint, query, variables, intervalMs: 0,
      onData: value => { values.push(value); }, onError: error => { errors.push(error); },
    });
    cleanups.push(subscribe(active));
    await waitFor(() => active.length === 1, "initial cached snapshot");
    assert.deepEqual(active, [account("session-a")]);
    // This replay is queued before the identity changes and must be discarded.
    cleanups.push(subscribe(queued));
    owner = "session-b";
    f.held.add(owner);
    endpoint.authGeneration = 2;
    cleanups.push(subscribe(joined));
    await waitFor(() => f.received.some(row => row.owner === owner), "automatic protected query restart");
    assert.deepEqual(queued, []);
    assert.deepEqual(joined, []);
    // A second identity switch must not wait for the old in-flight poll.
    owner = "session-c";
    endpoint.authGeneration = 3;
    await waitFor(() => joined.length === 1, "new identity snapshot");
    f.release("session-b");
    assert.deepEqual(active, [account("session-a"), account("session-c")]);
    assert.deepEqual(queued, [account("session-c")]);
    assert.deepEqual(joined, [account("session-c")]);
    const separate = f.client.gqlEndpoint(endpoint.url, {
      authGeneration: 3, authHeaders: () => ({ Authorization: "Bearer independent-session" }),
    });
    cleanups.push(f.client.subscribeQuery<Account>({
      endpoint: separate, query, variables, intervalMs: 0,
      onData: value => { independent.push(value); },
    }));
    await waitFor(() => independent.length === 1, "isolated poller at same URL");
    assert.deepEqual(independent, [account("independent-session")]);
    assert.deepEqual(joined, [account("session-c")]);
    assert.deepEqual(errors, []);
    const replay: Account[] = [];
    cleanups.push(subscribe(replay));
    await waitFor(() => replay.length === 1, "current generation cache replay");
    assert.deepEqual(replay, [account("session-c")]);
    assert.equal(f.received.filter(row => row.owner === "session-c").length, 1);
    // Logout clears the protected cache and refreshes existing consumers too.
    owner = "anonymous";
    endpoint.authHeaders = () => ({});
    endpoint.authGeneration = 4;
    await waitFor(() => joined.length === 2, "logout snapshot");
    assert.deepEqual(joined, [account("session-c"), account("anonymous")]);
    assert.deepEqual(errors, []);
  } finally {
    cleanups.reverse().forEach(cleanup => cleanup());
    await f.close();
  }
}

interface Frame {
  type: string;
  id?: string;
  payload?: { Authorization?: string; query?: string; variables?: Record<string, unknown>; data?: Account };
}

// A real loopback graphql-transport-ws peer; no browser or transport replacement.
class AuthenticatedGraphQLPeer {
  frames: Frame[] = [];
  owner = "anonymous";
  ended = false;
  private buffer = Buffer.alloc(0);
  constructor(private readonly socket: Duplex) {
    socket.on("data", chunk => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      this.drain();
    });
    socket.on("close", () => { this.ended = true; });
    socket.on("error", () => { this.ended = true; });
  }
  private drain() {
    for (;;) {
      if (this.buffer.length < 2) return;
      const opcode = this.buffer[0]! & 0x0f;
      let length = this.buffer[1]! & 0x7f;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2); offset = 4;
      } else if (length === 127) {
        if (this.buffer.length < 10) return;
        length = Number(this.buffer.readBigUInt64BE(2)); offset = 10;
      }
      if (this.buffer.length < offset + 4 + length) return;
      const mask = this.buffer.subarray(offset, offset + 4);
      const payload = Buffer.from(this.buffer.subarray(offset + 4, offset + 4 + length));
      for (let i = 0; i < payload.length; i++) payload[i] = payload[i]! ^ mask[i % 4]!;
      this.buffer = this.buffer.subarray(offset + 4 + length);
      if (opcode === 8) { this.socket.end(Buffer.from([0x88, 0])); return; }
      if (opcode !== 1) continue;
      const frame = JSON.parse(payload.toString()) as Frame;
      this.frames.push(frame);
      if (frame.type === "connection_init") {
        this.owner = frame.payload?.Authorization?.replace(/^Bearer /, "") ?? "anonymous";
        this.send({ type: "connection_ack" });
      }
      if (frame.type === "subscribe") this.send({ id: frame.id, type: "next", payload: { data: account(this.owner) } });
      if (frame.type === "ping") this.send({ type: "pong" });
    }
  }
  send(frame: Frame) {
    if (this.socket.destroyed || this.socket.writableEnded) return;
    const payload = Buffer.from(JSON.stringify(frame));
    const header = payload.length < 126 ? Buffer.from([0x81, payload.length])
      : Buffer.from([0x81, 126, payload.length >> 8, payload.length & 0xff]);
    this.socket.write(Buffer.concat([header, payload]));
  }
  destroy() { this.socket.destroy(); }
}

async function graphqlAuthGenerationIsolatesAndRestartsConnections() {
  const f = await fixture();
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  const previousDocument = Object.getOwnPropertyDescriptor(globalThis, "document");
  Object.defineProperty(globalThis, "window", { configurable: true, value: new EventTarget() });
  Object.defineProperty(globalThis, "document", { configurable: true, value: Object.assign(new EventTarget(), { hidden: false }) });
  const peers: AuthenticatedGraphQLPeer[] = [];
  const cleanups: (() => void)[] = [];
  f.server.on("upgrade", (request, socket) => {
    const accept = createHash("sha1").update(`${request.headers["sec-websocket-key"]}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`).digest("base64");
    socket.write(`HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\nSec-WebSocket-Protocol: graphql-transport-ws\r\n\r\n`);
    peers.push(new AuthenticatedGraphQLPeer(socket));
  });
  try {
    let owner = "session-a";
    const endpoint = f.client.gqlEndpoint(`${f.origin}/graphql`, {
      wsUrl: `${f.origin.replace("http:", "ws:")}/graphql`, authGeneration: 1,
      authHeaders: () => ({ Authorization: `Bearer ${owner}` }),
    });
    const other = f.client.gqlEndpoint(endpoint.url, {
      wsUrl: endpoint.wsUrl, authGeneration: 1,
      authHeaders: () => ({ Authorization: "Bearer independent-session" }),
    });
    const active: Account[] = [], multiplexed: Account[] = [], independent: Account[] = [];
    const errors: Error[] = [];
    const subscription = "subscription Account { account { owner } }";
    const subscribe = (target: typeof endpoint, values: Account[]) => f.client.subscribeStream<Account>({
      endpoint: target, query: subscription, variables,
      onData: value => { values.push(value); }, onError: error => { errors.push(error); },
    });
    cleanups.push(subscribe(endpoint, active), subscribe(endpoint, multiplexed), subscribe(other, independent));
    await waitFor(() => active.length === 1 && multiplexed.length === 1 && independent.length === 1, "isolated authenticated streams");
    assert.deepEqual(active, [account("session-a")]);
    assert.deepEqual(multiplexed, [account("session-a")]);
    assert.deepEqual(independent, [account("independent-session")]);
    assert.equal(peers.length, 2);
    const oldPeer = peers.find(peer => peer.owner === "session-a")!;
    const oldSubscription = oldPeer.frames.find(frame => frame.type === "subscribe")!;
    assert.deepEqual(oldSubscription.payload, { query: subscription, variables });
    owner = "session-b";
    endpoint.authGeneration = 2;
    // A frame already sent on the old socket cannot enter the new session.
    oldPeer.send({ id: oldSubscription.id, type: "next", payload: { data: account("stale-session-a") } });
    await waitFor(() => active.length === 2 && multiplexed.length === 2 && oldPeer.ended, "rotated connection and subscriptions");
    assert.deepEqual(active, [account("session-a"), account("session-b")]);
    assert.deepEqual(multiplexed, active);
    assert.deepEqual(independent, [account("independent-session")]);
    assert.equal(peers.length, 3);
    const current = peers.find(peer => peer.owner === "session-b")!;
    assert.equal(current.frames.filter(frame => frame.type === "subscribe").length, 2);
    endpoint.authGeneration = 2;
    const joined: Account[] = [];
    cleanups.push(subscribe(endpoint, joined));
    await waitFor(() => joined.length === 1, "same generation multiplexing");
    assert.deepEqual(joined, [account("session-b")]);
    assert.equal(peers.length, 3);
    assert.deepEqual(errors, []);
  } finally {
    cleanups.reverse().forEach(cleanup => cleanup());
    peers.forEach(peer => peer.destroy());
    await waitFor(() => peers.every(peer => peer.ended), "closed stream peers");
    await f.close();
    if (previousWindow) Object.defineProperty(globalThis, "window", previousWindow);
    else Reflect.deleteProperty(globalThis, "window");
    if (previousDocument) Object.defineProperty(globalThis, "document", previousDocument);
    else Reflect.deleteProperty(globalThis, "document");
  }
}

test(graphqlAuthCacheIsolatesConcurrentSessions);
test(graphqlAuthGenerationCancelsPreviousRequests);
test(graphqlAuthCacheClearsSnapshotsAndRestartsSubscribers);
test(graphqlAuthGenerationIsolatesAndRestartsConnections);

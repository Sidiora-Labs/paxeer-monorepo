import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { createServer as httpServer } from "node:http";
import { createServer as tcpServer, connect } from "node:net";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { PeerIdentity } from "../src/identity.mjs";
import { SignalClient } from "../src/transport.mjs";
import { RoomHost, manifest, vector } from "./host-signal.mjs";
import {
  DealClient,
  dealDigest,
  encodeActivation,
  decodeActivation,
  loadLayerRange,
} from "../src/deal.mjs";
import { encodeFrame, decodeFrame, MAX_FRAME_BYTES } from "../src/wire.mjs";
import {
  ScriptedRuntime,
  scriptedModelSource,
  scriptedEmbedding,
  scriptedToken,
} from "../tests/scripted_model.mjs";

function fixed(n) {
  return Array(32).fill(n);
}
function toHex(bytes) {
  return Buffer.from(bytes).toString("hex");
}
const roomId = toHex(fixed(4));

// Socket endpoints carry the actual bounded room codec over loopback TCP.
// They implement the mesh event/send contract without emulating WebRTC APIs.
class SocketEndpoint extends EventTarget {
  sockets = new Map();
  sequence = 0;
  constructor(key) {
    super();
    this.key = key;
  }
  attach(peer, socket) {
    this.sockets.set(peer, socket);
    let pending = Buffer.alloc(0);
    socket.on("data", (bytes) => {
      pending = Buffer.concat([pending, bytes]);
      while (pending.length >= 6) {
        const size = pending.readUInt32BE(2) + 6;
        assert.ok(size <= MAX_FRAME_BYTES);
        if (pending.length < size) break;
        const frame = decodeFrame(pending.subarray(0, size));
        pending = pending.subarray(size);
        this.dispatchEvent(
          new CustomEvent("frame", { detail: { peerKey: peer, frame } }),
        );
      }
    });
    socket.on("error", (error) =>
      this.dispatchEvent(new CustomEvent("error", { detail: error })),
    );
  }
  send(peer, kind, payload) {
    const socket = this.sockets.get(peer);
    if (!socket || socket.destroyed)
      throw new Error("Socket peer is unavailable");
    const bytes = encodeFrame({
      kind,
      roomId,
      sequence: this.sequence++,
      payload,
    });
    return new Promise((resolve, reject) =>
      socket.write(bytes, (error) => (error ? reject(error) : resolve())),
    );
  }
  close() {
    for (const socket of this.sockets.values()) socket.destroy();
    this.sockets.clear();
    this.dispatchEvent(new Event("close"));
  }
}

async function link(t, a, b) {
  const server = tcpServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const accepted = once(server, "connection");
  const outbound = connect(server.address().port, "127.0.0.1");
  await once(outbound, "connect");
  const [inbound] = await accepted;
  a.attach(b.key, outbound);
  b.attach(a.key, inbound);
  t.after(() => {
    outbound.destroy();
    inbound.destroy();
    server.close();
  });
}

function event(target, type, predicate = () => true) {
  return new Promise((resolve) => {
    function listener({ detail }) {
      if (!predicate(detail)) return;
      target.removeEventListener(type, listener);
      resolve(detail);
    }
    target.addEventListener(type, listener);
  });
}

// Independent implementation of the Rust DealFrame producer's hashing layout.
function hostDigest(deal) {
  const hash = createHash("sha256").update("PAXAI/inference-deal/v1\0");
  function number(value, bytes) {
    let n = BigInt(value);
    const out = Buffer.alloc(bytes);
    for (let i = bytes - 1; i >= 0; i--) {
      out[i] = Number(n & 255n);
      n >>= 8n;
    }
    hash.update(out);
  }
  for (const name of [
    "chain",
    "program",
    "market",
    "room_id",
    "model_digest",
    "runtime_digest",
    "deal_id",
    "ask_id",
    "roster_digest",
  ])
    hash.update(Buffer.from(deal[name]));
  for (const name of ["epoch", "config", "round"]) number(deal[name], 8);
  number(deal.price, 16);
  hash.update(Buffer.from(deal.plan.embedding_host));
  hash.update(Buffer.from(deal.plan.head_host));
  number(deal.plan.layer_count, 2);
  number(deal.plan.layers.length, 2);
  for (const split of deal.plan.layers) {
    for (const name of ["peer_key", "owner", "bid_id", "manifest_digest"])
      hash.update(Buffer.from(split[name]));
    number(split.first_layer, 2);
    number(split.end_layer, 2);
  }
  return hash.digest();
}

async function setup(
  t,
  { corruptShard, corruptManifest, validate = true } = {},
) {
  const directory = await mkdtemp(join(tmpdir(), "inference-deal-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const identities = await Promise.all(
    ["host", "a", "b"].map((name) =>
      PeerIdentity.open({ path: join(directory, name) }),
    ),
  );
  const [hostIdentity, ...peers] = identities;
  const source = await scriptedModelSource();
  if (corruptShard !== undefined) {
    const bytes = source.resources.get(`/layer-${corruptShard}.json`).slice();
    bytes[bytes.length - 2] ^= 1;
    source.resources.set(`/layer-${corruptShard}.json`, bytes);
  }
  if (corruptManifest) source.resources.get("/manifest.json")[0] ^= 1;
  const downloads = [];
  const server = httpServer((request, response) => {
    downloads.push(request.url);
    const bytes = source.resources.get(request.url);
    response.writeHead(bytes ? 200 : 404, {
      "content-type": "application/octet-stream",
    });
    response.end(bytes);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(
    () =>
      new Promise((resolve) => {
        server.closeAllConnections();
        server.close(resolve);
      }),
  );
  const offer = {
    model_digest: Array.from(source.digest),
    runtime_digest: fixed(6),
    room_id: fixed(4),
    layer_plan: peers.map((identity, i) => ({
      peer_key: Array.from(Buffer.from(identity.peerKey, "hex")),
      first_layer: i * 2,
      end_layer: i * 2 + 2,
    })),
    price: "100000000000000000000",
  };
  const deal = {
    chain: fixed(1),
    program: fixed(2),
    market: fixed(3),
    room_id: fixed(4),
    model_digest: Array.from(source.digest),
    runtime_digest: fixed(6),
    deal_id: fixed(7),
    ask_id: fixed(8),
    roster_digest: fixed(9),
    epoch: 1,
    config: 2,
    round: 3,
    price: "100000000000000000000",
    plan: {
      embedding_host: fixed(10),
      head_host: fixed(10),
      layer_count: 4,
      layers: peers.map((identity, i) => ({
        peer_key: Array.from(Buffer.from(identity.peerKey, "hex")),
        owner: fixed(11 + i),
        bid_id: fixed(13 + i),
        manifest_digest: fixed(15 + i),
        first_layer: i * 2,
        end_layer: i * 2 + 2,
      })),
    },
  };
  const modelSource = {
    manifest_url: `http://127.0.0.1:${server.address().port}/manifest.json`,
  };
  const signalHost = new RoomHost(),
    signalPeers = new Map();
  const signalURL = await signalHost.listen(t, (connection, frame) => {
    if (frame.type === "hello") {
      signalPeers.set(toHex(frame.peer_key), connection);
      connection.send({ type: "offer", offer });
    } else if (frame.type === "deal_ack") {
      assert.deepEqual(frame.deal_id, deal.deal_id);
      assert.deepEqual(Buffer.from(frame.digest), hostDigest(deal));
      connection.send({ type: "deal_live", deal_id: deal.deal_id });
    }
  });
  const signals = peers.map(
    (identity) => new SignalClient(signalURL, { roomId, identity, manifest }),
  );
  for (const signal of signals) t.after(() => signal.close());
  await Promise.all(signals.map((signal) => signal.connect()));
  const [host, a, b] = identities.map(
    (identity) => new SocketEndpoint(identity.peerKey),
  );
  for (const endpoint of [host, a, b]) t.after(() => endpoint.close());
  await link(t, host, a);
  await link(t, host, b);
  await link(t, a, b);
  const runtimes = peers.map(() => new ScriptedRuntime());
  const clients = peers.map(
    (identity, i) =>
      new DealClient({
        roomId,
        identity,
        offer,
        modelSource,
        runtime: runtimes[i],
        mesh: [a, b][i],
        hostPeerKey: hostIdentity.peerKey,
        validateDeal: async (received) =>
          validate && JSON.stringify(received) === JSON.stringify(deal),
        acknowledge: (ack) => signals[i].acknowledgeDeal(ack),
      }),
  );
  for (const [i, client] of clients.entries()) {
    t.after(() => client.close());
    signals[i].addEventListener("frame", ({ detail }) => {
      if (["deal", "deal_live"].includes(detail.type))
        client.handleFrame(detail).catch((error) => {
          client.dispatchEvent(new CustomEvent("error", { detail: error }));
        });
    });
  }
  return {
    source,
    signalHost,
    signalPeers,
    modelSource,
    downloads,
    offer,
    deal,
    host,
    a,
    b,
    peers,
    runtimes,
    clients,
  };
}

async function admit(context) {
  const { signalHost, signalPeers, clients, peers, deal } = context;
  const acknowledgements = peers.map(
    (peer) =>
      new Promise((resolve) => {
        signalHost.on("frame", (connection, frame) => {
          if (
            toHex(connection.frames[0].peer_key) === peer.peerKey &&
            frame.type === "deal_ack"
          )
            resolve({ frame });
        });
      }),
  );
  const ready = clients.map((client) => event(client, "ready"));
  for (const peer of peers)
    signalPeers.get(peer.peerKey).send({ type: "deal", deal });
  await Promise.all(ready);
  for (const [i, incoming] of (await Promise.all(acknowledgements)).entries()) {
    const ack = incoming.frame;
    assert.equal(ack.type, "deal_ack");
    assert.deepEqual(ack.deal_id, deal.deal_id);
    assert.deepEqual(Buffer.from(ack.digest), hostDigest(deal));
    const key = await crypto.subtle.importKey(
      "raw",
      Buffer.from(peers[i].peerKey, "hex"),
      "Ed25519",
      false,
      ["verify"],
    );
    const signingBytes = Buffer.concat([
      Buffer.from("PAXAI/inference-deal-ack/v1\0"),
      hostDigest(deal),
    ]);
    assert.equal(
      await crypto.subtle.verify(
        "Ed25519",
        key,
        Uint8Array.from(ack.signature),
        signingBytes,
      ),
      true,
    );
    const changed = signingBytes.slice();
    changed[changed.length - 1] ^= 1;
    assert.equal(
      await crypto.subtle.verify(
        "Ed25519",
        key,
        Uint8Array.from(ack.signature),
        changed,
      ),
      false,
    );
  }
}

async function twoPeerSplitReproducesSingleProcessTokenForToken(t) {
  const context = await setup(t);
  await admit(context);
  const { source, clients, host, a, downloads, runtimes } = context;
  assert.deepEqual(
    await dealDigest(context.deal),
    new Uint8Array(hostDigest(context.deal)),
  );
  assert.deepEqual(
    downloads.filter((path) => path !== "/manifest.json").sort(),
    ["/layer-0.json", "/layer-1.json", "/layer-2.json", "/layer-3.json"],
  );
  assert.deepEqual(
    runtimes.map((runtime) =>
      runtime.models[0].layers.map((layer) => layer.index),
    ),
    [
      [0, 1],
      [2, 3],
    ],
  );
  const cache = source.model.createCache("single");
  let token = 3;
  const tokens = [];
  for (let position = 0; position < 16; position++) {
    const input = scriptedEmbedding(token);
    const expected = await source.model.runActivation(input, position, cache);
    const received = event(
      host,
      "frame",
      ({ frame }) => frame.kind === "activation",
    );
    await host.send(
      a.key,
      "activation",
      encodeActivation({
        requestId: "generation",
        digest: clients[0].digest,
        position,
        layer: 0,
        values: input,
      }),
    );
    const output = decodeActivation((await received).frame.payload);
    assert.equal(output.requestId, "generation");
    assert.equal(output.position, position);
    assert.equal(output.layer, 4);
    assert.deepEqual(output.values, expected);
    token = scriptedToken(output.values);
    assert.equal(token, scriptedToken(expected));
    tokens.push(token);
  }
  assert.equal(tokens.length, 16);
  assert.ok(
    new Set(tokens).size > 1,
    "the model produces a changing token stream",
  );
  assert.deepEqual(
    clients.map((client) => client.cacheCount),
    [1, 1],
  );
}

async function wrongShardDigestRefusesDealWithoutAcknowledgement(t) {
  const context = await setup(t, { corruptShard: 1 });
  let acknowledgements = 0;
  context.signalHost.on("frame", (connection, frame) => {
    if (frame.type === "deal_ack") acknowledgements++;
  });
  const refused = event(context.clients[0], "error");
  context.signalPeers
    .get(context.a.key)
    .send({ type: "deal", deal: context.deal });
  assert.match((await refused).message, /Shard digest mismatch: layer-1.json/);
  assert.equal(acknowledgements, 0);
  assert.equal(context.clients[0].state, "closed");
  assert.equal(context.clients[0].cacheCount, 0);
  assert.equal(context.runtimes[0].models.length, 0);
  assert.deepEqual(context.downloads, [
    "/manifest.json",
    "/layer-0.json",
    "/layer-1.json",
  ]);
}

async function manifestDigestAndDealValidationRefuseBeforeWeights(t) {
  for (const options of [{ corruptManifest: true }, { validate: false }]) {
    await t.test(
      options.corruptManifest ? "manifest" : "deal authority",
      async (t) => {
        const context = await setup(t, options);
        const refused = event(context.clients[0], "error");
        context.signalPeers
          .get(context.a.key)
          .send({ type: "deal", deal: context.deal });
        assert.match(
          (await refused).message,
          options.corruptManifest
            ? /manifest digest mismatch/
            : /Deal validation refused/,
        );
        assert.equal(context.runtimes[0].models.length, 0);
        assert.deepEqual(
          context.downloads,
          options.corruptManifest ? ["/manifest.json"] : [],
        );
      },
    );
  }
}

async function byeDropsOnlyItsRequestCacheAndRoomByeClearsAll(t) {
  const context = await setup(t);
  await admit(context);
  const { host, a, clients, runtimes } = context;
  async function send(id, position = 0) {
    const received = event(
      host,
      "frame",
      ({ frame }) => frame.kind === "activation",
    );
    await host.send(
      a.key,
      "activation",
      encodeActivation({
        requestId: id,
        digest: clients[0].digest,
        position,
        layer: 0,
        values: scriptedEmbedding(5),
      }),
    );
    return decodeActivation((await received).frame.payload);
  }
  const first = await send("first");
  await send("second");
  const caches = runtimes.map((runtime) => [...runtime.models[0].liveCaches]);
  assert.deepEqual(
    clients.map((client) => client.cacheCount),
    [2, 2],
  );
  const ended = clients.map((client) => event(client, "bye"));
  await Promise.all(
    [a.key, context.b.key].map((key) =>
      host.send(key, "bye", { requestId: "first" }),
    ),
  );
  await Promise.all(ended);
  assert.deepEqual(
    clients.map((client) => client.cacheCount),
    [1, 1],
  );
  for (let i = 0; i < 2; i++) {
    assert.equal(caches[i][0].layers.size, 0);
    assert.equal(runtimes[i].models[0].liveCaches.has(caches[i][0]), false);
    assert.equal(caches[i][1].position, 1);
  }
  const fresh = await send("fresh");
  assert.deepEqual(fresh.values, first.values);
  await send("second", 1);
  const closed = clients.map((client) => event(client, "bye"));
  await Promise.all(
    [a.key, context.b.key].map((key) => host.send(key, "bye", {})),
  );
  await Promise.all(closed);
  assert.deepEqual(
    clients.map((client) => client.cacheCount),
    [0, 0],
  );
  assert.deepEqual(
    clients.map((client) => client.state),
    ["closed", "closed"],
  );
  for (const runtime of runtimes) {
    assert.equal(runtime.models[0].liveCaches.size, 0);
    assert.ok(
      runtime.models[0].droppedCaches.every((cache) => cache.layers.size === 0),
    );
  }
}

async function unorderedActivationsDrainInTokenOrderAndRequestsStayIsolated(t) {
  const context = await setup(t);
  await admit(context);
  const { host, a, clients, source } = context;
  const outputs = [];
  const complete = new Promise((resolve) =>
    host.addEventListener("frame", ({ detail: { frame } }) => {
      if (frame.kind === "activation") {
        outputs.push(decodeActivation(frame.payload));
        if (outputs.length === 3) resolve();
      }
    }),
  );
  for (const [requestId, position, token] of [
    ["one", 1, 7],
    ["two", 0, 3],
    ["one", 0, 5],
  ])
    await host.send(
      a.key,
      "activation",
      encodeActivation({
        requestId,
        position,
        digest: clients[0].digest,
        layer: 0,
        values: scriptedEmbedding(token),
      }),
    );
  await complete;
  assert.deepEqual(
    outputs.map(({ requestId, position }) => [requestId, position]),
    [
      ["two", 0],
      ["one", 0],
      ["one", 1],
    ],
  );
  const one = source.model.createCache("one"),
    two = source.model.createCache("two");
  for (const [i, token, position, cache] of [
    [0, 3, 0, two],
    [1, 5, 0, one],
    [2, 7, 1, one],
  ])
    assert.deepEqual(
      outputs[i].values,
      await source.model.runActivation(
        scriptedEmbedding(token),
        position,
        cache,
      ),
    );
}

async function invalidRoutesReplaysAndEndedRequestsRefuse(t) {
  const context = await setup(t);
  await admit(context);
  const { host, a, clients } = context;
  const base = {
    requestId: "request",
    position: 0,
    digest: clients[0].digest,
    layer: 0,
    values: scriptedEmbedding(1),
  };
  const frame = (payload) => ({ roomId, kind: "activation", payload });
  await assert.rejects(
    clients[0].handleFrame(frame(encodeActivation(base)), context.b.key),
    /assigned deal route/,
  );
  await assert.rejects(
    clients[0].handleFrame(
      frame(encodeActivation({ ...base, digest: fixed(99) })),
    ),
    /assigned deal route/,
  );
  const received = event(
    host,
    "frame",
    ({ frame }) => frame.kind === "activation",
  );
  await clients[0].handleFrame(frame(encodeActivation(base)));
  await received;
  await assert.rejects(
    clients[0].handleFrame(frame(encodeActivation(base))),
    /Replayed activation/,
  );
  await assert.rejects(
    clients[0].handleFrame(frame(encodeActivation({ ...base, position: 34 }))),
    /reorder limit/,
  );
  await clients[0].handleFrame({
    roomId,
    kind: "bye",
    payload: { requestId: "request" },
  });
  await assert.rejects(
    clients[0].handleFrame(frame(encodeActivation({ ...base, position: 1 }))),
    /already ended/,
  );
  assert.equal(clients[0].cacheCount, 0);
  assert.throws(
    () => encodeActivation({ ...base, values: new Float32Array([NaN]) }),
    /finite/,
  );
  assert.throws(
    () => decodeActivation(encodeActivation(base).subarray(1)),
    /activation/,
  );
  assert.equal(a.sockets.size, 2);
}

async function shardBoundariesCannotDownloadUnassignedWeights(t) {
  const context = await setup(t);
  await assert.rejects(
    loadLayerRange({
      offer: context.offer,
      modelSource: context.modelSource,
      firstLayer: 1,
      endLayer: 5,
      runtime: context.runtimes[0],
    }),
    /end layer/,
  );
  assert.deepEqual(context.downloads, ["/manifest.json"]);
}

test(
  twoPeerSplitReproducesSingleProcessTokenForToken.name,
  { timeout: 15000 },
  twoPeerSplitReproducesSingleProcessTokenForToken,
);
test(
  wrongShardDigestRefusesDealWithoutAcknowledgement.name,
  { timeout: 15000 },
  wrongShardDigestRefusesDealWithoutAcknowledgement,
);
test(
  manifestDigestAndDealValidationRefuseBeforeWeights.name,
  { timeout: 15000 },
  manifestDigestAndDealValidationRefuseBeforeWeights,
);
test(
  byeDropsOnlyItsRequestCacheAndRoomByeClearsAll.name,
  { timeout: 15000 },
  byeDropsOnlyItsRequestCacheAndRoomByeClearsAll,
);
test(
  unorderedActivationsDrainInTokenOrderAndRequestsStayIsolated.name,
  { timeout: 15000 },
  unorderedActivationsDrainInTokenOrderAndRequestsStayIsolated,
);
test(
  invalidRoutesReplaysAndEndedRequestsRefuse.name,
  { timeout: 15000 },
  invalidRoutesReplaysAndEndedRequestsRefuse,
);
test(
  shardBoundariesCannotDownloadUnassignedWeights.name,
  { timeout: 15000 },
  shardBoundariesCannotDownloadUnassignedWeights,
);

import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import type { Duplex } from "node:stream";
import { after, before, test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import transcript from "../src/hooks/perps/repair-22-31-20-fixtures.json";
import type {
  IndexerOrder,
  IndexerTrade,
  IndexerLiquidation,
} from "../src/hooks/perps/useIndexerSubscriptions";

const user = "0x00000000000000000000000000000000000000ab";
const stamp = "2026-01-01T00:00:00.123456Z";
const normalizeQuery = (query: string) => query.replace(/\s+/g, " ").trim();
const globals = new Map<string, PropertyDescriptor | undefined>();
const previousGateway = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
let origin: string;
let ranked = false;

interface GraphQLRequest {
  query: string;
  variables?: Record<string, unknown>;
}

interface GraphQLFrame {
  type: string;
  id?: string;
  payload?: GraphQLRequest;
}

// This peer replays captured producer bytes over an actual RFC6455 socket.
// The production subscribeStream client and hook adapters consume every frame.
class TranscriptPeer {
  private buffer = Buffer.alloc(0);
  private subscriptions = new Map<
    string,
    (typeof transcript.subscriptions)[number]
  >();

  constructor(private readonly socket: Duplex) {
    socket.on("data", (chunk: Buffer) => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      this.drain();
    });
  }

  private writeFrame(opcode: number, payload: Buffer) {
    assert.ok(payload.length <= 65535);
    const header =
      payload.length < 126
        ? Buffer.from([opcode, payload.length])
        : Buffer.from([
            opcode,
            126,
            payload.length >> 8,
            payload.length & 0xff,
          ]);
    this.socket.write(Buffer.concat([header, payload]));
  }

  private send(text: string) {
    this.writeFrame(0x81, Buffer.from(text));
  }

  private drain() {
    for (;;) {
      if (this.buffer.length < 2) return;
      const opcode = this.buffer[0]! & 0x0f;
      assert.ok(this.buffer[0]! & 0x80, "client frames must not be fragmented");
      assert.ok(this.buffer[1]! & 0x80, "client frames must be masked");
      let length = this.buffer[1]! & 0x7f;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      } else if (length === 127) {
        if (this.buffer.length < 10) return;
        const wideLength = this.buffer.readBigUInt64BE(2);
        assert.ok(wideLength <= 65535n);
        length = Number(wideLength);
        offset = 10;
      }
      if (this.buffer.length < offset + 4 + length) return;
      const mask = this.buffer.subarray(offset, offset + 4);
      const payload = Buffer.from(
        this.buffer.subarray(offset + 4, offset + 4 + length),
      );
      for (let i = 0; i < payload.length; i++)
        payload[i] = payload[i]! ^ mask[i % 4]!;
      this.buffer = this.buffer.subarray(offset + 4 + length);
      if (opcode === 8) {
        this.writeFrame(0x88, payload);
        this.socket.end();
        return;
      }
      assert.equal(opcode, 1);
      const frame: GraphQLFrame = JSON.parse(payload.toString("utf8"));
      switch (frame.type) {
        case "connection_init":
          this.send(transcript.connectionAck);
          break;
        case "subscribe": {
          assert.ok(frame.id);
          assert.ok(frame.payload);
          const request = frame.payload;
          const fixture = transcript.subscriptions.find(
            (candidate) =>
              normalizeQuery(candidate.request.query) ===
                normalizeQuery(request.query) &&
              JSON.stringify(candidate.request.variables) ===
                JSON.stringify(request.variables),
          );
          assert.ok(
            fixture,
            "subscription query and variables must match the producer capture",
          );
          this.subscriptions.set(frame.id, fixture);
          this.replay();
          break;
        }
        case "complete":
          assert.ok(frame.id);
          this.subscriptions.delete(frame.id);
          break;
        default:
          assert.fail(`Unexpected GraphQL frame: ${frame.type}`);
      }
    }
  }

  replay() {
    for (const [id, fixture] of this.subscriptions) {
      for (const message of fixture.messages) {
        const recorded: { id: string; type: string; payload: unknown } =
          JSON.parse(message);
        assert.equal(recorded.type, "next");
        // Only the session-local subscription id changes during replay.
        this.send(JSON.stringify({ ...recorded, id }));
      }
    }
  }

  async close() {
    if (this.socket.destroyed) return;
    const closed = once(this.socket, "close");
    this.socket.destroy();
    await closed;
  }
}

const peers: TranscriptPeer[] = [];
const server = createServer(async (request, response) => {
  assert.equal(request.method, "POST");
  if (request.url === "/rewind") {
    for (const peer of peers) peer.replay();
    response.writeHead(204).end();
    return;
  }
  if (request.url === "/rank") {
    ranked = true;
    response.writeHead(204).end();
    return;
  }
  assert.equal(request.url, "/graphql");
  assert.equal(request.headers["content-type"], "application/json");
  let body = "";
  for await (const chunk of request) body += chunk;
  const operation: GraphQLRequest = JSON.parse(body);
  const fixtures = transcript.referrals;
  const fixture =
    operation.variables?.address === user
      ? ranked
        ? fixtures.ranked
        : fixtures.unranked
      : operation.variables?.address ===
          fixtures.missing.request.variables.address
        ? fixtures.missing
        : fixtures.tiersOnly;
  assert.equal(
    normalizeQuery(operation.query),
    normalizeQuery(fixture.request.query),
  );
  assert.deepEqual(
    operation.variables,
    "variables" in fixture.request ? fixture.request.variables : undefined,
  );
  response
    .writeHead(200, { "Content-Type": "application/json" })
    .end(fixture.response);
});

before(async () => {
  server.on("upgrade", (request, socket, head) => {
    assert.equal(request.url, "/graphql");
    assert.equal(
      request.headers["sec-websocket-protocol"],
      "graphql-transport-ws",
    );
    assert.equal(head.length, 0);
    const accept = createHash("sha1")
      .update(
        `${request.headers["sec-websocket-key"]}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`,
      )
      .digest("base64");
    socket.write(
      `HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\nSec-WebSocket-Protocol: graphql-transport-ws\r\n\r\n`,
    );
    peers.push(new TranscriptPeer(socket));
  });
  await new Promise<void>((resolveListen) =>
    server.listen(0, "127.0.0.1", resolveListen),
  );
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  // The test owns this loopback endpoint; no caller-supplied environment is required.
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  for (const [key, value] of [
    ["window", new EventTarget()],
    ["document", new EventTarget()],
  ] as const) {
    globals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, value });
  }
});

after(async () => {
  await Promise.all(peers.map((peer) => peer.close()));
  await new Promise<void>((resolveClose) => server.close(() => resolveClose()));
  for (const [key, descriptor] of globals) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor);
    else Reflect.deleteProperty(globalThis, key);
  }
  if (previousGateway === undefined)
    delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  else process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = previousGateway;
});

async function indexerSubscriptionWireTypes() {
  const adapter = await import("../src/hooks/perps/useIndexerSubscriptions");
  const orders = new Map<string, IndexerOrder>();
  const liquidations = new Map<string, IndexerLiquidation>();
  let trade: IndexerTrade | undefined;
  const cleanup = [
    adapter.subscribeIndexerOrder(user, (row) => orders.set(row.orderId, row)),
    adapter.subscribeIndexerTrade(2, (row) => {
      trade = row;
    }),
    adapter.subscribeIndexerLiquidation(2, (row) =>
      liquidations.set(row.positionId, row),
    ),
    adapter.subscribeIndexerLiquidation(3, (row) =>
      liquidations.set(row.positionId, row),
    ),
  ];
  try {
    // Rewind replays the producer capture until all real subscriptions register.
    const deadline = Date.now() + 10_000;
    while (orders.size !== 2 || liquidations.size !== 2 || !trade) {
      assert.ok(
        Date.now() < deadline,
        "real producer must deliver all four subscriptions",
      );
      const response = await fetch(`${origin}/rewind`, { method: "POST" });
      assert.equal(response.status, 204);
      await new Promise((resolveWait) => setTimeout(resolveWait, 25));
    }
    assert.deepEqual(orders.get("7"), {
      orderId: "7",
      status: "active",
      marketId: 2,
      orderType: 4,
      remainingSize: "600",
      blockTimestamp: null,
    });
    assert.deepEqual(orders.get("8"), {
      orderId: "8",
      status: "active",
      marketId: 2,
      orderType: 4,
      remainingSize: "0",
      blockTimestamp: stamp,
    });
    assert.deepEqual(trade, {
      positionId: "9",
      userAddress: user,
      sizeUsd: "500",
      executionPrice: "1800",
      tradeType: "liquidation",
      blockTimestamp: stamp,
    });
    assert.deepEqual(liquidations.get("9"), {
      positionId: "9",
      userAddress: user,
      sizeUsd: "500",
      executionPrice: "1800",
      blockTimestamp: stamp,
    });
    assert.deepEqual(liquidations.get("10"), {
      positionId: "10",
      userAddress: user,
      sizeUsd: null,
      executionPrice: "1900",
      blockTimestamp: stamp,
    });
    assert.deepEqual(
      adapter.decodeIndexerOrder({ ...orders.get("7"), remainingSize: null }),
      {
        ...orders.get("7"),
        remainingSize: null,
      },
    );
    assert.deepEqual(
      adapter.decodeIndexerTrade({
        ...trade,
        userAddress: null,
        executionPrice: null,
      }),
      {
        ...trade,
        userAddress: null,
        executionPrice: null,
      },
    );
    assert.deepEqual(
      adapter.decodeIndexerLiquidation({
        ...liquidations.get("10"),
        executionPrice: null,
      }),
      {
        ...liquidations.get("10"),
        executionPrice: null,
      },
    );
    const nanos = "2026-01-01T00:00:00.123456789Z";
    assert.equal(
      adapter.decodeIndexerTrade({ ...trade, blockTimestamp: nanos })
        .blockTimestamp,
      nanos,
    );
    const large =
      "115792089237316195423570985008687907853269984665640564039457584007913129639935";
    assert.equal(
      adapter.decodeIndexerOrder({ ...orders.get("7"), orderId: large })
        .orderId,
      large,
    );
    // Malformed variants of a real producer row must never become fabricated
    // dates or zero amounts at the adapter boundary.
    assert.throws(
      () =>
        adapter.decodeIndexerTrade({ ...trade, blockTimestamp: 1_767_225_600 }),
      /RFC3339|string/,
    );
    assert.throws(
      () =>
        adapter.decodeIndexerTrade({ ...trade, blockTimestamp: "not-a-date" }),
      /RFC3339/,
    );
    assert.throws(
      () =>
        adapter.decodeIndexerLiquidation({
          ...liquidations.get("10"),
          sizeUsd: undefined,
        }),
      /string/,
    );
  } finally {
    for (const unsubscribe of cleanup) unsubscribe();
  }
}

test(indexerSubscriptionWireTypes);

async function referralNullTier() {
  const { fetchPerfdbReferrals, referralTierView } = await import(
    "../src/hooks/perps/usePerfdbReferrals"
  );
  const { ReferralTierStatus } = await import(
    "../src/page-modules/referrals/tier-status"
  );
  const unranked = await fetchPerfdbReferrals(user);
  assert.deepEqual(unranked.data, {
    referralCode: "PERSISTED",
    tier: null,
    referralCount: 0,
    totalEarned: null,
  });
  const before = JSON.stringify(unranked);
  assert.deepEqual(referralTierView(unranked.data, unranked.tiers), {
    status: "unranked",
    label: "Unranked",
    currentTierIndex: -1,
    nextTier: unranked.tiers[0],
    progressPct: 0,
    isMaxTier: false,
  });
  assert.equal(
    JSON.stringify(unranked),
    before,
    "view preserves the nullable resolver result",
  );
  assert.ok(unranked.data);
  const unrankedHtml = renderToStaticMarkup(
    createElement(ReferralTierStatus, {
      data: unranked.data,
      view: referralTierView(unranked.data, unranked.tiers),
    }),
  );
  assert.match(unrankedHtml, />Unranked<\/span>/);
  assert.match(unrankedHtml, /0 \/ 1 referrals to/);
  assert.match(unrankedHtml, /width:0%/);
  assert.doesNotMatch(unrankedHtml, /Max tier unlocked/);
  const response = await fetch(`${origin}/rank`, { method: "POST" });
  assert.equal(response.status, 204);
  const ranked = await fetchPerfdbReferrals(user);
  assert.equal(ranked.data?.tier, "bronze");
  assert.equal(ranked.data?.referralCount, 1);
  assert.deepEqual(referralTierView(ranked.data, ranked.tiers), {
    status: "ranked",
    label: "Bronze",
    currentTierIndex: 0,
    nextTier: ranked.tiers[1],
    progressPct: 50,
    isMaxTier: false,
  });
  assert.ok(ranked.data);
  const rankedHtml = renderToStaticMarkup(
    createElement(ReferralTierStatus, {
      data: ranked.data,
      view: referralTierView(ranked.data, ranked.tiers),
    }),
  );
  assert.match(rankedHtml, />Bronze<\/span>/);
  assert.match(rankedHtml, /width:50%/);
  assert.equal(
    referralTierView(ranked.data, ranked.tiers.slice(0, 1)).isMaxTier,
    true,
  );
  const missing = await fetchPerfdbReferrals(
    "0x00000000000000000000000000000000000000ad",
  );
  assert.equal(missing.data, null);
  assert.equal(
    referralTierView(missing.data, missing.tiers).status,
    "unavailable",
  );
  assert.equal(referralTierView(missing.data, missing.tiers).isMaxTier, false);
  assert.equal(referralTierView(unranked.data, []).isMaxTier, false);
  const tiersOnly = await fetchPerfdbReferrals(undefined);
  assert.equal(tiersOnly.data, null);
  assert.deepEqual(tiersOnly.tiers, unranked.tiers);
  const unavailable = referralTierView(
    { ...ranked.data, tier: "unrecognized" },
    ranked.tiers,
  );
  assert.equal(unavailable.status, "unavailable");
  assert.equal(unavailable.currentTierIndex, -1);
  assert.equal(unavailable.nextTier, null);
  assert.equal(unavailable.progressPct, null);
  assert.equal(unavailable.isMaxTier, false);
  const maxTierHtml = renderToStaticMarkup(
    createElement(ReferralTierStatus, {
      data: ranked.data,
      view: referralTierView(ranked.data, ranked.tiers.slice(0, 1)),
    }),
  );
  assert.match(maxTierHtml, /Max tier unlocked/);
}

test(referralNullTier);

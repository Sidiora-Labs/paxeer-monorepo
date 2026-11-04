import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer, request, type IncomingHttpHeaders, type OutgoingHttpHeaders, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { test, type TestContext } from "node:test";
import { apiResolver } from "next/dist/server/api-utils/node/api-resolver";

interface Snapshot {
  total: number;
  byMarket: Record<string, number>;
  top: Array<{ market: string; count: number; delta5m: number }>;
  rising: Array<{ market: string; count: number; deltaPct: number }>;
  history: Array<{ at: number; total: number }>;
}

interface Reply {
  status: number;
  headers: IncomingHttpHeaders;
  body: unknown;
}

async function listen(server: Server): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

function send(origin: string, client: string, body?: unknown, headers: OutgoingHttpHeaders = {}): Promise<Reply> {
  return new Promise((resolve, reject) => {
    const req = request(`${origin}/api/presence`, {
      method: body === undefined ? "GET" : "POST",
      localAddress: client,
      agent: false,
      headers: { "Content-Type": "application/json", ...headers },
    }, (res) => {
      const chunks: Buffer[] = [];
      res.on("data", (chunk: Buffer) => chunks.push(chunk));
      res.on("error", reject);
      res.on("end", () => {
        try {
          resolve({ status: res.statusCode!, headers: res.headers, body: JSON.parse(Buffer.concat(chunks).toString()) });
        } catch (error) { reject(error); }
      });
    });
    req.on("error", reject);
    req.setTimeout(10_000, () => req.destroy(new Error("Presence fixture timeout")));
    req.end(body === undefined ? undefined : JSON.stringify(body));
  });
}

function issuedCookie(reply: Reply): string {
  assert.equal(reply.status, 200);
  assert.deepEqual(reply.body, { ok: true });
  const cookies = reply.headers["set-cookie"];
  assert.ok(cookies && cookies.length === 1, "registration must issue exactly one server identity");
  assert.match(cookies[0], /^pax-presence-id=[A-Za-z0-9_-]{43};/);
  assert.match(cookies[0], /; Path=\/api\/presence; Max-Age=300; HttpOnly; SameSite=Strict/);
  return cookies[0].split(";")[0];
}

function limited(reply: Reply): void {
  assert.equal(reply.status, 429);
  assert.equal(reply.headers["retry-after"], "300");
  assert.equal(reply.headers["set-cookie"], undefined);
  assert.deepEqual(reply.body, { error: "Presence registration limit reached" });
}

async function presenceRepairsUseRealHttp(t: TestContext): Promise<void> {
  const route = createServer();
  const servers = [route];
  const savedEnv = new Map<string, string | undefined>();
  t.after(async () => {
    for (const server of servers.reverse()) {
      server.closeAllConnections();
      if (server.listening) await new Promise<void>((resolve, reject) => server.close((err) => err ? reject(err) : resolve()));
    }
    for (const [key, value] of savedEnv) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
  const routeOrigin = await listen(route);
  for (const [key, value] of Object.entries({
    NEXT_PUBLIC_PERPS_GATEWAY_URL: routeOrigin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: routeOrigin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${routeOrigin}/api/v2`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: routeOrigin.replace("http:", "ws:"),
    PRESENCE_TRUSTED_PROXY_IPS: "::ffff:127.0.0.2",
  })) {
    savedEnv.set(key, process.env[key]);
    process.env[key] = value;
  }
  const { default: handler } = await import("../src/pages/api/presence");
  route.on("request", (req, res) => {
    void apiResolver(req, res, {}, { default: handler }, {
      dev: false, previewModeId: "", previewModeEncryptionKey: "", previewModeSigningKey: "",
    }, false);
  });
  // The ingress fixture derives X-Real-IP from its socket, discarding caller input.
  const proxy = createServer((req, res) => {
    const forwarded = request(new URL(req.url!, routeOrigin), {
      method: req.method,
      localAddress: "127.0.0.2",
      agent: false,
      headers: { ...req.headers, "x-real-ip": req.socket.remoteAddress },
    }, (upstream) => {
      res.writeHead(upstream.statusCode!, upstream.headers);
      upstream.pipe(res);
    });
    forwarded.on("error", (error) => res.destroy(error));
    req.pipe(forwarded);
  });
  servers.push(proxy);
  const proxyOrigin = await listen(proxy);
  // Only Date advances: the sockets, Next request parsing, cookies and stores are real.
  t.mock.timers.enable({ apis: ["Date"], now: Date.UTC(2026, 0, 1) });
  const advance = (milliseconds: number) => t.mock.timers.tick(milliseconds);
  async function snapshot(): Promise<Snapshot> {
    const reply = await send(routeOrigin, "127.0.0.1");
    assert.equal(reply.status, 200);
    assert.equal(reply.headers["cache-control"], "no-store, max-age=0");
    return reply.body as Snapshot;
  }
  async function clearExpired(): Promise<void> {
    advance(31 * 60_000);
    assert.equal((await snapshot()).total, 0);
  }

  async function presenceNewestFiveMinuteSampleAcrossFullHistory(): Promise<void> {
    const start = Date.now();
    assert.equal((await snapshot()).total, 0);
    const cookies = new Map<string, string>();
    async function populate(minute: number): Promise<void> {
      for (const [market, count, subnet] of [["BTC_USD", minute + 2, 10], ["ETH_USD", 20 - Math.floor(minute / 2), 11]] as const) {
        for (let i = 1; i <= count; i++) {
          const client = `127.${subnet}.0.${i}`;
          const reply = await send(proxyOrigin, client, { market, sessionId: `legacy_${client}` }, { cookie: cookies.get(client) ?? "" });
          assert.equal(reply.status, 200);
          if (reply.headers["set-cookie"]) cookies.set(client, issuedCookie(reply));
        }
      }
    }
    for (let minute = 1; minute <= 16; minute++) {
      advance(50_000);
      await populate(minute);
      advance(10_000);
      const current = await snapshot();
      assert.equal(current.byMarket.BTC_USD, minute + 2);
      assert.equal(current.byMarket.ETH_USD, 20 - Math.floor(minute / 2));
    }
    const current = await snapshot();
    assert.equal(current.history.length, 12);
    assert.equal(current.history[0].at, start + 5 * 60_000);
    assert.equal(current.history[11].at, start + 16 * 60_000);
    assert.deepEqual(current.history, Array.from({ length: 12 }, (_, index) => {
      const minute = index + 5;
      return { at: start + minute * 60_000, total: 22 + minute - Math.floor(minute / 2) };
    }));
    assert.deepEqual(current.top, [
      { market: "BTC_USD", count: 18, delta5m: 5 },
      { market: "ETH_USD", count: 12, delta5m: -3 },
    ]);
    assert.deepEqual(current.rising, [{ market: "BTC_USD", count: 18, deltaPct: 5 / 13 * 100 }]);
    advance(30_000);
    await populate(16);
    const betweenSamples = await snapshot();
    assert.deepEqual(betweenSamples.top, current.top);
    assert.deepEqual(betweenSamples.rising, current.rising);
    assert.deepEqual(betweenSamples.history, current.history);
  }

  async function presenceServerIdentitiesBoundSpoofedRegistrations(): Promise<void> {
    await clearExpired();
    const victim = issuedCookie(await send(proxyOrigin, "127.0.0.4", { market: "ETH_USD", sessionId: "victim_123" }));
    const attackerCookies = new Set<string>();
    for (let i = 0; i < 5; i++) {
      attackerCookies.add(issuedCookie(await send(proxyOrigin, "127.0.0.3", { market: "BTC_USD", sessionId: "same_legacy_id" })));
    }
    assert.equal(attackerCookies.size, 5);
    for (let i = 0; i < 1_024; i++) {
      limited(await send(proxyOrigin, "127.0.0.3", { market: "BTC_USD", sessionId: `attacker_${i}` }, {
        cookie: `pax-presence-id=forged_${i}`,
        "x-real-ip": `198.51.${Math.floor(i / 256)}.${i % 256}`,
        "x-forwarded-for": `203.0.${Math.floor(i / 256)}.${i % 256}`,
      }));
    }
    const live = await snapshot();
    assert.equal(live.total, 6);
    assert.deepEqual(live.byMarket, { ETH_USD: 1, BTC_USD: 5 });
    for (let i = 0; i < 10; i++) {
      const heartbeat = await send(proxyOrigin, "127.0.0.4", { market: "ETH_USD", sessionId: `rotated_${i}` }, { cookie: victim });
      assert.equal(heartbeat.status, 200);
      assert.equal(heartbeat.headers["set-cookie"], undefined);
    }
    assert.equal((await snapshot()).total, 6);
    const stolen = await send(proxyOrigin, "127.0.0.5", { market: "BTC_USD" }, { cookie: victim });
    assert.equal(stolen.status, 403);
    assert.deepEqual(stolen.body, { error: "Session belongs to another client" });
    assert.deepEqual((await snapshot()).byMarket, live.byMarket);
    advance(15_000);
    assert.equal((await snapshot()).total, 0);
    const revived = await send(proxyOrigin, "127.0.0.4", { market: "ETH_USD" }, { cookie: victim });
    assert.equal(revived.status, 200);
    assert.equal(revived.headers["set-cookie"], undefined);
    assert.equal((await snapshot()).total, 1);
    advance(45_000);
    limited(await send(proxyOrigin, "127.0.0.3", { market: "BTC_USD" }));
    advance(240_000);
    assert.equal((await snapshot()).total, 0);
    const replacement = issuedCookie(await send(proxyOrigin, "127.0.0.4", { market: "ETH_USD" }, { cookie: victim }));
    assert.notEqual(replacement, victim);
    assert.equal((await snapshot()).total, 1);
  }

  async function presenceDirectSocketBudgetsIgnoreForwardedHeaders(): Promise<void> {
    await clearExpired();
    for (let i = 0; i < 5; i++) {
      issuedCookie(await send(routeOrigin, "127.0.0.6", { market: "BTC_USD", sessionId: `direct_${i}` }, { "x-real-ip": `192.0.2.${i + 1}` }));
    }
    for (const headers of [{}, { "x-real-ip": "192.0.2.99" }, { "x-forwarded-for": "192.0.2.100" }, { forwarded: "for=192.0.2.101" }]) {
      limited(await send(routeOrigin, "127.0.0.6", { market: "BTC_USD", sessionId: "direct_999" }, headers));
    }
    for (const identity of ["::ffff:127.0.0.6", "0:0:0:0:0:ffff:7f00:6"]) {
      limited(await send(routeOrigin, "127.0.0.2", { market: "BTC_USD" }, { "x-real-ip": identity }));
    }
    for (let i = 0; i < 5; i++) {
      issuedCookie(await send(routeOrigin, "127.0.0.2", { market: "ETH_USD" }, { "x-real-ip": i % 2 ? "2001:db8::1" : "2001:0db8:0:0:0:0:0:1" }));
    }
    limited(await send(routeOrigin, "127.0.0.2", { market: "ETH_USD" }, { "x-real-ip": "2001:DB8::1" }));
    for (const headers of [{}, { "x-real-ip": "not-an-ip" }, { "x-real-ip": "192.0.2.1, 192.0.2.2" }, { "x-real-ip": "fe80::1%eth0" }, { "x-forwarded-for": "192.0.2.3" }]) {
      const invalid = await send(routeOrigin, "127.0.0.2", { market: "BTC_USD", sessionId: "invalid_123" }, headers);
      assert.equal(invalid.status, 400);
      assert.deepEqual(invalid.body, { error: "Invalid client address" });
    }
    assert.deepEqual((await snapshot()).byMarket, { BTC_USD: 5, ETH_USD: 5 });
  }

  async function presenceCapacityPreservesVisitorsAndExpires(): Promise<void> {
    await clearExpired();
    const first = issuedCookie(await send(proxyOrigin, "127.20.0.1", { market: "ETH_USD" }));
    for (let base = 1; base < 5_000; base += 32) {
      await Promise.all(Array.from({ length: Math.min(32, 5_000 - base) }, async (_, offset) => {
        const index = base + offset;
        issuedCookie(await send(proxyOrigin, `127.21.${Math.floor(index / 256)}.${index % 256}`, { market: "BTC_USD" }));
      }));
    }
    limited(await send(proxyOrigin, "127.22.0.1", { market: "BTC_USD", sessionId: "overflow_123" }));
    assert.equal((await snapshot()).total, 5_000);
    const heartbeat = await send(proxyOrigin, "127.20.0.1", { market: "ETH_USD" }, { cookie: first });
    assert.equal(heartbeat.status, 200);
    assert.equal(heartbeat.headers["set-cookie"], undefined);
    assert.deepEqual((await snapshot()).byMarket, { ETH_USD: 1, BTC_USD: 4_999 });
    advance(300_000);
    assert.equal((await snapshot()).total, 0);
    issuedCookie(await send(proxyOrigin, "127.22.0.1", { market: "BTC_USD" }));
    assert.equal((await snapshot()).total, 1);
  }

  await t.test(presenceNewestFiveMinuteSampleAcrossFullHistory);
  await t.test(presenceServerIdentitiesBoundSpoofedRegistrations);
  await t.test(presenceDirectSocketBudgetsIgnoreForwardedHeaders);
  await t.test(presenceCapacityPreservesVisitorsAndExpires);
}

test(presenceRepairsUseRealHttp);

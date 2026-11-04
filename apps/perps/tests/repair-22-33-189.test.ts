import assert from "node:assert/strict";
import { once } from "node:events";
import {
  createServer,
  request,
  type IncomingHttpHeaders,
  type OutgoingHttpHeaders,
  type Server,
} from "node:http";
import type { AddressInfo } from "node:net";
import { test, type TestContext } from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { apiResolver } from "next/dist/server/api-utils/node/api-resolver";

type Reply = { status: number; headers: IncomingHttpHeaders; body: unknown };

async function listen(server: Server): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

function send(
  origin: string,
  localAddress: string,
  headers: OutgoingHttpHeaders = {},
): Promise<Reply> {
  return new Promise((resolve, reject) => {
    const req = request(
      `${origin}/api/candle/marks`,
      { localAddress, headers, agent: false },
      (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (chunk: Buffer) => chunks.push(chunk));
        res.on("error", reject);
        res.on("end", () => {
          try {
            resolve({
              status: res.statusCode!,
              headers: res.headers,
              body: JSON.parse(Buffer.concat(chunks).toString()),
            });
          } catch (error) {
            reject(error);
          }
        });
      },
    );
    req.on("error", reject);
    req.setTimeout(10_000, () =>
      req.destroy(new Error("Fixture request timeout")),
    );
    req.end();
  });
}

function allowed(reply: Reply): void {
  assert.equal(reply.status, 200);
  assert.deepEqual(reply.body, []);
}

function limited(reply: Reply): void {
  assert.equal(reply.status, 429);
  assert.equal(reply.headers["retry-after"], "60");
  assert.deepEqual(reply.body, { error: "Too many requests" });
}

async function candleVerifiedIdentityAndBoundedAdmission(
  t: TestContext,
): Promise<void> {
  const route = createServer();
  const servers = [route];
  const savedEnv = new Map<string, string | undefined>();
  t.after(async () => {
    for (const server of servers.reverse()) {
      server.closeAllConnections();
      if (server.listening) {
        await new Promise<void>((resolve, reject) => {
          server.close((error) => (error ? reject(error) : resolve()));
        });
      }
    }
    for (const [key, value] of savedEnv) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
  const routeOrigin = await listen(route);
  const fixtureEnv = {
    NEXT_PUBLIC_PERPS_GATEWAY_URL: routeOrigin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: routeOrigin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${routeOrigin}/api/v2`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: routeOrigin.replace("http:", "ws:"),
    MARKET_PROXY_ORIGIN: routeOrigin,
    CANDLE_TRUSTED_PROXY_IPS: "::ffff:127.0.0.2",
  };
  for (const [key, value] of Object.entries(fixtureEnv)) {
    savedEnv.set(key, process.env[key]);
    process.env[key] = value;
  }
  const { default: handler } =
    await import("../src/pages/api/candle/[...path]");
  route.on("request", (req, res) => {
    const url = new URL(req.url!, routeOrigin);
    void apiResolver(
      req,
      res,
      { path: url.pathname.slice("/api/candle/".length).split("/") },
      { default: handler },
      {
        dev: false,
        previewModeId: "",
        previewModeEncryptionKey: "",
        previewModeSigningKey: "",
      },
      false,
    );
  });

  // The committed ingress fixture overwrites the identity from its real socket
  // and connects to Next from the explicitly trusted proxy address.
  const proxy = createServer((req, res) => {
    const forwarded = request(
      new URL(req.url!, routeOrigin),
      {
        method: req.method,
        localAddress: "127.0.0.2",
        agent: false,
        headers: { ...req.headers, "x-real-ip": req.socket.remoteAddress },
      },
      (upstream) => {
        res.writeHead(upstream.statusCode!, upstream.headers);
        upstream.pipe(res);
      },
    );
    forwarded.on("error", (error) => res.destroy(error));
    req.pipe(forwarded);
  });
  servers.push(proxy);
  const proxyOrigin = await listen(proxy);
  const started = Date.now();

  async function candleDirectPeersCannotRotateForwardedHeaders(): Promise<void> {
    for (let i = 0; i < 120; i++) {
      allowed(
        await send(routeOrigin, "127.0.0.1", {
          "x-forwarded-for": `198.51.100.${i + 1}, 127.0.0.2`,
          "x-real-ip": `203.0.113.${i + 1}`,
          forwarded: `for=192.0.2.${i + 1}`,
        }),
      );
    }
    for (const headers of [
      {},
      { "x-forwarded-for": "192.0.2.250" },
      { "x-forwarded-for": ["192.0.2.251", "192.0.2.252"] },
      { "x-real-ip": "192.0.2.253" },
      { "x-real-ip": "127.0.0.2", "x-forwarded-for": "192.0.2.254" },
    ]) {
      limited(await send(routeOrigin, "127.0.0.1", headers));
    }
  }

  async function candleTrustedProxyUsesVerifiedSocketIdentity(): Promise<void> {
    for (let i = 0; i < 120; i++) {
      allowed(
        await send(proxyOrigin, "127.0.0.3", {
          "x-forwarded-for": `198.51.100.${i + 1}`,
          "x-real-ip": `203.0.113.${i + 1}`,
        }),
      );
    }
    limited(
      await send(proxyOrigin, "127.0.0.3", {
        "x-forwarded-for": "192.0.2.200",
        "x-real-ip": "127.0.0.4",
      }),
    );
    allowed(await send(proxyOrigin, "127.0.0.4"));
    limited(
      await send(routeOrigin, "127.0.0.2", {
        "x-real-ip": "::ffff:127.0.0.3",
      }),
    );
    limited(
      await send(routeOrigin, "127.0.0.2", {
        "x-real-ip": "0:0:0:0:0:ffff:7f00:3",
      }),
    );
    for (let i = 0; i < 120; i++) {
      allowed(
        await send(routeOrigin, "127.0.0.2", {
          "x-real-ip": i % 2 ? "2001:db8::1" : "2001:0db8:0:0:0:0:0:1",
        }),
      );
    }
    limited(
      await send(routeOrigin, "127.0.0.2", { "x-real-ip": "2001:DB8::1" }),
    );
  }

  async function candleTrustedProxyRejectsInvalidIdentity(): Promise<void> {
    for (const headers of [
      {},
      { "x-forwarded-for": "198.51.100.1" },
      { "x-real-ip": "not-an-ip" },
      { "x-real-ip": "198.51.100.1, 198.51.100.2" },
      { "x-real-ip": ["198.51.100.1", "198.51.100.2"] },
      { "x-real-ip": "[2001:db8::1]:443" },
      { "x-real-ip": "fe80::1%eth0" },
      { "x-real-ip": "a".repeat(128) },
    ]) {
      const reply = await send(routeOrigin, "127.0.0.2", headers);
      assert.equal(reply.status, 400);
      assert.deepEqual(reply.body, { error: "Invalid client address" });
    }
  }

  async function candleBucketCapacityRejectsExcessWithoutEviction(): Promise<void> {
    // Four identities already exist: direct peer, two proxied peers and IPv6.
    for (let base = 0; base < 4_996; base += 32) {
      await Promise.all(
        Array.from(
          { length: Math.min(32, 4_996 - base) },
          async (_, offset) => {
            const index = base + offset;
            const ip = `127.10.${Math.floor(index / 256)}.${index % 256}`;
            allowed(await send(proxyOrigin, ip));
          },
        ),
      );
    }
    assert.ok(
      Date.now() - started < 60_000,
      "All 5000 buckets must still be active",
    );
    for (let i = 1; i <= 32; i++) {
      limited(await send(proxyOrigin, `127.11.0.${i}`));
    }
    allowed(await send(proxyOrigin, "127.0.0.4"));
    limited(await send(proxyOrigin, "127.0.0.3"));
    limited(await send(routeOrigin, "127.0.0.1"));
  }

  async function candleExpiredBucketsReleaseCapacityAndResetBudgets(): Promise<void> {
    await delay(60_050);
    allowed(await send(proxyOrigin, "127.11.0.1"));
    allowed(await send(proxyOrigin, "127.0.0.3"));
    allowed(await send(routeOrigin, "127.0.0.1"));
  }

  await t.test(candleDirectPeersCannotRotateForwardedHeaders);
  await t.test(candleTrustedProxyUsesVerifiedSocketIdentity);
  await t.test(candleTrustedProxyRejectsInvalidIdentity);
  await t.test(candleBucketCapacityRejectsExcessWithoutEviction);
  await t.test(candleExpiredBucketsReleaseCapacityAndResetBudgets);
}

test({ timeout: 120_000 }, candleVerifiedIdentityAndBoundedAdmission);

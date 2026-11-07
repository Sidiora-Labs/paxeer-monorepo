import assert from "node:assert/strict";
import { createServer, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import { setTimeout as delay } from "node:timers/promises";
import { test } from "node:test";

async function healthFixture() {
  let requests = 0;
  let respond = (response: ServerResponse) => {
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end(JSON.stringify({ latest_block: 1234, chain_connected: true }));
  };
  const paths: string[] = [];
  const server = createServer((request, response) => {
    requests++;
    paths.push(request.url ?? "");
    respond(response);
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  const production = await import("../src/widgets/network-pill/NetworkPill");
  return {
    ...production,
    paths,
    url: `${origin}/api/explorer/health`,
    get requests() {
      return requests;
    },
    respondWith(handler: typeof respond) {
      respond = handler;
    },
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      });
    },
  };
}

async function until(predicate: () => boolean, message: string) {
  const deadline = Date.now() + 5000;
  while (!predicate() && Date.now() < deadline) await delay(10);
  assert.ok(predicate(), message);
}

async function f043_failedRefreshMarksLastBlockStaleAndRecovers() {
  const fixture = await healthFixture();
  const observations: ReturnType<
    InstanceType<typeof fixture.NetworkStatusMonitor>["getSnapshot"]
  >[] = [];
  const monitor = new fixture.NetworkStatusMonitor(
    (status) => observations.push(status),
    fixture.url,
  );
  try {
    assert.deepEqual(monitor.getSnapshot(), {
      blockNumber: null,
      latency: null,
      observedAt: null,
      stale: true,
    });
    assert.equal(fixture.networkBlockLabel(monitor.getSnapshot()), "…");
    const started = Date.now();
    monitor.start();
    await until(() => !monitor.getSnapshot().stale, "initial health was observed");
    const first = monitor.getSnapshot();
    assert.equal(first.blockNumber, 1234);
    assert.ok(first.latency !== null && first.latency >= 0);
    assert.ok(first.observedAt !== null && first.observedAt >= started);
    assert.ok(first.observedAt <= Date.now());
    assert.equal(fixture.networkBlockLabel(first), `#${(1234).toLocaleString()}`);

    for (const failure of ["http", "parse", "network", "disconnected"] as const) {
      monitor.stop();
      fixture.respondWith((response) => {
        if (failure === "network") return response.destroy();
        response.writeHead(failure === "http" ? 503 : 200, {
          "Content-Type": "application/json",
        });
        response.end(
          failure === "parse"
            ? "{invalid-json"
            : JSON.stringify({ latest_block: 9999, chain_connected: false }),
        );
      });
      const beforeRequests = fixture.requests;
      const lastGood = monitor.getSnapshot();
      monitor.start();
      await until(() => monitor.getSnapshot().stale, `${failure} marks stale`);
      assert.equal(
        fixture.requests - beforeRequests,
        failure === "http" || failure === "network" ? 3 : 1,
        "the real transport applies its retry policy",
      );
      assert.deepEqual(monitor.getSnapshot(), {
        blockNumber: lastGood.blockNumber,
        observedAt: lastGood.observedAt,
        latency: null,
        stale: true,
      });
      assert.equal(
        fixture.networkBlockLabel(monitor.getSnapshot()),
        `#${(1234).toLocaleString()} (stale)`,
      );

      // Reopening keeps the stale observation until the health body arrives.
      monitor.stop();
      let held: ServerResponse | undefined;
      fixture.respondWith((response) => {
        held = response;
      });
      monitor.start();
      await until(
        () => held !== undefined,
        "reopened health request reached server",
      );
      assert.equal(monitor.getSnapshot().stale, true);
      assert.equal(monitor.getSnapshot().latency, null);
      held!.writeHead(200, { "Content-Type": "application/json" });
      held!.end(JSON.stringify({ latest_block: 1234, chain_connected: true }));
      await until(() => !monitor.getSnapshot().stale, "connected response recovers");
      assert.equal(monitor.getSnapshot().blockNumber, 1234);
      assert.ok(monitor.getSnapshot().observedAt! >= lastGood.observedAt!);
      assert.ok(monitor.getSnapshot().latency !== null);
      assert.equal(
        fixture.networkBlockLabel(monitor.getSnapshot()),
        `#${(1234).toLocaleString()}`,
      );
    }

    // A completed request from a closed popover cannot overwrite a newer one.
    monitor.stop();
    let oldResponse: ServerResponse | undefined;
    fixture.respondWith((response) => {
      oldResponse = response;
    });
    monitor.start();
    await until(() => oldResponse !== undefined, "old request is in flight");
    monitor.stop();
    fixture.respondWith((response) => {
      response.end(JSON.stringify({ latest_block: 5678, chain_connected: true }));
    });
    monitor.start();
    await until(
      () => monitor.getSnapshot().blockNumber === 5678,
      "new observation wins",
    );
    const current = monitor.getSnapshot();
    const count = observations.length;
    const finished = new Promise<void>((resolve) =>
      oldResponse!.once("finish", resolve),
    );
    oldResponse!.end(JSON.stringify({ latest_block: 1, chain_connected: true }));
    await finished;
    await delay(100);
    assert.deepEqual(monitor.getSnapshot(), current);
    assert.equal(observations.length, count);
    assert.ok(fixture.paths.every((path) => path === "/api/explorer/health"));

    monitor.dispose();
    await delay(300);
    assert.equal(observations.length, count, "disposed monitor publishes nothing");
  } finally {
    monitor.dispose();
    await fixture.close();
  }
}

async function f043_closedObservationsExpireBeforeReopen() {
  const fixture = await healthFixture();
  let expired: (() => void) | undefined;
  const monitor = new fixture.NetworkStatusMonitor(
    (status) => {
      if (status.stale && status.observedAt !== null) expired?.();
    },
    fixture.url,
  );
  try {
    monitor.start();
    await until(() => !monitor.getSnapshot().stale, "health was observed");
    const first = monitor.getSnapshot();
    monitor.stop();
    const requestCount = fixture.requests;
    let deadline: ReturnType<typeof setTimeout> | undefined;
    try {
      await new Promise<void>((resolve, reject) => {
        expired = resolve;
        deadline = setTimeout(
          () => reject(new Error("closed observation did not expire")),
          fixture.NETWORK_STATUS_MAX_AGE_MS + 2000,
        );
      });
    } finally {
      clearTimeout(deadline);
    }
    assert.ok(Date.now() - first.observedAt! >= fixture.NETWORK_STATUS_MAX_AGE_MS);
    assert.equal(fixture.requests, requestCount, "closed popover does not poll");
    assert.deepEqual(monitor.getSnapshot(), {
      ...first,
      latency: null,
      stale: true,
    });
    assert.equal(
      fixture.networkBlockLabel(monitor.getSnapshot()),
      `#${(1234).toLocaleString()} (stale)`,
    );

    let held: ServerResponse | undefined;
    fixture.respondWith((response) => {
      held = response;
    });
    monitor.start();
    await until(() => held !== undefined, "reopen starts a new health request");
    assert.equal(monitor.getSnapshot().stale, true);
    assert.equal(monitor.getSnapshot().latency, null);
    held!.end(JSON.stringify({ latest_block: 5678, chain_connected: true }));
    await until(() => !monitor.getSnapshot().stale, "fresh health replaces expiry");
    assert.equal(monitor.getSnapshot().blockNumber, 5678);
    assert.ok(monitor.getSnapshot().observedAt! > first.observedAt!);
    assert.ok(monitor.getSnapshot().latency !== null);
    assert.equal(
      fixture.networkBlockLabel(monitor.getSnapshot()),
      `#${(5678).toLocaleString()}`,
    );
  } finally {
    monitor.dispose();
    await fixture.close();
  }
}

test(f043_failedRefreshMarksLastBlockStaleAndRecovers);
test(f043_closedObservationsExpireBeforeReopen);

import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, readdir, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { Window } from "happy-dom";

// 22.33.354 closes explorer-rum-route-drop with explorer_detail_vitals_reach_sink.
// Baseline verification was not run: the ordered gates require complete implementation first.
// No out-of-path wiring is required; WebVitalsRum uses the collector exercised here.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") {
      return nextResolve("next/dist/compiled/server-only/empty.js", context);
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const source = new URL(`${specifier}.ts`, context.parentURL);
      if (existsSync(source)) return nextResolve(source.href, context);
    }
    return nextResolve(specifier, context);
  },
});

const { classifyPerformanceRoute, ROUTE_SCRIPT_BUDGETS } = await import("../src/perf/budgets.ts");
const { collectWebVital } = await import("../src/perf/collect-web-vital.ts");
const { parseRedactedWebVital } = await import("../src/perf/rum-store.ts");
const { persistPerformanceObservation, performanceSinkSnapshot } = await import(
  "../src/api/performance-sink.ts"
);
const { accountIdentifierPath } = await import("../src/explorer/model.ts");

const DETAIL_PATHS = [
  accountIdentifierPath(`0x${"12".repeat(20)}`),
  `/explorer/programs/${"34".repeat(32)}`,
  `/explorer/receipts/${"56".repeat(32)}`,
] as const;

const EXCLUDED_PATHS = [
  "/explorer/verify",
  "/explorer/verify/",
  "/explorer/verify/receipt",
  "/explorer/batches",
  "/explorer/batches/1",
  "/explorer/checkpoints",
  "/explorer/checkpoints/1",
  "/paxeer-x/anchors",
  "/paxeer-x/receipts/1",
  "/explorer/accounts",
  "/explorer/accounts/",
  "/explorer/programs",
  "/explorer/programs/",
  "/explorer/receipts",
  "/explorer/receipts/",
  "/explorer/unknown/id",
  "/explorer/accounts/id/unknown",
  "/explorer/programs/id/unknown",
  "/explorer/receipts/id/unknown",
  "/explorer/accounts-extra/id",
  "/explorer-extra/accounts/id",
  "/app/deposit",
  "/app/deposit/id",
  "/app/withdraw",
  "/app/withdraw/id",
  "/app/move/withdraw",
  "/app/move/withdraw/id",
  "/app/journeys",
  "/app/journeys/id",
  "/app/settings/exit",
  "/app/settings/exit/id",
] as const;

function explorer_detail_templates_are_budgeted() {
  for (const route of [
    "/explorer",
    "/explorer/accounts/[accountId]",
    "/explorer/programs/[programId]",
    "/explorer/receipts/[receiptId]",
  ] as const) {
    assert.ok(existsSync(new URL(`../src/app${route}/page.tsx`, import.meta.url)), route);
    assert.equal(ROUTE_SCRIPT_BUDGETS[route], ROUTE_SCRIPT_BUDGETS["/explorer"]);
    assert.equal(classifyPerformanceRoute(route), "explorer", route);
  }
  for (const pathname of [...DETAIL_PATHS, accountIdentifierPath(`did:layerx:${"78".repeat(32)}`)]) {
    assert.equal(classifyPerformanceRoute(pathname), "explorer", pathname);
    assert.equal(classifyPerformanceRoute(`${pathname}/`), "explorer", pathname);
  }
  for (const pathname of EXCLUDED_PATHS) {
    assert.equal(classifyPerformanceRoute(pathname), undefined, pathname);
  }
}

async function explorer_detail_vitals_reach_sink() {
  const directory = await mkdtemp(path.join(tmpdir(), "repair-22-33-354-"));
  const previousStorage = process.env.LAYERX_RUM_STORAGE_DIRECTORY;
  const previousNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  const previousBlob = Object.getOwnPropertyDescriptor(globalThis, "Blob");
  const received: string[] = [];
  const failures: unknown[] = [];
  const server = createServer((request, response) => {
    void (async () => {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/api/performance/vitals");
      assert.equal(request.headers["content-type"], "application/json");
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString("utf8");
      const observation = parseRedactedWebVital(JSON.parse(body));
      assert.ok(observation, body);
      const result = await persistPerformanceObservation(observation);
      assert.equal(result.accepted, true);
      assert.equal(result.durable, true);
      received.push(body);
      response.writeHead(202).end();
    })().catch((error: unknown) => {
      failures.push(error);
      response.writeHead(500).end();
    });
  });
  let browser: Window | undefined;
  try {
    process.env.LAYERX_RUM_STORAGE_DIRECTORY = directory;
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    const origin = `http://127.0.0.1:${address.port}`;
    browser = new Window({ url: origin });
    Object.defineProperty(globalThis, "navigator", { configurable: true, value: browser.navigator });
    Object.defineProperty(globalThis, "Blob", { configurable: true, value: browser.Blob });
    assert.deepEqual((await performanceSinkSnapshot()).aggregates, []);

    const samples = [
      { LCP: 1_000, INP: 100, CLS: 0.01 },
      { LCP: 2_000, INP: 180, CLS: 0.09 },
      { LCP: 2_500, INP: 201, CLS: 0.11 },
    ] as const;
    const expected: string[] = [];
    for (const [index, pathname] of DETAIL_PATHS.entries()) {
      const sample = samples[index];
      assert.ok(sample);
      browser.location.href = `${origin}${pathname}?private=not-for-telemetry#private-fragment`;
      for (const name of ["LCP", "INP", "CLS"] as const) {
        collectWebVital(browser.location.pathname, { name, value: sample[name] });
        expected.push(JSON.stringify({
          version: 1,
          route: "explorer",
          metric: name,
          observed: Math.round(sample[name] * (name === "CLS" ? 1_000_000 : 1_000)),
        }));
      }
      await browser.happyDOM.waitUntilComplete();
      assert.deepEqual(failures, []);
      assert.equal(received.length, expected.length, `${pathname} must reach the sink`);
    }
    assert.deepEqual([...received].sort(), [...expected].sort());

    for (const pathname of EXCLUDED_PATHS) {
      collectWebVital(pathname, { name: "LCP", value: 1_000 });
    }
    for (const metric of [
      { name: "FCP", value: 100 },
      { name: "LCP", value: -1 },
      { name: "INP", value: Number.NaN },
      { name: "CLS", value: Number.POSITIVE_INFINITY },
      { name: "LCP", value: 120_001 },
    ]) {
      collectWebVital(DETAIL_PATHS[0], metric);
    }
    await browser.happyDOM.waitUntilComplete();
    assert.deepEqual(failures, []);
    assert.equal(received.length, 9, "retired paths and invalid observations emit no beacon");
    const snapshot = await performanceSinkSnapshot();
    assert.equal(snapshot.health.mode, "durable-file");
    assert.equal(snapshot.health.delivery, "healthy");
    assert.deepEqual(snapshot.aggregates, [
      { route: "explorer", metric: "CLS", samples: 3, percentile75: 110_000, withinBudget: false },
      { route: "explorer", metric: "INP", samples: 3, percentile75: 201_000, withinBudget: false },
      { route: "explorer", metric: "LCP", samples: 3, percentile75: 2_500_000, withinBudget: true },
    ]);
    const records = await readdir(directory);
    assert.equal(records.length, 9);
    const stored = await Promise.all(records.map((name) => readFile(path.join(directory, name), "utf8")));
    assert.deepEqual(stored.map((record) => record.trim()).sort(), [...expected].sort());
  } finally {
    await browser?.happyDOM.close();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
    if (previousStorage === undefined) delete process.env.LAYERX_RUM_STORAGE_DIRECTORY;
    else process.env.LAYERX_RUM_STORAGE_DIRECTORY = previousStorage;
    for (const [name, descriptor] of [["navigator", previousNavigator], ["Blob", previousBlob]] as const) {
      if (descriptor === undefined) Reflect.deleteProperty(globalThis, name);
      else Object.defineProperty(globalThis, name, descriptor);
    }
    await rm(directory, { recursive: true, force: true });
  }
}

test(explorer_detail_templates_are_budgeted);
test(explorer_detail_vitals_reach_sink);

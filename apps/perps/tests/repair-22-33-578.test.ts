import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import { isMainThread, parentPort, Worker, workerData } from "node:worker_threads";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { decodeFunctionData, encodeFunctionResult, type Address, type Hex } from "viem";
import type { AccountState } from "../src/core/perps/calls";
import type { PortfolioBalanceReader, PortfolioBalanceSnapshot } from "../src/page-modules/portfolio/balances";

const alice: Address = "0x0000000000000000000000000000000000000001";
const bob: Address = "0x0000000000000000000000000000000000000002";
const accountState: AccountState = {
  collateral: 100_000_000n,
  withdrawable: 40_000_000n,
  equityUsd: 100n * 10n ** 18n,
  initialMarginUsd: 60n * 10n ** 18n,
  maintenanceMarginUsd: 30n * 10n ** 18n,
  feeTier: 2,
  volume14dUsd: 1_000n * 10n ** 18n,
  positionCount: 1n,
  openOrderCount: 3n,
};

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => { resolve = done; });
  return { promise, resolve };
}

type Source = "account" | "wallet";
const ledger: { source: Source; address: Address; target: Address }[] = [];
const failures = new Set<Source>();
const stores = new Map<Address, { account: AccountState; wallet: bigint }>([
  [alice, { account: accountState, wallet: 25_000_000n }],
  [bob, { account: { ...accountState, collateral: 200_000_000n }, wallet: 75_000_000n }],
]);
let hold: { address: Address; source: Source; entered: ReturnType<typeof deferred>; release: ReturnType<typeof deferred> } | undefined;
const serverErrors: unknown[] = [];
let balances: typeof import("../src/page-modules/portfolio/balances");
let boundary: typeof import("../src/page-modules/portfolio/PortfolioBalanceBoundary");
let orders: typeof import("../src/page-modules/portfolio/PortfolioOrdersAvailability");
let perps: typeof import("../src/core/network/abi/PerpsPrecompile");
let config: typeof import("../src/core/perps/config");
let precompiles: typeof import("../src/core/network/precompiles");

type RpcRequest = { id: number; method: string; params?: [{ to: Address; data: Hex }] };
function json(response: ServerResponse, id: number, payload: { result: unknown } | { error: { code: number; message: string } }) {
  response.writeHead(200, { "content-type": "application/json" });
  response.end(JSON.stringify({ jsonrpc: "2.0", id, ...payload }));
}

const server = createServer((request, response) => {
  void (async () => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const rpc = JSON.parse(Buffer.concat(chunks).toString("utf8")) as RpcRequest;
    if (rpc.method === "eth_chainId") return json(response, rpc.id, { result: "0x7d" });
    if (rpc.method === "eth_blockNumber") return json(response, rpc.id, { result: "0x1" });
    if (rpc.method === "net_listening") return json(response, rpc.id, { result: true });
    assert.equal(rpc.method, "eth_call");
    const call = rpc.params![0];
    const native = call.to.toLowerCase() === precompiles.PRECOMPILES.perps.toLowerCase();
    const decoded = decodeFunctionData({ abi: native ? perps.PerpsPrecompileABI : config.erc20Abi, data: call.data });
    assert.equal(decoded.functionName, native ? "getAccount" : "balanceOf");
    if (!native) assert.equal(call.to.toLowerCase(), config.COLLATERAL_TOKENS.usdl.address.toLowerCase());
    const address = String(decoded.args![0]).toLowerCase() as Address;
    const source: Source = native ? "account" : "wallet";
    ledger.push({ source, address, target: call.to });
    const state = stores.get(address);
    assert.ok(state, "the RPC consumer requested a fixture account");
    // Encode a snapshot of the in-memory ledger before delaying its delivery.
    const result = native ? encodeFunctionResult({
      abi: perps.PerpsPrecompileABI, functionName: "getAccount", result: state.account,
    }) : encodeFunctionResult({ abi: config.erc20Abi, functionName: "balanceOf", result: state.wallet });
    const failed = failures.has(source);
    const pending = hold;
    if (pending?.address === address && pending.source === source) {
      hold = undefined;
      pending.entered.resolve();
      await pending.release.promise;
    }
    if (failed) return json(response, rpc.id, { error: { code: -32000, message: `${source} read unavailable` } });
    json(response, rpc.id, { result });
  })().catch((error) => {
    serverErrors.push(error);
    response.destroy(error instanceof Error ? error : new Error(String(error)));
  });
});

async function setupPortfolioFixture() {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  perps = await import("../src/core/network/abi/PerpsPrecompile");
  config = await import("../src/core/perps/config");
  precompiles = await import("../src/core/network/precompiles");
  balances = await import("../src/page-modules/portfolio/balances");
  boundary = await import("../src/page-modules/portfolio/PortfolioBalanceBoundary");
  orders = await import("../src/page-modules/portfolio/PortfolioOrdersAvailability");
}

async function teardownPortfolioFixture() {
  hold?.release.resolve();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  assert.deepEqual(serverErrors, []);
}

function view(reader: PortfolioBalanceReader) {
  return React.createElement(boundary.PortfolioBalanceBoundary, {
    snapshot: reader.getSnapshot(), onRetry: reader.refresh,
    children: React.createElement("span", null, "Verified financial balances"),
  });
}

function findRetry(element: React.ReactNode): (() => Promise<void>) | undefined {
  if (!React.isValidElement<{ children?: React.ReactNode; onClick?: () => Promise<void> }>(element)) return;
  if (element.type === "button") return element.props.onClick;
  for (const child of React.Children.toArray(element.props.children)) {
    const retry = findRetry(child);
    if (retry) return retry;
  }
}

async function portfolioRpcZeroFailedSourcesAndRetry() {
  if (isMainThread) return runPortfolioRepairWorker("portfolioRpcZeroFailedSourcesAndRetry");
  for (const failedSources of [["account"], ["wallet"], ["account", "wallet"]] as const) {
    failures.clear();
    failedSources.forEach((source) => failures.add(source));
    const reader = new balances.PortfolioBalanceReader(alice);
    const notifications: PortfolioBalanceSnapshot[] = [];
    const unsubscribe = reader.subscribe(() => notifications.push(reader.getSnapshot()));
    try {
      assert.match(renderToStaticMarkup(view(reader)), /Loading portfolio balances/);
      assert.doesNotMatch(renderToStaticMarkup(view(reader)), /Verified financial balances/);
      await reader.refresh();
      const snapshot = reader.getSnapshot();
      for (const source of ["account", "wallet"] as const) {
        assert.equal(snapshot[source].status, failures.has(source) ? "error" : "ready");
        if (snapshot[source].status === "error") assert.ok(snapshot[source].error instanceof Error);
      }
      assert.deepEqual(balances.portfolioBalances(snapshot), [], "a failed read never produces an invented zero balance");
      const html = renderToStaticMarkup(view(reader));
      assert.match(html, /Portfolio balances are unavailable/);
      assert.match(html, /Retry balances/);
      assert.doesNotMatch(html, /Verified financial balances|\$0/);
      for (const source of failedSources) assert.match(html, new RegExp(`${source === "account" ? "Account" : "Wallet"} balance could not be loaded`));
      const retry = findRetry(boundary.PortfolioBalanceBoundary(view(reader).props));
      assert.ok(retry, "the rendered retry control is bound to the real reader");
      failures.clear();
      const callsBeforeRetry = ledger.length;
      const retrying = retry();
      assert.equal(reader.getSnapshot().account.status, "loading");
      assert.equal(reader.getSnapshot().wallet.status, "loading");
      assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), []);
      await retrying;
      assert.deepEqual(ledger.slice(callsBeforeRetry).map((entry) => entry.source).sort(), ["account", "wallet"]);
      assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), [{
        ...config.COLLATERAL_TOKENS.usdl, vaultBalance: 100_000_000n, lockedBalance: 60_000_000n,
        available: 40_000_000n, walletBalance: 25_000_000n,
      }]);
      assert.match(renderToStaticMarkup(view(reader)), /Verified financial balances/);
      assert.ok(notifications.some((state) => state.account.status === "error" || state.wallet.status === "error"));
    } finally {
      unsubscribe();
      reader.cancel();
      failures.clear();
    }
  }
}

async function portfolioRpcZeroAccountSwitchIgnoresInFlightReads() {
  if (isMainThread) return runPortfolioRepairWorker("portfolioRpcZeroAccountSwitchIgnoresInFlightReads");
  const oldReader = new balances.PortfolioBalanceReader(alice);
  await oldReader.refresh();
  assert.equal(balances.portfolioBalances(oldReader.getSnapshot())[0].vaultBalance, 100_000_000n);
  const pending = { address: alice, source: "account" as const, entered: deferred(), release: deferred() };
  hold = pending;
  const oldRequest = oldReader.refresh();
  try {
    await pending.entered.promise;
    oldReader.cancel();
    const newReader = new balances.PortfolioBalanceReader(bob);
    const initial = newReader.getSnapshot();
    assert.equal(initial.address, bob);
    assert.equal(initial.account.status, "loading");
    assert.equal(initial.wallet.status, "loading");
    assert.deepEqual(balances.portfolioBalances(initial), []);
    assert.doesNotMatch(renderToStaticMarkup(view(newReader)), /Verified financial balances/);
    await newReader.refresh();
    const selected = newReader.getSnapshot();
    assert.equal(balances.portfolioBalances(selected)[0].vaultBalance, 200_000_000n);
    assert.equal(balances.portfolioBalances(selected)[0].walletBalance, 75_000_000n);
    pending.release.resolve();
    await oldRequest;
    assert.equal(newReader.getSnapshot(), selected, "late Alice responses cannot publish into Bob's state");
    assert.equal(oldReader.getSnapshot().account.status, "loading", "cancelled readers ignore late completion");
    const disconnected = new balances.PortfolioBalanceReader(null);
    const callsBeforeDisconnect = ledger.length;
    await disconnected.refresh();
    assert.equal(disconnected.getSnapshot().account.status, "idle");
    assert.deepEqual(balances.portfolioBalances(disconnected.getSnapshot()), []);
    assert.equal(ledger.length, callsBeforeDisconnect);
    assert.ok(ledger.some((entry) => entry.address === alice && entry.source === "account"));
    assert.ok(ledger.some((entry) => entry.address === bob && entry.source === "wallet"));
    newReader.cancel();
  } finally {
    pending.release.resolve();
    await oldRequest;
    oldReader.cancel();
  }
}

async function portfolioRpcZeroSupersededRefreshAndTrueZero() {
  if (isMainThread) return runPortfolioRepairWorker("portfolioRpcZeroSupersededRefreshAndTrueZero");
  const reader = new balances.PortfolioBalanceReader(alice);
  const pending = { address: alice, source: "wallet" as const, entered: deferred(), release: deferred() };
  hold = pending;
  const oldRequest = reader.refresh();
  const original = stores.get(alice)!;
  try {
    await pending.entered.promise;
    assert.equal(reader.getSnapshot().wallet.status, "loading");
    assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), []);
    stores.set(alice, { account: { ...accountState, collateral: 0n, withdrawable: 0n }, wallet: 0n });
    await reader.refresh();
    const current = reader.getSnapshot();
    assert.equal(current.account.status, "ready");
    assert.equal(current.wallet.status, "ready");
    assert.equal(balances.portfolioBalances(current)[0].vaultBalance, 0n);
    assert.equal(balances.portfolioBalances(current)[0].walletBalance, 0n);
    assert.match(renderToStaticMarkup(view(reader)), /Verified financial balances/);
    pending.release.resolve();
    await oldRequest;
    assert.equal(reader.getSnapshot(), current);
  } finally {
    pending.release.resolve();
    await oldRequest;
    reader.cancel();
    stores.set(alice, original);
  }
}

async function portfolioOrdersDiamondAvailabilityMatchesNativeVenue() {
  if (isMainThread) return runPortfolioRepairWorker("portfolioOrdersDiamondAvailabilityMatchesNativeVenue");
  const html = renderToStaticMarkup(React.createElement(orders.PortfolioOrdersAvailability));
  assert.match(html, /Native perps order management is not available in Portfolio yet/);
  assert.match(html, /View and cancel your orders on the trading page/);
  assert.match(html, /href="\/perps"/);
  assert.doesNotMatch(html, /Diamond|OrderBook facet|No open orders/i);
  const page = readFileSync(new URL("../src/page-modules/portfolio/index.tsx", import.meta.url), "utf8");
  assert.match(page, /<PortfolioOrdersAvailability\s*\/>/);
  assert.doesNotMatch(page, /Diamond|OrderBook facet/);
  assert.match(page, /usePortfolioBalances\(/);
  assert.match(page, /<PortfolioBalanceBoundary\s+snapshot=\{balanceSnapshot\}\s+onRetry=\{retryBalances\}>/);
}

// The production transport ranks RPC endpoints on recurring timers. Each worker
// owns that transport and is disposed only after assertions and fixture teardown.
async function runPortfolioRepairWorker(testName: string) {
  const worker = new Worker(new URL(import.meta.url), {
    workerData: { testName },
    execArgv: ["--import", "tsx"],
  });
  let deadline: ReturnType<typeof setTimeout> | undefined;
  try {
    await new Promise<void>((resolve, reject) => {
      deadline = setTimeout(() => reject(new Error(`${testName} timed out`)), 30_000);
      worker.once("message", (message: { completed?: string }) => {
        if (message.completed === testName) resolve();
        else reject(new Error(`Unexpected worker completion for ${testName}`));
      });
      worker.once("error", reject);
      worker.once("exit", (code) => reject(new Error(`${testName} exited before completion: ${code}`)));
    });
  } finally {
    clearTimeout(deadline);
    await worker.terminate();
  }
}

if (isMainThread) {
  test(portfolioRpcZeroFailedSourcesAndRetry);
  test(portfolioRpcZeroAccountSwitchIgnoresInFlightReads);
  test(portfolioRpcZeroSupersededRefreshAndTrueZero);
  test(portfolioOrdersDiamondAvailabilityMatchesNativeVenue);
} else {
  async function executePortfolioRepairWorker() {
    const repairTest = [
      portfolioRpcZeroFailedSourcesAndRetry,
      portfolioRpcZeroAccountSwitchIgnoresInFlightReads,
      portfolioRpcZeroSupersededRefreshAndTrueZero,
      portfolioOrdersDiamondAvailabilityMatchesNativeVenue,
    ].find((candidate) => candidate.name === workerData.testName);
    assert.ok(repairTest, "the worker must execute a declared repair test");
    await setupPortfolioFixture();
    try {
      await repairTest();
    } finally {
      await teardownPortfolioFixture();
    }
    assert.ok(parentPort);
    parentPort.postMessage({ completed: repairTest.name });
  }
  void executePortfolioRepairWorker();
}

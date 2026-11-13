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
let topBar: typeof import("../src/widgets/portfolio/PortfolioTopBar");
let overview: typeof import("../src/widgets/portfolio/PortfolioOverviewPanel");
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
  topBar = await import("../src/widgets/portfolio/PortfolioTopBar");
  overview = await import("../src/widgets/portfolio/PortfolioOverviewPanel");
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
    snapshot: reader.getSnapshot(),
    onRetry: reader.refresh,
    children: React.createElement("span", null, "Verified financial balances"),
  });
}

function findRetry(
  element: React.ReactNode,
): (() => Promise<void>) | undefined {
  if (
    !React.isValidElement<{
      children?: React.ReactNode;
      onClick?: () => Promise<void>;
    }>(element)
  )
    return;
  if (element.type === "button") return element.props.onClick;
  for (const child of React.Children.toArray(element.props.children)) {
    const retry = findRetry(child);
    if (retry) return retry;
  }
}

function findHeaderButton(
  element: React.ReactNode,
  label: string,
):
  | React.ReactElement<{
      children?: React.ReactNode;
      onClick: () => void | Promise<void>;
      disabled?: boolean;
    }>
  | undefined {
  if (
    !React.isValidElement<{
      children?: React.ReactNode;
      onClick: () => void | Promise<void>;
      disabled?: boolean;
    }>(element)
  )
    return;
  const children = React.Children.toArray(element.props.children);
  if (children.includes(label)) {
    assert.equal(typeof element.props.onClick, "function");
    return element;
  }
  for (const child of children) {
    const button = findHeaderButton(child, label);
    if (button) return button;
  }
}

async function portfolioTopBarBalanceFailureAndRetry() {
  if (isMainThread)
    return runPortfolioRepairWorker("portfolioTopBarBalanceFailureAndRetry");
  const page = readFileSync(
    new URL("../src/page-modules/portfolio/index.tsx", import.meta.url),
    "utf8",
  );
  assert.match(
    page,
    /const\s*\{\s*snapshot:\s*balanceSnapshot,\s*balances,\s*retry:\s*retryBalances,?\s*\}\s*=\s*usePortfolioBalances\(/,
  );
  const header = page.match(/<PortfolioTopBar\b[\s\S]*?\/>/);
  assert.ok(header, "PortfolioPage renders the production header");
  assert.match(
    header[0],
    /\{\.\.\.portfolioTopBarBalanceProps\(\s*balanceSnapshot,\s*retryBalances,?\s*\)\}/,
  );
  assert.match(
    page,
    /<PortfolioOverviewPanel\b[\s\S]*?balanceSnapshot=\{balanceSnapshot\}[\s\S]*?onRetryBalances=\{retryBalances\}/,
  );

  const actions: string[] = [];
  const onDeposit = () => {
    actions.push("deposit");
  };
  const onWithdraw = () => {
    actions.push("withdraw");
  };
  const onConnect = () => {
    actions.push("connect");
  };
  function headerView(
    snapshot: PortfolioBalanceSnapshot,
    reader: PortfolioBalanceReader,
  ) {
    const props = {
      activeNav: "overview" as const,
      isConnected: snapshot.address !== null,
      hasVault: snapshot.address !== null,
      ...topBar.portfolioTopBarBalanceProps(snapshot, reader.refresh),
      onDeposit,
      onWithdraw,
      onConnect,
    };
    assert.equal(props.accountStatus, snapshot.account.status);
    assert.equal(props.walletStatus, snapshot.wallet.status);
    assert.equal(props.onRetryBalances, reader.refresh);
    const element = topBar.PortfolioTopBar(props);
    return { snapshot, element, html: renderToStaticMarkup(element) };
  }
  function assertFailure(
    view: ReturnType<typeof headerView>,
    pending: boolean,
  ) {
    assert.match(view.html, /role="alert"/);
    assert.match(view.html, /Portfolio balances are unavailable/);
    assert.match(view.html, /Retry balances/);
    assert.doesNotMatch(
      view.html,
      /Resolving vault|Deposit|Withdraw|Connect wallet|\$0/,
    );
    const retry = findHeaderButton(view.element, "Retry balances");
    assert.ok(retry, "the displayed retry control has a callback");
    assert.equal(retry.props.disabled, pending);
    assert.equal(/<button[^>]*disabled=""/.test(view.html), pending);
    return retry.props.onClick;
  }
  function assertPending(view: ReturnType<typeof headerView>) {
    assert.match(view.html, /role="status"/);
    assert.match(view.html, /aria-busy="true"/);
    assert.match(view.html, /Resolving vault/);
    assert.doesNotMatch(view.html, /role="alert"|Deposit|Withdraw|\$0/);
  }

  for (const failedSources of [
    ["account"],
    ["wallet"],
    ["account", "wallet"],
  ] as const) {
    failures.clear();
    failedSources.forEach((source) => failures.add(source));
    const reader = new balances.PortfolioBalanceReader(alice);
    const pending = {
      address: alice,
      source:
        failedSources[0] === "account"
          ? ("wallet" as const)
          : ("account" as const),
      entered: deferred(),
      release: deferred(),
    };
    const errorReached = deferred();
    const notifications: ReturnType<typeof headerView>[] = [];
    const unsubscribe = reader.subscribe(() => {
      const view = headerView(reader.getSnapshot(), reader);
      notifications.push(view);
      if (view.snapshot[failedSources[0]].status === "error")
        errorReached.resolve();
    });
    hold = pending;
    const initialRead = reader.refresh();
    try {
      assertPending(headerView(reader.getSnapshot(), reader));
      await pending.entered.promise;
      await errorReached.promise;
      const partial = headerView(reader.getSnapshot(), reader);
      assert.equal(partial.snapshot[pending.source].status, "loading");
      assertFailure(partial, true);
      pending.release.resolve();
      await initialRead;
      for (const source of ["account", "wallet"] as const) {
        assert.equal(
          reader.getSnapshot()[source].status,
          failures.has(source) ? "error" : "ready",
        );
      }
      assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), []);
      const retry = assertFailure(
        headerView(reader.getSnapshot(), reader),
        false,
      );
      assert.equal(retry, reader.refresh);
      const callsBeforeRepeatedFailure = ledger.length;
      const repeated = retry();
      assert.equal(reader.getSnapshot().account.status, "loading");
      assert.equal(reader.getSnapshot().wallet.status, "loading");
      assertPending(headerView(reader.getSnapshot(), reader));
      await repeated;
      assert.deepEqual(
        ledger
          .slice(callsBeforeRepeatedFailure)
          .map((entry) => entry.source)
          .sort(),
        ["account", "wallet"],
      );
      const retryAgain = assertFailure(
        headerView(reader.getSnapshot(), reader),
        false,
      );
      assert.equal(retryAgain, reader.refresh);
      failures.clear();
      const callsBeforeRecovery = ledger.length;
      const recovering = retryAgain();
      assertPending(headerView(reader.getSnapshot(), reader));
      assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), []);
      await recovering;
      assert.deepEqual(
        ledger
          .slice(callsBeforeRecovery)
          .map((entry) => entry.source)
          .sort(),
        ["account", "wallet"],
      );
      const recovered = headerView(reader.getSnapshot(), reader);
      assert.equal(recovered.snapshot.account.status, "ready");
      assert.equal(recovered.snapshot.wallet.status, "ready");
      assert.match(recovered.html, /Deposit/);
      assert.match(recovered.html, /Withdraw/);
      assert.doesNotMatch(
        recovered.html,
        /role="alert"|Retry balances|Resolving vault|Connect wallet/,
      );
      const deposit = findHeaderButton(recovered.element, "Deposit");
      const withdraw = findHeaderButton(recovered.element, "Withdraw");
      assert.ok(deposit);
      assert.ok(withdraw);
      assert.equal(deposit.props.onClick, onDeposit);
      assert.equal(withdraw.props.onClick, onWithdraw);
      await deposit.props.onClick();
      await withdraw.props.onClick();
      for (const view of notifications) {
        const failed =
          view.snapshot.account.status === "error" ||
          view.snapshot.wallet.status === "error";
        const loading =
          view.snapshot.account.status === "loading" ||
          view.snapshot.wallet.status === "loading";
        if (failed) assertFailure(view, loading);
        else if (loading) assertPending(view);
        else {
          assert.match(view.html, /Deposit/);
          assert.match(view.html, /Withdraw/);
        }
      }
      assert.ok(
        notifications.some(
          (view) =>
            view.snapshot.account.status === "loading" &&
            view.snapshot.wallet.status === "loading",
        ),
      );
      assert.ok(
        notifications.some(
          (view) =>
            view.snapshot.account.status === "error" ||
            view.snapshot.wallet.status === "error",
        ),
      );
    } finally {
      pending.release.resolve();
      await initialRead;
      unsubscribe();
      reader.cancel();
      failures.clear();
    }
  }
  const disconnected = new balances.PortfolioBalanceReader(null);
  const callsBeforeDisconnect = ledger.length;
  await disconnected.refresh();
  const view = headerView(disconnected.getSnapshot(), disconnected);
  assert.match(view.html, /Connect wallet/);
  assert.doesNotMatch(
    view.html,
    /role="alert"|Retry balances|Resolving vault|Deposit|Withdraw/,
  );
  const connect = findHeaderButton(view.element, "Connect wallet");
  assert.ok(connect);
  assert.equal(connect.props.onClick, onConnect);
  await connect.props.onClick();
  assert.equal(ledger.length, callsBeforeDisconnect);
  assert.deepEqual(actions, [
    "deposit",
    "withdraw",
    "deposit",
    "withdraw",
    "deposit",
    "withdraw",
    "connect",
  ]);
  disconnected.cancel();
}

async function portfolioBalanceIsolationKeepsOverviewVisible() {
  if (isMainThread) return runPortfolioRepairWorker("portfolioBalanceIsolationKeepsOverviewVisible");
  const { JSDOM } = await import("../src/ui/atoms/test-support/dom");
  const dom = new JSDOM("", { url: "http://localhost", pretendToBeVisual: true });
  const reader = new balances.PortfolioBalanceReader(alice);
  const observedStates: PortfolioBalanceSnapshot[] = [];
  const unexpectedAction = () => assert.fail("Server rendering must not dispatch an account action");
  const propsFor = (selected: PortfolioBalanceReader) => ({
    isConnected: true, hasVault: true, address: selected.address,
    totalVaultUsd: 2468.12, unrealizedPnl: -123.45, openPositions: 2,
    realizedPnl: -61.37, totalTrades: 37,
    balances: balances.portfolioBalances(selected.getSnapshot()),
    balanceSnapshot: selected.getSnapshot(), onRetryBalances: selected.refresh,
    dailyPnl: [{ date: new Date().toISOString().slice(0, 10), pnl: "7000000000000000000", volume: "350000000000000000000", trades: 11 }],
    dailyPnlLoading: false, dailyPnlError: null,
    dailyPnlSkippedDays: 0, dailyPnlHistoryCapped: false,
    isLoading: false, privacy: false,
    onTogglePrivacy: unexpectedAction, onRefresh: selected.refresh,
    onConnect: unexpectedAction, onDeposit: unexpectedAction, onWithdraw: unexpectedAction,
  });
  const render = (selected = reader, privacy = false) => {
    observedStates.push(selected.getSnapshot());
    return renderToStaticMarkup(
      React.createElement(overview.PortfolioOverviewPanel, { ...propsFor(selected), privacy }),
    );
  };
  const assertIndependent = (html: string) => {
    assert.match(html, /\$2\.47K/);
    assert.match(html, /-\$123\.45/);
    assert.match(html, /-5\.00%/);
    assert.match(html, /Open positions<\/div><div[^>]*>2<\/div>/);
    assert.match(html, /Realized PnL<\/div><div[^>]*>-\$61\.37<\/div>/);
    assert.match(html, /Trades<\/div><div[^>]*>37<\/div>/);
    for (const label of ["Assets · PnL-only estimate", "Performance", "Daily PnL", "Cumulative PnL", "Daily Volume", "Cumulative Volume", "11 trades"]) {
      assert.ok(html.includes(label), `${label} remains visible independently of balance reads`);
    }
    assert.match(html, /7D PNL<\/div><div[^>]*>\+\$7\.00<\/div>/);
    assert.match(html, /7D Volume<\/div><div[^>]*>\+\$350\.00<\/div>/);
    assert.match(html, /7D ROI<\/div><div[^>]*>\+2\.00%<\/div>/);
    dom.window.document.body.innerHTML = html;
    const refresh = dom.window.document.querySelector<HTMLButtonElement>('button[aria-label="Refresh"]');
    assert.ok(refresh, "Refresh remains visible");
    assert.equal(refresh.disabled, false, "Refresh remains enabled independently of balance reads");
    assert.match(html, /aria-label="Hide balances"/);
    for (const label of ["Deposit", "Withdraw"]) {
      const control = html.match(/<button\b[^>]*>[\s\S]*?<\/button>/g)?.find((button) => button.includes(label));
      assert.ok(control, `${label} remains visible`);
      assert.doesNotMatch(control.match(/^<button[^>]*>/)![0], /\sdisabled(?:\s|=|>)/);
    }
    assert.doesNotMatch(html, /Performance history is unavailable|Could not load data|No data yet/);
  };
  const assertUnresolved = (html: string) => {
    assertIndependent(html);
    const strip = html.match(/<section aria-label="Portfolio balances"[\s\S]*?<\/section>/)?.[0];
    assert.ok(strip, "only the balances strip carries the loading or error boundary");
    assert.doesNotMatch(strip, /Open positions|Realized PnL|Trades|Daily PnL|Deposit|Withdraw|aria-label="Refresh"|Hide balances/);
    assert.doesNotMatch(html, />100<\/span>|>200<\/span>|>USDL<\/span>/);
    assert.doesNotMatch(strip, /\$0|>0</);
  };
  let partialReady: { source: Source; observed: ReturnType<typeof deferred> } | undefined;
  const unsubscribe = reader.subscribe(() => {
    const snapshot = reader.getSnapshot();
    assertIndependent(render());
    if (snapshot.account.status !== "ready" || snapshot.wallet.status !== "ready") {
      assert.deepEqual(balances.portfolioBalances(snapshot), []);
      assertUnresolved(render());
    }
    if (partialReady && snapshot[partialReady.source].status === "loading" &&
        snapshot[partialReady.source === "account" ? "wallet" : "account"].status === "ready") {
      partialReady.observed.resolve();
    }
  });
  let pending: typeof hold;
  let inFlight: Promise<void> | undefined;
  let newReader: PortfolioBalanceReader | undefined;
  try {
    const initial = render();
    assertUnresolved(initial);
    assert.match(initial, /Loading account balance/);
    assert.match(initial, /Loading wallet balance/);
    await reader.refresh();
    const successful = render();
    assertIndependent(successful);
    assert.match(successful, />100<\/span>/);
    assert.match(successful, />USDL<\/span>/);
    assert.doesNotMatch(successful, /Loading portfolio balances|Portfolio balances are unavailable/);
    const hidden = render(reader, true);
    assert.match(hidden, /aria-label="Show balances"/);
    assert.doesNotMatch(hidden, />100<\/span>|123\.45|61\.37|5\.00%|2\.47K/);

    for (const source of ["account", "wallet"] as const) {
      pending = { address: alice, source, entered: deferred(), release: deferred() };
      partialReady = { source, observed: deferred() };
      hold = pending;
      inFlight = propsFor(reader).onRefresh();
      assertUnresolved(render());
      await pending.entered.promise;
      await partialReady.observed.promise;
      assert.equal(reader.getSnapshot()[source].status, "loading");
      assert.equal(reader.getSnapshot()[source === "account" ? "wallet" : "account"].status, "ready");
      assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), []);
      assertUnresolved(render());
      pending.release.resolve();
      await inFlight;
      partialReady = undefined;
      assert.equal(render(), successful, "a delayed refresh restores the same overview and values");
    }

    for (const failedSources of [["account"], ["wallet"], ["account", "wallet"]] as const) {
      failures.clear();
      failedSources.forEach((source) => failures.add(source));
      await reader.refresh();
      const html = render();
      assertUnresolved(html);
      assert.match(html, /Portfolio balances are unavailable/);
      assert.match(html, /Retry balances/);
      for (const source of ["account", "wallet"] as const) {
        const label = source === "account" ? "Account" : "Wallet";
        assert.match(html, new RegExp(`${label} balance ${failures.has(source) ? "could not be loaded" : "loaded"}`));
      }
      assert.deepEqual(balances.portfolioBalances(reader.getSnapshot()), []);
      const props = propsFor(reader);
      const retry = findRetry(boundary.PortfolioBalanceBoundary({
        snapshot: props.balanceSnapshot, onRetry: props.onRetryBalances, children: null,
      }));
      assert.ok(retry, "the overview retry handler reaches the real boundary control");
      failures.clear();
      const callsBeforeRetry = ledger.length;
      inFlight = retry();
      assertUnresolved(render());
      assert.match(render(), /Loading account balance/);
      assert.match(render(), /Loading wallet balance/);
      await inFlight;
      assert.deepEqual(ledger.slice(callsBeforeRetry).map((entry) => entry.source).sort(), ["account", "wallet"]);
      assert.equal(render(), successful, "retry recovers balance values without changing independent content");
    }

    pending = { address: alice, source: "account", entered: deferred(), release: deferred() };
    hold = pending;
    inFlight = reader.refresh();
    await pending.entered.promise;
    reader.cancel();
    newReader = new balances.PortfolioBalanceReader(bob);
    assert.equal(newReader.getSnapshot().address, bob);
    assert.deepEqual(balances.portfolioBalances(newReader.getSnapshot()), []);
    assertUnresolved(render(newReader));
    await newReader.refresh();
    const bobHtml = render(newReader);
    assertIndependent(bobHtml);
    assert.match(bobHtml, />200<\/span>/);
    assert.doesNotMatch(bobHtml, />100<\/span>/);
    pending.release.resolve();
    await inFlight;
    assert.equal(render(newReader), bobHtml, "late prior-account reads never reveal prior-account balances");
    assert.ok(observedStates.some((snapshot) => snapshot.account.status === "ready" && snapshot.wallet.status === "loading"));
    assert.ok(observedStates.some((snapshot) => snapshot.wallet.status === "ready" && snapshot.account.status === "loading"));
    assert.ok(observedStates.some((snapshot) => snapshot.account.status === "error" && snapshot.wallet.status === "error"));
  } finally {
    unsubscribe();
    pending?.release.resolve();
    await inFlight;
    reader.cancel();
    newReader?.cancel();
    failures.clear();
    dom.window.close();
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
  assert.doesNotMatch(page, /PortfolioBalanceBoundary/);
  assert.match(page, /<PortfolioOverviewPanel[\s\S]*?balanceSnapshot=\{balanceSnapshot\}[\s\S]*?onRetryBalances=\{retryBalances\}/);
  const panel = readFileSync(new URL("../src/widgets/portfolio/PortfolioOverviewPanel.tsx", import.meta.url), "utf8");
  assert.match(panel, /<TotalValueCard[\s\S]*?balanceSnapshot=\{balanceSnapshot\}[\s\S]*?onRetryBalances=\{onRetryBalances\}/);
  const strip = panel.match(/<PortfolioBalanceBoundary snapshot=\{balanceSnapshot\} onRetry=\{onRetryBalances\}>([\s\S]*?)<\/PortfolioBalanceBoundary>/);
  assert.ok(strip, "the production boundary surrounds only the balance strip");
  assert.match(strip[1], /topBalances\.map/);
  assert.doesNotMatch(strip[1], /MiniStat|onRefresh|onTogglePrivacy|onDeposit|onWithdraw|PortfolioAssetsChart|PortfolioPerformanceGrid/);
  assert.ok(panel.indexOf('aria-label="Refresh"') < panel.indexOf(strip[0]));
  assert.ok(panel.indexOf(strip[0]) < panel.indexOf('<MiniStat label="Open positions"'));
  assert.ok(panel.indexOf(strip[0]) < panel.indexOf('onClick={onDeposit}'));
  assert.match(panel, /\/>\s*<PortfolioAssetsChart/);
  assert.match(panel, /<PortfolioPerformanceGrid/);
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
  test(portfolioTopBarBalanceFailureAndRetry);
  test(portfolioBalanceIsolationKeepsOverviewVisible);
  test(portfolioRpcZeroFailedSourcesAndRetry);
  test(portfolioRpcZeroAccountSwitchIgnoresInFlightReads);
  test(portfolioRpcZeroSupersededRefreshAndTrueZero);
  test(portfolioOrdersDiamondAvailabilityMatchesNativeVenue);
} else {
  async function executePortfolioRepairWorker() {
    const repairTest = [
      portfolioTopBarBalanceFailureAndRetry,
      portfolioBalanceIsolationKeepsOverviewVisible,
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

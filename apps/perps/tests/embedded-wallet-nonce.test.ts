import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import { setTimeout as pause } from "node:timers/promises";
import type { Session } from "@supabase/supabase-js";
import { zeroAddress } from "viem";
import {
  createPaxeerEIP1193Provider,
  type PaxeerProviderOptions,
} from "../packages/paxeer-backend/src/core/wallet/embedded/eip1193";
import {
  PaxeerWallet,
  type TxRequest,
} from "../packages/paxeer-backend/src/core/wallet/embedded/sdk";

const session: Session = {
  access_token: "nonce-test-session",
  refresh_token: "nonce-test-refresh",
  token_type: "bearer",
  expires_in: 3600,
  user: {
    id: "nonce-test-user",
    aud: "authenticated",
    app_metadata: {},
    user_metadata: {},
    created_at: "2026-01-01T00:00:00Z",
  },
};
const hash = `0x${"1".repeat(64)}` as const;

class WalletClient extends PaxeerWallet {
  transactions: TxRequest[] = [];
  currentSession: Session | null = session;
  onSend?: (tx: TxRequest) => Promise<void>;

  constructor() {
    super({
      apiUrl: "/v1/wallet",
      supabaseUrl: "https://zezsqawedbikldiedlse.supabase.co",
      supabaseAnonKey: "nonce-test-publishable",
    });
  }

  override async getSession() {
    return this.currentSession;
  }

  override onAuthStateChange() {
    return () => {};
  }

  override async getWallet() {
    return {
      wallet: {
        id: "nonce-test-wallet",
        address: zeroAddress,
        chain_id: 125,
        created_at: session.user.created_at,
        last_used_at: null,
      },
      chain: { id: 125, rpc_url: "/rpc", explorer_url: null },
    };
  }

  override async sendTransaction(
    tx: TxRequest,
    _session?: Session,
    guard?: () => void,
  ) {
    guard?.();
    this.transactions.push(tx);
    await this.onSend?.(tx);
    guard?.();
    return { tx_hash: hash, address: zeroAddress, chain_id: 125 };
  }
}

function harness(
  t: TestContext,
  options: Pick<
    PaxeerProviderOptions,
    "nonceWaitTimeoutMs" | "rpcTimeoutMs"
  > = {},
) {
  const wallet = new WalletClient();
  const reads: { address: string; tag: string; time: number }[] = [];
  const state = {
    count: 7,
    result: undefined as unknown,
    error: null as { code: number; message: string } | null,
    onRead: undefined as (() => Promise<void>) | undefined,
  };
  const fetchImpl: typeof fetch = async (_url, init) => {
    const request = JSON.parse(String(init?.body));
    assert.equal(request.method, "eth_getTransactionCount");
    assert.deepEqual(request.params, [zeroAddress, "latest"]);
    reads.push({
      address: request.params[0],
      tag: request.params[1],
      time: performance.now(),
    });
    await state.onRead?.();
    return Response.json({
      jsonrpc: "2.0",
      id: request.id,
      ...(state.error
        ? { error: state.error }
        : {
            result:
              state.result === undefined
                ? `0x${state.count.toString(16)}`
                : state.result,
          }),
    });
  };
  const provider = createPaxeerEIP1193Provider({
    wallet,
    chainId: 125,
    rpcUrl: "/rpc",
    fetchImpl,
    ...options,
  });
  t.after(() => provider.destroy());
  const send = (tx: Record<string, unknown> = {}) =>
    provider.request({
      method: "eth_sendTransaction",
      params: [{ from: zeroAddress, to: zeroAddress, ...tx }],
    });
  return { wallet, reads, state, provider, send };
}

async function waitUntil(condition: () => boolean) {
  const deadline = performance.now() + 2000;
  while (!condition() && performance.now() < deadline) await pause(5);
  assert.ok(condition(), "the wallet operation finishes within two seconds");
}

async function aSendWithoutANonceCarriesTheCountAtLatest(t: TestContext) {
  const h = harness(t);
  assert.equal(await h.send(), hash);
  assert.equal(h.wallet.transactions.length, 1);
  assert.equal(h.wallet.transactions[0].nonce, 7);
  assert.deepEqual(
    h.reads.map(({ address, tag }) => ({ address, tag })),
    [{ address: zeroAddress, tag: "latest" }],
  );
}
test(aSendWithoutANonceCarriesTheCountAtLatest);

async function aNonceOfTheCallerGoesOnAndNothingIsRead(t: TestContext) {
  for (const nonce of [0, 19, "0x14", 21n]) {
    const h = harness(t);
    assert.equal(await h.send({ nonce }), hash);
    assert.equal(h.wallet.transactions.length, 1);
    assert.equal(h.wallet.transactions[0].nonce, Number(nonce));
    assert.deepEqual(h.reads, []);
  }
}
test(aNonceOfTheCallerGoesOnAndNothingIsRead);

async function aSecondSendWaitsUntilTheCountHasPassedTheFirst(t: TestContext) {
  const h = harness(t);
  await h.send();
  const second = h.send();
  await waitUntil(() => h.reads.length === 2);
  await pause(100);
  assert.equal(h.reads.length, 2);
  assert.equal(h.wallet.transactions.length, 1);
  await waitUntil(() => h.reads.length >= 3);
  assert.equal(h.wallet.transactions.length, 1);
  h.state.count = 8;
  assert.equal(await second, hash);
  assert.deepEqual(
    h.wallet.transactions.map((tx) => tx.nonce),
    [7, 8],
  );
}
test(aSecondSendWaitsUntilTheCountHasPassedTheFirst);

async function afterTheLimitTheSendGoesOnWithTheCountOfTheNode(t: TestContext) {
  const h = harness(t, { nonceWaitTimeoutMs: 300 });
  await h.send();
  const started = performance.now();
  const second = h.send();
  await pause(100);
  assert.equal(h.wallet.transactions.length, 1);
  assert.equal(await second, hash);
  const elapsed = performance.now() - started;
  assert.ok(elapsed >= 300, `the send waited ${elapsed}ms`);
  assert.ok(elapsed < 2000, `the send completed in ${elapsed}ms`);
  assert.ok(h.reads.length >= 3);
  assert.deepEqual(
    h.wallet.transactions.map((tx) => tx.nonce),
    [7, 7],
  );
}
test(afterTheLimitTheSendGoesOnWithTheCountOfTheNode);

async function twoSendsAtOnceGetTwoNumbers(t: TestContext) {
  const h = harness(t);
  let acceptFirst!: () => void;
  const acceptance = new Promise<void>((resolve) => {
    acceptFirst = resolve;
  });
  h.wallet.onSend = async () => {
    if (h.wallet.transactions.length === 1) {
      await acceptance;
      h.state.count = 8;
    }
  };
  const first = h.send();
  const second = h.send();
  await waitUntil(() => h.wallet.transactions.length === 1);
  await pause(50);
  assert.equal(h.wallet.transactions.length, 1);
  assert.equal(h.reads.length, 1);
  acceptFirst();
  assert.deepEqual(await Promise.all([first, second]), [hash, hash]);
  assert.equal(h.reads.length, 2);
  assert.deepEqual(
    h.wallet.transactions.map((tx) => tx.nonce),
    [7, 8],
  );
}
test(twoSendsAtOnceGetTwoNumbers);

async function aReadThatFailsSendsNothing(t: TestContext) {
  const h = harness(t);
  h.state.error = { code: -32000, message: "Transaction count unavailable" };
  await assert.rejects(h.send(), h.state.error);
  assert.equal(h.reads.length, 1);
  assert.deepEqual(h.wallet.transactions, []);
  h.state.error = null;
  assert.equal(await h.send(), hash);
  assert.deepEqual(
    h.wallet.transactions.map((tx) => tx.nonce),
    [7],
  );
}
test(aReadThatFailsSendsNothing);

async function aCountMustBeAHexQuantityAndASafeInteger(t: TestContext) {
  for (const result of [
    null,
    7,
    "7",
    "0x",
    "0x00",
    "0x-1",
    "0x20000000000000",
    {},
  ]) {
    const h = harness(t);
    h.state.result = result;
    await assert.rejects(h.send(), {
      code: -32603,
      message: "Invalid wallet transaction count",
    });
    assert.deepEqual(h.wallet.transactions, []);
  }
  for (const count of [0, Number.MAX_SAFE_INTEGER]) {
    const h = harness(t);
    h.state.count = count;
    await h.send();
    assert.equal(h.wallet.transactions[0].nonce, count);
  }
}
test(aCountMustBeAHexQuantityAndASafeInteger);

async function theNonceWaitLimitMustBeWithinItsRange(t: TestContext) {
  for (const nonceWaitTimeoutMs of [0, -1, 1.5, NaN, Infinity, 60_001]) {
    assert.throws(
      () => harness(t, { nonceWaitTimeoutMs }),
      /nonceWaitTimeoutMs must be an integer between 1 and 60000/,
    );
  }
  for (const nonceWaitTimeoutMs of [1, 60_000]) {
    const h = harness(t, { nonceWaitTimeoutMs });
    await h.send();
    assert.equal(h.wallet.transactions[0].nonce, 7);
  }
}
test(theNonceWaitLimitMustBeWithinItsRange);

async function aSessionChangedDuringTheReadSendsNothing(t: TestContext) {
  const h = harness(t);
  h.state.onRead = async () => {
    h.wallet.currentSession = null;
  };
  await assert.rejects(h.send(), { code: 4100 });
  assert.deepEqual(h.wallet.transactions, []);
}
test(aSessionChangedDuringTheReadSendsNothing);

async function aDestroyedProviderStopsItsQueuedSends(t: TestContext) {
  const h = harness(t);
  await h.send();
  const second = h.send();
  const third = h.send();
  const stopped = Promise.all([
    assert.rejects(second, { code: 4100 }),
    assert.rejects(third, { code: 4100 }),
  ]);
  await waitUntil(() => h.reads.length === 2);
  h.provider.destroy();
  await stopped;
  assert.equal(h.wallet.transactions.length, 1);
  assert.equal(h.reads.length, 2);
}
test(aDestroyedProviderStopsItsQueuedSends);

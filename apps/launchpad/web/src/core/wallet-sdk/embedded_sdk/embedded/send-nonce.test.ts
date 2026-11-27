// @vitest-environment jsdom
import { spawn, type ChildProcess } from "node:child_process";
import { createHmac, randomBytes, randomUUID } from "node:crypto";
import { createServer, type Server } from "node:http";
import type { Session } from "@supabase/supabase-js";
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  expect,
  test,
} from "vitest";
import {
  parseTransaction,
  recoverTransactionAddress,
  type Hex,
  type TransactionSerialized,
} from "viem";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";
import {
  createPaxeerEIP1193Provider,
  type PaxeerEIP1193Provider,
} from "./eip1193";
import { PaxeerWallet, type PublicWallet, type TxRequest } from "./sdk";

const signer = privateKeyToAccount(generatePrivateKey());
const recipient = privateKeyToAccount(generatePrivateKey()).address;
const countReads: unknown[][] = [];
const rpcFaults: string[] = [];
const accepted: { tx: TxRequest; hash: Hex; signed: Hex; observed: unknown }[] =
  [];
const walletRequests: TxRequest[] = [];
let node: ChildProcess | undefined;
let server: Server | undefined;
let nodeUrl: string;
let origin: string;
let session: Session;
let publicWallet: PublicWallet;
let wallet: PaxeerWallet | undefined;
let provider: PaxeerEIP1193Provider | undefined;
let failCountRead = false;
let mineOnAccept = false;
let holdFirstResponse = false;
let releaseResponse: (() => void) | undefined;
let nodeError: Error | undefined;
let holdCountResponse = false;
let releaseCountResponse: (() => void) | undefined;

async function listen(peer: Server): Promise<string> {
  await new Promise<void>((resolve) => peer.listen(0, "127.0.0.1", resolve));
  const address = peer.address();
  if (!address || typeof address === "string")
    throw new Error("loopback bind failed");
  return `http://127.0.0.1:${address.port}`;
}

async function nodeRpc(
  method: string,
  params: unknown[] = [],
): Promise<unknown> {
  const response = await fetch(nodeUrl, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const body = await response.json();
  if (body.error)
    throw Object.assign(new Error(body.error.message), {
      code: body.error.code,
    });
  return body.result;
}

beforeAll(async () => {
  const reservation = createServer();
  nodeUrl = await listen(reservation);
  const port = new URL(nodeUrl).port;
  await new Promise<void>((resolve, reject) =>
    reservation.close((error) => (error ? reject(error) : resolve())),
  );
  node = spawn(
    "anvil",
    [
      "--host",
      "127.0.0.1",
      "--port",
      port,
      "--chain-id",
      "125",
      "--no-mining",
      "--silent",
    ],
    { stdio: "ignore" },
  );
  node.on("error", (error) => {
    nodeError = error;
  });
  const deadline = performance.now() + 2000;
  for (;;) {
    if (nodeError) throw nodeError;
    let chainId: unknown;
    try {
      chainId = await nodeRpc("eth_chainId");
    } catch {
      chainId = undefined;
    }
    if (chainId === "0x7d") break;
    if (performance.now() >= deadline)
      throw new Error("chain startup timed out");
    await new Promise<void>((resolve) => setTimeout(resolve, 25));
  }

  server = createServer(async (req, res) => {
    const send = (status: number, body: unknown) => {
      res.writeHead(status, { "content-type": "application/json" });
      res.end(JSON.stringify(body));
    };
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const text = Buffer.concat(chunks).toString();
      const body = text ? JSON.parse(text) : {};
      if (req.url === "/rpc") {
        if (body.method === "eth_getTransactionCount") {
          countReads.push(body.params);
          if (body.params[1] !== "latest") {
            rpcFaults.push(`unexpected count tag ${body.params[1]}`);
            throw new Error(rpcFaults.at(-1));
          }
          if (failCountRead) body.params = [];
        }
        const response = await fetch(nodeUrl, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(body),
        });
        const result = await response.json();
        if (body.method === "eth_getTransactionCount" && holdCountResponse)
          await new Promise<void>((resolve) => {
            releaseCountResponse = resolve;
          });
        send(response.status, result);
        return;
      }
      if (req.headers.authorization !== `Bearer ${session.access_token}`)
        throw new Error("invalid bearer");
      if (req.url === "/auth/v1/user" && req.method === "GET") {
        send(200, session.user);
        return;
      }
      if (req.url === "/v1/wallet/me" && req.method === "GET") {
        send(200, {
          wallet: publicWallet,
          chain: { id: 125, rpc_url: `${origin}/rpc`, explorer_url: null },
        });
        return;
      }
      if (req.url === "/v1/wallet/send" && req.method === "POST") {
        const tx: TxRequest = body.tx;
        walletRequests.push(tx);
        if (tx.chainId !== 125) throw new Error("foreign transaction chain");
        const signed = await signer.signTransaction({
          type: "eip1559",
          chainId: tx.chainId,
          to: tx.to,
          value: BigInt(tx.value!),
          gas: BigInt(tx.gas!),
          maxFeePerGas: BigInt(tx.maxFeePerGas!),
          maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas!),
          nonce: tx.nonce,
        });
        const hash = (await nodeRpc("eth_sendRawTransaction", [signed])) as Hex;
        const observed = await nodeRpc("eth_getTransactionByHash", [hash]);
        accepted.push({ tx, signed, hash, observed });
        if (accepted.length === 1 && holdFirstResponse)
          await new Promise<void>((resolve) => {
            releaseResponse = resolve;
          });
        if (mineOnAccept) await nodeRpc("evm_mine");
        send(200, { address: signer.address, chain_id: 125, tx_hash: hash });
        return;
      }
      throw new Error(`unexpected route ${req.method} ${req.url}`);
    } catch (error) {
      send(500, { error: "peer_request_failed", message: String(error) });
    }
  });
  origin = await listen(server);
  const id = randomUUID();
  const expiresAt = Math.floor(Date.now() / 1000) + 3600;
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const claims = Buffer.from(
    JSON.stringify({
      sub: id,
      aud: "authenticated",
      role: "authenticated",
      iss: `${origin}/auth/v1`,
      exp: expiresAt,
    }),
  ).toString("base64url");
  const input = `${header}.${claims}`;
  session = {
    access_token: `${input}.${createHmac("sha256", randomBytes(32)).update(input).digest("base64url")}`,
    refresh_token: randomBytes(32).toString("hex"),
    token_type: "bearer",
    expires_in: 3600,
    expires_at: expiresAt,
    user: {
      id,
      aud: "authenticated",
      role: "authenticated",
      app_metadata: {},
      user_metadata: {},
      created_at: new Date().toISOString(),
    },
  };
  publicWallet = {
    id: randomUUID(),
    address: signer.address,
    chain_id: 125,
    created_at: new Date().toISOString(),
    last_used_at: null,
  };
  localStorage.setItem("sb-127-auth-token", JSON.stringify(session));
  wallet = new PaxeerWallet({
    apiUrl: origin,
    supabaseUrl: origin,
    supabaseAnonKey: "embedded-wallet-public-key",
  });
  expect((await wallet.getSession())?.user.id).toBe(id);
});

beforeEach(async () => {
  failCountRead = false;
  mineOnAccept = false;
  holdFirstResponse = false;
  holdCountResponse = false;
  releaseCountResponse = undefined;
  releaseResponse = undefined;
  countReads.length = 0;
  rpcFaults.length = 0;
  accepted.length = 0;
  walletRequests.length = 0;
  await nodeRpc("anvil_reset");
  await nodeRpc("evm_setAutomine", [false]);
  await nodeRpc("anvil_setBalance", [signer.address, "0x3635c9adc5dea00000"]);
  await openProvider();
});

async function openProvider(nonceWaitTimeoutMs = 1000) {
  provider = createPaxeerEIP1193Provider({
    wallet: wallet!,
    chainId: 125,
    rpcUrl: `${origin}/rpc`,
    nonceWaitTimeoutMs,
  });
  await new Promise<void>((resolve) => {
    const unsubscribe: (() => void) | undefined = wallet!.onAuthStateChange(
      (event) => {
        if (event === "INITIAL_SESSION") {
          queueMicrotask(() => unsubscribe?.());
          resolve();
        }
      },
    );
  });
  expect(await provider.request({ method: "eth_requestAccounts" })).toEqual([
    signer.address,
  ]);
}

afterEach(() => {
  releaseResponse?.();
  releaseCountResponse?.();
  provider?.destroy();
  expect(rpcFaults).toEqual([]);
});

afterAll(async () => {
  releaseResponse?.();
  releaseCountResponse?.();
  provider?.destroy();
  await wallet?.supabase.auth.stopAutoRefresh();
  if (server)
    await new Promise<void>((resolve, reject) =>
      server!.close((error) => (error ? reject(error) : resolve())),
    );
  if (node && node.exitCode === null && node.signalCode === null) {
    const stopped = new Promise<void>((resolve) =>
      node!.once("exit", () => resolve()),
    );
    node.kill("SIGTERM");
    await stopped;
  }
  localStorage.clear();
});

function sendTransaction(nonce?: string, replacement = false) {
  return provider!.request({
    method: "eth_sendTransaction",
    params: [
      {
        from: signer.address,
        to: recipient,
        value: replacement ? "0x2" : "0x1",
        gas: "0x5208",
        maxFeePerGas: replacement ? "0xee6b2800" : "0x77359400",
        maxPriorityFeePerGas: replacement ? "0x77359400" : "0x3b9aca00",
        ...(nonce === undefined ? {} : { nonce }),
      },
    ],
  });
}

async function assertAcceptedNonces(expected: number[]) {
  expect(accepted.map(({ tx }) => tx.nonce)).toEqual(expected);
  for (const [index, transaction] of accepted.entries()) {
    expect(parseTransaction(transaction.signed).nonce).toBe(expected[index]);
    expect(
      await recoverTransactionAddress({
        serializedTransaction: transaction.signed as TransactionSerialized,
      }),
    ).toBe(signer.address);
    expect(transaction.observed).toMatchObject({
      from: signer.address.toLowerCase(),
      nonce: `0x${expected[index].toString(16)}`,
    });
  }
}

async function embedded_send_takes_the_count_at_latest() {
  for (const nonceWaitTimeoutMs of [
    0,
    -1,
    1.5,
    60_001,
    NaN,
    Infinity,
    -Infinity,
  ]) {
    expect(() =>
      createPaxeerEIP1193Provider({
        wallet: wallet!,
        chainId: 125,
        rpcUrl: `${origin}/rpc`,
        nonceWaitTimeoutMs,
      }),
    ).toThrow("nonceWaitTimeoutMs must be an integer from 1 to 60000");
  }
  for (const nonceWaitTimeoutMs of [1, 60_000]) {
    const bounded = createPaxeerEIP1193Provider({
      wallet: wallet!,
      chainId: 125,
      rpcUrl: `${origin}/rpc`,
      nonceWaitTimeoutMs,
    });
    bounded.destroy();
  }
  await nodeRpc("anvil_setNonce", [signer.address, "0x7"]);
  expect(await sendTransaction()).toBe(accepted[0].hash);
  expect(countReads).toEqual([[signer.address, "latest"]]);
  await assertAcceptedNonces([7]);
}
test(
  embedded_send_takes_the_count_at_latest,
  embedded_send_takes_the_count_at_latest,
  3000,
);

async function embedded_send_keeps_a_caller_nonce() {
  await sendTransaction("0x2a");
  expect(countReads).toEqual([]);
  await assertAcceptedNonces([42]);
}
test(
  embedded_send_keeps_a_caller_nonce,
  embedded_send_keeps_a_caller_nonce,
  3000,
);

async function embedded_send_waits_for_the_count() {
  await sendTransaction();
  const second = sendTransaction();
  await expect
    .poll(() => countReads.length, { timeout: 500 })
    .toBeGreaterThanOrEqual(3);
  expect(accepted).toHaveLength(1);
  expect(walletRequests).toHaveLength(1);
  expect(
    await nodeRpc("eth_getTransactionCount", [signer.address, "latest"]),
  ).toBe("0x0");
  await nodeRpc("evm_mine");
  await second;
  await assertAcceptedNonces([0, 1]);
}
test(
  embedded_send_waits_for_the_count,
  embedded_send_waits_for_the_count,
  3000,
);

async function embedded_send_goes_on_after_the_limit() {
  provider!.destroy();
  await openProvider(300);
  await sendTransaction();
  const started = performance.now();
  await sendTransaction(undefined, true);
  const elapsed = performance.now() - started;
  expect(elapsed).toBeGreaterThanOrEqual(300);
  expect(elapsed).toBeLessThan(1500);
  expect(countReads.length).toBeGreaterThanOrEqual(3);
  expect(
    await nodeRpc("eth_getTransactionCount", [signer.address, "latest"]),
  ).toBe("0x0");
  await assertAcceptedNonces([0, 0]);
}
test(
  embedded_send_goes_on_after_the_limit,
  embedded_send_goes_on_after_the_limit,
  3000,
);

async function embedded_sends_take_turns() {
  mineOnAccept = true;
  holdFirstResponse = true;
  const first = sendTransaction();
  const second = sendTransaction();
  await expect.poll(() => accepted.length, { timeout: 500 }).toBe(1);
  await new Promise<void>((resolve) => setTimeout(resolve, 120));
  expect(accepted).toHaveLength(1);
  expect(walletRequests).toHaveLength(1);
  expect(countReads).toHaveLength(1);
  releaseResponse!();
  expect(await Promise.all([first, second])).toEqual(
    accepted.map(({ hash }) => hash),
  );
  expect(countReads).toEqual([
    [signer.address, "latest"],
    [signer.address, "latest"],
  ]);
  await assertAcceptedNonces([0, 1]);
}
test(embedded_sends_take_turns, embedded_sends_take_turns, 3000);

async function embedded_send_ends_when_the_read_fails() {
  failCountRead = true;
  await expect(sendTransaction()).rejects.toMatchObject({ code: -32602 });
  expect(countReads).toEqual([[signer.address, "latest"]]);
  expect(accepted).toEqual([]);
  expect(walletRequests).toEqual([]);
}
test(
  embedded_send_ends_when_the_read_fails,
  embedded_send_ends_when_the_read_fails,
  3000,
);

async function embedded_send_limits_a_slow_read() {
  provider!.destroy();
  await openProvider(300);
  holdCountResponse = true;
  const started = performance.now();
  await expect(sendTransaction()).rejects.toMatchObject({ code: -32000 });
  const elapsed = performance.now() - started;
  expect(elapsed).toBeGreaterThanOrEqual(300);
  expect(elapsed).toBeLessThan(1500);
  expect(countReads).toEqual([[signer.address, "latest"]]);
  expect(walletRequests).toEqual([]);
  expect(accepted).toEqual([]);
}
test(embedded_send_limits_a_slow_read, embedded_send_limits_a_slow_read, 3000);

async function embedded_send_tracks_only_assigned_nonces() {
  await sendTransaction("0x2a");
  const started = performance.now();
  await sendTransaction();
  expect(performance.now() - started).toBeLessThan(500);
  expect(countReads).toEqual([[signer.address, "latest"]]);
  await assertAcceptedNonces([42, 0]);
}
test(
  embedded_send_tracks_only_assigned_nonces,
  embedded_send_tracks_only_assigned_nonces,
  3000,
);

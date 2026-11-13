import { after, before, test } from "node:test";
import { strict as assert } from "node:assert";
import { spawn, type ChildProcess } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from "node:http";
import { Agent, request as httpsRequest } from "node:https";
import type { AddressInfo } from "node:net";
import { fileURLToPath } from "node:url";
import {
  getAddress,
  hashMessage,
  recoverAddress,
  recoverMessageAddress,
  recoverTransactionAddress,
  parseTransaction,
  toHex,
  type Hex,
} from "viem";
import {
  createPaxeerEIP1193Provider,
  type PaxeerEIP1193Provider,
} from "../src/core/wallet/embedded/eip1193";
import { PaxeerWallet } from "../src/core/wallet/embedded/sdk";
import {
  constructionDigest,
  toWireConstruction,
} from "@paxeer/wallet/provider";
import type { DigestConstruction } from "@paxeer/wallet/types";

interface Identity {
  url: string;
  issuer: string;
  mint(subject: string): Promise<string>;
  close(): Promise<void>;
}
interface InventoryKey {
  key_id: string;
  owner: string;
  epoch: number;
  curve: "secp256k1";
  public_key: string;
  operations: Array<"generate" | "sign" | "refresh">;
}
interface AttestorNetwork {
  nodes: Array<{ apiUrl: string }>;
  pki: {
    caFile: string;
    clientCert: string;
    clientKey: string;
    operatorCert: string;
    operatorKey: string;
  };
  authority: {
    producerKeyFile: string;
    tenant: string;
    inventoryFile: string;
    inventoryPublicKeyFile: string;
    approveInventory(keys: InventoryKey[]): void;
  };
  stop(): Promise<void>;
}
interface Database {
  query(sql: string, values?: unknown[]): Promise<{ rows: Record<string, unknown>[] }>;
  closePool(): Promise<void>;
}
interface Exchange {
  path: string;
  token: string;
  body: Record<string, unknown> | null;
  status: number;
  response: Record<string, unknown>;
}

const gateway = new URL("../../../services/wallet/gateway/", import.meta.url);
const gatewayDir = fileURLToPath(gateway);
const source = (path: string): string => new URL(path, gateway).href;
const subjectA = randomUUID();
const subjectB = randomUUID();
const delegate = "0x4444444444444444444444444444444444444444";
let identity: Identity;
let postgres: { stop(): Promise<void> };
let db: Database;
let evm: ChildProcess;
let api: ChildProcess;
let authServer: Server;
let proxy: Server;
let rpcUrl: string;
let apiUrl: string;
let proxyUrl: string;
let authUrl: string;
let tokenA: string;
let tokenB: string;
let addressA: Hex;
let addressB: Hex;
let network: AttestorNetwork | undefined;
const wallets: PaxeerWallet[] = [];
const providers: PaxeerEIP1193Provider[] = [];
const traffic: Exchange[] = [];
const processLogs: string[] = [];

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

type Hold = {
  path: string;
  token: string;
  entered: ReturnType<typeof deferred>;
  released: ReturnType<typeof deferred>;
};
let hold: Hold | undefined;
let corrupt: { path: string; apply: (body: Record<string, unknown>) => void } | undefined;

function holdResponse(path: string, token: string): Hold {
  assert.equal(hold, undefined);
  hold = { path, token, entered: deferred(), released: deferred() };
  return hold;
}

async function readBody(req: IncomingMessage): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString("utf8");
}

function json(res: ServerResponse, status: number, body: unknown): void {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

async function listen(server: Server): Promise<string> {
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

async function freePort(): Promise<number> {
  const server = createServer();
  const base = await listen(server);
  const port = Number(new URL(base).port);
  await close(server);
  return port;
}

async function close(server?: Server): Promise<void> {
  if (!server) return;
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
}

function launch(command: string, args: string[], env: NodeJS.ProcessEnv): ChildProcess {
  const child = spawn(command, args, {
    cwd: gatewayDir,
    env,
    stdio: ["ignore", "pipe", "pipe"],
  });
  child.stdout!.on("data", (chunk: Buffer) => processLogs.push(chunk.toString()));
  child.stderr!.on("data", (chunk: Buffer) => processLogs.push(chunk.toString()));
  child.on("error", (error) => processLogs.push(error.message));
  return child;
}

async function stop(child?: ChildProcess): Promise<void> {
  if (!child?.pid || child.exitCode !== null || child.signalCode !== null) return;
  const exited = new Promise<void>((resolve) => child.once("exit", () => resolve()));
  child.kill("SIGTERM");
  const timer = setTimeout(() => child.kill("SIGKILL"), 5_000);
  await exited;
  clearTimeout(timer);
}

async function waitFor(child: ChildProcess, url: string, rpc = false): Promise<void> {
  const deadline = Date.now() + 30_000;
  for (;;) {
    if (!child.pid || child.exitCode !== null || child.signalCode !== null) {
      throw new Error(`Service exited: ${processLogs.join("")}`);
    }
    const res = await fetch(url, {
      ...(rpc
        ? {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_chainId", params: [] }),
          }
        : {}),
      signal: AbortSignal.timeout(1_000),
    }).catch(() => null);
    if (res) {
      await res.text();
      if (res.ok) return;
    }
    if (Date.now() >= deadline) throw new Error(`Service unavailable: ${processLogs.join("")}`);
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
}

async function startGateway(): Promise<void> {
  api = launch(process.execPath, ["--import", "tsx", "src/index.ts"], {
    ...process.env,
    PORT: new URL(apiUrl).port,
    API_WORKERS: "1",
  });
  await waitFor(api, `${apiUrl}/healthz`);
}

async function relay(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const path = req.url!;
  const raw = await readBody(req);
  const token = (req.headers.authorization ?? "").replace(/^Bearer /, "");
  const upstream = await fetch(`${apiUrl}${path}`, {
    method: req.method,
    headers: {
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      ...(raw ? { "content-type": "application/json" } : {}),
    },
    ...(raw ? { body: raw } : {}),
  });
  const response = (await upstream.json()) as Record<string, unknown>;
  traffic.push({
    path,
    token,
    body: raw ? JSON.parse(raw) : null,
    status: upstream.status,
    response: structuredClone(response),
  });
  const paused = hold;
  if (paused?.path === path && paused.token === token) {
    hold = undefined;
    paused.entered.resolve();
    await paused.released.promise;
  }
  if (corrupt?.path === path) {
    const fault = corrupt;
    corrupt = undefined;
    fault.apply(response);
  }
  json(res, upstream.status, response);
}

async function signIn(wallet: PaxeerWallet, token: string): Promise<void> {
  const result = await wallet.supabase.auth.setSession({
    access_token: token,
    refresh_token: "local-test-refresh",
  });
  assert.equal(result.error, null);
}

async function openWallet(token: string): Promise<PaxeerWallet> {
  const wallet = new PaxeerWallet({
    apiUrl: proxyUrl,
    supabaseUrl: authUrl,
    supabaseAnonKey: "local-public-key",
  });
  wallets.push(wallet);
  await signIn(wallet, token);
  return wallet;
}

function openProvider(wallet: PaxeerWallet): PaxeerEIP1193Provider {
  const provider = createPaxeerEIP1193Provider({ wallet, chainId: 125, rpcUrl });
  providers.push(provider);
  return provider;
}

async function connected(token: string) {
  const wallet = await openWallet(token);
  const provider = openProvider(wallet);
  const expected = token === tokenB ? addressB : addressA;
  await eventually(async () => {
    assert.deepEqual(await provider.request({ method: "eth_requestAccounts" }), [expected]);
  });
  return { wallet, provider };
}

async function eventually(assertion: () => Promise<void> | void): Promise<void> {
  const deadline = Date.now() + 5_000;
  for (;;) {
    try {
      await assertion();
      return;
    } catch (error) {
      if (Date.now() >= deadline) throw error;
      await new Promise((resolve) => setTimeout(resolve, 10));
    }
  }
}

function signingTraffic(): Exchange[] {
  return traffic.filter((entry) =>
    [
      "/v1/wallet/sign",
      "/v1/wallet/send",
      "/v1/wallet/sign-message",
      "/v1/wallet/sign-digest",
    ].includes(entry.path),
  );
}

async function fund(address: Hex): Promise<void> {
  const res = await fetch(rpcUrl, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method: "anvil_setBalance",
      params: [address, "0xde0b6b3a7640000"],
    }),
  });
  const body = (await res.json()) as { error?: unknown; result?: unknown };
  assert.equal(body.error, undefined);
  assert.equal(body.result, null);
}

before(
  async function startRealWalletServices() {
    const helpers = (await import(source("test/e2e/attestors.ts"))) as {
      startIdentity(): Promise<Identity>;
    };
    identity = await helpers.startIdentity();
    rpcUrl = `http://127.0.0.1:${await freePort()}`;
    apiUrl = `http://127.0.0.1:${await freePort()}`;
    Object.assign(process.env, {
      NODE_ENV: "test",
      LOG_LEVEL: "error",
      SUPABASE_URL: identity.url,
      HYPERPAXEER_RPC_URL: rpcUrl,
      RPC_URLS: rpcUrl,
      SEQUENCER_RPC_URL: rpcUrl,
      HYPERPAXEER_CHAIN_ID: "125",
      CORS_ORIGINS: "http://localhost:3000",
      WALLET_MASTER_KEY: randomBytes(32).toString("base64"),
      WALLET_AUTHORIZATION_DELEGATES: delegate,
    });
    evm = launch(
      process.env.ANVIL_BIN ?? "anvil",
      ["--host", "127.0.0.1", "--port", new URL(rpcUrl).port, "--chain-id", "125", "--silent"],
      process.env,
    );
    await waitFor(evm, rpcUrl, true);
    if (!process.env.PG_BIN_DIR) {
      const version = readdirSync("/usr/lib/postgresql").find((entry) =>
        existsSync(`/usr/lib/postgresql/${entry}/bin/initdb`),
      );
      assert.ok(version, "A real PostgreSQL server is required");
      process.env.PG_BIN_DIR = `/usr/lib/postgresql/${version}/bin`;
    }
    const pg = (await import(source("test/support/postgres.ts"))) as {
      startPostgres(): Promise<{ stop(): Promise<void> }>;
    };
    postgres = await pg.startPostgres();
    db = (await import(source("src/db/pool.ts"))) as Database;
    const auth = (await import(source("src/auth/jwt.ts"))) as {
      verifyAccessToken(token: string): Promise<{ sub: string }>;
    };
    authServer = createServer((req, res) => {
      void (async () => {
        assert.equal(req.url, "/auth/v1/user");
        const user = await auth.verifyAccessToken(
          (req.headers.authorization ?? "").replace(/^Bearer /, ""),
        );
        json(res, 200, {
          id: user.sub,
          aud: "authenticated",
          role: "authenticated",
          app_metadata: {},
          user_metadata: {},
          created_at: new Date(0).toISOString(),
        });
      })().catch((error: unknown) => json(res, 401, { error: String(error) }));
    });
    authUrl = await listen(authServer);
    await startGateway();
    proxy = createServer((req, res) => {
      void relay(req, res).catch((error: unknown) => json(res, 500, { error: String(error) }));
    });
    proxyUrl = await listen(proxy);
    tokenA = await identity.mint(subjectA);
    tokenB = await identity.mint(subjectB);
    // The real gateway provisions and persists both keys, then returns its own wallet records.
    const a = await openWallet(tokenA);
    const b = await openWallet(tokenB);
    addressA = getAddress((await a.getWallet()).wallet.address);
    addressB = getAddress((await b.getWallet()).wallet.address);
    assert.notEqual(addressA, addressB);
    await fund(addressA);
    await fund(addressB);
  },
  { timeout: 180_000 },
);

after(
  async function stopRealWalletServices() {
    hold?.released.resolve();
    for (const provider of providers) provider.destroy();
    for (const wallet of wallets) await wallet.supabase.auth.stopAutoRefresh();
    await close(proxy);
    await close(authServer);
    await stop(api);
    await network?.stop();
    await stop(evm);
    await db?.closePool();
    await postgres?.stop();
    await identity?.close();
  },
  { timeout: 60_000 },
);

async function personalSignPreservesAuthorizedBytes() {
  const { wallet, provider } = await connected(tokenA);
  const session = await wallet.getSession();
  assert.ok(session);
  const canonical = wallet.createProvider({ chainId: 125, rpcUrl }, session);
  await canonical.request({ method: "eth_requestAccounts" });
  try {
    for (const message of ["Paxeer π\u0000🌐", "\uFEFFsigned BOM"]) {
      const encoded = toHex(message);
      for (const signer of [provider, canonical]) {
        const before = signingTraffic().length;
        const signature = (await signer.request({
          method: "personal_sign",
          params: [encoded, addressA],
        })) as Hex;
        assert.equal(signingTraffic().length, before + 1);
        const exchange = signingTraffic().at(-1)!;
        assert.equal(exchange.path, "/v1/wallet/sign-message");
        assert.equal(exchange.token, tokenA);
        assert.equal(exchange.body!.message, message);
        assert.equal(exchange.status, 200);
        assert.equal(String(exchange.response.address).toLowerCase(), addressA.toLowerCase());
        assert.equal(
          await recoverMessageAddress({ message: { raw: encoded }, signature }),
          addressA,
        );
        assert.equal(signature, exchange.response.signature);
      }
    }
    for (const signer of [provider, canonical]) {
      await assert.rejects(
        signer.request({ method: "personal_sign", params: ["0x", addressA] }),
      );
      assert.equal(signingTraffic().at(-1)!.body!.message, "");
      assert.equal(
        signingTraffic().at(-1)!.status,
        400,
        "the text-only gateway explicitly rejects empty messages",
      );
    }
    for (const invalid of ["0xff", "0xc328", "0xabc", "0x0", "0xzz", "0x61f"]) {
      for (const signer of [provider, canonical]) {
        const before = signingTraffic().length;
        await assert.rejects(
          signer.request({ method: "personal_sign", params: [invalid, addressA] }),
          /UTF-8|even-length/,
        );
        assert.equal(
          signingTraffic().length,
          before,
          "invalid message bytes must never reach a signer",
        );
      }
    }
  } finally {
    canonical.disconnect();
  }
}

test(personalSignPreservesAuthorizedBytes);

async function sessionAccountCacheTracksIdentityAndReopen() {
  const { wallet, provider } = await connected(tokenA);
  const events: unknown[] = [];
  provider.on("accountsChanged", (accounts) => events.push(accounts));
  await signIn(wallet, tokenB);
  await eventually(async () => {
    assert.deepEqual(await provider.request({ method: "eth_accounts" }), [addressB]);
    assert.deepEqual(events.at(-1), [addressB]);
  });
  assert.ok(events.some((event) => Array.isArray(event) && event.length === 0));
  const refreshed = await identity.mint(subjectB);
  const beforeRefresh = events.length;
  await signIn(wallet, refreshed);
  await eventually(async () => {
    assert.deepEqual(await provider.request({ method: "eth_accounts" }), [addressB]);
  });
  assert.equal(events.length, beforeRefresh, "token refresh must preserve the account");
  const message = "current account after refresh";
  const signature = (await provider.request({
    method: "personal_sign",
    params: [toHex(message), addressB],
  })) as Hex;
  assert.equal(await recoverMessageAddress({ message, signature }), addressB);
  assert.equal(signingTraffic().at(-1)!.token, refreshed);
  provider.destroy();
  await assert.rejects(provider.request({ method: "eth_accounts" }), /destroyed/);
  const reopened = openProvider(wallet);
  await eventually(async () => {
    assert.deepEqual(await reopened.request({ method: "eth_accounts" }), [addressB]);
  });
  const recovered = (await reopened.request({
    method: "personal_sign",
    params: [toHex("reopened"), addressB],
  })) as Hex;
  assert.equal(
    await recoverMessageAddress({ message: "reopened", signature: recovered }),
    addressB,
  );
  assert.ok(
    traffic.some((entry) => entry.path === "/v1/wallet/me" && entry.token === refreshed),
  );
  await wallet.signOut();
  await eventually(async () => {
    assert.deepEqual(await reopened.request({ method: "eth_accounts" }), []);
  });
}

test(sessionAccountCacheTracksIdentityAndReopen);

async function staleLookupsAndSigningCannotCrossSessions() {
  const wallet = await openWallet(tokenA);
  const paused = holdResponse("/v1/wallet/me", tokenA);
  const provider = openProvider(wallet);
  const stale = provider.request({ method: "eth_accounts" });
  await paused.entered.promise;
  const events: unknown[] = [];
  provider.on("accountsChanged", (accounts) => events.push(accounts));
  await signIn(wallet, tokenB);
  await eventually(async () => {
    assert.deepEqual(await provider.request({ method: "eth_accounts" }), [addressB]);
  });
  paused.released.resolve();
  assert.deepEqual(await stale, [], "late A lookup must be discarded");
  assert.deepEqual(await provider.request({ method: "eth_accounts" }), [addressB]);
  assert.ok(!events.some((event) => JSON.stringify(event) === JSON.stringify([addressA])));
  const before = signingTraffic().length;
  await assert.rejects(
    provider.request({ method: "personal_sign", params: [toHex("stale account"), addressA] }),
    /signer/,
  );
  assert.equal(signingTraffic().length, before);
  const inFlight = holdResponse("/v1/wallet/sign-message", tokenB);
  const pending = provider.request({
    method: "personal_sign",
    params: [toHex("B authorization"), addressB],
  });
  const rejected = assert.rejects(pending, /session changed/i);
  await inFlight.entered.promise;
  await signIn(wallet, tokenA);
  inFlight.released.resolve();
  await rejected;
  const signed = signingTraffic().at(-1)!;
  assert.equal(signed.token, tokenB);
  assert.equal(String(signed.response.address).toLowerCase(), addressB.toLowerCase());
  assert.deepEqual(await provider.request({ method: "eth_accounts" }), [addressA]);
  // A signing request paused during its first account lookup must not dispatch under B.
  const freshWallet = await openWallet(tokenA);
  const delayed = holdResponse("/v1/wallet/me", tokenA);
  const freshProvider = openProvider(freshWallet);
  const count = signingTraffic().length;
  const attempt = freshProvider.request({
    method: "eth_sendTransaction",
    params: [{ from: addressA, to: addressB, value: "0x0" }],
  });
  const refused = assert.rejects(attempt, /session|connected/i);
  await delayed.entered.promise;
  await signIn(freshWallet, tokenB);
  delayed.released.resolve();
  await refused;
  assert.equal(signingTraffic().length, count);
}

test(staleLookupsAndSigningCannotCrossSessions);

async function signerAndChainConsentRejectMismatchesBeforeDispatch() {
  const { provider } = await connected(tokenA);
  for (const method of ["eth_sendTransaction", "eth_signTransaction"]) {
    for (const tx of [
      { from: addressB, to: addressB },
      { from: addressA, to: addressB, chainId: "0x1" },
      { from: addressA, to: addressB, chainId: 126 },
      { from: "0x1", to: addressB },
      { from: addressA, to: addressB, chainId: "bad" },
    ]) {
      const count = signingTraffic().length;
      await assert.rejects(provider.request({ method, params: [tx] }), /signer|address|chain/i);
      assert.equal(
        signingTraffic().length,
        count,
        "mismatched consent cannot sign or broadcast",
      );
    }
  }
  for (const address of [addressB, undefined, "0x1"]) {
    const count = signingTraffic().length;
    await assert.rejects(
      provider.request({ method: "personal_sign", params: [toHex("consent"), address] }),
      /signer|address/i,
    );
    assert.equal(signingTraffic().length, count);
  }
  const tx = {
    from: addressA.toLowerCase(),
    to: addressB,
    value: "0x1",
    gas: "0x5208",
    maxFeePerGas: "0x77359400",
    maxPriorityFeePerGas: "0x3b9aca00",
    chainId: "0x7d",
    nonce: "0x7",
  };
  const serialized = (await provider.request({
    method: "eth_signTransaction",
    params: [tx],
  })) as `0x02${string}`;
  assert.match(serialized, /^0x02(?:[0-9a-fA-F]{2})+$/);
  assert.equal(
    await recoverTransactionAddress({ serializedTransaction: serialized }),
    addressA,
  );
  assert.equal(parseTransaction(serialized).chainId, 125);
  const exchange = signingTraffic().at(-1)!;
  assert.equal(exchange.path, "/v1/wallet/sign");
  assert.equal((exchange.body!.tx as Record<string, unknown>).chainId, 125);
  assert.equal((exchange.body!.tx as Record<string, unknown>).value, "1");
  assert.equal(exchange.response.signed_tx, serialized);
}

test(signerAndChainConsentRejectMismatchesBeforeDispatch);

async function signingResponsesMustMatchAccountAndChain() {
  const { provider } = await connected(tokenA);
  corrupt = {
    path: "/v1/wallet/sign-message",
    apply: (body) => {
      body.address = addressB;
    },
  };
  await assert.rejects(
    provider.request({ method: "personal_sign", params: [toHex("real signer"), addressA] }),
    /signer/,
  );
  // Metadata can claim A while the genuine signature authorizes different bytes.
  const response = (await provider.request({
    method: "personal_sign",
    params: [toHex("other text"), addressA],
  })) as Hex;
  corrupt = {
    path: "/v1/wallet/sign-message",
    apply: (body) => {
      body.signature = response;
    },
  };
  await assert.rejects(
    provider.request({ method: "personal_sign", params: [toHex("requested text"), addressA] }),
    /signer/,
  );
  for (const [index, field] of ["address", "chain_id"].entries()) {
    corrupt = {
      path: "/v1/wallet/sign",
      apply: (body) => {
        body[field] = field === "address" ? addressB : 126;
      },
    };
    await assert.rejects(
      provider.request({
        method: "eth_signTransaction",
        params: [
          {
            from: addressA,
            to: addressB,
            nonce: 10 + index,
            gas: "0x5208",
            maxFeePerGas: "0x77359400",
            maxPriorityFeePerGas: "0x3b9aca00",
          },
        ],
      }),
      /signer|chain/i,
    );
    assert.equal(
      signingTraffic().at(-1)!.status,
      200,
      "fault must alter a genuine successful signing reply",
    );
  }
  const tx = {
    from: addressA,
    to: addressB,
    value: "0x1",
    gas: "0x5208",
    maxFeePerGas: "0x77359400",
    maxPriorityFeePerGas: "0x3b9aca00",
    chainId: "0x7d",
  };
  const hash = (await provider.request({
    method: "eth_sendTransaction",
    params: [{ ...tx, nonce: 0 }],
  })) as Hex;
  assert.match(hash, /^0x[0-9a-fA-F]{64}$/);
  assert.equal(signingTraffic().at(-1)!.response.tx_hash, hash);
  const broadcast = (await provider.request({
    method: "eth_getTransactionByHash",
    params: [hash],
  })) as { from: string; chainId: string };
  assert.equal(broadcast.from.toLowerCase(), addressA.toLowerCase());
  assert.equal(Number(BigInt(broadcast.chainId)), 125);
  for (const [index, field] of ["address", "chain_id"].entries()) {
    corrupt = {
      path: "/v1/wallet/send",
      apply: (body) => {
        body[field] = field === "address" ? addressB : 126;
      },
    };
    await assert.rejects(
      provider.request({
        method: "eth_sendTransaction",
        params: [{ ...tx, nonce: index + 1 }],
      }),
      /signer|chain/i,
    );
    const exchange = signingTraffic().at(-1)!;
    assert.match(String(exchange.response.tx_hash), /^0x[0-9a-fA-F]{64}$/);
    assert.equal(exchange.response.chain_id, 125);
    assert.equal(String(exchange.response.address).toLowerCase(), addressA.toLowerCase());
  }
  const after = await db.query(
    "select count(*)::text as count from wallet_signatures where user_id=$1",
    [subjectA],
  );
  assert.ok(
    Number(after.rows[0]!.count) >= 4,
    "the gateway audited the real signing operations",
  );
}

test(signingResponsesMustMatchAccountAndChain);

async function attestorCall(
  agent: Agent,
  endpoint: string,
  path: string,
  body: unknown,
): Promise<Record<string, unknown>> {
  return new Promise((resolve, reject) => {
    const req = httpsRequest(
      new URL(path, endpoint),
      {
        agent,
        method: "POST",
        timeout: 180_000,
        headers: { "content-type": "application/json" },
      },
      (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (chunk: Buffer) => chunks.push(chunk));
        res.on("error", reject);
        res.on("end", () => {
          try {
            const reply = JSON.parse(Buffer.concat(chunks).toString("utf8")) as Record<
              string,
              unknown
            >;
            assert.equal(res.statusCode, 200, JSON.stringify(reply));
            resolve(reply);
          } catch (error) {
            reject(error);
          }
        });
      },
    );
    req.on("error", reject);
    req.on("timeout", () => req.destroy(new Error("attestor request timed out")));
    req.end(JSON.stringify(body));
  });
}

async function ethSignUsesCanonicalConstructionAndRecoversSigner() {
  const helper = (await import(source("test/e2e/attestors.ts"))) as {
    startAttestorNetwork(options: {
      identity: Identity;
      rpcUrl: string;
    }): Promise<AttestorNetwork>;
  };
  network = await helper.startAttestorNetwork({ identity, rpcUrl });
  const userId = randomUUID();
  const keyId = `wallet:${userId}:standard:secp256k1:0`;
  const admission: InventoryKey = {
    key_id: keyId,
    owner: userId,
    epoch: 0,
    curve: "secp256k1",
    public_key: "",
    operations: ["generate"],
  };
  network.authority.approveInventory([admission]);
  const tls = {
    ca: readFileSync(network.pki.caFile),
    cert: readFileSync(network.pki.clientCert),
    key: readFileSync(network.pki.clientKey),
  };
  const agent = new Agent(tls);
  const operator = new Agent({
    ...tls,
    cert: readFileSync(network.pki.operatorCert),
    key: readFileSync(network.pki.operatorKey),
  });
  try {
    const sessionId = randomUUID();
    const generated = await Promise.all(
      network.nodes.map((node) =>
        attestorCall(agent, node.apiUrl, "/v1/keys/generate", {
          session_id: sessionId,
          key_id: keyId,
          curve: "secp256k1",
          owner: userId,
        }),
      ),
    );
    const key = generated[0]!;
    assert.ok(
      generated.every(
        (answer) =>
          answer.public_key === key.public_key &&
          answer.address === key.address &&
          answer.epoch === key.epoch,
      ),
    );
    admission.public_key = String(key.public_key);
    admission.epoch = Number(key.epoch);
    admission.operations = ["sign", "refresh"];
    network.authority.approveInventory([admission]);
    if (generated.some((answer) => !answer.refreshed)) {
      const refresh = randomUUID();
      const refreshed = await Promise.all(
        network.nodes.map((node) =>
          attestorCall(operator, node.apiUrl, "/v1/keys/refresh", {
            session_id: refresh,
            key_id: keyId,
          }),
        ),
      );
      assert.ok(
        refreshed.every(
          (answer) =>
            answer.refreshed &&
            answer.public_key === key.public_key &&
            answer.epoch === admission.epoch + 1,
        ),
      );
      admission.epoch += 1;
      network.authority.approveInventory([admission]);
    }
    const address = getAddress(String(key.address));
    await db.query(
      "insert into wallets(user_id,address,chain_id,attestor_key_id,migrated_at,binding_state) values($1,$2,125,$3,now(),'bound')",
      [userId, address.toLowerCase(), keyId],
    );
    await stop(api);
    Object.assign(process.env, {
      ATTESTOR_ENDPOINTS: network.nodes.map((node) => node.apiUrl).join(","),
      ATTESTOR_CLIENT_CERT_FILE: network.pki.clientCert,
      ATTESTOR_CLIENT_KEY_FILE: network.pki.clientKey,
      ATTESTOR_CA_FILE: network.pki.caFile,
      ATTESTOR_QUORUM: "3",
      WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: network.authority.producerKeyFile,
      WALLET_IDENTITY_BINDING_TENANT: network.authority.tenant,
      WALLET_CUSTODY_INVENTORY_FILE: network.authority.inventoryFile,
      WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: network.authority.inventoryPublicKeyFile,
    });
    await startGateway();
    const token = await identity.mint(userId);
    const wallet = await openWallet(token);
    const provider = openProvider(wallet);
    await eventually(async () => {
      assert.deepEqual(await provider.request({ method: "eth_requestAccounts" }), [address]);
    });
    const constructions: DigestConstruction[] = [
      { kind: "eip7702_authorization", chainId: 125, nonce: 0, address: delegate },
      {
        kind: "sponsored_batch",
        chainId: 125,
        account: address,
        nonce: 0,
        calls: [{ to: delegate, value: 0, data: "0x" }],
        quote: {
          sponsor: delegate,
          token: "0x21f7b20a555199fa73A238B1a91FD0f549068fEe",
          maxTokenAmount: 1,
          tokenAmount: 0,
          deadline: Math.floor(Date.now() / 1000) + 300,
          quoteNonce: 0,
          gasCost: 0,
        },
      },
    ];
    const signatures: Hex[] = [];
    for (const construction of constructions) {
      const wire = toWireConstruction(construction);
      const digest = constructionDigest(wire);
      const before = signingTraffic().length;
      const signature = (await provider.request({
        method: "eth_sign",
        params: [address, digest, construction],
      })) as Hex;
      signatures.push(signature);
      const exchange = signingTraffic().at(-1)!;
      assert.equal(signingTraffic().length, before + 1);
      assert.equal(exchange.path, "/v1/wallet/sign-digest");
      assert.equal(exchange.status, 200, JSON.stringify(exchange.response));
      assert.equal(exchange.token, token);
      assert.deepEqual(exchange.body, { construction: wire });
      assert.equal(signature, exchange.response.signature);
      assert.equal(String(exchange.response.address).toLowerCase(), address.toLowerCase());
      assert.equal(await recoverAddress({ hash: digest, signature }), address);
      assert.notEqual(
        await recoverAddress({ hash: hashMessage({ raw: digest }), signature }),
        address,
      );
    }
    const good = constructions[0]!;
    const digest = constructionDigest(toWireConstruction(good));
    for (const params of [
      [address, digest],
      [address, "0x01", good],
      [addressB, digest, good],
      [address, digest, { kind: "unknown" }],
      [address, digest, { ...good, nonce: 1 }],
      [address, digest, { ...good, chainId: 1 }],
      [address, digest, { ...good, nonce: undefined }],
    ]) {
      const before = signingTraffic().length;
      await assert.rejects(provider.request({ method: "eth_sign", params }));
      assert.equal(
        signingTraffic().length,
        before,
        "invalid construction must not reach signing",
      );
    }
    // The production gateway itself refuses an unapproved delegate.
    const denied = { ...constructions[0]!, address: addressB } as DigestConstruction;
    const deniedDigest = constructionDigest(toWireConstruction(denied));
    await assert.rejects(
      provider.request({ method: "eth_sign", params: [address, deniedDigest, denied] }),
    );
    assert.equal(signingTraffic().at(-1)!.status, 403);
    assert.equal(signingTraffic().at(-1)!.response.error, "delegate_not_allowed");
    corrupt = {
      path: "/v1/wallet/sign-digest",
      apply: (body) => {
        body.address = addressB;
      },
    };
    await assert.rejects(
      provider.request({ method: "eth_sign", params: [address, digest, good] }),
      /account/,
    );
    assert.equal(signingTraffic().at(-1)!.status, 200);
    corrupt = {
      path: "/v1/wallet/sign-digest",
      apply: (body) => {
        body.signature = signatures[1]!;
      },
    };
    await assert.rejects(
      provider.request({ method: "eth_sign", params: [address, digest, good] }),
      /account/,
    );
    assert.equal(signingTraffic().at(-1)!.status, 200);
    const session = await wallet.getSession();
    assert.ok(session);
    const canonical = wallet.createProvider({ chainId: 125, rpcUrl }, session);
    await canonical.request({ method: "eth_requestAccounts" });
    try {
      for (const signer of [provider, canonical]) {
        await signIn(wallet, token);
        const paused = holdResponse("/v1/wallet/sign-digest", token);
        const pending = signer.request({ method: "eth_sign", params: [address, digest, good] });
        const refused = assert.rejects(pending, (error: unknown) => {
          assert.ok(error instanceof Error);
          assert.match(error.message, /session changed/i);
          assert.ok("code" in error);
          assert.equal(error.code, 4100);
          return true;
        });
        await paused.entered.promise;
        await signIn(wallet, tokenB);
        paused.released.resolve();
        await refused;
        const exchange = signingTraffic().at(-1)!;
        assert.equal(exchange.token, token);
        assert.equal(exchange.status, 200);
        assert.equal(
          await recoverAddress({ hash: digest, signature: exchange.response.signature as Hex }),
          address,
        );
      }
    } finally {
      canonical.disconnect();
    }
  } finally {
    agent.destroy();
    operator.destroy();
  }
}

test({ timeout: 600_000 }, ethSignUsesCanonicalConstructionAndRecoversSigner);

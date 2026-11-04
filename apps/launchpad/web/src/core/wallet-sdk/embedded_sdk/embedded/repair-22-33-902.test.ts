// @vitest-environment jsdom
import {
  createHmac,
  randomBytes,
  randomUUID,
  timingSafeEqual,
} from "node:crypto";
import { createServer, type Server } from "node:http";
import { act, createElement, useLayoutEffect } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterAll, beforeAll, expect, test } from "vitest";
import type { Session } from "@supabase/supabase-js";
import {
  fromRlp,
  slice,
  toHex,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  type Hex,
  type TransactionSerialized,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { getPaxeerWallet } from "./client";
import {
  PaxeerProvider,
  usePaxeer,
  type PaxeerContextValue,
} from "./PaxeerProvider";
import {
  createPaxeerEIP1193Provider,
  type PaxeerEIP1193Provider,
} from "./eip1193";
import { PaxeerWallet, type PublicWallet } from "./sdk";

const tokenKey = randomBytes(32);
const signers = [
  privateKeyToAccount(`0x${"01".repeat(32)}`),
  privateKeyToAccount(`0x${"02".repeat(32)}`),
];
type Failure = "read" | "provision" | "reread";
const failures = new Map<string, Failure>();
const provisioned = new Set<string>();
const users = new Map<
  string,
  { session: Session; wallet: PublicWallet; signer: (typeof signers)[number] }
>();
const requests: {
  userId: string;
  path: string;
  body: Record<string, unknown>;
}[] = [];
const signedTransactions = new Map<Hex, Hex>();
const providers = new Set<PaxeerEIP1193Provider>();
let server: Server;
let origin: string;
let wallet: PaxeerWallet;
let sessionA: Session;
let sessionB: Session;
let accountA: PublicWallet;
let accountB: PublicWallet;
let root: Root | undefined;
let container: HTMLDivElement | undefined;
const envKeys = [
  "NEXT_PUBLIC_SUPABASE_URL",
  "NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY",
  "NEXT_PUBLIC_PAXEER_WALLET_API",
] as const;
const savedEnv = new Map<string, string | undefined>();

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function holdLookup(userId: string) {
  const arrived = deferred();
  const released = deferred();
  const finished = deferred();
  const hold = { userId, arrived, released, finished, claimed: false };
  holds.add(hold);
  return hold;
}
const holds = new Set<ReturnType<typeof holdLookup>>();

function mintSession(id: string): Session {
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const claims = Buffer.from(
    JSON.stringify({
      sub: id,
      aud: "authenticated",
      role: "authenticated",
      iss: `${origin}/auth/v1`,
      exp: Math.floor(Date.now() / 1000) + 3600,
    }),
  ).toString("base64url");
  const input = `${header}.${claims}`;
  return {
    access_token: `${input}.${createHmac("sha256", tokenKey).update(input).digest("base64url")}`,
    refresh_token: randomBytes(32).toString("hex"),
    token_type: "bearer",
    expires_in: 3600,
    user: {
      id,
      aud: "authenticated",
      role: "authenticated",
      app_metadata: {},
      user_metadata: {},
      created_at: new Date().toISOString(),
    },
  };
}

function authenticate(authorization: string | undefined) {
  if (!authorization?.startsWith("Bearer ")) throw new Error("missing bearer");
  const [header, claims, signature] = authorization.slice(7).split(".");
  const expected = createHmac("sha256", tokenKey)
    .update(`${header}.${claims}`)
    .digest();
  const actual = Buffer.from(signature, "base64url");
  if (actual.length !== expected.length || !timingSafeEqual(actual, expected))
    throw new Error("invalid signature");
  const payload = JSON.parse(Buffer.from(claims, "base64url").toString());
  if (
    payload.iss !== `${origin}/auth/v1` ||
    payload.aud !== "authenticated" ||
    payload.exp <= Date.now() / 1000
  )
    throw new Error("invalid claims");
  const user = users.get(payload.sub);
  if (!user) throw new Error("unknown principal");
  return user;
}

beforeAll(async () => {
  server = createServer(async (req, res) => {
    const send = (status: number, body: unknown) => {
      res.writeHead(status, { "content-type": "application/json" });
      res.end(JSON.stringify(body));
    };
    try {
      const {
        session,
        wallet: publicWallet,
        signer,
      } = authenticate(req.headers.authorization);
      const path = req.url!;
      if (path === "/auth/v1/user" && req.method === "GET") {
        send(200, session.user);
        return;
      }
      if (path.startsWith("/auth/v1/logout") && req.method === "POST") {
        res.writeHead(204);
        res.end();
        return;
      }
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const text = Buffer.concat(chunks).toString();
      const body = text ? JSON.parse(text) : {};
      requests.push({ userId: session.user.id, path, body });
      const failure = failures.get(session.user.id);
      if (path === "/v1/wallet/me" && req.method === "GET") {
        if (
          failure === "read" ||
          (failure === "reread" && provisioned.has(session.user.id))
        ) {
          send(500, { error: "wallet_read_failed" });
          return;
        }
        if (failure === "provision" || failure === "reread") {
          send(404, { error: "wallet_not_found" });
          return;
        }
        const response = {
          wallet: publicWallet,
          chain: { id: 125, rpc_url: origin, explorer_url: null },
        };
        const hold = [...holds].find(
          (entry) => !entry.claimed && entry.userId === session.user.id,
        );
        if (hold) {
          hold.claimed = true;
          hold.arrived.resolve();
          await hold.released.promise;
          send(200, response);
          hold.finished.resolve();
          holds.delete(hold);
        } else send(200, response);
        return;
      }
      if (path === "/v1/wallet/provision" && req.method === "POST") {
        if (failure === "provision")
          send(500, { error: "wallet_provision_failed" });
        else {
          provisioned.add(session.user.id);
          send(200, { wallet: publicWallet });
        }
        return;
      }
      if (
        (path === "/v1/wallet/sign" || path === "/v1/wallet/send") &&
        req.method === "POST"
      ) {
        const tx = body.tx;
        const signed = await signer.signTransaction({
          type: "eip1559",
          chainId: tx.chainId,
          to: tx.to,
          value: BigInt(tx.value ?? 0),
          gas: BigInt(tx.gas),
          maxFeePerGas: BigInt(tx.maxFeePerGas),
          maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas),
          nonce: tx.nonce ?? 0,
        });
        const hash = keccak256(signed);
        signedTransactions.set(hash, signed);
        send(
          200,
          path === "/v1/wallet/sign"
            ? { address: signer.address, chain_id: 125, signed_tx: signed }
            : { address: signer.address, chain_id: 125, tx_hash: hash },
        );
        return;
      }
      throw new Error(`unexpected route ${req.method} ${path}`);
    } catch (error) {
      send(500, { error: "fixture_peer_failure", message: String(error) });
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string")
    throw new Error("loopback bind failed");
  origin = `http://127.0.0.1:${address.port}`;
  for (const signer of signers) {
    const session = mintSession(randomUUID());
    const publicWallet: PublicWallet = {
      id: randomUUID(),
      address: signer.address,
      chain_id: 125,
      created_at: new Date().toISOString(),
      last_used_at: null,
    };
    users.set(session.user.id, { session, wallet: publicWallet, signer });
  }
  const [a, b] = [...users.values()];
  sessionA = a.session;
  accountA = a.wallet;
  sessionB = b.session;
  accountB = b.wallet;
  for (const key of envKeys) savedEnv.set(key, process.env[key]);
  process.env.NEXT_PUBLIC_SUPABASE_URL = origin;
  process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY =
    "embedded-repair-public-key";
  process.env.NEXT_PUBLIC_PAXEER_WALLET_API = origin;
  localStorage.setItem("sb-127-auth-token", JSON.stringify(sessionA));
  const client = getPaxeerWallet();
  if (!client) throw new Error("loopback wallet configuration unavailable");
  wallet = client;
  await wallet.getSession();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
});

afterAll(async () => {
  for (const hold of holds) hold.released.resolve();
  if (root) await act(async () => root!.unmount());
  container?.remove();
  for (const provider of providers) provider.destroy();
  await wallet?.supabase.auth.stopAutoRefresh();
  if (server)
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  localStorage.clear();
  for (const key of envKeys) {
    const previous = savedEnv.get(key);
    if (previous === undefined) delete process.env[key];
    else process.env[key] = previous;
  }
});

async function replaceSession(session: Session) {
  const result = await wallet.supabase.auth.setSession(session);
  expect(result.error).toBeNull();
  expect(result.data.session?.user.id).toBe(session.user.id);
}

async function flushReact(predicate: () => boolean) {
  await expect
    .poll(
      async () => {
        await act(async () => {
          await new Promise<void>((resolve) => setTimeout(resolve, 0));
        });
        return predicate();
      },
      { timeout: 5000 },
    )
    .toBe(true);
}

async function embedded_react_identity() {
  const snapshots: {
    userId: string | null;
    address: string | null;
    authenticated: boolean;
  }[] = [];
  let current: PaxeerContextValue | undefined;
  function Consumer() {
    const context = usePaxeer();
    current = context;
    useLayoutEffect(() => {
      snapshots.push({
        userId: context.user?.id ?? null,
        address: context.publicWallet?.address ?? null,
        authenticated: context.isAuthenticated,
      });
    }, [context]);
    return createElement(
      "output",
      null,
      context.publicWallet?.address ?? "disconnected",
    );
  }
  await replaceSession(sessionA);
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  await act(async () =>
    root!.render(createElement(PaxeerProvider, null, createElement(Consumer))),
  );
  await flushReact(() => current?.isAuthenticated === true);
  expect(container.textContent).toBe(accountA.address);

  for (const failure of ["read", "provision", "reread"] as const) {
    failures.clear();
    provisioned.clear();
    await act(async () => replaceSession(sessionA));
    await flushReact(
      () =>
        current?.publicWallet?.address === accountA.address &&
        !current.isLoading,
    );
    const old = holdLookup(sessionA.user.id);
    await act(async () => current!.refresh());
    await old.arrived.promise;
    failures.set(sessionB.user.id, failure);
    const before = requests.length;
    await act(async () => replaceSession(sessionB));
    await flushReact(
      () =>
        current?.user?.id === sessionB.user.id &&
        !current.isLoading &&
        current.authError !== null,
    );
    expect(current!.publicWallet).toBeNull();
    expect(current!.isAuthenticated).toBe(false);
    expect(container.textContent).toBe("disconnected");
    const attempted = requests
      .slice(before)
      .filter((entry) => entry.userId === sessionB.user.id)
      .map((entry) => entry.path);
    expect(attempted).toEqual(
      failure === "read"
        ? ["/v1/wallet/me"]
        : failure === "provision"
          ? ["/v1/wallet/me", "/v1/wallet/provision"]
          : ["/v1/wallet/me", "/v1/wallet/provision", "/v1/wallet/me"],
    );
    old.released.resolve();
    await old.finished.promise;
    await flushReact(
      () => current?.user?.id === sessionB.user.id && !current.isLoading,
    );
    expect(current!.publicWallet).toBeNull();
  }
  expect(
    snapshots.filter((entry) => entry.userId === sessionB.user.id).length,
  ).toBeGreaterThan(0);
  expect(
    snapshots
      .filter((entry) => entry.userId === sessionB.user.id)
      .every((entry) => entry.address === null && !entry.authenticated),
  ).toBe(true);
  failures.clear();
  await act(async () => replaceSession(sessionB));
  await flushReact(
    () =>
      current?.publicWallet?.address === accountB.address &&
      current.isAuthenticated,
  );
  await act(async () => root!.unmount());
  root = undefined;
  container.remove();
  container = undefined;
}
test(embedded_react_identity, embedded_react_identity);

async function openProvider() {
  const provider = createPaxeerEIP1193Provider({
    wallet,
    chainId: 125,
    rpcUrl: origin,
  });
  providers.add(provider);
  const accounts: unknown[] = [];
  const disconnects: unknown[] = [];
  provider.on("accountsChanged", (value) => accounts.push(value));
  provider.on("disconnect", (value) => disconnects.push(value));
  await expect
    .poll(() => accounts.at(-1), { timeout: 5000 })
    .toEqual([accountA.address]);
  expect(await provider.request({ method: "eth_accounts" })).toEqual([
    accountA.address,
  ]);
  return { provider, accounts, disconnects };
}

async function embedded_principal_cache() {
  await replaceSession(sessionA);
  const { provider, accounts, disconnects } = await openProvider();
  const replacement = holdLookup(sessionB.user.id);
  await replaceSession(sessionB);
  await replacement.arrived.promise;
  expect(accounts.at(-1)).toEqual([]);
  replacement.released.resolve();
  await expect
    .poll(() => accounts.at(-1), { timeout: 5000 })
    .toEqual([accountB.address]);
  expect(
    await provider.request({ method: "wallet_getPermissions" }),
  ).toMatchObject([{ caveats: [{ value: [accountB.address] }] }]);

  await replaceSession(sessionA);
  await expect.poll(() => accounts.at(-1)).toEqual([accountA.address]);
  const delayedA = holdLookup(sessionA.user.id);
  const pendingA = provider.request({ method: "eth_accounts" });
  await delayedA.arrived.promise;
  failures.set(sessionB.user.id, "read");
  const index = accounts.length;
  await replaceSession(sessionB);
  expect(accounts.slice(index)).toEqual([[]]);
  expect(await provider.request({ method: "eth_accounts" })).toEqual([]);
  delayedA.released.resolve();
  expect(await pendingA).toEqual([]);
  expect(
    accounts
      .slice(index)
      .every((value) => Array.isArray(value) && value.length === 0),
  ).toBe(true);
  failures.clear();

  await replaceSession(sessionA);
  await expect.poll(() => accounts.at(-1)).toEqual([accountA.address]);
  const signedOut = holdLookup(sessionA.user.id);
  const oldRequest = provider.request({ method: "eth_accounts" });
  await signedOut.arrived.promise;
  await wallet.signOut();
  expect(accounts.at(-1)).toEqual([]);
  expect(disconnects.at(-1)).toMatchObject({ code: 4900 });
  expect(await provider.request({ method: "eth_accounts" })).toEqual([]);
  // Even re-entry with the exact same principal/token is a new session epoch.
  await replaceSession(sessionA);
  await expect.poll(() => accounts.at(-1)).toEqual([accountA.address]);
  signedOut.released.resolve();
  expect(await oldRequest).toEqual([]);

  const destroyed = holdLookup(sessionA.user.id);
  const destroyedRequest = provider.request({ method: "eth_accounts" });
  await destroyed.arrived.promise;
  const eventCount = accounts.length;
  provider.destroy();
  destroyed.released.resolve();
  expect(await destroyedRequest).toEqual([]);
  expect(await provider.request({ method: "eth_accounts" })).toEqual([]);
  expect(accounts).toHaveLength(eventCount);
  providers.delete(provider);
}
test(embedded_principal_cache, embedded_principal_cache);

async function embedded_quantity_validation() {
  await replaceSession(sessionA);
  const { provider } = await openProvider();
  const tx = {
    from: accountA.address,
    to: accountB.address,
    chainId: "0x7d",
    value: "0x1",
    gas: "0x5208",
    maxFeePerGas: "0x77359400",
    maxPriorityFeePerGas: "0x3b9aca00",
    nonce: "0x0",
  };
  const signingRequests = () =>
    requests.filter(
      (entry) =>
        entry.path === "/v1/wallet/sign" || entry.path === "/v1/wallet/send",
    );
  const invalid = [
    null,
    "",
    " ",
    "invalid",
    "0x",
    "0xz",
    "1.5",
    "-1",
    "+1",
    "01",
    " 1",
    "1 ",
    "1e2",
    "0b10",
    "0o10",
    -1,
    -1n,
    1.5,
    NaN,
    Infinity,
    Number.MAX_SAFE_INTEGER + 1,
    true,
    {},
    [],
    1n << 256n,
    `0x1${"0".repeat(64)}`,
  ];
  const before = signingRequests().length;
  for (const method of ["eth_sendTransaction", "eth_signTransaction"]) {
    for (const field of [
      "value",
      "gas",
      "gasLimit",
      "maxFeePerGas",
      "maxPriorityFeePerGas",
      "nonce",
    ]) {
      for (const value of invalid) {
        await expect(
          provider.request({ method, params: [{ ...tx, [field]: value }] }),
        ).rejects.toMatchObject({ code: -32602 });
      }
    }
    for (const nonce of [
      BigInt(Number.MAX_SAFE_INTEGER) + 1n,
      "9007199254740992",
      "0x20000000000000",
    ]) {
      await expect(
        provider.request({ method, params: [{ ...tx, nonce }] }),
      ).rejects.toMatchObject({ code: -32602 });
    }
    await expect(
      provider.request({ method, params: [{ ...tx, gasLimit: "0x5209" }] }),
    ).rejects.toMatchObject({ code: -32602 });
  }
  expect(signingRequests()).toHaveLength(before);

  const max = (1n << 256n) - 1n;
  for (const method of ["eth_sendTransaction", "eth_signTransaction"]) {
    for (const input of [
      {
        value: 0,
        nonce: 0,
        gas: 21000n,
        maxFeePerGas: 2000000000n,
        maxPriorityFeePerGas: 1000000000,
      },
      {
        value: max,
        nonce: Number.MAX_SAFE_INTEGER,
        gas: max,
        maxFeePerGas: max,
        maxPriorityFeePerGas: max,
      },
      { value: "0x2a", nonce: "0x2a", gas: undefined, gasLimit: "0x5208" },
      { value: undefined, nonce: undefined },
    ]) {
      const result = (await provider.request({
        method,
        params: [{ ...tx, ...input }],
      })) as Hex;
      const signed =
        method === "eth_signTransaction"
          ? result
          : signedTransactions.get(result)!;
      expect(signed).toBeDefined();
      expect(
        await recoverTransactionAddress({
          serializedTransaction: signed as TransactionSerialized,
        }),
      ).toBe(accountA.address);
      // viem omits zero value in its parsed object; RLP encodes it as empty bytes.
      expect(slice(signed, 0, 1)).toBe("0x02");
      const envelope = fromRlp(slice(signed, 1), "hex");
      if (!Array.isArray(envelope))
        throw new Error("expected an EIP-1559 RLP list");
      const encodedValue = envelope[6];
      if (typeof encodedValue !== "string")
        throw new Error("expected a value quantity");
      const expectedValue = BigInt(input.value ?? 0);
      expect(encodedValue).toBe(
        expectedValue === 0n ? "0x" : toHex(expectedValue),
      );
      const decodedValue = encodedValue === "0x" ? 0n : BigInt(encodedValue);
      expect({
        ...parseTransaction(signed),
        value: decodedValue,
      }).toMatchObject({
        chainId: 125,
        value: BigInt(input.value ?? 0),
        nonce: Number(input.nonce ?? 0),
      });
      const wire = signingRequests().at(-1)!.body.tx;
      expect(wire).toEqual({
        to: accountB.address,
        chainId: 125,
        ...(input.value === undefined
          ? {}
          : { value: BigInt(input.value).toString() }),
        ...(input.nonce === undefined ? {} : { nonce: Number(input.nonce) }),
        gas: BigInt(
          input.gas ?? ("gasLimit" in input ? input.gasLimit : tx.gas)!,
        ).toString(),
        maxFeePerGas: BigInt(input.maxFeePerGas ?? tx.maxFeePerGas).toString(),
        maxPriorityFeePerGas: BigInt(
          input.maxPriorityFeePerGas ?? tx.maxPriorityFeePerGas,
        ).toString(),
      });
    }
  }
  expect(signingRequests()).toHaveLength(before + 8);
  provider.destroy();
  providers.delete(provider);
}
test(embedded_quantity_validation, embedded_quantity_validation);

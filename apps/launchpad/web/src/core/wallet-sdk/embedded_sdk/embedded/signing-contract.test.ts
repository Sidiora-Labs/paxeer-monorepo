// @vitest-environment jsdom
import {
  createHmac,
  randomBytes,
  randomUUID,
  timingSafeEqual,
} from "node:crypto";
import { createServer, type Server } from "node:http";
import { afterAll, beforeAll, expect, test } from "vitest";
import {
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  verifyMessage,
  verifyTypedData,
  type Hex,
  type TransactionSerialized,
  type TypedDataDefinition,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import type { Session } from "@supabase/supabase-js";
import {
  createPaxeerEIP1193Provider,
  type PaxeerEIP1193Provider,
} from "./eip1193";
import {
  PaxeerWallet,
  type PublicWallet,
  type TypedDataPayload,
  type TxRequest,
} from "./sdk";

// The production provider and REST SDK talk to committed loopback peers. Each
// response is produced by a real viem signer; no transport or SDK method is replaced.
const fixtureSigningKey = randomBytes(32);
const fixtureSigners = [
  privateKeyToAccount(`0x${"01".repeat(32)}`),
  privateKeyToAccount(`0x${"02".repeat(32)}`),
];
const fixtureUsers = new Map<
  string,
  { signer: (typeof fixtureSigners)[number]; wallet: PublicWallet }
>();
const signatureAudit: { user_id: string; address: Hex; kind: string }[] = [];
const transactions = new Map<Hex, { from: Hex; chainId: Hex; value: Hex }>();
const nonces = new Map<string, number>();
let gateway: Server | undefined;
let gatewayUrl: string;

function mintUserToken(id: string): string {
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const claims = Buffer.from(
    JSON.stringify({
      sub: id,
      aud: "authenticated",
      role: "authenticated",
      iss: `${gatewayUrl}/auth/v1`,
      exp: Math.floor(Date.now() / 1000) + 600,
    }),
  ).toString("base64url");
  const input = `${header}.${claims}`;
  return `${input}.${createHmac("sha256", fixtureSigningKey).update(input).digest("base64url")}`;
}

function authenticatedUser(authorization: string | undefined): string {
  if (!authorization?.startsWith("Bearer "))
    throw new Error("missing bearer token");
  const [header, claims, signature] = authorization.slice(7).split(".");
  const expected = createHmac("sha256", fixtureSigningKey)
    .update(`${header}.${claims}`)
    .digest();
  const actual = Buffer.from(signature, "base64url");
  if (actual.length !== expected.length || !timingSafeEqual(actual, expected))
    throw new Error("invalid bearer signature");
  const payload = JSON.parse(Buffer.from(claims, "base64url").toString());
  if (
    payload.iss !== `${gatewayUrl}/auth/v1` ||
    payload.aud !== "authenticated" ||
    payload.exp <= Date.now() / 1000 ||
    !fixtureUsers.has(payload.sub)
  )
    throw new Error("invalid bearer claims");
  return payload.sub;
}

async function broadcast(signed: Hex): Promise<Hex> {
  const transaction = parseTransaction(signed);
  const from = await recoverTransactionAddress({
    serializedTransaction: signed as TransactionSerialized,
  });
  if (
    transaction.chainId !== 125 ||
    transaction.nonce !== (nonces.get(from) ?? 0)
  )
    throw new Error("transaction chain or nonce mismatch");
  const hash = keccak256(signed);
  transactions.set(hash, {
    from: from.toLowerCase() as Hex,
    chainId: "0x7d",
    value: `0x${(transaction.value ?? 0n).toString(16)}`,
  });
  nonces.set(from, transaction.nonce + 1);
  return hash;
}

async function startGatewayPeer(): Promise<string> {
  gateway = createServer(async (req, res) => {
    const send = (status: number, body: unknown): void => {
      res.writeHead(status, { "content-type": "application/json" });
      res.end(JSON.stringify(body));
    };
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const text = Buffer.concat(chunks).toString();
      if (req.url === "/rpc") {
        const request = JSON.parse(text);
        let result: unknown;
        if (request.method === "eth_sendRawTransaction")
          result = await broadcast(request.params[0]);
        else if (request.method === "eth_getTransactionByHash")
          result = transactions.get(request.params[0]) ?? null;
        else throw new Error(`unexpected RPC method ${request.method}`);
        send(200, { jsonrpc: "2.0", id: request.id, result });
        return;
      }
      const userId = authenticatedUser(req.headers.authorization);
      const { signer, wallet } = fixtureUsers.get(userId)!;
      if (req.method === "GET" && req.url === "/v1/wallet/me") {
        send(200, {
          wallet,
          chain: { id: 125, rpc_url: rpcUrl, explorer_url: null },
        });
        return;
      }
      if (req.method !== "POST")
        throw new Error(`unexpected method ${req.method}`);
      const body = JSON.parse(text);
      let result: unknown;
      let kind: string;
      switch (req.url) {
        case "/v1/wallet/sign-message": {
          if (
            typeof body.message !== "string" ||
            body.message.length < 1 ||
            body.message.length > 10000
          ) {
            send(400, { error: "invalid_body" });
            return;
          }
          result = {
            address: signer.address,
            signature: await signer.signMessage({ message: body.message }),
          };
          kind = "message";
          break;
        }
        case "/v1/wallet/sign-typed-data": {
          const typedData: TypedDataPayload = body.typedData;
          if (
            typedData.domain.chainId !== undefined &&
            BigInt(String(typedData.domain.chainId)) !== 125n
          )
            throw new Error("foreign typed-data chain");
          result = {
            address: signer.address,
            signature: await signer.signTypedData(
              typedData as unknown as TypedDataDefinition,
            ),
          };
          kind = "typed_data";
          break;
        }
        case "/v1/wallet/sign":
        case "/v1/wallet/send": {
          const tx: TxRequest = body.tx;
          if (tx.chainId !== 125) throw new Error("foreign transaction chain");
          const signed = await signer.signTransaction({
            type: "eip1559",
            chainId: tx.chainId,
            to: tx.to,
            data: tx.data,
            value: BigInt(tx.value ?? 0),
            gas: BigInt(tx.gas!),
            maxFeePerGas: BigInt(tx.maxFeePerGas!),
            maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas!),
            nonce: tx.nonce ?? nonces.get(signer.address) ?? 0,
          });
          result =
            req.url === "/v1/wallet/send"
              ? {
                  address: signer.address,
                  chain_id: 125,
                  tx_hash: await broadcast(signed),
                }
              : { address: signer.address, chain_id: 125, signed_tx: signed };
          kind = "transaction";
          break;
        }
        default:
          throw new Error(`unexpected gateway path ${req.url}`);
      }
      signatureAudit.push({ user_id: userId, address: signer.address, kind });
      send(200, result);
    } catch (error) {
      send(500, { error: "fixture_peer_failure", message: String(error) });
    }
  });
  return listen(gateway);
}

interface HeldResponse {
  path: string;
  arrived: Promise<void>;
  release(): void;
  entered(): void;
  released: Promise<void>;
}

let proxy: Server | undefined;
let proxyUrl: string;
let rpcUrl: string;
let sessionA: Session;
let sessionB: Session;
let accountA: PublicWallet;
let accountB: PublicWallet;
let wallet: PaxeerWallet;
let provider: PaxeerEIP1193Provider;
let held: HeldResponse | undefined;
let foreignSigner = false;
let changedMessage = false;
let changedTypedData = false;
const signingRequests: { path: string; body: string }[] = [];
const providers: PaxeerEIP1193Provider[] = [];
const holds = new Set<HeldResponse>();
const wallets: PaxeerWallet[] = [];

function holdResponse(path: string): HeldResponse {
  let entered!: () => void;
  let release!: () => void;
  const arrived = new Promise<void>((resolve) => {
    entered = resolve;
  });
  const released = new Promise<void>((resolve) => {
    release = resolve;
  });
  const pending: HeldResponse = {
    path,
    arrived,
    released,
    entered,
    release: () => {
      release();
      holds.delete(pending);
    },
  };
  holds.add(pending);
  held = pending;
  return pending;
}

async function listen(server: Server): Promise<string> {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string")
    throw new Error("loopback server did not bind");
  return `http://127.0.0.1:${address.port}`;
}

async function rpc(method: string, params: unknown[]): Promise<unknown> {
  const response = await fetch(rpcUrl, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const result = await response.json();
  if (result.error) throw new Error(JSON.stringify(result.error));
  return result.result;
}

async function browserSession(id: string): Promise<Session> {
  const access_token = mintUserToken(id);
  const claims = JSON.parse(
    Buffer.from(access_token.split(".")[1], "base64url").toString(),
  );
  return {
    access_token,
    refresh_token: randomBytes(32).toString("hex"),
    token_type: "bearer",
    expires_in: 600,
    expires_at: claims.exp,
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

function storeSession(session: Session): void {
  localStorage.setItem("sb-127-auth-token", JSON.stringify(session));
}

async function openProvider(): Promise<{
  wallet: PaxeerWallet;
  provider: PaxeerEIP1193Provider;
}> {
  storeSession(sessionA);
  const wallet = new PaxeerWallet({
    apiUrl: proxyUrl,
    supabaseUrl: gatewayUrl,
    supabaseAnonKey: "embedded-repair-public-key",
  });
  wallets.push(wallet);
  await wallet.getSession();
  const provider = createPaxeerEIP1193Provider({
    wallet,
    chainId: 125,
    rpcUrl,
  });
  providers.push(provider);
  // Wait for the actual Supabase INITIAL_SESSION notification and its wallet lookup.
  await new Promise<void>((resolve) => {
    let unsubscribe: (() => void) | undefined;
    unsubscribe = wallet.onAuthStateChange((event) => {
      if (event === "INITIAL_SESSION") {
        queueMicrotask(() => unsubscribe?.());
        resolve();
      }
    });
  });
  expect(await provider.request({ method: "eth_requestAccounts" })).toEqual([
    accountA.address,
  ]);
  return { wallet, provider };
}

beforeAll(async () => {
  gatewayUrl = await startGatewayPeer();
  rpcUrl = `${gatewayUrl}/rpc`;
  sessionA = await browserSession(randomUUID());
  sessionB = await browserSession(randomUUID());
  for (const [index, session] of [sessionA, sessionB].entries()) {
    const signer = fixtureSigners[index];
    const publicWallet: PublicWallet = {
      id: randomUUID(),
      address: signer.address,
      chain_id: 125,
      created_at: new Date().toISOString(),
      last_used_at: null,
    };
    fixtureUsers.set(session.user.id, { signer, wallet: publicWallet });
    if (session === sessionA) accountA = publicWallet;
    else accountB = publicWallet;
  }
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      let body = Buffer.concat(chunks).toString();
      const path = req.url!;
      const signing = req.method === "POST" && path !== "/v1/wallet/provision";
      if (signing) signingRequests.push({ path, body });
      // Faults still obtain their entire response from the cryptographic producer.
      if (changedMessage && path === "/v1/wallet/sign-message")
        body = JSON.stringify({ message: "different content" });
      if (changedTypedData && path === "/v1/wallet/sign-typed-data") {
        const document = JSON.parse(body);
        document.typedData.message.contents = "different content";
        body = JSON.stringify(document);
      }
      const response = await fetch(`${gatewayUrl}${path}`, {
        method: req.method,
        headers: {
          authorization:
            foreignSigner && signing
              ? `Bearer ${sessionB.access_token}`
              : String(req.headers.authorization),
          ...(body ? { "content-type": "application/json" } : {}),
        },
        ...(body ? { body } : {}),
      });
      const payload = await response.text();
      if (held?.path === path) {
        const pending = held;
        held = undefined;
        pending.entered();
        await pending.released;
      }
      res.writeHead(response.status, { "content-type": "application/json" });
      res.end(payload);
    } catch (error) {
      res.writeHead(502);
      res.end(String(error));
    }
  });
  proxyUrl = await listen(proxy);
  ({ wallet, provider } = await openProvider());
}, 120_000);

afterAll(async () => {
  for (const pending of holds) pending.release();
  for (const provider of providers) provider.destroy();
  for (const wallet of wallets) wallet.supabase.auth.stopAutoRefresh();
  if (proxy)
    await new Promise<void>((resolve, reject) =>
      proxy!.close((error) => (error ? reject(error) : resolve())),
    );
  if (gateway)
    await new Promise<void>((resolve, reject) =>
      gateway!.close((error) => (error ? reject(error) : resolve())),
    );
  localStorage.clear();
}, 120_000);

const recipient: Hex = "0x00000000000000000000000000000000000000e1";
const tx = {
  from: "",
  to: recipient,
  chainId: "0x7d",
  value: "0x1",
  gas: "0x5208",
  maxFeePerGas: "0x77359400",
  maxPriorityFeePerGas: "0x3b9aca00",
};
const typedData = {
  domain: {
    name: "Embedded repair",
    version: "1",
    chainId: 125,
    verifyingContract: recipient,
  },
  types: {
    EIP712Domain: [
      { name: "name", type: "string" },
      { name: "version", type: "string" },
      { name: "chainId", type: "uint256" },
      { name: "verifyingContract", type: "address" },
    ],
    Note: [
      { name: "contents", type: "string" },
      { name: "amount", type: "uint256" },
    ],
  } as const,
  primaryType: "Note" as const,
  message: { contents: "Preserve this typed message", amount: "42" },
};

async function embedded_signer_scope() {
  const before = signingRequests.length;
  for (const method of ["eth_sendTransaction", "eth_signTransaction"]) {
    await expect(
      provider.request({ method, params: [{ ...tx, from: accountB.address }] }),
    ).rejects.toMatchObject({ code: 4100 });
    for (const chainId of [126, "0x7e", "invalid"]) {
      await expect(
        provider.request({
          method,
          params: [{ ...tx, from: accountA.address, chainId }],
        }),
      ).rejects.toHaveProperty("code");
    }
  }
  await expect(
    provider.request({
      method: "personal_sign",
      params: ["hello", accountB.address],
    }),
  ).rejects.toMatchObject({ code: 4100 });
  await expect(
    provider.request({
      method: "eth_signTypedData_v4",
      params: [accountB.address, typedData],
    }),
  ).rejects.toMatchObject({ code: 4100 });
  expect(signingRequests).toHaveLength(before);
  const signed = (await provider.request({
    method: "eth_signTransaction",
    params: [{ ...tx, from: accountA.address }],
  })) as Hex;
  expect(
    await recoverTransactionAddress({
      serializedTransaction: signed as TransactionSerialized,
    }),
  ).toBe(accountA.address);
  expect(parseTransaction(signed)).toMatchObject({
    chainId: 125,
    value: 1n,
    gas: 21000n,
  });
  await rpc("eth_sendRawTransaction", [signed]);
  const hash = await provider.request({
    method: "eth_sendTransaction",
    params: [{ ...tx, from: accountA.address }],
  });
  const sent = await rpc("eth_getTransactionByHash", [hash]);
  expect(sent).toMatchObject({
    from: accountA.address.toLowerCase(),
    chainId: "0x7d",
    value: "0x1",
  });
}
test(embedded_signer_scope, embedded_signer_scope);

async function embedded_message_encoding() {
  const before = signingRequests.length;
  for (const message of [
    "0x1",
    "0x123",
    "0xff",
    "0xc328",
    "0xc080",
    "0xed a0",
    "0xzz",
    42,
    "\ud800",
  ]) {
    await expect(
      provider.request({
        method: "personal_sign",
        params: [message, accountA.address],
      }),
    ).rejects.toMatchObject({ code: -32602 });
  }
  expect(signingRequests).toHaveLength(before);
  for (const message of [
    "Hello 🌍\u0000é",
    "\ufeffKeep the BOM",
    "literal text",
  ]) {
    const encoded = `0x${Buffer.from(message).toString("hex")}`;
    const signature = (await provider.request({
      method: "personal_sign",
      params: [encoded, accountA.address],
    })) as Hex;
    expect(
      await verifyMessage({
        address: accountA.address,
        message: { raw: encoded as Hex },
        signature,
      }),
    ).toBe(true);
    expect(JSON.parse(signingRequests.at(-1)!.body)).toEqual({ message });
  }
  for (const empty of ["", "0x"]) {
    await expect(
      provider.request({
        method: "personal_sign",
        params: [empty, accountA.address],
      }),
    ).rejects.toMatchObject({ code: "invalid_body" });
    expect(JSON.parse(signingRequests.at(-1)!.body)).toEqual({ message: "" });
  }
  const literal = "Plain UTF-8 🌍";
  const signature = (await provider.request({
    method: "personal_sign",
    params: [literal, accountA.address],
  })) as Hex;
  expect(
    await verifyMessage({
      address: accountA.address,
      message: literal,
      signature,
    }),
  ).toBe(true);
}
test(embedded_message_encoding, embedded_message_encoding);

async function embedded_eth_sign_contract() {
  const before = signingRequests.length;
  const construction = {
    kind: "eip7702_authorization",
    chainId: "125",
    address: recipient,
    nonce: "0",
  };
  for (const params of [
    [accountA.address, "0x1"],
    [accountA.address, `0x${"01".repeat(32)}`],
    [accountA.address, `0x${"01".repeat(32)}`, construction],
    [accountB.address, "hello"],
  ]) {
    await expect(
      provider.request({ method: "eth_sign", params }),
    ).rejects.toMatchObject({ code: 4200 });
  }
  expect(signingRequests).toHaveLength(before);
}
test(embedded_eth_sign_contract, embedded_eth_sign_contract);

async function embedded_typed_data() {
  const before = signingRequests.length;
  for (const domain of [
    { ...typedData.domain, chainId: 126 },
    { ...typedData.domain, chainId: "0x7e" },
  ]) {
    await expect(
      provider.request({
        method: "eth_signTypedData_v4",
        params: [accountA.address, { ...typedData, domain }],
      }),
    ).rejects.toMatchObject({ code: 4901 });
  }
  for (const raw of [
    "{",
    {},
    { ...typedData, types: { Note: [{ type: "string" }] } },
    { ...typedData, message: {} },
  ]) {
    await expect(
      provider.request({
        method: "eth_signTypedData_v4",
        params: [accountA.address, raw],
      }),
    ).rejects.toMatchObject({ code: -32602 });
  }
  expect(signingRequests).toHaveLength(before);
  for (const raw of [typedData, JSON.stringify(typedData)]) {
    const signature = (await provider.request({
      method: "eth_signTypedData_v4",
      params: [accountA.address, raw],
    })) as Hex;
    expect(
      await verifyTypedData({
        ...typedData,
        domain: {
          ...typedData.domain,
          chainId: BigInt(typedData.domain.chainId),
        },
        message: {
          ...typedData.message,
          amount: BigInt(typedData.message.amount),
        },
        address: accountA.address,
        signature,
      }),
    ).toBe(true);
    expect(signingRequests.at(-1)?.path).toBe("/v1/wallet/sign-typed-data");
    expect(JSON.parse(signingRequests.at(-1)!.body)).toEqual({ typedData });
  }
  for (const method of ["eth_signTypedData", "eth_signTypedData_v3"]) {
    await expect(
      provider.request({ method, params: [accountA.address, typedData] }),
    ).rejects.toMatchObject({ code: 4200 });
  }
  const audit = {
    rows: signatureAudit.filter(
      (entry) =>
        entry.user_id === sessionA.user.id && entry.kind === "typed_data",
    ),
  };
  expect(audit.rows).toHaveLength(2);
  expect(
    audit.rows.every(
      (row) => row.address === accountA.address && row.kind === "typed_data",
    ),
  ).toBe(true);
}
test(embedded_typed_data, embedded_typed_data);

async function embedded_gateway_identity_and_signature() {
  foreignSigner = true;
  try {
    for (const method of ["eth_sendTransaction", "eth_signTransaction"]) {
      await expect(
        provider.request({
          method,
          params: [{ ...tx, from: accountA.address }],
        }),
      ).rejects.toMatchObject({ code: 4100 });
    }
    await expect(
      provider.request({
        method: "personal_sign",
        params: ["hello", accountA.address],
      }),
    ).rejects.toMatchObject({ code: 4100 });
    await expect(
      provider.request({
        method: "eth_signTypedData_v4",
        params: [accountA.address, typedData],
      }),
    ).rejects.toMatchObject({ code: 4100 });
  } finally {
    foreignSigner = false;
  }
  changedMessage = true;
  try {
    await expect(
      provider.request({
        method: "personal_sign",
        params: ["original content", accountA.address],
      }),
    ).rejects.toMatchObject({ code: 4100 });
  } finally {
    changedMessage = false;
  }
  changedTypedData = true;
  try {
    await expect(
      provider.request({
        method: "eth_signTypedData_v4",
        params: [accountA.address, typedData],
      }),
    ).rejects.toMatchObject({ code: 4100 });
  } finally {
    changedTypedData = false;
  }
}
test(
  embedded_gateway_identity_and_signature,
  embedded_gateway_identity_and_signature,
);

async function embedded_principal_epoch() {
  const response = holdResponse("/v1/wallet/sign-message");
  const pending = provider.request({
    method: "personal_sign",
    params: ["in flight", accountA.address],
  });
  const rejected = expect(pending).rejects.toMatchObject({
    code: "SESSION_CHANGED",
  });
  await response.arrived;
  storeSession(sessionB);
  response.release();
  await rejected;
  storeSession(sessionA);

  const second = await openProvider();
  const lookup = holdResponse("/v1/wallet/me");
  const before = signingRequests.length;
  const signing = second.provider.request({
    method: "personal_sign",
    params: ["must not route", accountA.address],
  });
  const refused = expect(signing).rejects.toMatchObject({ code: 4100 });
  await lookup.arrived;
  second.provider.destroy();
  lookup.release();
  await refused;
  expect(signingRequests).toHaveLength(before);
}
test(embedded_principal_epoch, embedded_principal_epoch);

async function embedded_request_snapshot() {
  const lookup = holdResponse("/v1/wallet/me");
  const data = structuredClone(typedData);
  const signing = provider.request({
    method: "eth_signTypedData_v4",
    params: [accountA.address, data],
  });
  await lookup.arrived;
  data.domain.chainId = 126;
  data.message.contents = "mutated during lookup";
  lookup.release();
  const signature = (await signing) as Hex;
  expect(
    await verifyTypedData({
      ...typedData,
      domain: {
        ...typedData.domain,
        chainId: BigInt(typedData.domain.chainId),
      },
      message: {
        ...typedData.message,
        amount: BigInt(typedData.message.amount),
      },
      address: accountA.address,
      signature,
    }),
  ).toBe(true);
  expect(JSON.parse(signingRequests.at(-1)!.body)).toEqual({ typedData });
}
test(embedded_request_snapshot, embedded_request_snapshot);

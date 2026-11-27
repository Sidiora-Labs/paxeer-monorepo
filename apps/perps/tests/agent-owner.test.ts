import assert from "node:assert/strict";
import { test, type TestContext } from "node:test";
import { isMainThread, parentPort, Worker } from "node:worker_threads";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import {
  createHash,
  createPublicKey,
  randomBytes,
  randomUUID,
  verify,
} from "node:crypto";
import { readFile } from "node:fs/promises";
import {
  AgentOwnerClient,
  browserAgentClaimDigest,
  browserAgentRequestDigest,
  agentHex,
  checkedAgentAmount,
  parseAgentClaim,
  prepareAgentClaim,
  type AgentPolicy,
  type AgentRule,
  type AgentBudget,
  type AgentDetail,
} from "../packages/paxeer-backend/src/core/wallet/embedded/sdk/agents";
import {
  createPublicClient,
  decodeFunctionData,
  encodeFunctionResult,
  http,
  toHex,
  type Address,
  type Hex,
} from "viem";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";

type RecordRequest = {
  method: string;
  path: string;
  body: unknown;
  authorization: string | undefined;
};
interface Principal {
  did: string;
  label: string;
  publicKey: string;
  owner: string | null;
  frozen: boolean;
  address: Address | null;
  bound: boolean;
  policy: AgentPolicy;
  rules: AgentRule[];
  budgets: AgentBudget[];
  events: {
    kind: string;
    to_address: string;
    value_wei: string;
    tx_hash: Hex;
    created_at: string;
  }[];
}
const defaultPolicy: AgentPolicy = {
  mode: "read_only",
  max_tx_value_wei: "0",
  max_daily_value_wei: "0",
  max_approve_wei: "0",
  rate_limit_per_min: 60,
  allow_native_transfer: false,
  withdrawal_allowlist_only: true,
  daily_reset_utc_hour: 0,
};
function u32(n: number) {
  const b = Buffer.alloc(4);
  b.writeUInt32BE(n);
  return b;
}
function u64(n: bigint) {
  const b = Buffer.alloc(8);
  b.writeBigUInt64BE(n);
  return b;
}
function prefixed(value: string) {
  const b = Buffer.from(value);
  return Buffer.concat([u32(b.length), b]);
}
function hash(...bytes: Uint8Array[]) {
  return createHash("sha256").update(Buffer.concat(bytes)).digest();
}
function requestDigest(
  method: string,
  did: string,
  nonce: Buffer,
  expiry: bigint,
  body: string,
) {
  return hash(
    Buffer.from("PXW:AGENT-REQUEST:v1"),
    prefixed(method),
    prefixed(did),
    nonce,
    u64(expiry),
    hash(Buffer.from(body)),
  );
}
function claimDigest(did: string, owner: string, expiry: bigint) {
  return hash(
    Buffer.from("PXW:AGENT-CLAIM:v1"),
    prefixed(did),
    prefixed(owner),
    u64(expiry),
  );
}
function verifies(publicKey: string, message: Uint8Array, signature: string) {
  return verify(
    null,
    message,
    createPublicKey({
      key: Buffer.concat([
        Buffer.from("302a300506032b6570032100", "hex"),
        Buffer.from(publicKey, "hex"),
      ]),
      format: "der",
      type: "spki",
    }),
    Buffer.from(signature, "hex"),
  );
}

async function agentGateway() {
  const owner = privateKeyToAccount(generatePrivateKey()).address;
  const userId = randomUUID();
  const ownerToken = randomBytes(24).toString("base64url");
  const principals = new Map<string, Principal>();
  const challenges = new Map<string, string>();
  const balances = new Map<string, { native: bigint; collateral: bigint }>([
    [
      owner.toLowerCase(),
      { native: 100n * 10n ** 18n, collateral: 10_000n * 10n ** 6n },
    ],
  ]);
  const requests: RecordRequest[] = [];
  const violations: unknown[] = [];
  const nonces = new Set<string>();
  const failures: { path: string; message: string }[] = [];
  const heldReads = new Map<
    string,
    { promise: Promise<void>; release: () => void }
  >();
  let nextId = 1;
  let failProvision = false;
  let holdList: Promise<void> | null = null;
  let releaseList: (() => void) | null = null;
  const receipts = new Map<string, { from: Address; to: Address }>();
  let erc20Abi: typeof import("../packages/paxeer-backend/src/core/perps/config").erc20Abi;
  let token: Address;
  function owned(p: Principal) {
    return {
      did: p.did,
      label: p.label,
      address: p.address,
      mode: p.policy.mode,
      is_frozen: p.frozen,
      created_at: new Date(0).toISOString(),
      last_seen_at: null,
    };
  }
  function detail(p: Principal): AgentDetail {
    return {
      did: p.did,
      label: p.label,
      owner_user_id: p.owner!,
      is_frozen: p.frozen,
      wallet: p.address
        ? {
            id: p.did,
            address: p.address,
            native_wei: String(balances.get(p.address.toLowerCase())!.native),
          }
        : null,
      policy: p.policy,
      rules: p.rules,
      budgets: p.budgets,
    };
  }
  function send(reply: ServerResponse, body: unknown, status = 200) {
    reply.writeHead(status, { "content-type": "application/json" });
    reply.end(JSON.stringify(body));
  }
  const server = createServer((req, reply) => {
    void handle(req, reply).catch((error) => {
      violations.push(error);
      send(reply, { error: "invalid_request", message: String(error) }, 500);
    });
  });
  async function handle(req: IncomingMessage, reply: ServerResponse) {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const raw = Buffer.concat(chunks).toString();
    const body = raw ? JSON.parse(raw) : undefined;
    const path = new URL(req.url!, "http://localhost").pathname;
    if (path === "/rpc") {
      const { id, method, params } = body;
      let result: unknown;
      if (method === "eth_getBalance")
        result = toHex(balances.get(params[0].toLowerCase())!.native);
      else if (method === "eth_call") {
        const decoded = decodeFunctionData({
          abi: erc20Abi,
          data: params[0].data,
        });
        assert.equal(params[0].to.toLowerCase(), token.toLowerCase());
        assert.equal(decoded.functionName, "balanceOf");
        const address = decoded.args![0] as string;
        result = encodeFunctionResult({
          abi: erc20Abi,
          functionName: "balanceOf",
          result: balances.get(address.toLowerCase())!.collateral,
        });
      } else if (method === "eth_getTransactionReceipt") {
        const receipt = receipts.get(params[0]);
        result = receipt
          ? {
              transactionHash: params[0],
              transactionIndex: "0x0",
              blockHash: toHex(1n, { size: 32 }),
              blockNumber: "0x1",
              from: receipt.from,
              to: receipt.to,
              cumulativeGasUsed: "0x5208",
              gasUsed: "0x5208",
              contractAddress: null,
              logs: [],
              logsBloom: `0x${"00".repeat(256)}`,
              status: "0x1",
              effectiveGasPrice: "0x1",
              type: "0x2",
            }
          : null;
      } else if (method === "eth_blockNumber") result = "0x1";
      else throw new Error(`Unexpected RPC ${method}`);
      send(reply, { jsonrpc: "2.0", id, result });
      return;
    }
    requests.push({
      method: req.method!,
      path,
      body,
      authorization: req.headers.authorization,
    });
    if (req.method === "GET") await heldReads.get(path)?.promise;
    const failureIndex = failures.findIndex((f) => f.path === path);
    if (failureIndex >= 0) {
      const failure = failures.splice(failureIndex, 1)[0];
      send(reply, { error: "unavailable", message: failure.message }, 503);
      return;
    }
    if (path === "/v1/agent/auth/challenge") {
      assert.equal(req.method, "POST");
      assert.deepEqual(Object.keys(body), ["did"]);
      assert.equal(req.headers.authorization, undefined);
      const nonce = randomBytes(24).toString("base64url");
      challenges.set(body.did, nonce);
      send(reply, {
        did: body.did,
        nonce,
        expires_in: 60,
        message: `paxeer-agent-auth:${body.did}:${nonce}`,
      });
      return;
    }
    if (path === "/v1/agent/auth/verify") {
      assert.equal(req.method, "POST");
      assert.equal(req.headers.authorization, undefined);
      assert.deepEqual(Object.keys(body).sort(), [
        "did",
        "nonce",
        "public_key",
        "signature",
      ]);
      assert.equal(challenges.get(body.did), body.nonce);
      challenges.delete(body.did);
      assert.equal(body.did.split(":").at(-1), body.public_key.slice(0, 16));
      assert.ok(
        verifies(
          body.public_key,
          Buffer.from(`paxeer-agent-auth:${body.did}:${body.nonce}`),
          body.signature,
        ),
      );
      const p: Principal = {
        did: body.did,
        label: body.did.split(":")[2],
        publicKey: body.public_key,
        owner: null,
        frozen: true,
        address: null,
        bound: false,
        policy: { ...defaultPolicy },
        rules: [],
        budgets: [],
        events: [],
      };
      principals.set(p.did, p);
      send(reply, {
        agent_token: randomBytes(24).toString("base64url"),
        owner_user_id: null,
        is_frozen: true,
        wallet_provisioned: false,
      });
      return;
    }
    if (path === "/v1/agent/provision") {
      assert.equal(req.method, "POST");
      assert.equal(req.headers.authorization, undefined);
      const did = req.headers["x-agent-key"] as string;
      const p = principals.get(did)!;
      assert.equal(p.frozen, false);
      const nonce = req.headers["x-agent-nonce"] as string;
      assert.match(nonce, /^[0-9a-f]{32}$/);
      assert.ok(!nonces.has(nonce));
      nonces.add(nonce);
      const expires = BigInt(req.headers["x-agent-expires"] as string);
      assert.ok(expires > BigInt(Math.floor(Date.now() / 1000)));
      assert.ok(
        verifies(
          p.publicKey,
          requestDigest(
            "POST /v1/agent/provision",
            did,
            Buffer.from(nonce, "hex"),
            expires,
            raw,
          ),
          req.headers["x-agent-signature"] as string,
        ),
      );
      if (failProvision) {
        send(
          reply,
          {
            error: "provisioning_unavailable",
            message: "Agent provisioning is unavailable",
          },
          503,
        );
        return;
      }
      if (!p.address) {
        p.address = privateKeyToAccount(generatePrivateKey()).address;
        balances.set(p.address.toLowerCase(), { native: 0n, collateral: 0n });
      }
      const chain = Buffer.alloc(32);
      chain.writeBigUInt64BE(125n, 24);
      const bind = Buffer.concat([
        Buffer.from("LX:PAXEER-BIND:v1"),
        chain,
        Buffer.from(p.address.slice(2), "hex"),
        u64(0n),
      ]);
      if (body.bind_signature) {
        assert.deepEqual(Object.keys(body), ["bind_signature"]);
        assert.ok(verifies(p.publicKey, bind, body.bind_signature));
        p.bound = true;
      } else assert.deepEqual(body, {});
      send(
        reply,
        {
          wallet: {
            id: p.did,
            address: p.address,
            chain_id: 125,
            kind: "agent",
          },
          provision: {
            state: p.bound ? "active" : "identity",
            awaiting: p.bound ? null : "agent_signature",
            bindMessage: p.bound ? null : `0x${bind.toString("hex")}`,
          },
        },
        p.bound ? 200 : 202,
      );
      return;
    }
    assert.equal(req.headers.authorization, `Bearer ${ownerToken}`);
    if (path === "/v1/agents") {
      assert.equal(req.method, "GET");
      assert.equal(body, undefined);
      if (holdList) await holdList;
      send(reply, {
        agents: Array.from(principals.values())
          .filter((p) => p.owner === userId)
          .map(owned),
      });
      return;
    }
    const parts = path.split("/");
    const did = decodeURIComponent(parts[3]);
    const p = principals.get(did)!;
    assert.ok(p);
    const action = parts[4];
    if (action === "claim") {
      assert.equal(req.method, "POST");
      assert.deepEqual(Object.keys(body).sort(), ["expires", "signature"]);
      assert.ok(
        verifies(
          p.publicKey,
          claimDigest(did, userId, BigInt(body.expires)),
          body.signature,
        ),
      );
      assert.equal(p.owner, null);
      p.owner = userId;
      send(reply, { did, owner_user_id: userId });
      return;
    }
    assert.equal(p.owner, userId);
    if (!action) {
      assert.equal(req.method, "GET");
      send(reply, detail(p));
      return;
    }
    if (action === "freeze" || action === "unfreeze") {
      assert.equal(req.method, "POST");
      assert.equal(body, undefined);
      p.frozen = action === "freeze";
      send(reply, { did, is_frozen: p.frozen });
      return;
    }
    if (action === "policy") {
      assert.equal(req.method, "PUT");
      assert.equal(p.frozen, true);
      p.policy = body;
      send(reply, { did, policy: p.policy });
      return;
    }
    if (action === "rules") {
      assert.equal(p.frozen, true);
      if (req.method === "POST") {
        const rule = { id: String(nextId++), ...body };
        p.rules.push(rule);
        send(reply, { rule }, 201);
      } else {
        assert.equal(req.method, "DELETE");
        assert.equal(body, undefined);
        assert.ok(p.rules.some((r) => r.id === parts[5]));
        p.rules = p.rules.filter((r) => r.id !== parts[5]);
        send(reply, { deleted: true });
      }
      return;
    }
    if (action === "budgets") {
      assert.equal(p.frozen, true);
      if (req.method === "POST") {
        assert.ok(
          body.expires_in_seconds > 0 && body.expires_in_seconds <= 31_536_000,
        );
        const budget = {
          id: String(nextId++),
          target_contract: body.target_contract,
          token: body.token,
          cap_wei: body.cap_wei,
          spent_wei: "0",
          remaining_wei: body.cap_wei,
          expires_at: new Date(
            Date.now() + body.expires_in_seconds * 1000,
          ).toISOString(),
        };
        p.budgets.push(budget);
        send(reply, { budget }, 201);
      } else {
        assert.equal(req.method, "DELETE");
        assert.equal(body, undefined);
        assert.ok(p.budgets.some((b) => b.id === parts[5]));
        p.budgets = p.budgets.filter((b) => b.id !== parts[5]);
        send(reply, { deactivated: true });
      }
      return;
    }
    if (action === "fund" || action === "sweep") {
      assert.equal(req.method, "POST");
      assert.deepEqual(Object.keys(body).sort(), ["amount", "token"]);
      assert.match(body.amount, /^[1-9][0-9]*$/);
      assert.ok(p.bound);
      assert.ok(body.token === null || body.token === token);
      const from = action === "fund" ? owner : p.address!;
      const to = action === "fund" ? p.address! : owner;
      const field = body.token === null ? "native" : "collateral";
      const amount = BigInt(body.amount);
      assert.ok(balances.get(from.toLowerCase())![field] >= amount);
      balances.get(from.toLowerCase())![field] -= amount;
      balances.get(to.toLowerCase())![field] += amount;
      const tx = `0x${randomBytes(32).toString("hex")}` as Hex;
      receipts.set(tx, { from, to });
      p.events.unshift({
        kind: "transaction",
        to_address: to,
        value_wei: field === "native" ? body.amount : "0",
        tx_hash: tx,
        created_at: new Date().toISOString(),
      });
      send(reply, { tx_hash: tx, from, to });
      return;
    }
    if (action === "activity") {
      assert.equal(req.method, "GET");
      send(reply, { did, events: p.events });
      return;
    }
    throw new Error(`Unexpected route ${path}`);
  }
  await new Promise<void>((resolve) => server.listen(0, resolve));
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  const origin = `http://localhost:${address.port}`;
  Object.assign(process.env, {
    NEXT_PUBLIC_PAXEER_RPC_URLS: `${origin}/rpc`,
    NEXT_PUBLIC_PERPS_GATEWAY_URL: origin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: origin,
    NEXT_PUBLIC_PAXSCAN_API_URL: origin,
    NEXT_PUBLIC_PAXSCAN_WS_URL: origin.replace("http:", "ws:"),
  });
  const config =
    await import("../packages/paxeer-backend/src/core/perps/config");
  erc20Abi = config.erc20Abi;
  token = config.COLLATERAL_TOKENS.usdl.address;
  const rpc = createPublicClient({
    transport: http(`${origin}/rpc`, { retryCount: 0 }),
  });
  const client = new AgentOwnerClient({
    apiUrl: origin,
    getSession: async () => ({
      access_token: ownerToken,
      user: { id: userId },
    }),
  });
  return {
    client,
    owner,
    userId,
    principals,
    balances,
    requests,
    violations,
    failures,
    origin,
    rpc,
    token,
    holdRead: (path: string) => {
      assert.ok(!heldReads.has(path));
      let release!: () => void;
      const promise = new Promise<void>((resolve) => {
        release = resolve;
      });
      heldReads.set(path, { promise, release });
      return () => {
        heldReads.delete(path);
        release();
      };
    },
    failProvision: () => {
      failProvision = true;
    },
    holdInventory: () => {
      holdList = new Promise<void>((resolve) => {
        releaseList = resolve;
      });
    },
    releaseInventory: () => {
      holdList = null;
      releaseList!();
    },
    close: async () => {
      releaseList?.();
      for (const held of heldReads.values()) held.release();
      heldReads.clear();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}

async function agentOwnerRequestAndAmountValidation() {
  const did = `did:matrix:trader:${randomBytes(8).toString("hex")}`;
  const owner = randomUUID();
  const nonce = randomBytes(16);
  const expiry = 1000n;
  assert.equal(
    agentHex(await browserAgentClaimDigest(did, owner, expiry)),
    claimDigest(did, owner, expiry).toString("hex"),
  );
  assert.equal(
    agentHex(
      await browserAgentRequestDigest({
        method: "POST /v1/agent/provision",
        keyId: did,
        nonce,
        expiry,
        body: "{}",
      }),
    ),
    requestDigest(
      "POST /v1/agent/provision",
      did,
      nonce,
      expiry,
      "{}",
    ).toString("hex"),
  );
  assert.notDeepEqual(
    await browserAgentClaimDigest(did, owner, expiry),
    await browserAgentClaimDigest(did, `${owner}x`, expiry),
  );
  assert.equal(checkedAgentAmount("1.000001", 6, 2_000_000n), "1000001");
  assert.equal(checkedAgentAmount("0.000000000000000001", 18, 1n), "1");
  for (const value of [
    "",
    "0",
    "-1",
    "1e3",
    "NaN",
    "1.0000001",
    "2.1",
    "1.",
    "0x10",
  ])
    assert.throws(() => checkedAgentAmount(value, 6, 2_000_000n));
  const request = await prepareAgentClaim(did, owner);
  const signature = randomBytes(64).toString("hex");
  assert.deepEqual(
    parseAgentClaim(
      JSON.stringify({ expires: request.expires, signature }),
      request,
    ),
    { expires: request.expires, signature },
  );
  assert.throws(() =>
    parseAgentClaim(
      JSON.stringify({
        expires: request.expires,
        signature,
        private_key: "not accepted",
      }),
      request,
    ),
  );
  assert.throws(() =>
    parseAgentClaim(JSON.stringify({ expires: "0", signature }), request),
  );
  assert.throws(
    () =>
      parseAgentClaim(JSON.stringify({ expires: "0", signature }), {
        ...request,
        expires: "0",
      }),
    /expired/,
  );
  assert.throws(
    () => new AgentOwnerClient({ apiUrl: "", getSession: async () => null }),
    /not configured/,
  );
  const unauthed = new AgentOwnerClient({
    apiUrl: "http://localhost",
    getSession: async () => null,
  });
  await assert.rejects(unauthed.list(), /Sign in/);
}

async function agentOwnerCreationFreezesOnProvisionFailure() {
  const gateway = await agentGateway();
  try {
    gateway.failProvision();
    const disclosures = [];
    await assert.rejects(
      gateway.client.create("recovery", gateway.userId, (key) => {
        disclosures.push(key);
      }),
      /provisioning is unavailable/,
    );
    assert.equal(disclosures.length, 1);
    assert.equal(gateway.principals.size, 1);
    const agent = Array.from(gateway.principals.values())[0];
    assert.equal(agent.owner, gateway.userId);
    assert.equal(agent.frozen, true);
    assert.equal(
      gateway.requests.at(-1)!.path,
      `/v1/agents/${encodeURIComponent(agent.did)}/freeze`,
    );
    assert.deepEqual(gateway.violations, []);
  } finally {
    await gateway.close();
  }
}

async function exerciseAgentOwnerWorkspace() {
  const gateway = await agentGateway();
  const { Window } = await import("happy-dom");
  const dom = new Window();
  let cleanup: (() => void) | undefined;
  try {
    for (const [name, value] of Object.entries({
      window: dom,
      document: dom.document,
      navigator: dom.navigator,
      localStorage: dom.localStorage,
      sessionStorage: dom.sessionStorage,
      HTMLElement: dom.HTMLElement,
      HTMLInputElement: dom.HTMLInputElement,
      HTMLTextAreaElement: dom.HTMLTextAreaElement,
      Element: dom.Element,
      Node: dom.Node,
      Event: dom.Event,
      CustomEvent: dom.CustomEvent,
      MutationObserver: dom.MutationObserver,
      ResizeObserver: dom.ResizeObserver,
      getComputedStyle: dom.getComputedStyle.bind(dom),
      requestAnimationFrame: dom.requestAnimationFrame.bind(dom),
      cancelAnimationFrame: dom.cancelAnimationFrame.bind(dom),
    }))
      Object.defineProperty(globalThis, name, {
        value,
        configurable: true,
        writable: true,
      });
    Object.assign(process.env, {
      NEXT_PUBLIC_PAXEER_RPC_URLS: `${gateway.origin}/rpc`,
      NEXT_PUBLIC_PERPS_GATEWAY_URL: gateway.origin,
      NEXT_PUBLIC_PAXSCAN_BASE_URL: gateway.origin,
      NEXT_PUBLIC_PAXSCAN_API_URL: gateway.origin,
      NEXT_PUBLIC_PAXSCAN_WS_URL: gateway.origin.replace("http:", "ws:"),
      NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY: "",
    });
    const React = await import("react");
    Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
    const { createElement } = React;
    const testing = await import("@testing-library/react");
    cleanup = testing.cleanup;
    const { render, fireEvent, waitFor, act } = testing;
    const { AgentsAccess, AgentsWorkspace } =
      await import("../packages/portfolio/src/pages/agents/agents.ui");
    const {
      readAgentWalletBalances,
      buildPerpsAgentPreset,
      PERPS_AGENT_FUNCTIONS,
      PERPS_AGENT_APPROVAL,
      savePerpsAgentPreset,
    } =
      await import("../packages/paxeer-backend/src/core/wallet/embedded/agentPreset");
    let copied = "";
    Object.defineProperty(dom.navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async (text: string) => {
          copied = text;
        },
      },
    });
    const readBalances = (address: Address) =>
      readAgentWalletBalances(gateway.rpc, address);
    const waitForReceipt = async (hash: Hex) =>
      (
        await gateway.rpc.waitForTransactionReceipt({
          hash,
          confirmations: 1,
          pollingInterval: 1,
          timeout: 1000,
        })
      ).status;
    for (const [state, text] of [
      ["loading", "Loading embedded wallet"],
      ["unconfigured", "gateway is not configured"],
      ["external", "Agents need the embedded wallet"],
      ["signed_out", "Sign in to the embedded wallet"],
    ] as const) {
      const view = render(createElement(AgentsAccess, { state }));
      assert.ok(view.container.textContent!.includes(text));
      view.unmount();
    }
    const browserCrypto = Object.getOwnPropertyDescriptor(
      globalThis,
      "crypto",
    )!;
    Object.defineProperty(globalThis, "crypto", {
      configurable: true,
      value: undefined,
    });
    const unsupported = render(
      createElement(AgentsWorkspace, {
        client: gateway.client,
        ownerUserId: gateway.userId,
        ownerAddress: gateway.owner,
        readBalances,
        waitForReceipt,
        explorerUrl: gateway.origin,
      }),
    );
    try {
      await waitFor(() =>
        assert.ok(
          unsupported.getByText(
            /This browser does not support Web Crypto Ed25519/,
          ),
        ),
      );
      assert.equal(
        (
          unsupported.getByRole("button", {
            name: "Create agent",
          }) as HTMLButtonElement
        ).disabled,
        true,
      );
    } finally {
      unsupported.unmount();
      Object.defineProperty(globalThis, "crypto", browserCrypto);
    }
    gateway.holdInventory();
    const props = {
      client: gateway.client,
      ownerUserId: gateway.userId,
      ownerAddress: gateway.owner,
      readBalances,
      waitForReceipt,
      explorerUrl: gateway.origin,
    };
    const view = render(createElement(AgentsWorkspace, props));
    const idle = async () => {
      await waitFor(() => {
        assert.equal(
          (
            view.getByRole("button", {
              name: "Refresh agents",
            }) as HTMLButtonElement
          ).disabled,
          false,
        );
      });
      await act(async () => {});
      await waitFor(() => {
        for (const message of [
          "Loading agent settings…",
          "Refreshing agent settings…",
          "Loading activity…",
          "Refreshing activity…",
          "Loading agents and balances…",
          "Refreshing agents and balances…",
          "Loading embedded wallet balances…",
          "Refreshing embedded wallet balances…",
          "Checking browser key support…",
        ])
          assert.ok(view.queryByText(message) === null);
      });
    };
    assert.ok(view.getByText("Loading agents and balances…"));
    await act(async () => gateway.releaseInventory());
    await waitFor(() =>
      assert.ok(
        view.getByText("No agents yet. Create an agent or claim one above."),
      ),
    );
    await waitFor(() =>
      assert.equal(
        (
          view.getByRole("button", {
            name: "Create agent",
          }) as HTMLButtonElement
        ).disabled,
        true,
      ),
    );
    fireEvent.change(view.getByLabelText("Agent name"), {
      target: { value: "trader" },
    });
    await waitFor(() =>
      assert.equal(
        (
          view.getByRole("button", {
            name: "Create agent",
          }) as HTMLButtonElement
        ).disabled,
        false,
      ),
    );
    fireEvent.click(view.getByRole("button", { name: "Create agent" }));
    await waitFor(() =>
      assert.ok(
        view.getByText(
          "Agent created and frozen. Save limits before unfreezing.",
        ),
      ),
    );
    await waitFor(() =>
      assert.ok(view.getByRole("button", { name: "Unfreeze agent" })),
    );
    await idle();
    const principal = Array.from(gateway.principals.values())[0];
    assert.equal(principal.frozen, true);
    assert.equal(principal.bound, true);
    const reveal = view.getByLabelText(
      "Agent credentials (32-byte Ed25519 seed in hex)",
    ) as HTMLTextAreaElement;
    const disclosed = JSON.parse(reveal.value);
    assert.equal(disclosed.did, principal.did);
    assert.match(disclosed.private_key, /^[0-9a-f]{64}$/);
    const pkcs8 = Buffer.concat([
      Buffer.from("302e020100300506032b657004220420", "hex"),
      Buffer.from(disclosed.private_key, "hex"),
    ]);
    const signingKey = await crypto.subtle.importKey(
      "pkcs8",
      pkcs8,
      "Ed25519",
      false,
      ["sign"],
    );
    const probe = randomBytes(32);
    assert.ok(
      verifies(
        principal.publicKey,
        probe,
        agentHex(
          new Uint8Array(
            await crypto.subtle.sign("Ed25519", signingKey, probe),
          ),
        ),
      ),
    );
    fireEvent.click(view.getByRole("button", { name: "Copy agent key" }));
    await waitFor(() => assert.ok(view.getByText("Agent key copied.")));
    assert.equal(JSON.parse(copied).private_key, disclosed.private_key);
    for (const storage of [dom.localStorage, dom.sessionStorage])
      for (let i = 0; i < storage.length; i++)
        assert.ok(
          !storage.getItem(storage.key(i)!)!.includes(disclosed.private_key),
        );
    assert.ok(
      gateway.requests.every(
        (request) =>
          !JSON.stringify(request.body ?? {}).includes(disclosed.private_key),
      ),
    );
    assert.ok(!JSON.stringify(gateway.client).includes(disclosed.private_key));
    fireEvent.click(view.getByRole("button", { name: "I saved the key" }));
    assert.ok(
      view.queryByLabelText(
        "Agent credentials (32-byte Ed25519 seed in hex)",
      ) === null,
    );
    view.rerender(createElement(AgentsWorkspace, props));
    assert.ok(
      view.queryByLabelText(
        "Agent credentials (32-byte Ed25519 seed in hex)",
      ) === null,
    );
    const start = gateway.requests.map((request) => request.path);
    assert.deepEqual(
      start
        .filter(
          (path) =>
            !path.endsWith("activity") &&
            !path.endsWith(encodeURIComponent(principal.did)) &&
            path !== "/v1/agents",
        )
        .slice(0, 7),
      [
        "/v1/agent/auth/challenge",
        "/v1/agent/auth/verify",
        `/v1/agents/${encodeURIComponent(principal.did)}/claim`,
        `/v1/agents/${encodeURIComponent(principal.did)}/unfreeze`,
        "/v1/agent/provision",
        "/v1/agent/provision",
        `/v1/agents/${encodeURIComponent(principal.did)}/freeze`,
      ],
    );
    const change = (label: string, value: string) =>
      fireEvent.change(view.getByLabelText(label), { target: { value } });
    await idle();
    const fundCount = () =>
      gateway.requests.filter((r) => r.path.endsWith("/fund")).length;
    change("Transfer amount", "10001");
    fireEvent.click(view.getByRole("button", { name: "Fund agent" }));
    await waitFor(() =>
      assert.ok(view.getByText("Amount exceeds the available wallet balance")),
    );
    await idle();
    assert.equal(fundCount(), 0);
    const path = `/v1/agents/${encodeURIComponent(principal.did)}`;
    const releaseSettings = gateway.holdRead(path);
    const releaseActivity = gateway.holdRead(`${path}/activity`);
    change("Transfer amount", "100.5");
    fireEvent.click(view.getByRole("button", { name: "Fund agent" }));
    await waitFor(() => assert.ok(view.getByText(/Fund: Confirmed/)));
    await waitFor(() =>
      assert.ok(view.getByText("Refreshing agent settings…")),
    );
    await waitFor(() => assert.ok(view.getByText("Refreshing activity…")));
    for (const name of ["USDL", "PAX", "Fund agent", "Sweep to my wallet"])
      assert.ok(view.getByRole("button", { name, exact: true }));
    assert.ok(view.getByLabelText("Transfer amount"));
    assert.ok(view.queryByText("Loading agent settings…") === null);
    assert.ok(view.queryByText("Loading activity…") === null);
    assert.ok(view.getByText("No activity yet."));
    assert.ok(view.getByText("0 PAX · 100.5 USDL"));
    await act(async () => {
      releaseSettings();
      releaseActivity();
    });
    await idle();
    assert.deepEqual(
      gateway.requests.find((r) => r.path.endsWith("/fund"))!.body,
      { token: gateway.token, amount: "100500000" },
    );
    assert.ok(view.getByText("0 PAX · 100.5 USDL"));
    fireEvent.click(view.getByRole("button", { name: "PAX", exact: true }));
    change("Transfer amount", "2");
    fireEvent.click(view.getByRole("button", { name: "Fund agent" }));
    await waitFor(() =>
      assert.ok(
        gateway.requests.some(
          (r) =>
            r.path.endsWith("/fund") &&
            (r.body as { token: unknown }).token === null,
        ),
      ),
    );
    await waitFor(() => assert.ok(view.getByText("2 PAX · 100.5 USDL")));
    await waitFor(() => assert.ok(view.getByText(/Fund: Confirmed/)));
    await idle();
    assert.deepEqual(
      gateway.requests.filter((r) => r.path.endsWith("/fund")).at(-1)!.body,
      { token: null, amount: String(2n * 10n ** 18n) },
    );
    change("Transfer amount", "1");
    fireEvent.click(view.getByRole("button", { name: "Sweep to my wallet" }));
    await waitFor(() => assert.ok(view.getByText(/Sweep: Confirmed/)));
    await waitFor(() => assert.ok(view.getByText("1 PAX · 100.5 USDL")));
    await idle();
    assert.deepEqual(
      gateway.requests.filter((r) => r.path.endsWith("/sweep")).at(-1)!.body,
      { token: null, amount: String(10n ** 18n) },
    );
    fireEvent.click(view.getByRole("button", { name: "USDL", exact: true }));
    change("Transfer amount", "0.5");
    fireEvent.click(view.getByRole("button", { name: "Sweep to my wallet" }));
    await waitFor(() =>
      assert.equal(
        gateway.requests.filter((r) => r.path.endsWith("/sweep")).length,
        2,
      ),
    );
    await waitFor(() => assert.ok(view.getByText("1 PAX · 100 USDL")));
    await waitFor(() => assert.ok(view.getByText(/Sweep: Confirmed/)));
    await idle();
    assert.deepEqual(
      gateway.requests.filter((r) => r.path.endsWith("/sweep")).at(-1)!.body,
      { token: gateway.token, amount: "500000" },
    );
    const preset = buildPerpsAgentPreset(gateway.owner, "1", "10", 30 * 86400);
    assert.throws(() =>
      buildPerpsAgentPreset(gateway.owner, "11", "10", 86400),
    );
    assert.throws(() => buildPerpsAgentPreset(gateway.owner, "1", "10", 0));
    const calls =
      await import("../packages/paxeer-backend/src/core/perps/calls");
    const { encodeFunctionData } = await import("viem");
    const { OrderType, TimeInForce } =
      await import("../packages/paxeer-backend/src/core/network/abi/PerpsPrecompile");
    assert.equal(OrderType.MARKET, 4);
    const unit = 10n ** 18n;
    const built = [
      calls.buildOpenPosition({
        marketId: 0n,
        isLong: true,
        collateralAmount: 1n,
        leverage: unit,
        quote: { entryPrice: unit, sizeUsd: unit },
      }),
      calls.buildAddSize({
        position: { marketId: 0n, isLong: true },
        additionalCollateral: 1n,
        leverage: unit,
        quote: { entryPrice: unit, sizeUsd: unit },
      }),
      calls.buildClosePosition({
        positionId: 1n,
        isLong: true,
        exitPrice: unit,
      }),
      calls.buildPartialClose({
        positionId: 1n,
        isLong: true,
        exitPrice: unit,
        closeSizeUsd: 1n,
      }),
      calls.buildSetPositionTpSl({
        positionId: 1n,
        isLong: true,
        takeProfitPrice: 0n,
        stopLossPrice: 0n,
      }),
      calls.buildAddCollateral(1n, 1n),
      calls.buildRemoveCollateral(1n, 1n),
      calls.buildTopUpPosition(1n, 1n),
      calls.buildCancelOrder(1n),
      calls.buildCancelByCloid(0n, "0x00000000000000000000000000000001"),
      calls.buildDeposit(1n),
      calls.buildWithdraw(1n),
      calls.buildPlaceLimitOrder({
        marketId: 0n,
        isLong: true,
        limitPrice: unit,
        sizeUsd: unit,
        leverage: unit,
        collateralAmount: 1n,
        timeInForce: TimeInForce.GTC,
        reduceOnly: false,
        cloid: "0x00000000000000000000000000000001",
      }),
      calls.buildPlaceStopLimitOrder({
        marketId: 0n,
        isLong: true,
        limitPrice: unit,
        triggerPrice: unit,
        sizeUsd: unit,
        leverage: unit,
        collateralAmount: 1n,
        timeInForce: TimeInForce.GTC,
        reduceOnly: false,
        cloid: "0x00000000000000000000000000000001",
      }),
    ];
    const selectors = new Set(
      built.map((call) => encodeFunctionData(call).slice(0, 10)),
    );
    assert.deepEqual(
      new Set(PERPS_AGENT_FUNCTIONS.map((call) => call.selector)),
      selectors,
    );
    assert.deepEqual(
      new Set(
        preset.rules
          .filter((r) => r.subject === "selector")
          .map((r) => r.value),
      ),
      new Set([...selectors, PERPS_AGENT_APPROVAL.selector]),
    );
    assert.deepEqual(
      new Set(
        preset.rules
          .filter((r) => r.subject === "contract")
          .map((r) => r.value.toLowerCase()),
      ),
      new Set([
        ...built.map((call) => call.address.toLowerCase()),
        gateway.token.toLowerCase(),
      ]),
    );
    assert.deepEqual(
      preset.rules
        .filter((r) => r.subject === "withdrawal")
        .map((r) => r.value),
      [gateway.owner],
    );
    assert.deepEqual(
      preset.rules.filter((r) => r.subject === "address").map((r) => r.value),
      [PERPS_AGENT_APPROVAL.spender],
    );
    assert.equal(preset.policy.allow_native_transfer, false);
    assert.equal(preset.policy.mode, "trade_only");
    change("Per-transaction cap", "1");
    change("Daily cap", "10");
    assert.ok(
      view.getByLabelText("Limits preview").textContent!.includes("1 PAX"),
    );
    fireEvent.click(
      view.getByRole("button", { name: "Save limits and keep frozen" }),
    );
    await waitFor(() =>
      assert.ok(view.getByText(/Budget remaining: 10000000/)),
    );
    await idle();
    assert.equal(principal.budgets.length, 1);
    assert.deepEqual(principal.policy, preset.policy);
    assert.equal(principal.frozen, true);
    assert.deepEqual(
      principal.rules.map(({ id, ...rule }) => rule),
      preset.rules,
    );
    assert.deepEqual(
      gateway.requests
        .filter((r) => r.path.endsWith("/budgets") && r.method === "POST")
        .at(-1)!.body,
      preset.budgets[0],
    );
    fireEvent.click(view.getByRole("button", { name: "Unfreeze agent" }));
    await waitFor(() =>
      assert.ok(view.getByRole("button", { name: "Freeze agent" })),
    );
    await idle();
    assert.equal(principal.frozen, false);
    fireEvent.click(view.getByRole("button", { name: "Freeze agent" }));
    await waitFor(() =>
      assert.ok(view.getByRole("button", { name: "Unfreeze agent" })),
    );
    await idle();
    assert.equal(principal.frozen, true);
    assert.ok(view.getAllByRole("row").length >= 5);
    assert.ok(view.getByRole("columnheader", { name: "Time" }));
    assert.ok(view.getByRole("columnheader", { name: "Native amount" }));
    gateway.failures.push({
      path: `${path}/rules`,
      message: "Rules temporarily unavailable",
    });
    fireEvent.click(
      view.getByRole("button", { name: "Save limits and keep frozen" }),
    );
    await waitFor(() =>
      assert.ok(view.getByText("Rules temporarily unavailable")),
    );
    await idle();
    assert.equal(principal.frozen, true);
    await act(async () =>
      savePerpsAgentPreset(gateway.client, principal.did, preset),
    );
    assert.equal(principal.budgets.length, 1);
    assert.equal(principal.rules.length, preset.rules.length);
    assert.ok(
      gateway.requests.some(
        (r) => r.method === "DELETE" && r.path.includes("/rules/"),
      ),
    );
    assert.ok(
      gateway.requests.some(
        (r) => r.method === "DELETE" && r.path.includes("/budgets/"),
      ),
    );
    const releaseInventory = gateway.holdRead("/v1/agents");
    const releaseRefreshedSettings = gateway.holdRead(path);
    const releaseRefreshedActivity = gateway.holdRead(`${path}/activity`);
    fireEvent.click(view.getByRole("button", { name: "Refresh agents" }));
    await waitFor(() =>
      assert.ok(view.getByText("Refreshing agents and balances…")),
    );
    assert.ok(view.getByRole("button", { name: "Manage trader" }));
    assert.ok(view.queryByText("Loading agents and balances…") === null);
    assert.ok(view.getByText("1 PAX · 100 USDL"));
    await act(async () => releaseInventory());
    await waitFor(() =>
      assert.ok(view.getByText("Refreshing agent settings…")),
    );
    await waitFor(() => assert.ok(view.getByText("Refreshing activity…")));
    await act(async () => {
      releaseRefreshedSettings();
      releaseRefreshedActivity();
    });
    await idle();
    const activityRows = view.getAllByRole("row").length;
    gateway.failures.push(
      { path, message: "Settings unavailable" },
      { path: "/v1/agents", message: "Agent inventory unavailable" },
      { path: `${path}/activity`, message: "Activity unavailable" },
    );
    fireEvent.click(view.getByRole("button", { name: "Refresh agents" }));
    await waitFor(() =>
      assert.ok(view.getByText("Agent inventory unavailable")),
    );
    await waitFor(() => assert.ok(view.getByText("Activity unavailable")));
    await waitFor(() => assert.ok(view.getByText("Settings unavailable")));
    await idle();
    assert.ok(view.getByRole("button", { name: "Manage trader" }));
    for (const name of ["USDL", "PAX", "Fund agent", "Retry settings"])
      assert.ok(view.getByRole("button", { name, exact: true }));
    assert.equal(view.getAllByRole("row").length, activityRows);
    fireEvent.click(view.getByRole("button", { name: "Retry settings" }));
    await waitFor(() =>
      assert.ok(view.queryByText("Settings unavailable") === null),
    );
    await idle();
    fireEvent.click(view.getByRole("button", { name: "Retry agents" }));
    await waitFor(() =>
      assert.ok(view.getByRole("button", { name: "Manage trader" })),
    );
    await idle();
    assert.ok(view.queryByText("Activity unavailable") === null);
    assert.ok(view.queryByText("Agent inventory unavailable") === null);
    assert.ok(
      view.queryByLabelText(
        "Agent credentials (32-byte Ed25519 seed in hex)",
      ) === null,
    );
    const externalKeys = (await crypto.subtle.generateKey("Ed25519", false, [
      "sign",
      "verify",
    ])) as CryptoKeyPair;
    const externalPub = agentHex(
      new Uint8Array(
        await crypto.subtle.exportKey("raw", externalKeys.publicKey),
      ),
    );
    const externalDid = `did:matrix:external:${externalPub.slice(0, 16)}`;
    const externalChallenge = await fetch(
      `${gateway.origin}/v1/agent/auth/challenge`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ did: externalDid }),
      },
    ).then((r) => r.json());
    const externalSignature = agentHex(
      new Uint8Array(
        await crypto.subtle.sign(
          "Ed25519",
          externalKeys.privateKey,
          new TextEncoder().encode(externalChallenge.message),
        ),
      ),
    );
    await fetch(`${gateway.origin}/v1/agent/auth/verify`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        did: externalDid,
        public_key: externalPub,
        nonce: externalChallenge.nonce,
        signature: externalSignature,
      }),
    });
    change("Agent DID", externalDid);
    fireEvent.click(
      view.getByRole("button", { name: "Prepare claim request" }),
    );
    await waitFor(() => assert.ok(view.getByLabelText("Claim request")));
    await idle();
    const claim = JSON.parse(view.getByLabelText("Claim request").textContent!);
    assert.equal(claim.owner_user_id, gateway.userId);
    assert.equal(claim.did, externalDid);
    const sig = agentHex(
      new Uint8Array(
        await crypto.subtle.sign(
          "Ed25519",
          externalKeys.privateKey,
          Buffer.from(claim.digest_hex, "hex"),
        ),
      ),
    );
    change(
      "Signed claim JSON",
      JSON.stringify({ expires: claim.expires, signature: sig }),
    );
    const releaseExternalSettings = gateway.holdRead(
      `/v1/agents/${encodeURIComponent(externalDid)}`,
    );
    fireEvent.click(
      view.getByRole("button", { name: "Claim agent", exact: true }),
    );
    await waitFor(() =>
      assert.ok(
        view.getByText("Agent claimed. Review its limits and frozen state."),
      ),
    );
    await waitFor(() => assert.ok(view.getByText("Loading agent settings…")));
    assert.ok(view.queryByRole("button", { name: "Fund agent" }) === null);
    await act(async () => releaseExternalSettings());
    await idle();
    assert.equal(gateway.principals.get(externalDid)!.owner, gateway.userId);
    assert.ok(view.getByText("Wallet: Wallet not provisioned"));
    assert.ok(view.getByText("No activity yet."));
    assert.ok(
      view.queryByLabelText(
        "Agent credentials (32-byte Ed25519 seed in hex)",
      ) === null,
    );
    assert.deepEqual(gateway.violations, []);
    view.unmount();
    const routes = await readFile(
      new URL("../apps/web/src/native/PaxeerNetwork.tsx", import.meta.url),
      "utf8",
    );
    assert.match(
      routes,
      /path\.endsWith\("\/agents"\).*AgentsModule\.AgentsPage/,
    );
    const sidebar = await readFile(
      new URL(
        "../packages/portfolio/src/layout/layout.script.tsx",
        import.meta.url,
      ),
      "utf8",
    );
    assert.ok(
      sidebar.indexOf("href: PortfolioLeftSidebarPath.ApiKey") <
        sidebar.indexOf("href: PortfolioLeftSidebarPath.Agents"),
    );
    assert.ok(
      sidebar.indexOf("href: PortfolioLeftSidebarPath.Agents") <
        sidebar.indexOf("href: PortfolioLeftSidebarPath.Setting"),
    );
  } finally {
    try {
      cleanup?.();
    } finally {
      try {
        await dom.happyDOM.close();
      } finally {
        await gateway.close();
      }
    }
  }
}

async function agentOwnerWorkspace(t: TestContext) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
  });
  t.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(
        new Error(
          `Agent workspace exited before completing assertions: ${code}`,
        ),
      ),
    );
    worker.once("message", (message) => {
      try {
        assert.equal(message, "assertions-passed");
        resolve();
      } catch (error) {
        reject(error);
      }
    });
  });
}
if (isMainThread) {
  test(agentOwnerRequestAndAmountValidation);
  test(agentOwnerCreationFreezesOnProvisionFailure);
  test({ timeout: 30_000 }, agentOwnerWorkspace);
} else {
  void exerciseAgentOwnerWorkspace()
    .then(() => {
      assert.ok(parentPort);
      parentPort.postMessage("assertions-passed");
      parentPort.close();
    })
    .catch((error) => {
      parentPort?.close();
      throw error;
    });
}

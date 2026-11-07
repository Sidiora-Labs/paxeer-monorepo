import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { EventEmitter, once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import type { Duplex } from "node:stream";
import { after, test } from "node:test";
import { Worker, isMainThread, parentPort } from "node:worker_threads";
import { fileURLToPath } from "node:url";
import React, { createElement as h, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { QueryClient } from "@tanstack/react-query";
import type { Config } from "wagmi";
import {
  decodeFunctionData,
  encodeFunctionResult,
  type Address,
  type Hex,
  type EIP1193Provider,
} from "viem";
import type { MarketConfig, MarginTier } from "../src/core/perps/marketConfig";

// In-memory host for the real React reconciler used by the installed renderer.
// Components, effects, context, external stores and hooks are production code.
const localRequire = createRequire(import.meta.url);
const rendererRequire = createRequire(
  localRequire.resolve("@react-three/fiber"),
);
interface Host {
  type: string;
  props: Record<string, unknown>;
  children: Host[];
  text?: string;
}
interface Renderer {
  createContainer(
    container: Host,
    tag: number,
    hydration: null,
    strict: boolean,
    concurrent: null,
    prefix: string,
    onError: (error: Error) => void,
    transition: null,
  ): unknown;
  updateContainer(
    node: ReactNode,
    root: unknown,
    parent: null,
    callback: (() => void) | null,
  ): void;
  flushSync<T>(callback: () => T): T;
  flushPassiveEffects(): boolean;
}
const makeRenderer: (config: Record<string, unknown>) => Renderer =
  rendererRequire("react-reconciler");
const emptyContext = {};
const append = (parent: Host, child: Host) => {
  parent.children = parent.children.filter((entry) => entry !== child);
  parent.children.push(child);
};
const remove = (parent: Host, child: Host) => {
  parent.children = parent.children.filter((entry) => entry !== child);
};
const insert = (parent: Host, child: Host, before: Host) => {
  remove(parent, child);
  parent.children.splice(parent.children.indexOf(before), 0, child);
};
const renderer = makeRenderer({
  now: Date.now,
  isPrimaryRenderer: true,
  supportsMutation: true,
  supportsPersistence: false,
  supportsHydration: false,
  getRootHostContext: () => emptyContext,
  getChildHostContext: () => emptyContext,
  getPublicInstance: (instance: Host) => instance,
  prepareForCommit: () => null,
  resetAfterCommit: () => {},
  createInstance: (type: string, props: Record<string, unknown>): Host => ({
    type,
    props,
    children: [],
  }),
  createTextInstance: (text: string): Host => ({
    type: "#text",
    props: {},
    children: [],
    text,
  }),
  appendInitialChild: append,
  appendChild: append,
  appendChildToContainer: append,
  removeChild: remove,
  removeChildFromContainer: remove,
  insertBefore: insert,
  insertInContainerBefore: insert,
  finalizeInitialChildren: () => false,
  shouldSetTextContent: () => false,
  prepareUpdate: () => true,
  commitUpdate: (
    instance: Host,
    _payload: unknown,
    _type: string,
    _old: unknown,
    props: Record<string, unknown>,
  ) => {
    instance.props = props;
  },
  commitTextUpdate: (instance: Host, _old: string, text: string) => {
    instance.text = text;
  },
  resetTextContent: (instance: Host) => {
    instance.children = [];
  },
  clearContainer: (container: Host) => {
    container.children = [];
  },
  scheduleTimeout: setTimeout,
  cancelTimeout: clearTimeout,
  noTimeout: -1,
  getCurrentEventPriority: () => 16,
  detachDeletedInstance: () => {},
});
class MemoryStorage implements Storage {
  private values = new Map<string, string>();
  get length() {
    return this.values.size;
  }
  getItem(key: string) {
    return this.values.get(key) ?? null;
  }
  setItem(key: string, value: string) {
    this.values.set(key, value);
  }
  removeItem(key: string) {
    this.values.delete(key);
  }
  clear() {
    this.values.clear();
  }
  key(index: number) {
    return [...this.values.keys()][index] ?? null;
  }
}

const alice: Address = "0x00000000000000000000000000000000000000ab";
const bob: Address = "0x00000000000000000000000000000000000000ac";
const token: Address = "0x85FcD13735F4309833A503EE804ea32395851479";
const otherToken: Address = "0x0000000000000000000000000000000000000002";
const storage = new MemoryStorage();
const events = new EventTarget();
const documentEvents = new EventTarget();
const walletEvents = new EventEmitter();
let walletAddress = alice;
let origin: string;
let wagmiConfig: Config;
let abi: typeof import("../src/core/network/abi/PerpsPrecompile").PerpsPrecompileABI;
let captured: typeof import("../src/hooks/perps/repair-22-31-20-fixtures.json");
let config: MarketConfig;
let tiers: readonly MarginTier[];
let governanceRevision = 0;
let queries: QueryClient;
let QueryClientProvider: typeof import("@tanstack/react-query").QueryClientProvider;
let WagmiContext: typeof import("wagmi").WagmiContext;
let connect: typeof import("wagmi/actions").connect;
let disconnect: typeof import("wagmi/actions").disconnect;
let getAccount: typeof import("wagmi/actions").getAccount;
let watchAccount: typeof import("wagmi/actions").watchAccount;
let connector: Config["connectors"][number];
const failures: unknown[] = [];
const counts = new Map<string, number>();
const pending = new Map<string, ReturnType<typeof hold>>();

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
function hold(key: string) {
  assert.ok(!pending.has(key), `only one held request for ${key}`);
  const entered = deferred<void>();
  const release = deferred<boolean>();
  const completed = deferred<void>();
  const gate = { entered, release, completed };
  pending.set(key, gate);
  return gate;
}
async function waitFor(condition: () => boolean, description: string) {
  const deadline = Date.now() + 5000;
  while (!condition() && Date.now() < deadline) {
    renderer.flushPassiveEffects();
    await new Promise((done) => setTimeout(done, 5));
  }
  renderer.flushPassiveEffects();
  assert.ok(condition(), description);
}
async function settle() {
  // Drain actual HTTP, promise, React external-store and query notifications.
  for (let i = 0; i < 8; i++) {
    await new Promise((done) => setTimeout(done, 5));
    renderer.flushPassiveEffects();
  }
}
function mount<T>(read: () => T) {
  let value: T;
  const renders: T[] = [];
  function Consumer() {
    value = read();
    renders.push(value);
    return null;
  }
  const container: Host = { type: "root", props: {}, children: [] };
  const root = renderer.createContainer(
    container,
    0,
    null,
    false,
    null,
    "",
    (error) => {
      throw error;
    },
    null,
  );
  const render = (node: ReactNode) => {
    renderer.flushSync(() =>
      renderer.updateContainer(
        h(
          WagmiContext.Provider,
          { value: wagmiConfig },
          h(QueryClientProvider, { client: queries }, node),
        ),
        root,
        null,
        null,
      ),
    );
    renderer.flushPassiveEffects();
  };
  render(h(Consumer));
  return {
    value() {
      return value!;
    },
    renders,
    update() {
      render(h(Consumer));
    },
    unmount() {
      render(null);
    },
  };
}
async function complete(gate: ReturnType<typeof hold>, success: boolean) {
  gate.release.resolve(success);
  await gate.completed.promise;
  await settle();
}

interface RpcRequest {
  id: number;
  method: string;
  params?: [{ to: Address; data: Hex }];
}
async function rpc(request: RpcRequest) {
  if (request.method === "net_listening")
    return { jsonrpc: "2.0", id: request.id, result: true };
  if (request.method === "eth_blockNumber")
    return { jsonrpc: "2.0", id: request.id, result: "0x1" };
  if (request.method === "eth_chainId")
    return { jsonrpc: "2.0", id: request.id, result: "0x7d" };
  assert.equal(request.method, "eth_call");
  const call = request.params?.[0];
  assert.ok(call);
  assert.equal(
    call.to.toLowerCase(),
    "0x0000000000000000000000000000000000001015",
  );
  const decoded = decodeFunctionData({ abi, data: call.data });
  const name = decoded.functionName;
  const key = `${name}:${
    decoded.args
      ?.map(String)
      .map((x) => x.toLowerCase())
      .join(":") ?? ""
  }`;
  counts.set(key, (counts.get(key) ?? 0) + 1);
  let result: Hex;
  switch (name) {
    case "getAccount": {
      const amount = decoded.args[0].toLowerCase() === alice ? 100n : 200n;
      result = encodeFunctionResult({
        abi,
        functionName: name,
        result: {
          collateral: amount,
          equityUsd: amount,
          initialMarginUsd: 0n,
          maintenanceMarginUsd: 0n,
          withdrawable: amount,
          feeTier: 1,
          volume14dUsd: 0n,
          positionCount: 0n,
          openOrderCount: 0n,
        },
      });
      break;
    }
    case "getPositionMargin":
      result = encodeFunctionResult({
        abi,
        functionName: name,
        result: {
          token,
          lockedAmount: decoded.args[0] * 10n,
          marketId: decoded.args[0],
          isLong: true,
        },
      });
      break;
    case "getAccountSummary": {
      const account = decoded.args.length === 2 ? decoded.args[0] : alice;
      const selectedToken = decoded.args[decoded.args.length - 1];
      const idle =
        (account.toLowerCase() === alice ? 100n : 200n) +
        (selectedToken.toLowerCase() === token.toLowerCase() ? 1n : 2n);
      result = encodeFunctionResult({
        abi,
        functionName: name,
        result: { idle, totalLocked: 10n, positionCount: 1n },
      });
      break;
    }
    case "getMarketConfig":
      result = encodeFunctionResult({
        abi,
        functionName: name,
        result: config,
      });
      break;
    case "getMarginTiers":
      result = encodeFunctionResult({ abi, functionName: name, result: tiers });
      break;
    case "getFeeTier":
      result = encodeFunctionResult({
        abi,
        functionName: name,
        result: [2, 250n, -50n, 9007199254740993n],
      });
      break;
    default:
      throw new Error(`Unexpected read: ${name}`);
  }
  const gate = pending.get(key);
  if (gate) {
    pending.delete(key);
    gate.entered.resolve();
    const success = await gate.release.promise;
    if (!success) result = "0x01"; // Malformed ABI bytes exercise the real decoder's failure path.
    gate.completed.resolve();
  }
  return { jsonrpc: "2.0", id: request.id, result };
}

const alertStore = new Map([
  [
    alice,
    [
      {
        id: 1,
        alertType: "price_above",
        marketId: 2,
        threshold: "100",
        enabled: true,
        triggeredAt: null,
      },
    ],
  ],
  [
    bob,
    [
      {
        id: 2,
        alertType: "price_below",
        marketId: 3,
        threshold: "200",
        enabled: true,
        triggeredAt: null,
      },
    ],
  ],
]);
async function graphql(body: {
  query: string;
  variables: Record<string, string>;
}) {
  const alerts = body.query.includes("UserAlerts");
  const account = body.variables.address;
  const key = `${alerts ? "alerts" : "query"}:${account}`;
  counts.set(key, (counts.get(key) ?? 0) + 1);
  const data = alerts
    ? { userAlerts: alertStore.get(account as Address) ?? [] }
    : { account };
  const gate = pending.get(key);
  if (gate) {
    pending.delete(key);
    gate.entered.resolve();
    const success = await gate.release.promise;
    gate.completed.resolve();
    if (!success)
      return { errors: [{ message: "Recorded selection read failure" }] };
  }
  return { data };
}

const server = createServer(async (request, response) => {
  try {
    let body = "";
    for await (const chunk of request) body += chunk;
    response.setHeader("Content-Type", "application/json");
    if (request.url === "/graphql") {
      response.end(JSON.stringify(await graphql(JSON.parse(body))));
    } else if (request.url === "/resync") {
      for (const peer of peers) peer.close();
      response.end(JSON.stringify({ ok: true }));
    } else if (request.url === "/governance") {
      // This committed in-memory peer owns mutable chain state. Governance writes
      // and ABI reads share this store; no production export or transport is replaced.
      const update: {
        enabled: boolean;
        tickSize: string;
        lotSize: string;
        maxLeverage: string;
        tierMax: string;
      } = JSON.parse(body);
      config = {
        ...config,
        enabled: update.enabled,
        tickSize: BigInt(update.tickSize),
        lotSize: BigInt(update.lotSize),
        maxLeverage: BigInt(update.maxLeverage),
      };
      tiers = tiers.map((tier) => ({
        ...tier,
        maxLeverage: BigInt(update.tierMax),
      }));
      governanceRevision++;
      response.end(JSON.stringify({ revision: governanceRevision }));
    } else {
      assert.equal(request.url, "/");
      const decoded = JSON.parse(body) as RpcRequest | RpcRequest[];
      response.end(
        JSON.stringify(
          Array.isArray(decoded)
            ? await Promise.all(decoded.map(rpc))
            : await rpc(decoded),
        ),
      );
    }
  } catch (error) {
    failures.push(error);
    response.writeHead(500).end(JSON.stringify({ error: String(error) }));
  }
});

interface StreamRequest {
  query: string;
  variables?: { userAddress?: string; marketId?: number };
}
class StreamPeer {
  private buffer = Buffer.alloc(0);
  private subscriptions = new Map<string, StreamRequest>();
  constructor(private socket: Duplex) {
    socket.on("data", (chunk: Buffer) => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      this.drain();
    });
  }
  private frame(opcode: number, payload: Buffer) {
    assert.ok(payload.length < 65536);
    const header =
      payload.length < 126
        ? Buffer.from([opcode, payload.length])
        : Buffer.from([opcode, 126, payload.length >> 8, payload.length & 255]);
    this.socket.write(Buffer.concat([header, payload]));
  }
  private send(value: unknown) {
    this.frame(0x81, Buffer.from(JSON.stringify(value)));
  }
  private drain() {
    for (;;) {
      if (this.buffer.length < 2) return;
      const opcode = this.buffer[0]! & 15;
      assert.ok(this.buffer[1]! & 128, "client frames are masked");
      let size = this.buffer[1]! & 127;
      let offset = 2;
      if (size === 126) {
        if (this.buffer.length < 4) return;
        size = this.buffer.readUInt16BE(2);
        offset = 4;
      } else if (size === 127) {
        if (this.buffer.length < 10) return;
        size = Number(this.buffer.readBigUInt64BE(2));
        offset = 10;
      }
      if (this.buffer.length < offset + 4 + size) return;
      const mask = this.buffer.subarray(offset, offset + 4);
      const payload = Buffer.from(
        this.buffer.subarray(offset + 4, offset + 4 + size),
      );
      for (let i = 0; i < payload.length; i++)
        payload[i] = payload[i]! ^ mask[i % 4]!;
      this.buffer = this.buffer.subarray(offset + 4 + size);
      if (opcode === 8) {
        this.frame(0x88, payload);
        this.socket.end();
        return;
      }
      assert.equal(opcode, 1);
      const message: { type: string; id: string; payload: StreamRequest } =
        JSON.parse(payload.toString());
      if (message.type === "connection_init")
        this.send({ type: "connection_ack" });
      else if (message.type === "subscribe") {
        this.subscriptions.set(message.id, message.payload);
        this.replay(message.id, message.payload);
      } else if (message.type === "complete")
        this.subscriptions.delete(message.id);
      else if (message.type === "ping") this.send({ type: "pong" });
      else assert.fail(`Unexpected stream frame ${message.type}`);
    }
  }
  private replay(id: string, request: StreamRequest) {
    const { userAddress, marketId } = request.variables ?? {};
    if (userAddress !== undefined && userAddress !== alice) return;
    if (marketId !== undefined && marketId !== 2) return;
    const fixture = captured.subscriptions.find(
      (entry) =>
        entry.request.query.replace(/\s+/g, " ").trim() ===
          request.query.replace(/\s+/g, " ").trim() &&
        JSON.stringify(entry.request.variables) ===
          JSON.stringify(request.variables),
    );
    if (fixture) {
      for (const message of fixture.messages)
        this.send({ ...JSON.parse(message), id });
    } else if (request.query.includes("PriceUpdated")) {
      // The in-memory indexer store emits the same SDL shape as the producer.
      this.send({
        id,
        type: "next",
        payload: {
          data: {
            priceUpdated: {
              marketId: 2,
              price: "1800",
              updatedAt: "2026-01-01T00:00:00Z",
              blockNumber: 1,
            },
          },
        },
      });
    } else if (request.query.includes("PositionChanged")) {
      this.send({
        id,
        type: "next",
        payload: {
          data: {
            positionChanged: [
              {
                positionId: "9",
                marketId: 2,
                isLong: true,
                sizeUsd: "500",
                leverage: "2",
                entryPrice: "1800",
                status: "active",
                realizedPnl: "0",
              },
            ],
          },
        },
      });
    } else if (request.query.includes("indexerStatusUpdated")) {
      this.send({
        id,
        type: "next",
        payload: {
          data: {
            indexerStatusUpdated: {
              lastIndexedBlock: 1,
              chainHead: 1,
              isSynced: true,
              blocksScanned: 1,
              eventsProcessed: 0,
            },
          },
        },
      });
    } else
      assert.fail(
        "Every stream must use a committed store or producer capture",
      );
  }
  close() {
    this.socket.destroy();
  }
}
const peers: StreamPeer[] = [];

async function setup() {
  server.on("upgrade", (request, socket, head) => {
    assert.equal(request.url, "/graphql");
    assert.equal(head.length, 0);
    const accept = createHash("sha1")
      .update(
        `${request.headers["sec-websocket-key"]}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`,
      )
      .digest("base64");
    socket.write(
      `HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\nSec-WebSocket-Protocol: graphql-transport-ws\r\n\r\n`,
    );
    peers.push(new StreamPeer(socket));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object");
  origin = `http://127.0.0.1:${address.port}`;
  // These are set before ANY production module is loaded, including ABI/fixtures.
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  delete process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY;
  Object.assign(globalThis, {
    React,
    localStorage: storage,
    sessionStorage: storage,
  });
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage: storage,
      sessionStorage: storage,
      location: {
        origin,
        href: origin,
        pathname: "/perps",
        search: "",
        hash: "",
      },
      addEventListener: events.addEventListener.bind(events),
      removeEventListener: events.removeEventListener.bind(events),
      dispatchEvent: events.dispatchEvent.bind(events),
    },
  });
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: documentEvents,
  });
  // Query timers detect browser/server mode at import, so load the real query
  // and wallet providers only after installing the in-memory browser host.
  const queryModule = localRequire(
    "@tanstack/react-query",
  ) as typeof import("@tanstack/react-query");
  QueryClientProvider = queryModule.QueryClientProvider;
  queries = new queryModule.QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  ({ WagmiContext } = localRequire("wagmi") as typeof import("wagmi"));
  ({ connect, disconnect, getAccount, watchAccount } = localRequire(
    "wagmi/actions",
  ) as typeof import("wagmi/actions"));
  const { injected } = localRequire(
    "wagmi/connectors",
  ) as typeof import("wagmi/connectors");
  ({ PerpsPrecompileABI: abi } = localRequire(
    "../src/core/network/abi/PerpsPrecompile",
  ) as typeof import("../src/core/network/abi/PerpsPrecompile"));
  captured = JSON.parse(
    readFileSync(
      new URL(
        "../src/hooks/perps/repair-22-31-20-fixtures.json",
        import.meta.url,
      ),
      "utf8",
    ),
  );
  const node = localRequire(
    "./node-leverage-output",
  ) as typeof import("./node-leverage-output");
  config = { ...node.nodeMarketConfig };
  tiers = node.nodeMarginTiers.map((tier) => ({ ...tier }));
  ({ wagmiConfig } = localRequire(
    "../src/core/wagmi",
  ) as typeof import("../src/core/wagmi"));
  // Set up one connector instance: the injected factory retains provider event
  // listeners, so recreating it on every connection leaves listeners bound to
  // an earlier connector instead of the active wallet selection.
  connector = wagmiConfig._internal.connectors.setup(
    injected({
      target: {
        id: "selection-fixture",
        name: "Selection fixture",
        provider: provider as EIP1193Provider,
      },
      shimDisconnect: false,
    }),
  );
}

const provider = {
  async request({ method }: { method: string }) {
    switch (method) {
      case "eth_requestAccounts":
      case "eth_accounts":
        return [walletAddress];
      case "eth_chainId":
        return "0x7d";
      case "wallet_requestPermissions":
        return [{ parentCapability: "eth_accounts" }];
      case "wallet_revokePermissions":
        return null;
      default:
        throw new Error(`Unexpected wallet method ${method}`);
    }
  },
  on(event: string, listener: (...args: unknown[]) => void) {
    walletEvents.on(event, listener);
  },
  removeListener(event: string, listener: (...args: unknown[]) => void) {
    walletEvents.removeListener(event, listener);
  },
};
async function selectWallet(address?: Address) {
  const matches = (account: ReturnType<typeof getAccount>) =>
    address
      ? account.status === "connected" &&
        account.address.toLowerCase() === address.toLowerCase()
      : account.status === "disconnected";
  let unsubscribe = () => {};
  let timeout: ReturnType<typeof setTimeout> | undefined;
  const selected = new Promise<void>((resolve, reject) => {
    unsubscribe = watchAccount(wagmiConfig, {
      onChange(account) {
        if (matches(account)) resolve();
      },
    });
    timeout = setTimeout(
      () =>
        reject(
          new Error(
            "real injected connector selects account within five seconds",
          ),
        ),
      5000,
    );
    if (matches(getAccount(wagmiConfig))) resolve();
  });
  try {
    await Promise.all([
      selected,
      (async () => {
        if (!address) await disconnect(wagmiConfig);
        else {
          walletAddress = address;
          if (getAccount(wagmiConfig).status === "connected")
            walletEvents.emit("accountsChanged", [address]);
          else await connect(wagmiConfig, { connector });
        }
      })(),
    ]);
    assert.ok(
      matches(getAccount(wagmiConfig)),
      "real injected connector selects account",
    );
  } finally {
    clearTimeout(timeout);
    unsubscribe();
  }
  await settle();
}

async function collateralAccountRace() {
  if (isMainThread) return runWorker("collateralAccountRace");
  const { useAutoCollateral } = localRequire(
    "../src/hooks/perps/useAutoCollateral",
  ) as typeof import("../src/hooks/perps/useAutoCollateral");
  for (const success of [true, false]) {
    for (const replacement of ["loaded", "loading", "disconnect"] as const) {
      await selectWallet(alice);
      const old = hold(`getAccount:${alice}`);
      const mounted = mount(useAutoCollateral);
      try {
        await old.entered.promise;
        let next: ReturnType<typeof hold> | undefined;
        if (replacement === "loading") next = hold(`getAccount:${bob}`);
        await selectWallet(replacement === "disconnect" ? undefined : bob);
        if (next) await next.entered.promise;
        if (replacement === "loaded")
          await waitFor(
            () => mounted.value().collateral === 200n,
            "B collateral arrives",
          );
        else {
          assert.equal(
            mounted.value().collateral,
            0n,
            "A collateral is not visible for B or disconnect",
          );
          assert.equal(mounted.value().isLoading, replacement === "loading");
        }
        await complete(old, success);
        assert.equal(
          mounted.value().collateral,
          replacement === "loaded" ? 200n : 0n,
          "late A success/failure cannot publish for another account",
        );
        assert.equal(
          mounted.value().isLoading,
          replacement === "loading",
          "late A cannot clear B loading",
        );
        if (next) {
          await complete(next, true);
          assert.equal(mounted.value().collateral, 200n);
        }
        await selectWallet();
        assert.equal(
          mounted.value().collateral,
          0n,
          "disconnect clears a previously successful read",
        );
        assert.equal(mounted.value().isLoading, false);
      } finally {
        mounted.unmount();
        old.release.resolve(true);
      }
    }
  }
}

async function alertAccountRace() {
  if (isMainThread) return runWorker("alertAccountRace");
  const { usePerfdbAlerts } = localRequire(
    "../src/hooks/perps/usePerfdbAlerts",
  ) as typeof import("../src/hooks/perps/usePerfdbAlerts");
  for (const success of [true, false]) {
    for (const replacement of ["loaded", "loading", "disconnect"] as const) {
      let address: string | undefined = alice;
      const old = hold(`alerts:${alice}`);
      const mounted = mount(() => usePerfdbAlerts(address));
      try {
        await old.entered.promise;
        const next =
          replacement === "loading" ? hold(`alerts:${bob}`) : undefined;
        address = replacement === "disconnect" ? undefined : bob;
        const before = mounted.renders.length;
        mounted.update();
        assert.deepEqual(
          mounted.renders[before]!.alerts,
          [],
          "selection clears before effects run",
        );
        if (next) await next.entered.promise;
        if (replacement === "loaded")
          await waitFor(
            () => mounted.value().alerts[0]?.id === "2",
            "B alerts arrive",
          );
        await complete(old, success);
        assert.deepEqual(
          mounted.value().alerts.map((alert) => alert.id),
          replacement === "loaded" ? ["2"] : [],
        );
        assert.equal(mounted.value().isLoading, replacement === "loading");
        assert.equal(mounted.value().error, null, "A error is not B error");
        if (next) {
          await complete(next, true);
          assert.equal(mounted.value().alerts[0]?.id, "2");
        }
        address = undefined;
        mounted.update();
        assert.deepEqual(mounted.value().alerts, []);
        assert.equal(mounted.value().error, null);
        assert.equal(mounted.value().isLoading, false);
      } finally {
        mounted.unmount();
        old.release.resolve(true);
      }
    }
  }
}

async function marginSelectionRace() {
  if (isMainThread) return runWorker("marginSelectionRace");
  const { usePositionMargin, useAccountSummary } = localRequire(
    "../src/hooks/perps/useMarginMode",
  ) as typeof import("../src/hooks/perps/useMarginMode");
  for (const success of [true, false]) {
    let address: Address | undefined = alice;
    let positionId: bigint | undefined = 1n;
    let selectedToken: Address | undefined = token;
    const oldPosition = hold("getPositionMargin:1");
    const oldSummary = hold(
      `getAccountSummary:${alice}:${token.toLowerCase()}`,
    );
    const mounted = mount(() => ({
      position: usePositionMargin(address, positionId),
      summary: useAccountSummary(address, selectedToken),
    }));
    try {
      await Promise.all([
        oldPosition.entered.promise,
        oldSummary.entered.promise,
      ]);
      // Account replacement also invalidates a position read with unchanged id.
      const nextPosition = hold("getPositionMargin:1");
      const nextSummary = hold(
        `getAccountSummary:${bob}:${token.toLowerCase()}`,
      );
      address = bob;
      mounted.update();
      await Promise.all([
        nextPosition.entered.promise,
        nextSummary.entered.promise,
      ]);
      await complete(oldPosition, success);
      await complete(oldSummary, success);
      assert.equal(mounted.value().position.data, undefined);
      assert.equal(mounted.value().summary.data, undefined);
      assert.equal(mounted.value().position.error, null);
      assert.equal(mounted.value().summary.error, null);
      assert.equal(mounted.value().position.isLoading, true);
      assert.equal(mounted.value().summary.isLoading, true);
      await complete(nextPosition, true);
      await complete(nextSummary, true);
      assert.equal(mounted.value().position.data?.lockedAmount, 10n);
      assert.equal(mounted.value().summary.data?.idle, 201n);
      const stalePosition = hold("getPositionMargin:1");
      const staleSummary = hold(
        `getAccountSummary:${bob}:${token.toLowerCase()}`,
      );
      void mounted.value().position.refetch();
      void mounted.value().summary.refetch();
      await Promise.all([
        stalePosition.entered.promise,
        staleSummary.entered.promise,
      ]);
      positionId = 2n;
      selectedToken = otherToken;
      const before = mounted.renders.length;
      mounted.update();
      assert.equal(mounted.renders[before]!.position.data, undefined);
      assert.equal(mounted.renders[before]!.summary.data, undefined);
      await waitFor(
        () =>
          mounted.value().position.data?.lockedAmount === 20n &&
          mounted.value().summary.data?.idle === 202n,
        "position and token replacements load",
      );
      await complete(stalePosition, success);
      await complete(staleSummary, success);
      assert.equal(mounted.value().position.data?.lockedAmount, 20n);
      assert.equal(mounted.value().summary.data?.idle, 202n);
      assert.equal(mounted.value().position.error, null);
      assert.equal(mounted.value().summary.error, null);
      positionId = undefined;
      selectedToken = undefined;
      mounted.update();
      assert.equal(mounted.value().position.data, undefined);
      assert.equal(mounted.value().summary.data, undefined);
      assert.equal(mounted.value().position.isLoading, false);
      assert.equal(mounted.value().summary.isLoading, false);
      address = undefined;
      positionId = 1n;
      selectedToken = token;
      mounted.update();
      assert.equal(mounted.value().position.data, undefined);
      assert.equal(mounted.value().summary.data, undefined);
    } finally {
      mounted.unmount();
      oldPosition.release.resolve(true);
      oldSummary.release.resolve(true);
    }
  }
}

async function sharedQuerySelectionRace() {
  if (isMainThread) return runWorker("sharedQuerySelectionRace");
  const { useSharedPolledQuery } = localRequire(
    "../src/hooks/perps/useGraphQLQuery",
  ) as typeof import("../src/hooks/perps/useGraphQLQuery");
  const endpoint = { url: `${origin}/graphql` };
  const query =
    "query Selection($address: String!) { account(address: $address) }";
  for (const success of [true, false]) {
    let address = alice;
    let enabled = true;
    let prefix = "A";
    const mounted = mount(() => {
      const capturedPrefix = prefix;
      return useSharedPolledQuery<{ account: string }, string>({
        endpoint,
        query,
        variables: { address },
        enabled,
        intervalMs: 0,
        pick: (raw) => `${capturedPrefix}:${raw.account}`,
      });
    });
    try {
      await waitFor(
        () => mounted.value().data === `A:${alice}`,
        "initial query arrives",
      );
      const old = hold(`query:${alice}`);
      mounted.value().refetch();
      await old.entered.promise;
      address = bob;
      prefix = "B";
      const next = hold(`query:${bob}`);
      const before = mounted.renders.length;
      mounted.update();
      assert.equal(
        mounted.renders[before]!.data,
        undefined,
        "no old data even before selection effect",
      );
      await next.entered.promise;
      await complete(old, success);
      assert.equal(mounted.value().data, undefined);
      assert.equal(mounted.value().error, null);
      assert.equal(
        mounted.value().isLoading,
        true,
        "old manual finally cannot clear B loading",
      );
      await complete(next, true);
      assert.equal(mounted.value().data, `B:${bob}`);
      const disabledRead = hold(`query:${bob}`);
      mounted.value().refetch();
      await disabledRead.entered.promise;
      enabled = false;
      mounted.update();
      await complete(disabledRead, success);
      assert.equal(mounted.value().data, undefined);
      assert.equal(mounted.value().error, null);
      assert.equal(mounted.value().isLoading, false);
      const count = counts.get(`query:${bob}`);
      mounted.value().refetch();
      await settle();
      assert.equal(
        counts.get(`query:${bob}`),
        count,
        "disabled refresh sends no request",
      );
    } finally {
      mounted.unmount();
    }
  }
  // A manual request captures its picker even when a same-selection render
  // replaces an inline picker while that HTTP request is outstanding.
  let prefix = "before";
  const mounted = mount(() => {
    const capturedPrefix = prefix;
    return useSharedPolledQuery<{ account: string }, string>({
      endpoint,
      query,
      variables: { address: alice },
      intervalMs: 0,
      pick: (raw) => `${capturedPrefix}:${raw.account}`,
    });
  });
  try {
    await waitFor(
      () => mounted.value().data === `before:${alice}`,
      "picker scenario loads",
    );
    const manual = hold(`query:${alice}`);
    mounted.value().refetch();
    await manual.entered.promise;
    prefix = "after";
    mounted.update();
    await complete(manual, true);
    assert.equal(
      mounted.value().data,
      `before:${alice}`,
      "completion uses the initiating picker",
    );
    const unmounted = hold(`query:${alice}`);
    mounted.value().refetch();
    await unmounted.entered.promise;
    mounted.unmount();
    const renders = mounted.renders.length;
    await complete(unmounted, false);
    assert.equal(
      mounted.renders.length,
      renders,
      "no completion renders after unmount",
    );
  } finally {
    mounted.unmount();
  }
}

async function indexerSelectionState() {
  if (isMainThread) return runWorker("indexerSelectionState");
  const hooks = localRequire(
    "../src/hooks/perps/useIndexerSubscriptions",
  ) as typeof import("../src/hooks/perps/useIndexerSubscriptions");
  let address: string | undefined = alice;
  let market: number | undefined = 2;
  const mounted = mount(() => ({
    price: hooks.useSingleMarketPriceSubscription(market),
    positions: hooks.usePositionChangedSubscription(address),
    order: hooks.useOrderChangedSubscription(address),
    trade: hooks.useTradeCreatedSubscription(market),
    liquidation: hooks.useLiquidationSubscription(market),
  }));
  const empty = {
    price: null,
    positions: null,
    order: null,
    trade: null,
    liquidation: null,
  };
  try {
    await waitFor(
      () => Object.values(mounted.value()).every((value) => value !== null),
      "all five real subscription hooks receive events",
    );
    assert.equal(mounted.value().price?.marketId, 2);
    assert.equal(mounted.value().positions?.[0]?.positionId, "9");
    assert.equal(mounted.value().order?.orderId, "8");
    assert.equal(mounted.value().trade?.positionId, "9");
    assert.equal(mounted.value().liquidation?.positionId, "9");
    const selected = mounted.value();
    address = `0x${alice.slice(2).toUpperCase()}`;
    mounted.update();
    assert.deepEqual(
      mounted.value(),
      selected,
      "account case changes preserve normalized selection",
    );
    address = bob;
    market = 3;
    const before = mounted.renders.length;
    mounted.update();
    assert.deepEqual(
      mounted.renders[before],
      empty,
      "replacement is empty before effects and without new events",
    );
    await settle();
    assert.deepEqual(mounted.value(), empty);
    address = undefined;
    market = undefined;
    mounted.update();
    assert.deepEqual(mounted.value(), empty);
    address = alice;
    market = 2;
    mounted.update();
    await waitFor(
      () => Object.values(mounted.value()).every((value) => value !== null),
      "reselection receives a fresh stream baseline",
    );
    address = undefined;
    market = undefined;
    mounted.update();
    assert.deepEqual(
      mounted.value(),
      empty,
      "disconnect clears previously received values",
    );
  } finally {
    mounted.unmount();
  }
}

async function registryFeeTier() {
  if (isMainThread) return runWorker("registryFeeTier");
  queries.clear();
  const { useFeeTier } = localRequire(
    "../src/hooks/perps/useFeeTier",
  ) as typeof import("../src/hooks/perps/useFeeTier");
  const { PortfolioFeeTierPanel } = localRequire(
    "../src/widgets/portfolio/PortfolioFeeTierPanel",
  ) as typeof import("../src/widgets/portfolio/PortfolioFeeTierPanel");
  let address: string | undefined;
  const mounted = mount(() => useFeeTier(address));
  try {
    assert.deepEqual(mounted.value(), {
      view: undefined,
      isLoading: false,
      error: null,
    });
    const gate = hold(`getFeeTier:${alice}`);
    address = alice;
    mounted.update();
    await gate.entered.promise;
    assert.equal(mounted.value().view, undefined);
    assert.equal(mounted.value().isLoading, true);
    await complete(gate, true);
    assert.deepEqual(mounted.value().view, {
      tier: 2,
      takerBps: 2.5,
      makerBps: -0.5,
      volume14dUsd: 9007199254740993n,
    });
    assert.equal(mounted.value().error, null);
    assert.equal(mounted.value().isLoading, false);
    const html = renderToStaticMarkup(
      h(PortfolioFeeTierPanel, {
        address: undefined,
        view: mounted.value().view,
      }),
    );
    assert.match(html, /Tier 2 · Taker 2.5 bps \/ Maker -0.5 bps/);
    const bad = hold(`getFeeTier:${bob}`);
    address = bob;
    mounted.update();
    await bad.entered.promise;
    await complete(bad, false);
    assert.equal(mounted.value().view, undefined);
    assert.equal(mounted.value().isLoading, false);
    assert.ok(
      mounted.value().error instanceof Error,
      "ABI failure is unavailable, not tier zero",
    );
    address = "not-an-address";
    mounted.update();
    await waitFor(
      () => mounted.value().error !== null,
      "invalid address is an explicit error",
    );
    assert.equal(mounted.value().view, undefined);
    address = undefined;
    mounted.update();
    assert.deepEqual(mounted.value(), {
      view: undefined,
      isLoading: false,
      error: null,
    });
    const docs = readFileSync(
      new URL("../../../contracts/perps-client.md", import.meta.url),
      "utf8",
    );
    assert.ok(docs.includes("useFeeTier(address: string | undefined)"));
    assert.ok(docs.includes("`{ view, isLoading, error }`"));
    assert.ok(docs.includes("undefined `view`"));
  } finally {
    mounted.unmount();
    queries.clear();
  }
}

async function registryMarketConfig() {
  if (isMainThread) return runWorker("registryMarketConfig");
  queries.clear();
  const { useMarketConfig } = localRequire(
    "../src/hooks/perps/useMarketConfig",
  ) as typeof import("../src/hooks/perps/useMarketConfig");
  const gate = hold(`getMarketConfig:${config.marketId}`);
  const mounted = mount(() => useMarketConfig(config.marketId));
  try {
    await gate.entered.promise;
    assert.equal(mounted.value().data, undefined);
    assert.equal(mounted.value().isLoading, true);
    assert.equal(mounted.value().isPending, true);
    await complete(gate, true);
    assert.equal(mounted.value().data?.config.marketId, config.marketId);
    assert.deepEqual(mounted.value().data?.tiers, tiers);
    assert.equal(mounted.value().isError, false);
    assert.equal("config" in mounted.value(), false);
    assert.equal("tiers" in mounted.value(), false);
    const prior = mounted.value().data;
    const bad = hold(`getMarketConfig:${config.marketId}`);
    const refresh = mounted.value().refetch();
    await bad.entered.promise;
    await complete(bad, false);
    await refresh;
    assert.equal(mounted.value().isError, true);
    assert.ok(mounted.value().error instanceof Error);
    assert.deepEqual(
      mounted.value().data,
      prior,
      "documented React Query behavior retains prior success on refresh failure",
    );
    const docs = readFileSync(
      new URL("../../../contracts/perps-client.md", import.meta.url),
      "utf8",
    );
    assert.ok(docs.includes("useMarketConfig(marketId: bigint)"));
    assert.ok(docs.includes("`{ config, tiers }`"));
    assert.ok(docs.includes("last successful `data`"));
    assert.ok(docs.includes("`config.enabled: false`"));
  } finally {
    mounted.unmount();
    queries.clear();
  }
  // A first-response error has no usable configuration.
  const bad = hold(`getMarketConfig:${config.marketId}`);
  const unavailable = mount(() => useMarketConfig(config.marketId));
  try {
    await bad.entered.promise;
    await complete(bad, false);
    assert.equal(unavailable.value().data, undefined);
    assert.equal(unavailable.value().isError, true);
    assert.equal(unavailable.value().isLoading, false);
  } finally {
    unavailable.unmount();
    queries.clear();
  }
}

async function marketConfigGovernanceRefresh() {
  if (isMainThread) return runWorker("marketConfigGovernanceRefresh");
  queries.clear();
  const { useMarketConfig, marketConfigQueryOptions } = localRequire(
    "../src/hooks/perps/useMarketConfig",
  ) as typeof import("../src/hooks/perps/useMarketConfig");
  const { orderLimitsFromConfig } = localRequire(
    "../src/widgets/trading/PerpsOrderForm",
  ) as typeof import("../src/widgets/trading/PerpsOrderForm");
  const { useIndexerStatus } = localRequire(
    "../src/hooks/perps/useIndexerSubscriptions",
  ) as typeof import("../src/hooks/perps/useIndexerSubscriptions");
  const options = marketConfigQueryOptions(config.marketId);
  assert.equal(options.staleTime, 30000);
  assert.equal(options.gcTime, 300000);
  assert.equal(options.refetchOnMount, true);
  assert.equal(options.refetchOnReconnect, true);
  assert.equal(options.refetchOnWindowFocus, true);
  assert.equal(options.refetchInterval, 30000);
  // The mounted production hook feeds the production order-form limit reader.
  // Its null result is the form's disabled/unavailable state.
  const mounted = mount(() => {
    const result = useMarketConfig(config.marketId);
    const limits = result.data ? orderLimitsFromConfig(result.data, 1n) : null;
    return { result, limits, status: useIndexerStatus() };
  });
  async function govern(
    enabled: boolean,
    tick: bigint,
    lot: bigint,
    leverage: bigint,
    tierMax: bigint,
  ) {
    const response = await fetch(`${origin}/governance`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        enabled,
        tickSize: String(tick),
        lotSize: String(lot),
        maxLeverage: String(leverage),
        tierMax: String(tierMax),
      }),
    });
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { revision: governanceRevision });
  }
  try {
    await waitFor(
      () => mounted.value().limits !== null && mounted.value().status !== null,
      "initial enabled market has live order limits and a real indexer stream",
    );
    const initial = mounted.value().limits!;
    const id = config.marketId;
    const configReads = counts.get(`getMarketConfig:${id}`)!;
    const tierReads = counts.get(`getMarginTiers:${id}`)!;
    await govern(true, initial.tickSize * 2n, initial.lotSize * 2n, 8n, 3n);
    // Closing the real socket forces subscribeStream to reconnect and emit its
    // production resync notification, which invalidates the mounted query.
    assert.equal(
      (await fetch(`${origin}/resync`, { method: "POST" })).status,
      200,
    );
    await waitFor(
      () => mounted.value().limits?.tickSize === initial.tickSize * 2n,
      "resync reads governance-updated order limits without remount",
    );
    assert.equal(mounted.value().limits?.lotSize, initial.lotSize * 2n);
    assert.equal(
      mounted.value().limits?.maxLeverage,
      3n * 10n ** 18n,
      "updated tiers and config are consumed together",
    );
    assert.ok(counts.get(`getMarketConfig:${id}`)! > configReads);
    assert.ok(counts.get(`getMarginTiers:${id}`)! > tierReads);
    await govern(false, initial.tickSize * 3n, initial.lotSize * 3n, 7n, 2n);
    // A governance update with no event/reconnect must also reach an already
    // mounted form through the real bounded timer; no fake clock or manual read.
    const deadline = Date.now() + 35000;
    while (
      mounted.value().result.data?.config.enabled !== false &&
      Date.now() < deadline
    ) {
      await new Promise((done) => setTimeout(done, 20));
      renderer.flushPassiveEffects();
    }
    assert.equal(mounted.value().result.data?.config.enabled, false);
    assert.equal(
      mounted.value().limits,
      null,
      "governance disables order-form limits on the mounted consumer",
    );
    assert.equal(
      mounted.value().result.data?.config.tickSize,
      initial.tickSize * 3n,
    );
    assert.equal(mounted.value().result.data?.tiers[0]?.maxLeverage, 2n);
    await govern(true, initial.tickSize, initial.lotSize, 9n, 4n);
    assert.equal(
      (await fetch(`${origin}/resync`, { method: "POST" })).status,
      200,
    );
    await waitFor(
      () => mounted.value().limits?.maxLeverage === 4n * 10n ** 18n,
      "recovery re-enables the same consumer with current governance limits",
    );
  } finally {
    mounted.unmount();
    queries.clear();
  }
}

const cases = {
  collateralAccountRace,
  registryFeeTier,
  sharedQuerySelectionRace,
  indexerSelectionState,
  marginSelectionRace,
  marketConfigGovernanceRefresh,
  registryMarketConfig,
  alertAccountRace,
};
let worker: Worker | undefined;
function runWorker(name: keyof typeof cases) {
  if (!worker)
    worker = new Worker(
      'const { workerData } = require("node:worker_threads"); require(workerData.loader); require(workerData.file)',
      {
        eval: true,
        workerData: {
          loader: localRequire.resolve("tsx/cjs"),
          file: fileURLToPath(import.meta.url),
        },
      },
    );
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error(`Proof ${name} exceeded 75 seconds`)),
      75000,
    );
    const onError = (error: Error) => {
      clearTimeout(timer);
      reject(error);
    };
    worker!.once("error", onError);
    worker!.once("message", (message: { name: string; error?: string }) => {
      clearTimeout(timer);
      worker!.removeListener("error", onError);
      if (message.name !== name)
        reject(new Error(`Unexpected proof response ${message.name}`));
      else if (message.error) reject(new Error(message.error));
      else resolve();
    });
    worker!.postMessage(name);
  });
}
async function cleanup() {
  queries.clear();
  for (const gate of pending.values()) gate.release.resolve(false);
  pending.clear();
  for (const peer of peers) peer.close();
  if (getAccount(wagmiConfig).status === "connected")
    await disconnect(wagmiConfig);
  server.closeAllConnections();
  await new Promise<void>((resolve) => server.close(() => resolve()));
  assert.deepEqual(
    failures,
    [],
    "every fixture request uses a real supported wire path",
  );
}
if (isMainThread) {
  test(collateralAccountRace);
  test(registryFeeTier);
  test(sharedQuerySelectionRace);
  test(indexerSelectionState);
  test(marginSelectionRace);
  test(marketConfigGovernanceRefresh);
  test(registryMarketConfig);
  test(alertAccountRace);
  after(async () => {
    if (!worker) return;
    await new Promise<void>((resolve, reject) => {
      worker!.once("message", (message: { error?: string }) =>
        message.error ? reject(new Error(message.error)) : resolve(),
      );
      worker!.postMessage("cleanup");
    }).finally(() => worker!.terminate());
  });
} else {
  const ready = setup();
  parentPort!.on("message", (name: keyof typeof cases | "cleanup") => {
    void ready
      .then(() => (name === "cleanup" ? cleanup() : cases[name]()))
      .then(
        () => parentPort!.postMessage({ name }),
        (error: unknown) =>
          parentPort!.postMessage({
            name,
            error: error instanceof Error ? error.stack : String(error),
          }),
      );
  });
}

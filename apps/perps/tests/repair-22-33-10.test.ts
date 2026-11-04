import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { createRequire } from "node:module";
import { test, after } from "node:test";
import { Worker, isMainThread, parentPort } from "node:worker_threads";
import { fileURLToPath } from "node:url";
import React, { createElement as h, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiContext } from "wagmi";
import { injected } from "wagmi/connectors";
import { connect, disconnect, getAccount, switchAccount } from "wagmi/actions";
import {
  decodeFunctionData,
  encodeFunctionResult,
  erc20Abi,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  toHex,
  type Address,
  type Hex,
  type TransactionSerialized,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

// In-memory host for the real React reconciler used by the installed renderer.
// Components, effects, context, external stores and hooks are production code.
const localRequire = createRequire(import.meta.url);
const rendererRequire = createRequire(localRequire.resolve("@react-three/fiber"));
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
  createTextInstance: (text: string): Host => ({ type: "#text", props: {}, children: [], text }),
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
const storage = new MemoryStorage();
const sessionStorage = new MemoryStorage();
const browserEvents = new EventTarget();
const alice = privateKeyToAccount(`0x${"11".repeat(32)}`);
const bob = privateKeyToAccount(`0x${"22".repeat(32)}`);
const signed: { from: Address; hash: Hex; functionName: string; amount: bigint }[] = [];
const transactions = new Map<Hex, Record<string, unknown>>();
const receipts = new Map<Hex, Record<string, unknown>>();
const requests: { method: string; params: unknown[] }[] = [];
const allowances = new Map<string, bigint>();
let receiptHold = false;
let failWalletReads = false;
let failAccountReads = false;
let rejectConnect = false;
const provisionedAddress: Address = alice.address;
let otpFailure = true;
let rejectRedirect = false;
let delayedRead: {
  address: string;
  entered: ReturnType<typeof deferred<void>>;
  release: ReturnType<typeof deferred<void>>;
} | null = null;
let perpsAbi: typeof import("../src/core/network/abi/PerpsPrecompile").PerpsPrecompileABI;
let token: Address;
let spender: Address;
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
const lower = (value: string) => value.toLowerCase();
const walletBalance = (account: string) =>
  lower(account) === lower(alice.address) ? 12_000_000n : 7_000_000n;
const collateral = new Map<string, bigint>([
  [lower(alice.address), 5_000_000n],
  [lower(bob.address), 3_000_000n],
]);
async function rpc(method: string, params: unknown[] = []): Promise<unknown> {
  requests.push({ method, params });
  switch (method) {
    case "eth_chainId":
      return "0x7d";
    case "net_version":
      return "125";
    case "net_listening":
      return true;
    case "wallet_revokePermissions":
      return null;
    case "eth_blockNumber":
      return "0x1";
    case "eth_gasPrice":
      return "0xa";
    case "eth_maxPriorityFeePerGas":
      return "0xa";
    case "eth_estimateGas":
      return "0x186a0";
    case "eth_getTransactionCount":
      return toHex(signed.filter((entry) => lower(entry.from) === lower(String(params[0]))).length);
    case "eth_getBalance":
      return "0xde0b6b3a7640000";
    case "eth_getCode":
      return "0x60006000";
    case "eth_getBlockByNumber":
      return {
        number: "0x1",
        hash: `0x${"ab".repeat(32)}`,
        parentHash: `0x${"00".repeat(32)}`,
        nonce: "0x0000000000000000",
        sha3Uncles: `0x${"00".repeat(32)}`,
        logsBloom: `0x${"00".repeat(256)}`,
        transactionsRoot: `0x${"00".repeat(32)}`,
        stateRoot: `0x${"00".repeat(32)}`,
        receiptsRoot: `0x${"00".repeat(32)}`,
        miner: alice.address,
        difficulty: "0x0",
        totalDifficulty: "0x0",
        extraData: "0x",
        size: "0x100",
        gasLimit: "0x1c9c380",
        gasUsed: "0x5208",
        timestamp: "0x1",
        transactions: [],
        uncles: [],
        baseFeePerGas: "0xa",
      };
    case "eth_call": {
      const call = params[0] as { to: Address; data: Hex; from?: Address };
      const abi = lower(call.to) === lower(token) ? erc20Abi : perpsAbi;
      const decoded = decodeFunctionData({ abi, data: call.data });
      const args = decoded.args ?? [];
      const account = String(args[0]);
      const held = delayedRead;
      if (held && lower(account) === lower(held.address) && decoded.functionName === "balanceOf") {
        delayedRead = null;
        held.entered.resolve();
        await held.release.promise;
      }
      if (decoded.functionName === "balanceOf") {
        if (failWalletReads) throw new Error("Wallet balance provider not found");
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "balanceOf",
          result: walletBalance(account),
        });
      }
      if (decoded.functionName === "allowance")
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "allowance",
          result: allowances.get(lower(account)) ?? 0n,
        });
      if (decoded.functionName === "getAccount") {
        if (failAccountReads) throw new Error("Account balance provider not found");
        const balance = collateral.get(lower(account)) ?? 0n;
        return encodeFunctionResult({
          abi: perpsAbi,
          functionName: "getAccount",
          result: {
            collateral: balance,
            equityUsd: balance * 10n ** 12n,
            initialMarginUsd: 0n,
            maintenanceMarginUsd: 0n,
            withdrawable: balance,
            feeTier: 0,
            volume14dUsd: 0n,
            positionCount: 0n,
            openOrderCount: 0n,
          },
        });
      }
      if (["approve", "deposit", "withdraw"].includes(decoded.functionName))
        return decoded.functionName === "approve"
          ? encodeFunctionResult({ abi: erc20Abi, functionName: "approve", result: true })
          : "0x";
      throw new Error(`Unrecorded call ${decoded.functionName}`);
    }
    case "eth_sendRawTransaction": {
      const raw = params[0] as TransactionSerialized;
      const transaction = parseTransaction(raw);
      const from = await recoverTransactionAddress({ serializedTransaction: raw });
      assert.equal(transaction.chainId, 125);
      const hash = keccak256(raw);
      assert.ok(!transactions.has(hash), "a signed broadcast is never replayed");
      const abi = lower(transaction.to!) === lower(token) ? erc20Abi : perpsAbi;
      const decoded = decodeFunctionData({ abi, data: transaction.data! });
      const args = decoded.args!;
      const amount = BigInt(String(args[1]));
      if (decoded.functionName === "approve") {
        assert.equal(lower(String(args[0])), lower(spender));
        allowances.set(lower(from), amount);
      } else {
        assert.equal(lower(String(args[0])), lower(token));
        const before = collateral.get(lower(from)) ?? 0n;
        if (decoded.functionName === "deposit") {
          assert.ok((allowances.get(lower(from)) ?? 0n) >= amount);
          collateral.set(lower(from), before + amount);
        } else {
          assert.equal(decoded.functionName, "withdraw");
          assert.ok(before >= amount);
          collateral.set(lower(from), before - amount);
        }
      }
      signed.push({ from, hash, functionName: decoded.functionName, amount });
      transactions.set(hash, {
        hash,
        from,
        to: transaction.to,
        input: transaction.data,
        value: "0x0",
        nonce: toHex(transaction.nonce ?? 0),
        chainId: "0x7d",
        gas: toHex(transaction.gas ?? 100000n),
        gasPrice: "0xa",
        maxFeePerGas: "0x14",
        maxPriorityFeePerGas: "0xa",
        type: "0x2",
        v: "0x0",
        r: `0x${"01".repeat(32)}`,
        s: `0x${"02".repeat(32)}`,
        blockHash: `0x${"ab".repeat(32)}`,
        blockNumber: "0x1",
        transactionIndex: "0x0",
        accessList: [],
      });
      receipts.set(hash, {
        transactionHash: hash,
        transactionIndex: "0x0",
        blockHash: `0x${"ab".repeat(32)}`,
        blockNumber: "0x1",
        from,
        to: transaction.to,
        cumulativeGasUsed: "0x5208",
        gasUsed: "0x5208",
        effectiveGasPrice: "0xa",
        contractAddress: null,
        logs: [],
        logsBloom: `0x${"00".repeat(256)}`,
        status: "0x1",
        type: "0x2",
      });
      return hash;
    }
    case "eth_getTransactionByHash":
      return transactions.get(params[0] as Hex) ?? null;
    case "eth_getTransactionReceipt":
      return receiptHold ? null : (receipts.get(params[0] as Hex) ?? null);
    default:
      throw new Error(`Unrecorded RPC method ${method}`);
  }
}

const server = createServer((request, response) => {
  void (async () => {
    const buffers: Buffer[] = [];
    for await (const chunk of request) buffers.push(Buffer.from(chunk));
    const body = Buffer.concat(buffers).toString();
    response.setHeader("content-type", "application/json");
    if (request.url?.startsWith("/auth/v1/otp")) {
      response.statusCode = otpFailure ? 429 : 200;
      response.end(
        JSON.stringify(
          otpFailure ? { msg: "rate limit exceeded", code: "over_email_send_rate_limit" } : {},
        ),
      );
      return;
    }
    if (request.url?.includes("/auth/v1/logout")) {
      response.end("{}");
      return;
    }
    if (request.url?.includes("/auth/v1/user")) {
      response.end(
        JSON.stringify({
          id: "fixture-user",
          aud: "authenticated",
          role: "authenticated",
          email: "fixture@example.test",
        }),
      );
      return;
    }
    if (request.url === "/v1/wallet/me") {
      response.end(
        JSON.stringify({
          wallet: {
            id: "wallet",
            address: provisionedAddress,
            chain_id: 125,
            created_at: "",
            last_used_at: null,
          },
          chain: { id: 125, rpc_url: origin, explorer_url: origin },
        }),
      );
      return;
    }
    const message = JSON.parse(body) as { id: number; method: string; params?: unknown[] };
    try {
      const result = await rpc(message.method, message.params);
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      response.end(
        JSON.stringify({
          jsonrpc: "2.0",
          id: message.id,
          error: { code: -32000, message: error instanceof Error ? error.message : String(error) },
        }),
      );
    }
  })().catch((error) => {
    response.destroy(error);
  });
});
let origin: string;
let wagmiConfig: typeof import("../src/core/wagmi").wagmiConfig;
let collateralModule: typeof import("../src/app/modals/DepositWithdrawModal");
let contractModule: typeof import("../src/hooks/perps/useContract");
let connectionModule: typeof import("../src/app/modals/ConnectWalletModal");
let embeddedModule: typeof import("../src/core/wallet/embedded");
let aliceConnector: NonNullable<ReturnType<typeof getAccount>["connector"]>;
let bobConnector: typeof aliceConnector;
const queries = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

async function setup() {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object");
  origin = `http://127.0.0.1:${address.port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  process.env.NEXT_PUBLIC_PAXEER_WALLET_API = origin;
  process.env.NEXT_PUBLIC_SUPABASE_URL = origin;
  process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY = "fixture-public-key";
  // Browser-owned storage/events are committed in-memory peers, shared across remounts.
  Object.assign(globalThis, { React, sessionStorage, localStorage: storage });
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage: storage,
      sessionStorage,
      location: {
        get origin() {
          if (rejectRedirect) throw new Error("provider not found");
          return origin;
        },
        href: origin,
        pathname: "/perps",
        search: "",
        hash: "",
        assign() {},
      },
      addEventListener: browserEvents.addEventListener.bind(browserEvents),
      removeEventListener: browserEvents.removeEventListener.bind(browserEvents),
      dispatchEvent: browserEvents.dispatchEvent.bind(browserEvents),
    },
  });
  // Supabase server mode still uses the explicit storage via getPaxeerWallet's real SDK.
  embeddedModule = localRequire(
    "../src/core/wallet/embedded",
  ) as typeof import("../src/core/wallet/embedded");
  ({ wagmiConfig } = localRequire("../src/core/wagmi") as typeof import("../src/core/wagmi"));
  ({ PerpsPrecompileABI: perpsAbi } = localRequire(
    "../src/core/network/abi/PerpsPrecompile",
  ) as typeof import("../src/core/network/abi/PerpsPrecompile"));
  ({
    COLLATERAL_TOKENS: {
      usdl: { address: token },
    },
  } = localRequire("../src/core/perps/config") as typeof import("../src/core/perps/config"));
  ({ PERPS_SPENDER: spender } = localRequire(
    "../src/core/network/approvals",
  ) as typeof import("../src/core/network/approvals"));
  collateralModule = localRequire(
    "../src/app/modals/DepositWithdrawModal",
  ) as typeof import("../src/app/modals/DepositWithdrawModal");
  contractModule = localRequire(
    "../src/hooks/perps/useContract",
  ) as typeof import("../src/hooks/perps/useContract");
  connectionModule = localRequire(
    "../src/app/modals/ConnectWalletModal",
  ) as typeof import("../src/app/modals/ConnectWalletModal");
  aliceConnector = wagmiConfig._internal.connectors.setup(walletAdapter(alice, "Alice"));
  bobConnector = wagmiConfig._internal.connectors.setup(walletAdapter(bob, "Bob"));
  wagmiConfig._internal.connectors.setState([
    ...wagmiConfig.connectors,
    aliceConnector,
    bobConnector,
  ]);
  await connect(wagmiConfig, { connector: bobConnector });
  await connect(wagmiConfig, { connector: aliceConnector });
}
function walletAdapter(signer: typeof alice, name: string) {
  const events = new Map<string, Set<(...args: unknown[]) => void>>();
  const provider = {
    async request({ method, params = [] }: { method: string; params?: readonly unknown[] }) {
      if (method === "eth_requestAccounts") {
        if (rejectConnect) throw Object.assign(new Error("User rejected request"), { code: 4001 });
        return [signer.address];
      }
      if (method === "eth_accounts") return [signer.address];
      if (method === "wallet_getCapabilities") return {};
      if (method === "wallet_requestPermissions") return [{ parentCapability: "eth_accounts" }];
      if (method === "eth_sendTransaction") {
        const tx = params[0] as {
          to: Address;
          data: Hex;
          gas: Hex;
          maxFeePerGas: Hex;
          maxPriorityFeePerGas: Hex;
        };
        const raw = await signer.signTransaction({
          chainId: 125,
          to: tx.to,
          data: tx.data,
          gas: BigInt(tx.gas),
          maxFeePerGas: BigInt(tx.maxFeePerGas),
          maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas),
          nonce: signed.filter((entry) => lower(entry.from) === lower(signer.address)).length,
          type: "eip1559",
        });
        return rpc("eth_sendRawTransaction", [raw]);
      }
      return rpc(method, [...params]);
    },
    on(event: string, listener: (...args: unknown[]) => void) {
      const set = events.get(event) ?? new Set();
      set.add(listener);
      events.set(event, set);
    },
    removeListener(event: string, listener: (...args: unknown[]) => void) {
      events.get(event)?.delete(listener);
    },
  };
  return injected({
    target: { id: `fixture-${name}`, name, provider: provider as import("viem").EIP1193Provider },
    shimDisconnect: false,
  });
}
function wrap(node: ReactNode) {
  return h(
    WagmiContext.Provider,
    { value: wagmiConfig },
    h(QueryClientProvider, { client: queries }, node),
  );
}
function mount(node: ReactNode) {
  const container: Host = { type: "root", props: {}, children: [] };
  const errors: Error[] = [];
  const root = renderer.createContainer(
    container,
    0,
    null,
    false,
    null,
    "",
    (error) => {
      errors.push(error);
    },
    null,
  );
  const update = (value: ReactNode) => {
    renderer.flushSync(() => renderer.updateContainer(wrap(value), root, null, null));
    renderer.flushPassiveEffects();
    assert.deepEqual(errors, []);
  };
  update(node);
  return { container, update, unmount: () => update(null) };
}
function nodes(host: Host): Host[] {
  return [host, ...host.children.flatMap(nodes)];
}
function text(host: Host): string {
  return host.text ?? host.children.map(text).join("");
}
function button(host: Host, label: string | RegExp) {
  const found = nodes(host).find(
    (node) =>
      node.type === "button" &&
      (typeof label === "string" ? text(node).trim() === label : label.test(text(node))),
  );
  assert.ok(found, `button ${label} exists: ${text(host)}`);
  return found;
}
function click(host: Host) {
  assert.ok(!host.props.disabled, `control is enabled: ${text(host)}`);
  const handler = host.props.onClick as () => unknown;
  renderer.flushSync(() => {
    handler();
  });
  renderer.flushPassiveEffects();
}
function changeAmount(host: Host, value: string) {
  const input = nodes(host).find((node) => node.type === "input");
  assert.ok(input && !input.props.disabled);
  renderer.flushSync(() => {
    (input.props.onChange as (event: { target: { value: string } }) => void)({ target: { value } });
  });
  renderer.flushPassiveEffects();
}
async function until(condition: () => boolean, description: string) {
  const deadline = Date.now() + 10_000;
  while (!condition() && Date.now() < deadline) {
    renderer.flushPassiveEffects();
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  renderer.flushPassiveEffects();
  assert.ok(condition(), description);
}
function modal(open = true) {
  return h(collateralModule.DepositWithdrawModal, { isOpen: open, onClose() {} });
}
async function ready(host: Host) {
  assert.equal(
    getAccount(wagmiConfig).status,
    "connected",
    "real wallet session survives modal mounting",
  );
  await until(
    () =>
      !button(host, "Deposit USDL").props.disabled ||
      Boolean(nodes(host).find((node) => node.type === "input" && !node.props.disabled)),
    `balances and recovery loaded for ${getAccount(wagmiConfig).address}: ${text(host)}`,
  );
}

async function walletChooserWrongConnection() {
  if (isMainThread) return runWorker("walletChooserWrongConnection");
  await setup();
  const { getPaxeerWallet } = localRequire(
    "../src/core/wallet/embedded/client",
  ) as typeof import("../src/core/wallet/embedded/client");
  const wallet = getPaxeerWallet();
  assert.ok(wallet);
  const payload = Buffer.from(
    JSON.stringify({
      exp: Math.floor(Date.now() / 1000) + 3600,
      sub: "fixture-user",
      aud: "authenticated",
    }),
  ).toString("base64url");
  const token = `${Buffer.from('{"alg":"HS256"}').toString("base64url")}.${payload}.fixture`;
  const result = await wallet.supabase.auth.setSession({
    access_token: token,
    refresh_token: "fixture-refresh",
  });
  assert.equal(result.error, null);
  const Dialog = localRequire("@radix-ui/react-dialog") as typeof import("@radix-ui/react-dialog");
  let closed = 0;
  const mounted = mount(
    h(
      embeddedModule.PaxeerProvider,
      null,
      h(
        Dialog.Root,
        null,
        h(connectionModule.ConnectModalBody, {
          onDone() {
            closed++;
          },
        }),
      ),
    ),
  );
  await until(() => text(mounted.container).includes("Alice"), "wallet chooser mounted");
  await new Promise((resolve) => setTimeout(resolve, 60));
  assert.equal(
    closed,
    0,
    "an authenticated embedded session does not dismiss deliberate wallet switching",
  );
  click(button(mounted.container, "Bob"));
  await until(() => closed === 1, "chosen browser connector completes");
  assert.equal(getAccount(wagmiConfig).address, bob.address);
  mounted.unmount();
}

async function walletConnectionErrors() {
  if (isMainThread) return runWorker("walletConnectionErrors");
  const Dialog = localRequire("@radix-ui/react-dialog") as typeof import("@radix-ui/react-dialog");
  await disconnect(wagmiConfig, { connector: bobConnector });
  await disconnect(wagmiConfig, { connector: aliceConnector });
  let closed = 0;
  const mounted = mount(
    h(
      embeddedModule.PaxeerProvider,
      null,
      h(
        Dialog.Root,
        null,
        h(connectionModule.ConnectModalBody, {
          onDone() {
            closed++;
          },
        }),
      ),
    ),
  );
  rejectConnect = true;
  click(button(mounted.container, "Alice"));
  await until(
    () =>
      Boolean(
        nodes(mounted.container).find(
          (node) => node.props["data-error-code"] === "WALLET_USER_REJECTED",
        ),
      ),
    "injected connection rejection is classified",
  );
  assert.equal(closed, 0);
  rejectConnect = false;
  click(button(mounted.container, "Retry Alice"));
  await until(() => closed === 1, "chosen injected connector retries");
  aliceConnector = getAccount(wagmiConfig).connector!;
  mounted.unmount();
  const { PaxeerWallet } = localRequire(
    "../src/core/wallet/embedded/sdk",
  ) as typeof import("../src/core/wallet/embedded/sdk");
  const isolatedWallet = new PaxeerWallet({
    apiUrl: origin,
    supabaseUrl: `${origin}/isolated`,
    supabaseAnonKey: "fixture-isolated-public-key",
  });
  const rejectingConnector = wagmiConfig._internal.connectors.setup(
    embeddedModule.paxeerEmbeddedConnector({
      wallet: isolatedWallet,
      chainId: 125,
      rpcUrl: origin,
    }),
  );
  const originalConnectors = wagmiConfig.connectors;
  wagmiConfig._internal.connectors.setState([
    rejectingConnector,
    ...originalConnectors.filter(
      (connector) => connector.id !== embeddedModule.PAXEER_CONNECTOR_ID,
    ),
  ]);
  sessionStorage.setItem("paxeer:auth:connect-intent", "embedded");
  const next = mount(
    h(
      embeddedModule.PaxeerProvider,
      null,
      h(
        Dialog.Root,
        null,
        h(connectionModule.ConnectModalBody, {
          onDone() {
            closed++;
          },
        }),
      ),
    ),
  );
  await until(
    () => Boolean(nodes(next.container).find((node) => node.props.role === "alert")),
    "embedded wagmi failure is displayed",
  );
  assert.ok(button(next.container, "Retry Paxeer Wallet"));
  assert.equal(closed, 1);
  // Authorize the actual isolated SDK and retry the selected embedded adapter.
  const primaryWallet = (
    localRequire(
      "../src/core/wallet/embedded/client",
    ) as typeof import("../src/core/wallet/embedded/client")
  ).getPaxeerWallet()!;
  const session = await primaryWallet.getSession();
  assert.ok(session);
  assert.equal(
    (
      await isolatedWallet.supabase.auth.setSession({
        access_token: session.access_token,
        refresh_token: session.refresh_token,
      })
    ).error,
    null,
  );
  click(button(next.container, "Retry Paxeer Wallet"));
  await until(() => closed === 2, "embedded selected connector retries after auth");
  assert.equal(getAccount(wagmiConfig).connector?.id, embeddedModule.PAXEER_CONNECTOR_ID);
  assert.equal(getAccount(wagmiConfig).address, alice.address);
  next.unmount();
  await disconnect(wagmiConfig, { connector: rejectingConnector });
  isolatedWallet.supabase.auth.stopAutoRefresh();
  wagmiConfig._internal.connectors.setState([...originalConnectors]);
  if (getAccount(wagmiConfig).connector?.uid !== aliceConnector.uid)
    await connect(wagmiConfig, { connector: aliceConnector });
  await connect(wagmiConfig, { connector: bobConnector });
  await switchAccount(wagmiConfig, { connector: aliceConnector });
}

async function emailRejectionState() {
  if (isMainThread) return runWorker("emailRejectionState");
  let attempts = 0;
  let failures = 0;
  function EmailSurface() {
    const paxeer = embeddedModule.usePaxeer();
    return h(connectionModule.EmbeddedSignInSection, {
      paxeer,
      disabled: false,
      onAttempt() {
        attempts++;
      },
      onFailure() {
        failures++;
      },
    });
  }
  const mounted = mount(h(embeddedModule.PaxeerProvider, null, h(EmailSurface)));
  changeAmount(mounted.container, "fixture@example.test");
  const submit = () => {
    const form = nodes(mounted.container).find((node) => node.type === "form");
    assert.ok(form);
    renderer.flushSync(() => {
      void (form.props.onSubmit as (event: { preventDefault(): void }) => Promise<void>)({
        preventDefault() {},
      });
    });
    renderer.flushPassiveEffects();
  };
  rejectRedirect = true;
  submit();
  await until(() => failures === 1, "rejected provider call is handled");
  assert.ok(!button(mounted.container, "Continue with email").props.disabled);
  assert.ok(
    nodes(mounted.container).some(
      (node) => node.props["data-error-code"] === "WALLET_PROVIDER_UNAVAILABLE",
    ),
  );
  rejectRedirect = false;
  submit();
  await until(() => failures === 2, "provider error result is handled");
  assert.ok(!button(mounted.container, "Continue with email").props.disabled);
  otpFailure = false;
  submit();
  await until(() => text(mounted.container).includes("Check your email"), "email retry succeeds");
  assert.equal(attempts, 3);
  mounted.unmount();
}

async function collateralOverlappingSubmissions() {
  if (isMainThread) return runWorker("collateralOverlappingSubmissions");
  receiptHold = true;
  const mounted = mount(modal());
  await ready(mounted.container);
  changeAmount(mounted.container, "1");
  const submit = button(mounted.container, "Deposit USDL");
  const withdrawAll = button(mounted.container, /^Withdraw all/);
  const staleWithdrawAll = withdrawAll.props.onClick as () => void;
  const start = signed.length;
  click(submit);
  renderer.flushSync(() => {
    staleWithdrawAll();
    (submit.props.onClick as () => void)();
  });
  await until(() => signed.length === start + 1, "approval broadcast through real wallet adapter");
  assert.equal(signed[start].functionName, "approve");
  for (const control of nodes(mounted.container).filter(
    (node) =>
      node.type === "input" ||
      (node.type === "button" &&
        /^(Deposit|Withdraw|Max:|25%|50%|75%|100%)/.test(text(node).trim())),
  )) {
    assert.equal(control.props.disabled, true, `pending control disabled: ${text(control)}`);
  }
  mounted.update(modal(false));
  mounted.update(modal(true));
  await until(
    () => Boolean(button(mounted.container, "Deposit USDL").props.disabled),
    "close and reopen retains pending guard",
  );
  mounted.unmount();
  const reopened = mount(modal());
  assert.equal(button(reopened.container, "Deposit USDL").props.disabled, true);
  receiptHold = false;
  await until(
    () =>
      contractModule.getPendingTransactions().length === 0 &&
      !nodes(reopened.container).find((node) => node.type === "input")?.props.disabled,
    "approval reconciles after remount",
  );
  assert.equal(
    signed.length,
    start + 1,
    "unmounted approval never continues to deposit or withdrawal",
  );
  assert.equal(signed[start].from, alice.address);
  reopened.unmount();
}

async function collateralReadErrors() {
  if (isMainThread) return runWorker("collateralReadErrors");
  failWalletReads = true;
  const mounted = mount(modal());
  await until(
    () =>
      Boolean(
        nodes(mounted.container).find(
          (node) => node.type === "button" && text(node) === "Retry wallet balance",
        ),
      ),
    "wallet read failure is visible",
  );
  assert.ok(!text(mounted.container).includes("Max: 0.00"));
  assert.equal(button(mounted.container, "Deposit USDL").props.disabled, true);
  failWalletReads = false;
  click(button(mounted.container, "Retry wallet balance"));
  await ready(mounted.container);
  assert.match(text(mounted.container), /Max: 12.00/);
  failAccountReads = true;
  mounted.update(modal(false));
  mounted.update(modal(true));
  await until(
    () =>
      Boolean(
        nodes(mounted.container).find(
          (node) => node.type === "button" && text(node) === "Retry account balance",
        ),
      ),
    "account read failure is independent",
  );
  changeAmount(mounted.container, "1");
  assert.equal(
    button(mounted.container, "Deposit USDL").props.disabled,
    false,
    "deposit only needs wallet read",
  );
  click(button(mounted.container, "Withdraw"));
  assert.equal(button(mounted.container, "Withdraw USDL").props.disabled, true);
  assert.equal(button(mounted.container, /^Withdraw all/).props.disabled, true);
  failAccountReads = false;
  click(button(mounted.container, "Retry account balance"));
  await until(
    () => text(mounted.container).includes("Max: 5.00"),
    "account retry reads actual account balance",
  );
  mounted.unmount();
}

async function collateralSessionStaleness() {
  if (isMainThread) return runWorker("collateralSessionStaleness");
  const entered = deferred<void>();
  const release = deferred<void>();
  delayedRead = { address: alice.address, entered, release };
  const mounted = mount(modal());
  await entered.promise;
  await switchAccount(wagmiConfig, { connector: bobConnector });
  await until(() => text(mounted.container).includes("Max: 7.00"), "new account's balance loads");
  release.resolve();
  await new Promise((resolve) => setTimeout(resolve, 70));
  assert.match(text(mounted.container), /Max: 7.00/);
  assert.doesNotMatch(text(mounted.container), /Max: 12.00/);
  await disconnect(wagmiConfig, { connector: bobConnector });
  await disconnect(wagmiConfig, { connector: aliceConnector });
  await until(
    () => text(mounted.container).includes("Connect wallet"),
    "disconnect clears account state",
  );
  assert.doesNotMatch(text(mounted.container), /Max: (7|12).00/);
  mounted.unmount();
  await connect(wagmiConfig, { connector: aliceConnector });
  await connect(wagmiConfig, { connector: bobConnector });
  await switchAccount(wagmiConfig, { connector: aliceConnector });
}

async function collateralLifecycleProof() {
  if (isMainThread) return runWorker("collateralLifecycleProof");
  receiptHold = true;
  allowances.set(lower(alice.address), 0n);
  const mounted = mount(modal());
  await ready(mounted.container);
  changeAmount(mounted.container, "2");
  const start = signed.length;
  click(button(mounted.container, "Deposit USDL"));
  await until(() => signed.length === start + 1, "real approval broadcast");
  const hash = signed[start].hash;
  const persisted = JSON.parse(storage.getItem(contractModule.PENDING_TRANSACTIONS_KEY)!);
  assert.equal(persisted[0].hash, hash, "hash persisted before delayed receipt");
  await switchAccount(wagmiConfig, { connector: bobConnector });
  await until(
    () =>
      text(mounted.container).includes(bob.address) ||
      text(mounted.container).includes("Max: 7.00"),
    "account switch reaches mounted consumer",
  );
  mounted.update(modal(false));
  mounted.update(modal(true));
  assert.equal(button(mounted.container, "Deposit USDL").props.disabled, true);
  receiptHold = false;
  await until(
    () =>
      contractModule.getPendingTransactions().length === 0 &&
      !nodes(mounted.container).find((node) => node.type === "input")?.props.disabled,
    "delayed approval reconciled after account change",
  );
  assert.equal(signed.length, start + 1);
  assert.equal(signed[start].from, alice.address);
  assert.match(text(mounted.container), /Max: 7.00/);
  assert.doesNotMatch(text(mounted.container), /Deposited 2/);
  mounted.unmount();
  await switchAccount(wagmiConfig, { connector: aliceConnector });
}

async function collateralPendingRecovery() {
  if (isMainThread) return runWorker("collateralPendingRecovery");
  receiptHold = true;
  allowances.set(lower(alice.address), 10_000_000n);
  const mounted = mount(modal());
  await ready(mounted.container);
  changeAmount(mounted.container, "1");
  const start = signed.length;
  click(button(mounted.container, "Deposit USDL"));
  await until(() => signed.length === start + 1, "deposit broadcast before receipt");
  const hash = signed[start].hash;
  await until(
    () =>
      contractModule
        .getPendingTransactions()
        .some((record) => record.hash === hash && record.account === alice.address),
    "journal binds chain, account and broadcast hash",
  );
  mounted.unmount();
  // A full module restart discards in-memory guards/listeners while retaining browser storage.
  const paths = [
    "../src/app/modals/DepositWithdrawModal",
    "../src/app/modals/useCollateralRecovery",
    "../src/hooks/perps/useContract",
  ];
  for (const path of paths) delete localRequire.cache[localRequire.resolve(path)];
  collateralModule = localRequire("../src/app/modals/DepositWithdrawModal");
  const restartedContract: typeof contractModule = localRequire("../src/hooks/perps/useContract");
  assert.equal(
    restartedContract.getPendingTransactions().length,
    0,
    "new runtime begins with no in-memory operations",
  );
  const afterRestart = mount(modal());
  await until(
    () => text(afterRestart.container).includes(hash),
    "reopened form recovers durable hash",
  );
  assert.equal(button(afterRestart.container, "Deposit USDL").props.disabled, true);
  assert.equal(signed.length, start + 1);
  const before = requests.filter(
    (request) => request.method === "eth_getTransactionReceipt",
  ).length;
  receiptHold = false;
  await until(
    () =>
      restartedContract.getPendingTransactions().length === 0 &&
      !nodes(afterRestart.container).find((node) => node.type === "input")?.props.disabled,
    "restart resumes receipt reconciliation",
  );
  assert.ok(
    requests.filter((request) => request.method === "eth_getTransactionReceipt").length > before,
  );
  assert.equal(signed.length, start + 1, "recovery never broadcasts a duplicate");
  assert.deepEqual(JSON.parse(storage.getItem(restartedContract.PENDING_TRANSACTIONS_KEY)!), []);
  assert.match(text(afterRestart.container), /Max: 12.00/);
  afterRestart.unmount();
  contractModule = restartedContract;
}

const cases = {
  walletChooserWrongConnection,
  walletConnectionErrors,
  emailRejectionState,
  collateralOverlappingSubmissions,
  collateralReadErrors,
  collateralSessionStaleness,
  collateralLifecycleProof,
  collateralPendingRecovery,
};
let worker: Worker | undefined;
function runWorker(name: keyof typeof cases) {
  if (!worker) {
    worker = new Worker(
      `const { workerData } = require("node:worker_threads"); require(workerData.loader); require(workerData.file)`,
      {
        eval: true,
        workerData: {
          loader: localRequire.resolve("tsx/cjs"),
          file: fileURLToPath(import.meta.url),
        },
      },
    );
    // workerData is a real Node module value, available in the worker bootstrap.
  }
  return new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error(`Proof ${name} exceeded 30 seconds`));
    }, 30_000);
    const onError = (error: Error) => {
      clearTimeout(timeout);
      reject(error);
    };
    worker!.once("error", onError);
    worker!.once("message", (message: { name: string; error?: string }) => {
      clearTimeout(timeout);
      worker!.removeListener("error", onError);
      if (message.name !== name) reject(new Error(`Unexpected proof response ${message.name}`));
      else if (message.error) reject(new Error(message.error));
      else resolve();
    });
    worker!.postMessage(name);
  });
}
if (isMainThread) {
  test(walletChooserWrongConnection);
  test(walletConnectionErrors);
  test(emailRejectionState);
  test(collateralOverlappingSubmissions);
  test(collateralReadErrors);
  test(collateralSessionStaleness);
  test(collateralLifecycleProof);
  test(collateralPendingRecovery);
  after(async () => {
    if (worker) {
      await new Promise<void>((resolve) => {
        worker!.once("message", () => resolve());
        worker!.postMessage("cleanup");
      });
      await worker.terminate();
    }
  });
} else {
  parentPort!.on("message", (name: keyof typeof cases | "cleanup") => {
    void (name === "cleanup" ? cleanup() : cases[name]()).then(
      () => parentPort!.postMessage({ name }),
      (error: unknown) =>
        parentPort!.postMessage({
          name,
          error: error instanceof Error ? error.stack : String(error),
        }),
    );
  });
}

async function cleanup() {
  receiptHold = false;
  queries.clear();
  for (const connection of wagmiConfig?.state.connections.values() ?? []) {
    await disconnect(wagmiConfig, { connector: connection.connector });
  }
  const { getPaxeerWallet } = localRequire(
    "../src/core/wallet/embedded/client",
  ) as typeof import("../src/core/wallet/embedded/client");
  getPaxeerWallet()?.supabase.auth.stopAutoRefresh();
  server.closeAllConnections();
  await new Promise<void>((resolve) => server.close(() => resolve()));
}

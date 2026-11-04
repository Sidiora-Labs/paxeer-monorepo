import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer, type ServerResponse } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { Worker, isMainThread, workerData } from "node:worker_threads";
import * as React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Block } from "../src/types/paxscan/api/block";
import type { Transaction } from "../src/types/paxscan/api/transaction";

// Each proof owns its network clients. Terminating its worker also releases
// viem's process-lifetime RPC ranking timer after the component has unmounted.
async function runProof(finding: string) {
  const worker = new Worker(
    `const { workerData, parentPort } = require('node:worker_threads');
     process.env.TSX_TSCONFIG_PATH = workerData.tsconfig;
     require('tsx/cjs/api').register();
     import('tsx/esm/api').then(({ register }) => {
       register({ tsconfig: workerData.tsconfig });
       return import(workerData.file);
     }).then(m => m.runFixture(workerData.finding)).then(
       () => parentPort.postMessage({ ok: true }),
       error => parentPort.postMessage({ error: error.stack })
     );`,
    {
      eval: true,
      workerData: {
        file: import.meta.url,
        finding,
        tsconfig: fileURLToPath(new URL("../tsconfig.json", import.meta.url)),
      },
    },
  );
  try {
    await new Promise<void>((resolve, reject) => {
      worker.once("error", reject);
      worker.once("exit", (code) => reject(new Error(`Proof exited before reporting: ${code}`)));
      worker.once("message", (message: { ok?: boolean; error?: string }) => {
        if (message.ok) resolve();
        else reject(new Error(message.error));
      });
    });
  } finally {
    await worker.terminate();
  }
}

async function f011_transaction_count_uses_transaction_label() {
  await runProof("f011");
}
async function f012_late_history_merges_and_recovers_after_reopen() {
  await runProof("f012");
}
async function f013_paxeer_transaction_has_no_retired_batch_model() {
  await runProof("f013");
}
if (isMainThread) {
  test(f011_transaction_count_uses_transaction_label);
  test(f012_late_history_merges_and_recovers_after_reopen);
  test(f013_paxeer_transaction_has_no_retired_batch_model);
}

// A memory host for React's real reconciler, already shipped by the declared
// @react-three/fiber dependency. Hooks, effects, queries and components are real;
// only the render destination is an in-memory tree rather than a browser DOM.
type Node = { type: string; props: Record<string, unknown>; children: Node[]; text?: string };
type Renderer = {
  createContainer(container: Node, tag: number, hydration: null, strict: boolean,
    concurrent: null, prefix: string, recover: (error: Error) => void, transitions: null): object;
  updateContainer(element: React.ReactNode, root: object, parent: null, callback: null): void;
  flushPassiveEffects(): boolean;
};
function memoryRoot() {
  const require = createRequire(import.meta.url);
  const fiberRequire = createRequire(require.resolve("@react-three/fiber"));
  const reconciler = fiberRequire("react-reconciler") as (host: object) => Renderer;
  const append = (parent: Node, child: Node) => {
    const existing = parent.children.indexOf(child);
    if (existing >= 0) parent.children.splice(existing, 1);
    parent.children.push(child);
  };
  const remove = (parent: Node, child: Node) => {
    const index = parent.children.indexOf(child);
    assert.ok(index >= 0, "React removes an attached node");
    parent.children.splice(index, 1);
  };
  const insert = (parent: Node, child: Node, before: Node) => {
    const existing = parent.children.indexOf(child);
    if (existing >= 0) parent.children.splice(existing, 1);
    const index = parent.children.indexOf(before);
    assert.ok(index >= 0, "React inserts before an attached node");
    parent.children.splice(index, 0, child);
  };
  const renderer = reconciler({
    now: Date.now,
    supportsMutation: true,
    supportsPersistence: false,
    supportsHydration: false,
    isPrimaryRenderer: true,
    getRootHostContext: () => ({}),
    getChildHostContext: (context: object) => context,
    getPublicInstance: (node: Node) => node,
    prepareForCommit: () => null,
    resetAfterCommit() {},
    createInstance: (type: string, props: Record<string, unknown>): Node => ({ type, props, children: [] }),
    createTextInstance: (text: string): Node => ({ type: "#text", props: {}, children: [], text }),
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
    commitUpdate: (node: Node, _payload: boolean, _type: string, _old: object, props: Record<string, unknown>) => { node.props = props; },
    commitTextUpdate: (node: Node, _old: string, text: string) => { node.text = text; },
    clearContainer: (node: Node) => { node.children.length = 0; },
    detachDeletedInstance() {},
    scheduleTimeout: setTimeout,
    cancelTimeout: clearTimeout,
    noTimeout: -1,
    getCurrentEventPriority: () => 16,
  });
  const tree: Node = { type: "root", props: {}, children: [] };
  const root = renderer.createContainer(tree, 0, null, false, null, "", (error) => { throw error; }, null);
  return {
    tree,
    render(element: React.ReactNode) {
      renderer.updateContainer(element, root, null, null);
      renderer.flushPassiveEffects();
    },
  };
}
function descendants(node: Node): Node[] {
  return [node, ...node.children.flatMap(descendants)];
}
function text(node: Node): string {
  return node.text ?? node.children.map(text).join("");
}
async function until(check: () => boolean, description: string) {
  const deadline = Date.now() + 6000;
  while (!check()) {
    assert.ok(Date.now() < deadline, description);
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}
const hash = (n: number) => `0x${n.toString(16).padStart(64, "0")}`;
const address: Block["miner"] = {
  hash: `0x${"1".repeat(40)}`, implementations: null, name: null,
  is_contract: false, is_verified: null, ens_domain_name: null,
  private_tags: null, watchlist_names: null, public_tags: null,
};
function block(height: number): Block {
  return {
    height, hash: hash(height), parent_hash: hash(height - 1),
    timestamp: "2026-01-01T00:00:00.000Z", transactions_count: 3,
    internal_transactions_count: 0, miner: address, gas_used: "63000",
    gas_limit: "30000000", nonce: "0x0000000000000000", burnt_fees: null,
    priority_fee: null, extra_data: "0x", state_root: hash(1), rewards: [],
    gas_target_percentage: null, gas_used_percentage: 0.21,
    burnt_fees_percentage: null, type: "block", transaction_fees: null,
    uncles_hashes: [],
  };
}
const transaction: Transaction = {
  hash: hash(901), from: address, to: address, created_contract: null,
  result: "success", confirmations: 2, status: "ok", block_number: 106,
  timestamp: "2026-01-01T00:00:00.000Z", confirmation_duration: null,
  value: "1000000000000000000", fee: { type: "actual", value: "63000" },
  gas_price: "1", type: 0, gas_used: "21000", gas_limit: "21000",
  max_fee_per_gas: null, max_priority_fee_per_gas: null, priority_fee: null,
  base_fee_per_gas: null, transaction_burnt_fee: null, nonce: 7, position: 1,
  revert_reason: null, raw_input: "0x", decoded_input: null,
  token_transfers: [], token_transfers_overflow: false, exchange_rate: null,
  historic_exchange_rate: null, method: null, transaction_types: ["coin_transfer"],
  transaction_tag: null, actions: [], has_error_in_internal_transactions: false,
};
function rpcBlock(height: number) {
  const source = block(height);
  return {
    number: `0x${height.toString(16)}`, hash: source.hash, parentHash: source.parent_hash,
    timestamp: "0x6955b900", gasUsed: "0xf618", gasLimit: "0x1c9c380",
    miner: address.hash, nonce: source.nonce, difficulty: "0x0", totalDifficulty: "0x0",
    size: "0x200", extraData: "0x", stateRoot: hash(1), receiptsRoot: hash(2),
    transactionsRoot: hash(3), sha3Uncles: hash(4), logsBloom: `0x${"0".repeat(512)}`,
    transactions: [hash(901), hash(902), hash(903)], uncles: [], baseFeePerGas: "0x1",
  };
}
function json(response: ServerResponse, value: unknown) {
  response.writeHead(200, { "Content-Type": "application/json" });
  response.end(JSON.stringify(value));
}

export async function runFixture(finding: string) {
  assert.equal(workerData.finding, finding);
  let pendingSeed: ServerResponse | undefined;
  let head = 106;
  let blockPolls = 0;
  const requests: string[] = [];
  const server = createServer((request, response) => {
    requests.push(request.url ?? "");
    if (request.url === "/api/v2/blocks") {
      assert.equal(pendingSeed, undefined);
      pendingSeed = response;
    } else if (request.url === `/api/v2/blocks/${hash(106)}`) {
      json(response, block(106));
    } else if (request.url === `/api/v2/transactions/${transaction.hash}`) {
      json(response, transaction);
    } else if (request.url === "/rpc") {
      void (async () => {
        let body = "";
        for await (const chunk of request) body += chunk;
        const rpc = JSON.parse(body) as { id: number; method: string; params: string[] };
        assert.ok(["net_listening", "eth_getBlockByNumber"].includes(rpc.method), rpc.method);
        if (rpc.method === "eth_getBlockByNumber") blockPolls += 1;
        json(response, { jsonrpc: "2.0", id: rpc.id, result: rpc.method === "net_listening"
          ? true : rpcBlock(rpc.params[0] === "latest" ? head : Number(BigInt(rpc.params[0]))) });
      })().catch((error) => response.destroy(error));
    } else {
      response.writeHead(404);
      response.end();
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = `${origin}/rpc`;
  Object.defineProperty(globalThis, "React", { value: React, configurable: true });
  const client = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const roots: ReturnType<typeof memoryRoot>[] = [];
  const clients: QueryClient[] = [];
  function mount(Component: React.FunctionComponent) {
    const cache = client();
    clients.push(cache);
    const root = memoryRoot();
    roots.push(root);
    root.render(React.createElement(QueryClientProvider, { client: cache }, React.createElement(Component)));
    return root;
  }
  try {
    if (finding === "f011") {
      const { buildUrl, paxscanFetch } = await import("../src/core/paxscan/api");
      const { BlockRow } = await import("../src/widgets/explorer/BlockRow");
      const received = await paxscanFetch<Block>(buildUrl("general:block", { height_or_hash: hash(106) }));
      let selected: Block | undefined;
      const root = mount(() => React.createElement(BlockRow, { block: received, onSelect: (value) => { selected = value; } }));
      const label = descendants(root.tree).find((node) => node.type === "div" && text(node) === "Transactions3");
      assert.ok(label, "f011: the producer's all-transaction count is labeled Transactions");
      assert.doesNotMatch(text(root.tree), /Fills/);
      const button = descendants(root.tree).find((node) => node.type === "button");
      assert.ok(button);
      (button.props.onClick as () => void)();
      assert.deepEqual(selected, received);
      assert.ok(requests.includes(`/api/v2/blocks/${hash(106)}`));
    } else if (finding === "f012") {
      const { LiveBlockStream } = await import("../src/widgets/explorer/LiveBlockStream");
      // Enable the real hook's browser lifecycle after module initialization;
      // no DOM API is used by the RPC watcher or by this memory renderer.
      Object.defineProperty(globalThis, "window", { value: {}, configurable: true });
      let selected: Block | undefined;
      const Feed = () => React.createElement(LiveBlockStream, { maxBlocks: 4, onSelectBlock: (value) => { selected = value; } });
      const heights = (root: ReturnType<typeof memoryRoot>) => descendants(root.tree)
        .filter((node) => node.type === "button")
        .map((node) => Number(String(node.props["aria-label"]).replace("Open block #", "")));
      function release(items: Block[]) {
        assert.ok(pendingSeed, "REST history request is outstanding");
        const response = pendingSeed;
        pendingSeed = undefined;
        json(response, { items, next_page_params: null });
      }
      const first = mount(Feed);
      await until(() => !!pendingSeed && heights(first).includes(106), "live head must arrive before history resolves");
      assert.deepEqual(heights(first), [106]);
      release([block(104), { ...block(106), hash: hash(999) }, block(105), block(105), block(103), block(102)]);
      await until(() => heights(first).length === 4, "late REST seed must populate the feed");
      assert.deepEqual(heights(first), [106, 105, 104, 103], "f012: seed preserves live head, deduplicates heights/hashes, sorts and caps");
      const liveButton = descendants(first.tree).find((node) => node.props["aria-label"] === "Open block #106");
      assert.ok(liveButton);
      (liveButton.props.onClick as () => void)();
      assert.equal(selected?.hash, hash(106), "live data takes precedence over the seed's competing height");
      assert.equal(selected?.transactions_count, 3, "the real EVM adapter counted the RPC transaction hashes");
      head = 107;
      await until(() => heights(first)[0] === 107, "live updates continue after seeding");
      assert.deepEqual(heights(first), [107, 106, 105, 104]);
      first.render(null);
      assert.equal(first.tree.children.length, 0, "the component stops");
      const stoppedPolls = blockPolls;
      await new Promise((resolve) => setTimeout(resolve, 1100));
      assert.equal(blockPolls, stoppedPolls, "unmount unsubscribes from block polling");
      clients[0].clear();
      head = 108;
      const reopened = mount(Feed);
      await until(() => !!pendingSeed && heights(reopened).includes(108), "reopened component receives live head while refetching history");
      release([block(105), block(107), block(106), block(104), block(107)]);
      await until(() => heights(reopened).length === 4, "reopened feed recovers history");
      assert.deepEqual(heights(reopened), [108, 107, 106, 105], "recovery uses fresh history and preserves the head received during reopening");
      assert.equal(requests.filter((path) => path === "/api/v2/blocks").length, 2);
    } else if (finding === "f013") {
      const { TxDetailPanel } = await import("../src/widgets/explorer/TxDetailPanel");
      // Inspect the production panel content within its real Sheet context,
      // omitting only the DOM portal that has no destination in a memory host.
      function PanelContent() {
        const panel = TxDetailPanel({ txHash: transaction.hash, onClose() {} });
        const content = React.Children.only(panel.props.children) as React.ReactElement<{ children: React.ReactNode }>;
        return React.cloneElement(panel, {}, content.props.children);
      }
      const root = mount(PanelContent);
      await until(() => text(root.tree).includes(transaction.value), "transaction REST query reaches the panel fields");
      assert.match(text(root.tree), /Paxeer Transaction/);
      assert.doesNotMatch(text(root.tree), /Layer X|Batch Context|batch-settlement/);
      assert.match(text(root.tree), /#106/);
      assert.ok(requests.includes(`/api/v2/transactions/${transaction.hash}`));
      const source = readFileSync(new URL("../src/widgets/explorer/TxDetailPanel.tsx", import.meta.url), "utf8");
      assert.doesNotMatch(source, /batch-settlement|BatchFillsTable|Batch Context|Batch context|uniform clearing|PoFQ/);
    } else {
      assert.fail(`Unknown finding ${finding}`);
    }
  } finally {
    for (const root of roots) root.render(null);
    for (const cache of clients) cache.clear();
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

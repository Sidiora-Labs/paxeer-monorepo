import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(new URL("../marketplace/package.json", import.meta.url));
const token = "marketplace-local-integration";
const hex = (bytes) => `0x${Buffer.from(bytes).toString("hex")}`;

async function loadMarketplace() {
  const { deriveFromSeed } = await import("../../../agent/sdk/typescript/dist/src/index.js");
  const marketplace = await import("../marketplace/deploy-payload.mjs");
  return { ...marketplace, account: deriveFromSeed(new Uint8Array(32).fill(7)) };
}

async function openNode(t) {
  const ganache = require("ganache");
  const { account } = await loadMarketplace();
  const provider = ganache.provider({
    chain: { chainId: 125, time: new Date("2026-01-01T00:00:00Z") },
    miner: { timestampIncrement: 1 },
    wallet: {
      accounts: [{ secretKey: hex(account.evmPrivateKey), balance: "0x3635c9adc5dea00000" }],
    },
    logging: { quiet: true },
  });
  assert.equal(await provider.request({ method: "eth_chainId", params: [] }), "0x7d");
  const exchanges = [];
  const server = createServer(async (request, response) => {
    const exchange = {
      method: request.method,
      path: request.url,
      authorization: request.headers.authorization,
      contentType: request.headers["content-type"],
    };
    exchanges.push(exchange);
    if (request.method !== "POST" || request.url !== "/") {
      response.writeHead(404).end();
      return;
    }
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    exchange.request = message;
    let answer;
    try {
      const result = await provider.request({ method: message.method, params: message.params });
      answer = { jsonrpc: "2.0", id: message.id, result };
    } catch (error) {
      answer = { jsonrpc: "2.0", id: message.id, error: { code: error.code, message: error.message } };
    }
    exchange.response = answer;
    response.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify(answer));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  let closed = false;
  const close = async () => {
    if (closed) return;
    closed = true;
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    await provider.disconnect();
  };
  t.after(close);
  const endpoint = `http://127.0.0.1:${server.address().port}`;
  const rpc = async (method, params) => {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "content-type": "application/json", authorization: `Bearer ${token}` },
      body: JSON.stringify({ jsonrpc: "2.0", id: 9, method, params }),
    });
    assert.equal(response.status, 200);
    const answer = await response.json();
    assert.equal(answer.jsonrpc, "2.0");
    assert.equal(answer.id, 9);
    assert.equal(Object.hasOwn(answer, "error"), false, JSON.stringify(answer));
    assert.ok(Object.hasOwn(answer, "result"));
    return answer.result;
  };
  return { provider, endpoint, exchanges, rpc, close };
}

function anchorOptions(endpoint, account) {
  return { endpoint, token, did: account.did, evmAddress: account.evmAddress };
}

function assertAnchorExchanges(exchanges, block, nonce, account) {
  assert.deepEqual(exchanges.map(({ method, path, authorization, contentType, request }) => ({
    method, path, authorization, contentType, request,
  })), [
    {
      method: "POST", path: "/", authorization: `Bearer ${token}`, contentType: "application/json",
      request: { jsonrpc: "2.0", id: 1, method: "eth_getBlockByNumber", params: ["latest", false] },
    },
    {
      method: "POST", path: "/", authorization: `Bearer ${token}`, contentType: "application/json",
      request: { jsonrpc: "2.0", id: 1, method: "eth_getTransactionCount", params: [account.evmAddress, "pending"] },
    },
  ]);
  assert.deepEqual(exchanges[0].response, { jsonrpc: "2.0", id: 1, result: block });
  assert.deepEqual(exchanges[1].response, { jsonrpc: "2.0", id: 1, result: nonce });
}

async function marketplaceRootAnchorDrivesEvmDeploymentAndCalls(t) {
  const { account, LIFECYCLE_WINDOW_MS, marketplaceCallRequest, readLifecycleAnchor } = await loadMarketplace();
  const node = await openNode(t);
  const initial = await readLifecycleAnchor(anchorOptions(`${node.endpoint}/rpc`, account));
  const block = await node.provider.request({ method: "eth_getBlockByNumber", params: ["latest", false] });
  assertAnchorExchanges(node.exchanges, block, "0x0", account);
  assert.deepEqual(initial, {
    identitySequence: 0n,
    notBefore: BigInt(block.timestamp) * 1000n,
    expiresAt: BigInt(block.timestamp) * 1000n + LIFECYCLE_WINDOW_MS,
    previousStateRoot: block.stateRoot.slice(2),
  });
  // Deploy executable EVM code that stores the first calldata word and returns the calldata.
  const runtime = "600035600055366000600037366000f3";
  const runtimeLength = (runtime.length / 2).toString(16).padStart(2, "0");
  const deploymentHash = await node.rpc("eth_sendTransaction", [{
    from: account.evmAddress,
    nonce: `0x${BigInt(initial.identitySequence).toString(16)}`,
    data: `0x60${runtimeLength}600c60003960${runtimeLength}6000f3${runtime}`,
    gas: "0x493e0", type: "0x2", maxFeePerGas: "0x77359400", maxPriorityFeePerGas: "0x3b9aca00",
  }]);
  const deployed = await node.rpc("eth_getTransactionReceipt", [deploymentHash]);
  assert.equal(deployed.status, "0x1");
  assert.match(deployed.contractAddress, /^0x[0-9a-f]{40}$/u);
  assert.equal(await node.rpc("eth_getCode", [deployed.contractAddress, "latest"]), `0x${runtime}`);

  let previousRoot = initial.previousStateRoot;
  for (const [index, action] of ["list", "buy"].entries()) {
    const start = node.exchanges.length;
    const anchor = await readLifecycleAnchor(anchorOptions(node.endpoint, account));
    const head = await node.provider.request({ method: "eth_getBlockByNumber", params: ["latest", false] });
    assertAnchorExchanges(node.exchanges.slice(start), head, `0x${index + 1}`, account);
    assert.equal(anchor.identitySequence, BigInt(index + 1));
    assert.equal(anchor.notBefore, BigInt(head.timestamp) * 1000n);
    assert.equal(anchor.expiresAt, anchor.notBefore + LIFECYCLE_WINDOW_MS);
    assert.equal(anchor.previousStateRoot, head.stateRoot.slice(2));
    assert.notEqual(anchor.previousStateRoot, previousRoot);
    assert.equal(Object.isFrozen(anchor), true);
    previousRoot = anchor.previousStateRoot;
    const request = marketplaceCallRequest({
      action, listingId: "11".repeat(32), asset: "22".repeat(32), seller: "33".repeat(32),
      price: "2500", receiptDigest: "44".repeat(32),
    });
    const transactionHash = await node.rpc("eth_sendTransaction", [{
      from: account.evmAddress, to: deployed.contractAddress,
      nonce: `0x${anchor.identitySequence.toString(16)}`, data: hex(request.calldata),
      gas: "0x493e0", type: "0x2", maxFeePerGas: "0x77359400", maxPriorityFeePerGas: "0x3b9aca00",
    }]);
    const receipt = await node.rpc("eth_getTransactionReceipt", [transactionHash]);
    assert.equal(receipt.status, "0x1");
    const transaction = await node.rpc("eth_getTransactionByHash", [transactionHash]);
    assert.equal(transaction.type, "0x2");
    assert.equal(BigInt(transaction.nonce), anchor.identitySequence);
    assert.equal(transaction.from.toLowerCase(), account.evmAddress.toLowerCase());
    assert.equal(transaction.input, hex(request.calldata));
    assert.equal(await node.rpc("eth_call", [{ to: deployed.contractAddress, data: hex(request.calldata) }, "latest"]), hex(request.calldata));
    assert.equal(await node.rpc("eth_getStorageAt", [deployed.contractAddress, "0x0", "latest"]), hex(request.calldata.subarray(0, 32)));
  }
  const finalAnchor = await readLifecycleAnchor(anchorOptions(node.endpoint, account));
  assert.equal(finalAnchor.identitySequence, 3n);
  assert.notEqual(finalAnchor.previousStateRoot, previousRoot);
  assert.equal(node.exchanges.every((entry) => entry.method === "POST" && entry.path === "/"), true);
  assert.equal(node.exchanges.some((entry) => entry.request?.method.startsWith("px_")), false);
}

async function marketplaceAnchorRequiresEvmSender(t) {
  const { account, readLifecycleAnchor } = await loadMarketplace();
  const node = await openNode(t);
  for (const evmAddress of [undefined, null, "", account.did, "0x1234", `0x${"z".repeat(40)}`]) {
    await assert.rejects(readLifecycleAnchor({ ...anchorOptions(node.endpoint, account), evmAddress }), {
      name: "LayerXApplicationStateError", state: "refused", message: "invalid_signing_key_evm_address",
    });
  }
  assert.deepEqual(node.exchanges, []);
}

async function marketplaceAnchorReportsUnavailableNode(t) {
  const { account, readLifecycleAnchor } = await loadMarketplace();
  const node = await openNode(t);
  await node.close();
  await assert.rejects(readLifecycleAnchor(anchorOptions(node.endpoint, account)), {
    name: "LayerXApplicationStateError", state: "unknown", message: "identity_sequence_unreachable",
  });
}

test(async function repair2231349(t) {
  await t.test(marketplaceRootAnchorDrivesEvmDeploymentAndCalls);
  await t.test(marketplaceAnchorRequiresEvmSender);
  await t.test(marketplaceAnchorReportsUnavailableNode);
});

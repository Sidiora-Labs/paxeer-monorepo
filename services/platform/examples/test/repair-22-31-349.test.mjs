import assert from "node:assert/strict";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import test from "node:test";

const fixture = JSON.parse(await readFile(new URL("./marketplace-chain-125.fixture.json", import.meta.url), "utf8"));
const token = "marketplace-local-integration";
const hex = (bytes) => `0x${Buffer.from(bytes).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const heads = fixture.exchanges.filter((entry) => entry.request.method === "eth_getBlockByNumber");
const nonces = fixture.exchanges.filter((entry) => entry.request.method === "eth_getTransactionCount");

async function loadMarketplace() {
  const sdk = await import("../../../agent/sdk/typescript/dist/src/index.js");
  const marketplace = await import("../marketplace/deploy-payload.mjs");
  return { ...sdk, ...marketplace, account: sdk.deriveFromSeed(new Uint8Array(32).fill(7)) };
}

async function openNode(t, { entries = fixture.exchanges, malformedReply } = {}) {
  const exchanges = [];
  const failures = [];
  let cursor = 0;
  const server = createServer(async (request, response) => {
    const exchange = {
      method: request.method,
      path: request.url,
      authorization: request.headers.authorization,
      contentType: request.headers["content-type"],
    };
    exchanges.push(exchange);
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      assert.equal(request.headers.authorization, `Bearer ${token}`);
      assert.equal(request.headers["content-type"], "application/json");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      exchange.request = message;
      assert.deepEqual(Object.keys(message).sort(), ["id", "jsonrpc", "method", "params"]);
      assert.equal(message.jsonrpc, "2.0");
      assert.ok(message.id === 1 || message.id === 9);
      const recorded = entries[cursor];
      assert.ok(recorded, `unrecorded RPC exchange ${cursor}: ${message.method}`);
      assert.equal(message.method, recorded.request.method);
      // Ethereum addresses are case-insensitive; the production envelope is also asserted verbatim below.
      const params = message.method === "eth_getTransactionCount"
        ? [message.params[0].toLowerCase(), ...message.params.slice(1)] : message.params;
      assert.deepEqual(params, recorded.request.params);
      const answer = malformedReply !== undefined && cursor === entries.length - 1
        ? malformedReply : { ...recorded.response, id: message.id };
      cursor += 1;
      exchange.response = answer;
      response.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify(answer));
    } catch (error) {
      failures.push(error.message);
      response.writeHead(500, { "content-type": "application/json" }).end(JSON.stringify({
        jsonrpc: "2.0", id: exchange.request?.id ?? null,
        error: { code: -32603, message: error.message },
      }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  let closed = false;
  const close = async () => {
    if (closed) return;
    closed = true;
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  };
  t.after(async () => {
    await close();
    assert.deepEqual(failures, [], "the RPC peer only serves recorded requests");
  });
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
  const assertConsumed = () => assert.equal(cursor, entries.length, "every recorded exchange was consumed");
  return { endpoint, exchanges, rpc, close, assertConsumed };
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

async function submitRecordedTransaction(node, sdk, recording, anchor, to, data) {
  const raw = bytes(recording.raw);
  const transaction = sdk.decodeRawEip1559(raw);
  assert.equal(transaction.chainId, 125n);
  assert.equal(transaction.nonce, anchor.identitySequence);
  assert.equal(transaction.maxPriorityFeePerGas, 1_000_000_000n);
  assert.equal(transaction.maxFeePerGas, 2_000_000_000n);
  assert.equal(transaction.gasLimit, 300_000n);
  assert.equal(transaction.value, 0n);
  assert.deepEqual(transaction.accessList, []);
  assert.equal(hex(transaction.to), to);
  assert.equal(hex(transaction.data), data);
  assert.equal(sdk.recoverEip1559Sender(raw), sdk.account.evmAddress);
  assert.equal(hex(sdk.keccak256(raw)), recording.hash);
  const hash = await node.rpc("eth_sendRawTransaction", [recording.raw]);
  assert.equal(hash, recording.hash);
  return hash;
}

async function marketplaceRootAnchorDrivesEvmDeploymentAndCalls(t) {
  const sdk = await loadMarketplace();
  const { account, LIFECYCLE_WINDOW_MS, marketplaceCallRequest, readLifecycleAnchor } = sdk;
  const node = await openNode(t);
  assert.equal(fixture.source.chain.chainId, 125);
  assert.equal(fixture.sender, account.evmAddress.toLowerCase());
  assert.equal(await node.rpc("eth_chainId", []), "0x7d");
  const initial = await readLifecycleAnchor(anchorOptions(`${node.endpoint}/rpc`, account));
  const block = heads[0].response.result;
  assertAnchorExchanges(node.exchanges.slice(1), block, "0x0", account);
  assert.deepEqual(initial, {
    identitySequence: 0n,
    notBefore: BigInt(block.timestamp) * 1000n,
    expiresAt: BigInt(block.timestamp) * 1000n + LIFECYCLE_WINDOW_MS,
    previousStateRoot: block.stateRoot.slice(2),
  });
  const runtime = "600035600055366000600037366000f3";
  const runtimeLength = (runtime.length / 2).toString(16).padStart(2, "0");
  const deploymentData = `0x60${runtimeLength}600c60003960${runtimeLength}6000f3${runtime}`;
  assert.equal(fixture.runtime, `0x${runtime}`);
  assert.equal(fixture.deploymentData, deploymentData);
  assert.deepEqual(fixture.transactions.map(({ action }) => action), ["deploy", "list", "buy"]);
  const deploymentHash = await submitRecordedTransaction(node, sdk, fixture.transactions[0], initial, "0x", deploymentData);
  const deployed = await node.rpc("eth_getTransactionReceipt", [deploymentHash]);
  assert.equal(deployed.status, "0x1");
  assert.match(deployed.contractAddress, /^0x[0-9a-f]{40}$/u);
  assert.equal(await node.rpc("eth_getCode", [deployed.contractAddress, "latest"]), `0x${runtime}`);
  const deployment = await node.rpc("eth_getTransactionByHash", [deploymentHash]);
  assert.equal(deployment.type, "0x2");
  assert.equal(BigInt(deployment.nonce), initial.identitySequence);
  assert.equal(deployment.from.toLowerCase(), account.evmAddress.toLowerCase());
  assert.equal(deployment.to, null);
  assert.equal(deployment.input, deploymentData);

  let previousRoot = initial.previousStateRoot;
  for (const [index, action] of ["list", "buy"].entries()) {
    const start = node.exchanges.length;
    const anchor = await readLifecycleAnchor(anchorOptions(node.endpoint, account));
    const head = heads[index + 1].response.result;
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
    const transactionHash = await submitRecordedTransaction(
      node, sdk, fixture.transactions[index + 1], anchor, deployed.contractAddress, hex(request.calldata),
    );
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
  const start = node.exchanges.length;
  const finalAnchor = await readLifecycleAnchor(anchorOptions(node.endpoint, account));
  assertAnchorExchanges(node.exchanges.slice(start), heads[3].response.result, "0x3", account);
  assert.equal(finalAnchor.identitySequence, 3n);
  assert.notEqual(finalAnchor.previousStateRoot, previousRoot);
  assert.equal(node.exchanges.every((entry) => entry.method === "POST" && entry.path === "/"), true);
  assert.equal(node.exchanges.some((entry) => entry.request?.method.startsWith("px_")), false);
  node.assertConsumed();
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

async function marketplaceAnchorRejectsMalformedEnvelopes(t) {
  const { account, readLifecycleAnchor } = await loadMarketplace();
  const block = heads[0].response.result;
  const envelope = { jsonrpc: "2.0", id: 1, result: block };
  const cases = [
    { name: "wrong JSON-RPC version", reply: { ...envelope, jsonrpc: "1.0" } },
    { name: "wrong response ID", reply: { ...envelope, id: 2 } },
    { name: "string response ID", reply: { ...envelope, id: "1" } },
    { name: "missing result", reply: { jsonrpc: "2.0", id: 1 } },
    { name: "result alongside error", reply: { ...envelope, error: { code: -32603, message: "refused" } } },
    { name: "RPC error", reply: { jsonrpc: "2.0", id: 1, error: { code: -32603, message: "refused" } } },
    { name: "null block", reply: { ...envelope, result: null }, message: "invalid_program_state_block" },
    { name: "array block", reply: { ...envelope, result: [] }, message: "invalid_program_state_block" },
    { name: "missing state root", reply: { ...envelope, result: { timestamp: block.timestamp } }, message: "invalid_program_state_block" },
    { name: "short state root", reply: { ...envelope, result: { ...block, stateRoot: "0x1234" } }, message: "invalid_program_state_block" },
    { name: "non-hex state root", reply: { ...envelope, result: { ...block, stateRoot: `0x${"z".repeat(64)}` } }, message: "invalid_program_state_block" },
    { name: "missing timestamp", reply: { ...envelope, result: { stateRoot: block.stateRoot } }, message: "invalid_program_state_timestamp" },
    { name: "noncanonical timestamp", reply: { ...envelope, result: { ...block, timestamp: "0x01" } }, message: "invalid_program_state_timestamp" },
    { name: "timestamp overflow", reply: { ...envelope, result: { ...block, timestamp: "0x10000000000000000" } }, message: "invalid_program_state_timestamp" },
    { name: "millisecond overflow", reply: { ...envelope, result: { ...block, timestamp: "0xffffffffffffffff" } }, message: "invalid_program_state_timestamp_ms" },
    { name: "expiry overflow", reply: { ...envelope, result: { ...block, timestamp: `0x${((0xffffffffffffffffn - 300_000n) / 1000n + 1n).toString(16)}` } }, message: "invalid_program_state_timestamp_ms" },
    { name: "missing block method", reply: { jsonrpc: "2.0", id: 1, error: { code: -32601, message: "method not found" } }, state: "refused", message: "eth_getBlockByNumber: JSON-RPC error -32601" },
    { name: "missing nonce method", nonce: true, reply: { jsonrpc: "2.0", id: 1, error: { code: -32601, message: "method not found" } }, state: "refused", message: "eth_getTransactionCount: JSON-RPC error -32601" },
    { name: "malformed nonce envelope", nonce: true, reply: { jsonrpc: "2.0", id: 2, result: "0x0" } },
    ...[null, 0, "0x", "0x00", "0x-1", "0xA", "0x10000000000000000"].map((result) => ({
      name: `invalid nonce ${JSON.stringify(result)}`, nonce: true,
      reply: { jsonrpc: "2.0", id: 1, result }, message: "invalid_identity_next_sequence",
    })),
  ];
  for (const scenario of cases) {
    await t.test(scenario.name, async (t) => {
      const node = await openNode(t, {
        entries: scenario.nonce ? [heads[0], nonces[0]] : [heads[0]],
        malformedReply: scenario.reply,
      });
      await assert.rejects(readLifecycleAnchor(anchorOptions(node.endpoint, account)), {
        name: "LayerXApplicationStateError", state: scenario.state ?? "unknown",
        message: scenario.message ?? "identity_sequence_rpc_refused",
      });
      node.assertConsumed();
    });
  }
}

async function repair2231349(t) {
  await t.test(marketplaceRootAnchorDrivesEvmDeploymentAndCalls);
  await t.test(marketplaceAnchorRequiresEvmSender);
  await t.test(marketplaceAnchorReportsUnavailableNode);
  await t.test(marketplaceAnchorRejectsMalformedEnvelopes);
}

test(repair2231349);

import assert from "node:assert/strict";
import { once } from "node:events";
import * as http from "node:http";
import { test } from "node:test";

import { keccak256, type Eip1559Transaction } from "../src/evm-transaction.js";
import { decodeJsonRpcResponse, JsonRpcClient, JsonRpcError, type Commitment } from "../src/rpc.js";

const address = `0x${"11".repeat(20)}`;
const did = `did:layerx:${"22".repeat(32)}`;
const assetId = "33".repeat(32);
const storageKey = `0x${"44".repeat(32)}`;
const identities = {
  evm_address: address, pax_address: null, layerx_did: did,
  layerx_account: "55".repeat(32), bound: true,
};
const balance = { asset_id: assetId, denom: null, custody: null, paxeer: null, layerx: { balance: "17" } };
const balances = { account: identities, balances: [balance], joined_limit: "1024" };
const assets = {
  assets: [
    { asset_id: assetId, layerx: { symbol: "USD" }, paxeer: null },
    { asset_id: "66".repeat(32), layerx: null, paxeer: null },
  ],
  joined_limit: "1024",
};

interface Exchange {
  readonly method: string;
  readonly params: readonly unknown[];
  readonly result: unknown;
}

async function withRpcServer(exchanges: readonly Exchange[], run: (client: JsonRpcClient) => Promise<void>): Promise<void> {
  const requests: unknown[] = [];
  const server = http.createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as Record<string, unknown>;
      const exchange = exchanges[requests.length];
      requests.push({ verb: request.method, path: request.url, ...body });
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result: exchange?.result ?? null }));
    } catch {
      response.writeHead(400);
      response.end();
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  try {
    const listener = server.address();
    assert(listener !== null && typeof listener === "object");
    await run(new JsonRpcClient(`http://127.0.0.1:${listener.port}`));
    assert.deepEqual(requests, exchanges.map((exchange, index) => ({
      verb: "POST", path: "/rpc", jsonrpc: "2.0", id: String(index + 1),
      method: exchange.method, params: exchange.params,
    })));
  } finally {
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
}

async function JsonRpcClient_reads_use_only_eth_and_px_methods(): Promise<void> {
  const call: Eip1559Transaction = {
    chainId: 125n, nonce: 3n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 30000n, to: new Uint8Array(20).fill(0x11), value: 7n,
    data: Uint8Array.from([0, 0xab, 0xff]),
    accessList: [{ address: new Uint8Array(20).fill(0x11), storageKeys: [new Uint8Array(32).fill(0x44)] }],
  };
  const block = { number: "0x7", hash: `0x${"77".repeat(32)}` };
  const network = { network_id: "test", latest_committed_height: "7" };
  const proof = { address, accountProof: [], storageProof: [] };
  const exchanges: Exchange[] = [
    { method: "px_getAccount", params: [address], result: { account: identities, paxeer: null, layerx: null } },
    { method: "eth_getBalance", params: [address, "pending"], result: "0x10000000000000001" },
    { method: "px_getBalances", params: [did], result: balances },
    { method: "eth_getTransactionCount", params: [address, "pending"], result: "0x3" },
    { method: "eth_getTransactionCount", params: [address, "pending"], result: "0x3" },
    { method: "eth_getBlockByNumber", params: ["0x7", false], result: block },
    { method: "px_getNetwork", params: [], result: network },
    { method: "px_listAssets", params: [], result: assets },
    { method: "px_listAssets", params: [], result: assets },
    { method: "px_listAssets", params: [], result: assets },
    { method: "px_listAssets", params: [], result: assets },
    { method: "eth_estimateGas", params: [{
      type: "0x2", chainId: "0x7d", nonce: "0x3", maxPriorityFeePerGas: "0x1", maxFeePerGas: "0x2",
      gas: "0x7530", to: address, value: "0x7", data: "0x00abff",
      accessList: [{ address, storageKeys: [storageKey] }],
    }, "pending"], result: "0x5208" },
    { method: "eth_getProof", params: [address, [storageKey], "0x7"], result: proof },
    { method: "px_getBalances", params: [did], result: balances },
    { method: "px_getBalances", params: [did], result: balances },
  ];
  await withRpcServer(exchanges, async client => {
    assert.deepEqual(await client.getAccount(address), exchanges[0]!.result);
    assert.equal(await client.getBalance(address), 0x10000000000000001n);
    assert.deepEqual(await client.getBalances(did), balances);
    assert.equal(await client.getSequence(address), 3n);
    assert.equal(await client.getIdentitySequence(address), 3n);
    assert.deepEqual(await client.getBatchHeader("0x7"), block);
    assert.deepEqual(await client.getNodeInfo(), network);
    assert.deepEqual((await client.listAssets()).assets.map(asset => asset.assetId), [assetId, "66".repeat(32)]);
    const page = await client.listAssetsPage(undefined, 1);
    assert.equal(page.assets.length, 1);
    assert.equal(page.nextCursor, assetId);
    const lastPage = await client.listAssetsPage(assetId, 1);
    assert.equal(lastPage.assets[0]!.assetId, "66".repeat(32));
    assert.equal(lastPage.nextCursor, null);
    assert.equal((await client.getAsset(assetId)).assetId, assetId);
    assert.equal(await client.estimateFee(call), 21000n);
    assert.throws(() => client.estimateFee({ ...call, data: new Uint8Array(524289) }), /Invalid transaction length/u);
    assert.deepEqual(await client.getProof(address, [storageKey], "0x7"), proof);
    const wallet = client.wallet("00".repeat(32));
    assert.deepEqual(await wallet.accounts(did), balances);
    assert.equal((await wallet.balance(did, assetId)).assetId, assetId);
    assert.equal("getCheckpoint" in client, false);
    assert.equal("getActivityStatus" in client, false);
    assert.equal("getReceipt" in client, false);
    assert.equal("sendActivity" in client, false);
    const commitments: readonly Commitment[] = ["submitted", "receipted", "committed"];
    assert.equal(commitments.length, 3);
  });
  assert(exchanges.every(exchange => /^(eth_|px_)/u.test(exchange.method)));
}

async function sendRawTransaction_posts_eth_sendRawTransaction_and_returns_the_hash(): Promise<void> {
  const rawHex = `02f8627d80010282520894${"11".repeat(20)}8080c080a0${"00".repeat(31)}01a0${"00".repeat(31)}02`;
  const raw = Buffer.from(rawHex, "hex");
  const txHash = `0x${Buffer.from(keccak256(raw)).toString("hex")}`;
  const receipt = { txHash, rung: "receipted" };
  await withRpcServer([
    { method: "eth_sendRawTransaction", params: [`0x${rawHex}`], result: txHash },
    { method: "pax_sequencerReceipt", params: [txHash], result: receipt },
    { method: "eth_sendRawTransaction", params: [`0x${rawHex}`], result: { hash: txHash } },
    { method: "eth_sendRawTransaction", params: [`0x${rawHex}`], result: "0x1234" },
  ], async client => {
    assert.equal(await client.sendRawTransaction(raw), txHash);
    assert.deepEqual(await client.sequencerReceipt(txHash), receipt);
    await assert.rejects(client.sendRawTransaction(raw), /Invalid RPC hash/u);
    await assert.rejects(client.sendRawTransaction(raw), /Invalid RPC hash/u);
    assert.throws(() => client.sendRawTransaction(new Uint8Array()), /Invalid submission/u);
    assert.throws(() => client.sendRawTransaction(new Uint8Array(524289)), /Invalid submission/u);
    assert.throws(() => client.sequencerReceipt("0x1234"), /Invalid RPC hash/u);
  });
  assert.throws(() => decodeJsonRpcResponse({ jsonrpc: "2.0", id: "other", result: txHash }, "1"));
  assert.throws(() => decodeJsonRpcResponse({ jsonrpc: "2.0", id: "1", result: txHash }, "1"));
  assert.throws(() => decodeJsonRpcResponse({ jsonrpc: "2.0", id: "1", error: { code: -32000, message: "rejected" } }, "1"), JsonRpcError);
}

async function identity_sequence_reads_the_pending_transaction_count(): Promise<void> {
  const values = ["0x0", "0x2a", "0x20000000000001", "0x01", "42", 42, {}, null];
  await withRpcServer(values.map(result => ({
    method: "eth_getTransactionCount", params: [address, "pending"], result,
  })), async client => {
    assert.equal(await client.getIdentitySequence(address), 0n);
    assert.equal(await client.getIdentitySequence(address), 42n);
    assert.equal(await client.getIdentitySequence(address), 9007199254740993n);
    for (const unused of values.slice(3)) {
      await assert.rejects(client.getIdentitySequence(address), /Invalid RPC quantity/u);
    }
    assert.throws(() => client.getIdentitySequence(did), /Invalid EVM address/u);
  });
}

test(JsonRpcClient_reads_use_only_eth_and_px_methods);
test(sendRawTransaction_posts_eth_sendRawTransaction_and_returns_the_hash);
test(identity_sequence_reads_the_pending_transaction_count);

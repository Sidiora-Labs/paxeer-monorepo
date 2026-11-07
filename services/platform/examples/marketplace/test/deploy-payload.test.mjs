import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { resolve } from "node:path";
import test from "node:test";
import { LayerXApplicationStateError } from "../../support/runtime.mjs";
import { decodeNativeProgramDeploy } from "@sidiora/layerx-sdk";
import {
  BUY_OPERATION,
  IDENTITY_SEQUENCE_SELECTOR,
  LIFECYCLE_WINDOW_MS,
  LIST_OPERATION,
  accountNameForDid,
  canonicalNativeProgramDeploy,
  marketplaceCallRequest,
  readLifecycleAnchor,
} from "../deploy-payload.mjs";

const root = resolve(import.meta.dirname, "../../../../..");
const wasm = Uint8Array.from(await readFile(resolve(root, "programs/fixtures/pay5/payments-merchant.wasm")));
const codeHash = createHash("sha256").update(wasm).digest("hex");
const programId = "3b7e1c40a9d25f8e6041bd9c73a5e8f210c4d6b98e07f35a1c2d4e6f80a9b3c5";
const listingId = "c41d8a2f60e9b7530d1a8c4f92b6e70a35d8c1f4a6b90e23d7c58f1a4b6d9e02";
const asset = "0100000000000000000000000000000000000000000000000000000000000000";
const seller = "7f2a91c604db85e3a0c7f16d928e40b5c73a61f8d20e94b7c5a3106df82b49ec";
const receiptDigest = "a5c3901e7bd426f80c19a3f56d82b4e70931cf6a8d25e40b7c1936af52d8e0b4";
const did = "did:layerx:marketplace-reference";

const deployment = canonicalNativeProgramDeploy({ programId, abiVersion: 2, codeHash, wasm });
const decoded = decodeNativeProgramDeploy(Uint8Array.from(deployment.payload));

assert.equal(Buffer.from(decoded.programId).toString("hex"), programId);
assert.equal(decoded.guestAbi, 2);
assert.equal(decoded.policy, 0);
assert.ok(Buffer.from(decoded.authority).equals(Buffer.alloc(32)));
assert.equal(Buffer.from(decoded.newHash).toString("hex"), codeHash);
assert.ok(Buffer.from(decoded.wasm).equals(Buffer.from(wasm)));
assert.equal(decoded.interface, undefined);
assert.equal(Buffer.from(deployment.value.programId).toString("hex"), programId);

assert.throws(
  () => canonicalNativeProgramDeploy({ programId, abiVersion: 2, codeHash: "00".repeat(32), wasm }),
  (error) => error.state === "refused" && error.message === "program_deployment_is_not_canonical",
);
assert.throws(
  () => canonicalNativeProgramDeploy({ programId, abiVersion: 4, codeHash, wasm }),
  (error) => error.state === "refused" && error.message === "program_build_reported_unsupported_guest_abi",
);
assert.throws(
  () => canonicalNativeProgramDeploy({ programId, abiVersion: 2, codeHash, wasm: wasm.slice(4) }),
  (error) => error.state === "refused" && error.message === "program_deployment_is_not_canonical",
);

const listRequest = marketplaceCallRequest({ action: "list", listingId, asset, seller, price: "2500" });
assert.equal(listRequest.calldata.length, 1 + 32 + 32 + 32 + 16);
assert.equal(listRequest.calldata[0], LIST_OPERATION);
assert.equal(listRequest.calldata.subarray(1, 33).toString("hex"), listingId);
assert.equal(listRequest.calldata.subarray(33, 65).toString("hex"), asset);
assert.equal(listRequest.calldata.subarray(65, 97).toString("hex"), seller);
assert.equal(listRequest.calldata.subarray(97).readBigUInt64BE(8), 2500n);
assert.deepEqual([...listRequest.capabilities], ["shared-storage-read", "shared-storage-write", "emit-event"]);

const buyRequest = marketplaceCallRequest({ action: "buy", listingId, asset, seller, price: "2500", receiptDigest });
assert.equal(buyRequest.calldata.length, 1 + 32 + 32);
assert.equal(buyRequest.calldata[0], BUY_OPERATION);
assert.equal(buyRequest.calldata.subarray(1, 33).toString("hex"), listingId);
assert.equal(buyRequest.calldata.subarray(33).toString("hex"), receiptDigest);
assert.deepEqual([...buyRequest.capabilities], [
  "shared-storage-read",
  "shared-storage-write",
  "emit-event",
  `receipt-read:${receiptDigest}`,
  `transfer402:${asset}:${seller}:2500`,
]);

assert.throws(
  () => marketplaceCallRequest({ action: "list", listingId, asset, seller, price: "0" }),
  (error) => error.state === "refused" && error.message === "invalid_marketplace_price",
);
assert.throws(
  () => marketplaceCallRequest({ action: "browse", listingId, asset, seller, price: "1" }),
  (error) => error.state === "refused" && error.message === "unsupported_marketplace_action",
);

assert.equal(accountNameForDid(did), `agent:${did}:main`);
assert.throws(
  () => accountNameForDid("marketplace-reference"),
  (error) => error.state === "refused" && error.message === "invalid_signing_key_did",
);

const transcript = JSON.parse(await readFile(new URL(
  "../../test/marketplace-chain-125.fixture.json", import.meta.url,
), "utf8"));
const heads = transcript.exchanges.filter(({ request }) => request.method === "eth_getBlockByNumber");
const nonces = transcript.exchanges.filter(({ request }) => request.method === "eth_getTransactionCount");

async function rootListener(t, entries, { reply, status = 200 } = {}) {
  const requests = [];
  const failures = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      const recorded = entries[requests.length];
      requests.push({
        method: request.method, path: request.url,
        authorization: request.headers.authorization,
        contentType: request.headers["content-type"], message,
      });
      assert.ok(recorded, "no unexpected RPC calls");
      assert.deepEqual(requests.at(-1), {
        method: "POST", path: "/", authorization: "Bearer reference-token",
        contentType: "application/json", message: { ...recorded.request, id: 1 },
      });
      const body = reply !== undefined && requests.length === entries.length
        ? reply : { ...recorded.response, id: 1 };
      response.writeHead(requests.length === entries.length ? status : 200, { "content-type": "application/json" })
        .end(JSON.stringify(body));
    } catch (error) {
      failures.push(error);
      response.writeHead(500, { "content-type": "application/json" }).end(JSON.stringify({
        jsonrpc: "2.0", id: 1, error: { code: -32603, message: error.message },
      }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    assert.deepEqual(failures, []);
    assert.equal(requests.length, entries.length, "all expected calls reached the retained root listener");
  });
  return { endpoint: `http://127.0.0.1:${server.address().port}/rpc`, requests };
}

function anchorOptions(endpoint) {
  return { endpoint, token: "reference-token", did, evmAddress: transcript.sender };
}

async function marketplaceAnchorConsumesRecordedNodeResponses(t) {
  assert.equal(IDENTITY_SEQUENCE_SELECTOR, "pending");
  assert.equal(transcript.source.chain.chainId, 125);
  for (const [index, environment] of ["emulator", "hosted"].entries()) {
    await t.test(environment, async (t) => {
      const node = await rootListener(t, [heads[index], nonces[index]]);
      const anchor = await readLifecycleAnchor(anchorOptions(node.endpoint));
      const block = heads[index].response.result;
      const timestamp = BigInt(block.timestamp) * 1000n;
      assert.deepEqual(anchor, {
        identitySequence: BigInt(nonces[index].response.result),
        notBefore: timestamp,
        expiresAt: timestamp + LIFECYCLE_WINDOW_MS,
        previousStateRoot: block.stateRoot.slice(2),
      });
      assert.equal(Object.isFrozen(anchor), true);
      assert.deepEqual(node.requests.map(({ message }) => message), [
        { jsonrpc: "2.0", id: 1, method: "eth_getBlockByNumber", params: ["latest", false] },
        { jsonrpc: "2.0", id: 1, method: "eth_getTransactionCount", params: [transcript.sender, "pending"] },
      ]);
    });
  }
}

async function marketplaceAnchorRefusesInvalidIdentity(t) {
  const node = await rootListener(t, []);
  for (const evmAddress of [undefined, "", did, "0x1234"]) {
    await assert.rejects(readLifecycleAnchor({ ...anchorOptions(node.endpoint), evmAddress }), {
      name: "LayerXApplicationStateError", state: "refused", message: "invalid_signing_key_evm_address",
    });
  }
  await assert.rejects(readLifecycleAnchor({ ...anchorOptions(node.endpoint), did: "marketplace-reference" }), {
    name: "LayerXApplicationStateError", state: "refused", message: "invalid_signing_key_did",
  });
  assert.deepEqual(node.requests, []);
}

async function marketplaceAnchorRejectsUntrustedNodeResponses(t) {
  const block = heads[0].response.result;
  const cases = [
    { name: "mismatched sequence envelope", nonce: true, reply: { jsonrpc: "2.0", id: 2, result: "0x1" }, message: "identity_sequence_rpc_refused" },
    { name: "unauthenticated snapshot cannot replace nonce", nonce: true,
      reply: { jsonrpc: "2.0", id: 1, result: { did, next_sequence: "61", verification: "settlement_anchored" } },
      message: "invalid_identity_next_sequence" },
    { name: "unknown state shape", reply: { jsonrpc: "2.0", id: 1, result: { network_mode: "mirror" } }, message: "invalid_program_state_block" },
    { name: "malformed root", reply: { jsonrpc: "2.0", id: 1, result: { ...block, stateRoot: "0x1234" } }, message: "invalid_program_state_block" },
    { name: "noncanonical timestamp", reply: { jsonrpc: "2.0", id: 1, result: { ...block, timestamp: "0x01" } }, message: "invalid_program_state_timestamp" },
    { name: "noncanonical nonce", nonce: true, reply: { jsonrpc: "2.0", id: 1, result: "0x01" }, message: "invalid_identity_next_sequence" },
    { name: "unavailable state", status: 503, reply: { jsonrpc: "2.0", id: 1, error: { code: -32000, message: "state unavailable" } }, state: "pending", message: "identity_sequence_http_503" },
    { name: "refused nonce", nonce: true, status: 404, reply: { jsonrpc: "2.0", id: 1, error: { code: -32000, message: "not found" } }, state: "refused", message: "identity_sequence_http_404" },
    ...[false, true].map((nonce) => ({
      name: nonce ? "missing nonce method" : "missing block method", nonce,
      reply: { jsonrpc: "2.0", id: 1, error: { code: -32601, message: "method not found" } },
      state: "refused", message: `${nonce ? "eth_getTransactionCount" : "eth_getBlockByNumber"}: JSON-RPC error -32601`,
    })),
  ];
  for (const scenario of cases) {
    await t.test(scenario.name, async (t) => {
      const node = await rootListener(t, scenario.nonce ? [heads[0], nonces[0]] : [heads[0]], scenario);
      await assert.rejects(readLifecycleAnchor(anchorOptions(node.endpoint)), (error) => {
        assert.ok(error instanceof LayerXApplicationStateError);
        assert.equal(error.state, scenario.state ?? "unknown");
        assert.equal(error.message, scenario.message);
        return true;
      });
    });
  }
}

test(marketplaceAnchorConsumesRecordedNodeResponses);
test(marketplaceAnchorRefusesInvalidIdentity);
test(marketplaceAnchorRejectsUntrustedNodeResponses);

import "./support/source-loader.mjs";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  LayerXApplicationStateError,
  SequencerReceiptClient,
  applyEndpointOverride,
  receiptKeysFromEnvironment,
} from "../support/runtime.mjs";

import { bytes, hex, nodeReceipt, portableReceipt, recordedNode } from "./support/node-receipts.mjs";

const signedReceipt = nodeReceipt;
const receiptListener = (t, result) => recordedNode(t).then((service) => {
  service.result = result;
  return service;
});

function hasState(state) {
  return (error) => {
    assert(error instanceof LayerXApplicationStateError);
    assert.equal(error.state, state);
    return true;
  };
}

async function sequencerReceiptClientReturnsVerifiedReceipt(t) {
  const vector = signedReceipt();
  const { raw, txHash, receipt, key } = vector;
  const service = await receiptListener(t, structuredClone(vector.receiptJson));
  const client = new SequencerReceiptClient(service.rpcUrl, [key]);
  const proof = await client.resolve(raw);
  assert.deepEqual(proof, portableReceipt(vector));
  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [key]);
  assert.equal(verified.height, 1000n);
  assert.equal(hex(verified.receiptDigest), vector.receiptDigest);
  assert.equal(verified.receipt.effects[0].kind, 1);
  assert.deepEqual(service.requests, [{ method: "POST", path: "/", message: {
    jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [txHash],
  } }]);
  assert.deepEqual(await client.resolve(hex(raw)), proof);
  for (const pin of [
    { ...key, keyId: new Uint8Array(32).fill(1) },
    { ...key, publicKey: new Uint8Array(32).fill(2) },
    { ...key, first_height: 1001n },
    { ...key, first_height: 999n, last_height: 999n },
  ]) {
    await assert.rejects(new SequencerReceiptClient(service.rpcUrl, [pin]).resolve(raw), hasState("refused"));
  }
  assert.deepEqual((await new SequencerReceiptClient(service.rpcUrl,
    [{ ...key, first_height: 1000n, last_height: 1000n }]).resolve(raw)).receiptKey,
  { ...proof.receiptKey, firstHeight: "0x3e8", lastHeight: "0x3e8" });
  assert.deepEqual(await new SequencerReceiptClient(service.rpcUrl,
    [{ ...key, publicKey: new Uint8Array(32).fill(2) }, key]).resolve(raw), proof);
  const mutable = { ...key, keyId: Buffer.from(key.keyId), publicKey: Buffer.from(key.publicKey) };
  const pinnedClient = new SequencerReceiptClient(service.rpcUrl, [mutable]);
  mutable.keyId.fill(0);
  mutable.publicKey.fill(0);
  assert.deepEqual(await pinnedClient.resolve(raw), proof);
  const corrupted = Buffer.from(receipt);
  corrupted[corrupted.length - 1] ^= 1;
  service.result = { txHash, rung: "receipted", receipt: hex(corrupted) };
  await assert.rejects(client.resolve(raw), hasState("refused"));
  const wrongRaw = Buffer.from(raw);
  wrongRaw[wrongRaw.length - 1] ^= 1;
  const alteredVector = { ...vector, txHash: hex(keccak256(wrongRaw)) };
  const alteredService = await recordedNode(t, alteredVector);
  alteredService.result = { ...vector.receiptJson, txHash: alteredVector.txHash };
  await assert.rejects(new SequencerReceiptClient(alteredService.rpcUrl, [key]).resolve(wrongRaw), hasState("refused"));
  service.result = { txHash: `0x${"00".repeat(32)}`, rung: "receipted", receipt: hex(receipt) };
  await assert.rejects(client.resolve(raw), hasState("unknown"));
  service.result = { ...vector.receiptJson, rung: "committed", blockNumber: "0x3e8", blockHash: `0x${"aa".repeat(32)}` };
  assert.equal((await client.resolve(raw)).verificationLevel, "receipted");
}

async function sequencerReceiptClientReportsSubmittedAsPending(t) {
  const { raw, txHash, key } = signedReceipt();
  const service = await receiptListener(t, { txHash, rung: "submitted" });
  const client = new SequencerReceiptClient(service.rpcUrl, [key]);
  await assert.rejects(client.resolve(raw), hasState("pending"));
  assert.equal(service.requests.length, 1);
  assert.equal(service.requests[0].message.method, "pax_sequencerReceipt");
  assert.deepEqual(service.requests[0].message.params, [txHash]);
  service.error = { code: -32000, message: "receipt refused" };
  await assert.rejects(client.resolve(raw), hasState("refused"));
  service.error = undefined;
  service.status = 503;
  await assert.rejects(client.resolve(raw), hasState("unknown"));
  await service.close();
  await assert.rejects(client.resolve(raw), hasState("unknown"));
}

function receiptKeysFromEnvironmentRefusesInvertedRange(t) {
  const name = "LAYERX_RUNTIME_TEST_RECEIPT_KEYS_JSON";
  const previous = process.env[name];
  t.after(() => {
    if (previous === undefined) delete process.env[name];
    else process.env[name] = previous;
  });
  const { key } = signedReceipt();
  const entry = { keyId: hex(key.keyId), publicKey: hex(key.publicKey), firstHeight: "10", lastHeight: "9" };
  process.env[name] = JSON.stringify([entry]);
  assert.throws(() => receiptKeysFromEnvironment(name), /invalid_receipt_key_range/u);
  process.env[name] = JSON.stringify([{ ...entry, firstHeight: "0x3e8", lastHeight: "0x7cf" }]);
  assert.deepEqual(receiptKeysFromEnvironment(name), [key]);
  process.env[name] = JSON.stringify([{ ...entry, firstHeight: 7, lastHeight: 7 }]);
  assert.equal(receiptKeysFromEnvironment(name)[0].first_height, 7n);
  for (const invalid of [[], {}, [{ ...entry, firstHeight: -1 }], [{ ...entry, lastHeight: "18446744073709551616" }]]) {
    process.env[name] = JSON.stringify(invalid);
    assert.throws(() => receiptKeysFromEnvironment(name));
  }
  delete process.env[name];
  assert.throws(() => receiptKeysFromEnvironment(name), /missing_layerx_runtime_test_receipt_keys_json/u);
  assert.throws(() => receiptKeysFromEnvironment("invalid binding"), /invalid_environment_binding/u);
  const previousEndpoint = process.env.LAYERX_EXAMPLE_ENDPOINT;
  t.after(() => {
    if (previousEndpoint === undefined) delete process.env.LAYERX_EXAMPLE_ENDPOINT;
    else process.env.LAYERX_EXAMPLE_ENDPOINT = previousEndpoint;
  });
  process.env.LAYERX_EXAMPLE_ENDPOINT = "http://127.0.0.1:1234/";
  assert.equal(applyEndpointOverride("beta", { rpcUrl: "https://example.com" }).rpcUrl, "http://127.0.0.1:1234/");
}

test(sequencerReceiptClientReturnsVerifiedReceipt);
test(sequencerReceiptClientReportsSubmittedAsPending);
test(receiptKeysFromEnvironmentRefusesInvertedRange);

async function runtimeProofMatchesCrossSdkPortableFixture(t) {
  const vector = nodeReceipt();
  const service = await recordedNode(t, vector);
  const proof = await new SequencerReceiptClient(service.rpcUrl, [vector.key]).resolve(vector.raw);
  const shared = JSON.parse(readFileSync(new URL(
    "../../../../sdk/platform/conformance/fixtures/vector-v5-portable.json", import.meta.url,
  ), "utf8")).bundles[0];
  assert.deepEqual(JSON.parse(JSON.stringify(proof)), shared);
  assert.notEqual(proof.receiptDigest, hex(keccak256(vector.receipt)));
  assert.deepEqual(proof.receiptJson, vector.receiptJson);
}

test(runtimeProofMatchesCrossSdkPortableFixture);

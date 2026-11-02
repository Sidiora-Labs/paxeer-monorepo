import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  LayerXApplicationStateError,
  SequencerReceiptClient,
  applyEndpointOverride,
  receiptKeysFromEnvironment,
} from "../support/runtime.mjs";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();

function uint(value, width) {
  assert(value >= 0n && value < 1n << BigInt(width * 8));
  return Buffer.from(value.toString(16).padStart(width * 2, "0"), "hex");
}

function bounded(value) {
  return Buffer.concat([uint(BigInt(value.length), 4), value]);
}

function signedReceipt() {
  const pair = generateKeyPairSync("ed25519");
  const keyId = Buffer.alloc(32, 0x55);
  const publicKey = pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32);
  const vectors = JSON.parse(readFileSync(new URL(
    "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
  ), "utf8"));
  const raw = bytes(vectors.cases[0].rawTransaction);
  const txHash = hex(keccak256(raw));
  const zero = Buffer.alloc(32);
  const prefix = Buffer.concat([
    uint(2n, 2), uint(0x5201n, 2), uint(2n, 2), bounded(bytes(txHash)), uint(1n, 8),
    bounded(zero), bounded(sha256(raw)), bounded(sha256(Buffer.alloc(0))), uint(0n, 4), uint(0n, 4),
    uint(0n, 16), bounded(uint(7n, 32)), uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2),
    bounded(zero), uint(0n, 16), bounded(zero), uint(0n, 16), uint(0n, 16), uint(0n, 8),
    bounded(zero), uint(0n, 16), uint(0n, 16), bounded(zero), bounded(zero), bounded(keyId), uint(1n, 8),
  ]);
  const digest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), prefix, Buffer.of(0)]));
  const receipt = Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, digest, pair.privateKey))]);
  return {
    raw, txHash, receipt,
    key: { keyId: Uint8Array.from(keyId), publicKey: Uint8Array.from(publicKey), first_height: 1n, last_height: 10n },
  };
}

async function receiptListener(t, result) {
  const requests = [];
  const service = { result, error: undefined, status: 200, requests };
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      requests.push({ method: request.method, path: request.url, message: JSON.parse(Buffer.concat(chunks).toString("utf8")) });
      const message = requests.at(-1).message;
      response.writeHead(service.status, { "content-type": "application/json" });
      response.end(JSON.stringify({
        jsonrpc: "2.0", id: message.id,
        ...(service.error === undefined ? { result: service.result } : { error: service.error }),
      }));
    } catch (error) {
      response.writeHead(500);
      response.end(String(error));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  service.rpcUrl = `http://127.0.0.1:${server.address().port}/rpc`;
  service.close = () => new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  t.after(async () => {
    if (server.listening) await service.close();
  });
  return service;
}

function hasState(state) {
  return (error) => {
    assert(error instanceof LayerXApplicationStateError);
    assert.equal(error.state, state);
    return true;
  };
}

async function sequencerReceiptClientReturnsVerifiedReceipt(t) {
  const { raw, txHash, receipt, key } = signedReceipt();
  const service = await receiptListener(t, { txHash, rung: "receipted", receipt: hex(receipt) });
  const client = new SequencerReceiptClient(service.rpcUrl, [key]);
  const proof = await client.resolve(raw);
  assert.deepEqual(proof, {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
    rawTransaction: hex(raw), signedReceipt: hex(receipt), receiptDigest: hex(keccak256(receipt)),
    receiptKey: { keyId: hex(key.keyId), publicKey: hex(key.publicKey), firstHeight: "0x1", lastHeight: "0xa" },
  });
  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [key]);
  assert.equal(verified.height, 7n);
  assert.deepEqual(service.requests, [{ method: "POST", path: "/rpc", message: {
    jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [txHash],
  } }]);
  assert.deepEqual(await client.resolve(hex(raw)), proof);
  for (const pin of [
    { ...key, keyId: new Uint8Array(32).fill(1) },
    { ...key, publicKey: new Uint8Array(32).fill(2) },
    { ...key, first_height: 8n },
    { ...key, last_height: 6n },
  ]) {
    await assert.rejects(new SequencerReceiptClient(service.rpcUrl, [pin]).resolve(raw), hasState("refused"));
  }
  assert.deepEqual((await new SequencerReceiptClient(service.rpcUrl,
    [{ ...key, first_height: 7n, last_height: 7n }]).resolve(raw)).receiptKey,
  { ...proof.receiptKey, firstHeight: "0x7", lastHeight: "0x7" });
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
  service.result = { txHash: hex(keccak256(wrongRaw)), rung: "receipted", receipt: hex(receipt) };
  await assert.rejects(client.resolve(wrongRaw), hasState("refused"));
  service.result = { txHash: `0x${"00".repeat(32)}`, rung: "receipted", receipt: hex(receipt) };
  await assert.rejects(client.resolve(raw), hasState("unknown"));
  service.result = { txHash, rung: "committed", receipt: hex(receipt) };
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
  process.env[name] = JSON.stringify([{ ...entry, firstHeight: "0x1", lastHeight: "0xa" }]);
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
  assert.equal(applyEndpointOverride("beta", { rpcUrl: "https://example.com/rpc" }).rpcUrl, "http://127.0.0.1:1234/rpc");
}

test(sequencerReceiptClientReturnsVerifiedReceipt);
test(sequencerReceiptClientReportsSubmittedAsPending);
test(receiptKeysFromEnvironmentRefusesInvertedRange);

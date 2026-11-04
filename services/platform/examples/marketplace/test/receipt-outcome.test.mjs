import assert from "node:assert/strict";
import { createHash, generateKeyPairSync } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { keccak256 } from "@sidiora/layerx-sdk";
import { LayerXApplicationStateError, SequencerReceiptClient } from "../../support/runtime.mjs";
import { verifyProgramOutcome } from "../receipt-outcome.mjs";

const bytes = (value) => Buffer.from(value.slice(2), "hex");
const vectors = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));

function signedReceipt(id = "V1-layerxprograms-call") {
  assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const vector = vectors.cases.find((entry) => entry.id === id);
  assert.ok(vector, `missing producer transcript ${id}`);
  assert.equal(vector.expected.verifier, "accepts");
  assert.equal(vector.receiptJson.receipt, vector.signedReceipt);
  assert.equal(vector.receiptJson.rung, "receipted");
  return {
    raw: vector.rawTransaction,
    txHash: vector.receiptJson.txHash,
    receipt: bytes(vector.signedReceipt),
    result: vector.receiptJson,
    digest: vector.receiptDigest.slice(2),
    key: {
      keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
      first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
    },
  };
}

async function receiptListener(t, signed) {
  const service = { signed, requests: [], failures: [] };
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      service.requests.push({ method: request.method, path: request.url, message });
      assert.equal(request.headers["content-type"], "application/json");
      assert.deepEqual(Object.keys(message).sort(), ["id", "jsonrpc", "method", "params"]);
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.id, 1);
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/rpc");
      assert.deepEqual(message.params, [service.signed.txHash]);
      let result;
      if (message.method === "eth_getRawTransactionByHash") result = service.signed.raw;
      else {
        assert.equal(message.method, "pax_sequencerReceipt");
        result = service.signed.result;
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      service.failures.push(error);
      response.writeHead(500).end(String(error));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    assert.deepEqual(service.failures, []);
  });
  service.rpcUrl = `http://127.0.0.1:${server.address().port}/rpc`;
  return service;
}

function canonicalReceiptDigest(signed) {
  const unsigned = Buffer.concat([signed.receipt.subarray(0, -69), Buffer.of(0)]);
  const digest = createHash("sha256").update(Buffer.from("LXP/v1/receipt\0")).update(unsigned).digest("hex");
  assert.equal(digest, signed.digest, "canonical digest matches the real producer's signing digest");
  assert.notEqual(digest, Buffer.from(keccak256(signed.receipt)).toString("hex"));
  return digest;
}

function assertReceiptRequests(service) {
  assert.deepEqual(service.requests.map(({ message }) => message), [
    { jsonrpc: "2.0", id: 1, method: "eth_getRawTransactionByHash", params: [service.signed.txHash] },
    { jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [service.signed.txHash] },
  ]);
}

async function programOutcomeCompletesOnSuccessfulReceipt(t) {
  const signed = signedReceipt();
  const service = await receiptListener(t, signed);
  const client = new SequencerReceiptClient(service.rpcUrl, [signed.key]);
  const output = { ok: true, data: { deployment: { transaction_hash: signed.txHash } } };
  assert.deepEqual(await verifyProgramOutcome(output, client), {
    state: "completed",
    receiptDigest: canonicalReceiptDigest(signed),
    rung: "receipted",
    transactionHash: signed.txHash,
    resultCode: 0,
    output,
  });
  assertReceiptRequests(service);
  service.signed = signedReceipt("V2-layerxledger-revert");
  service.requests.length = 0;
  const failedOutput = { data: [{ txHash: service.signed.txHash }] };
  assert.equal(service.signed.result.status, "0x0");
  const failed = await verifyProgramOutcome(failedOutput,
    new SequencerReceiptClient(service.rpcUrl, [service.signed.key]));
  assert.deepEqual(failed, {
    state: "refused",
    receiptDigest: canonicalReceiptDigest(service.signed),
    rung: "receipted",
    transactionHash: service.signed.txHash,
    resultCode: 1,
    output: failedOutput,
  });
  assertReceiptRequests(service);
}

async function programOutcomeRefusesReceiptUnderUnpinnedKey(t) {
  const signed = signedReceipt();
  const service = await receiptListener(t, signed);
  const output = { ok: true, data: { transactionHash: signed.txHash, state: "completed" } };
  for (const key of [
    { ...signed.key, keyId: Buffer.alloc(32, 0x66) },
    { ...signed.key, publicKey: generateKeyPairSync("ed25519").publicKey.export({ type: "spki", format: "der" }).subarray(-32) },
  ]) {
    await assert.rejects(verifyProgramOutcome(output, new SequencerReceiptClient(service.rpcUrl, [key])), (error) => {
      assert(error instanceof LayerXApplicationStateError);
      assert.equal(error.state, "refused");
      assert.equal(error.message, "sequencer_receipt_verification_failed");
      return true;
    });
  }
  assert.deepEqual(service.requests.map(({ message }) => message.method), [
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
  ]);
}

test(programOutcomeCompletesOnSuccessfulReceipt);
test(programOutcomeRefusesReceiptUnderUnpinnedKey);

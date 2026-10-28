import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { keccak256 } from "@sidiora/layerx-sdk";
import { LayerXApplicationStateError, SequencerReceiptClient } from "../../support/runtime.mjs";
import { verifyProgramOutcome } from "../receipt-outcome.mjs";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const vectors = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));

function signedReceipt(resultCode = 0) {
  const vector = vectors.cases[0];
  const pair = generateKeyPairSync("ed25519");
  const keyId = Buffer.alloc(32, 0x55);
  const publicKey = pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32);
  const receipt = bytes(vector.signedReceipt);
  const signatureBlockLength = 1 + 4 + 64;
  const prefix = receipt.subarray(0, receipt.length - signatureBlockLength);
  keyId.copy(prefix, prefix.length - 8 - 32);
  const resultCodeOffset = 6 + (4 + 32) + 8 + 3 * (4 + 32);
  prefix.writeInt32BE(resultCode, resultCodeOffset);
  const digest = createHash("sha256").update(Buffer.concat([
    Buffer.from("LXP/v1/receipt\0"), prefix, Buffer.of(0),
  ])).digest();
  sign(null, digest, pair.privateKey).copy(receipt, receipt.length - 64);
  return {
    raw: vector.rawTransaction,
    txHash: vector.receiptJson.txHash,
    receipt,
    key: { keyId, publicKey, first_height: 1000n, last_height: 1999n },
  };
}

async function receiptListener(t, signed) {
  const service = { signed, requests: [] };
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      service.requests.push({ method: request.method, path: request.url, message });
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/rpc");
      assert.deepEqual(message.params, [service.signed.txHash]);
      let result;
      if (message.method === "eth_getRawTransactionByHash") result = service.signed.raw;
      else {
        assert.equal(message.method, "pax_sequencerReceipt");
        result = { txHash: service.signed.txHash, rung: "receipted", receipt: hex(service.signed.receipt), status: "0x1" };
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      response.writeHead(500).end(String(error));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(() => new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve())));
  service.rpcUrl = `http://127.0.0.1:${server.address().port}/rpc`;
  return service;
}

async function programOutcomeCompletesOnSuccessfulReceipt(t) {
  const signed = signedReceipt();
  const service = await receiptListener(t, signed);
  const client = new SequencerReceiptClient(service.rpcUrl, [signed.key]);
  const output = { ok: true, data: { deployment: { transaction_hash: signed.txHash } } };
  assert.deepEqual(await verifyProgramOutcome(output, client), {
    state: "completed",
    receiptDigest: hex(keccak256(signed.receipt)),
    rung: "receipted",
    status: 1,
    output,
  });
  assert.deepEqual(service.requests.map(({ message }) => message), [
    { jsonrpc: "2.0", id: 1, method: "eth_getRawTransactionByHash", params: [signed.txHash] },
    { jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [signed.txHash] },
  ]);
  service.signed = signedReceipt(1);
  const failed = await verifyProgramOutcome({ data: [{ txHash: service.signed.txHash }] },
    new SequencerReceiptClient(service.rpcUrl, [service.signed.key]));
  assert.equal(failed.state, "refused");
  assert.equal(failed.status, 0);
  assert.equal(failed.rung, "receipted");
  assert.equal(failed.receiptDigest, hex(keccak256(service.signed.receipt)));
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

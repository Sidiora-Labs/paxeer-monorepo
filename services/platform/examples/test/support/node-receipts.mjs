import "./source-loader.mjs";
import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";

const corpus = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
assert.equal(corpus.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");

export const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
export const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;

export function nodeReceipt(id = "V1-pax-transfer") {
  const vector = structuredClone(corpus.cases.find((entry) => entry.id === id));
  assert.ok(vector, `missing producer recording ${id}`);
  return {
    ...vector,
    raw: bytes(vector.rawTransaction), receipt: bytes(vector.signedReceipt), txHash: vector.receiptJson.txHash,
    key: {
      keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
      first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
    },
  };
}

export function portableReceipt(vector) {
  return {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
    rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
    receiptDigest: vector.receiptDigest, receiptJson: structuredClone(vector.receiptJson),
    receiptKey: {
      keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
      firstHeight: `0x${vector.key.first_height.toString(16)}`,
      lastHeight: `0x${vector.key.last_height.toString(16)}`,
    },
  };
}

export async function recordedNode(t, vector = nodeReceipt()) {
  const service = { result: structuredClone(vector.receiptJson), status: 200, requests: [], failures: [] };
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      service.requests.push({ method: request.method, path: request.url, message });
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      assert.deepEqual(Object.keys(message).sort(), ["id", "jsonrpc", "method", "params"]);
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.id, 1);
      assert.deepEqual(message.params, [vector.txHash]);
      assert.ok(["pax_sequencerReceipt", "eth_getRawTransactionByHash"].includes(message.method));
      const answer = {
        jsonrpc: "2.0", id: message.id,
        ...(service.error === undefined ? {
          result: message.method === "eth_getRawTransactionByHash" ? vector.rawTransaction : service.result,
        } : { error: service.error }),
      };
      response.writeHead(service.status, { "content-type": "application/json" }).end(JSON.stringify(answer));
    } catch (error) {
      service.failures.push(error.message);
      response.writeHead(500, { "content-type": "application/json" }).end(JSON.stringify({ error: error.message }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  service.rpcUrl = `http://127.0.0.1:${server.address().port}/`;
  service.close = () => new Promise((resolve, reject) => {
    server.closeAllConnections();
    server.close((error) => error ? reject(error) : resolve());
  });
  t.after(async () => {
    if (server.listening) await service.close();
    assert.deepEqual(service.failures, [], "every request must match the producer transcript");
  });
  return service;
}

import assert from "node:assert/strict";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import { keccak256 } from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  decodePaymentExactLog,
  encodePaymentPayloadHeader,
  verifyRpcPayment,
} from "@sidiora/layerx-seller-middleware";

export const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => new Uint8Array(Buffer.from(value.slice(2), "hex"));
const corpus = JSON.parse(await readFile(new URL(
  "../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
assert.equal(corpus.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
export const vector = corpus.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(vector);
export const pin = {
  keyId: bytes(vector.pinnedKey.keyId),
  publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight),
  last_height: BigInt(vector.pinnedKey.lastHeight),
};
const log = vector.receiptJson.logs[0];
const payment = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
export const requirements = {
  scheme: "exact", network: "eip155:125", amount: payment.amount.toString(),
  asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", purposeHash: hex(payment.paymentReference).slice(2) } },
};
export const resource = { contentType: "text/plain", body: "receipt-backed resource" };

export class MemoryFulfillments {
  records = new Map();
  pending = new Map();

  async get(key) {
    return structuredClone(this.records.get(key));
  }

  async fulfill(proposed, release) {
    const key = proposed.idempotencyKey;
    const pending = this.pending.get(key);
    if (pending !== undefined) await pending;
    const existing = await this.get(key);
    if (existing !== undefined) {
      if (existing.requestDigest !== proposed.requestDigest) throw new MiddlewareError("fulfillment-conflict");
      return existing;
    }
    const operation = (async () => {
      const record = structuredClone({ ...proposed, resource: await release() });
      this.records.set(key, record);
      return structuredClone(record);
    })();
    this.pending.set(key, operation);
    try {
      return await operation;
    } finally {
      if (this.pending.get(key) === operation) this.pending.delete(key);
    }
  }
}

export async function buyerEvidence(required) {
  const { kind, verification, ...proof } = await verifyRpcPayment(
    vector.receiptJson, vector.rawTransaction, vector.receiptJson.from.slice(2).padStart(64, "0"),
    required.accepts[0], [pin],
  );
  assert.equal(kind, "verified");
  assert.equal(proof.rawTransaction, vector.rawTransaction);
  assert.equal(proof.signedReceipt, vector.signedReceipt);
  assert.equal(verification.txHash, vector.receiptJson.txHash);
  assert.equal(proof.receiptDigest, hex(verification.receiptDigest));
  assert.notEqual(proof.receiptDigest, hex(keccak256(verification.canonicalBytes)));
  return {
    proof, verification,
    header: encodePaymentPayloadHeader({
      x402Version: required.x402Version, accepted: required.accepts[0], resource: required.resource, payload: proof,
    }),
  };
}

export async function listen(context, server) {
  context.after(async () => {
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}/paid`;
}

export async function assertDigestTamperingRefused(adapter, seller, required) {
  const { header } = await buyerEvidence(required);
  const decision = await seller.handle("merchant", header, async () => resource);
  assert.equal(decision.kind, "released");
  await adapter.assertReceiptBacked(decision, required.accepts[0]);
  for (const receiptDigest of [
    `0x${"00".repeat(32)}`,
    hex(keccak256(decision.verification.canonicalBytes)),
  ]) {
    const tampered = structuredClone(decision);
    tampered.settlement.extensions.layerx.receiptDigest = receiptDigest;
    await assert.rejects(adapter.assertReceiptBacked(tampered, required.accepts[0]),
      (error) => error instanceof adapter.LayerXIntegrationError && error.code === "receipt-not-backed");
  }
}

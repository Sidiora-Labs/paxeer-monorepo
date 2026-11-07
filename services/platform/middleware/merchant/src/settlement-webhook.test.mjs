import "../../../examples/test/support/source-loader.mjs";
import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { nodeReceipt, portableReceipt, recordedNode } from "../../../examples/test/support/node-receipts.mjs";

const { MerchantError, MerchantSettlementWebhooks } = await import("./index.ts");
const { VerifiedWebhookConsumer } = await import("@sidiora/layerx-seller-middleware");
const { FileOrders } = await import("../../../examples/support/merchant-app.mjs");
const { SequencerReceiptClient } = await import("../../../examples/support/runtime.mjs");

class InMemoryWebhookDeliveries {
  records = new Map();

  async claim(value) {
    const stored = this.records.get(value.deliveryId);
    if (stored !== undefined) {
      if (stored.payloadDigest !== value.payloadDigest) return "conflict";
      return stored.completed ? "completed" : "processing";
    }
    this.records.set(value.deliveryId, { ...value, completed: false });
    return "claimed";
  }

  async complete(id, digest) {
    const stored = this.records.get(id);
    assert.equal(stored?.payloadDigest, digest);
    stored.completed = true;
  }

  async release(id, digest) {
    const stored = this.records.get(id);
    if (stored?.payloadDigest === digest && !stored.completed) this.records.delete(id);
  }
}

async function merchantMiddlewareKeepsProofAfterRefusedWebhook(t) {
  const vector = nodeReceipt("V1-layerxledger-pay-exact");
  const node = await recordedNode(t, vector);
  const client = new SequencerReceiptClient(node.rpcUrl, [vector.key]);
  const log = vector.receiptJson.logs[0];
  const requirements = {
    scheme: "exact", network: "eip155:125",
    amount: BigInt(`0x${log.data.slice(66)}`).toString(), asset: log.data.slice(0, 66),
    payTo: log.topics[3], maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer: log.topics[2].slice(2), purposeHash: log.topics[1].slice(2) } },
  };
  const quote = {
    lines: [{ sku: "report", quantity: 1, title: "Report", unitAmount: requirements.amount, lineAmount: requirements.amount }],
    totalAmount: requirements.amount, asset: requirements.asset, payTo: requirements.payTo,
    scheme: requirements.scheme, network: requirements.network, maxTimeoutSeconds: requirements.maxTimeoutSeconds,
    paymentRequired: { x402Version: 2, resource: { url: "https://merchant.example/report" }, accepts: [requirements] },
  };
  const directory = await mkdtemp(join(tmpdir(), "merchant-webhook-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const orders = new FileOrders(directory);
  const requestDigest = createHash("sha256").update(JSON.stringify(quote)).digest("hex");
  const order = await orders.open({ checkoutKey: "report", requestDigest, quote });
  const key = generateKeyPairSync("ed25519");
  const deliveries = new InMemoryWebhookDeliveries();
  const webhooks = new MerchantSettlementWebhooks({
    verifier: new VerifiedWebhookConsumer({
      publicKeys: { merchant: key.publicKey.export({ type: "spki", format: "der" }).subarray(-32) },
      deliveries,
    }),
    orders, receiptKeys: [vector.key],
    receipts: {
      async resolve(reference) {
        assert.equal(reference, vector.txHash);
        const proof = await client.resolve(vector.raw);
        assert.deepEqual(proof, portableReceipt(vector));
        return proof;
      },
    },
  });
  function delivery(transaction, digest = vector.receiptDigest.slice(2)) {
    const body = Buffer.from(JSON.stringify({
      order_id: order.orderId, request_digest: requestDigest, receipt_digest: digest,
      receipt_ref: vector.txHash, transaction, verification: "receipted",
    }));
    const id = "retryable-delivery";
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = sign(null, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]), key.privateKey);
    return { body, headers: { id, timestamp, keyId: "merchant", signature: `v1=${signature.toString("base64")}` } };
  }
  const { keccak256 } = await import("@sidiora/layerx-sdk");
  const signedByteDigest = Buffer.from(keccak256(vector.receipt)).toString("hex");
  assert.notEqual(signedByteDigest, vector.receiptDigest.slice(2));
  for (const refused of [
    delivery(`0x${"99".repeat(32)}`),
    delivery(vector.txHash, "99".repeat(32)),
    delivery(vector.txHash, signedByteDigest),
  ]) {
    await assert.rejects(webhooks.consume(refused.body, refused.headers),
      (error) => error instanceof MerchantError && error.code === "invalid-webhook");
    assert.deepEqual(await new FileOrders(directory).get(order.orderId), order);
    assert.equal(deliveries.records.size, 0, "a refused delivery must release its claim");
  }
  const valid = delivery(vector.txHash);
  assert.equal(await webhooks.consume(valid.body, valid.headers), "processed");
  const paid = await new FileOrders(directory).get(order.orderId);
  assert.equal(paid.state, "paid-verified");
  assert.equal(paid.transaction, vector.txHash);
  assert.equal(paid.receiptDigest, vector.receiptDigest.slice(2));
  assert.equal(node.requests.length, 4);
  assert.equal(await webhooks.consume(valid.body, valid.headers), "duplicate");
  assert.equal(node.requests.length, 4, "completed delivery must not resolve the proof again");
  assert.deepEqual(await orders.get(order.orderId), paid);
}

test(merchantMiddlewareKeepsProofAfterRefusedWebhook);

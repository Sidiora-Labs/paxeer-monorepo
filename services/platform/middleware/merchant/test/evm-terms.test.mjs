import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import {
  VerifiedWebhookConsumer,
  rpcCheckpointEvidence,
} from "@sidiora/layerx-seller-middleware";
import { MerchantError, MerchantSettlementWebhooks } from "../dist/index.js";

const bytes = (hex) => Uint8Array.from(Buffer.from(hex, "hex"));
const toHex = (value) => Buffer.from(value).toString("hex");
const readJson = async (path) => JSON.parse(await readFile(new URL(path, import.meta.url), "utf8"));
const fixture = await readJson("../../../../../sdk/platform/conformance/fixtures/receipt-positive-v2.json");
const batch = await readJson("../../conformance/pay6/batch.json");
const checkpoint = await readJson("../../conformance/pay6/checkpoint.json");
const canonicalReceipt = bytes(fixture.canonical_receipt_hex);
const authorized = fixture.authorized_batch;
const receiptEvidence = {
  canonicalReceipt,
  authorizedBatch: {
    batchId: bytes(authorized.batch_id_hex),
    asset: bytes(authorized.asset_hex),
    previousStateRoot: bytes(authorized.previous_state_root_hex),
    resultingStateRoot: bytes(authorized.resulting_state_root_hex),
    sequencerPublicKey: bytes(authorized.sequencer_public_key_hex),
  },
};

class OrderLedger {
  #orders = new Map();

  async open(request) {
    const existing = this.#orders.get(request.checkoutKey);
    if (existing !== undefined) {
      if (existing.requestDigest !== request.requestDigest) throw new MerchantError("order-conflict");
      return existing;
    }
    const order = {
      orderId: request.checkoutKey,
      checkoutKey: request.checkoutKey,
      requestDigest: request.requestDigest,
      quote: request.quote,
      state: "awaiting-payment",
    };
    this.#orders.set(order.orderId, order);
    return order;
  }

  async get(orderId) {
    return this.#orders.get(orderId);
  }

  async releaseResource(orderId) {
    return this.#required(orderId);
  }

  async markPaid(orderId, requestDigest, receiptDigest, transaction) {
    const current = this.#required(orderId);
    if (current.requestDigest !== requestDigest || current.state === "refused"
      || (current.state === "paid-verified"
        && (current.receiptDigest !== receiptDigest || current.transaction !== transaction))) {
      throw new MerchantError("order-conflict");
    }
    const paid = { ...current, state: "paid-verified", receiptDigest, transaction };
    this.#orders.set(orderId, paid);
    return paid;
  }

  async markRefused(orderId, requestDigest) {
    const current = this.#required(orderId);
    if (current.requestDigest !== requestDigest || current.state === "paid-verified") {
      throw new MerchantError("order-conflict");
    }
    const refused = { ...current, state: "refused" };
    this.#orders.set(orderId, refused);
    return refused;
  }

  #required(orderId) {
    const order = this.#orders.get(orderId);
    if (order === undefined) throw new MerchantError("order-conflict");
    return order;
  }
}

class DeliveryLedger {
  #deliveries = new Map();

  async claim(value) {
    const existing = this.#deliveries.get(value.deliveryId);
    if (existing !== undefined) {
      if (existing.payloadDigest !== value.payloadDigest) return "conflict";
      if (existing.done) return "completed";
      if (existing.leaseUntilMs > Date.now()) return "processing";
    }
    this.#deliveries.set(value.deliveryId, { ...value, done: false });
    return "claimed";
  }

  async complete(deliveryId, payloadDigest) {
    const current = this.#deliveries.get(deliveryId);
    if (current === undefined || current.payloadDigest !== payloadDigest) throw new Error("delivery-conflict");
    current.done = true;
  }

  async release(deliveryId, payloadDigest) {
    const current = this.#deliveries.get(deliveryId);
    if (current !== undefined && current.payloadDigest === payloadDigest && !current.done) {
      this.#deliveries.delete(deliveryId);
    }
  }
}

async function committedEvidence() {
  const operator = checkpoint.operator;
  return rpcCheckpointEvidence(checkpoint, {
    networkId: 7,
    canonicalHeader: bytes(batch.header),
    headerSignature: bytes(batch.signature),
    authorization: {
      sequencerId: bytes(batch.sequencer_id),
      publicKey: receiptEvidence.authorizedBatch.sequencerPublicKey,
      firstBatchNumber: 1n,
      lastBatchNumber: 1n,
    },
    proof: { leafIndex: 0, leafCount: 1, siblings: [] },
  }, {
    canonicalContext: bytes(operator.context),
    requiredGuarantors: operator.required_guarantors,
    verification: {
      bondedSet: [{ guarantorId: bytes(operator.guarantor_id), publicKey: bytes(operator.public_key), bonded: true }],
      registeredCheckpointId: bytes(checkpoint.checkpoint_evidence.checkpoint_id),
      expectedPaxeerChainId: BigInt(operator.chain_id),
      expectedSettlementContract: bytes(operator.contract),
      registeredSettlementReference: bytes(operator.reference),
      availabilityObtained: true,
    },
    signatures: {
      async verifyRecoverableSecp256k1(publicKey, signature, signatureV, signer, digest) {
        const recovered = secp256k1.Signature.fromBytes(signature)
          .addRecoveryBit(signatureV - 27).recoverPublicKey(digest);
        return secp256k1.verify(signature, digest, publicKey, { prehash: false })
          && toHex(recovered.toBytes(true)) === toHex(publicKey)
          && toHex(keccak_256(recovered.toBytes(false).subarray(1)).slice(-20)) === toHex(signer);
      },
    },
  });
}

async function webhookContext(commitment) {
  const key = await globalThis.crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"]);
  const publicKey = new Uint8Array(await globalThis.crypto.subtle.exportKey("raw", key.publicKey));
  const orders = new OrderLedger();
  const deliveries = new DeliveryLedger();
  const requirements = {
    scheme: "exact",
    network: "eip155:7",
    amount: fixture.expected.amount,
    asset: authorized.asset_hex,
    payTo: fixture.expected.to_hex,
    maxTimeoutSeconds: 120,
    extra: { layerx: { commitment } },
  };
  const quote = {
    lines: [{ sku: "report", quantity: 1, title: "Market report", unitAmount: requirements.amount, lineAmount: requirements.amount }],
    totalAmount: requirements.amount,
    asset: requirements.asset,
    payTo: requirements.payTo,
    scheme: requirements.scheme,
    network: requirements.network,
    maxTimeoutSeconds: requirements.maxTimeoutSeconds,
    paymentRequired: {
      x402Version: 2,
      resource: { url: "https://merchant.example/orders/report" },
      accepts: [requirements],
      extensions: {},
    },
  };
  const requestDigest = toHex(new Uint8Array(await globalThis.crypto.subtle.digest(
    "SHA-256", new TextEncoder().encode(JSON.stringify(quote)),
  )));
  const order = await orders.open({ checkoutKey: "report", requestDigest, quote });
  const leaf = Buffer.concat([Buffer.from("LXP/v1/merkle-leaf\0"), canonicalReceipt]);
  const receiptDigest = toHex(new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", leaf)));
  const receipts = new Map([["receipt:report", receiptEvidence]]);
  const proofs = new Map([[toHex(canonicalReceipt), await committedEvidence()]]);
  const webhooks = new MerchantSettlementWebhooks({
    verifier: new VerifiedWebhookConsumer({ publicKeys: { merchant: publicKey }, deliveries }),
    orders,
    receipts: {
      async resolve(receiptRef) {
        const evidence = receipts.get(receiptRef);
        if (evidence === undefined) throw new Error("receipt-missing");
        return evidence;
      },
    },
    commitments: {
      async resolve(receipt, network, rung) {
        assert.equal(network, requirements.network);
        assert.equal(rung, "committed");
        const evidence = proofs.get(toHex(receipt));
        if (evidence === undefined) throw new Error("commitment-missing");
        return evidence;
      },
    },
    protocolVersion: 2,
  });
  async function delivery(verification) {
    const body = new TextEncoder().encode(JSON.stringify({
      order_id: order.orderId,
      request_digest: order.requestDigest,
      receipt_digest: receiptDigest,
      receipt_ref: "receipt:report",
      transaction: `0x${receiptDigest}`,
      verification,
    }));
    const id = `delivery-${verification}`;
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = new Uint8Array(await globalThis.crypto.subtle.sign(
      "Ed25519", key.privateKey, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]),
    ));
    return { body, headers: { id, timestamp, keyId: "merchant", signature: `v1=${Buffer.from(signature).toString("base64")}` } };
  }
  return { webhooks, orders, order, receiptDigest, delivery };
}

async function settlementWebhookAcceptsRungNames() {
  for (const commitment of ["receipted", "committed"]) {
    for (const verification of commitment === "committed" ? ["receipted", "committed"] : ["receipted"]) {
      const context = await webhookContext(commitment);
      const { body, headers } = await context.delivery(verification);
      assert.equal(await context.webhooks.consume(body, headers), "processed");
      const paid = await context.orders.get(context.order.orderId);
      assert.equal(paid.state, "paid-verified");
      assert.equal(paid.receiptDigest, context.receiptDigest);
      assert.equal(paid.transaction, `0x${context.receiptDigest}`);
      assert.equal(await context.webhooks.consume(body, headers), "duplicate");
    }
  }
  const context = await webhookContext("receipted");
  const { body, headers } = await context.delivery("committed");
  await assert.rejects(context.webhooks.consume(body, headers),
    (error) => error instanceof MerchantError && error.code === "invalid-webhook");
  assert.equal((await context.orders.get(context.order.orderId)).state, "awaiting-payment");
}

async function settlementWebhookRefusesKernelLevels() {
  const context = await webhookContext("committed");
  for (const verification of ["sequencer-signed", "batch-included", "state-proven", "checkpoint-finalised", "settlement-anchored"]) {
    const { body, headers } = await context.delivery(verification);
    await assert.rejects(context.webhooks.consume(body, headers),
      (error) => error instanceof MerchantError && error.code === "invalid-webhook",
      `${verification} must be refused by the merchant parser`);
    assert.equal((await context.orders.get(context.order.orderId)).state, "awaiting-payment");
  }
}

test(settlementWebhookAcceptsRungNames);
test(settlementWebhookRefusesKernelLevels);

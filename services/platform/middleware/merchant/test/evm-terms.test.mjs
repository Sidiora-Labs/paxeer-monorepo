import assert from "node:assert/strict";
import test from "node:test";
import { VerifiedWebhookConsumer } from "@sidiora/layerx-seller-middleware";
import { committedFixture } from "../../conformance/pay6/committed-fixture.mjs";
import { MerchantError, MerchantSettlementWebhooks } from "../dist/index.js";

const toHex = (value) => Buffer.from(value).toString("hex");

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

async function committedEvidence(context) {
  return committedFixture(context);
}

async function webhookContext(commitment, context) {
  const { evidence, receipt, pins, requirements: fixtureRequirements } = await committedEvidence(context);
  const proof = { ...receipt, verificationLevel: commitment };
  const key = await globalThis.crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"]);
  const publicKey = new Uint8Array(await globalThis.crypto.subtle.exportKey("raw", key.publicKey));
  const orders = new OrderLedger();
  const deliveries = new DeliveryLedger();
  const requirements = {
    ...fixtureRequirements,
    extra: { layerx: { ...fixtureRequirements.extra.layerx, commitment } },
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
  const receiptDigest = receipt.receiptDigest.slice(2);
  const txHash = receipt.txHash;
  const receipts = new Map([["receipt:report", proof]]);
  const proofs = new Map([[txHash, evidence]]);
  const webhooks = new MerchantSettlementWebhooks({
    verifier: new VerifiedWebhookConsumer({ publicKeys: { merchant: publicKey }, deliveries }),
    orders,
    receiptKeys: pins,
    receipts: {
      async resolve(receiptRef) {
        const evidence = receipts.get(receiptRef);
        if (evidence === undefined) throw new Error("receipt-missing");
        return evidence;
      },
    },
    commitments: {
      async resolve(transactionHash, network) {
        assert.equal(network, requirements.network);
        assert.equal(proof.verificationLevel, "committed");
        const evidence = proofs.get(transactionHash);
        if (evidence === undefined) throw new Error("commitment-missing");
        return evidence;
      },
    },
  });
  async function delivery(verification) {
    const body = new TextEncoder().encode(JSON.stringify({
      order_id: order.orderId,
      request_digest: order.requestDigest,
      receipt_digest: receiptDigest,
      receipt_ref: "receipt:report",
      transaction: txHash,
      verification,
    }));
    const id = `delivery-${verification}`;
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = new Uint8Array(await globalThis.crypto.subtle.sign(
      "Ed25519", key.privateKey, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]),
    ));
    return { body, headers: { id, timestamp, keyId: "merchant", signature: `v1=${Buffer.from(signature).toString("base64")}` } };
  }
  return { webhooks, orders, order, receiptDigest, txHash, delivery };
}

async function settlementWebhookAcceptsRungNames(testContext) {
  for (const commitment of ["receipted", "committed"]) {
    for (const verification of commitment === "committed" ? ["receipted", "committed"] : ["receipted"]) {
      const context = await webhookContext(commitment, testContext);
      const { body, headers } = await context.delivery(verification);
      assert.equal(await context.webhooks.consume(body, headers), "processed");
      const paid = await context.orders.get(context.order.orderId);
      assert.equal(paid.state, "paid-verified");
      assert.equal(paid.receiptDigest, context.receiptDigest);
      assert.equal(paid.transaction, context.txHash);
      assert.equal(await context.webhooks.consume(body, headers), "duplicate");
    }
  }
  const context = await webhookContext("receipted", testContext);
  const { body, headers } = await context.delivery("committed");
  await assert.rejects(context.webhooks.consume(body, headers),
    (error) => error instanceof MerchantError && error.code === "invalid-webhook");
  assert.equal((await context.orders.get(context.order.orderId)).state, "awaiting-payment");
}

async function settlementWebhookRefusesKernelLevels(testContext) {
  const context = await webhookContext("committed", testContext);
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

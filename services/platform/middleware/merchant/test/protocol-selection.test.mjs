import assert from "node:assert/strict";
import { generateKeyPairSync } from "node:crypto";
import test from "node:test";
import { ReceiptPayloadAuthority, SellerMiddleware } from "@sidiora/layerx-seller-middleware";
import { MerchantMiddleware } from "../dist/index.js";

class InMemoryOrders {
  #orders = new Map();

  async open(request) {
    const existing = this.#orders.get(request.checkoutKey);
    if (existing !== undefined) {
      if (existing.requestDigest !== request.requestDigest) throw new Error("order-conflict");
      return existing;
    }
    const order = {
      orderId: request.checkoutKey,
      checkoutKey: request.checkoutKey,
      requestDigest: request.requestDigest,
      state: "awaiting-payment",
      quote: request.quote,
    };
    this.#orders.set(order.orderId, order);
    return order;
  }

  async releaseResource(orderId) {
    return this.#required(orderId);
  }

  async markPaid(orderId, requestDigest, receiptDigest, transaction) {
    const current = this.#required(orderId);
    if (current.requestDigest !== requestDigest) throw new Error("order-conflict");
    if (current.state === "paid-verified") {
      if (current.receiptDigest !== receiptDigest || current.transaction !== transaction) {
        throw new Error("order-conflict");
      }
      return current;
    }
    const paid = { ...current, state: "paid-verified", receiptDigest, transaction };
    this.#orders.set(orderId, paid);
    return paid;
  }

  async markRefused(orderId, requestDigest) {
    const current = this.#required(orderId);
    if (current.requestDigest !== requestDigest || current.state === "paid-verified") throw new Error("order-conflict");
    const refused = { ...current, state: "refused" };
    this.#orders.set(orderId, refused);
    return refused;
  }

  async get(orderId) {
    return this.#orders.get(orderId);
  }

  #required(orderId) {
    const order = this.#orders.get(orderId);
    if (order === undefined) throw new Error("order-missing");
    return order;
  }
}

class InMemoryFulfillments {
  #entries = new Map();

  async fulfill(proposed, release) {
    const existing = this.#entries.get(proposed.idempotencyKey);
    if (existing !== undefined) return existing;
    const stored = { ...proposed, resource: await release() };
    this.#entries.set(proposed.idempotencyKey, stored);
    return stored;
  }
}

async function merchantCheckoutUsesEip155Seller() {
  const pair = generateKeyPairSync("ed25519");
  const receiptKeys = [{
    keyId: new Uint8Array(32).fill(1),
    publicKey: new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32)),
    first_height: 1n,
    last_height: 1000n,
  }];
  const orders = new InMemoryOrders();
  const merchant = new MerchantMiddleware({
    catalog: {
      get: async () => ({
        sku: "market-report",
        title: "Receipt-backed market report",
        unitAmount: "25",
        asset: `0x${"11".repeat(32)}`,
        payTo: `0x${"22".repeat(32)}`,
        scheme: "exact",
        network: "eip155:125",
        maxTimeoutSeconds: 120,
        extra: { layerx: { commitment: "receipted", purposeHash: "44".repeat(32) } },
      }),
    },
    orders,
    sellers: {
      create: (paymentRequired) => new SellerMiddleware({
        paymentRequired,
        receiptKeys,
        authority: new ReceiptPayloadAuthority(receiptKeys),
        fulfillments: new InMemoryFulfillments(),
      }),
    },
    resourceUrl: (checkoutKey) => `https://merchant.example/orders/${checkoutKey}`,
  });
  const result = await merchant.checkout("acct:merchant", "merchant-checkout", [{ sku: "market-report", quantity: 1 }]);
  assert.equal(result.kind, "payment-required");
  assert.equal(result.order.state, "awaiting-payment");
  assert.equal(result.decision.body.accepts[0].network, "eip155:125");
  assert.equal(result.decision.body.accepts[0].amount, "25");
}

test(merchantCheckoutUsesEip155Seller);

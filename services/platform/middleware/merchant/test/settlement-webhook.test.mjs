import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256 } from "@sidiora/layerx-sdk";
import { LAYERX_LEDGER_ADDRESS, VerifiedWebhookConsumer } from "@sidiora/layerx-seller-middleware";
import { MerchantError, MerchantSettlementWebhooks } from "../dist/index.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const asset = word(0x11);
const payee = word(0x22);
const payer = word(0x33);
const reference = word(0x44);
const keyId = word(0x55);
const pair = generateKeyPairSync("ed25519");
const publicKey = new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
const pin = { keyId: new Uint8Array(keyId), publicKey, first_height: 1n, last_height: 1000n };
const requirements = {
  scheme: "exact", network: "eip155:125", amount: "25", asset: hex(asset), payTo: hex(payee),
  maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: payer.toString("hex"), purposeHash: reference.toString("hex") } },
};
const exactTopic = keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"));

function uint(value, width) {
  assert.ok(value >= 0n && value < 1n << BigInt(width * 8));
  return Buffer.from(value.toString(16).padStart(width * 2, "0"), "hex");
}

function bounded(value) {
  return Buffer.concat([uint(BigInt(value.length), 4), value]);
}

function integer(value) {
  if (value === 0n) return Buffer.alloc(0);
  const digits = value.toString(16);
  return Buffer.from(digits.padStart(digits.length + digits.length % 2, "0"), "hex");
}

function rlp(value) {
  const list = Array.isArray(value);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0] < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function transaction(amount) {
  const selector = keccak256(new TextEncoder().encode("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")).slice(0, 4);
  const data = Buffer.concat([selector, payer, payee, asset, uint(amount, 32), uint(1n, 32), reference]);
  const fields = [integer(125n), integer(0n), integer(1n), integer(2n), integer(200000n),
    bytes(LAYERX_LEDGER_ADDRESS), integer(0n), data, []];
  const unsigned = Buffer.concat([Buffer.of(2), rlp(fields)]);
  const signature = secp256k1.sign(keccak256(unsigned), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  const raw = Buffer.concat([Buffer.of(2), rlp([...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s)])]);
  const decoded = decodeRawEip1559(raw);
  assert.equal(decoded.chainId, 125n);
  assert.deepEqual(Buffer.from(decoded.data), data);
  return raw;
}

function effect(ordinal, kind, body) {
  return Buffer.concat([uint(12n, 2), uint(BigInt(ordinal), 2), uint(0n, 2), Buffer.of(kind, 0), bounded(Buffer.alloc(32)), bounded(body)]);
}

function signedProof({ amount = 25n, height = 7n, signingPin = pin, paymentReference = reference,
  paymentTo = payee, logAddress = LAYERX_LEDGER_ADDRESS, count = 1, resultCode = 0n } = {}) {
  const rawTransaction = transaction(amount);
  const txHash = keccak256(rawTransaction);
  const logData = Buffer.concat([asset, uint(amount, 32)]);
  const logBody = Buffer.concat([bytes(logAddress), Buffer.of(4), exactTopic, paymentReference, payer, paymentTo, sha256(logData)]);
  const summary = Buffer.concat([
    uint(50000n, 8), uint(50000n, 8), uint(2n, 8), uint(0n, 4), Buffer.alloc(40), sha256(Buffer.alloc(0)),
    uint(0n, 4), uint(BigInt(count), 4), Buffer.of(4), Buffer.from("uhpx"),
  ]);
  const effects = [effect(0, 1, summary), ...Array.from({ length: count }, (_, i) => effect(i + 1, 3, logBody))];
  const from = Buffer.from("0000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1", "hex");
  const to = Buffer.concat([Buffer.alloc(12), bytes(LAYERX_LEDGER_ADDRESS)]);
  const prefix = Buffer.concat([
    uint(2n, 2), uint(0x5201n, 2), uint(2n, 2), bounded(txHash), uint(1n, 8),
    bounded(Buffer.alloc(32)), bounded(sha256(Buffer.concat([uint(1n, 8), txHash]))), bounded(sha256(Buffer.alloc(0))),
    uint(resultCode, 4), uint(BigInt(effects.length), 4), ...effects,
    uint(100000n, 16), bounded(uint(height, 32)), uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2),
    bounded(sha256(Buffer.from("uhpx"))), uint(0n, 16), bounded(from), uint(1000000n, 16), uint(900000n, 16),
    uint(0n, 8), bounded(to), uint(0n, 16), uint(0n, 16), bounded(Buffer.alloc(32)), bounded(Buffer.alloc(32)),
    bounded(keyId), uint(1n, 8),
  ]);
  const unsignedReceipt = Buffer.concat([prefix, Buffer.of(0)]);
  const signingDigest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), unsignedReceipt]));
  const signedReceipt = Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, signingDigest, pair.privateKey))]);
  return {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted", rawTransaction: hex(rawTransaction),
    signedReceipt: hex(signedReceipt), receiptDigest: hex(keccak256(signedReceipt)),
    receiptKey: { keyId: hex(signingPin.keyId), publicKey: hex(signingPin.publicKey),
      firstHeight: `0x${signingPin.first_height.toString(16)}`, lastHeight: `0x${signingPin.last_height.toString(16)}` },
  };
}

class InMemoryMerchantOrderStore {
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

class InMemoryDeliveries {
  #records = new Map();

  async claim(value) {
    const existing = this.#records.get(value.deliveryId);
    if (existing === undefined) {
      this.#records.set(value.deliveryId, { digest: value.payloadDigest, done: false });
      return "claimed";
    }
    if (existing.digest !== value.payloadDigest) return "conflict";
    return existing.done ? "completed" : "processing";
  }

  async complete(deliveryId, payloadDigest) {
    const record = this.#records.get(deliveryId);
    assert.equal(record?.digest, payloadDigest);
    record.done = true;
  }

  async release(deliveryId, payloadDigest) {
    const record = this.#records.get(deliveryId);
    if (record !== undefined && record.digest === payloadDigest && !record.done) {
      this.#records.delete(deliveryId);
    }
  }
}

async function webhookContext() {
  const proof = signedProof();
  const orders = new InMemoryMerchantOrderStore();
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
  const requestDigest = sha256(Buffer.from(JSON.stringify(quote))).toString("hex");
  const order = await orders.open({ checkoutKey: "report", requestDigest, quote });
  const key = generateKeyPairSync("ed25519");
  const publicKey = new Uint8Array(key.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
  const webhooks = new MerchantSettlementWebhooks({
    verifier: new VerifiedWebhookConsumer({ publicKeys: { merchant: publicKey }, deliveries: new InMemoryDeliveries() }),
    orders,
    receiptKeys: [pin],
    receipts: {
      async resolve(receiptRef) {
        assert.equal(receiptRef, "receipt:report");
        return proof;
      },
    },
  });
  const receiptDigest = hex(keccak256(bytes(proof.signedReceipt))).slice(2);
  const txHash = hex(keccak256(bytes(proof.rawTransaction)));
  function delivery(transaction, id, digest = receiptDigest) {
    const body = Buffer.from(JSON.stringify({
      order_id: order.orderId,
      request_digest: order.requestDigest,
      receipt_digest: digest,
      receipt_ref: "receipt:report",
      transaction,
      verification: "receipted",
    }));
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = sign(null, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]), key.privateKey);
    return { body, headers: { id, timestamp, keyId: "merchant", signature: `v1=${signature.toString("base64")}` } };
  }
  return { webhooks, orders, order, receiptDigest, txHash, delivery };
}

async function webhookMarksOrderPaidWithTransactionHash() {
  const context = await webhookContext();
  const { body, headers } = context.delivery(context.txHash, "delivery-paid");
  assert.match(context.txHash, /^0x[0-9a-f]{64}$/u);
  assert.equal(await context.webhooks.consume(body, headers), "processed");
  const paid = await context.orders.get(context.order.orderId);
  assert.equal(paid.state, "paid-verified");
  assert.equal(paid.receiptDigest, context.receiptDigest);
  assert.equal(paid.transaction, context.txHash);
  assert.equal(await context.webhooks.consume(body, headers), "duplicate");
  assert.deepEqual(await context.orders.get(context.order.orderId), paid);
}

async function webhookRefusesTransactionOtherThanReceiptHash() {
  const context = await webhookContext();
  for (const [index, transaction] of [hex(word(0x99)), `lxp:${context.receiptDigest}`, `0x${context.receiptDigest}`].entries()) {
    assert.notEqual(transaction, context.txHash);
    const { body, headers } = context.delivery(transaction, `delivery-wrong-${index}`);
    await assert.rejects(context.webhooks.consume(body, headers),
      (error) => error instanceof MerchantError && error.code === "invalid-webhook");
    assert.deepEqual(await context.orders.get(context.order.orderId), context.order);
    assert.equal((await context.orders.get(context.order.orderId)).state, "awaiting-payment");
  }
  const wrongDigest = context.delivery(context.txHash, "delivery-wrong-digest", "99".repeat(32));
  await assert.rejects(context.webhooks.consume(wrongDigest.body, wrongDigest.headers),
    (error) => error instanceof MerchantError && error.code === "invalid-webhook");
  assert.deepEqual(await context.orders.get(context.order.orderId), context.order);
  const valid = context.delivery(context.txHash, "delivery-retry");
  assert.equal(await context.webhooks.consume(valid.body, valid.headers), "processed");
  assert.equal((await context.orders.get(context.order.orderId)).transaction, context.txHash);
}

test(webhookMarksOrderPaidWithTransactionHash);
test(webhookRefusesTransactionOtherThanReceiptHash);

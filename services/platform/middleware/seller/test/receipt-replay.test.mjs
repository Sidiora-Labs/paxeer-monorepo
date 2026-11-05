import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  PAYMENT_RESPONSE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  decodePaymentExactLog,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
} from "../dist/index.js";
import { FileFulfillmentRepository } from "../../../examples/paid-api/index.mjs";

const vectors = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const vector = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(vector);
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const log = vector.receiptJson.logs[0];
const payment = decodePaymentExactLog({
  address: log.address, data: bytes(log.data), topics: log.topics.map(bytes),
});
const pin = {
  keyId: bytes(vector.pinnedKey.keyId),
  publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight),
  last_height: BigInt(vector.pinnedKey.lastHeight),
};
const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [pin]);
const proof = {
  format: "layerx-receipt-proof-v2",
  verificationLevel: "receipted",
  rawTransaction: vector.rawTransaction,
  signedReceipt: vector.signedReceipt,
  receiptDigest: hex(verified.receiptDigest),
  receiptKey: {
    keyId: vector.pinnedKey.keyId,
    publicKey: vector.pinnedKey.publicKey,
    firstHeight: `0x${pin.first_height.toString(16)}`,
    lastHeight: `0x${pin.last_height.toString(16)}`,
  },
};
const requirements = {
  scheme: "exact", network: "eip155:125", amount: payment.amount.toString(),
  asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
  extra: { layerx: {
    commitment: "receipted", payer: hex(payment.from).slice(2),
    purposeHash: hex(payment.paymentReference).slice(2),
  } },
};
const resource = { url: "https://merchant.example/paid" };

function fulfillments(directory) {
  return new FileFulfillmentRepository(directory);
}

function seller(directory) {
  return new SellerMiddleware({
    paymentRequired: { x402Version: 2, resource, accepts: [requirements] },
    receiptKeys: [pin], authority: new ReceiptPayloadAuthority([pin]),
    fulfillments: fulfillments(directory),
  });
}

function header(description, receipt = proof) {
  return encodePaymentPayloadHeader({
    x402Version: 2, accepted: requirements, payload: receipt,
    resource: { ...resource, description },
  });
}

function assertReceiptReuse(decision) {
  assert.equal(decision.kind, "refused");
  assert.equal(decision.status, 409);
  assert.equal(decision.settlement.success, false);
  assert.equal(decision.settlement.errorReason, "receipt_reuse");
  assert.equal(decision.settlement.transaction, "");
  assert.deepEqual(decodeSettlementHeader(decision.headers[PAYMENT_RESPONSE_HEADER]), decision.settlement);
}

async function receiptReplayRefusesChangedRequestAndReplaysOriginal(t) {
  const directory = await mkdtemp(join(tmpdir(), "seller-receipt-replay-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const middleware = seller(directory);
  let releases = 0;
  const release = async (idempotencyKey) => {
    releases += 1;
    return { idempotencyKey, order: "first-order" };
  };
  const original = header("first request");
  const changed = header("second request");
  assert.notEqual(original, changed);
  const first = await middleware.handle("merchant", original, release);
  assert.equal(first.kind, "released");
  assert.equal(first.status, 200);
  assert.equal(first.resource.order, "first-order");
  assert.equal(first.verification.txHash, vector.receiptJson.txHash);
  assertReceiptReuse(await middleware.handle("merchant", changed, release));
  assert.equal(releases, 1);

  const restarted = seller(directory);
  assertReceiptReuse(await restarted.handle("merchant", changed, release));
  const replayed = await restarted.handle("merchant", original, release);
  assert.equal(replayed.kind, "released");
  assert.equal(replayed.status, 200);
  assert.deepEqual(replayed.resource, first.resource);
  assert.deepEqual(replayed.settlement, first.settlement);
  assert.deepEqual(replayed.headers, first.headers);
  assert.equal(releases, 1);
  assert.equal((await readdir(directory)).length, 1);
  const stored = await fulfillments(directory).get(first.resource.idempotencyKey);
  assert.equal(stored.receiptDigest, proof.receiptDigest);
  assert.deepEqual(stored.resource, first.resource);
}

async function receiptIdentityIsBoundToOnePrincipal(t) {
  const directory = await mkdtemp(join(tmpdir(), "seller-receipt-merchants-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const middleware = seller(directory);
  const releasedKeys = [];
  const release = async (key) => {
    releasedKeys.push(key);
    return { key };
  };
  const first = await middleware.handle("merchant-one", header("first request"), release);
  const second = await middleware.handle("merchant-two", header("second request"), release);
  assert.equal(first.kind, "released");
  assertReceiptReuse(second);
  assert.equal(releasedKeys.length, 1);
  assertReceiptReuse(await middleware.handle("merchant-one", header("second request"), release));
  assertReceiptReuse(await middleware.handle("merchant-two", header("first request"), release));
  assert.equal(releasedKeys.length, 1);
}

async function unverifiedReceiptCannotClaimFulfillment(t) {
  const directory = await mkdtemp(join(tmpdir(), "seller-receipt-verification-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const middleware = seller(directory);
  const tampered = bytes(proof.signedReceipt);
  tampered[tampered.length - 1] ^= 1;
  const invalid = {
    ...proof, signedReceipt: hex(tampered), receiptDigest: hex(keccak256(tampered)),
  };
  let releases = 0;
  const release = async () => {
    releases += 1;
    return { order: "verified-order" };
  };
  await assert.rejects(middleware.handle("merchant", header("invalid request", invalid), release),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure");
  assert.equal(releases, 0);
  assert.deepEqual(await readdir(directory), []);
  const valid = await middleware.handle("merchant", header("valid request"), release);
  assert.equal(valid.kind, "released");
  assert.equal(releases, 1);
}

test(receiptReplayRefusesChangedRequestAndReplaysOriginal);
test(receiptIdentityIsBoundToOnePrincipal);
test(unverifiedReceiptCannotClaimFulfillment);

import "./support/production-sources.mjs";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

await import("./grant-draw-header.test.mjs");
await import("./proof_middleware_buyer_ts_exports.test.mjs");

const { buyerDrawFixture, buyerExportDrawFixture } = await import("./support/grant-draw-walkthrough.mjs");
const { keccak256, verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
const { decodePaymentExactLog, verifyPaymentReceipt } = await import("@sidiora/layerx-seller-middleware");
const bytes = (value) => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;

async function buyerGrantRefusesAuthenticNodeExactEvent() {
  const vectors = JSON.parse(readFileSync(new URL("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8"));
  const vector = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
  assert.ok(vector, "the committed node-produced witness must exist");
  const topic = hex(keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")));
  const log = vector.receiptJson.logs.find((entry) => entry.topics[0] === topic);
  assert.ok(log);
  const payment = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
  const keys = [{ keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
    first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight) }];
  const verifiedReceipt = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), keys);
  const proof = { format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
    rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
    receiptDigest: hex(verifiedReceipt.receiptDigest), receiptKey: {
      keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
      firstHeight: `0x${BigInt(vector.pinnedKey.firstHeight).toString(16)}`,
      lastHeight: `0x${BigInt(vector.pinnedKey.lastHeight).toString(16)}`,
    } };
  const requirements = { scheme: "exact", network: "eip155:125", amount: payment.amount.toString(),
    asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer: hex(payment.from).slice(2),
      purposeHash: hex(payment.paymentReference).slice(2) } } };
  const verified = await verifyPaymentReceipt(proof, requirements, keys);
  assert.deepEqual(verified.payment, payment);
  assert.notDeepEqual(payment.from, payment.to);
  for (const scheme of ["metered", "subscription"]) {
    await assert.rejects(verifyPaymentReceipt(proof, { ...requirements, scheme }, keys),
      (error) => error.code === "verification-failure");
  }
}

test(buyerDrawFixture);
test(buyerExportDrawFixture);
test(buyerGrantRefusesAuthenticNodeExactEvent);

import assert from "node:assert/strict";
import test from "node:test";
import "./source-loader.mjs";

const { verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
const { MiddlewareError, verifyPaymentReceipt } = await import("../../seller/dist/index.js");
const { bytes, exact, hex, keys, offer, proof, txHash } = await import("./node-fixtures.mjs");

async function fixtureReceiptDigestMatchesVerifier() {
  const verified = await verifySequencerReceipt(bytes(exact.signedReceipt), bytes(exact.rawTransaction), keys);
  assert.equal(proof.rawTransaction, exact.rawTransaction);
  assert.equal(proof.signedReceipt, exact.signedReceipt);
  assert.equal(proof.receiptDigest, hex(verified.receiptDigest));
  assert.deepEqual(verified.canonicalBytes, bytes(exact.signedReceipt));
}

async function sellerAcceptsFixtureReceiptDigest() {
  const verified = await verifyPaymentReceipt(proof, offer, keys);
  assert.equal(hex(verified.receiptDigest), proof.receiptDigest);
  assert.equal(verified.txHash, txHash);
  assert.equal(verified.rung, "receipted");
  assert.equal(verified.payment.amount, BigInt(offer.amount));
  assert.equal(hex(verified.payment.assetId), offer.asset);
  assert.equal(hex(verified.payment.to), offer.payTo);

  const corruptedDigest = bytes(proof.receiptDigest);
  corruptedDigest[0] ^= 1;
  await assert.rejects(
    verifyPaymentReceipt({ ...proof, receiptDigest: hex(corruptedDigest) }, offer, keys),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure",
  );
}

test(fixtureReceiptDigestMatchesVerifier);
test(sellerAcceptsFixtureReceiptDigest);

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { keccak256, PlatformSdkError } from "@sidiora/layerx-sdk";
import { verifySequencerReceipt } from "../dist/daemon-providers.js";

const vectors = JSON.parse(readFileSync(
  new URL("../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8",
)).cases;
const vector = vectors[0];
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const digest = (value) => Buffer.from(keccak256(bytes(value))).toString("hex");
const context = (entry) => ({ idempotencyKey: "11".repeat(32), activityId: entry.receiptJson.txHash.slice(2) });
const status = (level = "SequencerSigned") => ({ state: "achieved", level });

function daemonReceipt(entry = vector, level = "SequencerSigned") {
  return {
    raw_transaction: entry.rawTransaction.slice(2),
    signed_receipt: entry.signedReceipt.slice(2),
    receipt_key: {
      key_id: entry.pinnedKey.keyId.slice(2), public_key: entry.pinnedKey.publicKey.slice(2),
      first_height: String(entry.pinnedKey.firstHeight), last_height: String(entry.pinnedKey.lastHeight),
    },
    verification_level: level,
  };
}

function refuses(code) {
  return (error) => {
    assert.ok(error instanceof PlatformSdkError);
    assert.equal(error.code, code);
    assert.equal(error.retry, "never");
    return true;
  };
}

async function daemonProviderVerifiesSignedSequencerReceipt() {
  const successful = vectors.filter((entry) => entry.id.startsWith("V1-"));
  assert.ok(successful.length > 0);
  for (const entry of successful) {
    const receiptDigest = digest(entry.signedReceipt);
    const verified = await verifySequencerReceipt(daemonReceipt(entry), status(), context(entry), receiptDigest);
    assert.equal(verified.receiptDigest, receiptDigest);
    assert.deepEqual(verified.evidence, {
      format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
      rawTransaction: entry.rawTransaction, signedReceipt: entry.signedReceipt,
      receiptDigest: `0x${receiptDigest}`,
      receiptKey: { keyId: entry.pinnedKey.keyId, publicKey: entry.pinnedKey.publicKey,
        firstHeight: String(entry.pinnedKey.firstHeight), lastHeight: String(entry.pinnedKey.lastHeight) },
    });
    assert.equal(Object.hasOwn(verified.evidence, "authorizedBatch"), false);
    assert.equal(Object.hasOwn(verified.evidence, "canonicalReceipt"), false);
  }

  const receipt = daemonReceipt();
  const changedSignature = Buffer.from(receipt.signed_receipt, "hex");
  changedSignature[changedSignature.length - 1] ^= 1;
  const changedTransaction = Buffer.from(receipt.raw_transaction, "hex");
  changedTransaction[changedTransaction.length - 1] ^= 1;
  for (const changed of [
    { ...receipt, signed_receipt: changedSignature.toString("hex") },
    { ...receipt, raw_transaction: changedTransaction.toString("hex") },
    { ...receipt, receipt_key: { ...receipt.receipt_key, public_key: "99".repeat(32) } },
    { ...receipt, receipt_key: { ...receipt.receipt_key, key_id: "99".repeat(32) } },
    { ...receipt, receipt_key: { ...receipt.receipt_key, first_height: "1001" } },
    { ...receipt, receipt_key: { ...receipt.receipt_key, first_height: "0", last_height: "999" } },
  ]) {
    await assert.rejects(verifySequencerReceipt(changed, status(), context(vector)), refuses("verification-failure"));
  }
  await assert.rejects(verifySequencerReceipt(receipt, status(), {
    ...context(vector), activityId: "99".repeat(32),
  }), refuses("verification-failure"));
  await assert.rejects(verifySequencerReceipt(receipt, status(), context(vector), "99".repeat(32)),
    refuses("verification-failure"));
  const reverted = vectors.find((entry) => entry.id === "V2-layerxledger-revert");
  assert.ok(reverted);
  await assert.rejects(verifySequencerReceipt(daemonReceipt(reverted), status(), context(reverted)),
    refuses("verification-failure"));

  for (const changed of [
    { canonical_bytes: receipt.signed_receipt, authorised_batch: {}, verification_level: "SequencerSigned" },
    { ...receipt, authorised_batch: {} },
    { ...receipt, signed_receipt: "0x" + receipt.signed_receipt },
    { ...receipt, raw_transaction: "" },
    { ...receipt, receipt_key: { ...receipt.receipt_key, public_key: "00" } },
    ...["-1", "01", "18446744073709551616"].map((first_height) => ({
      ...receipt, receipt_key: { ...receipt.receipt_key, first_height },
    })),
    { ...receipt, receipt_key: { ...receipt.receipt_key, first_height: "2000" } },
  ]) {
    await assert.rejects(verifySequencerReceipt(changed, status(), context(vector)), refuses("decode-failure"));
  }
}

async function daemonFinalityStagesExcludeSettlementAnchored() {
  for (const level of ["SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised"]) {
    const verified = await verifySequencerReceipt(daemonReceipt(vector, level), status(level), context(vector));
    assert.equal(verified.evidence.verificationLevel, "receipted");
    assert.equal(verified.receiptDigest, digest(vector.signedReceipt));
  }
  for (const level of ["SettlementAnchored", "Unverified", "Unknown"]) {
    await assert.rejects(verifySequencerReceipt(daemonReceipt(vector, level), status(level), context(vector)),
      refuses("verification-failure"));
  }
  for (const verificationStatus of [
    status("SettlementAnchored"), status("BatchIncluded"), { state: "pending", level: "SequencerSigned" },
  ]) {
    await assert.rejects(verifySequencerReceipt(daemonReceipt(), verificationStatus, context(vector)),
      refuses("verification-failure"));
  }
}

test(daemonProviderVerifiesSignedSequencerReceipt);
test(daemonFinalityStagesExcludeSettlementAnchored);

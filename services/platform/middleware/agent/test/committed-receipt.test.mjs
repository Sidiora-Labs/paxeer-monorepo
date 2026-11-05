import assert from "node:assert/strict";
import test from "node:test";
import { verifyCommittedAgentPayment } from "../dist/index.js";
import { signedAgentReceiptFixture } from "./protocol-selection.test.mjs";

const { evidence, request, reservation, receiptKeys } = signedAgentReceiptFixture();

export async function committedReceiptConsumer(evidence, reservation, request, receiptKeys) {
  const result = await verifyCommittedAgentPayment(evidence, reservation, request, receiptKeys);
  assert.equal(result.kind, "verified");
  assert.equal(result.verification.payment.amount, BigInt(request.amount));
  assert.equal(Object.hasOwn(result, "submission"), false);
  return result;
}

async function committedReservationReplaysAuthenticatedOutcome() {
  const first = await committedReceiptConsumer(evidence, reservation, request, receiptKeys);
  const replay = await verifyCommittedAgentPayment(evidence, reservation, request, receiptKeys);
  assert.equal(first.kind, "verified");
  assert.deepEqual(replay, first);
  assert.equal(Object.hasOwn(first, "submission"), false);
  assert.equal(first.verification.payment.amount, BigInt(request.amount));
}

async function committedReplayRefusesSubstitutions() {
  const corrupted = Buffer.from(evidence.signedReceipt.slice(2), "hex");
  corrupted[corrupted.length - 1] ^= 1;
  await assert.rejects(verifyCommittedAgentPayment({
    ...evidence, signedReceipt: `0x${corrupted.toString("hex")}`,
  }, reservation, request, receiptKeys));
  await assert.rejects(verifyCommittedAgentPayment(evidence, { ...reservation, receiptDigest: "00".repeat(32) }, request, receiptKeys));
  for (const changed of [
    { ...request, amount: (BigInt(request.amount) + 1n).toString() },
    { ...request, asset: "00".repeat(32) },
    { ...request, recipient: "00".repeat(32) },
  ]) await assert.rejects(verifyCommittedAgentPayment(evidence, reservation, changed, receiptKeys));
}

test(committedReservationReplaysAuthenticatedOutcome);
test(committedReplayRefusesSubstitutions);

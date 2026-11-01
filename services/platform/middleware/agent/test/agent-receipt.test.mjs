import assert from "node:assert/strict";
import test from "node:test";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { AgentMiddlewareError, verifyAgentPayment, verifyCommittedAgentPayment } from "../dist/index.js";
import { signedAgentReceiptFixture } from "./protocol-selection.test.mjs";

const bytes = (value) => Buffer.from(value.slice(2), "hex");

async function verifyAgentPaymentMatchesLedgerLog() {
  const { evidence, request, receiptKeys } = signedAgentReceiptFixture();
  const verification = await verifyAgentPayment(evidence, request, receiptKeys);
  assert.equal(verification.rung, "receipted");
  assert.equal(verification.height, 7n);
  assert.equal(verification.payment.amount, BigInt(request.amount));
  assert.equal(Buffer.from(verification.payment.assetId).toString("hex"), request.asset);
  assert.equal(Buffer.from(verification.payment.to).toString("hex"), request.recipient);
  assert.deepEqual(verification.receiptDigest, keccak256(bytes(evidence.signedReceipt)));
  assert.notEqual(verification.receipt.amount, BigInt(request.amount));
  assert.notEqual(Buffer.from(verification.receipt.asset).toString("hex"), request.asset);
  assert.notEqual(Buffer.from(verification.receipt.to).toString("hex"), request.recipient);
  for (const changed of [
    { ...request, amount: "26" },
    { ...request, asset: "99".repeat(32) },
    { ...request, recipient: "99".repeat(32) },
    { ...request, networkId: "1" },
  ]) await assert.rejects(verifyAgentPayment(evidence, changed, receiptKeys));
  for (const changes of [
    { paymentTo: Buffer.alloc(32, 0x99) },
    { logAddress: "0x000000000000000000000000000000000000101b" },
    { count: 0 }, { count: 2 }, { resultCode: 1n },
  ]) {
    const changed = signedAgentReceiptFixture(changes);
    await assert.rejects(verifyAgentPayment(changed.evidence, request, receiptKeys));
  }
  await assert.rejects(verifyAgentPayment(evidence, request, [{ ...receiptKeys[0], publicKey: new Uint8Array(32) }]));
  await assert.rejects(verifyAgentPayment(evidence, request, []));
  await assert.rejects(verifyAgentPayment(evidence, {
    ...request, commitment: { network: "eip155:125", level: "committed" },
  }, receiptKeys));
}

async function verifyCommittedAgentPaymentRefusesOtherDigest() {
  const { evidence, reservation, request, receiptKeys } = signedAgentReceiptFixture();
  const verified = await verifyCommittedAgentPayment(evidence, reservation, request, receiptKeys);
  assert.equal(verified.kind, "verified");
  const other = signedAgentReceiptFixture({ height: 8n });
  await verifySequencerReceipt(bytes(other.evidence.signedReceipt), bytes(other.evidence.rawTransaction), receiptKeys);
  assert.notEqual(other.evidence.receiptDigest, evidence.receiptDigest);
  await assert.rejects(verifyCommittedAgentPayment(other.evidence, reservation, request, receiptKeys),
    (error) => error instanceof AgentMiddlewareError && error.code === "budget-conflict");
  const kernelDigest = await verifySequencerReceipt(bytes(evidence.signedReceipt), bytes(evidence.rawTransaction), receiptKeys);
  await assert.rejects(verifyCommittedAgentPayment(evidence, {
    ...reservation, receiptDigest: Buffer.from(kernelDigest.receiptDigest).toString("hex"),
  }, request, receiptKeys),
  (error) => error instanceof AgentMiddlewareError && error.code === "budget-conflict");
}

test(verifyAgentPaymentMatchesLedgerLog);
test(verifyCommittedAgentPaymentRefusesOtherDigest);

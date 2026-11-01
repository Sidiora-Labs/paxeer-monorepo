import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { AgentMiddlewareError, verifyAgentPayment } from "../dist/index.js";

const fixture = JSON.parse(await readFile(new URL(
  "../../../../../sdk/platform/conformance/fixtures/receipt-positive-v3.json", import.meta.url,
), "utf8"));
const bytes = (hex) => Uint8Array.from(Buffer.from(hex, "hex"));
const batch = fixture.authorized_batch;
const evidence = {
  canonicalReceipt: bytes(fixture.canonical_receipt_hex),
  authorizedBatch: {
    batchId: bytes(batch.batch_id_hex),
    asset: bytes(batch.asset_hex),
    previousStateRoot: bytes(batch.previous_state_root_hex),
    resultingStateRoot: bytes(batch.resulting_state_root_hex),
    sequencerPublicKey: bytes(batch.sequencer_public_key_hex),
  },
};
const payload = "01";
const request = {
  tenant: "agent-spend-terms",
  preparation: {
    activity_type: "65537",
    actor: "agent:terms:main",
    authority: "terms-authority",
    account_sequence: "1",
    not_before: "0",
    not_after: "1000",
    idempotency_key: "11".repeat(32),
    fee_limit: "1",
    payload,
    payload_hash: createHash("sha256").update("LXP/v1/payload-hash\0")
      .update(Buffer.from(payload, "hex")).digest("hex"),
  },
  networkId: "125",
  authorityHex: "01",
  signerPublicKey: batch.sequencer_public_key_hex,
  submitIdempotencyKey: "22".repeat(32),
  approvalCurrentSequence: "1",
  amount: fixture.expected.amount,
  asset: batch.asset_hex,
  recipient: fixture.expected.to_hex,
  commitment: { network: "eip155:125", level: "receipted" },
};
const selection = { protocolVersion: fixture.expected.protocol_version };

async function agentSpendAcceptsEip155ReceiptedCommitment() {
  const verification = await verifyAgentPayment(evidence, request, undefined, selection);
  assert.equal(verification.receipt.amount, BigInt(request.amount));
  assert.equal(Buffer.from(verification.receipt.asset).toString("hex"), request.asset);
  assert.equal(Buffer.from(verification.receipt.to).toString("hex"), request.recipient);
  assert.equal(Buffer.from(verification.receiptDigest).toString("hex"), fixture.expected.receipt_digest_hex);
}

async function agentSpendRefusesKernelCommitmentTerms() {
  for (const commitment of [
    { network: "layerx:beta", level: "receipted" },
    { network: "layerx:125", level: "receipted" },
    { network: "eip155:125", level: "executed" },
    { network: "eip155:125", level: "batched" },
    { network: "eip155:125", level: "finalised" },
    { network: "layerx:beta", level: "finalised" },
    { network: "eip155:", level: "receipted" },
    { network: "eip155:0x7d", level: "receipted" },
    { network: "eip155:-125", level: "receipted" },
    { network: "eip155:1.25", level: "receipted" },
  ]) {
    await assert.rejects(
      verifyAgentPayment(evidence, { ...request, commitment }, undefined, selection),
      (error) => error instanceof AgentMiddlewareError && error.code === "invalid-request",
      `commitment ${JSON.stringify(commitment)} must be refused`,
    );
  }
}

test(agentSpendAcceptsEip155ReceiptedCommitment);
test(agentSpendRefusesKernelCommitmentTerms);

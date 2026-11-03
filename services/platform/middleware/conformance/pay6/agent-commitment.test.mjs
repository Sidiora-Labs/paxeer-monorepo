import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { verifyAgentPayment as agentVerifyPayment } from "../../agent/dist/index.js";
import { pinnedReceiptKeys } from "./generate_evm_receipt.mjs";

const fixture = JSON.parse(readFileSync(new URL("evm-receipt.json", import.meta.url), "utf8"));
const evidence = fixture.receipt;
const keys = pinnedReceiptKeys(fixture);
const payload = "00";
const request = {
  tenant: "tenant", networkId: "125", authorityHex: "01",
  signerPublicKey: Buffer.from(secp256k1.getPublicKey(new Uint8Array(32).fill(1))).toString("hex"),
  submitIdempotencyKey: "02".repeat(32), approvalCurrentSequence: "0",
  preparation: {
    activity_type: "786433", actor: "actor", authority: "authority", account_sequence: "0",
    not_before: "0", not_after: "1", idempotency_key: "01".repeat(32), fee_limit: "100000",
    payload, payload_hash: createHash("sha256").update("LXP/v1/payload-hash\0").update(Buffer.from(payload, "hex")).digest("hex"),
  },
  asset: fixture.offer.asset, amount: fixture.offer.amount, recipient: fixture.offer.payTo,
};

async function verifyAgentPayment(proof, spend) {
  return agentVerifyPayment(proof, spend, keys);
}

test("agent receipt verification enforces selected commitment before budget commit", async () => {
  const verified = await verifyAgentPayment(evidence, request);
  assert.equal(verified.txHash, fixture.txHash);
  assert.equal(verified.payment.amount, BigInt(request.amount));
  await verifyAgentPayment(evidence, { ...request, commitment: { network: "eip155:125", level: "receipted" } });
  for (const level of ["committed", "executed", "batched", "finalised", "acknowledged"]) await assert.rejects(verifyAgentPayment(evidence, { ...request, commitment: { network: "eip155:125", level } }));
  for (const change of [{ amount: "1" }, { asset: "ab".repeat(32) }, { recipient: "ab".repeat(32) }, { networkId: "1" }]) await assert.rejects(verifyAgentPayment(evidence, { ...request, ...change }));
});

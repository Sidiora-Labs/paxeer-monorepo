import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { paymentCommitment, verifyPaymentReceipt as sellerVerifyPaymentReceipt } from "../../seller/dist/index.js";
import { pinnedReceiptKeys } from "./generate_evm_receipt.mjs";

const fixture = JSON.parse(readFileSync(new URL("evm-receipt.json", import.meta.url), "utf8"));
const evidence = fixture.receipt;
const offer = fixture.offer;
const keys = pinnedReceiptKeys(fixture);

async function verifyPaymentReceipt(proof, requirements) {
  return sellerVerifyPaymentReceipt(proof, requirements, keys);
}

test("EVM receipted payment and amount, asset, recipient binding", async () => {
  const verified = await verifyPaymentReceipt(evidence, offer);
  const payer = Buffer.from(verified.payment.from).toString("hex");
  await verifyPaymentReceipt(evidence, { ...offer, extra: { layerx: { ...offer.extra.layerx, payer } } });
  for (const change of [{ amount: "25001" }, { asset: "01".repeat(32) }, { payTo: "01".repeat(32) }]) {
    await assert.rejects(verifyPaymentReceipt(evidence, { ...offer, ...change }));
  }
  await assert.rejects(verifyPaymentReceipt(evidence, { ...offer, extra: { layerx: { ...offer.extra.layerx, payer: "01".repeat(32) } } }));
  const corrupt = Buffer.from(evidence.signedReceipt.slice(2), "hex");
  corrupt[corrupt.length - 1] ^= 1;
  await assert.rejects(verifyPaymentReceipt({ ...evidence, signedReceipt: `0x${corrupt.toString("hex")}` }, offer));
});

test("explicit commitment cannot downgrade when evidence is missing", async () => {
  for (const commitment of ["committed", "executed", "batched", "finalised", "finalized", "acknowledged", null, 1]) {
    await assert.rejects(verifyPaymentReceipt(evidence, { ...offer, extra: { layerx: { ...offer.extra.layerx, commitment } } }));
  }
  assert.equal(paymentCommitment(), "receipted");
  for (const value of [null, [], {}, "executed"]) assert.throws(() => paymentCommitment({ layerx: value }));
});

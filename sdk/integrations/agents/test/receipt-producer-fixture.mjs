import "./receipt-source-loader.mjs";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

export const sdk = await import("@sidiora/layerx-sdk");
export const middleware = await import("@sidiora/layerx-agent-middleware");
export const daemon = await import("../src/daemon-providers.ts");
export const services = await import("../src/services.ts");
export const config = await import("../src/config.ts");

const { ConformanceSequencer, buildSignedReceipt, buyerPayload, offerFixture } = await import(
  "../../../../services/platform/middleware/conformance/src/receipts.ts"
);

export const transcript = JSON.parse(readFileSync(new URL("./daemon-current-transcript.json", import.meta.url), "utf8"));
export const currentStatus = Object.freeze(transcript.receipt_envelope.verification_status);
export const receiptedStatus = Object.freeze(transcript.receipted_envelope.verification_status);
export const receiptContext = Object.freeze({
  idempotencyKey: "ab".repeat(32), activityId: transcript.committed.activity_id,
});
export const currentKeys = [{
  keyId: Buffer.from(transcript.receipt_key.keyId.slice(2), "hex"),
  publicKey: Buffer.from(transcript.receipt_key.publicKey.slice(2), "hex"),
  first_height: BigInt(transcript.receipt_key.firstHeight), last_height: BigInt(transcript.receipt_key.lastHeight),
}];

export async function currentPortableReceipt() {
  const submission = middleware.decodeDaemonSubmission(transcript.committed);
  const resolved = await middleware.resolveDaemonReceipt(submission,
    Buffer.from(transcript.raw_transaction, "hex"), currentKeys);
  assert.ok(resolved.portableReceipt);
  return resolved.portableReceipt;
}

export async function currentPaymentReceipt() {
  const sequencer = await ConformanceSequencer.generate();
  const transaction = sdk.decodeRawEip1559(Buffer.from(transcript.raw_transaction, "hex"));
  const data = transaction.data;
  assert.equal(data.length, 196);
  const batch = transcript.committed.receipt.authorised_batch;
  const facts = {
    from: data.slice(4, 36), payTo: data.slice(36, 68), asset: data.slice(68, 100),
    amount: BigInt(`0x${Buffer.from(data.slice(100, 132)).toString("hex")}`),
    batchId: Buffer.from(batch.batch_id, "hex"),
    previousStateRoot: Buffer.from(batch.previous_state_root, "hex"),
    resultingStateRoot: Buffer.from(batch.resulting_state_root, "hex"),
  };
  const receipt = await buildSignedReceipt(sequencer, facts);
  const offer = offerFixture(facts.payTo, facts.asset, facts.amount);
  return buyerPayload(offer, receipt, "receipt-audit-payment").payload;
}

export function paymentFacts(proof) {
  const transaction = sdk.decodeRawEip1559(Buffer.from(proof.rawTransaction.slice(2), "hex"));
  assert.equal(transaction.data.length, 196);
  return {
    asset: Buffer.from(transaction.data.slice(68, 100)).toString("hex"),
    amount: BigInt(`0x${Buffer.from(transaction.data.slice(100, 132)).toString("hex")}`).toString(),
    recipient: Buffer.from(transaction.data.slice(36, 68)).toString("hex"),
  };
}

export function createAgentServices(environment) {
  const token = new sdk.SecretBytes(new TextEncoder().encode(environment.LAYERX_GATEWAY_KEY));
  const endpoint = new services.LayerXServiceEndpoint({
    url: environment.LAYERX_RECEIPT_FIXTURE_URL,
    token,
    timeoutMs: 2000,
  });
  return {
    signer: new services.LayerXRemoteSigner(endpoint),
    budgets: new services.LayerXBudgetLedger(endpoint),
    daemonReceiptStorePath: environment.LAYERX_RECEIPT_STORE_PATH,
    daemonReceiptEvidence: new services.LayerXReceiptResolver(endpoint),
    destroy() { token.destroy(); },
  };
}

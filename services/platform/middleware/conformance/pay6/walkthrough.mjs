import assert from "node:assert/strict";
import { AgentHttpTransport, ProductionClient, keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { BuyerMiddleware } from "@sidiora/layerx-buyer-middleware";
import { PaymentRpc, decodePaymentRequiredHeader, decodeSettlementHeader, encodePaymentPayloadHeader,
  paymentCommitment, verifyRpcPayment, verifyPaymentReceipt } from "../../seller/dist/index.js";
import { agentSpend, bytes, exact, hex, keys, offer, payer, payloadFor, proof, txHash } from "./node-fixtures.mjs";
import { paymentPeer } from "./peers.mjs";

export async function mergedHttpEnvelopesSurviveReopen(context) {
  const peer = await paymentPeer(context);
  const rpc = new PaymentRpc(peer.endpoint);
  const hash = await rpc.send(proof.rawTransaction);
  assert.equal(hash, txHash);
  const result = await rpc.receipt(hash);
  const authenticated = await verifySequencerReceipt(bytes(result.receipt), bytes(proof.rawTransaction), keys);
  const legacyDigest = hex(keccak256(bytes(result.receipt)));
  const produced = await verifyRpcPayment(result, proof.rawTransaction, payer, offer, keys);
  assert.equal(produced.kind, "verified");
  assert.equal(produced.receiptDigest, hex(authenticated.receiptDigest));
  assert.equal(produced.receiptDigest, exact.receiptDigest);
  assert.notEqual(produced.receiptDigest, legacyDigest);
  const agentResponse = await fetch(`${peer.endpoint}agent`, { method: "POST",
    headers: { "content-type": "application/json" }, body: JSON.stringify({ proof, spend: agentSpend }) });
  assert.equal(agentResponse.status, 200);
  assert.deepEqual(await agentResponse.json(), { txHash, receiptDigest: produced.receiptDigest,
    network: offer.network, amount: offer.amount, asset: offer.asset, payTo: offer.payTo });
  const challenge = await fetch(`${peer.endpoint}paid`);
  assert.equal(challenge.status, 402);
  assert.deepEqual(decodePaymentRequiredHeader(challenge.headers.get("payment-required")), peer.required);
  const paymentHeader = encodePaymentPayloadHeader({ ...payloadFor(peer.resource), payload: {
    format: produced.format, verificationLevel: produced.verificationLevel, rawTransaction: produced.rawTransaction,
    signedReceipt: produced.signedReceipt, receiptDigest: produced.receiptDigest, receiptKey: produced.receiptKey } });
  const paid = () => fetch(`${peer.endpoint}paid`, { headers: { "payment-signature": paymentHeader } });
  const first = await paid();
  assert.equal(first.status, 200);
  assert.equal(await first.text(), "resource");
  const settlementHeader = first.headers.get("payment-response");
  const settlement = decodeSettlementHeader(settlementHeader);
  assert.equal(settlement.network, "eip155:125");
  assert.equal(settlement.transaction, txHash);
  assert.equal(settlement.extensions.layerx.receiptDigest, proof.receiptDigest);
  const buyer = new BuyerMiddleware({ client: new ProductionClient(new AgentHttpTransport({ endpoint: peer.endpoint })),
    source: "acct:buyer", supported: [{ scheme: "exact", network: offer.network }], receiptKeys: keys });
  const captured = await buyer.captureSettlement(settlementHeader, { ...proof, txHash, offer: { accepted: offer } });
  assert.equal(captured.txHash, txHash);
  assert.equal(captured.receiptDigest, produced.receiptDigest);
  peer.reopen();
  const concurrent = await Promise.all(Array.from({ length: 8 }, paid));
  for (const response of concurrent) {
    assert.equal(response.status, 200);
    assert.equal(await response.text(), "resource");
    assert.equal(response.headers.get("payment-response"), settlementHeader);
  }
  assert.equal(peer.releases, 1);
  const replay = await fetch(`${peer.endpoint}paid`, { headers: { "payment-signature": paymentHeader, "x-principal": "another-payer" } });
  assert.equal(replay.status, 409);
  assert.equal(peer.releases, 1);
  assert.deepEqual(peer.requests.filter((request) => request.method === "POST" && request.url === "/").map((request) => request.body.method),
    ["eth_sendRawTransaction", "pax_sequencerReceipt"]);
  for (const corrupt of [
    { ...proof, receiptDigest: legacyDigest },
    { ...proof, receiptDigest: `0x${"ff".repeat(32)}` },
    { ...proof, signedReceipt: `${proof.signedReceipt.slice(0, -2)}00` },
    { ...proof, verificationLevel: "sequencer-signed" },
  ]) {
    await assert.rejects(verifyPaymentReceipt(corrupt, offer, keys));
  }
}

export async function submittedTracksWithoutPaymentAuthorization() {
  assert.throws(() => paymentCommitment({ layerx: { commitment: "submitted" } }));
  const requirements = { ...offer, extra: { layerx: { ...offer.extra.layerx, commitment: "submitted" } } };
  await assert.rejects(verifyPaymentReceipt(proof, requirements, keys));
  const tracked = await verifyRpcPayment({ txHash, rung: "submitted", nonce: "0x0" }, proof.rawTransaction, payer, offer, keys);
  assert.deepEqual(tracked, { kind: "pending" });
  assert.equal(Object.hasOwn(tracked, "verification"), false);
  assert.equal(Object.hasOwn(tracked, "signedReceipt"), false);
}

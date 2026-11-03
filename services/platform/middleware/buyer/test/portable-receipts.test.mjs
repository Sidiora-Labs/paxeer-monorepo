import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { ProductionClient, keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  PAYMENT_RESPONSE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  decodePaymentExactLog,
  decodePaymentPayloadHeader,
  decodeSettlementHeader,
  encodePaymentRequiredHeader,
  encodeSettlementHeader,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";
import { BuyerMiddleware, evidenceRung } from "../dist/index.js";

const document = JSON.parse(readFileSync(new URL("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8"));
const vector = document.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.equal(document.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
assert.ok(vector);
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const pin = {
  keyId: bytes(vector.pinnedKey.keyId),
  publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight),
  last_height: BigInt(vector.pinnedKey.lastHeight),
};
const proof = {
  format: "layerx-receipt-proof-v2",
  verificationLevel: "receipted",
  rawTransaction: vector.rawTransaction,
  signedReceipt: vector.signedReceipt,
  receiptDigest: hex(keccak256(bytes(vector.signedReceipt))),
  receiptKey: {
    keyId: vector.pinnedKey.keyId,
    publicKey: vector.pinnedKey.publicKey,
    firstHeight: `0x${pin.first_height.toString(16)}`,
    lastHeight: `0x${pin.last_height.toString(16)}`,
  },
};
const log = vector.receiptJson.logs[0];
const paid = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
const requirements = {
  scheme: "exact",
  network: "eip155:125",
  amount: paid.amount.toString(),
  asset: hex(paid.assetId),
  payTo: hex(paid.to),
  maxTimeoutSeconds: 120,
  extra: { layerx: {
    commitment: "receipted",
    account: "agent:did:layerx:payee:main",
    currency: "LXP",
    payer: hex(paid.from).slice(2),
    purposeHash: hex(paid.paymentReference).slice(2),
  } },
};
const required = { x402Version: 2, resource: { url: "https://merchant.example/paid" }, accepts: [requirements] };
const requiredHeader = encodePaymentRequiredHeader(required);
const golden = (operation) => JSON.parse(readFileSync(new URL(`../../../../human/schema/human-api/golden/${operation}.response.json`, import.meta.url), "utf8")).body.result;
const now = Date.parse("2026-08-18T09:32:00Z");

class RecordedHumanTransport {
  calls = [];

  constructor({ verification = "receipted", contentType = "application/vnd.layerx.receipt-proof-v2+json", portable = proof } = {}) {
    const evidence = golden("evidence.get");
    this.evidence = {
      ...evidence,
      verification,
      content_type: contentType,
      bytes_base64: Buffer.from(JSON.stringify(portable)).toString("base64"),
    };
    const reference = { evidence_id: evidence.evidence_id, class: evidence.class, verification };
    const journey = golden("move.commit");
    this.journey = {
      ...journey,
      evidence: [reference],
      stages: [{ ...journey.stages[0], evidence: [reference] }],
    };
    const quote = golden("move.quote");
    this.quote = { ...quote, money: { ...quote.money, amount: requirements.amount } };
  }

  async call(call) {
    this.calls.push(structuredClone(call));
    assert.equal(call.plane, "human");
    switch (call.operation) {
      case "move.quote":
        assert.deepEqual(call.request, {
          source: "acct:buyer-portable-receipts",
          destination: requirements.extra.layerx.account,
          money: { amount: requirements.amount, currency: "LXP" },
        });
        return structuredClone(this.quote);
      case "move.commit":
        assert.deepEqual(call.request, { quote_id: this.quote.quote_id });
        assert.equal(call.idempotencyKey, "portable-receipt-payment");
        return structuredClone({ ...this.journey, state: "processing" });
      case "journey.get":
        assert.deepEqual(call.request, { journey_id: this.journey.journey_id });
        return structuredClone(this.journey);
      case "evidence.get":
        assert.deepEqual(call.request, { evidence_id: this.evidence.evidence_id });
        return structuredClone(this.evidence);
      default:
        throw new Error(`Unexpected human operation: ${call.operation}`);
    }
  }
}

function fixture(options, receiptKeys = [pin], commitments) {
  const transport = new RecordedHumanTransport(options);
  const buyer = new BuyerMiddleware({
    client: new ProductionClient(transport),
    source: "acct:buyer-portable-receipts",
    supported: [{ scheme: "exact", network: "eip155:125" }],
    receiptKeys,
    commitments,
    now: () => now,
    retry: { maximumQuoteAttempts: 1, maximumCommitAttempts: 1, maximumJourneyPolls: 1, baseDelayMs: 0 },
  });
  return { buyer, transport, prepare: () => buyer.prepare(requiredHeader, "portable-receipt-payment") };
}

function verificationFailure(error) {
  return error instanceof MiddlewareError && error.code === "verification-failure";
}

class FulfillmentRecords {
  records = new Map();

  async get(key) {
    return structuredClone(this.records.get(key));
  }

  async fulfill(proposed, release) {
    const stored = await this.get(proposed.idempotencyKey);
    if (stored !== undefined) {
      if (stored.requestDigest !== proposed.requestDigest) throw new MiddlewareError("fulfillment-conflict");
      return stored;
    }
    const record = structuredClone({ ...proposed, resource: await release() });
    this.records.set(proposed.idempotencyKey, record);
    return structuredClone(record);
  }
}

async function prepareSendsPortableReceiptFromEvidence() {
  const run = fixture();
  const result = await run.prepare();
  assert.equal(result.kind, "ready");
  const payment = result.payment;
  const wire = decodePaymentPayloadHeader(payment.paymentHeader);
  assert.deepEqual(wire.payload, proof);
  assert.deepEqual(payment.payload, wire);
  assert.equal(payment.rawTransaction, proof.rawTransaction);
  assert.equal(payment.signedReceipt, proof.signedReceipt);
  assert.equal(payment.receiptDigest, proof.receiptDigest);
  assert.deepEqual(payment.receiptKey, proof.receiptKey);
  assert.equal(payment.txHash, hex(keccak256(bytes(proof.rawTransaction))));
  assert.equal(payment.verification.rung, "receipted");
  assert.equal(payment.verification.payment.amount, paid.amount);
  assert.equal("canonicalReceipt" in payment, false);
  assert.equal("authorizedBatch" in payment, false);
  assert.deepEqual(run.transport.calls.map((call) => call.operation), ["move.quote", "move.commit", "journey.get", "evidence.get"]);
  assert.equal(run.transport.calls.filter((call) => call.operation === "evidence.get").length, 1);

  const badPin = { ...pin, publicKey: new Uint8Array(32).fill(0xff) };
  await assert.rejects(fixture(undefined, [badPin]).prepare(), verificationFailure);
  await assert.rejects(fixture({ portable: { ...proof, receiptDigest: vector.receiptDigest } }).prepare(), verificationFailure);
  await assert.rejects(verifyPaymentReceipt(proof, {
    ...requirements, amount: (paid.amount + 1n).toString(),
  }, [pin]), verificationFailure);
  await assert.rejects(new ReceiptPayloadAuthority([pin]).settle({
    principal: "merchant", payload: { ...wire, payload: { ...proof, receiptDigest: `0x${"00".repeat(32)}` } },
    requirements, idempotencyKey: "tampered", requestDigest: "tampered",
  }), verificationFailure);
}

async function captureSettlementRequiresTransactionHash() {
  const run = fixture();
  const prepared = await run.prepare();
  assert.equal(prepared.kind, "ready");
  const seller = new SellerMiddleware({
    paymentRequired: required,
    receiptKeys: [pin],
    authority: new ReceiptPayloadAuthority([pin]),
    fulfillments: new FulfillmentRecords(),
  });
  const released = await seller.handle("merchant", prepared.payment.paymentHeader, async () => ({ access: "paid" }));
  assert.equal(released.kind, "released");
  const header = released.headers[PAYMENT_RESPONSE_HEADER];
  const response = decodeSettlementHeader(header);
  assert.equal(response.transaction, prepared.payment.txHash);
  const legacy = { ...response, transaction: `lxp:${proof.receiptDigest}` };
  await assert.rejects(run.buyer.captureSettlement(encodeSettlementHeader(legacy), prepared.payment), verificationFailure);
  const captured = await run.buyer.captureSettlement(header, prepared.payment);
  assert.deepEqual(captured.response, response);
  assert.equal(captured.txHash, prepared.payment.txHash);
  assert.equal(captured.rawTransaction, proof.rawTransaction);
  assert.equal(captured.signedReceipt, proof.signedReceipt);
  assert.deepEqual(captured.receiptKey, proof.receiptKey);
  assert.equal(captured.verification.rung, "receipted");
  for (const changed of [
    { ...proof, signedReceipt: `${proof.signedReceipt.slice(0, -2)}00` },
    { ...proof, rawTransaction: `${proof.rawTransaction.slice(0, -2)}00` },
    { ...proof, verificationLevel: "sequencer-signed" },
    { ...proof, format: "layerx-receipt-proof-v1" },
    { receipt: Buffer.from(bytes(proof.signedReceipt)).toString("base64"), receiptDigest: proof.receiptDigest, verificationLevel: "sequencer-signed" },
    { ...proof, receiptKey: { ...proof.receiptKey, publicKey: `0x${"ff".repeat(32)}` } },
  ]) {
    await assert.rejects(run.buyer.captureSettlement(encodeSettlementHeader({
      ...response, extensions: { layerx: changed },
    }), prepared.payment), verificationFailure);
  }
  await assert.rejects(run.buyer.captureSettlement(encodeSettlementHeader({
    ...response, transaction: `0x${"ff".repeat(32)}`,
  }), prepared.payment), verificationFailure);
}

async function findMatchingReceiptIgnoresKernelEvidence() {
  for (const verification of ["receipt-verified", "checkpoint-finalised"]) {
    const run = fixture({ verification, contentType: "application/vnd.layerx.receipt" });
    const result = await run.prepare();
    assert.equal(result.kind, "pending");
    assert.equal(result.journey.journey_id, run.transport.journey.journey_id);
    assert.deepEqual(run.transport.calls.map((call) => call.operation), ["move.quote", "move.commit", "journey.get", "evidence.get"]);
  }
  const oldContent = fixture({ contentType: "application/vnd.layerx.receipt" });
  assert.equal((await oldContent.prepare()).kind, "pending");
  assert.equal(oldContent.transport.calls.filter((call) => call.operation === "evidence.get").length, 1);
  for (const portable of [{ ...proof, verificationLevel: "sequencer-signed" }, { ...proof, format: "layerx-receipt-proof-v1" }]) {
    await assert.rejects(fixture({ portable }).prepare(), verificationFailure);
  }
}

test(prepareSendsPortableReceiptFromEvidence);
test(captureSettlementRequiresTransactionHash);
test(findMatchingReceiptIgnoresKernelEvidence);

function evidenceRungMapsHumanLabels() {
  assert.equal(evidenceRung("receipt-verified"), "receipted");
  assert.equal(evidenceRung("checkpoint-finalised"), "committed");
  assert.equal(evidenceRung("receipted"), "receipted");
  assert.equal(evidenceRung("committed"), "committed");
  for (const label of ["unknown", "submitted", "sequencer-signed", "Receipt-verified", ""]) {
    assert.equal(evidenceRung(label), undefined);
  }
}

function signedCommitment(verified) {
  const secrets = [1, 2, 3, 4].map((value) => new Uint8Array(32).fill(value));
  const validatorSet = secrets.map((secret) => {
    const publicKey = ed25519.getPublicKey(secret);
    return { publicKey, address: Uint8Array.from(createHash("sha256").update(publicKey).digest().subarray(0, 20)),
      votingPower: 1n };
  });
  const blockId = { hash: new Uint8Array(32).fill(0x11),
    partSetHeader: { total: 1, hash: new Uint8Array(32).fill(0x22) } };
  const height = verified.height;
  const heightBytes = Buffer.alloc(8);
  heightBytes.writeBigInt64LE(height);
  const signBytes = Buffer.concat([
    Buffer.from("65080211", "hex"), heightBytes,
    Buffer.from("22480a20", "hex"), blockId.hash,
    Buffer.from("122408011220", "hex"), blockId.partSetHeader.hash,
    Buffer.from("2a020801320a7061786565722d313235", "hex"),
  ]);
  const signatures = validatorSet.slice(0, 3).map((validator, index) => ({
    blockIdFlag: 2, validatorAddress: validator.address, timestamp: { seconds: 1n, nanos: 0 },
    signature: ed25519.sign(signBytes, secrets[index]),
  }));
  return { landingBlockHeight: height, blockHeader: { chainId: "paxeer-125", blockId },
    receiptInclusion: { transactionHash: verified.receipt.activityId.slice(),
      receiptDigest: verified.receiptDigest.slice(), sequence: verified.receipt.globalSequence },
    commit: { height, round: 0, blockId, signatures }, validatorSet };
}

async function prepareSelectsHumanEvidenceRungs() {
  const receipted = await fixture({ verification: "receipt-verified" }).prepare();
  assert.equal(receipted.kind, "ready");
  assert.equal(receipted.payment.verification.rung, "receipted");
  assert.equal(decodePaymentPayloadHeader(receipted.payment.paymentHeader).payload.verificationLevel, "receipted");

  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [pin]);
  const commitment = signedCommitment(verified);
  const resolutions = [];
  const commitments = { async resolve(transactionHash, network) {
    resolutions.push({ transactionHash, network });
    return structuredClone(commitment);
  } };
  const committed = await fixture({ verification: "checkpoint-finalised",
    portable: { ...proof, verificationLevel: "committed" } }, [pin], commitments).prepare();
  assert.equal(committed.kind, "ready");
  assert.equal(committed.payment.verification.rung, "committed");
  assert.equal(decodePaymentPayloadHeader(committed.payment.paymentHeader).payload.verificationLevel, "committed");
  assert.deepEqual(committed.payment.verification.commitmentEvidence, commitment);
  assert.deepEqual(resolutions, [{ transactionHash: hex(keccak256(bytes(proof.rawTransaction))), network: "eip155:125" }]);
  await assert.rejects(fixture({ verification: "checkpoint-finalised" }).prepare(), verificationFailure);
  await assert.rejects(fixture({ verification: "checkpoint-finalised",
    portable: { ...proof, verificationLevel: "committed" } }).prepare(), verificationFailure);

  const unknown = fixture({ verification: "unknown" });
  assert.equal((await unknown.prepare()).kind, "pending");
  assert.equal(unknown.transport.calls.filter((call) => call.operation === "evidence.get").length, 0);
  const unknownMaterial = fixture();
  unknownMaterial.transport.evidence.verification = "unknown";
  assert.equal((await unknownMaterial.prepare()).kind, "pending");
}

test(evidenceRungMapsHumanLabels);
test(prepareSelectsHumanEvidenceRungs);

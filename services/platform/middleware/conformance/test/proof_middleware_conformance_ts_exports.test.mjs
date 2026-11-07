// Proves: receipts.ts::ConformanceSequencer/buildSignedReceipt/buyerPayload and scenarios.ts::runScenarios/service.mjs::runServiceScenarios -> dist/receipts.js, dist/scenarios.js and real payment headers/RPC/HTTP -> run.mjs::runConformance/conformanceReport and paid-api/buyer receipt consumers.
// Repairs receipts.ts::ConformanceSequencer.publicKey exposing mutable signer
// authority, run.mjs accepting malformed, truthy check results as success, and
// service.mjs converting producer receipt-key heights to decimal before the
// hex-only LayerXReceiptResolver could decode them (leaving verified spends pending).
// ReceiptFacts, SignedReceipt and OfferFixture flow through the compiled exports;
// FixedReceiptKeys supplies the actual verification authority, never a double.
// Inventory finding: spec/layerx-merge/interfaces.kvx names
// services/platform/hosted/core/src/main.rs
// as a TypeScript ReceiptFacts caller. That file's ReceiptFacts/receipt_facts are
// independent native Rust facts and an authenticated native receipt decoder;
// it has no TypeScript module crossing. The executable consumer here is run.mjs.
import assert from "node:assert/strict";
import test from "node:test";
import {
  MiddlewareError,
  ReceiptPayloadAuthority,
  decodePaymentPayloadHeader,
  decodePaymentRequiredHeader,
  encodePaymentPayloadHeader,
  encodePaymentRequiredHeader,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  fixedBytes,
  offerFixture,
} from "../dist/receipts.js";
import { runScenarios } from "../dist/scenarios.js";
import { FixedReceiptKeys, Suite } from "../dist/support.js";
import { conformanceReport, runConformance } from "../run.mjs";

const verificationFailure = (error) => error instanceof MiddlewareError && error.code === "verification-failure";

async function middlewareConformanceReceiptExportsCrossRealMiddleware() {
  const sequencer = await ConformanceSequencer.generate();
  const originalKey = sequencer.receiptKey();
  sequencer.publicKey.fill(0);
  const exposedKey = sequencer.receiptKey();
  exposedKey.publicKey.fill(0);
  exposedKey.keyId.fill(0);
  assert.deepEqual(sequencer.publicKey, originalKey.publicKey);
  assert.deepEqual(sequencer.receiptKey(), originalKey);

  const facts = {
    asset: fixedBytes(0x11), payTo: fixedBytes(0x22), amount: 25n,
    from: fixedBytes(0x33), batchId: fixedBytes(0x44),
    previousStateRoot: fixedBytes(0x55), resultingStateRoot: fixedBytes(0x66),
  };
  const offer = offerFixture(facts.payTo, facts.asset, facts.amount);
  const required = decodePaymentRequiredHeader(encodePaymentRequiredHeader(offer.paymentRequired));
  assert.deepEqual(required.accepts, [offer.requirements]);
  const keys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const authority = new ReceiptPayloadAuthority(keys);
  const seen = [];
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const producedFacts = { ...facts, from: fixedBytes(0x33 + attempt) };
    const receipt = await buildSignedReceipt(sequencer, producedFacts);
    const idempotencyKey = `exports-payment-${attempt}`;
    const payload = decodePaymentPayloadHeader(encodePaymentPayloadHeader(buyerPayload(offer, receipt, idempotencyKey)));
    const settled = await authority.settle({
      principal: "conformance-exports", payload, requirements: offer.requirements,
      idempotencyKey, requestDigest: `exports-request-${attempt}`,
    });
    assert.equal(settled.kind, "settled");
    assert.equal(settled.signedReceipt, receipt.signedReceipt);
    assert.equal(settled.receiptDigest, receipt.receiptDigest);
    const observed = await verifyPaymentReceipt(settled, offer.requirements, keys);
    assert.equal(observed.rung, "receipted");
    assert.equal(observed.txHash, receipt.txHash);
    assert.equal(observed.payment.amount, facts.amount);
    assert.deepEqual(observed.payment.from, producedFacts.from);
    assert.deepEqual(observed.payment.to, facts.payTo);
    assert.deepEqual(observed.payment.assetId, facts.asset);
    seen.push(receipt.txHash);

    const malformed = JSON.parse(Buffer.from(encodePaymentPayloadHeader(payload), "base64").toString("utf8"));
    malformed.payload.signedReceipt = 7;
    // services/platform/middleware/seller/src/index.ts::decodePaymentPayloadHeader
    // decodes the x402 envelope; ReceiptPayloadAuthority.settle must decode and
    // refuse malformed LayerX evidence before it can produce a settlement.
    const malformedPayload = decodePaymentPayloadHeader(Buffer.from(JSON.stringify(malformed)).toString("base64"));
    assert.equal(malformedPayload.payload.signedReceipt, 7);
    await assert.rejects(
      authority.settle({
        principal: "conformance-exports", payload: malformedPayload, requirements: offer.requirements,
        idempotencyKey: `${idempotencyKey}-invalid-type`, requestDigest: `invalid-type-${attempt}`,
      }),
      (error) => error instanceof MiddlewareError && error.code === "invalid-payment-payload",
    );
    const truncated = decodePaymentPayloadHeader(encodePaymentPayloadHeader(buyerPayload(
      offer, { ...receipt, signedReceipt: receipt.signedReceipt.slice(0, -2) }, `${idempotencyKey}-malformed`,
    )));
    await assert.rejects(authority.settle({
      principal: "conformance-exports", payload: truncated, requirements: offer.requirements,
      idempotencyKey: `${idempotencyKey}-malformed`, requestDigest: `malformed-${attempt}`,
    }), verificationFailure);
  }
  assert.notEqual(seen[0], seen[1], "the second observed transaction must come from fresh producer state");
  const foreign = await ConformanceSequencer.generate();
  const foreignReceipt = await buildSignedReceipt(foreign, facts);
  const foreignPayload = decodePaymentPayloadHeader(encodePaymentPayloadHeader(buyerPayload(offer, foreignReceipt, "foreign")));
  await assert.rejects(authority.settle({
    principal: "conformance-exports", payload: foreignPayload, requirements: offer.requirements,
    idempotencyKey: "foreign", requestDigest: "foreign-request",
  }), verificationFailure);
  await assert.rejects(buildSignedReceipt(sequencer, { ...facts, asset: new Uint8Array(31) }), /32-byte words/u);
}

async function middlewareConformanceRunnerObservesFreshProducerRuns() {
  const runs = [];
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const report = await runConformance();
    assert.equal(report.exitCode, 0, JSON.stringify(report.results.filter((result) => !result.ok)));
    assert.equal(report.failures, 0);
    assert.ok(report.results.length > 13, "both in-process and all service scenarios must execute");
    assert.equal(new Set(report.results.map((result) => result.name)).size, report.results.length);
    for (const name of [
      "seller: a valid receipt releases the resource under receipted evidence",
      "seller: a receipt signed by an unauthorised sequencer is rejected",
      "service: a valid receipt unlocks the resource and settles over the wire",
      "service: a tampered payment cannot unlock the resource",
      "service: replaying the same payment is idempotent",
    ]) {
      assert.equal(report.results.find((result) => result.name === name)?.ok, true, name);
    }
    const { observations, receiptKey, config } = report.serviceEvidence;
    assert.equal(config.application, "paid-api");
    assert.equal(observations.payments.length, 2);
    assert.equal(observations.restarted, true, "stored fulfillment must survive closing and reopening the paid API");
    assert.equal(observations.replay.signedReceipt, observations.payments[0].signedReceipt);
    assert.equal(observations.replay.txHash, observations.payments[0].txHash);
    assert.equal(observations.replay.resource, observations.payments[0].resource);
    assert.equal(observations.malformed.status, 402);
    assert.equal(observations.malformed.state, "refused");
    assert.equal(observations.malformed.detail, "sequencer_receipt_verification_failed");
    for (const payment of observations.payments) {
      assert.equal(payment.status, 200);
      assert.equal(payment.observedTransaction, payment.txHash);
      assert.equal(payment.observedReceipt, payment.signedReceipt);
      assert.deepEqual(JSON.parse(payment.resource), { secret: "paid-resource-body", price: "250000" });
      assert.ok(payment.receiptReads > 0, "each result must cross the live receipt RPC transport");
    }
    assert.notEqual(observations.payments[0].txHash, observations.payments[1].txHash);
    assert.notEqual(observations.payments[0].receiptDigest, observations.payments[1].receiptDigest);
    runs.push({ receiptKey, results: report.results });
  }
  assert.notDeepEqual(runs[0].receiptKey.keyId, runs[1].receiptKey.keyId, "a second call must generate fresh signer authority");
  assert.notStrictEqual(runs[0].results, runs[1].results, "the runner must consume a fresh suite on every call");
  assert.deepEqual(runs[0].results.map((result) => result.name), runs[1].results.map((result) => result.name));
}

async function middlewareConformanceRunnerRefusesMalformedProducerResults() {
  const suite = await runScenarios();
  assert.equal(conformanceReport(suite).exitCode, 0);
  const result = suite.results()[0];
  const original = { ...result };
  result.ok = "true";
  assert.throws(() => conformanceReport(suite), /invalid_conformance_result/u);
  Object.assign(result, original);
  result.name = "";
  assert.throws(() => conformanceReport(suite), /invalid_conformance_result/u);
  Object.assign(result, original);
  result.detail = { message: "malformed detail" };
  assert.throws(() => conformanceReport(suite), /invalid_conformance_result/u);
  delete result.detail;
  Object.assign(result, original);
  assert.equal(conformanceReport(suite).exitCode, 0);
  assert.throws(() => conformanceReport(new Suite()), /invalid_conformance_results/u);
  const failed = new Suite();
  await failed.check("a real failed check", () => { throw new Error("asserted failure"); });
  const report = conformanceReport(failed);
  assert.equal(report.exitCode, 1);
  assert.equal(report.failures, 1);
  assert.equal(report.results[0].detail, "asserted failure");
}

test(middlewareConformanceReceiptExportsCrossRealMiddleware);
test(middlewareConformanceRunnerObservesFreshProducerRuns);
test(middlewareConformanceRunnerRefusesMalformedProducerResults);

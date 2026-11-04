import assert from "node:assert/strict";
import test from "node:test";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  MiddlewareError,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  encodePaymentPayloadHeader,
} from "@sidiora/layerx-seller-middleware";
import { DurableFulfillmentRepository } from "../../seller/dist/fulfillment.js";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  fixedBytes,
  offerFixture,
} from "../dist/receipts.js";
import { assertConcurrentReceiptFulfillment } from "../dist/scenarios.js";
import { FixedReceiptKeys, InMemoryFulfillmentRepository } from "../dist/support.js";

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

function middlewareError(code) {
  return (error) => error instanceof MiddlewareError && error.code === code;
}

async function paymentFixture() {
  const sequencer = await ConformanceSequencer.generate();
  const facts = {
    asset: fixedBytes(0xbb), payTo: fixedBytes(0xaa), amount: 250_000n,
    from: fixedBytes(0xcc), batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2), resultingStateRoot: fixedBytes(0xd3),
  };
  const offer = offerFixture(facts.payTo, facts.asset, facts.amount);
  const receipt = await buildSignedReceipt(sequencer, facts);
  const resolver = new FixedReceiptKeys([sequencer.receiptKey()]);
  const keys = await resolver.resolve();
  const header = encodePaymentPayloadHeader(buyerPayload(offer, receipt, "repair-22-33-1481"));
  const seller = (fulfillments) => new SellerMiddleware({
    paymentRequired: offer.paymentRequired, receiptKeys: keys,
    authority: new ReceiptPayloadAuthority(keys), fulfillments,
  });
  // Obtain the repository proposal from the actual seller's receipt/request binding.
  const seed = new InMemoryFulfillmentRepository();
  let key;
  const decision = await seller(seed).handle("repair-principal", header, async (claimedKey) => {
    key = claimedKey;
    return "seed-resource";
  });
  assert.equal(decision.kind, "released");
  const { resource, ...proposed } = await seed.get(key);
  assert.equal(resource, "seed-resource");
  return { offer, receipt, resolver, header, seller, proposed };
}

async function memory_fulfillment_concurrent_replays_share_one_release() {
  const { proposed } = await paymentFixture();
  const repository = new InMemoryFulfillmentRepository();
  const entered = deferred();
  const finish = deferred();
  let releases = 0;
  const first = repository.fulfill(proposed, async () => {
    releases += 1;
    entered.resolve();
    await finish.promise;
    return { secret: "paid-resource" };
  });
  await entered.promise;
  assert.equal(await repository.get(proposed.idempotencyKey), undefined);
  const replays = Array.from({ length: 16 }, () => repository.fulfill(proposed, async () => {
    releases += 1;
    return { secret: "duplicate" };
  }));
  finish.resolve();
  const records = await Promise.all([first, ...replays]);
  assert.equal(releases, 1);
  assert.equal(repository.releaseCount, 1);
  for (const record of records) {
    assert.strictEqual(record, records[0]);
    assert.deepEqual(record.resource, { secret: "paid-resource" });
    assert.equal(record.receiptDigest, proposed.receiptDigest);
    assert.equal(record.txHash, proposed.txHash);
  }
  assert.strictEqual(await repository.get(proposed.idempotencyKey), records[0]);
  assert.strictEqual(await repository.fulfill(proposed, async () => {
    releases += 1;
    return "unexpected-replay";
  }), records[0]);
  assert.equal(releases, 1);
}

async function memory_fulfillment_claim_precedes_reentrant_release_and_is_per_receipt() {
  const { proposed } = await paymentFixture();
  const repository = new InMemoryFulfillmentRepository();
  let nested;
  let releases = 0;
  const first = await repository.fulfill(proposed, async () => {
    releases += 1;
    nested = repository.fulfill(proposed, async () => {
      releases += 1;
      return "duplicate";
    });
    const other = await repository.fulfill({ ...proposed, idempotencyKey: "f".repeat(64) }, async () => {
      releases += 1;
      return "independent-resource";
    });
    assert.equal(other.resource, "independent-resource");
    return "original-resource";
  });
  assert.strictEqual(await nested, first);
  assert.equal(first.resource, "original-resource");
  assert.equal(releases, 2);
  assert.equal(repository.releaseCount, 2);
}

async function memory_fulfillment_conflicts_are_typed_before_and_after_release() {
  const { proposed } = await paymentFixture();
  const repository = new InMemoryFulfillmentRepository();
  const entered = deferred();
  const finish = deferred();
  let releases = 0;
  const first = repository.fulfill(proposed, async () => {
    releases += 1;
    entered.resolve();
    await finish.promise;
    return "original-resource";
  });
  await entered.promise;
  const competing = { ...proposed, requestDigest: "e".repeat(64) };
  const duplicateRelease = async () => { releases += 1; return "conflict"; };
  try {
    await assert.rejects(repository.fulfill(competing, duplicateRelease), middlewareError("fulfillment-conflict"));
  } finally {
    finish.resolve();
    await first;
  }
  await assert.rejects(repository.fulfill(competing, duplicateRelease), middlewareError("fulfillment-conflict"));
  assert.equal(releases, 1);
  assert.equal(repository.releaseCount, 1);
  assert.equal((await repository.get(proposed.idempotencyKey)).resource, "original-resource");
}

async function memory_fulfillment_failed_release_retains_uncertain_claim() {
  const { proposed } = await paymentFixture();
  for (const synchronous of [false, true]) {
    const repository = new InMemoryFulfillmentRepository();
    const entered = deferred();
    const finish = deferred();
    const failure = new Error("release-interrupted-after-side-effect");
    let releases = 0;
    const first = repository.fulfill(proposed, synchronous
      ? () => { releases += 1; throw failure; }
      : async () => {
        releases += 1;
        entered.resolve();
        await finish.promise;
        throw failure;
      });
    const firstRefusal = assert.rejects(first, (error) => error === failure);
    if (!synchronous) {
      await entered.promise;
      const replay = repository.fulfill(proposed, async () => { releases += 1; return "duplicate"; });
      const replayRefusal = assert.rejects(replay, (error) => error === failure);
      finish.resolve();
      await replayRefusal;
    }
    await firstRefusal;
    await assert.rejects(repository.fulfill(proposed, async () => {
      releases += 1;
      return "duplicate";
    }), middlewareError("fulfillment-outcome-unknown"));
    await assert.rejects(repository.fulfill({ ...proposed, requestDigest: "e".repeat(64) }, async () => {
      releases += 1;
      return "conflicting";
    }), middlewareError("fulfillment-conflict"));
    assert.equal(await repository.get(proposed.idempotencyKey), undefined);
    assert.equal(releases, 1);
    assert.equal(repository.releaseCount, 0);
  }
}

async function memory_fulfillment_real_seller_concurrency_scenario() {
  const { offer, receipt, resolver } = await paymentFixture();
  await assertConcurrentReceiptFulfillment(offer, receipt, resolver);
}

async function fulfillment_restart_recovers_durable_resource_and_receipt(t) {
  const fixture = await paymentFixture();
  const directory = await mkdtemp(join(tmpdir(), "fulfillment-restart-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  let repository = new DurableFulfillmentRepository(directory);
  let seller = fixture.seller(repository);
  let releases = 0;
  let key;
  const first = await seller.handle("repair-principal", fixture.header, async (claimedKey) => {
    releases += 1;
    key = claimedKey;
    const claim = JSON.parse(await readFile(join(directory, key, "claim.json"), "utf8"));
    assert.equal(claim.requestDigest, fixture.proposed.requestDigest);
    return { secret: "durable-paid-resource" };
  });
  assert.equal(first.kind, "released");
  const stored = await repository.get(key);
  // This repository has no open handles; discard both consumers, then reopen its disk state.
  seller = undefined;
  repository = undefined;
  const reopened = new DurableFulfillmentRepository(directory);
  assert.deepEqual(await reopened.get(key), stored);
  const replay = await fixture.seller(reopened).handle("repair-principal", fixture.header, async () => {
    releases += 1;
    return { secret: "duplicate" };
  });
  assert.equal(replay.kind, "released");
  assert.deepEqual(replay.resource, first.resource);
  assert.deepEqual(replay.settlement, first.settlement);
  assert.equal(replay.settlement.transaction, fixture.receipt.txHash);
  assert.equal(stored.signedReceipt, fixture.receipt.signedReceipt);
  const conflict = await fixture.seller(reopened).handle("other-principal", fixture.header, async () => {
    releases += 1;
    return "conflicting";
  });
  assert.equal(conflict.kind, "refused");
  assert.equal(conflict.status, 409);
  assert.equal(conflict.settlement.errorReason, "receipt_reuse");
  assert.equal(releases, 1);
}

async function fulfillment_restart_retains_interrupted_durable_claim(t) {
  const fixture = await paymentFixture();
  const directory = await mkdtemp(join(tmpdir(), "fulfillment-uncertain-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  let repository = new DurableFulfillmentRepository(directory);
  let seller = fixture.seller(repository);
  let releases = 0;
  let key;
  const failure = new Error("interrupted-after-side-effect");
  await assert.rejects(seller.handle("repair-principal", fixture.header, async (claimedKey) => {
    key = claimedKey;
    const claim = JSON.parse(await readFile(join(directory, key, "claim.json"), "utf8"));
    assert.equal(claim.requestDigest, fixture.proposed.requestDigest);
    releases += 1;
    throw failure;
  }), (error) => error === failure);
  seller = undefined;
  repository = undefined;
  const reopened = new DurableFulfillmentRepository(directory);
  assert.equal(await reopened.get(key), undefined);
  const releaseAgain = async () => { releases += 1; return "duplicate"; };
  await assert.rejects(fixture.seller(reopened).handle("repair-principal", fixture.header, releaseAgain),
    middlewareError("fulfillment-outcome-unknown"));
  const conflict = await fixture.seller(reopened).handle("other-principal", fixture.header, releaseAgain);
  assert.equal(conflict.kind, "refused");
  assert.equal(conflict.status, 409);
  assert.equal(releases, 1);
}

test(memory_fulfillment_concurrent_replays_share_one_release);
test(memory_fulfillment_claim_precedes_reentrant_release_and_is_per_receipt);
test(memory_fulfillment_conflicts_are_typed_before_and_after_release);
test(memory_fulfillment_failed_release_retains_uncertain_claim);
test(memory_fulfillment_real_seller_concurrency_scenario);
test(fulfillment_restart_recovers_durable_resource_and_receipt);
test(fulfillment_restart_retains_interrupted_durable_claim);

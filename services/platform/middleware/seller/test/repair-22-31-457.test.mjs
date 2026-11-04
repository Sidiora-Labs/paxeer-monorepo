import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { verifyCommittedReceipt } from "@sidiora/layerx-sdk";
import {
  DurableFulfillmentRepository, encodePaymentCommitmentEvidence, decodePaymentCommitmentEvidence,
  verifyPaymentCommitment, verifyPaymentCommitmentEvidence,
} from "../dist/index.js";
import { committedVector, recordedCommittedReceipt } from "./committed-fixture.mjs";
import "../../agent/test/repair-22-31-457.test.mjs";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const failure = (error) => error.code === "verification-failure";
function nodeEvidence(evidence) {
  return { chainId: evidence.blockHeader.chainId, landingHeight: evidence.landingBlockHeight,
    blockId: evidence.blockHeader.blockId, header: evidence.blockHeader,
    commit: evidence.commit, validators: evidence.validatorSet, inclusion: evidence.receiptInclusion };
}
async function sellerAcceptsRecordedCommittedReceipt() {
  const { verified, evidence, resolver } = await recordedCommittedReceipt();
  const committed = await verifyCommittedReceipt(verified, nodeEvidence(evidence));
  assert.equal(committed.level, "committed");
  assert.equal(committed.landingHeight, verified.height + 1n);
  assert.equal(committed.signedPower, committed.totalPower);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", resolver), undefined);
  const requests = [];
  await verifyPaymentCommitment(verified, "eip155:125", "committed", {
    ...resolver,
    async resolve(transactionHash, network) {
      requests.push({ transactionHash, network });
      return resolver.resolve(transactionHash, network);
    },
  });
  assert.deepEqual(requests, [{ transactionHash: hex(verified.receipt.activityId), network: "eip155:125" }]);
  await assert.rejects(verifyPaymentCommitmentEvidence(structuredClone(verified), "committed", evidence, "eip155:125", resolver), failure);
}
async function sellerRefusesTamperedCommittedInclusion() {
  const { verified, evidence, resolver } = await recordedCommittedReceipt();
  for (const mutate of [
    (p) => { p.executionHeader.dataHash[0] ^= 1; },
    (p) => { p.successorHeader.lastResultsHash[0] ^= 1; },
    (p) => { p.executionCommit.signatures[0].signature[0] ^= 1; },
    (p) => { p.executionValidators[0].votingPower += 1n; },
    (p) => { p.transaction[0] ^= 1; },
    (p) => { p.transactionProof.aunts[0][0] ^= 1; },
    (p) => { p.batchTransaction[40] ^= 1; },
    (p) => { p.batchProof.aunts[0][0] ^= 1; },
    (p) => { p.result.code = 45; },
    (p) => { p.result.data[0] ^= 1; },
    (p) => { p.result.gasWanted += 1n; },
    (p) => { p.result.gasUsed += 1n; },
    (p) => { p.resultProof.aunts[0][0] ^= 1; },
  ]) {
    const invalid = structuredClone(evidence);
    mutate(invalid.receiptInclusion);
    await assert.rejects(verifyCommittedReceipt(verified, nodeEvidence(invalid)));
    await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", invalid, "eip155:125", resolver), failure);
    const portable = JSON.parse(JSON.stringify(encodePaymentCommitmentEvidence(invalid)));
    await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed",
      decodePaymentCommitmentEvidence(portable), "eip155:125", resolver), failure);
  }
}
async function committedEvidenceSurvivesDurableReopen(t) {
  const { verified, evidence, resolver } = await recordedCommittedReceipt();
  const portable = JSON.parse(JSON.stringify(encodePaymentCommitmentEvidence(evidence)));
  assert.deepEqual(decodePaymentCommitmentEvidence(portable), evidence);
  const directory = await mkdtemp(join(tmpdir(), "seller-22-31-457-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const key = committedVector.pinnedKey;
  const proposed = {
    format: "layerx-receipt-proof-v2", verificationLevel: "committed",
    rawTransaction: committedVector.rawTransaction, signedReceipt: committedVector.signedReceipt,
    receiptDigest: hex(verified.receiptDigest),
    receiptKey: { keyId: key.keyId, publicKey: key.publicKey,
      firstHeight: `0x${BigInt(key.firstHeight).toString(16)}`, lastHeight: `0x${BigInt(key.lastHeight).toString(16)}` },
    txHash: hex(verified.receipt.activityId), idempotencyKey: "45".repeat(32), requestDigest: "57".repeat(32),
    commitmentEvidence: evidence,
  };
  let releases = 0;
  const release = async () => { releases++; return { product: "recorded-committed-receipt" }; };
  await new DurableFulfillmentRepository(directory).fulfill(proposed, release);
  const reopened = new DurableFulfillmentRepository(directory);
  const recovered = await reopened.get(proposed.idempotencyKey);
  assert.deepEqual(recovered.commitmentEvidence, evidence);
  await verifyPaymentCommitmentEvidence(verified, "committed", recovered.commitmentEvidence, "eip155:125", resolver);
  assert.equal((await verifyCommittedReceipt(verified, nodeEvidence(recovered.commitmentEvidence))).level, "committed");
  assert.deepEqual(await reopened.fulfill(proposed, release), recovered);
  assert.equal(releases, 1);
}
async function committedEvidenceDecoderRequiresEveryProofField() {
  const { evidence } = await recordedCommittedReceipt();
  const portable = encodePaymentCommitmentEvidence(evidence);
  for (const field of Object.keys(portable.receiptInclusion)) {
    const missing = structuredClone(portable);
    delete missing.receiptInclusion[field];
    assert.throws(() => decodePaymentCommitmentEvidence(missing), failure);
  }
  for (const mutate of [
    (p) => { p.receiptInclusion.executionHeader.extra = "unrecognized"; },
    (p) => { p.receiptInclusion.executionCommit.signatures[0].signature = "0x00"; },
    (p) => { p.receiptInclusion.executionValidators[0].publicKey = "0x00"; },
    (p) => { p.receiptInclusion.transaction = "0x0"; },
    (p) => { p.receiptInclusion.transactionProof.aunts[0] = "0x00"; },
    (p) => { p.receiptInclusion.result.data = "0xGG"; },
    (p) => { p.receiptInclusion.result.code = -1; },
    (p) => { p.receiptInclusion.resultProof.index = "1"; },
  ]) {
    const invalid = structuredClone(portable);
    mutate(invalid);
    assert.throws(() => decodePaymentCommitmentEvidence(invalid), failure);
  }
}

test(sellerAcceptsRecordedCommittedReceipt);
test(sellerRefusesTamperedCommittedInclusion);
test(committedEvidenceSurvivesDurableReopen);
test(committedEvidenceDecoderRequiresEveryProofField);

import assert from "node:assert/strict";
import test from "node:test";
import { verifyPaymentCommitmentEvidence } from "../dist/index.js";
import { committedVector, recordedCommittedReceipt } from "./committed-fixture.mjs";
import { recordedCommittedFixture } from "../../conformance/pay6/committed-fixture.mjs";

async function recordedAbsentVotePreservesFields() {
  const { evidence } = await recordedCommittedReceipt();
  const recorded = committedVector.receiptInclusion.NextCommit.Signatures;
  const index = recorded.findIndex((vote) => vote.BlockIDFlag === 1);
  assert.notEqual(index, -1);
  assert.equal(recorded[index].Signature, null);
  assert.equal(recorded[index].Timestamp, "0001-01-01T00:00:00Z");
  assert.equal(evidence.commit.signatures.length, recorded.length);
  const vote = evidence.commit.signatures[index];
  assert.equal(vote.blockIdFlag, 1);
  assert.deepEqual(vote.signature, new Uint8Array());
  assert.deepEqual(vote.validatorAddress, new Uint8Array(Buffer.from(recorded[index].ValidatorAddress, "base64")));
  assert.deepEqual(vote.timestamp, { seconds: -62135596800n, nanos: 0 });
  for (const [i, source] of recorded.entries()) {
    if (source.Signature !== null) {
      assert.deepEqual(evidence.commit.signatures[i].signature, new Uint8Array(Buffer.from(source.Signature, "base64")));
    }
  }
}

async function nullAndMissingSignaturesUseEmptyBytesInBothCommits() {
  const { evidence: baseline } = await recordedCommittedReceipt();
  for (const [source, expected, select] of [
    [committedVector.commit.Signatures[1], baseline.receiptInclusion.executionCommit.signatures[1],
      (evidence) => evidence.receiptInclusion.executionCommit.signatures[1]],
    [committedVector.receiptInclusion.NextCommit.Signatures[1], baseline.commit.signatures[1],
      (evidence) => evidence.commit.signatures[1]],
  ]) {
    const original = source.Signature;
    try {
      for (const value of [null, undefined]) {
        if (value === undefined) delete source.Signature;
        else source.Signature = value;
        const { evidence } = await recordedCommittedReceipt();
        assert.deepEqual(select(evidence), { ...expected, signature: new Uint8Array() });
      }
    } finally {
      source.Signature = original;
    }
  }
}

async function otherRecordedBytesRemainStrict() {
  for (const [source, field, value] of [
    [committedVector.commit.Signatures[0], "ValidatorAddress", null],
    [committedVector.receiptInclusion.NextCommit.Signatures[1], "ValidatorAddress", null],
    [committedVector.receiptInclusion.NextCommit.BlockID, "Hash", null],
    [committedVector.blockHeader, "AppHash", null],
    [committedVector.receiptInclusion, "Transaction", null],
    [committedVector.receiptInclusion.Result, "Data", null],
    [committedVector.receiptInclusion.NextCommit.Signatures[0], "Signature", 17],
  ]) {
    const original = source[field];
    try {
      source[field] = value;
      await assert.rejects(recordedCommittedReceipt(), { code: "ERR_INVALID_ARG_TYPE" });
    } finally {
      source[field] = original;
    }
  }
}

async function recordedReceiptVerifiesWithRealSellerVerifier() {
  assert.equal(recordedCommittedFixture, recordedCommittedReceipt);
  const { verified, evidence, resolver } = await recordedCommittedFixture();
  assert.equal(evidence.landingBlockHeight, verified.height + 1n);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", resolver), undefined);
}

test(recordedAbsentVotePreservesFields);
test(nullAndMissingSignaturesUseEmptyBytesInBothCommits);
test(otherRecordedBytesRemainStrict);
test(recordedReceiptVerifiesWithRealSellerVerifier);

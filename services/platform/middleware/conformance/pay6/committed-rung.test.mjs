import assert from "node:assert/strict";
import test from "node:test";
import { PlatformSdkError } from "@sidiora/layerx-sdk";
import {
  verifyPaymentCommitmentEvidence as sellerVerifyPaymentCommitmentEvidence,
} from "../../seller/dist/index.js";
import { fixedBytes } from "../src/receipts.ts";
import { committedFixture, merkleRoot, validator, validatorSetHash } from "./committed-fixture.mjs";

const verifyPaymentCommitmentEvidence = sellerVerifyPaymentCommitmentEvidence;

async function committedFixtureAcceptsThreeOfFourValidators(context) {
  const { verified, evidence } = await committedFixture(context);
  assert.equal(evidence.validatorSet.length, 4);
  assert.equal(evidence.commit.signatures.length, 3);
  assert.ok(evidence.validatorSet.every((entry) => entry.votingPower === 1n));
  assert.deepEqual(validatorSetHash(evidence.validatorSet), Buffer.from(evidence.blockHeader.validatorsHash));
  assert.deepEqual(merkleRoot([evidence.receiptInclusion.receiptDigest]), evidence.blockHeader.receiptRoot);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence), undefined);
}

async function committedFixtureRefusesValidatorSetMismatch(context) {
  const { verified, evidence } = await committedFixture(context);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence), undefined);
  const mismatched = structuredClone(evidence);
  const signed = mismatched.validatorSet.findIndex((entry) => mismatched.commit.signatures.some(
    (signature) => Buffer.from(signature.validatorAddress).equals(entry.address),
  ));
  assert.notEqual(signed, -1);
  mismatched.validatorSet[signed] = validator(fixedBytes(5));
  mismatched.validatorSet.sort((left, right) => Buffer.compare(left.address, right.address));
  assert.deepEqual(mismatched.commit, evidence.commit);
  assert.deepEqual(mismatched.blockHeader, structuredClone(evidence.blockHeader));
  assert.notDeepEqual(validatorSetHash(mismatched.validatorSet), Buffer.from(evidence.blockHeader.validatorsHash));
  assert.equal(mismatched.validatorSet.length, 4);
  assert.ok(mismatched.validatorSet.every((entry) => entry.votingPower === 1n));
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", mismatched), (error) => {
    assert.ok(error instanceof PlatformSdkError);
    assert.equal(error.code, "verification-failure");
    return true;
  });
}

test(committedFixtureAcceptsThreeOfFourValidators);
test(committedFixtureRefusesValidatorSetMismatch);

import assert from "node:assert/strict";
import test from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { PlatformSdkError } from "@sidiora/layerx-sdk";
import {
  verifyPaymentCommitmentEvidence as sellerVerifyPaymentCommitmentEvidence,
} from "../../seller/dist/index.js";
import { fixedBytes } from "../src/receipts.ts";
import { committedFixture, commitSignBytes, validator, validatorSetHash } from "./committed-fixture.mjs";

const verifyPaymentCommitmentEvidence = sellerVerifyPaymentCommitmentEvidence;

async function committedFixtureAcceptsThreeOfFourValidators(context) {
  const { verified, evidence, resolver } = await committedFixture(context);
  assert.equal(evidence.validatorSet.length, 4);
  assert.equal(evidence.commit.signatures.length, 3);
  assert.ok(evidence.validatorSet.every((entry) => entry.votingPower === 1n));
  assert.deepEqual(validatorSetHash(evidence.validatorSet), Buffer.from(evidence.blockHeader.validatorsHash));
  assert.equal(evidence.receiptInclusion.activityIds.length, 1);
  assert.deepEqual(evidence.receiptInclusion.activityIds[0], verified.receipt.activityId);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", resolver), undefined);
}

async function committedFixtureRefusesValidatorSetMismatch(context) {
  const { verified, evidence, resolver } = await committedFixture(context);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence, "eip155:125", resolver), undefined);
  const mismatched = structuredClone(evidence);
  const unsigned = mismatched.validatorSet.findIndex((entry) => !mismatched.commit.signatures.some(
    (signature) => Buffer.from(signature.validatorAddress).equals(entry.address),
  ));
  assert.notEqual(unsigned, -1);
  mismatched.validatorSet[unsigned] = validator(fixedBytes(5));
  mismatched.validatorSet.sort((left, right) => Buffer.compare(left.address, right.address));
  assert.deepEqual(mismatched.commit, evidence.commit);
  assert.deepEqual(mismatched.blockHeader, structuredClone(evidence.blockHeader));
  assert.notDeepEqual(validatorSetHash(mismatched.validatorSet), Buffer.from(evidence.blockHeader.validatorsHash));
  assert.equal(mismatched.validatorSet.length, 4);
  assert.ok(mismatched.validatorSet.every((entry) => entry.votingPower === 1n));
  let signedPower = 0n;
  for (const signature of mismatched.commit.signatures) {
    const member = mismatched.validatorSet.find((entry) => Buffer.from(entry.address).equals(signature.validatorAddress));
    assert.ok(member);
    assert.equal(ed25519.verify(signature.signature, commitSignBytes(mismatched.blockHeader), member.publicKey, { zip215: false }), true);
    signedPower += member.votingPower;
  }
  assert.ok(signedPower * 3n > mismatched.validatorSet.reduce((sum, entry) => sum + entry.votingPower, 0n) * 2n);
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", mismatched, "eip155:125", resolver), (error) => {
    assert.ok(error instanceof PlatformSdkError);
    assert.equal(error.code, "verification-failure");
    return true;
  });
}

test(committedFixtureAcceptsThreeOfFourValidators);
test(committedFixtureRefusesValidatorSetMismatch);

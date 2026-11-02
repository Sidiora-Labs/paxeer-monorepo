import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { PlatformSdkError, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { verifyPaymentCommitment, verifyPaymentCommitmentEvidence } from "../dist/commitment.js";

const vectors = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const vector = vectors.cases.find((entry) => entry.id === "V1-pax-transfer");
const bytes = (hex) => Uint8Array.from(Buffer.from(hex.replace(/^0x/u, ""), "hex"));
const secrets = [1, 2, 3, 4].map((value) => new Uint8Array(32).fill(value));
const power = 1n << 54n;

async function receiptedFixture() {
  const verified = await verifySequencerReceipt(
    bytes(vector.signedReceipt), bytes(vector.rawTransaction), [{
      keyId: bytes(vector.pinnedKey.keyId),
      publicKey: bytes(vector.pinnedKey.publicKey),
      first_height: BigInt(vector.pinnedKey.firstHeight),
      last_height: BigInt(vector.pinnedKey.lastHeight),
    }],
  );
  assert.equal(verified.height, BigInt(vector.receiptJson.height));
  return verified;
}

function committedFixture(verified, height = verified.height, validatorCount = 4, signerCount = 3) {
  const validatorSet = secrets.slice(0, validatorCount).map((secret) => {
    const publicKey = ed25519.getPublicKey(secret);
    return {
      publicKey,
      address: Uint8Array.from(createHash("sha256").update(publicKey).digest().subarray(0, 20)),
      votingPower: power,
    };
  });
  const hash = new Uint8Array(32).fill(0x11);
  const partsHash = new Uint8Array(32).fill(0x22);
  const blockId = { hash, partSetHeader: { total: 1, hash: partsHash } };
  const heightBytes = Buffer.alloc(8);
  heightBytes.writeBigInt64LE(height);
  const signBytes = Buffer.concat([
    Buffer.from("65080211", "hex"), heightBytes,
    Buffer.from("22480a20", "hex"), hash,
    Buffer.from("122408011220", "hex"), partsHash,
    Buffer.from("2a020801320a7061786565722d313235", "hex"),
  ]);
  const signatures = validatorSet.slice(0, signerCount).map((validator, index) => {
    const signature = ed25519.sign(signBytes, secrets[index]);
    assert.equal(ed25519.verify(signature, signBytes, validator.publicKey, { zip215: false }), true);
    return {
      blockIdFlag: 2,
      validatorAddress: validator.address,
      timestamp: { seconds: 1n, nanos: 0 },
      signature,
    };
  });
  return {
    landingBlockHeight: height,
    blockHeader: { chainId: "paxeer-125", blockId },
    receiptInclusion: {
      transactionHash: verified.receipt.activityId.slice(),
      receiptDigest: verified.receiptDigest.slice(),
      sequence: verified.receipt.globalSequence,
    },
    commit: { height, round: 0, blockId, signatures },
    validatorSet,
  };
}

function verificationFailure(error) {
  assert(error instanceof PlatformSdkError);
  assert.equal(error.code, "verification-failure");
  return true;
}

async function committedRungAcceptsTwoThirdsCommit() {
  const verified = await receiptedFixture();
  const evidence = committedFixture(verified);
  assert.equal(evidence.landingBlockHeight, verified.height);
  assert.equal(evidence.commit.signatures.length, 3);
  assert.equal(evidence.validatorSet.length, 4);
  const signedPower = evidence.validatorSet.slice(0, 3).reduce((sum, validator) => sum + validator.votingPower, 0n);
  const totalPower = evidence.validatorSet.reduce((sum, validator) => sum + validator.votingPower, 0n);
  assert(signedPower * 3n > totalPower * 2n);
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", evidence), undefined);
  assert.equal(await verifyPaymentCommitmentEvidence(
    verified, "committed", committedFixture(verified, verified.height + 1n),
  ), undefined);
  assert.equal(await verifyPaymentCommitment(verified, "eip155:125", "receipted"), undefined);
  await assert.rejects(verifyPaymentCommitment(verified, "eip155:125", "committed"), verificationFailure);

  const tampered = structuredClone(evidence);
  tampered.commit.signatures[0].signature[0] ^= 1;
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", tampered), verificationFailure);
  const mismatchedReceipt = structuredClone(evidence);
  mismatchedReceipt.receiptInclusion.transactionHash[0] ^= 1;
  await assert.rejects(verifyPaymentCommitmentEvidence(
    verified, "committed", mismatchedReceipt,
  ), verificationFailure);
}

async function committedRungRefusesTwoThirdsExactly() {
  const verified = await receiptedFixture();
  const evidence = committedFixture(verified, verified.height, 3, 2);
  const signedPower = evidence.validatorSet.slice(0, 2).reduce((sum, validator) => sum + validator.votingPower, 0n);
  const totalPower = evidence.validatorSet.reduce((sum, validator) => sum + validator.votingPower, 0n);
  assert.equal(signedPower * 3n, totalPower * 2n);
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", evidence), verificationFailure);

  const aboveTwoThirds = structuredClone(evidence);
  aboveTwoThirds.validatorSet[2].votingPower -= 1n;
  assert.equal(await verifyPaymentCommitmentEvidence(verified, "committed", aboveTwoThirds), undefined);
}

async function committedRungRefusesEarlierLandingBlock() {
  const verified = await receiptedFixture();
  const evidence = committedFixture(verified, verified.height - 1n);
  assert.equal(evidence.landingBlockHeight, verified.height - 1n);
  assert.equal(evidence.commit.height, evidence.landingBlockHeight);
  assert.equal(evidence.commit.signatures.length, 3);
  assert.equal(evidence.validatorSet.length, 4);
  await assert.rejects(verifyPaymentCommitmentEvidence(verified, "committed", evidence), verificationFailure);
  assert.equal(await verifyPaymentCommitmentEvidence(
    verified, "committed", committedFixture(verified, verified.height),
  ), undefined);
}

test(committedRungAcceptsTwoThirdsCommit);
test(committedRungRefusesTwoThirdsExactly);
test(committedRungRefusesEarlierLandingBlock);

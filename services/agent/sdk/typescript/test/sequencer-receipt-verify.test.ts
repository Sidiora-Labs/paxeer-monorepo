import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import {
  SequencerReceiptVerificationError,
  verifyCommittedReceipt,
  verifyCometCommit,
  verifySequencerReceipt,
  type CommittedReceiptEvidence,
  type CometCommitSignature,
  type SequencerReceiptKey,
  type SequencerReceiptRefusal,
} from "../src/verifier.js";

import { committedVector, recordedCommittedEvidence, recordedCommittedKeys } from "./recorded-committed.js";

interface ReceiptVector {
  readonly id: string;
  readonly signedReceipt: string;
  readonly rawTransaction: string;
  readonly receiptDigest: string;
  readonly receiptJson: { readonly height: string };
  readonly pinnedKey: {
    readonly keyId: string;
    readonly publicKey: string;
    readonly firstHeight: number;
    readonly lastHeight: number;
  };
}

const vectors: { readonly cases: readonly ReceiptVector[] } = JSON.parse(readFileSync(
  new URL("../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url),
  "utf8",
));
const bytes = (hex: string): Uint8Array => Uint8Array.from(Buffer.from(hex.replace(/^0x/u, ""), "hex"));
const vector = vectors.cases[0]!;
const signedReceipt = bytes(vector.signedReceipt);
const rawTransaction = bytes(vector.rawTransaction);
const pin = (entry: ReceiptVector): SequencerReceiptKey => ({
  keyId: bytes(entry.pinnedKey.keyId),
  publicKey: bytes(entry.pinnedKey.publicKey),
  first_height: BigInt(entry.pinnedKey.firstHeight),
  last_height: BigInt(entry.pinnedKey.lastHeight),
});
const refuses = (check: SequencerReceiptRefusal) => (error: unknown): boolean => {
  assert(error instanceof SequencerReceiptVerificationError);
  assert.equal(error.code, "verification-failure");
  assert.equal(error.check, check);
  return true;
};

async function verify_sequencer_receipt_pinned_key(): Promise<void> {
  for (const entry of vectors.cases.filter((candidate) => /^V[12]-/u.test(candidate.id))) {
    const verified = await verifySequencerReceipt(bytes(entry.signedReceipt), bytes(entry.rawTransaction), [pin(entry)]);
    assert.equal(verified.level, "sequencer-signed");
    assert.equal(verified.height, BigInt(entry.receiptJson.height));
    assert.deepEqual(verified.canonicalBytes, bytes(entry.signedReceipt));
    assert.deepEqual(verified.receiptDigest, bytes(entry.receiptDigest));
  }
  const key = pin(vector);
  await verifySequencerReceipt(signedReceipt, rawTransaction, [{ ...key, first_height: 1000n, last_height: 1000n }]);
  await assert.rejects(verifySequencerReceipt(signedReceipt, rawTransaction, []), refuses("unpinned_key"));
  await assert.rejects(verifySequencerReceipt(signedReceipt, rawTransaction,
    [{ ...key, keyId: new Uint8Array(32).fill(42) }]), refuses("unpinned_key"));
  for (const range of [{ first_height: 1001n, last_height: 1999n }, { first_height: 0n, last_height: 999n }]) {
    await assert.rejects(verifySequencerReceipt(signedReceipt, rawTransaction, [{ ...key, ...range }]),
      refuses("key_out_of_range"));
  }
  const wrongTransaction = rawTransaction.slice();
  wrongTransaction[wrongTransaction.length - 1]! ^= 1;
  await assert.rejects(verifySequencerReceipt(signedReceipt, wrongTransaction, [key]), refuses("hash_mismatch"));
  const badSignature = signedReceipt.slice();
  badSignature[badSignature.length - 1]! ^= 1;
  await assert.rejects(verifySequencerReceipt(badSignature, rawTransaction, [key]), refuses("bad_signature"));
  await assert.rejects(verifySequencerReceipt(signedReceipt, rawTransaction,
    [{ ...key, publicKey: ed25519.getPublicKey(new Uint8Array(32).fill(42)) }]), refuses("bad_signature"));
  await assert.rejects(verifySequencerReceipt(signedReceipt.subarray(0, 20), rawTransaction, [key]),
    refuses("bad_signature"));
}

const secrets = [1, 2, 3].map((value) => new Uint8Array(32).fill(value));
const power = 1n << 54n;
const validators = secrets.map((secret) => {
  const publicKey = ed25519.getPublicKey(secret);
  return { publicKey, address: Uint8Array.from(createHash("sha256").update(publicKey).digest().subarray(0, 20)),
    votingPower: power };
});

function commitEvidence(height = 1001n): CommittedReceiptEvidence {
  const hash = new Uint8Array(32).fill(0x11);
  const partsHash = new Uint8Array(32).fill(0x22);
  const heightBytes = Buffer.alloc(8);
  heightBytes.writeBigInt64LE(height);
  // Delimited Comet CanonicalVote: PRECOMMIT, round zero, one block part,
  // timestamp {seconds: 1, nanos: 0}, chain "paxeer-125".
  const signBytes = Buffer.concat([
    Buffer.from("65080211", "hex"), heightBytes,
    Buffer.from("22480a20", "hex"), hash,
    Buffer.from("122408011220", "hex"), partsHash,
    Buffer.from("2a020801320a7061786565722d313235", "hex"),
  ]);
  const signatures: CometCommitSignature[] = validators.map((validator, index) => ({
    blockIdFlag: 2,
    validatorAddress: validator.address,
    timestamp: { seconds: 1n, nanos: 0 },
    signature: ed25519.sign(signBytes, secrets[index]!),
  }));
  const blockId = { hash, partSetHeader: { total: 1, hash: partsHash } };
  return { chainId: "paxeer-125", landingHeight: height, blockId,
    commit: { height, round: 0, blockId, signatures }, validators };
}

async function verify_committed_receipt_recorded_inclusion(): Promise<void> {
  const receipt = await verifySequencerReceipt(bytes(committedVector.signedReceipt), bytes(committedVector.rawTransaction), recordedCommittedKeys());
  const evidence = recordedCommittedEvidence();
  const inclusion = evidence.inclusion!;
  const committed = await verifyCommittedReceipt(receipt, evidence);
  assert.equal(committed.level, "committed");
  assert.equal(committed.landingHeight, 11n);
  const recordedSignedPower = evidence.validators
    .filter((validator) => evidence.commit.signatures.some((signature) => signature.blockIdFlag === 2
      && Buffer.from(signature.validatorAddress).equals(validator.address)))
    .reduce((sum, validator) => sum + validator.votingPower, 0n);
  assert.equal(committed.signedPower, recordedSignedPower);
  assert(committed.signedPower * 3n >= committed.totalPower * 2n);
  assert.deepEqual(committed.blockHash, evidence.blockId.hash);
  assert.deepEqual(committed.receipt.canonicalBytes, bytes(committedVector.signedReceipt));
  for (const changed of [
    { ...inclusion, transaction: inclusion.batchTransaction },
    { ...inclusion, transactionProof: { ...inclusion.transactionProof, index: 0n } },
    { ...inclusion, batchProof: inclusion.transactionProof },
    { ...inclusion, result: { ...inclusion.result, code: 45 } },
    { ...inclusion, result: { ...inclusion.result, gasUsed: inclusion.result.gasUsed + 1n } },
    { ...inclusion, successorHeader: { ...inclusion.successorHeader, lastResultsHash: new Uint8Array(32) } },
    { ...inclusion, batchTransaction: inclusion.batchTransaction.map((byte, i) => i === 0 ? byte ^ 1 : byte) },
  ]) await assert.rejects(verifyCommittedReceipt(receipt, { ...evidence, inclusion: changed }), refuses("inclusion_mismatch"));
  // This quorum has valid signatures, but it commits an unrelated block with no inclusion evidence.
  const unrelated = commitEvidence(evidence.landingHeight);
  assert.equal((await verifyCometCommit(unrelated)).signedPower, power * 3n);
  await assert.rejects(verifyCommittedReceipt(receipt, unrelated), refuses("inclusion_mismatch"));
  await assert.rejects(verifyCommittedReceipt(receipt, { ...evidence, blockId: unrelated.blockId, commit: unrelated.commit,
    validators: unrelated.validators }), refuses("inclusion_mismatch"));
  await assert.rejects(verifyCommittedReceipt(receipt, { ...evidence, landingHeight: receipt.height - 1n }), refuses("height_regression"));
  await assert.rejects(verifyCommittedReceipt(receipt, { ...evidence, landingHeight: evidence.landingHeight + 1n }), refuses("height_regression"));
  const corrupted = evidence.commit.signatures.map((signature, index) => index === 0
    ? { ...signature, signature: new Uint8Array(64) } : signature);
  await assert.rejects(verifyCommittedReceipt(receipt, { ...evidence, commit: { ...evidence.commit, signatures: corrupted } }), refuses("bad_signature"));
}

async function verify_committed_receipt_two_thirds(): Promise<void> {
  // Isolate the cryptographic power boundary from recorded header/validator hash binding.
  const evidence = commitEvidence();
  const committed = await verifyCometCommit(evidence);
  assert.equal(committed.signedPower, power * 3n);
  assert.equal(committed.totalPower, power * 3n);
  assert.equal((await verifyCometCommit(commitEvidence(1000n))).signedPower, power * 3n);
  const twoVotes = evidence.commit.signatures.slice(0, 2);
  const twoThirds = { ...evidence, commit: { ...evidence.commit, signatures: twoVotes } };
  await assert.rejects(verifyCometCommit(twoThirds), refuses("insufficient_power"));
  const justAbove = { ...twoThirds,
    validators: validators.map((validator, index) => ({ ...validator, votingPower: power - (index === 2 ? 1n : 0n) })) };
  assert.equal((await verifyCometCommit(justAbove)).signedPower, power * 2n);
  for (const blockIdFlag of [1, 3] as const) {
    const signatures = evidence.commit.signatures.map((signature, index) => index === 2
      ? { ...signature, blockIdFlag } : signature);
    await assert.rejects(verifyCometCommit(
      { ...evidence, commit: { ...evidence.commit, signatures } }), refuses("insufficient_power"));
  }
  await assert.rejects(verifyCometCommit(
    { ...evidence, commit: { ...evidence.commit, signatures: [...twoVotes, twoVotes[0]!] } }), refuses("bad_signature"));
  const corrupted = evidence.commit.signatures.map((signature, index) => index === 0
    ? { ...signature, signature: new Uint8Array(64) } : signature);
  await assert.rejects(verifyCometCommit(
    { ...evidence, commit: { ...evidence.commit, signatures: corrupted } }), refuses("bad_signature"));
  await assert.rejects(verifyCometCommit({ ...evidence, chainId: "another-chain" }), refuses("bad_signature"));
  await assert.rejects(verifyCometCommit(
    { ...evidence, commit: { ...evidence.commit, round: 1 } }), refuses("bad_signature"));
  await assert.rejects(verifyCometCommit(
    { ...evidence, blockId: { ...evidence.blockId, hash: new Uint8Array(32).fill(33) } }), refuses("bad_signature"));
  await assert.rejects(verifyCometCommit(
    { ...evidence, validators: [...validators, validators[0]!] }), refuses("bad_signature"));
  await assert.rejects(verifyCometCommit(
    { ...evidence, validators: [] }), refuses("bad_signature"));
}

test(verify_sequencer_receipt_pinned_key);
test(verify_committed_receipt_recorded_inclusion);
test(verify_committed_receipt_two_thirds);

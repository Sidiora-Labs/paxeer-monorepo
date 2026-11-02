import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { PlatformSdkError, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  verifyPaymentCommitmentEvidence as sellerVerifyPaymentCommitmentEvidence,
  verifyPaymentReceipt,
} from "../../seller/dist/index.js";
import {
  ConformanceSequencer,
  buildSignedReceipt as conformanceBuildSignedReceipt,
  fixedBytes,
  offerFixture,
} from "../src/receipts.ts";

const verifyPaymentCommitmentEvidence = sellerVerifyPaymentCommitmentEvidence;
const buildSignedReceipt = conformanceBuildSignedReceipt;
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const sha256 = (...values) => createHash("sha256").update(Buffer.concat(values)).digest();
const chainId = "paxeer-125";

function varint(value) {
  const result = [];
  do {
    const low = Number(value & 127n);
    value >>= 7n;
    result.push(low | (value === 0n ? 0 : 128));
  } while (value !== 0n);
  return Buffer.from(result);
}

function protoBytes(field, value) {
  return Buffer.concat([varint(BigInt(field * 8 + 2)), varint(BigInt(value.length)), value]);
}

function protoInteger(field, value) {
  return value === 0n ? Buffer.alloc(0) : Buffer.concat([varint(BigInt(field * 8)), varint(value)]);
}

function merkleRoot(leaves) {
  if (leaves.length === 0) return sha256(Buffer.alloc(0));
  if (leaves.length === 1) return sha256(Buffer.of(0), leaves[0]);
  let split = 1;
  while (split * 2 < leaves.length) split *= 2;
  return sha256(Buffer.of(1), merkleRoot(leaves.slice(0, split)), merkleRoot(leaves.slice(split)));
}

function validatorSetHash(validators) {
  return merkleRoot(validators.map((validator) => Buffer.concat([
    protoBytes(1, protoBytes(1, validator.publicKey)),
    protoInteger(2, validator.votingPower),
  ])));
}

function validator(secret) {
  const publicKey = ed25519.getPublicKey(secret);
  return { publicKey, address: Uint8Array.from(sha256(publicKey).subarray(0, 20)), votingPower: 1n };
}

function blockHeader(verified, rawTransaction, validators) {
  const height = verified.height;
  const validatorsHash = validatorSetHash(validators);
  const receiptRoot = merkleRoot([verified.receiptDigest]);
  const dataHash = merkleRoot([rawTransaction]);
  const timestamp = protoInteger(1, 1n);
  const empty = Buffer.alloc(0);
  const fields = [
    Buffer.concat([protoInteger(1, 11n), protoInteger(2, 1n)]),
    protoBytes(1, Buffer.from(chainId)), protoInteger(1, height), timestamp,
    protoBytes(2, empty), empty, protoBytes(1, dataHash),
    protoBytes(1, validatorsHash), protoBytes(1, validatorsHash),
    protoBytes(1, sha256(Buffer.from("conformance-consensus-parameters"))),
    protoBytes(1, verified.receipt.resultingStateRoot), protoBytes(1, receiptRoot),
    protoBytes(1, merkleRoot([])), protoBytes(1, validators[0].address),
  ];
  const blockId = {
    hash: Uint8Array.from(merkleRoot(fields)),
    partSetHeader: { total: 1, hash: Uint8Array.from(merkleRoot([Buffer.concat(fields)])) },
  };
  return { chainId, height, validatorsHash: Uint8Array.from(validatorsHash), receiptRoot, dataHash, blockId };
}

function commitSignBytes(header) {
  const height = Buffer.alloc(9);
  height[0] = 17;
  height.writeBigInt64LE(header.height, 1);
  const partSet = Buffer.concat([
    protoInteger(1, BigInt(header.blockId.partSetHeader.total)),
    protoBytes(2, header.blockId.partSetHeader.hash),
  ]);
  const block = Buffer.concat([protoBytes(1, header.blockId.hash), protoBytes(2, partSet)]);
  const vote = Buffer.concat([
    protoInteger(1, 2n), height, protoBytes(4, block),
    protoBytes(5, protoInteger(1, 1n)), protoBytes(6, Buffer.from(header.chainId)),
  ]);
  return Buffer.concat([varint(BigInt(vote.length)), vote]);
}

async function committedFixture(context) {
  const sequencer = await ConformanceSequencer.generate();
  const facts = {
    asset: fixedBytes(0x11), payTo: fixedBytes(0x22), amount: 25n,
    from: fixedBytes(0x33), batchId: fixedBytes(0x44),
    previousStateRoot: fixedBytes(0x55), resultingStateRoot: fixedBytes(0x66),
  };
  const produced = await buildSignedReceipt(sequencer, facts);
  const directory = await mkdtemp(join(tmpdir(), "pay6-committed-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const path = join(directory, "receipt.json");
  await writeFile(path, JSON.stringify(produced));
  const receipt = JSON.parse(await readFile(path, "utf8"));
  assert.deepEqual(receipt, produced);
  const key = sequencer.receiptKey();
  const pins = [{
    keyId: key.keyId, publicKey: key.publicKey,
    first_height: key.firstHeight, last_height: key.lastHeight,
  }];
  const payment = await verifyPaymentReceipt(
    receipt, offerFixture(facts.payTo, facts.asset, facts.amount).requirements, pins,
  );
  assert.equal(payment.rung, "receipted");
  assert.equal(payment.payment.amount, facts.amount);
  const verified = await verifySequencerReceipt(bytes(receipt.signedReceipt), bytes(receipt.rawTransaction), pins);
  const keys = [1, 2, 3, 4].map((seed) => {
    const secret = fixedBytes(seed);
    return { secret, validator: validator(secret) };
  }).sort((left, right) => Buffer.compare(left.validator.address, right.validator.address));
  const validatorSet = keys.map((key) => key.validator);
  const header = blockHeader(verified, bytes(receipt.rawTransaction), validatorSet);
  const signBytes = commitSignBytes(header);
  const signatures = keys.slice(0, 3).map(({ secret, validator }) => {
    const signature = ed25519.sign(signBytes, secret);
    assert.equal(ed25519.verify(signature, signBytes, validator.publicKey, { zip215: false }), true);
    return { blockIdFlag: 2, validatorAddress: validator.address, timestamp: { seconds: 1n, nanos: 0 }, signature };
  });
  const evidence = {
    landingBlockHeight: header.height,
    blockHeader: header,
    receiptInclusion: {
      transactionHash: verified.receipt.activityId.slice(),
      receiptDigest: verified.receiptDigest.slice(),
      sequence: verified.receipt.globalSequence,
      leafIndex: 0, leafCount: 1, siblings: [],
    },
    commit: { height: header.height, round: 0, blockId: header.blockId, signatures },
    validatorSet,
  };
  return { verified, evidence };
}

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

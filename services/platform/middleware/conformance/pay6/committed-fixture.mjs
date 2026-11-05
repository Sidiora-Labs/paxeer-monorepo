import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { ed25519 } from "@noble/curves/ed25519.js";
import { cometHeaderHash, cometCommitHash } from "@sidiora/layerx-sdk";
import { fixedBytes } from "../src/receipts.ts";
import { recordedCommittedReceipt as recordedCommittedFixture } from "../../seller/test/committed-fixture.mjs";

export { recordedCommittedFixture };

const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const sha256 = (...values) => createHash("sha256").update(Buffer.concat(values)).digest();
const chainId = "paxeer-125";

export function varint(value) {
  const result = [];
  do {
    const low = Number(value & 127n);
    value >>= 7n;
    result.push(low | (value === 0n ? 0 : 128));
  } while (value !== 0n);
  return Buffer.from(result);
}

export function protoBytes(field, value) {
  return Buffer.concat([varint(BigInt(field * 8 + 2)), varint(BigInt(value.length)), value]);
}

export function protoInteger(field, value) {
  return value === 0n ? Buffer.alloc(0) : Buffer.concat([varint(BigInt(field * 8)), varint(value)]);
}

export function merkleRoot(leaves) {
  if (leaves.length === 0) return sha256(Buffer.alloc(0));
  if (leaves.length === 1) return sha256(Buffer.of(0), leaves[0]);
  let split = 1;
  while (split * 2 < leaves.length) split *= 2;
  return sha256(Buffer.of(1), merkleRoot(leaves.slice(0, split)), merkleRoot(leaves.slice(split)));
}

export function validatorSetHash(validators) {
  return merkleRoot(validators.map((validator) => Buffer.concat([
    protoBytes(1, protoBytes(1, validator.publicKey)),
    protoInteger(2, validator.votingPower),
  ])));
}

export function validator(secret) {
  const publicKey = ed25519.getPublicKey(secret);
  return { publicKey, address: Uint8Array.from(sha256(publicKey).subarray(0, 20)), votingPower: 1n };
}

export function blockHeader(verified, rawTransaction, validators) {
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

export function commitSignBytes(header) {
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

export async function committedFixture() {
  const { verified, evidence } = await recordedCommittedFixture();
  const keys = [1, 2, 3, 4].map((seed) => ({ secret: fixedBytes(seed), validator: validator(fixedBytes(seed)) }))
    .sort((left, right) => Buffer.compare(left.validator.address, right.validator.address));
  evidence.validatorSet = keys.map((key) => key.validator);
  const inclusion = evidence.receiptInclusion;
  inclusion.executionValidators = structuredClone(evidence.validatorSet);
  const execution = inclusion.executionHeader;
  execution.validatorsHash = Uint8Array.from(validatorSetHash(evidence.validatorSet));
  execution.nextValidatorsHash = execution.validatorsHash.slice();
  execution.proposerAddress = evidence.validatorSet[0].address.slice();
  const executionBlockId = { ...inclusion.executionCommit.blockId, hash: await cometHeaderHash(execution) };
  const signCommit = (header) => ({ height: header.height, round: 0, blockId: header.blockId,
    signatures: keys.slice(0, 3).map(({ secret, validator }) => ({ blockIdFlag: 2, validatorAddress: validator.address,
      timestamp: { seconds: 1n, nanos: 0 }, signature: ed25519.sign(commitSignBytes(header), secret) })) });
  inclusion.executionCommit = signCommit({ ...execution, blockId: executionBlockId });
  const header = evidence.blockHeader;
  header.lastBlockId = structuredClone(executionBlockId);
  header.lastCommitHash = cometCommitHash(inclusion.executionCommit);
  header.validatorsHash = Uint8Array.from(validatorSetHash(evidence.validatorSet));
  header.nextValidatorsHash = header.validatorsHash.slice();
  header.proposerAddress = evidence.validatorSet[0].address.slice();
  header.blockId = { ...header.blockId, hash: await cometHeaderHash(header) };
  evidence.commit = signCommit(header);
  const { blockId, ...successorHeader } = header;
  inclusion.successorHeader = structuredClone(successorHeader);
  const anchor = { chainId: header.chainId, validatorsHash: header.validatorsHash.slice() };
  const resolver = { async resolve() { return structuredClone(evidence); },
    async resolveValidators(network, height) {
      assert.equal(network, "eip155:125"); assert.equal(height, evidence.landingBlockHeight);
      return structuredClone(anchor);
    } };
  return { verified, evidence, resolver };
}

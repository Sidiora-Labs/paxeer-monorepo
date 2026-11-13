import "./source-loader.mjs";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { verifyCometCommit, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  decodePaymentExactLog, encodePaymentCommitmentEvidence, decodePaymentCommitmentEvidence,
  verifyPaymentCommitmentEvidence,
} from "../src/index.ts";

export const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
export const bytes = (value) => new Uint8Array(Buffer.from(value.replace(/^0x/u, ""), "hex"));
export const failure = (error) => error.code === "verification-failure";
export const committedVector = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/vector-v4-committed.json", import.meta.url,
), "utf8")).positive;
const base64 = (value) => new Uint8Array(Buffer.from(value, "base64"));
function timestamp(value) {
  const match = /^(.*?)(?:\.([0-9]{1,9}))?Z$/u.exec(value);
  return { seconds: BigInt(Date.parse(`${match[1]}Z`)) / 1000n, nanos: Number((match[2] ?? "").padEnd(9, "0")) };
}
function blockId(value) {
  return { hash: base64(value.Hash), partSetHeader: { total: value.PartSetTotal, hash: base64(value.PartSetHash) } };
}
function header(value) {
  return { versionBlock: BigInt(value.VersionBlock), versionApp: BigInt(value.VersionApp), chainId: value.ChainID,
    height: BigInt(value.Height), time: timestamp(value.Time), lastBlockId: blockId(value.LastBlockID),
    ...Object.fromEntries(["LastCommitHash", "DataHash", "ValidatorsHash", "NextValidatorsHash", "ConsensusHash", "AppHash", "LastResultsHash", "EvidenceHash", "ProposerAddress"]
      .map((field) => [field[0].toLowerCase() + field.slice(1), base64(value[field])])) };
}
function commit(value) {
  return { height: BigInt(value.Height), round: value.Round, blockId: blockId(value.BlockID),
    signatures: value.Signatures.map((signature) => ({ blockIdFlag: signature.BlockIDFlag,
      validatorAddress: base64(signature.ValidatorAddress), timestamp: timestamp(signature.Timestamp), signature: signature.Signature === null ? new Uint8Array() : base64(signature.Signature) })) };
}
function validators(value) {
  return value.map((validator) => {
    const publicKey = new Uint8Array(validator.PublicKey);
    return { publicKey, address: new Uint8Array(createHash("sha256").update(publicKey).digest().subarray(0, 20)),
      votingPower: BigInt(validator.VotingPower) };
  });
}
function proof(value) {
  return { total: BigInt(value.Total), index: BigInt(value.Index), aunts: value.Aunts.map((aunt) => new Uint8Array(aunt)) };
}
export async function recordedCommittedReceipt() {
  const vector = committedVector, inclusion = vector.receiptInclusion, key = vector.pinnedKey;
  const keys = [{ keyId: bytes(key.keyId), publicKey: bytes(key.publicKey),
    first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight) }];
  const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), keys);
  const next = commit(inclusion.NextCommit);
  const evidence = {
    landingBlockHeight: next.height, blockHeader: { ...header(inclusion.NextHeader), blockId: next.blockId },
    receiptInclusion: {
      transactionHash: verified.receipt.activityId, receiptDigest: verified.receiptDigest, sequence: verified.receipt.globalSequence,
      executionHeader: header(vector.blockHeader), successorHeader: header(inclusion.NextHeader),
      executionCommit: commit(vector.commit), executionValidators: validators(vector.validatorSet),
      transaction: base64(inclusion.Transaction), transactionProof: proof(inclusion.TransactionProof),
      batchTransaction: base64(inclusion.BatchTransaction), batchProof: proof(inclusion.BatchProof),
      result: { code: inclusion.Result.Code, data: base64(inclusion.Result.Data),
        gasWanted: BigInt(inclusion.Result.GasWanted), gasUsed: BigInt(inclusion.Result.GasUsed) },
      resultProof: proof(inclusion.ResultProof), activityIds: [verified.receipt.activityId],
    },
    commit: next, validatorSet: validators(inclusion.NextValidators),
  };
  const anchors = new Map([[`eip155:125/${evidence.landingBlockHeight}`, {
    chainId: evidence.blockHeader.chainId, validatorsHash: evidence.blockHeader.validatorsHash.slice(),
  }]]);
  const resolver = {
    async resolve() { return structuredClone(evidence); },
    async resolveValidators(network, height) {
      const anchor = anchors.get(`${network}/${height}`);
      if (!anchor) throw new Error("untrusted-chain-height");
      return structuredClone(anchor);
    },
  };
  return { verified, evidence, resolver };
}

export function nodeEvidence(evidence) {
  return { chainId: evidence.blockHeader.chainId, landingHeight: evidence.landingBlockHeight,
    blockId: evidence.commit.blockId, commit: evidence.commit, validators: evidence.validatorSet,
    header: evidence.blockHeader, inclusion: evidence.receiptInclusion };
}

export async function committedPaymentFixture() {
  const { verified, evidence, resolver: recordedResolver } = await recordedCommittedReceipt();
  const resolver = {
    async resolve(...args) {
      const recorded = await recordedResolver.resolve(...args);
      const wire = JSON.parse(JSON.stringify(encodePaymentCommitmentEvidence(recorded)));
      return decodePaymentCommitmentEvidence(wire);
    },
    resolveValidators: (...args) => recordedResolver.resolveValidators(...args),
  };
  const vector = committedVector;
  const log = vector.receiptJson.logs.find((entry) => entry.topics[0] ===
    "0xc4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1");
  assert.ok(log, "the recorded node execution must contain PaymentExact");
  const payment = decodePaymentExactLog({ address: log.address, data: bytes(log.data), topics: log.topics.map(bytes) });
  const key = vector.pinnedKey;
  const keys = [{ keyId: bytes(key.keyId), publicKey: bytes(key.publicKey),
    first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight) }];
  const proof = { format: "layerx-receipt-proof-v2", verificationLevel: "committed",
    rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt, receiptDigest: hex(verified.receiptDigest),
    receiptKey: { keyId: key.keyId, publicKey: key.publicKey,
      firstHeight: `0x${keys[0].first_height.toString(16)}`, lastHeight: `0x${keys[0].last_height.toString(16)}` } };
  const requirements = { scheme: "exact", network: "eip155:125", amount: payment.amount.toString(),
    asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "committed", payer: hex(payment.from).slice(2), purposeHash: hex(payment.paymentReference).slice(2) } } };
  return { verified, evidence, resolver, proof, requirements, keys, payment, vector };
}

export async function refusesUnboundCommitments(verify = verifyPaymentCommitmentEvidence) {
  const { verified, evidence, resolver } = await committedPaymentFixture();
  const check = (value, anchors = resolver) => verify(verified, "committed", value, "eip155:125", anchors);
  await check(evidence);
  const portable = JSON.parse(JSON.stringify(encodePaymentCommitmentEvidence(evidence)));
  assert.deepEqual(decodePaymentCommitmentEvidence(portable), evidence);
  await check(decodePaymentCommitmentEvidence(portable));
  for (const mutate of [
    (p) => { p.blockHeader.dataHash[0] ^= 1; },
    (p) => { p.receiptInclusion.executionHeader.dataHash[0] ^= 1; },
    (p) => { p.receiptInclusion.successorHeader.lastResultsHash[0] ^= 1; },
    (p) => { p.receiptInclusion.transactionProof.aunts[0][0] ^= 1; },
    (p) => { p.receiptInclusion.resultProof.aunts[0][0] ^= 1; },
    (p) => { p.receiptInclusion.batchProof.aunts[0][0] ^= 1; },
    (p) => { p.receiptInclusion.transactionHash[0] ^= 1; },
    (p) => { p.receiptInclusion.receiptDigest[0] ^= 1; },
    (p) => { p.validatorSet[0].votingPower += 1n; },
    (p) => { p.validatorSet[0].publicKey[0] ^= 1; },
    (p) => { p.commit.signatures[0].signature[0] ^= 1; },
  ]) {
    const invalid = structuredClone(evidence); mutate(invalid);
    await assert.rejects(check(invalid), failure);
    await assert.rejects(check(decodePaymentCommitmentEvidence(JSON.parse(JSON.stringify(
      encodePaymentCommitmentEvidence(invalid))))), failure);
  }
  for (const field of Object.keys(portable.receiptInclusion)) {
    const missing = structuredClone(portable); delete missing.receiptInclusion[field];
    assert.throws(() => decodePaymentCommitmentEvidence(missing), failure);
    const lost = structuredClone(evidence); delete lost.receiptInclusion[field];
    await assert.rejects(check(lost), failure);
  }
  const unrelated = structuredClone(evidence);
  unrelated.blockHeader = { ...unrelated.receiptInclusion.executionHeader,
    blockId: unrelated.receiptInclusion.executionCommit.blockId };
  unrelated.commit = unrelated.receiptInclusion.executionCommit;
  unrelated.validatorSet = unrelated.receiptInclusion.executionValidators;
  unrelated.landingBlockHeight = unrelated.commit.height;
  const quorum = await verifyCometCommit(nodeEvidence(unrelated));
  assert(quorum.signedPower * 3n > quorum.totalPower * 2n);
  await assert.rejects(check(unrelated, { async resolveValidators(network, height) {
    assert.equal(network, "eip155:125"); assert.equal(height, unrelated.landingBlockHeight);
    return { chainId: unrelated.blockHeader.chainId, validatorsHash: unrelated.blockHeader.validatorsHash };
  } }), failure);
  await assert.rejects(check(evidence, { async resolveValidators() {
    return { chainId: "foreign-chain", validatorsHash: evidence.blockHeader.validatorsHash };
  } }), failure);
}

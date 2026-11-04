import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";

export const committedVector = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/vector-v4-committed.json", import.meta.url,
), "utf8")).positive;
const bytes = (value) => new Uint8Array(Buffer.from(value.replace(/^0x/u, ""), "hex"));
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
      validatorAddress: base64(signature.ValidatorAddress), timestamp: timestamp(signature.Timestamp), signature: base64(signature.Signature) })) };
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

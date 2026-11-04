import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import type { CometHeader, CometProof } from "../src/committed-proof.js";
import type { CometBlockId, CometValidator, CommittedReceiptEvidence, SequencerReceiptKey } from "../src/verifier.js";
const fixture = <T>(path: string): T => JSON.parse(readFileSync(new URL(`../../../../../../${path}`, import.meta.url), "utf8")) as T;
const unhex = (s: string): Uint8Array => new Uint8Array(Buffer.from(s.replace(/^0x/u, ""), "hex"));
const base64 = (s: string): Uint8Array => new Uint8Array(Buffer.from(s, "base64"));
interface RecordedBlockId { readonly Hash: string; readonly PartSetTotal: number; readonly PartSetHash: string }
interface RecordedHeader {
  readonly VersionBlock: number; readonly VersionApp: number; readonly ChainID: string; readonly Height: number; readonly Time: string;
  readonly LastBlockID: RecordedBlockId; readonly LastCommitHash: string; readonly DataHash: string;
  readonly ValidatorsHash: string; readonly NextValidatorsHash: string; readonly ConsensusHash: string;
  readonly AppHash: string; readonly LastResultsHash: string; readonly EvidenceHash: string; readonly ProposerAddress: string;
}
interface RecordedCommit {
  readonly Height: number; readonly Round: number; readonly BlockID: RecordedBlockId;
  readonly Signatures: readonly { readonly BlockIDFlag: 1 | 2 | 3; readonly ValidatorAddress: string;
    readonly Timestamp: string; readonly Signature: string }[];
}
interface RecordedValidator { readonly PublicKey: readonly number[]; readonly VotingPower: number }
interface RecordedProof { readonly Total: number; readonly Index: number; readonly Aunts: readonly (readonly number[])[] }
interface CommittedVector {
  readonly positive: {
    readonly receiptJson: { readonly logs: readonly { readonly address: string; readonly topics: readonly string[]; readonly data: string }[] };
    readonly signedReceipt: string; readonly rawTransaction: string;
    readonly pinnedKey: { readonly keyId: string; readonly publicKey: string; readonly firstHeight: number; readonly lastHeight: number };
    readonly blockHeader: RecordedHeader; readonly commit: RecordedCommit; readonly validatorSet: readonly RecordedValidator[];
    readonly receiptInclusion: { readonly Transaction: string; readonly TransactionProof: RecordedProof; readonly BatchTransaction: string;
      readonly BatchProof: RecordedProof; readonly Result: { readonly Code: number; readonly Data: string; readonly GasWanted: number; readonly GasUsed: number };
      readonly ResultProof: RecordedProof; readonly NextHeader: RecordedHeader; readonly NextCommit: RecordedCommit; readonly NextValidators: readonly RecordedValidator[] };
  };
}
export const committedVector = fixture<CommittedVector>("services/agent/sdk/typescript/test/fixtures/repair-22-33-24/committed.json").positive;
function time(s: string): CometHeader["time"] {
  const match = /^(.*?)(?:\.(\d{1,9}))?Z$/u.exec(s); assert(match);
  return { seconds: BigInt(Date.parse(`${match[1]}Z`)) / 1000n, nanos: Number((match[2] ?? "").padEnd(9, "0")) };
}
function blockId(b: RecordedBlockId): CometBlockId { return { hash: base64(b.Hash), partSetHeader: { total: b.PartSetTotal, hash: base64(b.PartSetHash) } }; }
function header(h: RecordedHeader): CometHeader {
  return { versionBlock: BigInt(h.VersionBlock), versionApp: BigInt(h.VersionApp), chainId: h.ChainID, height: BigInt(h.Height), time: time(h.Time),
    lastBlockId: blockId(h.LastBlockID), lastCommitHash: base64(h.LastCommitHash), dataHash: base64(h.DataHash), validatorsHash: base64(h.ValidatorsHash),
    nextValidatorsHash: base64(h.NextValidatorsHash), consensusHash: base64(h.ConsensusHash), appHash: base64(h.AppHash), lastResultsHash: base64(h.LastResultsHash),
    evidenceHash: base64(h.EvidenceHash), proposerAddress: base64(h.ProposerAddress) };
}
function commit(c: RecordedCommit): CommittedReceiptEvidence["commit"] {
  return { height: BigInt(c.Height), round: c.Round, blockId: blockId(c.BlockID), signatures: c.Signatures.map(s => ({
    blockIdFlag: s.BlockIDFlag, validatorAddress: base64(s.ValidatorAddress), timestamp: time(s.Timestamp), signature: base64(s.Signature),
  })) };
}
function validators(vs: readonly RecordedValidator[]): readonly CometValidator[] {
  return vs.map(v => { const publicKey = Uint8Array.from(v.PublicKey); return {
    publicKey, address: new Uint8Array(createHash("sha256").update(publicKey).digest().subarray(0, 20)), votingPower: BigInt(v.VotingPower),
  }; });
}
const proof = (p: RecordedProof): CometProof => ({ total: BigInt(p.Total), index: BigInt(p.Index), aunts: p.Aunts.map(a => Uint8Array.from(a)) });
export function recordedCommittedEvidence(): CommittedReceiptEvidence {
  const v = committedVector, p = v.receiptInclusion, next = commit(p.NextCommit);
  return { chainId: v.blockHeader.ChainID, landingHeight: next.height, blockId: next.blockId, commit: next, validators: validators(p.NextValidators),
    inclusion: { executionHeader: header(v.blockHeader), successorHeader: header(p.NextHeader), executionCommit: commit(v.commit), executionValidators: validators(v.validatorSet),
      transaction: base64(p.Transaction), transactionProof: proof(p.TransactionProof), batchTransaction: base64(p.BatchTransaction), batchProof: proof(p.BatchProof),
      result: { code: p.Result.Code, data: base64(p.Result.Data), gasWanted: BigInt(p.Result.GasWanted), gasUsed: BigInt(p.Result.GasUsed) }, resultProof: proof(p.ResultProof) } };
}
export function recordedCommittedKeys(): readonly SequencerReceiptKey[] {
  const k = committedVector.pinnedKey;
  return [{ keyId: unhex(k.keyId), publicKey: unhex(k.publicKey), first_height: BigInt(k.firstHeight), last_height: BigInt(k.lastHeight) }];
}

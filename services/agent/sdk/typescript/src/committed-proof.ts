import { sha256 } from "@noble/hashes/sha2.js";
import { ed25519 } from "@noble/curves/ed25519.js";
import { encodeSequencerLogEffect } from "./sequencer-log.js";
import { SequencerReceiptVerificationError, verifyCometCommit, type CometBlockId,
  type CometValidator, type CometHeader, type CommittedReceiptEvidence, type SequencerReceiptKey, type SequencerReceiptVerification } from "./verifier.js";

export type { CometHeader } from "./verifier.js";
export interface CometProof {
  readonly total: bigint;
  readonly index: bigint;
  readonly aunts: readonly Uint8Array[];
}
export interface CommittedReceiptInclusion {
  readonly executionHeader: CometHeader;
  readonly successorHeader: CometHeader;
  readonly executionCommit: CommittedReceiptEvidence["commit"];
  readonly executionValidators: readonly CometValidator[];
  readonly transaction: Uint8Array;
  readonly transactionProof: CometProof;
  readonly batchTransaction: Uint8Array;
  readonly batchProof: CometProof;
  readonly result: { readonly code: number; readonly data: Uint8Array; readonly gasWanted: bigint; readonly gasUsed: bigint };
  readonly resultProof: CometProof;
}

function join(...parts: readonly Uint8Array[]): Uint8Array {
  const result = new Uint8Array(parts.reduce((n, part) => n + part.length, 0));
  let offset = 0;
  for (const part of parts) { result.set(part, offset); offset += part.length; }
  return result;
}
const hash = (...parts: readonly Uint8Array[]): Uint8Array => sha256(join(...parts));
const utf8 = (s: string): Uint8Array => new TextEncoder().encode(s);
function fromHex(s: string): Uint8Array { return Uint8Array.from(s.match(/../gu) ?? [], b => Number.parseInt(b, 16)); }
function requireProof(value: boolean): asserts value {
  if (!value) throw new SequencerReceiptVerificationError("inclusion_mismatch");
}
function same(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let diff = 0; for (let i = 0; i < a.length; i++) diff |= a[i]! ^ b[i]!;
  return diff === 0;
}
function varint(value: bigint): Uint8Array {
  requireProof(value >= -(1n << 63n) && value < 1n << 64n);
  let n = BigInt.asUintN(64, value);
  const output: number[] = [];
  while (n >= 128n) { output.push(Number(n & 127n) | 128); n >>= 7n; }
  output.push(Number(n));
  return Uint8Array.from(output);
}
const num = (tag: number, n: bigint): Uint8Array => n === 0n ? new Uint8Array() : join(varint(BigInt(tag * 8)), varint(n));
const message = (tag: number, b: Uint8Array): Uint8Array => join(varint(BigInt(tag * 8 + 2)), varint(BigInt(b.length)), b);
const bytes = (tag: number, b: Uint8Array): Uint8Array => b.length === 0 ? new Uint8Array() : message(tag, b);
function timestamp(t: CometHeader["time"]): Uint8Array {
  requireProof(t.seconds >= -62135596800n && t.seconds <= 253402300799n && Number.isInteger(t.nanos) && t.nanos >= 0 && t.nanos < 1e9);
  return join(num(1, t.seconds), num(2, BigInt(t.nanos)));
}
function blockId(b: CometBlockId): Uint8Array {
  requireProof((b.hash.length === 0 || b.hash.length === 32) && (b.partSetHeader.hash.length === 0 || b.partSetHeader.hash.length === 32)
    && Number.isInteger(b.partSetHeader.total) && b.partSetHeader.total >= 0 && b.partSetHeader.total <= 0xffffffff);
  return join(bytes(1, b.hash), message(2, join(num(1, BigInt(b.partSetHeader.total)), bytes(2, b.partSetHeader.hash))));
}
function split(n: bigint): bigint { let k = 1n; while (k * 2n < n) k *= 2n; return k; }
function root(leaves: readonly Uint8Array[]): Uint8Array {
  if (leaves.length === 0) return hash();
  if (leaves.length === 1) return hash(Uint8Array.of(0), leaves[0]!);
  const k = Number(split(BigInt(leaves.length)));
  return hash(Uint8Array.of(1), root(leaves.slice(0, k)), root(leaves.slice(k)));
}
export function cometHeaderHash(h: CometHeader): Uint8Array {
  requireProof(h.height > 0n && h.height < 1n << 63n && h.chainId.length > 0 && utf8(h.chainId).length <= 50
    && h.proposerAddress.length === 20 && h.validatorsHash.length === 32 && h.nextValidatorsHash.length === 32);
  for (const b of [h.lastCommitHash, h.dataHash, h.consensusHash, h.lastResultsHash, h.evidenceHash]) requireProof(b.length === 0 || b.length === 32);
  return root([join(num(1, h.versionBlock), num(2, h.versionApp)), bytes(1, utf8(h.chainId)), num(1, h.height), timestamp(h.time),
    blockId(h.lastBlockId), bytes(1, h.lastCommitHash), bytes(1, h.dataHash), bytes(1, h.validatorsHash), bytes(1, h.nextValidatorsHash),
    bytes(1, h.consensusHash), bytes(1, h.appHash), bytes(1, h.lastResultsHash), bytes(1, h.evidenceHash), bytes(1, h.proposerAddress)]);
}
export function cometValidatorsHash(validators: readonly CometValidator[]): Uint8Array {
  requireProof(validators.length > 0 && validators.length <= 10000);
  return root(validators.map(v => {
    requireProof(v.publicKey.length === 32 && v.votingPower > 0n && v.votingPower < 1n << 63n);
    return join(bytes(1, bytes(1, v.publicKey)), num(2, v.votingPower));
  }));
}
export function cometCommitHash(commit: CommittedReceiptEvidence["commit"]): Uint8Array {
  return root([num(1, commit.height), num(1, BigInt(commit.round)), blockId(commit.blockId),
    ...commit.signatures.map(s => join(num(1, BigInt(s.blockIdFlag)), bytes(2, s.validatorAddress), message(3, timestamp(s.timestamp)), bytes(4, s.signature)))]);
}
export function verifyCometProof(leaf: Uint8Array, proof: CometProof, expected: Uint8Array): void {
  requireProof(proof.total > 0n && proof.total < 1n << 63n && proof.index >= 0n && proof.index < proof.total
    && proof.aunts.length <= 63 && proof.aunts.every(a => a.length === 32) && expected.length === 32);
  let cursor = proof.aunts.length;
  function reconstruct(i: bigint, count: bigint): Uint8Array {
    if (count === 1n) { requireProof(cursor === 0); return hash(Uint8Array.of(0), leaf); }
    requireProof(cursor > 0);
    const aunt = proof.aunts[--cursor]!;
    const k = split(count);
    return i < k ? hash(Uint8Array.of(1), reconstruct(i, k), aunt) : hash(Uint8Array.of(1), aunt, reconstruct(i - k, count - k));
  }
  requireProof(same(reconstruct(proof.index, proof.total), expected) && cursor === 0);
}

interface Field { readonly tag: number; readonly wire: number; readonly data: Uint8Array; readonly value: bigint }
function parse(input: Uint8Array): Field[] {
  requireProof(input.length <= 4 * 1048576);
  let offset = 0;
  function read(): bigint {
    let value = 0n;
    for (let shift = 0n; shift < 70n; shift += 7n) {
      requireProof(offset < input.length);
      const next = input[offset++]!;
      requireProof(shift !== 63n || next <= 1);
      value |= BigInt(next & 127) << shift;
      if (next < 128) { requireProof(shift === 0n || next !== 0); return value; }
    }
    throw new SequencerReceiptVerificationError("inclusion_mismatch");
  }
  const fields: Field[] = [];
  while (offset < input.length) {
    const key = read(), tag = Number(key >> 3n), wire = Number(key & 7n);
    requireProof(tag > 0 && tag <= 536870911 && [0, 1, 2, 5].includes(wire));
    if (wire === 0) fields.push({ tag, wire, value: read(), data: new Uint8Array() });
    else {
      const length = wire === 2 ? Number(read()) : wire === 1 ? 8 : 4;
      requireProof(Number.isSafeInteger(length) && length <= input.length - offset);
      fields.push({ tag, wire, value: 0n, data: input.slice(offset, offset + length) });
      offset += length;
    }
  }
  return fields;
}
function field(fields: readonly Field[], tag: number, wire = 2): Field {
  const matches = fields.filter(f => f.tag === tag);
  requireProof(matches.length <= 1 && (matches.length === 0 || matches[0]!.wire === wire));
  return matches[0] ?? { tag, wire, data: new Uint8Array(), value: 0n };
}
const text = (b: Uint8Array): string => new TextDecoder("utf-8", { fatal: true }).decode(b);
function hexBytes(b: Uint8Array, length: number): Uint8Array {
  const s = text(b);
  requireProof(new RegExp(`^0x[0-9a-fA-F]{${length * 2}}$`, "u").test(s));
  return fromHex(s.slice(2));
}
function unsigned(n: bigint): Uint8Array {
  requireProof(n >= 0n && n < 1n << 256n);
  const h = n.toString(16);
  return n === 0n ? new Uint8Array() : fromHex(h.padStart(h.length + h.length % 2, "0"));
}
function rlp(b: Uint8Array, list = false): Uint8Array {
  if (!list && b.length === 1 && b[0]! < 128) return b.slice();
  const prefix = list ? 192 : 128;
  if (b.length <= 55) return join(Uint8Array.of(prefix + b.length), b);
  const size = unsigned(BigInt(b.length));
  return join(Uint8Array.of(prefix + 55 + size.length), size, b);
}
function evmValue(f: Field, format: string): Uint8Array {
  switch (format) {
    case "u": return unsigned(f.value);
    case "d": {
      if (f.data.length === 0) return f.data;
      const s = text(f.data); requireProof(/^(0|[1-9][0-9]{0,77})$/u.test(s)); return unsigned(BigInt(s));
    }
    case "h": return f.data.length === 0 ? f.data : hexBytes(f.data, 20);
    case "b": {
      requireProof(f.data.length <= 32);
      let start = 0; while (f.data[start] === 0) start++;
      return f.data.slice(start);
    }
    default: return f.data;
  }
}
function evmList(values: readonly Field[], kind: string): Uint8Array {
  return rlp(join(...values.map(v => {
    requireProof(v.wire === 2);
    if (kind === "x") { requireProof(v.data.length === 32); return rlp(v.data); }
    const fs = parse(v.data);
    if (kind === "o") return rlp(join(...[..."dhubbb"].map((fmt, i) => rlp(evmValue(field(fs, i + 1, fmt === "u" ? 0 : 2), fmt)))), true);
    return rlp(join(rlp(hexBytes(field(fs, 1).data, 20)), rlp(join(...fs.filter(f => f.tag === 2).map(f => {
      requireProof(f.wire === 2); return rlp(hexBytes(f.data, 32));
    })), true)), true);
  })), true);
}
export function unwrapCometEvmTransaction(transaction: Uint8Array): Uint8Array {
  requireProof(transaction.length > 0 && transaction.length <= 1048576);
  const tx = parse(transaction), body = parse(field(tx, 1).data);
  requireProof(body.filter(f => f.tag === 1).length === 1);
  const msg = parse(field(body, 1).data);
  requireProof(["/paxprotocol.paxchain.evm.MsgEVMTransaction", "/ethermint.evm.v1.MsgEthereumTx"].includes(text(field(msg, 1).data)));
  const data = field(parse(field(msg, 2).data), 1).data;
  requireProof(data.length > 0);
  if (data[0]! >= 192 || data[0]! >= 1 && data[0]! <= 4) return data;
  const any = parse(data), kind = text(field(any, 1).data), fs = parse(field(any, 2).data);
  const layouts: Readonly<Record<string, readonly [number, string]>> = {
    "/paxprotocol.paxchain.eth.LegacyTx": [0, "uduhdrbbb"],
    "/paxprotocol.paxchain.eth.AccessListTx": [1, "duduhdrabbb"],
    "/paxprotocol.paxchain.eth.DynamicFeeTx": [2, "dudduhdrabbb"],
    "/paxprotocol.paxchain.eth.BlobTx": [3, "dudduhdradx_bbb"],
    "/paxprotocol.paxchain.eth.SetCodeTx": [4, "dudduhdraobbb"],
  };
  const layout = layouts[kind]; requireProof(layout !== undefined);
  const encoded = rlp(join(...[...layout[1]].flatMap((fmt, i) => fmt === "_" ? [] : [
    ["a", "x", "o"].includes(fmt) ? evmList(fs.filter(f => f.tag === i + 1), fmt) : rlp(evmValue(field(fs, i + 1, fmt === "u" ? 0 : 2), fmt)),
  ])), true);
  return layout[0] === 0 ? encoded : join(Uint8Array.of(layout[0]), encoded);
}

function verifyResult(result: CommittedReceiptInclusion["result"], verified: SequencerReceiptVerification): void {
  const receipt = verified.receipt, summary = receipt.effects[0]?.body;
  requireProof(summary !== undefined && summary.length >= 108 && result.code === 0 && receipt.resultCode === 0
    && result.gasUsed >= 0n && result.gasWanted >= result.gasUsed && result.gasWanted < 1n << 63n);
  const view = new DataView(summary.buffer, summary.byteOffset, summary.byteLength);
  requireProof(view.getBigUint64(0) === result.gasUsed);
  const messages = parse(result.data).filter(f => f.tag === 1 || f.tag === 2);
  requireProof(messages.length === 1 && messages[0]!.wire === 2);
  const msg = parse(messages[0]!.data);
  requireProof(["evm", "/paxprotocol.paxchain.evm.MsgEVMTransaction", "/ethermint.evm.v1.MsgEthereumTx"].includes(text(field(msg, 1).data)));
  const response = parse(field(msg, 2).data), returned = field(response, 3).data;
  requireProof(field(response, 1, 0).value === result.gasUsed && same(hash(field(response, 2).data), summary.slice(68, 100))
    && returned.length === view.getUint32(100) && same(hash(returned), receipt.activityRoot)
    && same(hexBytes(field(response, 4).data, 32), receipt.activityId));
  const logs = response.filter(f => f.tag === 5);
  requireProof(logs.length === receipt.effects.length - 1 && logs.length === view.getUint32(104));
  logs.forEach((f, i) => {
    requireProof(f.wire === 2);
    const log = parse(f.data), committed = receipt.effects[i + 1]!.body, topics = log.filter(f => f.tag === 2);
    requireProof(topics.length <= 4 && topics.every(topic => topic.wire === 2));
    requireProof(same(encodeSequencerLogEffect(hexBytes(field(log, 1).data, 20),
      topics.map(topic => hexBytes(topic.data, 32)), field(log, 3).data), committed));
  });
}

export async function verifyReceiptInclusion(verified: SequencerReceiptVerification, evidence: CommittedReceiptEvidence,
  raw: Uint8Array, keys: readonly SequencerReceiptKey[]): Promise<void> {
  const p = evidence.inclusion; requireProof(p !== undefined);
  const h = p.executionHeader, next = p.successorHeader, commit = p.executionCommit;
  requireProof(h.height === verified.height && evidence.landingHeight === next.height && next.height === h.height + 1n
    && h.chainId === evidence.chainId && next.chainId === evidence.chainId
    && same(cometHeaderHash(h), commit.blockId.hash) && commit.height === h.height
    && same(cometHeaderHash(next), evidence.commit.blockId.hash)
    && same(blockId(next.lastBlockId), blockId(commit.blockId)) && same(next.lastCommitHash, cometCommitHash(commit))
    && same(h.validatorsHash, cometValidatorsHash(p.executionValidators)) && same(next.validatorsHash, cometValidatorsHash(evidence.validators))
    && same(h.nextValidatorsHash, next.validatorsHash));
  await verifyCometCommit({ chainId: evidence.chainId, landingHeight: h.height, blockId: commit.blockId, commit, validators: p.executionValidators });
  const tp = p.transactionProof, bp = p.batchProof, rp = p.resultProof;
  requireProof(tp.index > 0n && tp.index === rp.index && tp.total === rp.total && tp.total === bp.total && bp.index === 0n);
  const summary = verified.receipt.effects[0]?.body;
  requireProof(summary !== undefined && summary.length >= 28 && BigInt(new DataView(summary.buffer, summary.byteOffset).getUint32(24)) === tp.index);
  requireProof(same(unwrapCometEvmTransaction(p.transaction), raw));
  verifyCometProof(hash(p.transaction), tp, h.dataHash);
  verifyCometProof(hash(p.batchTransaction), bp, h.dataHash);
  const resultBytes = join(num(1, BigInt(p.result.code)), bytes(2, p.result.data), num(5, p.result.gasWanted), num(6, p.result.gasUsed));
  verifyCometProof(resultBytes, rp, next.lastResultsHash);
  verifyResult(p.result, verified);
  const domain = utf8("PAXEER/v1/sequence-batch\0"), batch = p.batchTransaction, offset = domain.length;
  requireProof(batch.length === offset + 24 + 128 + 64 && same(batch.subarray(0, offset), domain));
  const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
  const height = view.getBigUint64(offset), from = view.getBigUint64(offset + 8), to = view.getBigUint64(offset + 16);
  requireProof(height === h.height && from > 0n && to >= from && verified.receipt.globalSequence === from + tp.index - 1n && tp.total === to - from + 2n);
  const keyId = batch.subarray(offset + 120, offset + 152);
  const pins = keys.filter(k => same(k.keyId, keyId) && k.first_height <= height && k.last_height >= height);
  requireProof(pins.length > 0 && same(keyId, verified.receipt.contextHash));
  const digest = hash(domain, utf8(evidence.chainId), batch.subarray(offset, offset + 120));
  requireProof(pins.some(k => ed25519.verify(batch.subarray(offset + 152), digest, k.publicKey, { zip215: false })));
}

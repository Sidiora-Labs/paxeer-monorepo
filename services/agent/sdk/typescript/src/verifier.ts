import { PlatformSdkError } from "./production.js";
import { ed25519 } from "@noble/curves/ed25519.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { ReceiptFailureCode } from "./generated/receipt.js";

const RECEIPT_DOMAIN = new TextEncoder().encode("LXP/v1/receipt\0");
const MAX_MESSAGE_BYTES = 1_048_576;
const MAX_EFFECTS = 512;
const MAX_EFFECT_BODY = 256;
const MAX_U128 = 0xffff_ffff_ffff_ffff_ffff_ffff_ffff_ffffn;
export const LEGACY_PROTOCOL_VERSION = 1;
export const DEFAULT_PROTOCOL_VERSION = 2;
export const STATE_COMMITMENT_PROTOCOL_VERSION = 3;

export type SelectableProtocolVersion = typeof DEFAULT_PROTOCOL_VERSION | typeof STATE_COMMITMENT_PROTOCOL_VERSION;

export interface ProtocolSelection {
  readonly protocolVersion?: SelectableProtocolVersion;
}

export function isSelectableProtocolVersion(version: number): version is SelectableProtocolVersion {
  return version === DEFAULT_PROTOCOL_VERSION || version === STATE_COMMITMENT_PROTOCOL_VERSION;
}

export function protocolVersionUsesOccupancy(version: number): boolean {
  return isSelectableProtocolVersion(version);
}

export function selectedProtocolVersion(selection?: ProtocolSelection): SelectableProtocolVersion {
  const version = selection?.protocolVersion ?? DEFAULT_PROTOCOL_VERSION;
  if (!isSelectableProtocolVersion(version)) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  return version;
}

export function supportedProgramGuestAbi(abiVersion: number): boolean {
  return abiVersion === 1 || abiVersion === 2;
}

export interface ReceiptEffect {
  readonly moduleId: number;
  readonly ordinal: number;
  readonly eventType: number;
  readonly kind: 1 | 2 | 3;
  readonly monetary: boolean;
  readonly transferSetRoot: Uint8Array;
  readonly body: Uint8Array;
}

export interface SequencerReceipt {
  readonly protocolVersion: number;
  readonly activityId: Uint8Array;
  readonly globalSequence: bigint;
  readonly previousStateRoot: Uint8Array;
  readonly resultingStateRoot: Uint8Array;
  readonly activityRoot: Uint8Array;
  readonly resultCode: number;
  readonly effects: readonly ReceiptEffect[];
  readonly feeCharged: bigint;
  readonly batchId: Uint8Array;
  readonly moduleId: number;
  readonly moduleVersion: number;
  readonly parameterVersion: number;
  readonly operation: number;
  readonly asset: Uint8Array;
  readonly amount: bigint;
  readonly from: Uint8Array;
  readonly fromBalanceBefore: bigint;
  readonly fromBalanceAfter: bigint;
  readonly fromSequence: bigint;
  readonly to: Uint8Array;
  readonly toBalanceBefore: bigint;
  readonly toBalanceAfter: bigint;
  readonly transferSetRoot: Uint8Array;
  readonly authorizationHash: Uint8Array;
  readonly contextHash: Uint8Array;
  readonly timestamp: bigint;
  readonly totalUnits?: readonly [bigint, bigint];
  readonly sequencerSignature: Uint8Array;
}

export interface ProtocolReceipt extends SequencerReceipt {}

export interface AuthorizedReceiptBatch {
  readonly batchId: Uint8Array;
  readonly asset: Uint8Array;
  readonly previousStateRoot: Uint8Array;
  readonly resultingStateRoot: Uint8Array;
  readonly sequencerPublicKey: Uint8Array;
}

export interface SequencerAuthorization {
  readonly sequencerId: Uint8Array;
  readonly publicKey: Uint8Array;
  readonly firstBatchNumber: bigint;
  readonly lastBatchNumber: bigint;
}

export interface ReceiptVerification {
  readonly level: "sequencer-signed";
  readonly receipt: SequencerReceipt;
  readonly canonicalBytes: Uint8Array;
  readonly receiptDigest: Uint8Array;
}

export class ReceiptVerificationError extends PlatformSdkError {
  public readonly check: ReceiptFailureCode;

  public constructor(check: ReceiptFailureCode) {
    super({ code: "verification-failure", retry: "never" });
    this.name = "ReceiptVerificationError";
    this.check = check;
  }
}

function receiptFailure(check: ReceiptFailureCode): never {
  throw new ReceiptVerificationError(check);
}

function verificationFailure(): never {
  throw new PlatformSdkError({ code: "verification-failure", retry: "never" });
}

function exactBytes(value: Uint8Array, length: number): Uint8Array {
  if (value.length !== length) {
    return verificationFailure();
  }
  return value;
}

function arrayBuffer(value: Uint8Array): ArrayBuffer {
  const copy = new ArrayBuffer(value.length);
  new Uint8Array(copy).set(value);
  return copy;
}

function concatenate(...values: readonly Uint8Array[]): Uint8Array {
  const length = values.reduce((total, value) => total + value.length, 0);
  const result = new Uint8Array(length);
  let offset = 0;
  for (const value of values) {
    result.set(value, offset);
    offset += value.length;
  }
  return result;
}

async function sha256(...values: readonly Uint8Array[]): Promise<Uint8Array> {
  const digest = await globalThis.crypto.subtle.digest("SHA-256", arrayBuffer(concatenate(...values)));
  return new Uint8Array(digest);
}

function equal(left: Uint8Array, right: Uint8Array): boolean {
  if (left.length !== right.length) {
    return false;
  }
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= (left[index] ?? 0) ^ (right[index] ?? 0);
  }
  return difference === 0;
}

class Decoder {
  #offset = 0;

  public constructor(private readonly bytes: Uint8Array) {}

  public finish(): void {
    if (this.#offset !== this.bytes.length) {
      return verificationFailure();
    }
  }

  public u8(): number {
    const value = this.bytes[this.#offset];
    if (value === undefined) {
      return verificationFailure();
    }
    this.#offset += 1;
    return value;
  }

  public u16(): number {
    return Number(this.integer(2));
  }

  public u32(): number {
    return Number(this.integer(4));
  }

  public u64(): bigint {
    return this.integer(8);
  }

  public u128(): bigint {
    return this.integer(16);
  }

  public i32(): number {
    const value = this.u32();
    return value > 0x7fff_ffff ? value - 0x1_0000_0000 : value;
  }

  public position(): number {
    return this.#offset;
  }

  public fixed(length: number): Uint8Array {
    const end = this.#offset + length;
    if (!Number.isSafeInteger(end) || end > this.bytes.length) {
      return verificationFailure();
    }
    const value = this.bytes.slice(this.#offset, end);
    this.#offset = end;
    return value;
  }

  public bounded(length: number): Uint8Array {
    if (this.u32() !== length) {
      return verificationFailure();
    }
    return this.fixed(length);
  }

  public boundedAtMost(maximum: number): Uint8Array {
    const length = this.u32();
    if (length > maximum) {
      return verificationFailure();
    }
    return this.fixed(length);
  }

  private integer(length: number): bigint {
    const bytes = this.fixed(length);
    let value = 0n;
    for (const byte of bytes) {
      value = (value << 8n) | BigInt(byte);
    }
    return value;
  }
}

async function verifyEd25519(publicKey: Uint8Array, signature: Uint8Array, digest: Uint8Array): Promise<boolean> {
  try {
    const key = await globalThis.crypto.subtle.importKey(
      "raw",
      arrayBuffer(exactBytes(publicKey, 32)),
      { name: "Ed25519" },
      false,
      ["verify"],
    );
    return await globalThis.crypto.subtle.verify(
      { name: "Ed25519" },
      key,
      arrayBuffer(exactBytes(signature, 64)),
      arrayBuffer(exactBytes(digest, 32)),
    );
  } catch {
    return false;
  }
}

interface DecodedReceipt {
  readonly receipt: SequencerReceipt;
  readonly unsignedBytes: Uint8Array;
}

function allZero(value: Uint8Array): boolean {
  let aggregate = 0;
  for (const byte of value) {
    aggregate |= byte;
  }
  return aggregate === 0;
}

function decodeSequencerReceipt(canonicalReceipt: Uint8Array): DecodedReceipt {
  if (canonicalReceipt.length === 0 || canonicalReceipt.length > MAX_MESSAGE_BYTES) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  const decoder = new Decoder(canonicalReceipt);
  const envelopeVersion = decoder.u16();
  const structureTag = decoder.u16();
  if (
    envelopeVersion !== DEFAULT_PROTOCOL_VERSION
    || (structureTag !== 0x5201 && structureTag !== 0x5202)
  ) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  const protocolVersion = decoder.u16();
  if (protocolVersion !== envelopeVersion) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  const activityId = decoder.bounded(32);
  const globalSequence = decoder.u64();
  const previousStateRoot = decoder.bounded(32);
  const resultingStateRoot = decoder.bounded(32);
  const activityRoot = decoder.bounded(32);
  const resultCode = decoder.i32();
  const effectCount = decoder.u32();
  if (effectCount > MAX_EFFECTS) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  const effects: ReceiptEffect[] = [];
  for (let index = 0; index < effectCount; index += 1) {
    const moduleId = decoder.u16();
    const ordinal = decoder.u16();
    const eventType = decoder.u16();
    const kindValue = decoder.u8();
    if (kindValue < 1 || kindValue > 3) {
      return receiptFailure(ReceiptFailureCode.Decode);
    }
    const monetaryValue = decoder.u8();
    if (monetaryValue > 1 || (monetaryValue === 1 && kindValue !== 2)) {
      return receiptFailure(ReceiptFailureCode.Decode);
    }
    effects.push(Object.freeze({
      moduleId,
      ordinal,
      eventType,
      kind: kindValue as 1 | 2 | 3,
      monetary: monetaryValue === 1,
      transferSetRoot: decoder.bounded(32),
      body: decoder.boundedAtMost(MAX_EFFECT_BODY),
    }));
  }
  const feeCharged = decoder.u128();
  const batchId = decoder.bounded(32);
  const moduleId = decoder.u16();
  const moduleVersion = decoder.u32();
  const parameterVersion = decoder.u32();
  const operation = decoder.u8();
  const asset = decoder.bounded(32);
  const amount = decoder.u128();
  const from = decoder.bounded(32);
  const fromBalanceBefore = decoder.u128();
  const fromBalanceAfter = decoder.u128();
  const fromSequence = decoder.u64();
  const to = decoder.bounded(32);
  const toBalanceBefore = decoder.u128();
  const toBalanceAfter = decoder.u128();
  const transferSetRoot = decoder.bounded(32);
  const authorizationHash = decoder.bounded(32);
  const contextHash = decoder.bounded(32);
  const timestamp = decoder.u64();
  let totalUnits: readonly [bigint, bigint] | undefined;
  if (structureTag === 0x5202) {
    const before = decoder.u128();
    const after = decoder.u128();
    const expected = operation === 1 && before === 0n ? 0n
      : operation >= 2 && operation <= 8 ? before
      : operation === 10 && amount > 0n && before + amount <= MAX_U128 ? before + amount
      : operation === 11 && amount > 0n && before >= amount ? before - amount
      : undefined;
    if (moduleId !== 1 || resultCode !== 0 || allZero(asset) || expected !== after) {
      return receiptFailure(ReceiptFailureCode.CanonicalEncoding);
    }
    totalUnits = Object.freeze([before, after]);
  }
  if (globalSequence === 0n) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  if (moduleId === 0) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  if (moduleVersion === 0) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  if (timestamp === 0n) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  if (allZero(activityId)) {
    return receiptFailure(ReceiptFailureCode.ActivityId);
  }
  if (allZero(resultingStateRoot)) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  const signatureFlagOffset = decoder.position();
  if (decoder.u8() !== 1) {
    return receiptFailure(ReceiptFailureCode.Decode);
  }
  const sequencerSignature = decoder.bounded(64);
  decoder.finish();
  return Object.freeze({
    receipt: Object.freeze({
      protocolVersion,
      activityId,
      globalSequence,
      previousStateRoot,
      resultingStateRoot,
      activityRoot,
      resultCode,
      effects: Object.freeze(effects),
      feeCharged,
      batchId,
      moduleId,
      moduleVersion,
      parameterVersion,
      operation,
      asset,
      amount,
      from,
      fromBalanceBefore,
      fromBalanceAfter,
      fromSequence,
      to,
      toBalanceBefore,
      toBalanceAfter,
      transferSetRoot,
      authorizationHash,
      contextHash,
      timestamp,
      ...(totalUnits === undefined ? {} : { totalUnits }),
      sequencerSignature,
    }),
    unsignedBytes: concatenate(canonicalReceipt.slice(0, signatureFlagOffset), new Uint8Array([0])),
  });
}

export type SequencerReceiptRefusal =
  | "unpinned_key"
  | "key_out_of_range"
  | "bad_signature"
  | "hash_mismatch"
  | "height_regression"
  | "insufficient_power";

export class SequencerReceiptVerificationError extends PlatformSdkError {
  public constructor(public readonly check: SequencerReceiptRefusal) {
    super({ code: "verification-failure", retry: "never" });
    this.name = "SequencerReceiptVerificationError";
  }
}

export interface SequencerReceiptKey {
  readonly keyId: Uint8Array;
  readonly publicKey: Uint8Array;
  readonly first_height: bigint;
  readonly last_height: bigint;
}

export interface SequencerReceiptVerification extends ReceiptVerification {
  readonly height: bigint;
}

export interface CometBlockId {
  readonly hash: Uint8Array;
  readonly partSetHeader: {
    readonly total: number;
    readonly hash: Uint8Array;
  };
}

export interface CometCommitSignature {
  readonly blockIdFlag: 1 | 2 | 3;
  readonly validatorAddress: Uint8Array;
  readonly timestamp: { readonly seconds: bigint; readonly nanos: number };
  readonly signature: Uint8Array;
}

export interface CometValidator {
  readonly address: Uint8Array;
  readonly publicKey: Uint8Array;
  readonly votingPower: bigint;
}

export interface CommittedReceiptEvidence {
  readonly chainId: string;
  readonly landingHeight: bigint;
  readonly blockId: CometBlockId;
  readonly commit: {
    readonly height: bigint;
    readonly round: number;
    readonly blockId: CometBlockId;
    readonly signatures: readonly CometCommitSignature[];
  };
  /** The validator set trusted by the caller for this chain and landing height. */
  readonly validators: readonly CometValidator[];
}

export interface CommittedReceiptVerification {
  readonly level: "committed";
  readonly receipt: SequencerReceiptVerification;
  readonly landingHeight: bigint;
  readonly blockHash: Uint8Array;
  readonly signedPower: bigint;
  readonly totalPower: bigint;
}

function sequencerReceiptFailure(check: SequencerReceiptRefusal): never {
  throw new SequencerReceiptVerificationError(check);
}

export async function verifySequencerReceipt(
  signedReceipt: Uint8Array,
  rawTransaction: Uint8Array,
  receiptKeys: readonly SequencerReceiptKey[],
): Promise<SequencerReceiptVerification> {
  const canonicalBytes = new Uint8Array(signedReceipt);
  const transactionHash = keccak_256(new Uint8Array(rawTransaction));
  let decoded: DecodedReceipt;
  try {
    decoded = decodeSequencerReceipt(canonicalBytes);
  } catch {
    return sequencerReceiptFailure("bad_signature");
  }
  const { receipt, unsignedBytes } = decoded;
  if (receipt.protocolVersion !== 2 || receipt.moduleVersion !== 1
    || receipt.parameterVersion !== 0
    || !allZero(receipt.batchId.subarray(0, 24))) {
    return sequencerReceiptFailure("bad_signature");
  }
  const height = new DataView(arrayBuffer(receipt.batchId)).getBigUint64(24, false);
  const pinned = receiptKeys.filter((key) => equal(key.keyId, receipt.contextHash));
  if (pinned.length === 0) return sequencerReceiptFailure("unpinned_key");
  const active = pinned.filter((key) => key.first_height >= 0n
    && key.last_height <= 0xffff_ffff_ffff_ffffn
    && key.first_height <= height && height <= key.last_height);
  if (active.length === 0) return sequencerReceiptFailure("key_out_of_range");
  const publicKeys = active.map((key) => new Uint8Array(key.publicKey));
  const receiptDigest = await sha256(RECEIPT_DOMAIN, unsignedBytes);
  let valid = false;
  for (const publicKey of publicKeys) {
    if (await verifyEd25519(publicKey, receipt.sequencerSignature, receiptDigest)) valid = true;
  }
  if (!valid) return sequencerReceiptFailure("bad_signature");
  if (!equal(receipt.activityId, transactionHash)) return sequencerReceiptFailure("hash_mismatch");
  return Object.freeze({ level: "sequencer-signed", receipt, canonicalBytes, receiptDigest, height });
}

function cometVarint(value: bigint): Uint8Array {
  const bytes: number[] = [];
  let remaining = BigInt.asUintN(64, value);
  while (remaining >= 128n) {
    bytes.push(Number(remaining & 127n) | 128);
    remaining >>= 7n;
  }
  bytes.push(Number(remaining));
  return Uint8Array.from(bytes);
}

function cometBytes(field: number, bytes: Uint8Array): Uint8Array {
  return concatenate(Uint8Array.of(field * 8 + 2), cometVarint(BigInt(bytes.length)), bytes);
}

function cometFixed64(field: number, value: bigint): Uint8Array {
  if (value === 0n) return new Uint8Array();
  const bytes = new Uint8Array(9);
  bytes[0] = field * 8 + 1;
  new DataView(bytes.buffer).setBigInt64(1, value, true);
  return bytes;
}

function cometVoteSignBytes(evidence: CommittedReceiptEvidence, signature: CometCommitSignature): Uint8Array {
  const { blockId, height, round } = evidence.commit;
  const { seconds, nanos } = signature.timestamp;
  if (seconds < -62135596800n || seconds > 253402300799n
    || !Number.isInteger(nanos) || nanos < 0 || nanos >= 1_000_000_000) {
    return sequencerReceiptFailure("bad_signature");
  }
  const partSet = concatenate(Uint8Array.of(8), cometVarint(BigInt(blockId.partSetHeader.total)),
    cometBytes(2, blockId.partSetHeader.hash));
  const block = concatenate(cometBytes(1, blockId.hash), cometBytes(2, partSet));
  const timestamp = concatenate(
    seconds === 0n ? new Uint8Array() : concatenate(Uint8Array.of(8), cometVarint(seconds)),
    nanos === 0 ? new Uint8Array() : concatenate(Uint8Array.of(16), cometVarint(BigInt(nanos))),
  );
  const vote = concatenate(Uint8Array.of(8, 2), cometFixed64(2, height), cometFixed64(3, BigInt(round)),
    cometBytes(4, block), cometBytes(5, timestamp), cometBytes(6, new TextEncoder().encode(evidence.chainId)));
  return concatenate(cometVarint(BigInt(vote.length)), vote);
}

export async function verifyCommittedReceipt(
  receipt: SequencerReceiptVerification,
  evidence: CommittedReceiptEvidence,
): Promise<CommittedReceiptVerification> {
  if (evidence.landingHeight < receipt.height || evidence.commit.height !== evidence.landingHeight) {
    return sequencerReceiptFailure("height_regression");
  }
  const { commit, validators } = evidence;
  const blockId = commit.blockId;
  if (commit.height <= 0n || commit.height > 0x7fff_ffff_ffff_ffffn
    || !Number.isInteger(commit.round) || commit.round < 0 || commit.round > 0x7fff_ffff
    || evidence.chainId.length === 0 || new TextEncoder().encode(evidence.chainId).length > 50
    || blockId.hash.length !== 32 || blockId.partSetHeader.hash.length !== 32
    || !Number.isInteger(blockId.partSetHeader.total) || blockId.partSetHeader.total <= 0
    || blockId.partSetHeader.total > 0xffff_ffff
    || !equal(blockId.hash, evidence.blockId.hash)
    || !equal(blockId.partSetHeader.hash, evidence.blockId.partSetHeader.hash)
    || blockId.partSetHeader.total !== evidence.blockId.partSetHeader.total) {
    return sequencerReceiptFailure("bad_signature");
  }
  let totalPower = 0n;
  const addresses: Uint8Array[] = [];
  for (const validator of validators) {
    if (validator.votingPower <= 0n || validator.votingPower > 0x7fff_ffff_ffff_ffffn
      || validator.publicKey.length !== 32 || validator.address.length !== 20
      || addresses.some((address) => equal(address, validator.address))
      || !equal((await sha256(validator.publicKey)).subarray(0, 20), validator.address)) {
      return sequencerReceiptFailure("bad_signature");
    }
    addresses.push(validator.address);
    totalPower += validator.votingPower;
  }
  let signedPower = 0n;
  const seen: Uint8Array[] = [];
  for (const signature of commit.signatures) {
    if (signature.blockIdFlag === 1 || signature.blockIdFlag === 3) continue;
    const validator = validators.find((entry) => equal(entry.address, signature.validatorAddress));
    if (signature.blockIdFlag !== 2 || validator === undefined
      || seen.some((address) => equal(address, signature.validatorAddress))) {
      return sequencerReceiptFailure("bad_signature");
    }
    let valid = false;
    try {
      valid = ed25519.verify(signature.signature, cometVoteSignBytes(evidence, signature), validator.publicKey,
        { zip215: false });
    } catch {
      return sequencerReceiptFailure("bad_signature");
    }
    if (!valid) return sequencerReceiptFailure("bad_signature");
    seen.push(signature.validatorAddress);
    signedPower += validator.votingPower;
  }
  if (totalPower === 0n || signedPower * 3n <= totalPower * 2n) {
    return sequencerReceiptFailure("insufficient_power");
  }
  return Object.freeze({ level: "committed", receipt, landingHeight: evidence.landingHeight,
    blockHash: new Uint8Array(blockId.hash), signedPower, totalPower });
}

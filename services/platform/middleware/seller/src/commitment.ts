import {
  PlatformSdkError,
  verifyCommittedReceipt,
  type CommittedReceiptEvidence,
  type CometHeader,
  type NodeReceiptInclusion,
  cometValidatorSetHash,
  type SequencerReceiptVerification,
} from "@sidiora/layerx-sdk";

export type PaymentCommitment = "receipted" | "committed";

export interface PaymentCommitmentEvidence {
  readonly landingBlockHeight: CommittedReceiptEvidence["landingHeight"];
  readonly blockHeader: CometHeader & {
    readonly chainId: CommittedReceiptEvidence["chainId"];
    readonly blockId: CommittedReceiptEvidence["blockId"];
  };
  readonly receiptInclusion: NodeReceiptInclusion & {
    readonly transactionHash: Uint8Array;
    readonly receiptDigest: Uint8Array;
    readonly sequence: bigint;
  };
  readonly commit: CommittedReceiptEvidence["commit"];
  readonly validatorSet: CommittedReceiptEvidence["validators"];
}

export interface TrustedPaymentValidators {
  readonly chainId: string;
  readonly validatorsHash: Uint8Array;
}

export interface PaymentValidatorResolver {
  resolveValidators(network: string, height: bigint): Promise<TrustedPaymentValidators>;
}

export interface PaymentCommitmentResolver extends PaymentValidatorResolver {
  resolve(transactionHash: string, network: string): Promise<PaymentCommitmentEvidence>;
}

function failure(): never {
  throw new PlatformSdkError({ code: "verification-failure", retry: "never" });
}

export function paymentCommitment(extra: unknown): PaymentCommitment {
  if (extra === undefined || extra === null || typeof extra !== "object" || Array.isArray(extra)) return "receipted";
  const layerx = (extra as Record<string, unknown>)["layerx"];
  if (layerx === undefined) return "receipted";
  if (layerx === null || typeof layerx !== "object" || Array.isArray(layerx)) return failure();
  const commitment = (layerx as Record<string, unknown>)["commitment"];
  if (commitment !== "receipted" && commitment !== "committed") return failure();
  return commitment;
}

function layerXTerms(extra: unknown): Record<string, unknown> | undefined {
  if (extra === undefined || extra === null || typeof extra !== "object" || Array.isArray(extra)) return undefined;
  const layerx = (extra as Record<string, unknown>)["layerx"];
  if (layerx === undefined) return undefined;
  if (layerx === null || typeof layerx !== "object" || Array.isArray(layerx)) return failure();
  return layerx as Record<string, unknown>;
}

export function paymentPayer(extra: unknown, required = false): string | undefined {
  const payer = layerXTerms(extra)?.["payer"];
  if (payer === undefined && !required) return undefined;
  if (typeof payer !== "string" || !/^[0-9a-f]{64}$/u.test(payer) || /^0+$/u.test(payer)) return failure();
  return payer;
}

export function paymentPurpose(extra: unknown, required = false): string | undefined {
  const purpose = layerXTerms(extra)?.["purposeHash"];
  if (purpose === undefined && !required) return undefined;
  if (typeof purpose !== "string" || !/^[0-9a-f]{64}$/u.test(purpose) || /^0+$/u.test(purpose)) return failure();
  return purpose;
}

export const MAX_PAYMENT_ACCOUNT_BYTES = 512;
export const MAX_PAYMENT_DID_BYTES = 255;
export const MAX_PAYMENT_CURRENCY_BYTES = 16;

export function paymentAccount(extra: unknown, required = false): string | undefined {
  const account = layerXTerms(extra)?.["account"];
  if (account === undefined && !required) return undefined;
  if (typeof account !== "string" || account.length > MAX_PAYMENT_ACCOUNT_BYTES) return failure();
  const did = /^agent:([a-z0-9._:-]+):main$/u.exec(account)?.[1];
  if (did === undefined || did.length > MAX_PAYMENT_DID_BYTES || did.startsWith(":") || did.endsWith(":")
    || did.includes("::") || did.includes(":asset:")) return failure();
  return account;
}

export function paymentCurrency(extra: unknown, required = false): string | undefined {
  const currency = layerXTerms(extra)?.["currency"];
  if (currency === undefined && !required) return undefined;
  if (typeof currency !== "string" || currency.length > MAX_PAYMENT_CURRENCY_BYTES || !/^[A-Z0-9]+$/u.test(currency)) {
    return failure();
  }
  return currency;
}

function equal(left: Uint8Array, right: Uint8Array): boolean {
  return left.length === right.length && left.every((byte, index) => byte === right[index]);
}

export async function verifyPaymentCommitment(
  verified: SequencerReceiptVerification,
  network: string,
  commitment: PaymentCommitment,
  resolver?: PaymentCommitmentResolver,
): Promise<void> {
  if (commitment === "receipted") return;
  if (commitment !== "committed" || resolver === undefined) return failure();
  if (!/^eip155:[1-9][0-9]*$/u.test(network)
    || BigInt(network.slice(7)) > 0xffff_ffffn) return failure();
  verified = structuredClone(verified);
  const transactionHash = `0x${Array.from(verified.receipt.activityId,
    (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
  const evidence = await resolver.resolve(transactionHash, network);
  await verifyPaymentCommitmentEvidence(verified, commitment, evidence, network, resolver);
}

export async function verifyPaymentCommitmentEvidence(
  verified: SequencerReceiptVerification,
  commitment: PaymentCommitment,
  evidence: PaymentCommitmentEvidence,
  network: string,
  validators: PaymentValidatorResolver,
): Promise<void> {
  if (commitment === "receipted") return;
  if (commitment !== "committed") return failure();
  try {
    verified = structuredClone(verified);
    evidence = structuredClone(evidence);
    if (typeof verified.height !== "bigint" || verified.height < 0n) return failure();
    if (!/^eip155:[1-9][0-9]*$/u.test(network) || BigInt(network.slice(7)) > 0xffff_ffffn
      || validators === undefined) return failure();
    const anchor = structuredClone(await validators.resolveValidators(network, evidence.landingBlockHeight));
    if (anchor.chainId !== evidence.blockHeader.chainId || anchor.validatorsHash.length !== 32
      || !equal(anchor.validatorsHash, evidence.blockHeader.validatorsHash)
      || !equal(await cometValidatorSetHash(evidence.validatorSet), anchor.validatorsHash)) return failure();
    const inclusion = evidence.receiptInclusion;
    if (!equal(inclusion.transactionHash, verified.receipt.activityId)
      || !equal(inclusion.receiptDigest, verified.receiptDigest)
      || inclusion.sequence !== verified.receipt.globalSequence) return failure();
    const nodeEvidence = {
      header: evidence.blockHeader,
      inclusion: evidence.receiptInclusion,
      chainId: evidence.blockHeader.chainId,
      landingHeight: evidence.landingBlockHeight,
      blockId: evidence.blockHeader.blockId,
      commit: evidence.commit,
      validators: evidence.validatorSet,
    };
    await verifyCommittedReceipt(verified, nodeEvidence);
  } catch {
    return failure();
  }
}

export function encodePaymentCommitmentEvidence(evidence: PaymentCommitmentEvidence) {
  const hex = (value: Uint8Array): string => `0x${Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
  const quantity = (value: bigint): string => `0x${value.toString(16)}`;
  const blockId = (value: PaymentCommitmentEvidence["blockHeader"]["blockId"]) => ({
    hash: hex(value.hash),
    partSetHeader: { total: value.partSetHeader.total, hash: hex(value.partSetHeader.hash) },
  });
  return {
    landingBlockHeight: quantity(evidence.landingBlockHeight),
    blockHeader: {
      ...Object.fromEntries(["lastCommitHash", "dataHash", "validatorsHash", "nextValidatorsHash", "consensusHash", "appHash", "lastResultsHash", "evidenceHash", "proposerAddress"].map((field) => [field, hex(evidence.blockHeader[field as "dataHash"])])),
      versionBlock: quantity(evidence.blockHeader.versionBlock), versionApp: quantity(evidence.blockHeader.versionApp),
      height: quantity(evidence.blockHeader.height), time: { seconds: quantity(evidence.blockHeader.time.seconds), nanos: evidence.blockHeader.time.nanos },
      lastBlockId: blockId(evidence.blockHeader.lastBlockId),
      chainId: evidence.blockHeader.chainId, blockId: blockId(evidence.blockHeader.blockId),
    },
    receiptInclusion: {
      transactionHash: hex(evidence.receiptInclusion.transactionHash),
      receiptDigest: hex(evidence.receiptInclusion.receiptDigest),
      sequence: quantity(evidence.receiptInclusion.sequence),
      batchTransaction: hex(evidence.receiptInclusion.batchTransaction),
      batchProof: { total: quantity(evidence.receiptInclusion.batchProof.total), index: quantity(evidence.receiptInclusion.batchProof.index),
        aunts: evidence.receiptInclusion.batchProof.aunts.map(hex) },
      activityIds: evidence.receiptInclusion.activityIds.map(hex),
    },
    commit: {
      height: quantity(evidence.commit.height),
      round: evidence.commit.round,
      blockId: blockId(evidence.commit.blockId),
      signatures: evidence.commit.signatures.map((signature) => ({
        blockIdFlag: signature.blockIdFlag,
        validatorAddress: hex(signature.validatorAddress),
        timestamp: { seconds: quantity(signature.timestamp.seconds), nanos: signature.timestamp.nanos },
        signature: hex(signature.signature),
      })),
    },
    validatorSet: evidence.validatorSet.map((validator) => ({
      address: hex(validator.address),
      publicKey: hex(validator.publicKey),
      votingPower: quantity(validator.votingPower),
    })),
  };
}

export function decodePaymentCommitmentEvidence(value: unknown): PaymentCommitmentEvidence {
  const object = (input: unknown, fields: readonly string[]): Record<string, unknown> => {
    if (input === null || typeof input !== "object" || Array.isArray(input)) return failure();
    const record = input as Record<string, unknown>;
    if (Object.keys(record).length !== fields.length || fields.some((field) => !Object.hasOwn(record, field))) return failure();
    return record;
  };
  const bytes = (input: unknown, size: number): Uint8Array => {
    if (typeof input !== "string" || !/^0x[0-9a-f]*$/u.test(input) || input.length !== 2 + size * 2) return failure();
    return Uint8Array.from(input.slice(2).match(/../gu) ?? [], (pair) => Number.parseInt(pair, 16));
  };
  const quantity = (input: unknown): bigint => {
    if (typeof input !== "string" || !/^0x(?:0|-?[1-9a-f][0-9a-f]*)$/u.test(input) || input.length > 66) return failure();
    return input.startsWith("0x-") ? -BigInt(`0x${input.slice(3)}`) : BigInt(input);
  };
  const integer = (input: unknown): number => {
    if (typeof input !== "number" || !Number.isSafeInteger(input) || input < 0 || input > 0xffff_ffff) return failure();
    return input;
  };
  const list = (input: unknown): readonly unknown[] => {
    if (!Array.isArray(input) || input.length > 10_000) return failure();
    return input;
  };
  const blockId = (input: unknown, empty = false): PaymentCommitmentEvidence["blockHeader"]["blockId"] => {
    const block = object(input, ["hash", "partSetHeader"]);
    const parts = object(block["partSetHeader"], ["total", "hash"]);
    return { hash: bytes(block["hash"], empty && block["hash"] === "0x" ? 0 : 32), partSetHeader: { total: integer(parts["total"]), hash: bytes(parts["hash"], empty && parts["hash"] === "0x" ? 0 : 32) } };
  };
  const evidence = object(value, ["landingBlockHeight", "blockHeader", "receiptInclusion", "commit", "validatorSet"]);
  const header = object(evidence["blockHeader"], ["chainId", "blockId", "versionBlock", "versionApp", "height", "time", "lastBlockId", "lastCommitHash", "dataHash", "validatorsHash", "nextValidatorsHash", "consensusHash", "appHash", "lastResultsHash", "evidenceHash", "proposerAddress"]);
  const time = object(header["time"], ["seconds", "nanos"]);
  if (typeof header["chainId"] !== "string" || header["chainId"].length === 0 || new TextEncoder().encode(header["chainId"]).length > 50) return failure();
  const inclusion = object(evidence["receiptInclusion"], ["transactionHash", "receiptDigest", "sequence", "batchTransaction", "batchProof", "activityIds"]);
  const proof = object(inclusion["batchProof"], ["total", "index", "aunts"]);
  const optionalHash = (value: unknown): Uint8Array => bytes(value, value === "0x" ? 0 : 32);
  const commit = object(evidence["commit"], ["height", "round", "blockId", "signatures"]);
  return {
    landingBlockHeight: quantity(evidence["landingBlockHeight"]),
    blockHeader: { chainId: header["chainId"], blockId: blockId(header["blockId"]),
      versionBlock: quantity(header["versionBlock"]), versionApp: quantity(header["versionApp"]), height: quantity(header["height"]),
      time: { seconds: quantity(time["seconds"]), nanos: integer(time["nanos"]) }, lastBlockId: blockId(header["lastBlockId"], true),
      lastCommitHash: optionalHash(header["lastCommitHash"]), dataHash: optionalHash(header["dataHash"]),
      validatorsHash: bytes(header["validatorsHash"], 32), nextValidatorsHash: bytes(header["nextValidatorsHash"], 32),
      consensusHash: bytes(header["consensusHash"], 32), appHash: optionalHash(header["appHash"]),
      lastResultsHash: optionalHash(header["lastResultsHash"]), evidenceHash: optionalHash(header["evidenceHash"]),
      proposerAddress: bytes(header["proposerAddress"], 20),
    },
    receiptInclusion: { transactionHash: bytes(inclusion["transactionHash"], 32), receiptDigest: bytes(inclusion["receiptDigest"], 32), sequence: quantity(inclusion["sequence"]),
      batchTransaction: bytes(inclusion["batchTransaction"], 241),
      batchProof: { total: quantity(proof["total"]), index: quantity(proof["index"]), aunts: list(proof["aunts"]).map((aunt) => bytes(aunt, 32)) },
      activityIds: list(inclusion["activityIds"]).map((id) => bytes(id, 32)),
    },
    commit: {
      height: quantity(commit["height"]), round: integer(commit["round"]), blockId: blockId(commit["blockId"]),
      signatures: list(commit["signatures"]).map((input) => {
        const signature = object(input, ["blockIdFlag", "validatorAddress", "timestamp", "signature"]);
        const flag = signature["blockIdFlag"];
        if (flag !== 1 && flag !== 2 && flag !== 3) return failure();
        const timestamp = object(signature["timestamp"], ["seconds", "nanos"]);
        return { blockIdFlag: flag, validatorAddress: bytes(signature["validatorAddress"], flag === 1 ? 0 : 20),
          timestamp: { seconds: quantity(timestamp["seconds"]), nanos: integer(timestamp["nanos"]) },
          signature: bytes(signature["signature"], flag === 1 ? 0 : 64) };
      }),
    },
    validatorSet: list(evidence["validatorSet"]).map((input) => {
      const validator = object(input, ["address", "publicKey", "votingPower"]);
      return { address: bytes(validator["address"], 20), publicKey: bytes(validator["publicKey"], 32), votingPower: quantity(validator["votingPower"]) };
    }),
  };
}

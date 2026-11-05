import {
  PlatformSdkError,
  verifyCommittedReceipt,
  type CommittedReceiptEvidence,
  type CometHeader,
  type NodeCommittedReceiptEvidence,
  type CometProof,
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
  readonly receiptInclusion: NodeCommittedReceiptEvidence["inclusion"] & {
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
    const snapshot = structuredClone(verified);
    evidence = structuredClone(evidence);
    if (typeof snapshot.height !== "bigint" || snapshot.height < 0n) return failure();
    if (!/^eip155:[1-9][0-9]*$/u.test(network) || BigInt(network.slice(7)) > 0xffff_ffffn
      || validators === undefined) return failure();
    const anchor = structuredClone(await validators.resolveValidators(network, evidence.landingBlockHeight));
    if (anchor.chainId !== evidence.blockHeader.chainId || anchor.validatorsHash.length !== 32
      || !equal(anchor.validatorsHash, evidence.blockHeader.validatorsHash)
      || !equal(await cometValidatorSetHash(evidence.validatorSet), anchor.validatorsHash)) return failure();
    const inclusion = evidence.receiptInclusion;
    if (!equal(inclusion.transactionHash, snapshot.receipt.activityId)
      || !equal(inclusion.receiptDigest, snapshot.receiptDigest)
      || inclusion.sequence !== snapshot.receipt.globalSequence) return failure();
    const nodeEvidence: NodeCommittedReceiptEvidence = {
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
  const blockId = (value: CommittedReceiptEvidence["blockId"]) => ({
    hash: hex(value.hash),
    partSetHeader: { total: value.partSetHeader.total, hash: hex(value.partSetHeader.hash) },
  });
  const header = (value: CometHeader) => ({
    chainId: value.chainId,
    versionBlock: quantity(value.versionBlock), versionApp: quantity(value.versionApp),
    height: quantity(value.height), time: { seconds: quantity(value.time.seconds), nanos: value.time.nanos },
    lastBlockId: blockId(value.lastBlockId), lastCommitHash: hex(value.lastCommitHash),
    dataHash: hex(value.dataHash), validatorsHash: hex(value.validatorsHash), nextValidatorsHash: hex(value.nextValidatorsHash),
    consensusHash: hex(value.consensusHash), appHash: hex(value.appHash), lastResultsHash: hex(value.lastResultsHash),
    evidenceHash: hex(value.evidenceHash), proposerAddress: hex(value.proposerAddress),
  });
  const proof = (value: CometProof) => ({ total: quantity(value.total), index: quantity(value.index), aunts: value.aunts.map(hex) });
  const commit = (value: CommittedReceiptEvidence["commit"]) => ({
    height: quantity(value.height), round: value.round, blockId: blockId(value.blockId),
    signatures: value.signatures.map((signature) => ({
      blockIdFlag: signature.blockIdFlag, validatorAddress: hex(signature.validatorAddress),
      timestamp: { seconds: quantity(signature.timestamp.seconds), nanos: signature.timestamp.nanos },
      signature: hex(signature.signature),
    })),
  });
  const validators = (value: CommittedReceiptEvidence["validators"]) => value.map((validator) => ({
    address: hex(validator.address), publicKey: hex(validator.publicKey), votingPower: quantity(validator.votingPower),
  }));
  const inclusion = evidence.receiptInclusion;
  return {
    landingBlockHeight: quantity(evidence.landingBlockHeight),
    blockHeader: { ...header(evidence.blockHeader), blockId: blockId(evidence.blockHeader.blockId) },
    receiptInclusion: {
      transactionHash: hex(inclusion.transactionHash), receiptDigest: hex(inclusion.receiptDigest), sequence: quantity(inclusion.sequence),
      executionHeader: header(inclusion.executionHeader), successorHeader: header(inclusion.successorHeader),
      executionCommit: commit(inclusion.executionCommit), executionValidators: validators(inclusion.executionValidators),
      transaction: hex(inclusion.transaction), transactionProof: proof(inclusion.transactionProof),
      batchTransaction: hex(inclusion.batchTransaction), batchProof: proof(inclusion.batchProof),
      result: { code: inclusion.result.code, data: hex(inclusion.result.data),
        gasWanted: quantity(inclusion.result.gasWanted), gasUsed: quantity(inclusion.result.gasUsed) },
      resultProof: proof(inclusion.resultProof), activityIds: inclusion.activityIds.map(hex),
    },
    commit: commit(evidence.commit), validatorSet: validators(evidence.validatorSet),
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
  const payload = (input: unknown): Uint8Array => {
    if (typeof input !== "string" || input.length > 2 + 4 * 1048576 * 2 || input.length % 2 !== 0) return failure();
    return bytes(input, (input.length - 2) / 2);
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
  const blockId = (input: unknown, empty = false): CommittedReceiptEvidence["blockId"] => {
    const block = object(input, ["hash", "partSetHeader"]);
    const parts = object(block["partSetHeader"], ["total", "hash"]);
    return { hash: bytes(block["hash"], empty && block["hash"] === "0x" ? 0 : 32),
      partSetHeader: { total: integer(parts["total"]), hash: bytes(parts["hash"], empty && parts["hash"] === "0x" ? 0 : 32) } };
  };
  const optionalHash = (input: unknown): Uint8Array => bytes(input, input === "0x" ? 0 : 32);
  const headerFields = ["chainId", "versionBlock", "versionApp", "height", "time", "lastBlockId", "lastCommitHash", "dataHash", "validatorsHash", "nextValidatorsHash", "consensusHash", "appHash", "lastResultsHash", "evidenceHash", "proposerAddress"];
  const header = (input: Record<string, unknown>): CometHeader => {
    const time = object(input["time"], ["seconds", "nanos"]);
    const chainId = input["chainId"];
    if (typeof chainId !== "string" || chainId.length === 0 || new TextEncoder().encode(chainId).length > 50) return failure();
    return {
      chainId, versionBlock: quantity(input["versionBlock"]), versionApp: quantity(input["versionApp"]), height: quantity(input["height"]),
      time: { seconds: quantity(time["seconds"]), nanos: integer(time["nanos"]) }, lastBlockId: blockId(input["lastBlockId"], true),
      lastCommitHash: optionalHash(input["lastCommitHash"]), dataHash: optionalHash(input["dataHash"]),
      validatorsHash: bytes(input["validatorsHash"], 32), nextValidatorsHash: bytes(input["nextValidatorsHash"], 32),
      consensusHash: bytes(input["consensusHash"], 32), appHash: optionalHash(input["appHash"]),
      lastResultsHash: optionalHash(input["lastResultsHash"]), evidenceHash: optionalHash(input["evidenceHash"]),
      proposerAddress: bytes(input["proposerAddress"], 20),
    };
  };
  const proof = (input: unknown): CometProof => {
    const record = object(input, ["total", "index", "aunts"]);
    return { total: quantity(record["total"]), index: quantity(record["index"]), aunts: list(record["aunts"]).map((aunt) => bytes(aunt, 32)) };
  };
  const commit = (input: unknown): CommittedReceiptEvidence["commit"] => {
    const record = object(input, ["height", "round", "blockId", "signatures"]);
    return {
      height: quantity(record["height"]), round: integer(record["round"]), blockId: blockId(record["blockId"]),
      signatures: list(record["signatures"]).map((input) => {
        const signature = object(input, ["blockIdFlag", "validatorAddress", "timestamp", "signature"]);
        const flag = signature["blockIdFlag"];
        if (flag !== 1 && flag !== 2 && flag !== 3) return failure();
        const timestamp = object(signature["timestamp"], ["seconds", "nanos"]);
        return { blockIdFlag: flag, validatorAddress: bytes(signature["validatorAddress"], flag === 1 ? 0 : 20),
          timestamp: { seconds: quantity(timestamp["seconds"]), nanos: integer(timestamp["nanos"]) },
          signature: bytes(signature["signature"], flag === 1 ? 0 : 64) };
      }),
    };
  };
  const validators = (input: unknown): CommittedReceiptEvidence["validators"] => list(input).map((input) => {
    const validator = object(input, ["address", "publicKey", "votingPower"]);
    return { address: bytes(validator["address"], 20), publicKey: bytes(validator["publicKey"], 32), votingPower: quantity(validator["votingPower"]) };
  });
  const evidence = object(value, ["landingBlockHeight", "blockHeader", "receiptInclusion", "commit", "validatorSet"]);
  const blockHeader = object(evidence["blockHeader"], [...headerFields, "blockId"]);
  const inclusion = object(evidence["receiptInclusion"], ["transactionHash", "receiptDigest", "sequence", "executionHeader", "successorHeader", "executionCommit", "executionValidators", "transaction", "transactionProof", "batchTransaction", "batchProof", "result", "resultProof", "activityIds"]);
  const result = object(inclusion["result"], ["code", "data", "gasWanted", "gasUsed"]);
  return {
    landingBlockHeight: quantity(evidence["landingBlockHeight"]),
    blockHeader: { ...header(blockHeader), blockId: blockId(blockHeader["blockId"]) },
    receiptInclusion: {
      transactionHash: bytes(inclusion["transactionHash"], 32), receiptDigest: bytes(inclusion["receiptDigest"], 32), sequence: quantity(inclusion["sequence"]),
      executionHeader: header(object(inclusion["executionHeader"], headerFields)),
      successorHeader: header(object(inclusion["successorHeader"], headerFields)),
      executionCommit: commit(inclusion["executionCommit"]), executionValidators: validators(inclusion["executionValidators"]),
      transaction: payload(inclusion["transaction"]), transactionProof: proof(inclusion["transactionProof"]),
      batchTransaction: bytes(inclusion["batchTransaction"], 241), batchProof: proof(inclusion["batchProof"]),
      result: { code: integer(result["code"]), data: payload(result["data"]), gasWanted: quantity(result["gasWanted"]), gasUsed: quantity(result["gasUsed"]) },
      resultProof: proof(inclusion["resultProof"]), activityIds: list(inclusion["activityIds"]).map((id) => bytes(id, 32)),
    },
    commit: commit(evidence["commit"]), validatorSet: validators(evidence["validatorSet"]),
  };
}

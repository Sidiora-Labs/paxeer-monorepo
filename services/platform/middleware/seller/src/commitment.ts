import {
  PlatformSdkError,
  verifyCommittedReceipt,
  type CommittedReceiptEvidence,
  type SequencerReceiptVerification,
} from "@sidiora/layerx-sdk";

export type PaymentCommitment = "receipted" | "committed";

export interface PaymentCommitmentEvidence {
  readonly landingBlockHeight: CommittedReceiptEvidence["landingHeight"];
  readonly blockHeader: {
    readonly chainId: CommittedReceiptEvidence["chainId"];
    readonly blockId: CommittedReceiptEvidence["blockId"];
  };
  readonly receiptInclusion: {
    readonly transactionHash: Uint8Array;
    readonly receiptDigest: Uint8Array;
    readonly sequence: bigint;
  };
  readonly commit: CommittedReceiptEvidence["commit"];
  readonly validatorSet: CommittedReceiptEvidence["validators"];
}

export interface PaymentCommitmentResolver {
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
  await verifyPaymentCommitmentEvidence(verified, commitment, evidence);
}

export async function verifyPaymentCommitmentEvidence(
  verified: SequencerReceiptVerification,
  commitment: PaymentCommitment,
  evidence: PaymentCommitmentEvidence,
): Promise<void> {
  if (commitment === "receipted") return;
  if (commitment !== "committed") return failure();
  try {
    verified = structuredClone(verified);
    evidence = structuredClone(evidence);
    if (typeof verified.height !== "bigint" || verified.height < 0n) return failure();
    const inclusion = evidence.receiptInclusion;
    if (!equal(inclusion.transactionHash, verified.receipt.activityId)
      || !equal(inclusion.receiptDigest, verified.receiptDigest)
      || inclusion.sequence !== verified.receipt.globalSequence) return failure();
    await verifyCommittedReceipt(verified, {
      chainId: evidence.blockHeader.chainId,
      landingHeight: evidence.landingBlockHeight,
      blockId: evidence.blockHeader.blockId,
      commit: evidence.commit,
      validators: evidence.validatorSet,
    });
  } catch {
    return failure();
  }
}

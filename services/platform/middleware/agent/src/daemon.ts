import { createHash, ECDH } from "node:crypto";
import {
  encodeNativePrepareRequest, decodeNativeApprovalListResult,
  eip1559SigningHash, encodeUnsignedEip1559, recoverEip1559Sender, keccak256,
  PlatformSdkError, type Eip1559Transaction, type NativePrepareRequestV1, type ProductionClient,
} from "@sidiora/layerx-sdk";
import type { AgentSpendRequest, PreparedActivity, Submission, ApprovalHold, PortableReceiptV2 } from "./index.js";

export interface DaemonPrepareRequest {
  readonly activity_type: string;
  readonly actor: string;
  readonly authority: string;
  readonly account_sequence: string;
  readonly not_before: string;
  readonly not_after: string;
  readonly idempotency_key: string;
  readonly fee_limit: string;
  readonly payload: string;
  readonly payload_hash: string;
  readonly capability_id?: string;
}
export type DaemonPreparation = DaemonPrepareRequest | NativePrepareRequestV1;
const U64 = 18446744073709551615n;
const U128 = 340282366920938463463374607431768211455n;
export function native(request: DaemonPreparation): request is NativePrepareRequestV1 {
  return "variant" in request;
}
function fail(): never { throw new PlatformSdkError({ code: "decode-failure", retry: "unknown-outcome" }); }
function object(value: unknown): Readonly<Record<string, unknown>> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) fail();
  return value as Readonly<Record<string, unknown>>;
}
function exact(value: Readonly<Record<string, unknown>>, fields: readonly string[], optional: readonly string[] = []): void {
  if (fields.some((key) => !(key in value)) || Object.keys(value).some((key) => !fields.includes(key) && !optional.includes(key))) fail();
}
function text(value: unknown, max: number): string {
  if (typeof value !== "string" || value.length === 0 || Buffer.byteLength(value) > max || value.includes("\0") || Buffer.from(value).toString("utf8") !== value) fail();
  return value;
}
export function hex(value: unknown, max: number, size?: number): string {
  if (typeof value !== "string" || !/^(?:[0-9a-f]{2})+$/u.test(value) || value.length > max * 2 || size !== undefined && value.length !== size * 2) fail();
  return value;
}
function decimal(value: unknown, max = U64): string {
  if (typeof value !== "string" || !/^(0|[1-9][0-9]*)$/u.test(value) || value.length > 39 || BigInt(value) > max) fail();
  return value;
}
export function digest(bytes: Uint8Array, domain = ""): string {
  return createHash("sha256").update(domain).update(bytes).digest("hex");
}
export function validateDaemonSpend(request: AgentSpendRequest): void {
  const p = request.preparation;
  if (native(p)) {
    encodeNativePrepareRequest(p);
    if (p.purpose.purpose.tenant !== request.tenant) fail();
  } else {
    exact(object(p), ["activity_type", "actor", "authority", "account_sequence", "not_before", "not_after", "idempotency_key", "fee_limit", "payload", "payload_hash"], ["capability_id"]);
    const activity = BigInt(decimal(p.activity_type, 4294967295n));
    if (activity === 0n || (activity & 65535n) === 0n) fail();
    if (p.capability_id !== undefined) hex(p.capability_id, 32, 32);
  }
  text(p.actor, 255); text(p.authority, 524288);
  decimal(p.account_sequence); decimal(p.not_before); decimal(p.not_after); decimal(p.fee_limit, U128);
  if (BigInt(p.not_before) > BigInt(p.not_after)) fail();
  hex(p.idempotency_key, 32, 32); hex(p.payload, 524288); hex(p.payload_hash, 32, 32);
  hex(request.submitIdempotencyKey, 32, 32); senderAddress(request.signerPublicKey);
  if (request.submitIdempotencyKey === p.idempotency_key) fail();
  hex(request.authorityHex, 524288); decimal(request.networkId, 4294967295n);
  decimal(request.approvalCurrentSequence);
  if (request.approvalReleaseRef !== undefined) hex(request.approvalReleaseRef, 32, 32);
  if (request.walletApprovalId !== undefined && (typeof request.walletApprovalId !== "string" || !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u.test(request.walletApprovalId))) fail();
  if (digest(Buffer.from(p.payload, "hex"), "LXP/v1/payload-hash\0") !== p.payload_hash) fail();
}

function senderAddress(publicKey: string): string {
  hex(publicKey, 65);
  if (!(publicKey.length === 66 && /^(02|03)/u.test(publicKey)
    || publicKey.length === 130 && publicKey.startsWith("04"))) fail();
  try {
    const key = ECDH.convertKey(Buffer.from(publicKey, "hex"), "secp256k1", undefined, undefined, "uncompressed");
    if (typeof key === "string") fail();
    return `0x${Buffer.from(keccak256(key.subarray(1))).subarray(12).toString("hex")}`;
  } catch { fail(); }
}
function transaction(fields: Readonly<Record<string, unknown>>): Eip1559Transaction {
  const chainId = BigInt(decimal(fields.chain_id)), nonce = BigInt(decimal(fields.nonce));
  const maxPriorityFeePerGas = BigInt(decimal(fields.max_priority_fee_per_gas, U128));
  const maxFeePerGas = BigInt(decimal(fields.max_fee_per_gas, U128));
  const gasLimit = BigInt(decimal(fields.gas_limit));
  if (chainId === 0n || gasLimit === 0n || maxPriorityFeePerGas > maxFeePerGas) fail();
  const data = fields.data === "" ? "" : hex(fields.data, 1048576);
  return { chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit,
    to: Buffer.from(hex(fields.to, 20, 20), "hex"), data: Buffer.from(data, "hex"), value: 0n, accessList: [] };
}
export function decodeDaemonPrepared(value: unknown, request: AgentSpendRequest, _protocolVersion: number): PreparedActivity {
  const p = request.preparation, result = object(value);
  exact(result, ["preparation_ref", "chain_id", "nonce", "max_priority_fee_per_gas", "max_fee_per_gas", "gas_limit", "to", "data", "signing_hash",
    "activity_type", "actor", "authority", "account_sequence", "not_before", "not_after", "fee_limit", "payload", "payload_hash", "idempotency_key"]);
  const activity = native(p) ? (BigInt(p.activity.module) * 65536n + BigInt(p.activity.ordinal)).toString() : p.activity_type;
  if (result.activity_type !== activity) fail();
  for (const field of ["actor", "authority", "account_sequence", "not_before", "not_after", "fee_limit", "payload", "payload_hash", "idempotency_key"] as const) if (result[field] !== p[field]) fail();
  const tx = transaction(result), signingHash = hex(result.signing_hash, 32, 32);
  if (tx.chainId !== BigInt(decimal(request.networkId)) || Buffer.from(eip1559SigningHash(tx)).toString("hex") !== signingHash) fail();
  return Object.freeze({ preparation_ref: text(result.preparation_ref, 255),
    chain_id: tx.chainId.toString(), nonce: tx.nonce.toString(), max_priority_fee_per_gas: tx.maxPriorityFeePerGas.toString(),
    max_fee_per_gas: tx.maxFeePerGas.toString(), gas_limit: tx.gasLimit.toString(), to: result.to as string, data: result.data as string,
    signing_hash: signingHash, sender: senderAddress(request.signerPublicKey),
    disclosure: Object.freeze({ ...p, canonical_digest: signingHash, network_id: request.networkId, authority_bytes: request.authorityHex }),
    expiry: p.not_after, signer_public_key: request.signerPublicKey,
    ...(request.walletApprovalId === undefined ? {} : { wallet_approval_id: request.walletApprovalId }) });
}

function integerBytes(value: bigint): Buffer {
  if (value === 0n) return Buffer.alloc(0);
  const encoded = value.toString(16);
  return Buffer.from(encoded.padStart(encoded.length + encoded.length % 2, "0"), "hex");
}
function rlp(bytes: Buffer, list = false): Buffer {
  if (!list && bytes.length === 1 && bytes[0]! < 128) return bytes;
  const prefix = list ? 192 : 128;
  if (bytes.length < 56) return Buffer.concat([Buffer.from([prefix + bytes.length]), bytes]);
  const length = integerBytes(BigInt(bytes.length));
  return Buffer.concat([Buffer.from([prefix + 55 + length.length]), length, bytes]);
}
export function signedActivityId(prepared: PreparedActivity, signature: string, publicKey: string): string {
  const bytes = Buffer.from(hex(signature, 65, 65), "hex"), tx = transaction({ ...prepared });
  if (Buffer.from(eip1559SigningHash(tx)).toString("hex") !== prepared.signing_hash) fail();
  const parity = bytes[64]!;
  if (parity !== 0 && parity !== 1) fail();
  const unsigned = Buffer.from(encodeUnsignedEip1559(tx));
  // Retain the SDK's encoded fields and replace the outer RLP list after appending the signature.
  const prefix = unsigned[1]!;
  const payload = unsigned.subarray(prefix < 248 ? 2 : 2 + prefix - 247);
  const raw = Buffer.concat([Buffer.from([2]), rlp(Buffer.concat([payload,
    rlp(integerBytes(BigInt(parity))), rlp(integerBytes(BigInt(`0x${bytes.subarray(0, 32).toString("hex")}`))),
    rlp(integerBytes(BigInt(`0x${bytes.subarray(32, 64).toString("hex")}`))),
  ]), true)]);
  try {
    if (senderAddress(publicKey) !== prepared.sender || recoverEip1559Sender(raw).toLowerCase() !== prepared.sender) throw new Error("sender mismatch");
  } catch { throw new PlatformSdkError({ code: "verification-failure", retry: "never" }); }
  return Buffer.from(keccak256(raw)).toString("hex");
}
export function decodeDaemonSubmission(value: unknown, expectedTxHash?: string, expectedReference?: string): Submission {
  const observation = object(value); exact(observation, ["tx_hash", "submission", "receipt"]);
  const txHash = hex(observation.tx_hash, 32, 32);
  if (txHash === "00".repeat(32) || expectedTxHash !== undefined && txHash !== expectedTxHash) fail();
  const s = object(observation.submission), state = text(s.state, 32);
  const terminal = state === "Executed" ? ["receipt_ref"] : state === "Failed" ? ["result_code"] : [];
  exact(s, ["submission_ref", "state", "verification_level", "evidence", "transitions", ...terminal]);
  if (!["Prepared", "Signed", "Queued", "Submitted", "Acknowledged", "Unknown", "Executed", "Failed", "Expired"].includes(state)) fail();
  const reference = text(s.submission_ref, 255);
  if (expectedReference !== undefined && reference !== expectedReference) fail();
  if (!Array.isArray(s.evidence) || s.evidence.length > 64 || !Array.isArray(s.transitions) || s.transitions.length > 64) fail();
  const level = text(s.verification_level, 64);
  const levels = ["Submitted", "Receipted", "Committed"];
  if (level !== "Submitted" && level !== "Receipted" && level !== "Committed") fail();
  for (const value of s.evidence) { const item = object(value); exact(item, ["kind", "digest"]); text(item.kind, 255); hex(item.digest, 32, 32); }
  for (const value of s.transitions) {
    const item = object(value); exact(item, ["from", "to", "cause", "at"]);
    const states = ["Prepared", "Signed", "Queued", "Submitted", "Acknowledged", "Unknown", "Expired"];
    if (!states.includes(text(item.from, 32)) || !states.includes(text(item.to, 32))) fail();
    text(item.cause, 255); decimal(item.at);
  }
  let portableReceipt: PortableReceiptV2 | undefined;
  if (state === "Executed") {
    text(s.receipt_ref, 255);
    const receipt = object(observation.receipt); exact(receipt, ["signed_receipt", "raw_transaction", "receipt_key", "verification_level"]);
    const receiptLevel = text(receipt.verification_level, 64);
    if (!levels.slice(1).includes(receiptLevel) || receiptLevel !== level) fail();
    const signedReceipt = hex(receipt.signed_receipt, 1048576), rawTransaction = hex(receipt.raw_transaction, 1048576);
    try { recoverEip1559Sender(Buffer.from(rawTransaction, "hex")); } catch { fail(); }
    if (Buffer.from(keccak256(Buffer.from(rawTransaction, "hex"))).toString("hex") !== txHash) fail();
    const key = object(receipt.receipt_key);
    exact(key, ["key_id", "public_key", "first_height", "last_height"]);
    const firstHeight = decimal(key.first_height), lastHeight = decimal(key.last_height);
    if (BigInt(firstHeight) > BigInt(lastHeight)) fail();
    portableReceipt = Object.freeze({ format: "layerx-receipt-proof-v2", verificationLevel: receiptLevel === "Receipted" ? "receipted" : "committed",
      signedReceipt: `0x${signedReceipt}`, rawTransaction: `0x${rawTransaction}`,
      receiptDigest: `0x${Buffer.from(keccak256(Buffer.from(signedReceipt, "hex"))).toString("hex")}`,
      receiptKey: Object.freeze({ keyId: `0x${hex(key.key_id, 32, 32)}`, publicKey: `0x${hex(key.public_key, 32, 32)}`, firstHeight, lastHeight }),
    });
  } else if (observation.receipt !== null) fail();
  if (state === "Failed" && (typeof s.result_code !== "string" || s.result_code.length > 11 || !/^(0|-?[1-9][0-9]*)$/u.test(s.result_code) || BigInt(s.result_code) < -2147483648n || BigInt(s.result_code) > 2147483647n)) fail();
  if (state !== "Executed" && level !== "Submitted") fail();
  return { submission_ref: reference, state, tx_hash: txHash, verification_level: level,
    evidence: s.evidence, transitions: s.transitions,
    ...(state === "Executed" ? { receipt_ref: s.receipt_ref as string } : {}),
    ...(portableReceipt === undefined ? {} : { portableReceipt }) };
}
export async function daemonApprovalHold(client: ProductionClient, request: AgentSpendRequest, prepared: PreparedActivity): Promise<ApprovalHold | undefined> {
  const digest = prepared.preparation_ref;
  if (native(request.preparation)) {
    const result = decodeNativeApprovalListResult(await client.agent("approval.list", { variant: "native_v1" }));
    if (new Set(result.approvals.map((entry) => entry.approval_id)).size !== result.approvals.length) fail();
    const approval = result.approvals.find((entry) => entry.approval_id === digest);
    if (approval === undefined || approval.state !== "Awaiting") return undefined;
    if (approval.activity.module !== request.preparation.activity.module || approval.activity.ordinal !== request.preparation.activity.ordinal) fail();
    return { approvalId: approval.approval_id, state: "Held", canonicalBytesDigest: digest, enforcement: "daemon_enforced" };
  }
  let cursor: string | null = null;
  const seen = new Set<string>();
  const approvalsSeen = new Set<string>();
  for (let page = 0; page < 100; page += 1) {
    const result = object(await client.agent("approval.list", { current_sequence: request.approvalCurrentSequence, cursor, limit: "100" }));
    exact(result, ["approvals", "next_cursor"]);
    if (!Array.isArray(result.approvals) || result.approvals.length > 100) fail();
    for (const value of result.approvals) {
      const a = object(value);
      exact(a, ["approval_id", "canonical_digest", "activity_type", "actor", "authority", "counterparties", "amounts", "asset", "fee_limit", "expiry", "idempotency_key", "canonical_bytes_digest", "hold_reason_code", "hold_reason", "created_at_sequence", "expires_at_sequence", "state"], ["submission_ref"]);
      const approvalId = hex(a.approval_id, 32, 32);
      if (approvalsSeen.has(approvalId)) fail(); approvalsSeen.add(approvalId);
      hex(a.canonical_digest, 32, 32); hex(a.canonical_bytes_digest, 32, 32); decimal(a.activity_type, 65535n);
      text(a.actor, 255); text(a.authority, 255); text(a.asset, 255); text(a.idempotency_key, 255);
      decimal(a.fee_limit, U128); decimal(a.expiry); decimal(a.created_at_sequence); decimal(a.expires_at_sequence);
      text(a.hold_reason_code, 255); text(a.hold_reason, 255);
      if (!Array.isArray(a.counterparties) || a.counterparties.length > 64 || !Array.isArray(a.amounts) || a.amounts.length > 64) fail();
      for (const party of a.counterparties) text(party, 255);
      for (const amount of a.amounts) { const row = object(amount); exact(row, ["counterparty", "amount"]); text(row.counterparty, 255); decimal(row.amount, U128); }
      if (!["AwaitingApproval", "Approved", "Rejected", "Expired", "Defective"].includes(text(a.state, 32))) fail();
      if (a.state === "Approved") hex(a.submission_ref, 32, 32); else if (a.submission_ref !== undefined) fail();
      if (a.canonical_bytes_digest === digest && a.state === "AwaitingApproval") {
        if (a.canonical_digest !== digest || a.actor !== request.preparation.actor || a.authority !== request.preparation.authority
          || a.idempotency_key !== request.preparation.idempotency_key) fail();
        return { approvalId: hex(a.approval_id, 32, 32), state: "Held", canonicalBytesDigest: digest, enforcement: "daemon_enforced" };
      }
    }
    if (result.next_cursor === null) return undefined;
    cursor = hex(result.next_cursor, 32, 32); if (seen.has(cursor)) fail(); seen.add(cursor);
  }
  fail();
}

import { decodeRawEip1559, keccak256, type MerkleProof, type SequencerAuthorization, type SequencerReceiptKey } from "@sidiora/layerx-sdk";
import { MiddlewareError, verifyPaymentReceipt, paymentCommitment, type PaymentCommitment, type PaymentRequirements, type PaymentCommitmentResolver, type PaymentReceiptVerification, type PortableReceiptV2 } from "./index.js";

export function rpcObject(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error("invalid-rpc-object");
  return value as Record<string, unknown>;
}

export function rpcHex(value: unknown, size?: number): Uint8Array {
  if (typeof value !== "string") throw new Error("invalid-rpc-hex");
  const prefixed = value.startsWith("0x");
  let hex = prefixed ? value.slice(2) : value;
  if (hex.length > 2_097_152 || /[^0-9a-f]/u.test(hex)
    || (!prefixed && (hex.length === 0 || hex.length % 2 !== 0))
    || (size !== undefined && hex.length !== size * 2)) throw new Error("invalid-rpc-hex");
  if (hex.length % 2 !== 0) hex = `0${hex}`;
  return Uint8Array.from(hex.match(/../gu) ?? [], (pair) => Number.parseInt(pair, 16));
}

function rawTransactionBytes(value: string): Uint8Array {
  if (!value.startsWith("0x") || value.length % 2 !== 0) throw new Error("invalid-rpc-submit");
  const raw = rpcHex(value);
  if (raw.length > 524_288) throw new Error("invalid-rpc-submit");
  decodeRawEip1559(raw);
  return raw;
}

function transactionHash(value: string): string {
  if (!value.startsWith("0x")) throw new Error("invalid-rpc-transaction-hash");
  rpcHex(value, 32);
  return value;
}

function hexString(value: Uint8Array): string {
  return `0x${Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

export class PaymentRpcError extends Error {
  public constructor(public readonly code: number, public readonly data: unknown) { super(`JSON-RPC error ${code}`); }
}

export class PaymentRpc {
  #id = 0;
  readonly #url: URL;
  public constructor(endpoint: string, private readonly headers: Readonly<Record<string, string>> = {}) {
    this.#url = new URL(endpoint);
    if (this.#url.username || this.#url.password || this.#url.hash || this.#url.search
      || this.#url.pathname !== "/rpc"
      || (this.#url.protocol !== "https:" && !(this.#url.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(this.#url.hostname)))) throw new Error("invalid-rpc-endpoint");
  }
  public async call(method: string, params: readonly unknown[]): Promise<unknown> {
    const id = ++this.#id;
    const response = await fetch(this.#url, { method: "POST", redirect: "error", signal: AbortSignal.timeout(30_000),
      headers: { ...this.headers, "content-type": "application/json" }, body: JSON.stringify({ jsonrpc: "2.0", id, method, params }) });
    if (!response.ok) { await response.body?.cancel(); throw new Error(`rpc-http-${response.status}`); }
    const reader = response.body?.getReader();
    if (!reader) throw new Error("missing-rpc-body");
    const chunks: Uint8Array[] = []; let size = 0;
    for (;;) {
      const part = await reader.read(); if (part.done) break;
      size += part.value.length;
      if (size > 8_388_608) { await reader.cancel(); throw new Error("rpc-body-too-large"); }
      chunks.push(part.value);
    }
    const bytes = new Uint8Array(size); let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
    const body = rpcObject(JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)));
    if (body["jsonrpc"] !== "2.0" || body["id"] !== id || (("error" in body) === ("result" in body))) throw new Error("invalid-rpc-envelope");
    if ("error" in body) {
      const error = rpcObject(body["error"]);
      if (!Number.isInteger(error["code"]) || typeof error["message"] !== "string") throw new Error("invalid-rpc-error");
      throw new PaymentRpcError(error["code"] as number, error["data"]);
    }
    return body["result"];
  }
  public async send(rawTransaction: string, commitment: PaymentCommitment = "receipted"): Promise<string> {
    rawTransactionBytes(rawTransaction);
    if (!["receipted", "committed"].includes(commitment)) throw new Error("invalid-rpc-submit");
    const result = await this.call("eth_sendRawTransaction", [rawTransaction]);
    if (typeof result !== "string" || result.length !== 66) throw new Error("invalid-rpc-transaction-hash");
    return transactionHash(result);
  }
  public async receipt(txHash: string): Promise<Record<string, unknown> | null> {
    const result = await this.call("pax_sequencerReceipt", [transactionHash(txHash)]);
    if (result === null) return null;
    const receipt = rpcObject(result);
    if ((receipt["rung"] !== "submitted" && receipt["rung"] !== "receipted" && receipt["rung"] !== "committed")
      || receipt["txHash"] !== txHash) throw new Error("invalid-rpc-receipt");
    return receipt;
  }
  public async status(txHash: string): Promise<Record<string, unknown> | null> {
    const result = await this.call("eth_getTransactionReceipt", [transactionHash(txHash)]);
    if (result === null) return null;
    const receipt = rpcObject(result);
    if (receipt["transactionHash"] !== txHash) throw new Error("invalid-rpc-receipt");
    if (receipt["blockNumber"] === null || receipt["blockNumber"] === undefined) return null;
    rpcHex(receipt["blockNumber"]);
    return receipt;
  }
}

export async function verifyRpcPayment(
  result: Record<string, unknown>, rawTransaction: string, expectedPayer: string,
  requirements: PaymentRequirements, receiptKeys: readonly SequencerReceiptKey[],
  commitments?: PaymentCommitmentResolver,
): Promise<{ readonly kind: "pending" } | (PortableReceiptV2 & { readonly kind: "verified"; readonly verification: PaymentReceiptVerification })> {
  const expectedHash = hexString(keccak256(rawTransactionBytes(rawTransaction)));
  const payer = rpcHex(expectedPayer, 32);
  if (result["txHash"] !== expectedHash) throw new MiddlewareError("verification-failure");
  if (result["rung"] === "submitted") return { kind: "pending" };
  if (result["rung"] !== "receipted" && result["rung"] !== "committed") throw new MiddlewareError("payment-refused");
  const signedReceipt = rpcHex(result["receipt"]);
  const commitment = paymentCommitment(requirements.extra);
  if (commitment === "committed" && result["rung"] !== "committed") throw new MiddlewareError("verification-failure");
  if (typeof result["height"] !== "string" || !/^0x(?:0|[1-9a-f][0-9a-f]{0,15})$/u.test(result["height"])) {
    throw new MiddlewareError("verification-failure");
  }
  const height = BigInt(result["height"]);
  const key = receiptKeys.find((pin) => hexString(pin.keyId) === result["keyId"]
    && pin.first_height <= height && height <= pin.last_height);
  if (key === undefined) throw new MiddlewareError("verification-failure");
  const evidence: PortableReceiptV2 = {
    format: "layerx-receipt-proof-v2", verificationLevel: result["rung"], rawTransaction,
    signedReceipt: hexString(signedReceipt), receiptDigest: hexString(keccak256(signedReceipt)),
    receiptKey: { keyId: hexString(key.keyId), publicKey: hexString(key.publicKey),
      firstHeight: `0x${key.first_height.toString(16)}`, lastHeight: `0x${key.last_height.toString(16)}` },
  };
  const verification = await verifyPaymentReceipt(evidence, requirements, receiptKeys, commitments);
  if (verification.height !== height || verification.txHash !== expectedHash
    || hexString(verification.receipt.from) !== hexString(payer)) throw new MiddlewareError("verification-failure");
  return { kind: "verified", ...evidence, verification };
}

export interface RpcBatchEvidence {
  readonly networkId: number;
  readonly canonicalHeader: Uint8Array;
  readonly headerSignature: Uint8Array;
  readonly authorization: SequencerAuthorization;
  readonly proof: MerkleProof;
}

export function rpcBatchEvidence(result: Record<string, unknown>, activityId: string, receipt: Uint8Array,
  trusted: Pick<RpcBatchEvidence, "networkId" | "authorization">): RpcBatchEvidence {
  const bundle = rpcObject(result["batch_evidence"]);
  const signed = rpcObject(bundle["signed_header"]);
  const proof = rpcObject(bundle["proof"]);
  const same = (left: Uint8Array, right: Uint8Array) => left.length === right.length && left.every((b, i) => b === right[i]);
  if (bundle["kind"] !== "receipt" || bundle["activity_id"] !== activityId
    || !same(rpcHex(bundle["canonical_value"]), receipt)
    || !same(rpcHex(signed["public_key"], 32), trusted.authorization.publicKey)
    || !same(rpcHex(signed["sequencer_id"], 32), trusted.authorization.sequencerId)
    || !Number.isSafeInteger(proof["leaf_index"]) || !Number.isSafeInteger(proof["leaf_count"])
    || !Array.isArray(proof["siblings"]) || proof["siblings"].length > 64) throw new MiddlewareError("verification-failure");
  return { ...trusted, canonicalHeader: rpcHex(signed["canonical_header"]), headerSignature: rpcHex(signed["signature"], 64),
    proof: { leafIndex: proof["leaf_index"] as number, leafCount: proof["leaf_count"] as number, siblings: proof["siblings"].map(value => rpcHex(value, 32)) } };
}

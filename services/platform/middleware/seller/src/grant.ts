import { decodeRawEip1559, recoverEip1559Sender, type SequencerReceiptKey } from "@sidiora/layerx-sdk";
import { MiddlewareError, verifyPaymentReceipt, type SellerPaymentAuthority, type SellerSettlementRequest, type SellerSettlementOutcome } from "./index.js";
import { LAYERX_LEDGER_ADDRESS, LEDGER_DRAW_SELECTOR } from "./ledger.js";
import { type PaymentCommitmentResolver } from "./commitment.js";

export interface ResolvedPayerGrant {
  readonly source: string;
  readonly asset: string;
  readonly recipient: string;
  readonly executor: string;
  readonly active: boolean;
}

export interface GrantDrawExecution {
  resolveGrant(grantId: string, network: string): Promise<ResolvedPayerGrant | undefined>;
  execute(request: { readonly principal: string; readonly requestDigest: string; readonly rawTransaction: string;
    readonly idempotencyKey: string; readonly requirements: SellerSettlementRequest["requirements"] }): Promise<SellerSettlementOutcome>;
}

export class GrantPaymentAuthority implements SellerPaymentAuthority {
  readonly #keys: readonly SequencerReceiptKey[];
  public constructor(private readonly draws: GrantDrawExecution, private readonly exact: SellerPaymentAuthority,
    receiptKeys: readonly SequencerReceiptKey[], private readonly commitments?: PaymentCommitmentResolver) {
    this.#keys = structuredClone(receiptKeys);
  }
  public async settle(request: SellerSettlementRequest): Promise<SellerSettlementOutcome> {
    if (request.requirements.scheme === "exact") return this.exact.settle(request);
    if (request.requirements.scheme !== "metered" && request.requirements.scheme !== "subscription") throw new MiddlewareError("unsupported-payment");
    const payload = request.payload.payload;
    if (Object.keys(payload).length !== 2 || typeof payload["rawTransaction"] !== "string"
      || typeof payload["idempotencyKey"] !== "string" || !/^[0-9a-f]{64}$/u.test(payload["idempotencyKey"])) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    let data: Uint8Array;
    let executor: string;
    try {
      const raw = grantHex(payload["rawTransaction"]);
      const transaction = decodeRawEip1559(raw);
      executor = recoverEip1559Sender(raw).toLowerCase();
      data = transaction.data;
      const terms = request.requirements.extra as { readonly layerx?: { readonly purposeHash?: string } } | undefined;
      if (grantHexString(transaction.to) !== LAYERX_LEDGER_ADDRESS || transaction.value !== 0n
        || request.requirements.network !== `eip155:${transaction.chainId}`
        || data.length !== 228 || !sameGrantBytes(data.subarray(0, 4), LEDGER_DRAW_SELECTOR)
        || !/^(0|[1-9][0-9]{0,38})$/u.test(request.requirements.amount)
        || grantUint(data.subarray(68, 100)) >= 1n << 128n
        || grantUint(data.subarray(68, 100)) !== BigInt(request.requirements.amount)
        || grantUint(data.subarray(132, 164)) >= 1n << 64n
        || grantHexString(data.subarray(100, 132)).slice(2) !== payload["idempotencyKey"]
        || !sameGrantBytes(data.subarray(36, 68), grantHex(request.requirements.payTo, 32))
        || grantHexString(data.subarray(164, 196)).slice(2) !== terms?.layerx?.purposeHash) throw new Error("invalid-draw");
    } catch {
      throw new MiddlewareError("invalid-payment-payload");
    }
    const grant = await this.draws.resolveGrant(grantHexString(data.subarray(4, 36)), request.requirements.network);
    if (grant === undefined || !grant.active) return { kind: "refused", reason: "grant_unavailable" };
    const terms = request.requirements.extra as { readonly layerx?: { readonly payer?: string } } | undefined;
    if (grant.executor.toLowerCase() !== executor || grant.recipient.toLowerCase() !== request.requirements.payTo.toLowerCase()
      || grant.asset.toLowerCase() !== request.requirements.asset.toLowerCase()
      || grant.source.toLowerCase() !== `0x${terms?.layerx?.payer}`) throw new MiddlewareError("verification-failure");
    const outcome = await this.draws.execute({ principal: request.principal, requestDigest: request.requestDigest,
      rawTransaction: payload["rawTransaction"], idempotencyKey: payload["idempotencyKey"], requirements: request.requirements });
    if (outcome.kind !== "settled") return outcome;
    if (outcome.rawTransaction !== payload["rawTransaction"]) throw new MiddlewareError("verification-failure");
    const verified = await verifyPaymentReceipt(outcome, request.requirements, this.#keys, this.commitments);
    if (grantHexString(verified.payment.from) !== grant.source.toLowerCase()
      || grantHexString(verified.payment.assetId) !== grant.asset.toLowerCase()) throw new MiddlewareError("verification-failure");
    return outcome;
  }
}

function grantHex(value: string, size?: number): Uint8Array {
  if (typeof value !== "string" || value.length > 1_048_578 || !/^0x(?:[0-9a-fA-F]{2})+$/u.test(value)
    || (size !== undefined && value.length !== 2 + size * 2)) throw new Error("invalid-grant-hex");
  return Uint8Array.from(value.slice(2).match(/../gu)!, (pair) => Number.parseInt(pair, 16));
}
function grantHexString(value: Uint8Array): string {
  return `0x${Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}
function sameGrantBytes(left: Uint8Array, right: Uint8Array): boolean {
  return left.length === right.length && left.every((byte, index) => byte === right[index]);
}
function grantUint(word: Uint8Array): bigint { return BigInt(grantHexString(word)); }

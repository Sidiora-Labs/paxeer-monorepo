import { decodeRawEip1559, keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { MiddlewareError, type SellerPaymentAuthority, type SellerSettlementRequest, type SellerSettlementOutcome } from "./index.js";
import { decodePaymentDrawnLog, LAYERX_LEDGER_ADDRESS, LEDGER_DRAW_SELECTOR } from "./ledger.js";

export interface GrantDrawExecution {
  execute(request: { readonly principal: string; readonly requestDigest: string; readonly rawTransaction: string;
    readonly idempotencyKey: string; readonly requirements: SellerSettlementRequest["requirements"] }): Promise<SellerSettlementOutcome>;
}

export class GrantPaymentAuthority implements SellerPaymentAuthority {
  public constructor(private readonly draws: GrantDrawExecution, private readonly exact: SellerPaymentAuthority) {}
  public async settle(request: SellerSettlementRequest): Promise<SellerSettlementOutcome> {
    if (request.requirements.scheme === "exact") return this.exact.settle(request);
    if (request.requirements.scheme !== "metered" && request.requirements.scheme !== "subscription") throw new MiddlewareError("unsupported-payment");
    const payload = request.payload.payload;
    if (Object.keys(payload).length !== 2 || typeof payload["rawTransaction"] !== "string"
      || typeof payload["idempotencyKey"] !== "string" || !/^[0-9a-f]{64}$/u.test(payload["idempotencyKey"])) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    let raw: Uint8Array;
    let data: Uint8Array;
    try {
      raw = grantHex(payload["rawTransaction"]);
      const transaction = decodeRawEip1559(raw);
      data = transaction.data;
      if (grantHexString(transaction.to) !== LAYERX_LEDGER_ADDRESS || transaction.value !== 0n
        || request.requirements.network !== `eip155:${transaction.chainId}`
        || data.length !== 4 + 7 * 32 || !sameGrantBytes(data.subarray(0, 4), LEDGER_DRAW_SELECTOR)
        || !/^(0|[1-9][0-9]{0,38})$/u.test(request.requirements.amount)
        || grantUint(data.subarray(68, 100)) >= 1n << 128n
        || grantUint(data.subarray(68, 100)) !== BigInt(request.requirements.amount)
        || grantUint(data.subarray(132, 164)) >= 1n << 64n
        || grantHexString(data.subarray(164, 196)).slice(2) !== payload["idempotencyKey"]) {
        throw new Error("invalid-draw");
      }
    } catch {
      throw new MiddlewareError("invalid-payment-payload");
    }
    const outcome = await this.draws.execute({ principal: request.principal, requestDigest: request.requestDigest,
      rawTransaction: payload["rawTransaction"], idempotencyKey: payload["idempotencyKey"], requirements: request.requirements });
    if (outcome.kind !== "settled") return outcome;
    try {
      const signedReceipt = grantHex(outcome.signedReceipt);
      if (outcome.rawTransaction !== payload["rawTransaction"] || outcome.format !== "layerx-receipt-proof-v2"
        || (outcome.verificationLevel !== "receipted" && outcome.verificationLevel !== "committed")
        || grantHexString(keccak256(signedReceipt)) !== outcome.receiptDigest) throw new Error("invalid-draw-receipt");
      const verified = await verifySequencerReceipt(signedReceipt, raw, [{
        keyId: grantHex(outcome.receiptKey.keyId, 32), publicKey: grantHex(outcome.receiptKey.publicKey, 32),
        first_height: BigInt(outcome.receiptKey.firstHeight), last_height: BigInt(outcome.receiptKey.lastHeight),
      }]);
      const { receipt } = verified;
      const summary = receipt.effects[0];
      if (receipt.resultCode !== 0 || receipt.moduleId <= 11 || summary === undefined
        || summary.moduleId !== receipt.moduleId || summary.ordinal !== 0 || summary.kind !== 1
        || summary.monetary || summary.eventType !== 0 || summary.body.length < 110
        || summary.body[108] === 0 || summary.body[108]! > 64 || summary.body.length !== 109 + summary.body[108]!
        || new DataView(new Uint8Array(summary.body).buffer).getUint32(104, false) !== receipt.effects.length - 1) {
        throw new Error("invalid-draw-receipt");
      }
      const eventData = new Uint8Array(128);
      eventData.set(data.subarray(36, 100));
      eventData.set(data.subarray(164, 228), 64);
      const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", eventData));
      const topic = keccak256(new TextEncoder().encode("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)"));
      let matches = 0;
      for (const [index, effect] of receipt.effects.slice(1).entries()) {
        const body = effect.body;
        const count = body[20];
        if (effect.moduleId !== receipt.moduleId || effect.ordinal !== index + 1 || effect.kind !== 3
          || effect.monetary || effect.eventType !== 0 || count === undefined || count > 4
          || body.length !== 53 + count * 32) throw new Error("invalid-draw-log");
        const address = grantHexString(body.subarray(0, 20));
        if (address !== LAYERX_LEDGER_ADDRESS || !sameGrantBytes(body.subarray(21, 53), topic)) continue;
        if (!sameGrantBytes(body.subarray(body.length - 32), digest)) throw new Error("invalid-draw-log");
        const payment = decodePaymentDrawnLog({ address, data: eventData,
          topics: Array.from({ length: count }, (_, i) => body.slice(21 + i * 32, 53 + i * 32)) });
        if (!sameGrantBytes(payment.grantId, data.subarray(100, 132))
          || !sameGrantBytes(payment.from, data.subarray(4, 36))
          || !sameGrantBytes(payment.to, grantHex(request.requirements.payTo, 32))
          || !sameGrantBytes(payment.assetId, grantHex(request.requirements.asset, 32))) throw new Error("draw-payment-mismatch");
        matches += 1;
      }
      if (matches !== 1) throw new Error("missing-draw-payment");
    } catch {
      throw new MiddlewareError("verification-failure");
    }
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

function grantUint(word: Uint8Array): bigint {
  return BigInt(grantHexString(word));
}

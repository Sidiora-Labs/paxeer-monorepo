import { keccak256 } from "@sidiora/layerx-sdk";
import { MiddlewareError } from "./index.js";

export const LAYERX_LEDGER_ADDRESS = "0x000000000000000000000000000000000000101a";

const DRAW_SELECTOR = signatureHash("draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)").slice(0, 4);
export const LEDGER_DRAW_SELECTOR = DRAW_SELECTOR.slice();

const PAYMENT_EXACT_TOPIC = signatureHash("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)");
const PAYMENT_DRAWN_TOPIC = signatureHash("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)");
const TRANSFER_TOPIC = signatureHash("Transfer(bytes32,bytes32,bytes32,uint128,uint8)");

export interface LedgerDraw {
  readonly from: Uint8Array;
  readonly assetId: Uint8Array;
  readonly amount: bigint;
  readonly grantId: Uint8Array;
  readonly receiverSequence: bigint;
  readonly idempotencyKey: Uint8Array;
  readonly contextHash: Uint8Array;
}

export interface LedgerLog {
  readonly address: string;
  readonly topics: readonly Uint8Array[];
  readonly data: Uint8Array;
}

export interface PaymentExactEvent {
  readonly paymentReference: Uint8Array;
  readonly from: Uint8Array;
  readonly to: Uint8Array;
  readonly assetId: Uint8Array;
  readonly amount: bigint;
}

export interface PaymentDrawnEvent {
  readonly grantId: Uint8Array;
  readonly from: Uint8Array;
  readonly to: Uint8Array;
  readonly assetId: Uint8Array;
  readonly amount: bigint;
  readonly idempotencyKey: Uint8Array;
  readonly contextHash: Uint8Array;
}

export interface LedgerTransferEvent {
  readonly from: Uint8Array;
  readonly to: Uint8Array;
  readonly assetId: Uint8Array;
  readonly amount: bigint;
  readonly kind: number;
}

export function encodeLedgerDraw(draw: LedgerDraw): Uint8Array {
  if (typeof draw !== "object" || draw === null) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  const words = [
    draw.from,
    draw.assetId,
    encodeUint(draw.amount, 128),
    draw.grantId,
    encodeUint(draw.receiverSequence, 64),
    draw.idempotencyKey,
    draw.contextHash,
  ];
  const result = new Uint8Array(4 + 7 * 32);
  result.set(DRAW_SELECTOR);
  for (const [index, word] of words.entries()) {
    if (!(word instanceof Uint8Array) || word.length !== 32) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    result.set(word, 4 + index * 32);
  }
  return result;
}

export function decodePaymentExactLog(log: LedgerLog): PaymentExactEvent {
  validateLog(log, PAYMENT_EXACT_TOPIC, 64);
  return {
    paymentReference: log.topics[1]!.slice(),
    from: log.topics[2]!.slice(),
    to: log.topics[3]!.slice(),
    assetId: log.data.slice(0, 32),
    amount: decodeUint(log.data.subarray(32, 64), 128),
  };
}

export function decodePaymentDrawnLog(log: LedgerLog): PaymentDrawnEvent {
  validateLog(log, PAYMENT_DRAWN_TOPIC, 128);
  return {
    grantId: log.topics[1]!.slice(),
    from: log.topics[2]!.slice(),
    to: log.topics[3]!.slice(),
    assetId: log.data.slice(0, 32),
    amount: decodeUint(log.data.subarray(32, 64), 128),
    idempotencyKey: log.data.slice(64, 96),
    contextHash: log.data.slice(96, 128),
  };
}

export function decodeLedgerTransferLog(log: LedgerLog): LedgerTransferEvent {
  validateLog(log, TRANSFER_TOPIC, 64);
  return {
    from: log.topics[1]!.slice(),
    to: log.topics[2]!.slice(),
    assetId: log.topics[3]!.slice(),
    amount: decodeUint(log.data.subarray(0, 32), 128),
    kind: Number(decodeUint(log.data.subarray(32, 64), 8)),
  };
}

function signatureHash(signature: string): Uint8Array {
  return keccak256(new TextEncoder().encode(signature));
}

function encodeUint(value: bigint, bits: number): Uint8Array {
  if (typeof value !== "bigint" || value < 0n || value >= 1n << BigInt(bits)) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  const result = new Uint8Array(32);
  for (let index = 31; value > 0n; index -= 1) {
    result[index] = Number(value & 0xffn);
    value >>= 8n;
  }
  return result;
}

function decodeUint(word: Uint8Array, bits: number): bigint {
  let value = 0n;
  for (const byte of word) value = (value << 8n) | BigInt(byte);
  if (value >= 1n << BigInt(bits)) {
    throw new MiddlewareError("verification-failure");
  }
  return value;
}

function validateLog(log: LedgerLog, topic: Uint8Array, dataLength: number): void {
  if (typeof log !== "object" || log === null
    || typeof log.address !== "string" || log.address.toLowerCase() !== LAYERX_LEDGER_ADDRESS
    || !Array.isArray(log.topics) || log.topics.length !== 4
    || !Array.from(log.topics).every((value) => value instanceof Uint8Array && value.length === 32)
    || !topic.every((byte, index) => byte === log.topics[0]![index])
    || !(log.data instanceof Uint8Array) || log.data.length !== dataLength) {
    throw new MiddlewareError("verification-failure");
  }
}

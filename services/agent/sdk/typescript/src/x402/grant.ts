import { keccak256 } from "../evm-transaction.js";

export interface GrantOffer {
  readonly scheme: "metered" | "subscription";
  readonly asset: string;
  readonly amount: string;
  readonly payTo: string;
  readonly extra: { readonly layerx: {
    readonly payer: string;
    readonly grantId: string;
    readonly receiverSequence: string;
    readonly purposeHash: string;
    readonly contextHash: string;
    readonly windowSeconds?: string;
  } };
}

export interface GrantDraw {
  readonly grantId: string;
  readonly recipient: string;
  readonly amount: bigint;
  readonly idempotencyKey: string;
  readonly sequence: bigint;
  readonly purposeHash: string;
  readonly contextHash: string;
}

const DRAW_SELECTOR = keccak256(new TextEncoder().encode(
  "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)",
)).subarray(0, 4);

export function decodeGrantDraw(wire: Uint8Array): GrantDraw {
  if (!(wire instanceof Uint8Array) || wire.length !== 228
    || !DRAW_SELECTOR.every((byte, index) => wire[index] === byte)) throw new Error("invalid-grant-draw");
  const word = (index: number): string => Buffer.from(wire.subarray(4 + index * 32, 36 + index * 32)).toString("hex");
  const amount = BigInt(`0x${word(2)}`);
  const sequence = BigInt(`0x${word(4)}`);
  if (amount === 0n || amount >= 1n << 128n || sequence >= 1n << 64n) throw new Error("invalid-grant-draw");
  return {
    grantId: word(0), recipient: word(1), amount, idempotencyKey: word(3),
    sequence, purposeHash: word(5), contextHash: word(6),
  };
}

export function validateGrantDraw(wire: Uint8Array, offer: GrantOffer, idempotencyKey: string): GrantDraw {
  const draw = decodeGrantDraw(wire);
  const terms = offer.extra.layerx;
  const identifier = (value: string): boolean => typeof value === "string" && /^[0-9a-f]{64}$/u.test(value) && !/^0+$/u.test(value);
  const integer = (value: string, bits: number): boolean => typeof value === "string"
    && /^(0|[1-9][0-9]{0,38})$/u.test(value) && BigInt(value) < 1n << BigInt(bits);
  if (!integer(offer.amount, 128) || BigInt(offer.amount) === 0n
    || !identifier(offer.asset) || !identifier(offer.payTo) || !identifier(terms.payer)
    || !identifier(terms.grantId) || !identifier(terms.purposeHash) || !identifier(terms.contextHash)
    || !identifier(idempotencyKey) || !integer(terms.receiverSequence, 64)
    || draw.grantId !== terms.grantId || draw.recipient !== offer.payTo
    || draw.amount !== BigInt(offer.amount) || draw.idempotencyKey !== idempotencyKey
    || draw.sequence !== BigInt(terms.receiverSequence)
    || draw.purposeHash !== terms.purposeHash || draw.contextHash !== terms.contextHash) throw new Error("invalid-grant-draw");
  if (offer.scheme === "metered") {
    if (terms.windowSeconds !== undefined) throw new Error("invalid-metered-grant");
  } else if (offer.scheme === "subscription") {
    if (terms.windowSeconds === undefined || !integer(terms.windowSeconds, 64) || terms.windowSeconds === "0") throw new Error("invalid-subscription-grant");
  } else throw new Error("unsupported-grant-scheme");
  return draw;
}

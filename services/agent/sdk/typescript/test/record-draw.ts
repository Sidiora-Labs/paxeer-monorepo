import { secp256k1 } from "@noble/curves/secp256k1.js";
import { eip1559SigningHash, type Eip1559Transaction } from "../src/evm-transaction.js";
import type { ExpectedDrawTransaction } from "../src/x402/activity.js";
import { LAYERX_LEDGER_PRECOMPILE } from "../src/x402/receive.js";
const bytes = (value: string): Uint8Array => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const word = (value: bigint): string => value.toString(16).padStart(64, "0");
function calldata(draw: ExpectedDrawTransaction): Uint8Array {
  return bytes("2e89490f" + draw.grantId + draw.recipient + word(draw.amount) + draw.idempotencyKey + word(draw.sequence) + draw.purposeHash + draw.contextHash);
}
type RlpValue = Uint8Array | readonly RlpValue[];

function integer(value: bigint): Uint8Array {
  if (value === 0n) return new Uint8Array();
  const text = value.toString(16);
  return bytes(text.padStart(text.length + text.length % 2, "0"));
}

function rlp(value: RlpValue): Uint8Array {
  const list = !(value instanceof Uint8Array);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0]! < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

export function signedDraw(nonce: bigint, draw: ExpectedDrawTransaction): Uint8Array {
  const transaction: Eip1559Transaction = {
    chainId: draw.chainId, nonce, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 100000n, to: bytes(LAYERX_LEDGER_PRECOMPILE), value: 0n,
    data: calldata(draw), accessList: [],
  };
  const signature = secp256k1.sign(eip1559SigningHash(transaction), new Uint8Array(32).fill(1),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return Uint8Array.from(Buffer.concat([Buffer.of(2), rlp([
    integer(transaction.chainId), integer(transaction.nonce), integer(transaction.maxPriorityFeePerGas),
    integer(transaction.maxFeePerGas), integer(transaction.gasLimit), transaction.to,
    integer(transaction.value), transaction.data, [],
    integer(BigInt(signature[0]!)), integer(parsed.r), integer(parsed.s),
  ])]));
}


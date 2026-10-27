import { decodeRawEip1559, keccak256 } from "../evm-transaction.js";
import { precompileWriteCall } from "../exchange.js";
import { LAYERX_LEDGER_PRECOMPILE } from "./receive.js";

export interface ExpectedDrawTransaction {
  readonly grantId: string;
  readonly recipient: string;
  readonly amount: bigint;
  readonly idempotencyKey: string;
  readonly sequence: bigint;
  readonly purposeHash: string;
  readonly contextHash: string;
  readonly chainId: bigint;
}

export function bindDrawTransaction(raw: Uint8Array, expected: ExpectedDrawTransaction): string {
  const transaction = decodeRawEip1559(raw);
  const identifier = (value: string): string => {
    if (!/^[0-9a-f]{64}$/u.test(value)) throw new Error("invalid-draw-identifier");
    return `0x${value}`;
  };
  const call = precompileWriteCall(
    LAYERX_LEDGER_PRECOMPILE,
    "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)",
    { types: ["bytes32", "bytes32", "uint128", "bytes32", "uint64", "bytes32", "bytes32"], payable: false },
    [identifier(expected.grantId), identifier(expected.recipient), expected.amount,
      identifier(expected.idempotencyKey), expected.sequence, identifier(expected.purposeHash), identifier(expected.contextHash)],
  );
  if (transaction.chainId !== expected.chainId || transaction.value !== 0n
    || `0x${Buffer.from(transaction.to).toString("hex")}` !== call.to
    || `0x${Buffer.from(transaction.data).toString("hex")}` !== call.data) throw new Error("draw-transaction-mismatch");
  return `0x${Buffer.from(keccak256(raw)).toString("hex")}`;
}


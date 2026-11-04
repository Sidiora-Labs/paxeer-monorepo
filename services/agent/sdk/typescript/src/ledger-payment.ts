import { decodeRawEip1559 } from "./evm-transaction.js";

export interface LedgerTransactionCall {
  readonly chainId: bigint;
  readonly to: Uint8Array;
  readonly value: bigint;
  readonly data: Uint8Array;
}

export function decodeLedgerTransaction(raw: Uint8Array): LedgerTransactionCall {
  if (raw.length === 0 || raw.length > 1048576) throw new TypeError("invalid ledger transaction");
  if (raw[0] === 2) return decodeRawEip1559(raw);
  const invalid = (): never => { throw new TypeError("invalid ledger transaction"); };
  function item(offset: number): { list: boolean; bytes: Uint8Array; end: number } {
    const first = raw[offset]; if (first === undefined) return invalid();
    let start = offset + 1, length: number, list = false;
    if (first < 128) { start = offset; length = 1; }
    else if (first <= 183) length = first - 128;
    else if (first <= 191 || first >= 248) {
      list = first >= 248;
      const width = first - (list ? 247 : 183);
      if (width > 4 || raw[start] === 0 || start + width > raw.length) return invalid();
      length = 0;
      for (let i = 0; i < width; i++) length = length * 256 + raw[start++]!;
      if (length < 56) return invalid();
    } else { list = true; length = first - 192; }
    const end = start + length;
    if (end > raw.length || !list && length === 1 && first === 129 && raw[start]! < 128) return invalid();
    return { list, bytes: raw.slice(start, end), end };
  }
  const envelope = item(0);
  if (!envelope.list || envelope.end !== raw.length) return invalid();
  let offset = raw.length - envelope.bytes.length;
  const fields: Uint8Array[] = [];
  while (offset < raw.length) {
    const field = item(offset); if (field.list) return invalid();
    fields.push(field.bytes); offset = field.end;
  }
  if (fields.length !== 9 || fields[3]!.length !== 20) return invalid();
  function integer(b: Uint8Array): bigint {
    if (b.length > 32 || b[0] === 0) return invalid();
    let value = 0n; for (const byte of b) value = value * 256n + BigInt(byte);
    return value;
  }
  for (const i of [0, 1, 2, 4, 6, 7, 8]) integer(fields[i]!);
  const v = integer(fields[6]!);
  if (v < 37n || integer(fields[7]!) === 0n || integer(fields[8]!) === 0n) return invalid();
  return { chainId: (v - 35n) / 2n, to: fields[3]!, value: integer(fields[4]!), data: fields[5]! };
}

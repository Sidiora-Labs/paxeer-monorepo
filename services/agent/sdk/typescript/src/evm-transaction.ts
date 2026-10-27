import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";

import { checksumAddress } from "./account-derivation.js";

export interface Eip1559AccessListEntry {
  readonly address: Uint8Array;
  readonly storageKeys: readonly Uint8Array[];
}

export interface Eip1559Transaction {
  readonly chainId: bigint;
  readonly nonce: bigint;
  readonly maxPriorityFeePerGas: bigint;
  readonly maxFeePerGas: bigint;
  readonly gasLimit: bigint;
  readonly to: Uint8Array;
  readonly value: bigint;
  readonly data: Uint8Array;
  readonly accessList: readonly Eip1559AccessListEntry[];
}

export interface SignedEip1559Transaction extends Eip1559Transaction {
  readonly yParity: 0 | 1;
  readonly r: bigint;
  readonly s: bigint;
}

export function keccak256(bytes: Uint8Array): Uint8Array {
  return keccak_256(bytes);
}

function concat(parts: readonly Uint8Array[]): Uint8Array {
  const bytes = new Uint8Array(parts.reduce((size, part) => size + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    bytes.set(part, offset);
    offset += part.length;
  }
  return bytes;
}

function uintBytes(value: bigint): Uint8Array {
  if (typeof value !== "bigint" || value < 0n || value >= 1n << 256n) {
    throw new RangeError("EIP-1559 integer must be a uint256");
  }
  if (value === 0n) return new Uint8Array();
  const text = value.toString(16);
  const padded = text.padStart(text.length + text.length % 2, "0");
  return Uint8Array.from(padded.match(/../gu)!, (byte) => Number.parseInt(byte, 16));
}

function rlpLength(size: number, prefix: number): Uint8Array {
  if (size < 56) return new Uint8Array([prefix + size]);
  const length = uintBytes(BigInt(size));
  return concat([new Uint8Array([prefix + 55 + length.length]), length]);
}

function rlpBytes(bytes: Uint8Array): Uint8Array {
  if (bytes.length === 1 && bytes[0]! < 0x80) return bytes;
  return concat([rlpLength(bytes.length, 0x80), bytes]);
}

function rlpList(items: readonly Uint8Array[]): Uint8Array {
  const payload = concat(items);
  return concat([rlpLength(payload.length, 0xc0), payload]);
}

function sized(bytes: Uint8Array, length: number): Uint8Array {
  if (bytes.length !== length) throw new TypeError(`Expected ${length} bytes`);
  return bytes;
}

function unsignedFields(transaction: Eip1559Transaction): Uint8Array[] {
  if (transaction.to.length !== 0 && transaction.to.length !== 20) {
    throw new TypeError("EIP-1559 destination must be empty or 20 bytes");
  }
  return [
    ...[
      transaction.chainId,
      transaction.nonce,
      transaction.maxPriorityFeePerGas,
      transaction.maxFeePerGas,
      transaction.gasLimit,
    ].map((value) => rlpBytes(uintBytes(value))),
    rlpBytes(transaction.to),
    rlpBytes(uintBytes(transaction.value)),
    rlpBytes(transaction.data),
    rlpList(transaction.accessList.map((entry) => rlpList([
      rlpBytes(sized(entry.address, 20)),
      rlpList(entry.storageKeys.map((key) => rlpBytes(sized(key, 32)))),
    ]))),
  ];
}

export function encodeUnsignedEip1559(transaction: Eip1559Transaction): Uint8Array {
  return concat([new Uint8Array([2]), rlpList(unsignedFields(transaction))]);
}

export function eip1559SigningHash(transaction: Eip1559Transaction): Uint8Array {
  return keccak256(encodeUnsignedEip1559(transaction));
}

interface RlpCursor {
  readonly bytes: Uint8Array;
  offset: number;
}

interface RlpItem {
  readonly list: boolean;
  readonly bytes: Uint8Array;
}

function takeItem(cursor: RlpCursor): RlpItem {
  const prefix = cursor.bytes[cursor.offset++];
  if (prefix === undefined) throw new TypeError("Truncated RLP");
  if (prefix < 0x80) return { list: false, bytes: new Uint8Array([prefix]) };
  const list = prefix >= 0xc0;
  const short = prefix - (list ? 0xc0 : 0x80);
  let length = short;
  if (short >= 56) {
    const width = short - 55;
    if (cursor.offset + width > cursor.bytes.length || cursor.bytes[cursor.offset] === 0) {
      throw new TypeError("Invalid RLP length");
    }
    length = 0;
    for (let index = 0; index < width; index += 1) {
      length = length * 256 + cursor.bytes[cursor.offset++]!;
    }
    if (!Number.isSafeInteger(length) || length < 56) throw new TypeError("Non-canonical RLP length");
  }
  if (length > cursor.bytes.length - cursor.offset) throw new TypeError("Truncated RLP payload");
  const bytes = cursor.bytes.slice(cursor.offset, cursor.offset + length);
  cursor.offset += length;
  if (!list && length === 1 && bytes[0]! < 0x80) throw new TypeError("Non-canonical RLP string");
  return { list, bytes };
}

function takeBytes(cursor: RlpCursor): Uint8Array {
  const item = takeItem(cursor);
  if (item.list) throw new TypeError("Expected RLP string");
  return item.bytes;
}

function takeList(cursor: RlpCursor): RlpCursor {
  const item = takeItem(cursor);
  if (!item.list) throw new TypeError("Expected RLP list");
  return { bytes: item.bytes, offset: 0 };
}

function exhausted(cursor: RlpCursor): void {
  if (cursor.offset !== cursor.bytes.length) throw new TypeError("Unexpected trailing RLP fields or bytes");
}

function takeUint(cursor: RlpCursor): bigint {
  const bytes = takeBytes(cursor);
  if (bytes.length > 32 || bytes[0] === 0) throw new TypeError("Non-canonical uint256");
  return bytes.reduce((value, byte) => (value << 8n) | BigInt(byte), 0n);
}

function signatureOf(transaction: SignedEip1559Transaction) {
  if (transaction.yParity !== 0 && transaction.yParity !== 1) throw new TypeError("Invalid yParity");
  const signature = new secp256k1.Signature(transaction.r, transaction.s, transaction.yParity);
  if (signature.hasHighS()) throw new TypeError("Non-canonical high-s signature");
  return signature;
}

export function decodeRawEip1559(raw: Uint8Array): SignedEip1559Transaction {
  if (raw[0] !== 2) throw new TypeError("Expected EIP-1559 transaction type 2");
  const envelope = { bytes: raw.slice(1), offset: 0 };
  const fields = takeList(envelope);
  exhausted(envelope);
  const chainId = takeUint(fields);
  const nonce = takeUint(fields);
  const maxPriorityFeePerGas = takeUint(fields);
  const maxFeePerGas = takeUint(fields);
  const gasLimit = takeUint(fields);
  const to = takeBytes(fields);
  if (to.length !== 0 && to.length !== 20) throw new TypeError("Invalid EIP-1559 destination");
  const value = takeUint(fields);
  const data = takeBytes(fields);
  const access = takeList(fields);
  const accessList: Eip1559AccessListEntry[] = [];
  while (access.offset < access.bytes.length) {
    const entry = takeList(access);
    const address = sized(takeBytes(entry), 20);
    const keys = takeList(entry);
    const storageKeys: Uint8Array[] = [];
    while (keys.offset < keys.bytes.length) storageKeys.push(sized(takeBytes(keys), 32));
    exhausted(entry);
    accessList.push({ address, storageKeys });
  }
  const parity = takeUint(fields);
  if (parity !== 0n && parity !== 1n) throw new TypeError("Invalid yParity");
  const r = takeUint(fields);
  const s = takeUint(fields);
  exhausted(fields);
  const transaction: SignedEip1559Transaction = {
    chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList,
    yParity: parity === 0n ? 0 : 1, r, s,
  };
  signatureOf(transaction);
  return transaction;
}

export function recoverEip1559Sender(raw: Uint8Array): string {
  const transaction = decodeRawEip1559(raw);
  const publicKey = signatureOf(transaction).recoverPublicKey(eip1559SigningHash(transaction)).toBytes(false);
  return checksumAddress(keccak256(publicKey.slice(1)).slice(12));
}

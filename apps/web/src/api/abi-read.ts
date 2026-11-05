import { PrecompileAbiError } from "./sdk.ts";

export type AbiReadUint = "uint8" | "uint32" | "uint64" | "uint256";

export type AbiReadScalar = "address" | "bool" | "bytes" | "bytes32" | "string" | AbiReadUint;

export type AbiReadType =
  | AbiReadScalar
  | Readonly<{ tuple: readonly AbiReadField[] }>
  | Readonly<{ array: AbiReadType }>
  | Readonly<{ fixed: AbiReadType; length: number }>;

export type AbiReadField = Readonly<{ name: string; type: AbiReadType }>;

export type AbiReadValue = bigint | boolean | string | readonly AbiReadValue[] | AbiReadRecord;

export interface AbiReadRecord {
  readonly [name: string]: AbiReadValue;
}

const WORD = 32;
export const ABI_DECODE_WORK_LIMIT = 16_384;
const ABI_DECODE_DEPTH_LIMIT = 64;

interface DecodeWork { remaining: number }
interface DecodedValue { value: AbiReadValue; end: number }

function spend(work: DecodeWork, count: number): void {
  if (!Number.isSafeInteger(count) || count < 0 || count > work.remaining) {
    throw new PrecompileAbiError("data_length", "ABI decoding work bound");
  }
  work.remaining -= count;
}

function extent(data: Uint8Array, start: number, count: number, size: number): number {
  if (!Number.isSafeInteger(start) || start < 0 || start > data.length ||
      !Number.isSafeInteger(count) || count < 0 || !Number.isSafeInteger(size) || size < 0 ||
      (size > 0 && count > Math.floor((data.length - start) / size))) {
    throw new PrecompileAbiError("data_length", "ABI head extent");
  }
  return start + count * size;
}
const HEX = /^0x(?:[0-9a-fA-F]{2})*$/u;

function bytesOf(value: string): Uint8Array {
  if (!HEX.test(value)) {
    throw new PrecompileAbiError("invalid_value", "return data");
  }
  const body = value.slice(2);
  return Uint8Array.from({ length: body.length / 2 }, (_, index) =>
    Number.parseInt(body.slice(index * 2, index * 2 + 2), 16),
  );
}

function hexOf(bytes: Uint8Array): string {
  return `0x${Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

function word(data: Uint8Array, position: number): Uint8Array {
  if (position < 0 || position + WORD > data.length) {
    throw new PrecompileAbiError("data_length", String(data.length));
  }
  return data.subarray(position, position + WORD);
}

function integer(bytes: Uint8Array): bigint {
  return bytes.reduce((value, byte) => (value << 8n) | BigInt(byte), 0n);
}

function offset(data: Uint8Array, position: number): number {
  const value = integer(word(data, position));
  if (value > BigInt(data.length)) {
    throw new PrecompileAbiError("data_length", String(data.length));
  }
  return Number(value);
}

function scalarBits(type: AbiReadUint): number {
  switch (type) {
    case "uint8":
      return 8;
    case "uint32":
      return 32;
    case "uint64":
      return 64;
    case "uint256":
      return 256;
  }
}

function dynamic(type: AbiReadType): boolean {
  if (typeof type === "string") {
    return type === "string" || type === "bytes";
  }
  if ("tuple" in type) {
    return type.tuple.some((field) => dynamic(field.type));
  }
  if ("array" in type) {
    return true;
  }
  return dynamic(type.fixed);
}

function headSize(type: AbiReadType): number {
  if (dynamic(type) || typeof type === "string") {
    return WORD;
  }
  if ("tuple" in type) {
    return type.tuple.reduce((total, field) => total + headSize(field.type), 0);
  }
  if ("fixed" in type) {
    return type.length * headSize(type.fixed);
  }
  return WORD;
}

function scalar(data: Uint8Array, position: number, type: AbiReadScalar): AbiReadValue {
  const bytes = word(data, position);
  switch (type) {
    case "address":
      if (bytes.subarray(0, 12).some((byte) => byte !== 0)) {
        throw new PrecompileAbiError("non_canonical_word", type);
      }
      return hexOf(bytes.subarray(12));
    case "bytes32":
      return hexOf(bytes);
    case "bool": {
      const value = integer(bytes);
      if (value > 1n) {
        throw new PrecompileAbiError("non_canonical_word", type);
      }
      return value === 1n;
    }
    case "string":
    case "bytes":
      throw new PrecompileAbiError("invalid_value", type);
    case "uint8":
    case "uint32":
    case "uint64":
    case "uint256": {
      const value = integer(bytes);
      if (value >= 1n << BigInt(scalarBits(type))) {
        throw new PrecompileAbiError("non_canonical_word", type);
      }
      return value;
    }
  }
}

function sequence(
  data: Uint8Array, base: number, types: readonly AbiReadType[], work: DecodeWork, depth: number,
): { values: AbiReadValue[]; end: number } {
  if (depth > ABI_DECODE_DEPTH_LIMIT) {
    throw new PrecompileAbiError("data_length", "ABI decoding depth bound");
  }
  spend(work, types.length);
  let end = base;
  for (const type of types) end = extent(data, end, 1, headSize(type));
  const values: AbiReadValue[] = [];
  let head = base;
  for (const type of types) {
    if (dynamic(type)) {
      const relative = offset(data, head);
      if (relative % WORD !== 0 || relative !== end - base) {
        throw new PrecompileAbiError("non_canonical_word", "ABI offset");
      }
      const decoded = tail(data, end, type, work, depth + 1);
      values.push(decoded.value);
      end = decoded.end;
      head += WORD;
    } else {
      values.push(inline(data, head, type, work, depth + 1));
      head += headSize(type);
    }
  }
  return { values, end };
}

function repeated(
  data: Uint8Array, base: number, length: number, type: AbiReadType, work: DecodeWork, depth: number,
): { values: AbiReadValue[]; end: number } {
  extent(data, base, length, headSize(type));
  if (length > work.remaining) {
    throw new PrecompileAbiError("data_length", "ABI decoding work bound");
  }
  return sequence(data, base, Array.from({ length }, () => type), work, depth);
}

function record(fields: readonly AbiReadField[], values: readonly AbiReadValue[]): AbiReadRecord {
  const out: Record<string, AbiReadValue> = {};
  fields.forEach((field, index) => {
    const value = values[index];
    if (value !== undefined) {
      out[field.name] = value;
    }
  });
  return out;
}

function inline(data: Uint8Array, position: number, type: AbiReadType, work: DecodeWork, depth: number): AbiReadValue {
  if (typeof type === "string") return scalar(data, position, type);
  if ("tuple" in type) {
    return record(type.tuple, sequence(data, position, type.tuple.map((field) => field.type), work, depth).values);
  }
  if ("fixed" in type) return repeated(data, position, type.length, type.fixed, work, depth).values;
  throw new PrecompileAbiError("invalid_value", "array");
}

function tail(data: Uint8Array, position: number, type: AbiReadType, work: DecodeWork, depth: number): DecodedValue {
  if (typeof type === "string") {
    const length = offset(data, position);
    const start = position + WORD;
    const end = extent(data, start, Math.ceil(length / WORD), WORD);
    if (data.subarray(start + length, end).some((byte) => byte !== 0)) {
      throw new PrecompileAbiError("non_canonical_word", "ABI padding");
    }
    const bytes = data.subarray(start, start + length);
    if (type === "bytes") return { value: hexOf(bytes), end };
    try {
      return { value: new TextDecoder("utf-8", { fatal: true }).decode(bytes), end };
    } catch {
      throw new PrecompileAbiError("invalid_string");
    }
  }
  if ("tuple" in type) {
    const decoded = sequence(data, position, type.tuple.map((field) => field.type), work, depth);
    return { value: record(type.tuple, decoded.values), end: decoded.end };
  }
  const decoded = "array" in type
    ? repeated(data, position + WORD, offset(data, position), type.array, work, depth)
    : repeated(data, position, type.length, type.fixed, work, depth);
  return { value: decoded.values, end: decoded.end };
}

/** Decodes canonical precompile return data with a shared allocation and traversal budget. */
export function decodeAbiResult(returnData: string, outputs: readonly AbiReadField[]): AbiReadRecord {
  const data = bytesOf(returnData);
  const decoded = sequence(data, 0, outputs.map((field) => field.type), { remaining: ABI_DECODE_WORK_LIMIT }, 0);
  if (decoded.end !== data.length) throw new PrecompileAbiError("data_length", "ABI trailing data");
  return record(outputs, decoded.values);
}

export function readUint(value: AbiReadValue | undefined, label: string): bigint {
  if (typeof value !== "bigint") {
    throw new PrecompileAbiError("invalid_value", label);
  }
  return value;
}

export function readText(value: AbiReadValue | undefined, label: string): string {
  if (typeof value !== "string") {
    throw new PrecompileAbiError("invalid_value", label);
  }
  return value;
}

export function readFlag(value: AbiReadValue | undefined, label: string): boolean {
  if (typeof value !== "boolean") {
    throw new PrecompileAbiError("invalid_value", label);
  }
  return value;
}

export function readList(value: AbiReadValue | undefined, label: string): readonly AbiReadValue[] {
  if (!Array.isArray(value)) {
    throw new PrecompileAbiError("invalid_value", label);
  }
  return value as readonly AbiReadValue[];
}

export function readRecord(value: AbiReadValue | undefined, label: string): AbiReadRecord {
  if (value === undefined || typeof value !== "object" || Array.isArray(value)) {
    throw new PrecompileAbiError("invalid_value", label);
  }
  return value as AbiReadRecord;
}

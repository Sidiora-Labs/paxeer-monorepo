export interface NativeAsset {
  readonly id: string;
  readonly denom: string;
  readonly symbol: string | null;
  readonly decimals: number | null;
}

export interface NativeActivity {
  readonly kind: "transaction";
  readonly hash: string;
  readonly block_number: number;
  readonly ordinal: number;
  readonly status: "committed";
  readonly result: "ok" | "error" | null;
  readonly timestamp: string;
  readonly asset: NativeAsset;
  readonly amount: string;
  readonly counterparty: string | null;
}

export interface NativeUnifiedAccount {
  readonly identities: Readonly<{ evm: string; pax: string | null; did: string | null }>;
  readonly balances: readonly Readonly<{ asset: NativeAsset; total: string }>[];
  readonly activity: readonly NativeActivity[];
  readonly next_page_params: Readonly<{ cursor: string }> | null;
  readonly page_cursor: string;
  readonly first_page_cursor: string;
  readonly page_number: number;
  readonly activity_total: number | null;
}

export const NATIVE_CURSOR_REFUSAL_REASONS = [
  "expired",
  "malformed",
  "malformed, wrong account, changed binding or reorganized snapshot",
  "use the returned cursor instead of a partial activity key",
  "invalid_unix_time",
] as const;

export type NativeCursorRefusalReason = typeof NATIVE_CURSOR_REFUSAL_REASONS[number];

function object(value: unknown, at: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(`${at} must be an object`);
  }
  return value as Record<string, unknown>;
}

function keys(value: Record<string, unknown>, expected: readonly string[], at: string): void {
  if (Object.keys(value).sort().join(",") !== [...expected].sort().join(",")) {
    throw new TypeError(`${at} has undeclared or missing fields`);
  }
}

function string(value: unknown, at: string): string {
  if (typeof value !== "string" || value.length === 0) throw new TypeError(`${at} must be a non-empty string`);
  return value;
}

function integer(value: unknown, at: string, minimum = 0): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < minimum) {
    throw new TypeError(`${at} must be a safe integer >= ${String(minimum)}`);
  }
  return value;
}

export function nativeAddress(value: unknown, at = "address"): string {
  const address = string(value, at);
  if (!/^0x[0-9a-fA-F]{40}$/u.test(address)) throw new TypeError(`${at} must be an EVM address`);
  return address;
}

function hash(value: unknown, at: string): string {
  const result = string(value, at);
  if (!/^0x[0-9a-fA-F]{64}$/u.test(result)) throw new TypeError(`${at} must be a full hash`);
  return result;
}

function amount(value: unknown, at: string): string {
  const result = string(value, at);
  // Decimal.to_string/1 can emit scientific notation. Keep the exact amount encoding.
  if (!/^(?:0|[1-9]\d*)(?:\.\d+)?(?:E[+-]?\d+)?$/u.test(result)) {
    throw new TypeError(`${at} must be a non-negative decimal string`);
  }
  return result;
}

export function nativeCursor(value: unknown, at = "cursor"): string {
  const result = string(value, at);
  if (new TextEncoder().encode(result).length > 4096) throw new TypeError(`${at} exceeds 4096 bytes`);
  return result;
}

function asset(value: unknown, at: string): NativeAsset {
  const item = object(value, at);
  keys(item, ["id", "denom", "symbol", "decimals"], at);
  const id = string(item.id, `${at}.id`);
  if (id !== "native") nativeAddress(id, `${at}.id`);
  return Object.freeze({
    id,
    denom: string(item.denom, `${at}.denom`),
    symbol: item.symbol === null ? null : string(item.symbol, `${at}.symbol`),
    decimals: item.decimals === null ? null : integer(item.decimals, `${at}.decimals`),
  });
}

function timestamp(value: unknown, at: string): string {
  const result = string(value, at);
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u.test(result)
    || !Number.isFinite(Date.parse(result))) throw new TypeError(`${at} must be an ISO timestamp`);
  return result;
}

function activity(value: unknown, at: string): NativeActivity {
  const item = object(value, at);
  keys(item, ["kind", "hash", "block_number", "ordinal", "status", "result", "timestamp", "asset", "amount", "counterparty"], at);
  if (item.kind !== "transaction" || item.status !== "committed"
    || (item.result !== null && item.result !== "ok" && item.result !== "error")) {
    throw new TypeError(`${at} must describe a committed native transaction`);
  }
  return Object.freeze({
    kind: item.kind,
    hash: hash(item.hash, `${at}.hash`),
    block_number: integer(item.block_number, `${at}.block_number`),
    ordinal: integer(item.ordinal, `${at}.ordinal`),
    status: item.status,
    result: item.result,
    timestamp: timestamp(item.timestamp, `${at}.timestamp`),
    asset: asset(item.asset, `${at}.asset`),
    amount: amount(item.amount, `${at}.amount`),
    counterparty: item.counterparty === null ? null : nativeAddress(item.counterparty, `${at}.counterparty`),
  });
}

export function decodeNativeUnifiedAccount(value: unknown): NativeUnifiedAccount {
  const item = object(value, "native_account");
  keys(item, ["identities", "balances", "activity", "next_page_params", "page_cursor", "first_page_cursor", "page_number", "activity_total"], "native_account");
  const identities = object(item.identities, "identities");
  keys(identities, ["evm", "pax", "did"], "identities");
  if (!Array.isArray(item.balances) || !Array.isArray(item.activity) || item.activity.length > 50) {
    throw new TypeError("Native balances must be an array and activity must contain at most 50 rows");
  }
  let next: Readonly<{ cursor: string }> | null = null;
  if (item.next_page_params !== null) {
    const params = object(item.next_page_params, "next_page_params");
    keys(params, ["cursor"], "next_page_params");
    next = Object.freeze({ cursor: nativeCursor(params.cursor) });
  }
  const rows = item.activity.map((entry, index) => activity(entry, `activity[${String(index)}]`));
  for (let index = 1; index < rows.length; index += 1) {
    const previous = rows[index - 1];
    const current = rows[index];
    if (previous === undefined || current === undefined) throw new TypeError("Native activity row is missing");
    if (current.block_number > previous.block_number
      || (current.block_number === previous.block_number && current.ordinal >= previous.ordinal)) {
      throw new TypeError("Native activity must be strictly descending by block and ordinal");
    }
  }
  return Object.freeze({
    identities: Object.freeze({
      evm: nativeAddress(identities.evm, "identities.evm"),
      pax: identities.pax === null ? null : string(identities.pax, "identities.pax"),
      did: identities.did === null ? null : string(identities.did, "identities.did"),
    }),
    balances: Object.freeze(item.balances.map((entry, index) => {
      const balance = object(entry, `balances[${String(index)}]`);
      keys(balance, ["asset", "total"], `balances[${String(index)}]`);
      return Object.freeze({ asset: asset(balance.asset, `balances[${String(index)}].asset`), total: amount(balance.total, `balances[${String(index)}].total`) });
    })),
    activity: Object.freeze(rows),
    next_page_params: next,
    page_cursor: nativeCursor(item.page_cursor, "page_cursor"),
    first_page_cursor: nativeCursor(item.first_page_cursor, "first_page_cursor"),
    page_number: integer(item.page_number, "page_number", 1),
    activity_total: item.activity_total === null ? null : integer(item.activity_total, "activity_total"),
  });
}

export function decodeNativeCursorRefusal(value: unknown): NativeCursorRefusalReason {
  const item = object(value, "cursor_refusal");
  keys(item, ["message", "reason"], "cursor_refusal");
  if (item.message !== "Invalid activity cursor") throw new TypeError("Not a native cursor refusal");
  for (const reason of NATIVE_CURSOR_REFUSAL_REASONS) if (item.reason === reason) return reason;
  throw new TypeError("Undeclared native cursor refusal reason");
}

export interface NativeArchiveReceipt {
  readonly id: string;
  readonly account: string | null;
  readonly block_number: number | null;
  readonly origin: "kernel" | "evm";
  readonly verification: string;
  readonly verification_status: string;
  readonly payload_hash: string | null;
  readonly transaction_hash: string | null;
  readonly timestamp: string | null;
  readonly frozen: true;
  readonly provenance: Readonly<Record<string, unknown>>;
}

export function decodeNativeArchiveReceipt(value: unknown): NativeArchiveReceipt {
  const item = object(value, "archive_receipt");
  keys(item, ["id", "account", "block_number", "origin", "verification", "verification_status", "payload_hash", "transaction_hash", "timestamp", "frozen", "provenance"], "archive_receipt");
  if (item.frozen !== true || (item.origin !== "kernel" && item.origin !== "evm")) {
    throw new TypeError("Not a frozen native archive receipt");
  }
  const provenance = object(item.provenance, "provenance");
  keys(provenance, [item.origin], "provenance");
  const source = object(provenance[item.origin], `provenance.${item.origin}`);
  const verification = string(item.verification, "verification");
  const status = string(item.verification_status, "verification_status");
  const levels = ["unverified", "sequencer_signed", "batch_included", "state_proven", "checkpoint_finalised"];
  if (![...levels, "sequencer_verified"].includes(verification) || !levels.includes(status)) {
    throw new TypeError("Undeclared archive verification");
  }
  if (item.origin === "evm") {
    keys(source, ["transaction_hash", "log_index", "block_hash", "block_number"], "provenance.evm");
    hash(source.transaction_hash, "provenance.evm.transaction_hash");
    hash(source.block_hash, "provenance.evm.block_hash");
    integer(source.log_index, "provenance.evm.log_index");
    integer(source.block_number, "provenance.evm.block_number");
  } else {
    keys(source, ["batch_number", "batch_id", "sequence", "activity_id", "result_code", "receipt_sha256", "canonical_sha256", "batch_raw_sha256", "state_root", "proof"], "provenance.kernel");
    for (const field of ["batch_number", "sequence"]) {
      if (!/^(?:0|[1-9]\d*)$/u.test(string(source[field], field))) throw new TypeError(`Invalid archive ${field}`);
    }
    for (const field of ["batch_id", "activity_id", "receipt_sha256", "canonical_sha256", "batch_raw_sha256", "state_root"]) hash(source[field], field);
    if (typeof source.result_code !== "number" || !Number.isSafeInteger(source.result_code)) throw new TypeError("Invalid archive result_code");
    const proof = object(source.proof, "proof");
    keys(proof, ["header_hex", "signature_hex"], "proof");
    if (!/^(?:[0-9a-f]{2})+$/u.test(string(proof.header_hex, "header_hex"))
      || !/^[0-9a-f]{128}$/u.test(string(proof.signature_hex, "signature_hex"))) throw new TypeError("Invalid archive proof");
  }
  return Object.freeze({
    id: hash(item.id, "id"),
    account: item.account === null ? null : hash(item.account, "account"),
    block_number: item.block_number === null ? null : integer(item.block_number, "block_number"),
    origin: item.origin,
    verification,
    verification_status: status,
    payload_hash: item.payload_hash === null ? null : hash(item.payload_hash, "payload_hash"),
    transaction_hash: item.transaction_hash === null ? null : hash(item.transaction_hash, "transaction_hash"),
    timestamp: item.timestamp === null ? null : timestamp(item.timestamp, "timestamp"),
    frozen: true,
    provenance: Object.freeze({ [item.origin]: Object.freeze({
      ...source,
      ...(item.origin === "kernel" ? { proof: Object.freeze({ ...object(source.proof, "proof") }) } : {}),
    }) }),
  });
}

/** @deprecated Frozen compatibility exports; current consumers use the native contracts below. */
export {
  type ExplorerVerificationLevel,
  type ExplorerFreshness,
  type CheckpointRecord,
  type BatchRecord,
  type ReceiptRecord,
  type AccountActivityRecord,
  type ExplorerPage,
  type ExplorerRecord,
  type EvidenceVerificationReport,
  type MirrorVerificationProvenance,
  type UnifiedAccountEvidence,
  type PaxeerActivityEvent,
  type UnifiedIdentities,
  type UnifiedBalanceRecord,
  type UnifiedSettlement,
  type PaxeerActivityRecord,
  type PaxeerActivityWindow,
  type UnifiedAccountRecord,
  type ReportedAvailability,
  type UnifiedAccountAvailabilityRecord,
  decodeFreshness,
  decodeCheckpoint,
  decodeBatch,
  decodeReceipt,
  decodeAccountActivity,
  decodePage,
  decodeRecord,
  decodeVerificationReport,
  decodeMirrorProvenance,
  encodeFreshness,
  encodeVerificationReport,
  decodeUnifiedAccount,
  decodeUnifiedAccountUnavailable,
  decodeUnifiedAccountAvailability,
} from "./historical-model.ts";
export * as historicalExplorer from "./historical-model.ts";

export interface NativeTransactionStatus {
  readonly rung: "submitted" | "receipted" | "committed";
  readonly blockNumber: number | null;
  readonly committedHeight: number | null;
}

export type NativeExplorerFreshness =
  | Readonly<{ state: "unavailable"; committedHeight: number | null; indexedHeight: number | null }>
  | Readonly<{ state: "current" | "stale"; committedHeight: number; indexedHeight: number }>;

function nativeHeight(value: unknown, at: string): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0) {
    throw new TypeError(`${at} must be a non-negative safe integer`);
  }
  return value;
}

export function decodeNativeTransactionStatus(value: unknown): NativeTransactionStatus {
  const item = record(value, "transaction status");
  if (Object.keys(item).sort().join(",") !== "block_number,committed_height,rung"
    || (item.rung !== "submitted" && item.rung !== "receipted" && item.rung !== "committed")) {
    throw new TypeError("Invalid native transaction status");
  }
  const blockNumber = item.block_number === null ? null : nativeHeight(item.block_number, "block_number");
  const committedHeight = item.committed_height === null ? null : nativeHeight(item.committed_height, "committed_height");
  if (item.rung === "committed" && (committedHeight === null
    || (blockNumber !== null && blockNumber > committedHeight))) {
    throw new TypeError("Committed transaction lacks a covering committed height");
  }
  return Object.freeze({ rung: item.rung, blockNumber, committedHeight });
}

export function decodeNativeIndexedHeight(value: unknown): number | null {
  const page = record(value, "blocks");
  if (!Array.isArray(page.items)) throw new TypeError("Native blocks must carry items");
  let highest: number | null = null;
  for (const block of page.items) {
    const height = nativeHeight(record(block, "block").height, "block.height");
    highest = highest === null ? height : Math.max(highest, height);
  }
  return highest;
}

export function nativeFreshnessFromPositions(committedHeight: number | null, indexedHeight: number | null): NativeExplorerFreshness {
  if (committedHeight !== null) nativeHeight(committedHeight, "committedHeight");
  if (indexedHeight !== null) nativeHeight(indexedHeight, "indexedHeight");
  if (committedHeight === null || indexedHeight === null) {
    return Object.freeze({ state: "unavailable", committedHeight, indexedHeight });
  }
  return Object.freeze({ state: indexedHeight < committedHeight ? "stale" : "current", committedHeight, indexedHeight });
}

export function encodeNativeTransactionStatus(status: NativeTransactionStatus): Readonly<Record<string, unknown>> {
  const encoded = { rung: status.rung, block_number: status.blockNumber, committed_height: status.committedHeight };
  decodeNativeTransactionStatus(encoded);
  return Object.freeze(encoded);
}

export type ProgramLifecycle = "active" | "deprecated" | "tombstoned";

export type ProgramUpgradePolicy =
  | Readonly<{ kind: "immutable" }>
  | Readonly<{ kind: "upgradeable"; authority: string }>;

export type ProgramSourceStatus =
  | Readonly<{ status: "unpublished" }>
  | Readonly<{ status: "verified"; sourceDigest: string; environmentDigest: string }>
  | Readonly<{ status: "mismatch"; expected: string; reproduced: string }>;

export interface ProgramVersionRecord {
  readonly version: string;
  readonly codeHash: string;
  readonly abiVersion: string;
  readonly interfaceDigest?: string;
  readonly source: ProgramSourceStatus;
}

export interface ProgramValueAccountRecord {
  readonly account: string;
  readonly asset: string;
  readonly balance: string;
  readonly frozen: boolean;
}

export interface ProgramRecord {
  readonly program: string;
  readonly upgradePolicy: ProgramUpgradePolicy;
  readonly lifecycle: ProgramLifecycle;
  readonly versions: readonly ProgramVersionRecord[];
  readonly valueAccounts: readonly ProgramValueAccountRecord[];
  readonly observedSequence: string;
  readonly observedAt: string;
  readonly receiptDigest: string;
  readonly stateRoot: string;
}

export interface NameResolutionRecord {
  readonly name: string;
  readonly did: string;
  readonly expiry: string;
}

type JsonRecord = Readonly<Record<string, unknown>>;

function record(value: unknown, at: string): JsonRecord {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(`${at} must be an object`);
  }
  return value as JsonRecord;
}

function text(value: unknown, at: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new TypeError(`${at} must be a non-empty string`);
  }
  return value;
}

function decimal(value: unknown, at: string): string {
  return signedDecimal(value, at, false);
}

function signedDecimal(value: unknown, at: string, signed = true): string {
  const candidate = text(value, at);
  const grammar = signed ? /^(?:0|-?[1-9]\d*)$/u : /^(?:0|[1-9]\d*)$/u;
  if (!grammar.test(candidate)) {
    throw new TypeError(`${at} must be ${signed ? "a signed" : "an unsigned"} decimal string`);
  }
  return candidate;
}

function hex(value: unknown, at: string): string {
  const candidate = text(value, at).toLowerCase();
  if (!/^[0-9a-f]{64}$/u.test(candidate)) {
    throw new TypeError(`${at} must be a 32-byte lowercase hex identifier`);
  }
  return candidate;
}

function optionalHex(value: unknown, at: string): string | undefined {
  return value === undefined || value === null ? undefined : hex(value, at);
}

function boolean(value: unknown, at: string): boolean {
  if (typeof value !== "boolean") {
    throw new TypeError(`${at} must be a boolean`);
  }
  return value;
}

function decodeProgramSource(value: unknown, at: string): ProgramSourceStatus {
  const item = record(value, at);
  const status = text(item.status, `${at}.status`);
  if (status === "unpublished") {
    return Object.freeze({ status });
  }
  if (status === "verified") {
    return Object.freeze({
      status,
      sourceDigest: hex(item.source_digest, `${at}.source_digest`),
      environmentDigest: hex(item.environment_digest, `${at}.environment_digest`),
    });
  }
  if (status === "mismatch") {
    return Object.freeze({
      status,
      expected: hex(item.expected, `${at}.expected`),
      reproduced: hex(item.reproduced, `${at}.reproduced`),
    });
  }
  throw new TypeError(`${at}.status is not a declared source status`);
}

export function decodeNameResolution(value: unknown, at = "name"): NameResolutionRecord {
  const item = record(value, at);
  const name = text(item.name, `${at}.name`);
  if (!validExplorerName(name)) {
    throw new TypeError(`${at}.name is not a registrable name`);
  }
  return Object.freeze({
    name,
    did: hex(item.did, `${at}.did`),
    expiry: decimal(item.expiry, `${at}.expiry`),
  });
}

export function decodeProgram(value: unknown, at = "program"): ProgramRecord {
  const item = record(value, at);
  const policy = record(item.upgrade_policy, `${at}.upgrade_policy`);
  const policyKind = text(policy.kind, `${at}.upgrade_policy.kind`);
  if (policyKind !== "immutable" && policyKind !== "upgradeable") {
    throw new TypeError(`${at}.upgrade_policy.kind is invalid`);
  }
  const lifecycle = text(item.lifecycle, `${at}.lifecycle`);
  if (lifecycle !== "active" && lifecycle !== "deprecated" && lifecycle !== "tombstoned") {
    throw new TypeError(`${at}.lifecycle is invalid`);
  }
  if (!Array.isArray(item.versions) || item.versions.length === 0 || item.versions.length > 1_024) {
    throw new TypeError(`${at}.versions must be a bounded non-empty array`);
  }
  if (!Array.isArray(item.value_accounts) || item.value_accounts.length > 1_024) {
    throw new TypeError(`${at}.value_accounts must be a bounded array`);
  }
  return Object.freeze({
    program: hex(item.program, `${at}.program`),
    upgradePolicy: policyKind === "immutable"
      ? Object.freeze({ kind: "immutable" as const })
      : Object.freeze({
          kind: "upgradeable" as const,
          authority: hex(policy.authority, `${at}.upgrade_policy.authority`),
        }),
    lifecycle,
    versions: Object.freeze(item.versions.map((candidate, index) => {
      const version = record(candidate, `${at}.versions[${String(index)}]`);
      const interfaceDigest = optionalHex(version.interface_digest, `${at}.versions[${String(index)}].interface_digest`);
      return Object.freeze({
        version: decimal(version.version, `${at}.versions[${String(index)}].version`),
        codeHash: hex(version.code_hash, `${at}.versions[${String(index)}].code_hash`),
        abiVersion: decimal(version.abi_version, `${at}.versions[${String(index)}].abi_version`),
        ...(interfaceDigest === undefined ? {} : { interfaceDigest }),
        source: decodeProgramSource(version.source, `${at}.versions[${String(index)}].source`),
      });
    })),
    valueAccounts: Object.freeze(item.value_accounts.map((candidate, index) => {
      const account = record(candidate, `${at}.value_accounts[${String(index)}]`);
      return Object.freeze({
        account: hex(account.account, `${at}.value_accounts[${String(index)}].account`),
        asset: hex(account.asset, `${at}.value_accounts[${String(index)}].asset`),
        balance: decimal(account.balance, `${at}.value_accounts[${String(index)}].balance`),
        frozen: boolean(account.frozen, `${at}.value_accounts[${String(index)}].frozen`),
      });
    })),
    observedSequence: decimal(item.observed_sequence, `${at}.observed_sequence`),
    observedAt: decimal(item.observed_at, `${at}.observed_at`),
    receiptDigest: hex(item.receipt_digest, `${at}.receipt_digest`),
    stateRoot: hex(item.state_root, `${at}.state_root`),
  });
}

export function decodeProgramForIdentifier(value: unknown, identifier: string): ProgramRecord {
  if (!validExplorerIdentifier(identifier)) {
    throw new TypeError("Invalid program identifier");
  }
  const program = decodeProgram(value);
  if (program.program !== identifier.toLowerCase()) {
    throw new TypeError("Program projection names another program");
  }
  return program;
}

export type AccountIdentifierKind = "evm" | "did" | "account";

/// One account spelling, normalised to the single form its page lives at.
export interface AccountIdentifier {
  readonly kind: AccountIdentifierKind;
  readonly canonical: string;
}

const DID_PREFIX = "did:layerx:";

export function parseAccountIdentifier(value: string): AccountIdentifier | undefined {
  const candidate = value.trim().normalize("NFC").toLowerCase();
  if (/^0x[0-9a-f]{40}$/u.test(candidate)) {
    return Object.freeze({ kind: "evm" as const, canonical: candidate });
  }
  if (candidate.startsWith(DID_PREFIX)) {
    const key = candidate.slice(DID_PREFIX.length);
    return validExplorerIdentifier(key)
      ? Object.freeze({ kind: "did" as const, canonical: `${DID_PREFIX}${key}` })
      : undefined;
  }
  if (validExplorerIdentifier(candidate)) {
    return Object.freeze({ kind: "account" as const, canonical: candidate });
  }
  return undefined;
}

export function accountIdentifierPath(canonical: string): string {
  return `/explorer/accounts/${encodeURIComponent(canonical)}`;
}

export function validExplorerIdentifier(value: string): boolean {
  return /^[0-9a-fA-F]{64}$/u.test(value);
}

const NAME_GRAMMAR = /^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/u;

export function validExplorerName(value: string): boolean {
  return value.normalize("NFC") === value && NAME_GRAMMAR.test(value);
}

export function validExplorerCoordinate(value: string): boolean {
  return /^(?:0|[1-9]\d*)$/u.test(value);
}

export function explorer(): Readonly<{ publicOnly: true; maximumPageSize: 100 }> {
  return Object.freeze({ publicOnly: true, maximumPageSize: 100 });
}

export function human_web_explorer() {
  return explorer();
}

export {
  decodeNativeUnifiedAccount,
  decodeNativeArchiveReceipt,
  decodeNativeCursorRefusal,
  type NativeUnifiedAccount,
  type NativeActivity,
  type NativeAsset,
  type NativeArchiveReceipt,
  type NativeCursorRefusalReason,
} from "./native-model.ts";

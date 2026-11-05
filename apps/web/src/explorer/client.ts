import "server-only";

import { ethTransactionReceipt } from "../api/gateway.ts";
import { explorerLinkPath, parseExplorerBaseUrl, type ExplorerLinkTarget } from "./links.ts";
import {
  decodeNativeIndexedHeight,
  decodeNativeTransactionStatus,
  nativeFreshnessFromPositions,
  validExplorerIdentifier,
  validExplorerName,
  type NameResolutionRecord,
  type ProgramRecord,
} from "./model.ts";
import {
  decodeReceipt,
  decodeRecord,
  type ExplorerRecord,
  type ReceiptRecord,
  type UnifiedAccountAvailabilityRecord,
} from "./historical-model.ts";
import {
  decodeNativeArchiveReceipt,
  decodeNativeCursorRefusal,
  decodeNativeUnifiedAccount,
  nativeAddress,
  nativeCursor,
  type NativeArchiveReceipt,
  type NativeCursorRefusalReason,
  type NativeUnifiedAccount,
} from "./native-model.ts";

const FETCH_TIMEOUT_MS = 8_000;

export class ExplorerUnavailableError extends Error {
  constructor() {
    super("The public explorer service is unavailable");
    this.name = "ExplorerUnavailableError";
  }
}

export class ExplorerHttpError extends ExplorerUnavailableError {
  readonly status: number;
  readonly responseBody: unknown;

  constructor(status: number, responseBody: unknown) {
    super();
    this.name = "ExplorerHttpError";
    this.status = status;
    this.responseBody = responseBody;
  }
}

export class NativeCursorRefusedError extends ExplorerHttpError {
  readonly reason: NativeCursorRefusalReason;

  constructor(reason: NativeCursorRefusalReason, responseBody: unknown) {
    super(422, responseBody);
    this.name = "NativeCursorRefusedError";
    this.reason = reason;
  }
}

export class ExplorerInterfaceUnavailableError extends ExplorerUnavailableError {
  readonly capability: "canonical-receipt-bytes" | "program-read" | "naming-program-read" | "legacy-account-projection";

  constructor(capability: ExplorerInterfaceUnavailableError["capability"]) {
    super();
    this.name = "ExplorerInterfaceUnavailableError";
    this.capability = capability;
    this.message = `The native explorer does not provide ${capability}`;
  }
}

function explorerBaseUrl(): URL {
  const base = parseExplorerBaseUrl(process.env.PAXEER_X_EXPLORER_BASE_URL);
  if (base === undefined || base.href !== `${base.origin}/`) throw new ExplorerUnavailableError();
  return base;
}

export function explorerLink(target: ExplorerLinkTarget): string {
  return new URL(explorerLinkPath(target), explorerBaseUrl()).toString();
}

// Native receipts are read by transaction hash; archive receipt digests are a separate contract.
export async function currentReceiptLink(transactionHash: string): Promise<string | undefined> {
  const destination = explorerLink({ kind: "transaction", transactionHash });
  const receipt = await ethTransactionReceipt(transactionHash);
  if (receipt === undefined) return undefined;
  if (receipt.transactionHash.toLowerCase() !== transactionHash.toLowerCase()) {
    throw new TypeError("Native receipt names another transaction");
  }
  return destination;
}

async function nativeGet(path: string, cursor?: string): Promise<unknown | undefined> {
  const url = new URL(path, explorerBaseUrl());
  if (cursor !== undefined) url.searchParams.set("cursor", nativeCursor(cursor));
  let response: Response;
  try {
    response = await fetch(url, {
      headers: { Accept: "application/json" },
      cache: "no-store",
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
  } catch {
    throw new ExplorerUnavailableError();
  }
  if (response.status === 404) return undefined;
  let value: unknown;
  try {
    value = await response.json();
  } catch {
    throw new ExplorerHttpError(response.status, undefined);
  }
  if (response.status === 422 && cursor !== undefined) {
    // Invalid parameters and undeclared refusals remain HTTP errors, not snapshot resets.
    if (typeof value === "object" && value !== null && "message" in value
      && value.message === "Invalid activity cursor") {
      throw new NativeCursorRefusedError(decodeNativeCursorRefusal(value), value);
    }
  }
  if (!response.ok) throw new ExplorerHttpError(response.status, value);
  return value;
}

export async function nativeExplorerFreshness(transactionHash: string) {
  explorerLinkPath({ kind: "transaction", transactionHash });
  const [status, blocks] = await Promise.all([
    nativeGet(`/api/v2/transactions/${transactionHash.toLowerCase()}/status`),
    nativeGet("/api/v2/blocks?type=block"),
  ]);
  return nativeFreshnessFromPositions(
    status === undefined ? null : decodeNativeTransactionStatus(status).committedHeight,
    blocks === undefined ? null : decodeNativeIndexedHeight(blocks),
  );
}

export async function nativeUnifiedAccount(identifier: string, cursor?: string): Promise<NativeUnifiedAccount | undefined> {
  const address = nativeAddress(identifier).toLowerCase();
  if (cursor !== undefined) nativeCursor(cursor);
  const value = await nativeGet(`/api/v2/addresses/${encodeURIComponent(address)}/unified`, cursor);
  if (value === undefined) return undefined;
  const account = decodeNativeUnifiedAccount(value);
  if (account.identities.evm.toLowerCase() !== address) throw new TypeError("Native account projection names another address");
  return account;
}

export async function unifiedAccount(
  identifier: string,
  cursor?: string,
  retiredBefore?: string,
  retiredLimit?: number,
): Promise<(NativeUnifiedAccount & Readonly<{ canonical: string }>) | undefined> {
  if (retiredBefore !== undefined || retiredLimit !== undefined) {
    throw new TypeError("Native unified accounts require the returned opaque cursor, without before/before_block or limit");
  }
  const account = await nativeUnifiedAccount(identifier, cursor);
  return account === undefined ? undefined : Object.freeze({ ...account, canonical: account.identities.evm.toLowerCase() });
}

export async function accountActivityPage(identifier: string, cursor?: string, retiredLimit?: string) {
  if (retiredLimit !== undefined) throw new TypeError("Native account activity has a fixed page size; use its opaque cursor");
  const account = await nativeUnifiedAccount(identifier, cursor);
  if (account === undefined) throw new ExplorerHttpError(404, undefined);
  return Object.freeze({
    items: account.activity,
    ...(account.next_page_params === null ? {} : { nextBefore: account.next_page_params.cursor }),
    pageCursor: account.page_cursor,
    firstPageCursor: account.first_page_cursor,
    pageNumber: account.page_number,
    activityTotal: account.activity_total,
  });
}

export async function archiveReceiptRecord(identifier: string): Promise<NativeArchiveReceipt | undefined> {
  if (!validExplorerIdentifier(identifier)) throw new TypeError("Invalid receipt identifier");
  const id = `0x${identifier.toLowerCase()}`;
  const value = await nativeGet(`/api/v2/paxeer-x/archive/receipts/${id}`);
  if (value === undefined) return undefined;
  const receipt = decodeNativeArchiveReceipt(value);
  if (receipt.id.toLowerCase() !== id) throw new TypeError("Archive projection names another receipt");
  return receipt;
}

// The native archive contains provenance hashes, not canonical bytes or index freshness.
// Keep the old page contract explicit until its owner wires the archive presentation.
export async function receiptRecord(identifier: string): Promise<ExplorerRecord<ReceiptRecord>> {
  await archiveReceiptRecord(identifier);
  throw new ExplorerInterfaceUnavailableError("canonical-receipt-bytes");
}

export function decodeReceiptRecordForIdentifier(value: unknown, identifier: string): ExplorerRecord<ReceiptRecord> {
  if (!validExplorerIdentifier(identifier)) throw new TypeError("Invalid receipt identifier");
  const record = decodeRecord(value, decodeReceipt, "receipt record");
  if (record.value !== undefined && record.value.receiptId !== identifier.toLowerCase()) {
    throw new TypeError("Receipt projection names another receipt");
  }
  return record;
}

export async function unifiedAccountAvailability(
  identifier: string,
  _beforeBlock?: string,
  _before?: string,
  _limit = 25,
): Promise<UnifiedAccountAvailabilityRecord | undefined> {
  explorerBaseUrl();
  throw new ExplorerInterfaceUnavailableError("legacy-account-projection");
}

export class UnifiedAccountUnavailableError extends ExplorerUnavailableError {
  readonly reason: "gateway_unavailable" | "index_incomplete";

  constructor(reason: "gateway_unavailable" | "index_incomplete") {
    super();
    this.reason = reason;
  }
}

export async function programRecord(identifier: string): Promise<ProgramRecord | undefined> {
  if (!validExplorerIdentifier(identifier)) throw new TypeError("Invalid program identifier");
  explorerBaseUrl();
  throw new ExplorerInterfaceUnavailableError("program-read");
}

export async function resolveName(name: string): Promise<NameResolutionRecord | undefined> {
  if (!validExplorerName(name)) throw new TypeError("Invalid name");
  explorerBaseUrl();
  throw new ExplorerInterfaceUnavailableError("naming-program-read");
}

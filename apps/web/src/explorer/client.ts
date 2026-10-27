import "server-only";

import { explorerLinkPath, parseExplorerBaseUrl, type ExplorerLinkTarget } from "./links";
import {
  decodeAccountActivity,
  decodeNameResolution,
  decodePage,
  decodeProgramForIdentifier,
  decodeReceipt,
  decodeRecord,
  decodeUnifiedAccount,
  decodeUnifiedAccountAvailability,
  decodeUnifiedAccountUnavailable,
  parseAccountIdentifier,
  validExplorerCoordinate,
  validExplorerIdentifier,
  validExplorerName,
  type AccountActivityRecord,
  type ExplorerPage,
  type ExplorerRecord,
  type NameResolutionRecord,
  type ProgramRecord,
  type ReceiptRecord,
  type UnifiedAccountRecord,
  type UnifiedAccountAvailabilityRecord,
} from "./model";

const FETCH_TIMEOUT_MS = 8_000;

export class ExplorerUnavailableError extends Error {
  constructor() {
    super("The public explorer service is unavailable");
    this.name = "ExplorerUnavailableError";
  }
}

function explorerOrigin(): URL {
  return explorerBaseUrl();
}

/// PAXEER_X_EXPLORER_BASE_URL names the public base URL of the Paxeer X Network explorer, the
/// one surface that renders anchors, receipts, transactions and addresses. It is the only place
/// the explorer's host is configured: no host is written into this repository, and an unset or
/// malformed value fails closed with the unavailable state rather than guessing an origin.
function explorerBaseUrl(): URL {
  const base = parseExplorerBaseUrl(process.env.PAXEER_X_EXPLORER_BASE_URL);
  if (base === undefined) {
    throw new ExplorerUnavailableError();
  }
  return base;
}

export function explorerLink(target: ExplorerLinkTarget): string {
  return new URL(explorerLinkPath(target), explorerBaseUrl()).toString();
}

async function get(
  path: string,
  query?: Readonly<Record<string, string>>,
  profile?: "unified" | "unified-availability",
): Promise<unknown> {
  const url = new URL(path, explorerOrigin());
  for (const [name, value] of Object.entries(query ?? {})) {
    url.searchParams.set(name, value);
  }
  let response: Response;
  try {
    response = await fetch(url, {
      headers: {
        Accept: "application/json",
        ...(profile === "unified-availability" ? { "LayerX-Unified-Profile": "2" } : {}),
      },
      ...(profile === undefined
        ? { cache: "force-cache", next: { revalidate: 60 } } as const
        : { cache: "no-store" } as const),
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
  } catch {
    throw new ExplorerUnavailableError();
  }
  if (!response.ok) {
    if (response.status === 404) {
      if (profile !== undefined) {
        return undefined;
      }
      try {
        return await response.json();
      } catch {
        throw new ExplorerUnavailableError();
      }
    }
    if (response.status === 503 && profile === "unified-availability") {
      throw new UnifiedAccountUnavailableError(decodeUnifiedAccountUnavailable(await response.json()));
    }
    throw new ExplorerUnavailableError();
  }
  try {
    return await response.json();
  } catch {
    throw new ExplorerUnavailableError();
  }
}

export async function receiptRecord(
  identifier: string,
): Promise<ExplorerRecord<ReceiptRecord>> {
  if (!validExplorerIdentifier(identifier)) {
    throw new TypeError("Invalid receipt identifier");
  }
  return decodeRecord(
    await get(`/v1/explorer/receipts/${encodeURIComponent(identifier.toLowerCase())}`),
    decodeReceipt,
    "receipt record",
  );
}

export async function accountActivityPage(
  identifier: string,
  before?: string,
  limit = "25",
): Promise<ExplorerPage<AccountActivityRecord>> {
  if (
    !validExplorerIdentifier(identifier)
    || (before !== undefined && !validExplorerCoordinate(before))
    || !validExplorerCoordinate(limit)
  ) {
    throw new TypeError("Invalid account activity query");
  }
  return decodePage(
    await get(`/v1/explorer/accounts/${encodeURIComponent(identifier.toLowerCase())}`, {
      ...(before === undefined ? {} : { before }),
      limit,
    }),
    decodeAccountActivity,
    "account activity page",
  );
}

export async function programRecord(identifier: string): Promise<ProgramRecord | undefined> {
  if (!validExplorerIdentifier(identifier)) {
    throw new TypeError("Invalid program identifier");
  }
  const url = new URL(`/v1/programs/${encodeURIComponent(identifier.toLowerCase())}`, explorerOrigin());
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
  if (response.status === 404) {
    return undefined;
  }
  if (!response.ok) {
    throw new ExplorerUnavailableError();
  }
  try {
    return decodeProgramForIdentifier(await response.json(), identifier);
  } catch (error) {
    if (error instanceof TypeError) {
      throw error;
    }
    throw new ExplorerUnavailableError();
  }
}

export async function unifiedAccount(
  identifier: string,
  beforeBlock?: string,
  before?: string,
  limit = 25,
): Promise<UnifiedAccountRecord | undefined> {
  const account = parseAccountIdentifier(identifier);
  if (account === undefined
    || (beforeBlock !== undefined && !validExplorerCoordinate(beforeBlock))
    || (before !== undefined && (!validExplorerCoordinate(before) || before === "0"))
    || !Number.isSafeInteger(limit) || limit < 1 || limit > 100) {
    throw new TypeError("Invalid unified account query");
  }
  const value = await get(`/v1/accounts/${encodeURIComponent(account.canonical)}/unified`, {
    ...(beforeBlock === undefined ? {} : { before_block: beforeBlock }),
    ...(before === undefined ? {} : { before }),
    limit: String(limit),
  }, "unified");
  if (value === undefined) {
    return undefined;
  }
  try {
    return decodeUnifiedAccount(value);
  } catch (error) {
    if (error instanceof TypeError) {
      throw error;
    }
    throw new ExplorerUnavailableError();
  }
}

export async function unifiedAccountAvailability(
  identifier: string,
  beforeBlock?: string,
  before?: string,
  limit = 25,
): Promise<UnifiedAccountAvailabilityRecord | undefined> {
  const account = parseAccountIdentifier(identifier);
  if (account === undefined
    || (beforeBlock !== undefined && !validExplorerCoordinate(beforeBlock))
    || (before !== undefined && (!validExplorerCoordinate(before) || before === "0"))
    || !Number.isSafeInteger(limit) || limit < 1 || limit > 100) {
    throw new TypeError("Invalid unified account query");
  }
  const value = await get(`/v1/accounts/${encodeURIComponent(account.canonical)}/unified`, {
    ...(beforeBlock === undefined ? {} : { before_block: beforeBlock }),
    ...(before === undefined ? {} : { before }),
    limit: String(limit),
  }, "unified-availability");
  if (value === undefined) {
    return undefined;
  }
  try {
    return decodeUnifiedAccountAvailability(value);
  } catch (error) {
    if (error instanceof TypeError) {
      throw error;
    }
    throw new ExplorerUnavailableError();
  }
}

export class UnifiedAccountUnavailableError extends ExplorerUnavailableError {
  constructor(readonly reason: "gateway_unavailable" | "index_incomplete") {
    super();
  }
}


function namingProgram(): string {
  const configured = process.env.LAYERX_EXPLORER_NAMING_PROGRAM;
  if (configured === undefined || !validExplorerIdentifier(configured)) {
    throw new ExplorerUnavailableError();
  }
  return configured.toLowerCase();
}

export async function resolveName(name: string): Promise<NameResolutionRecord | undefined> {
  if (!validExplorerName(name)) {
    throw new TypeError("Invalid name");
  }
  const url = new URL(`/v1/programs/${namingProgram()}/reads/resolve`, explorerOrigin());
  url.searchParams.set("name", name);
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
  if (response.status === 404) {
    return undefined;
  }
  if (!response.ok) {
    throw new ExplorerUnavailableError();
  }
  try {
    const resolution = decodeNameResolution(await response.json());
    if (resolution.name !== name) {
      throw new TypeError("Name projection names another name");
    }
    return resolution;
  } catch (error) {
    if (error instanceof TypeError) {
      throw error;
    }
    throw new ExplorerUnavailableError();
  }
}

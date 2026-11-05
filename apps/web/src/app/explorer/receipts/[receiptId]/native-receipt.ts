import "server-only";

import { ExplorerHttpError, ExplorerUnavailableError } from "../../../../explorer/client.ts";
import { parseExplorerBaseUrl } from "../../../../explorer/links.ts";

export interface NativeReceiptTransaction {
  readonly hash: string;
  readonly block_number: number | null;
  readonly position: number | null;
  readonly status: "ok" | "error" | null;
}

function transactionHash(identifier: string): string {
  if (!/^(?:0x)?[0-9a-fA-F]{64}$/u.test(identifier)) throw new TypeError("Invalid native transaction hash");
  return `0x${identifier.replace(/^0x/u, "").toLowerCase()}`;
}

// Read the native transaction API's block_number and position (transaction.index).
// An archive receipt digest is never interpreted as a transaction mapping.
export async function readNativeReceiptTransaction(identifier: string): Promise<NativeReceiptTransaction | undefined> {
  const hash = transactionHash(identifier);
  const base = parseExplorerBaseUrl(process.env.PAXEER_X_EXPLORER_BASE_URL);
  if (base === undefined) throw new ExplorerUnavailableError();
  const response = await fetch(new URL(`/api/v2/transactions/${hash}`, base), {
    headers: { Accept: "application/json" },
    cache: "no-store",
    signal: AbortSignal.timeout(8_000),
  });
  if (response.status === 404) return undefined;
  const value: unknown = await response.json();
  if (!response.ok) throw new ExplorerHttpError(response.status, value);
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError("Invalid native transaction");
  }
  if (!("hash" in value) || typeof value.hash !== "string" || transactionHash(value.hash) !== hash) {
    throw new TypeError("Native transaction response names another transaction");
  }
  if (!("block_number" in value) || !("position" in value) || !("status" in value)) {
    throw new TypeError("Native transaction position is missing");
  }
  const block = value.block_number;
  const position = value.position;
  const status = value.status;
  if ((block !== null && (typeof block !== "number" || !Number.isSafeInteger(block) || block < 0))
    || (position !== null && (typeof position !== "number" || !Number.isSafeInteger(position) || position < 0))
    || ((block === null) !== (position === null))
    || (status !== null && status !== "ok" && status !== "error")) {
    throw new TypeError("Invalid native transaction position or status");
  }
  return Object.freeze({ hash, block_number: block, position, status });
}

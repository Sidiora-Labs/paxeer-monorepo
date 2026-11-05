import { validExplorerIdentifier } from "./model.ts";

/// The explorer surfaces the control plane links into, each carrying its own identifier.
export const EXPLORER_LINK_KINDS = [
  "receipt",
  "historical-receipt",
  "transaction",
  "address",
] as const;

export type ExplorerLinkKind = (typeof EXPLORER_LINK_KINDS)[number];

export type ExplorerLinkTarget =
  | Readonly<{ kind: "receipt"; receiptId: string; transactionHash: string }>
  | Readonly<{ kind: "historical-receipt"; receiptId: string; frozen: true }>
  | Readonly<{ kind: "transaction"; transactionHash: string }>
  | Readonly<{ kind: "address"; address: string }>;

// Kept as the identifier of the retired surface for admission checks, never a link target.
export const EXPLORER_ANCHOR_PATH = "/paxeer-x/anchors";

const TRANSACTION_HASH_GRAMMAR = /^0x[0-9a-fA-F]{64}$/u;
const ADDRESS_GRAMMAR = /^0x[0-9a-fA-F]{40}$/u;

export function explorerLinkPath(target: ExplorerLinkTarget): string {
  switch (target.kind) {
    case "receipt":
      if (!validExplorerIdentifier(target.receiptId)) {
        throw new TypeError("Invalid receipt identifier");
      }
      if (!TRANSACTION_HASH_GRAMMAR.test(target.transactionHash)) {
        throw new TypeError("Current receipt requires its resolved native transaction hash");
      }
      return `/tx/${target.transactionHash.toLowerCase()}`;
    case "historical-receipt":
      if (target.frozen !== true || !validExplorerIdentifier(target.receiptId)) {
        throw new TypeError("Invalid frozen historical receipt identity");
      }
      return `/paxeer-x/receipts/${target.receiptId.toLowerCase()}`;
    case "transaction":
      if (!TRANSACTION_HASH_GRAMMAR.test(target.transactionHash)) {
        throw new TypeError("Invalid transaction hash");
      }
      return `/tx/${target.transactionHash.toLowerCase()}`;
    case "address":
      if (!ADDRESS_GRAMMAR.test(target.address)) {
        throw new TypeError("Invalid address");
      }
      return `/address/${target.address.toLowerCase()}`;
  }
  throw new TypeError("Unsupported explorer link kind");
}

/// The explorer's public base URL, as the control plane reads it from its environment. A base URL
/// is accepted only when it is an origin on its own: HTTPS, or HTTP for a loopback host in local
/// development, carrying no credentials, no path, no query and no fragment. Anything else, and an
/// unset variable, yields undefined so the caller fails closed instead of guessing an origin.
export function parseExplorerBaseUrl(configured: string | undefined): URL | undefined {
  if (configured === undefined) {
    return undefined;
  }
  let base: URL;
  try {
    base = new URL(configured);
  } catch {
    return undefined;
  }
  const loopback = base.hostname === "127.0.0.1" || base.hostname === "localhost";
  if (
    (base.protocol !== "https:" && !(loopback && base.protocol === "http:"))
    || base.username !== ""
    || base.password !== ""
    || base.pathname !== "/"
    || base.search !== ""
    || base.hash !== ""
  ) {
    return undefined;
  }
  return base;
}

/// The control plane's own explorer routes, in the order they are offered. Every entry names a
/// route the control plane still serves.
export const EXPLORER_NAVIGATION = [
  Object.freeze({ href: "/explorer", copyKey: "explorer.navigation.overview" }),
] as const;

export interface CurrentReceiptIdentity {
  readonly receiptId: string;
  readonly transactionHash: string;
}

export async function resolveReceiptLink(
  receiptId: string,
  read: (receiptId: string) => Promise<CurrentReceiptIdentity | undefined>,
): Promise<ExplorerLinkTarget> {
  if (!validExplorerIdentifier(receiptId)) throw new TypeError("Invalid receipt identifier");
  const receipt = await read(receiptId.toLowerCase());
  if (receipt === undefined || receipt.receiptId.toLowerCase() !== receiptId.toLowerCase()) {
    throw new TypeError("Native receipt identity does not match the requested receipt");
  }
  const target = { kind: "receipt", receiptId: receipt.receiptId, transactionHash: receipt.transactionHash } as const;
  explorerLinkPath(target);
  return target;
}

export function historicalReceiptLinkPath(identity: Readonly<{ receiptId: string; frozen: true; }>): string {
  if (identity.frozen !== true || !validExplorerIdentifier(identity.receiptId)) {
    throw new TypeError("Historical receipt requires a frozen archive identity");
  }
  return `/api/v2/paxeer-x/archive/receipts/0x${identity.receiptId.toLowerCase()}`;
}

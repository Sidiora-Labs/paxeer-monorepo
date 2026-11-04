/**
 * Local, per-account transfer ledger (localStorage). This is the user's own
 * record of what they submitted from this browser — chain state is always
 * re-derived from receipts, isNullified and the attestation API, never from
 * this store. v1 shape; bump the key to migrate.
 */
import { isAddress, type Address, type Hex } from 'viem';
import type { SettlementStatus } from './settlement';

export type TransferDirection = 'in' | 'out';

export type BridgeTransfer = {

  /** `${sourceTxHash.toLowerCase()}:${direction}`, as returned by transferIdOf. */
  id: string;
  direction: TransferDirection;

  /** Registry key of the foreign chain (e.g. "base", "solana"). */
  foreignChainKey: string;

  /** Bridge chain id registered with the precompile, decimal string. */
  foreignChainId: string;
  symbol: string;
  decimals: number;

  /** Base units, decimal string. */
  amount: string;

  /** Asset id the precompile keys the pair by (address or Solana handle). */
  assetId: Address;
  vault?: Address;

  /** Wallet that submitted the source transaction. */
  account: Address;

  /** Destination account (Paxeer address for `in`, foreign recipient for `out`). */
  recipient: string;

  /** Hold-back: a Paxeer destination that differs from the connected wallet. */
  destinationHeldBack?: boolean;
  sourceTxHash: Hex;

  /** Confirmation depth target recorded at submission (finality depth). */
  requiredConfirmations: number;
  submittedAt: number;
  // Deposit (`in`) attestation identity:
  logIndex?: string;
  nonce?: string;
  claimedAt?: number;
  claimTxHash?: Hex;
  // Withdrawal (`out`) attestation identity:
  bridgeOutNonce?: string;
  releaseTxHash?: Hex;
  releasedAt?: number;
  settlementSubmittedAt?: number;
  settlementStatus?: SettlementStatus;
};

const STORAGE_KEY = 'paxeer-bridge:transfers:v1';
const MAX_TRANSFERS_PER_ACCOUNT = 50;

function isHash(value: unknown): value is Hex {
  return typeof value === 'string' && /^0x[0-9a-fA-F]{64}$/.test(value);
}

function isDecimal(value: unknown): value is string {
  return typeof value === 'string' && /^(0|[1-9]\d*)$/.test(value);
}

function isBridgeTransfer(value: unknown): value is BridgeTransfer {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false;
  const transfer = value as Record<string, unknown>;
  if (transfer.direction !== 'in' && transfer.direction !== 'out') return false;
  if (!isHash(transfer.sourceTxHash)) return false;
  return (
    transfer.id === transferIdOf(transfer.direction, transfer.sourceTxHash) &&
    typeof transfer.account === 'string' && isAddress(transfer.account, { strict: false }) &&
    typeof transfer.foreignChainKey === 'string' && transfer.foreignChainKey.length > 0 &&
    isDecimal(transfer.foreignChainId) && BigInt(transfer.foreignChainId) < (1n << 64n) &&
    typeof transfer.symbol === 'string' && transfer.symbol.length > 0 &&
    typeof transfer.decimals === 'number' && Number.isInteger(transfer.decimals) && transfer.decimals >= 0 && transfer.decimals <= 255 &&
    isDecimal(transfer.amount) && BigInt(transfer.amount) < (1n << 256n) &&
    typeof transfer.assetId === 'string' && isAddress(transfer.assetId, { strict: false }) &&
    typeof transfer.recipient === 'string' && transfer.recipient.length > 0 &&
    typeof transfer.requiredConfirmations === 'number' && Number.isSafeInteger(transfer.requiredConfirmations) && transfer.requiredConfirmations > 0 &&
    typeof transfer.submittedAt === 'number' && Number.isSafeInteger(transfer.submittedAt) && transfer.submittedAt >= 0 &&
    (transfer.vault === undefined || (typeof transfer.vault === 'string' && isAddress(transfer.vault, { strict: false }))) &&
    [ 'logIndex', 'nonce', 'bridgeOutNonce' ].every((field) => transfer[field] === undefined || (isDecimal(transfer[field]) && BigInt(transfer[field]) < (1n << 64n))) &&
    [ 'claimTxHash', 'releaseTxHash' ].every((field) => transfer[field] === undefined || isHash(transfer[field])) &&
    [ 'claimedAt', 'releasedAt', 'settlementSubmittedAt' ].every((field) => transfer[field] === undefined ||
      (typeof transfer[field] === 'number' && Number.isSafeInteger(transfer[field]) && transfer[field] >= 0)) &&
    (transfer.settlementStatus === undefined || [ 'pending', 'dropped', 'reverted', 'confirmed' ].includes(String(transfer.settlementStatus))) &&
    (transfer.destinationHeldBack === undefined || typeof transfer.destinationHeldBack === 'boolean')
  );
}

function readAll(): Array<BridgeTransfer> {
  if (typeof window === 'undefined') return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter(isBridgeTransfer) : [];
  } catch {
    return [];
  }
}

function writeAll(list: Array<BridgeTransfer>): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
  } catch {
    // Storage full or unavailable — the ledger is a convenience, not state.
  }
}

export function listTransfers(account?: Address): Array<BridgeTransfer> {
  const all = readAll();
  if (!account) return all;
  return all.filter((t) => t.account.toLowerCase() === account.toLowerCase());
}

export function saveTransfer(transfer: BridgeTransfer): void {
  const all = readAll().filter((t) => t.id !== transfer.id);
  const account = transfer.account.toLowerCase();
  const own = [ transfer, ...all.filter((t) => t.account.toLowerCase() === account) ]
    .sort((a, b) => b.submittedAt - a.submittedAt)
    .slice(0, MAX_TRANSFERS_PER_ACCOUNT);
  writeAll([ ...own, ...all.filter((t) => t.account.toLowerCase() !== account) ]);
}

export function updateTransfer(id: string, patch: Partial<BridgeTransfer>): void {
  writeAll(readAll().map((t) => (t.id === id ? { ...t, ...patch } : t)));
}

export function transferIdOf(direction: TransferDirection, sourceTxHash: Hex): string {
  return `${ sourceTxHash.toLowerCase() }:${ direction }`;
}

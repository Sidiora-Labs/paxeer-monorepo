import { isAddress, type Address, type Hash } from "viem";
import { getPublicClient, getWalletClient } from "@/core/viem/clients";
import { paxeerChain } from "@/core/chain";
import { createLogger } from "@/core/log/logger";
import { gasWithMargin } from "@/core/network/gas";

const log = createLogger("perps:contract");

export async function readContract(params: {
  address: Address;
  abi: readonly unknown[];
  functionName: string;
  args?: readonly unknown[];
}) {
  const client = getPublicClient();
  return client.readContract(params as any);
}

// The chain enforces a minimum gas price of 10 wei. The RPC's fee
// suggestions (base fee ~0 on this chain) can come back below that,
// and a wallet that trusts them signs a tx the chain never includes.
// Clamp both EIP-1559 fee fields to the floor before sending.
const MIN_FEE_PER_GAS_WEI = 10n;

async function estimateClampedFees(): Promise<{
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
}> {
  let maxFeePerGas = MIN_FEE_PER_GAS_WEI;
  let maxPriorityFeePerGas = MIN_FEE_PER_GAS_WEI;
  try {
    const est = await getPublicClient().estimateFeesPerGas();
    if (est.maxFeePerGas && est.maxFeePerGas > maxFeePerGas) {
      maxFeePerGas = est.maxFeePerGas;
    }
    if (est.maxPriorityFeePerGas && est.maxPriorityFeePerGas > maxPriorityFeePerGas) {
      maxPriorityFeePerGas = est.maxPriorityFeePerGas;
    }
  } catch (err) {
    // Estimation failing is not fatal — the floor alone is a valid fee
    // on this chain. Log so a broken fee endpoint is still visible.
    const msg = err instanceof Error ? err.message : String(err);
    log.warn(`estimateFeesPerGas failed, using ${MIN_FEE_PER_GAS_WEI} wei floor: ${msg}`);
  }
  // maxFee must always cover the priority tip.
  if (maxPriorityFeePerGas > maxFeePerGas) maxFeePerGas = maxPriorityFeePerGas;
  return { maxFeePerGas, maxPriorityFeePerGas };
}

export async function writeContract(params: {
  address: Address;
  abi: readonly unknown[];
  functionName: string;
  args?: readonly unknown[];
  value?: bigint;
}): Promise<`0x${string}`> {
  const walletClient = await getWalletClient();
  if (!walletClient) {
    log.error("No wallet client; wallet probably disconnected");
    throw new Error("Wallet not connected — please connect your wallet");
  }

  const account = walletClient.account;
  if (!account) {
    log.error("Wallet client has no account");
    throw new Error("No active account");
  }

  // Preflight: simulate the call against the public RPC so a genuine
  // revert surfaces its decoded reason HERE. Without this, the embedded
  // wallet backend simulates server-side and a revert comes back as an
  // opaque `request failed: 500` that the UI mislabels; injected
  // wallets would burn gas on a doomed tx instead.
  const call = {
    account: account.address,
    address: params.address,
    abi: params.abi,
    functionName: params.functionName,
    args: params.args ?? [],
    ...(params.value ? { value: params.value } : {}),
  } as any;
  const publicClient = getPublicClient();
  await publicClient.simulateContract(call);

  // Gas limit: eth_estimateGas on the public client plus a 20% margin,
  // which covers log gas of a fill the simulation did not see. Fees are
  // estimated here and clamped to the chain's 10 wei minimum gas price —
  // wallet-side estimation was producing sub-floor fees the chain
  // silently never included.
  const gas = gasWithMargin(await publicClient.estimateContractGas(call));
  const { maxFeePerGas, maxPriorityFeePerGas } = await estimateClampedFees();

  const hash = await walletClient.writeContract({
    chain: paxeerChain,
    account,
    address: params.address,
    abi: params.abi,
    functionName: params.functionName,
    args: params.args ?? [],
    gas,
    maxFeePerGas,
    maxPriorityFeePerGas,
    ...(params.value ? { value: params.value } : {}),
  } as any);
  recordTransactionBroadcast(hash);
  return hash;
}

// Transaction confirmation.
//
// The chain is a Sei-EVM fork with instant CometBFT finality (no
// reorgs), so confirmation only needs ONE RPC method:
// `eth_getTransactionReceipt`, polled until the receipt lands.
//
// We deliberately do NOT use viem's `waitForTransactionReceipt`. Its
// replacement-detection path gates every receipt fetch behind
// `eth_getTransactionByHash` inside a 6-attempt exponential retry
// ladder (~12.6 s per cycle). Sei-lineage RPCs only surface txs by
// hash once the async EVM indexer flushes them, and report not-found
// as a JSON-RPC *error* rather than geth's `null` — so viem either
// burns the whole timeout without ever asking for the receipt, or
// rejects with an unrecognized error after ~13 s of retries. Both
// showed up as "tx failed" toasts for transactions that confirmed
// on-chain in under a second. Replacement detection buys nothing on
// an instant-finality chain with no fee-bump UX, so we drop it.
//
// A reverted receipt throws; a missing receipt past the deadline
// throws a timeout the error classifier maps to `RPC_TX_TIMEOUT`.
// We never synthesize success.

const RECEIPT_TIMEOUT_MS = 30_000;
const RECEIPT_POLL_INTERVAL_MS = 400;

export interface WaitForTxResult {
  status: "success";
  transactionHash: `0x${string}`;
  blockNumber: bigint;
}

/**
 * True when an error means "receipt not indexed yet — keep polling".
 * Covers viem's TransactionReceiptNotFoundError (RPC returned null)
 * and Sei-style error-shaped not-found responses.
 */
function isReceiptPendingError(err: unknown): boolean {
  const name = err instanceof Error ? err.name : "";
  if (name === "TransactionReceiptNotFoundError") return true;
  const msg = err instanceof Error ? err.message : String(err);
  return /not\s*(be\s*)?found|does not exist|indexing|not indexed/i.test(msg);
}

export interface PendingTransaction {
  readonly chainId: number;
  /** Null until the node indexes the broadcast's sender; it blocks every account meanwhile. */
  readonly account: Address | null;
  readonly hash: Hash;
}

export const PENDING_TRANSACTIONS_KEY = "paxeer:pending-transactions:v1";
let pendingTransactions: readonly PendingTransaction[] = [];
const transactionListeners = new Set<() => void>();
const receiptWaits = new Map<Hash, Promise<WaitForTxResult>>();

export function subscribePendingTransactions(listener: () => void) {
  transactionListeners.add(listener);
  return () => {
    transactionListeners.delete(listener);
  };
}

export function getPendingTransactions(): readonly PendingTransaction[] {
  return pendingTransactions;
}

function decodePendingTransactions(raw: string | null): PendingTransaction[] {
  if (raw === null) return [];
  const records: unknown = JSON.parse(raw);
  if (!Array.isArray(records)) throw new Error("Invalid pending transaction journal");
  return records.map((record: unknown) => {
    if (
      typeof record !== "object" ||
      record === null ||
      !("chainId" in record) ||
      record.chainId !== paxeerChain.id ||
      !("account" in record) ||
      !(
        record.account === null ||
        (typeof record.account === "string" && isAddress(record.account))
      ) ||
      !("hash" in record) ||
      typeof record.hash !== "string" ||
      !/^0x[0-9a-fA-F]{64}$/.test(record.hash)
    ) {
      throw new Error("Invalid pending transaction journal entry");
    }
    return { chainId: record.chainId, account: record.account, hash: record.hash as Hash };
  });
}

function notifyTransactions() {
  for (const listener of transactionListeners) listener();
}

/** Called before allowing a financial operation. Storage errors must remain visible. */
export function hydratePendingTransactions() {
  if (typeof window === "undefined")
    throw new Error("Transaction recovery requires browser storage");
  const stored = decodePendingTransactions(window.localStorage.getItem(PENDING_TRANSACTIONS_KEY));
  const merged = new Map(stored.map((record) => [record.hash, record]));
  for (const record of pendingTransactions) merged.set(record.hash, record);
  pendingTransactions = [...merged.values()];
  window.localStorage.setItem(PENDING_TRANSACTIONS_KEY, JSON.stringify(pendingTransactions));
  notifyTransactions();
}

function persistTransactions() {
  // Keep the in-memory hash even if durable storage fails after a broadcast.
  try {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(PENDING_TRANSACTIONS_KEY, JSON.stringify(pendingTransactions));
    }
  } finally {
    notifyTransactions();
  }
}

/** Exposes the hash synchronously, before any receipt RPC or transaction lookup. */
export function recordTransactionBroadcast(hash: Hash) {
  if (pendingTransactions.some((record) => record.hash === hash)) return;
  pendingTransactions = [...pendingTransactions, { chainId: paxeerChain.id, account: null, hash }];
  if (typeof window !== "undefined") {
    try {
      hydratePendingTransactions();
    } finally {
      notifyTransactions();
    }
  } else {
    notifyTransactions();
  }
}

function forgetTransaction(hash: Hash) {
  pendingTransactions = pendingTransactions.filter((record) => record.hash !== hash);
  persistTransactions();
}

export class TransactionRevertedError extends Error {
  constructor(readonly hash: Hash) {
    super(`Transaction reverted (hash: ${hash})`);
    this.name = "TransactionRevertedError";
  }
}

async function reconcileTransaction(hash: Hash): Promise<WaitForTxResult> {
  const client = getPublicClient();
  if ((await client.getChainId()) !== paxeerChain.id) {
    throw new Error("Wrong network for pending transaction recovery");
  }
  // Sender indexing is optional: receipt polling must not wait for it on
  // nodes whose transaction index trails committed receipts.
  void client
    .getTransaction({ hash })
    .then((transaction) => {
      if (!pendingTransactions.some((record) => record.hash === hash)) return;
      pendingTransactions = pendingTransactions.map((record) =>
        record.hash === hash ? { ...record, account: transaction.from } : record,
      );
      persistTransactions();
    })
    .catch((error: unknown) => {
      if (!isReceiptPendingError(error)) {
        log.warn(
          `Pending transaction sender lookup failed for ${hash}: ${error instanceof Error ? error.message : String(error)}`,
        );
      }
    });
  const deadline = Date.now() + RECEIPT_TIMEOUT_MS;
  while (Date.now() < deadline) {
    let receipt: Awaited<ReturnType<typeof client.getTransactionReceipt>> | null = null;
    try {
      receipt = await client.getTransactionReceipt({ hash });
    } catch (error: unknown) {
      if (!isReceiptPendingError(error)) throw error;
    }
    if (receipt) {
      if (receipt.transactionHash.toLowerCase() !== hash.toLowerCase()) {
        throw new Error("Pending transaction receipt hash mismatch");
      }
      if (receipt.status === "reverted") {
        forgetTransaction(hash);
        throw new TransactionRevertedError(hash);
      }
      if (receipt.status !== "success") throw new Error("Invalid transaction receipt status");
      forgetTransaction(hash);
      return { status: "success", transactionHash: hash, blockNumber: receipt.blockNumber };
    }
    await new Promise((resolve) => setTimeout(resolve, RECEIPT_POLL_INTERVAL_MS));
  }
  throw new Error(
    `Transaction timed out — not confirmed within ${RECEIPT_TIMEOUT_MS}ms (hash: ${hash})`,
  );
}

export function waitForTx(hash: Hash): Promise<WaitForTxResult> {
  recordTransactionBroadcast(hash);
  const existing = receiptWaits.get(hash);
  if (existing) return existing;
  const waiting = reconcileTransaction(hash).finally(() => {
    receiptWaits.delete(hash);
  });
  receiptWaits.set(hash, waiting);
  return waiting;
}

import { keccak256, stringToHex, type EIP1193Provider, type Hex } from 'viem';
import { PendingAttemptStore, ReceiptRecoveryScheduler, type PendingStorage } from '@/widgets/trading/transaction-controls/recovery';
import type { PendingAttemptRecord, ReceiptEvidence } from '@/widgets/trading/transaction-controls/domain';
import { getWalletClient } from '@/core/wallet-sdk/client';
import type { BountyTransactionIntent, PreparedTransactionContext, TransactionRequest, ValidatedTransaction } from './prepared-transaction';
import { gasCostUpperBound, validatePreparedEnvelope, validatePreparedTransaction } from './prepared-transaction';

export function bountyIntentId(intent: BountyTransactionIntent): string {
  return `bounty:${keccak256(stringToHex(JSON.stringify(intent)))}`;
}

function signingFingerprint(tx: ValidatedTransaction): string {
  return keccak256(stringToHex(JSON.stringify([tx.chainId, tx.from, tx.to, tx.data, tx.value.toString(), tx.gas?.toString() ?? null, tx.maxFeePerGas?.toString() ?? null, tx.maxPriorityFeePerGas?.toString() ?? null])));
}

export function freezeBountyReview(tx: ValidatedTransaction, gas: bigint, feePerGas: bigint) {
  const priorityFeePerGas = tx.maxPriorityFeePerGas ?? 0n;
  if (priorityFeePerGas > feePerGas) throw new Error('Priority fee exceeds the reviewed fee cap');
  return Object.freeze({ tx: Object.freeze({ ...tx }), gas, feePerGas, priorityFeePerGas, costRaw: gasCostUpperBound(gas, feePerGas) });
}
export type BountyFeeReview = ReturnType<typeof freezeBountyReview>;

export function assertBountyReview(review: BountyFeeReview, tx: ValidatedTransaction): void {
  if (signingFingerprint(tx) !== signingFingerprint(review.tx)) throw new Error('Prepared transaction changed; estimate and review again');
}

export function bountyWalletRequest(review: BountyFeeReview) {
  return Object.freeze({ account: review.tx.from, to: review.tx.to, data: review.tx.data, value: review.tx.value, gas: review.gas, maxFeePerGas: review.feePerGas, maxPriorityFeePerGas: review.priorityFeePerGas });
}

export async function prepareBountyReview(request: TransactionRequest, context: PreparedTransactionContext, client: {
  estimateGas(input: { account: ValidatedTransaction['from']; to: ValidatedTransaction['to']; data: Hex; value: bigint }): Promise<bigint>;
  getGasPrice(): Promise<bigint>;
}) {
  const tx = validatePreparedTransaction(request, context);
  const gas = tx.gas ?? await client.estimateGas({ account: tx.from, to: tx.to, data: tx.data, value: tx.value });
  const fee = tx.maxFeePerGas ?? await client.getGasPrice();
  return freezeBountyReview(tx, gas, fee);
}

export async function sendReviewedBountyTransaction(review: BountyFeeReview, tx: ValidatedTransaction, provider: EIP1193Provider): Promise<Hex> {
  assertBountyReview(review, tx);
  return getWalletClient(provider).sendTransaction(bountyWalletRequest(review));
}

export type BountyAttemptState =
  | { kind: 'pending' | 'confirmed' | 'reverted'; tx: ValidatedTransaction; hash: Hex }
  | { kind: 'uncertain'; tx: ValidatedTransaction; hash: Hex; error: string };
export interface BountyReceiptClient {
  getTransactionReceipt(input: { hash: Hex }): Promise<ReceiptEvidence>;
}

/** Uses the shared account-scoped attempt store and bounded receipt scheduler. */
export class BountyTransactionAttempt {
  private readonly store: PendingAttemptStore;
  private readonly recovery: ReceiptRecoveryScheduler;
  private readonly listeners = new Set<() => void>();
  private current: BountyAttemptState | null = null;
  private closed = false;
  private readonly watching = new Set<string>();
  private readonly intentId: string;
  get state() { return this.current; }
  constructor(private readonly storage: PendingStorage, readonly account: string, readonly intent: BountyTransactionIntent, client: BountyReceiptClient) {
    this.store = new PendingAttemptStore(storage, account);
    this.intentId = bountyIntentId(intent);
    this.recovery = new ReceiptRecoveryScheduler(async (attempt, signal) => {
      const receipt = await client.getTransactionReceipt({ hash: attempt.txHash as Hex });
      signal.throwIfAborted();
      return receipt;
    });
    try {
      const loaded = this.store.load();
      if (loaded.corrupt) throw new Error('Local transaction history is corrupt; recover wallet activity before signing');
      const attempts = loaded.records.filter(record => record.intentId === this.intentId);
      for (const attempt of attempts) this.watch(attempt, this.readSnapshot(attempt));
    } catch (error) {
      this.recovery.close();
      throw error;
    }
  }
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; };
  snapshot = () => this.current;
  private snapshotKey(hash: string) { return `sidiora:bounty:1:125:${this.account.toLowerCase()}:${hash.toLowerCase()}`; }
  private readSnapshot(attempt: PendingAttemptRecord): ValidatedTransaction {
    const text = this.storage.getItem(this.snapshotKey(attempt.txHash));
    if (!text || text.length > 100000) throw new Error('Bounty recovery details unavailable; check wallet activity');
    const saved = JSON.parse(text) as { intent: BountyTransactionIntent; request: TransactionRequest; role: string };
    if (bountyIntentId(saved.intent) !== this.intentId) throw new Error('Bounty recovery intent mismatch');
    const tx = validatePreparedEnvelope(saved.request, { wallet: this.account, deployment: { [saved.role]: saved.request.to } });
    if (signingFingerprint(tx) !== attempt.actionFingerprint) throw new Error('Bounty recovery transaction mismatch');
    return tx;
  }
  private publish(state: BountyAttemptState) {
    if (this.closed) return;
    this.current = state;
    for (const listener of this.listeners) listener();
  }
  private watch(attempt: PendingAttemptRecord, tx: ValidatedTransaction) {
    if (this.watching.has(attempt.txHash)) return;
    this.watching.add(attempt.txHash);
    const hash = attempt.txHash as Hex;
    this.publish({ kind: 'pending', tx, hash });
    this.recovery.watch(attempt, update => {
      if (update.receipt) this.publish({ kind: update.receipt.status === 'success' ? 'confirmed' : 'reverted', tx, hash });
      else if (update.error) this.publish({ kind: 'uncertain', tx, hash, error: 'Receipt unavailable. Recover this transaction before trying again' });
    });
  }
  assertCanSubmit(): void {
    const loaded = this.store.load();
    if (this.closed || loaded.corrupt || loaded.records.length >= 200 || this.current || loaded.records.some(record => record.intentId === this.intentId)) throw new Error('Recover the existing bounty transaction before another submission');
  }
  async broadcastReviewed(review: BountyFeeReview, tx: ValidatedTransaction, provider: EIP1193Provider): Promise<Hex> {
    this.assertCanSubmit();
    const hash = await sendReviewedBountyTransaction(review, tx, provider);
    this.recordBroadcast({ ...tx, gas: review.gas, maxFeePerGas: review.feePerGas, maxPriorityFeePerGas: review.priorityFeePerGas }, hash);
    return hash;
  }
  recordBroadcast(tx: ValidatedTransaction, hash: Hex): void {
    if (tx.chainId !== 125 || tx.from.toLowerCase() !== this.account.toLowerCase()) throw new Error('Bounty broadcast scope mismatch');
    const request: TransactionRequest = { chainId: '125', from: tx.from, to: tx.to, data: tx.data, valueRaw: tx.value.toString(), gasLimitRaw: tx.gas?.toString() ?? null, maxFeePerGasRaw: tx.maxFeePerGas?.toString() ?? null, maxPriorityFeePerGasRaw: tx.maxPriorityFeePerGas?.toString() ?? null };
    const attempt: PendingAttemptRecord = { version: 1, chainId: 125, from: this.account, intentId: this.intentId, action: `bounty-${this.intent.action}`, actionFingerprint: signingFingerprint(tx), txHash: hash, submittedAt: Date.now() };
    // Keep the original hash in memory even if browser storage refuses a write.
    this.publish({ kind: 'pending', tx, hash });
    try {
      this.storage.setItem(this.snapshotKey(hash), JSON.stringify({ intent: this.intent, request, role: tx.contractRole }));
      this.store.save(attempt);
    } catch {
      this.publish({ kind: 'uncertain', tx, hash, error: 'Transaction sent, but local recovery storage failed. Keep this hash and check wallet activity' });
      throw new Error('Bounty transaction recovery could not be persisted');
    }
    if (!this.closed) this.watch(attempt, tx);
  }
  retry() { if (this.current) this.recovery.wake(this.current.hash); }
  close() { this.closed = true; this.recovery.close(); this.listeners.clear(); }
}

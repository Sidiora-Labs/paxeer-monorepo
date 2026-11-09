import {
  BaseError,
  UserRejectedRequestError,
  TransactionRejectedRpcError,
  InvalidParamsRpcError,
  keccak256,
  stringToHex,
  type Hex,
  type TransactionReceipt,
} from 'viem';
import { PendingAttemptStore, type PendingStorage } from '../trading/transaction-controls/recovery';
import { address, abiHex, type Address } from './domain';

export type AdvancedAction = 'placeOrder' | 'swapViaAdapter' | 'swapMultiHop';
export interface AdvancedAttempt {
  version: 1;
  chainId: 125;
  account: Address;
  deployment: string;
  action: AdvancedAction;
  target: Address;
  calldata: Hex;
  intentId: Hex;
  submittedAt: number;
  hash?: Hex;
}
const dispatches = new Set<string>();
const broadcasts = new Map<string, AdvancedAttempt>();

export class AdvancedActionRecovery {
  private closed = false;
  constructor(
    private readonly storage: PendingStorage,
    private readonly deployment: string,
  ) {}

  close() {
    this.closed = true;
  }

  private key(account: Address, action: AdvancedAction) {
    if (this.closed) throw new Error('Advanced provider closed');
    return `sidiora:advanced-action:1:125:${this.deployment}:${address(account)}:${action}`;
  }

  pending(account: Address, action: AdvancedAction): AdvancedAttempt | null {
    const key = this.key(account, action);
    const text = this.storage.getItem(key);
    if (text !== null && text.length > 32768)
      throw new Error('Advanced attempt exceeds storage bound');
    const value: AdvancedAttempt | undefined =
      broadcasts.get(key) ?? (text === null ? undefined : JSON.parse(text));
    const stored = new PendingAttemptStore(this.storage, account).load();
    if (stored.corrupt) throw new Error('Corrupt advanced recovery storage');
    if (!value) {
      if (
        stored.records.some(
          (record) =>
            record.action === `advanced:${action}` && record.intentId.startsWith(this.deployment),
        )
      )
        throw new Error('Advanced attempt descriptor missing; reconcile existing hash');
      return null;
    }
    if (
      value.version !== 1 ||
      value.chainId !== 125 ||
      value.account !== address(account) ||
      value.deployment !== this.deployment ||
      value.action !== action ||
      !Number.isSafeInteger(value.submittedAt) ||
      value.submittedAt > Date.now() ||
      value.submittedAt < 0
    )
      throw new Error('Advanced attempt identity mismatch');
    address(value.target);
    abiHex(value.calldata);
    if (value.calldata.length > 16386 || value.intentId !== this.identity(value))
      throw new Error('Advanced attempt intent mismatch');
    const records = stored.records.filter(
      (record) =>
        record.intentId === this.deployment + value.intentId.slice(2) &&
        record.action === `advanced:${action}`,
    );
    if (
      records.length > 1 ||
      records.some(
        (record) =>
          record.actionFingerprint !== value.intentId ||
          (value.hash !== undefined && record.txHash !== value.hash),
      )
    )
      throw new Error('Conflicting advanced attempts');
    const hash = value.hash ?? records[0]?.txHash;
    if (hash !== undefined) abiHex(hash, 32);
    return Object.freeze({ ...value, hash: hash as Hex | undefined });
  }

  private identity(value: Pick<AdvancedAttempt, 'account' | 'action' | 'target' | 'calldata'>) {
    return keccak256(
      stringToHex(
        JSON.stringify([
          125,
          this.deployment,
          value.account,
          value.action,
          value.target,
          value.calldata,
        ]),
      ),
    );
  }

  private persist(key: string, attempt: AdvancedAttempt & { hash: Hex }) {
    broadcasts.set(key, attempt);
    new PendingAttemptStore(this.storage, attempt.account).save({
      version: 1,
      chainId: 125,
      from: attempt.account,
      txHash: attempt.hash,
      submittedAt: attempt.submittedAt,
      intentId: this.deployment + attempt.intentId.slice(2),
      action: `advanced:${attempt.action}`,
      actionFingerprint: attempt.intentId,
    });
    this.storage.setItem(key, JSON.stringify(attempt));
    broadcasts.delete(key);
  }

  async run<T>(
    account: Address,
    action: AdvancedAction,
    target: Address,
    calldata: Hex,
    send: () => Promise<Hex>,
    read: (hash: Hex) => Promise<TransactionReceipt>,
    reconcile: (attempt: AdvancedAttempt, receipt: TransactionReceipt) => Promise<T>,
  ): Promise<T> {
    const key = this.key(account, action);
    if (dispatches.has(key))
      throw new Error('Advanced action already signing; resume existing attempt');
    dispatches.add(key);
    try {
      if (this.pending(account, action))
        throw new Error('Advanced action unresolved; resume existing hash');
      const submittedAt = Date.now();
      const attempt: AdvancedAttempt = {
        version: 1,
        chainId: 125,
        account,
        deployment: this.deployment,
        action,
        target,
        calldata,
        intentId: this.identity({ account, action, target, calldata }),
        submittedAt,
      };
      // A persisted signing reservation also fences a crash before the wallet returns its hash.
      const reservation = JSON.stringify(attempt);
      this.storage.setItem(key, reservation);
      let hash: Hex;
      try {
        hash = await send();
      } catch (error) {
        const confirmed =
          error instanceof BaseError &&
          error.walk(
            (cause) =>
              cause instanceof UserRejectedRequestError ||
              cause instanceof TransactionRejectedRpcError ||
              cause instanceof InvalidParamsRpcError,
          );
        try {
          if (confirmed) {
            const stored = new PendingAttemptStore(this.storage, account).load();
            if (
              !stored.corrupt &&
              this.storage.getItem(key) === reservation &&
              !broadcasts.has(key) &&
              !stored.records.some(
                (record) =>
                  record.action === `advanced:${action}` &&
                  record.intentId.startsWith(this.deployment),
              )
            )
              this.storage.removeItem(key);
          }
        } finally {
          throw error;
        }
      }
      this.persist(key, { ...attempt, hash });
      return await this.settle(account, action, read, reconcile);
    } finally {
      dispatches.delete(key);
    }
  }

  async recover<T>(
    account: Address,
    action: AdvancedAction,
    read: (hash: Hex) => Promise<TransactionReceipt>,
    reconcile: (attempt: AdvancedAttempt, receipt: TransactionReceipt) => Promise<T>,
    validate: (attempt: AdvancedAttempt) => Promise<void>,
    hash?: Hex,
  ): Promise<T> {
    const key = this.key(account, action);
    if (dispatches.has(key))
      throw new Error('Advanced action already signing; resume existing attempt');
    dispatches.add(key);
    try {
      const attempt = this.pending(account, action);
      if (!attempt) throw new Error('No advanced action to recover');
      if (hash !== undefined) {
        abiHex(hash, 32);
        if (attempt.hash && attempt.hash.toLowerCase() !== hash.toLowerCase())
          throw new Error('Conflicting advanced transaction hash');
        if (!attempt.hash) {
          const candidate = { ...attempt, hash: hash.toLowerCase() as Hex };
          await validate(candidate);
          const current = this.pending(account, action);
          if (!current || JSON.stringify(current) !== JSON.stringify(attempt))
            throw new Error('Advanced recovery reservation changed');
          this.persist(key, candidate);
        }
      }
      return await this.settle(account, action, read, reconcile);
    } finally {
      dispatches.delete(key);
    }
  }

  private async settle<T>(
    account: Address,
    action: AdvancedAction,
    read: (hash: Hex) => Promise<TransactionReceipt>,
    reconcile: (attempt: AdvancedAttempt, receipt: TransactionReceipt) => Promise<T>,
  ): Promise<T> {
    const key = this.key(account, action),
      attempt = this.pending(account, action);
    if (!attempt) throw new Error('No advanced action to recover');
    if (!attempt.hash)
      throw new Error('Signature outcome unknown; reconcile wallet before another submission');
    const receipt = await read(attempt.hash);
    const result = await reconcile(attempt, receipt);
    if (this.closed) throw new Error('Advanced provider closed; reopen to reconcile');
    new PendingAttemptStore(this.storage, account).remove(attempt.hash);
    this.storage.removeItem(key);
    broadcasts.delete(key);
    return result;
  }
}

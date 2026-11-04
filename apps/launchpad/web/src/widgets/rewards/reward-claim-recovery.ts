'use client';

import { useEffect, useRef, useState } from 'react';
import { decodeV2Contract } from '@kindle/sdk';
import { getAddress, type EIP1193Provider, type Hex } from 'viem';
import type { RewardClaimResponse } from '../../../../../../sdk/launchpad/packages/production-contracts/src/types.gen';
import type { RuntimeAuthority } from '@/core/runtime-authority';
import { getWalletClient, PaxeerWallet } from '@/core/wallet-sdk/client';
import type { WalletPublicClient } from '@/core/wallet-sdk/assets';
import { assertAccount, type AccountSession } from '../profile/account-transport';
import { normalizeAddress } from '../profile/account-disclosure';
import { validatedRewardClaim } from './reward-claim-state';

type RecoveryStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;
export type RewardClaimAuthority = Pick<RuntimeAuthority, 'revision' | 'expiresAt' | 'network'>;
export type RewardClaimClient = Pick<WalletPublicClient, 'getChainId' | 'getBlock' | 'getTransaction' | 'getTransactionReceipt'>;
export interface RewardClaimContext {
  scope: AccountSession | null;
  address?: string;
  chainId?: number;
  authority?: RewardClaimAuthority;
}
export interface RewardClaimRecord {
  version: 1;
  attemptId: string;
  owner: string;
  chainId: 125;
  authorityRevision: string;
  rewardId: string;
  prepared: RewardClaimResponse;
  hash?: Hex;
}
const hashPattern = /^0x[0-9a-fA-F]{64}$/;

export class RewardClaimRecovery {
  readonly key: string;
  private closed = false;

  constructor(
    private readonly storage: RecoveryStorage,
    private readonly scope: AccountSession,
    private readonly context: () => RewardClaimContext,
    private readonly client: RewardClaimClient,
  ) {
    this.key = `sidiora:reward-claim:1:125:${normalizeAddress(scope.session.wallet)}`;
  }

  private current(record?: RewardClaimRecord) {
    const context = this.context();
    assertAccount(this.scope);
    if (this.closed || context.scope !== this.scope || context.chainId !== 125 || !context.address || normalizeAddress(context.address) !== normalizeAddress(this.scope.session.wallet)) throw new Error('Reward claim account changed');
    const authority = context.authority;
    if (!authority || !Number.isFinite(authority.expiresAt) || authority.expiresAt <= Date.now() || authority.network.network.chainId !== 125 || !/^[0-9a-f]{64}$/.test(authority.revision.revisionSha256)) throw new Error('Reward deployment authority unavailable');
    const router = authority.network.network.addresses.FeesRouter;
    if (!router) throw new Error('Fee settlement authority unavailable');
    if (record) {
      if (record.chainId !== 125 || normalizeAddress(record.owner) !== normalizeAddress(this.scope.session.wallet) || record.authorityRevision !== authority.revision.revisionSha256) throw new Error('Saved reward claim authority changed');
      validatedRewardClaim(record.prepared.data.transactionRequest, this.scope.session.wallet, router);
    }
    return authority;
  }

  restore(): RewardClaimRecord | null {
    this.current();
    const text = this.storage.getItem(this.key);
    if (text === null) return null;
    if (text.length > 64 * 1024) throw new Error('Saved reward claim is invalid');
    const value: unknown = JSON.parse(text);
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Saved reward claim is invalid');
    const record = value as Record<string, unknown>;
    if (record.version !== 1 || typeof record.attemptId !== 'string' || !/^[0-9a-f-]{36}$/i.test(record.attemptId) || typeof record.owner !== 'string' || record.chainId !== 125 || typeof record.authorityRevision !== 'string' || typeof record.rewardId !== 'string' || !record.rewardId || (record.hash !== undefined && (typeof record.hash !== 'string' || !hashPattern.test(record.hash)))) throw new Error('Saved reward claim is invalid');
    const prepared = decodeV2Contract('RewardClaimResponse', record.prepared);
    const restored: RewardClaimRecord = { version: 1, attemptId: record.attemptId, owner: record.owner, chainId: 125, authorityRevision: record.authorityRevision, rewardId: record.rewardId, prepared, ...(record.hash === undefined ? {} : { hash: record.hash as Hex }) };
    this.current(restored);
    return restored;
  }

  private save(record: RewardClaimRecord) {
    const text = JSON.stringify(record);
    if (text.length > 64 * 1024) throw new Error('Saved reward claim is too large');
    this.storage.setItem(this.key, text);
    if (this.storage.getItem(this.key) !== text) throw new Error('Reward claim could not be saved');
  }

  async broadcast(rewardId: string, prepared: RewardClaimResponse, provider: EIP1193Provider): Promise<RewardClaimRecord> {
    const authority = this.current();
    if (this.restore()) throw new Error('Reconcile the saved reward claim before another attempt');
    const record: RewardClaimRecord = { version: 1, attemptId: crypto.randomUUID(), owner: normalizeAddress(this.scope.session.wallet), chainId: 125, authorityRevision: authority.revision.revisionSha256, rewardId, prepared: decodeV2Contract('RewardClaimResponse', prepared) };
    if (!rewardId) throw new Error('Reward identity required');
    this.current(record);
    const signer = new PaxeerWallet(provider);
    await signer.assertChain();
    if ((await signer.getAccounts())[0]?.toLowerCase() !== record.owner) throw new Error('Prepared account unavailable');
    this.current(record);
    if (this.restore()) throw new Error('Reconcile the saved reward claim before another attempt');
    // No await may separate the durable intent from invoking the wallet.
    this.save(record);
    const request = record.prepared.data.transactionRequest;
    const hash = await getWalletClient(provider).sendTransaction({
      account: getAddress(request.from), to: getAddress(request.to), data: request.data as Hex, value: BigInt(request.valueRaw),
      ...(request.gasLimitRaw === null ? {} : { gas: BigInt(request.gasLimitRaw) }),
      ...(request.maxFeePerGasRaw === null ? {} : { maxFeePerGas: BigInt(request.maxFeePerGasRaw) }),
      ...(request.maxPriorityFeePerGasRaw === null ? {} : { maxPriorityFeePerGas: BigInt(request.maxPriorityFeePerGasRaw) }),
    });
    if (!hashPattern.test(hash)) throw new Error('Wallet returned an invalid transaction hash');
    // Save a late wallet response even after unmount/sign-out; never put it under the new account.
    const saved: unknown = JSON.parse(this.storage.getItem(this.key) ?? 'null');
    if (!saved || typeof saved !== 'object' || !('attemptId' in saved) || saved.attemptId !== record.attemptId) throw new Error('Saved reward claim changed during broadcast');
    const pending = { ...record, hash };
    this.save(pending);
    this.current(pending);
    return pending;
  }

  async reconcile(explicitHash?: string): Promise<{ record: RewardClaimRecord; outcome: 'confirmed' | 'reverted' }> {
    const record = this.restore();
    if (!record) throw new Error('No saved reward claim');
    const hash = record.hash ?? explicitHash;
    if (!hash || !hashPattern.test(hash)) throw new Error('Transaction hash required');
    const authority = this.current(record);
    if (await this.client.getChainId() !== 125) throw new Error('Reward claim RPC is not chain 125');
    this.current(record);
    const head = await authority.network.committedHead(this.scope.signal);
    this.current(record);
    if (head.number < 0n || !hashPattern.test(head.hash)) throw new Error('Committed reward head unavailable');
    if ((await this.client.getBlock({ blockNumber: head.number })).hash?.toLowerCase() !== head.hash.toLowerCase()) throw new Error('Committed reward head is not canonical');
    const transaction = await this.client.getTransaction({ hash: hash as Hex });
    this.current(record);
    const request = record.prepared.data.transactionRequest;
    if (transaction.chainId !== 125 || transaction.hash.toLowerCase() !== hash.toLowerCase() || normalizeAddress(transaction.from) !== normalizeAddress(request.from) || !transaction.to || normalizeAddress(transaction.to) !== normalizeAddress(request.to) || transaction.input.toLowerCase() !== request.data.toLowerCase() || transaction.value !== BigInt(request.valueRaw) || request.gasLimitRaw !== null && transaction.gas !== BigInt(request.gasLimitRaw) || request.maxFeePerGasRaw !== null && transaction.maxFeePerGas !== BigInt(request.maxFeePerGasRaw) || request.maxPriorityFeePerGasRaw !== null && transaction.maxPriorityFeePerGas !== BigInt(request.maxPriorityFeePerGasRaw)) throw new Error('Transaction does not match the saved reward claim');
    // Only bind an explicitly supplied hash after checking the entire saved request.
    const pending: RewardClaimRecord = { ...record, hash: hash as Hex };
    const existing = this.restore();
    if (existing?.attemptId !== record.attemptId || existing.hash !== record.hash) throw new Error('Saved reward claim changed');
    this.save(pending);
    const receipt = await this.client.getTransactionReceipt({ hash: hash as Hex });
    this.current(pending);
    if (receipt.transactionHash.toLowerCase() !== hash.toLowerCase() || normalizeAddress(receipt.from) !== normalizeAddress(request.from) || !receipt.to || normalizeAddress(receipt.to) !== normalizeAddress(request.to) || receipt.blockNumber > head.number || receipt.blockNumber < 0n || transaction.blockHash?.toLowerCase() !== receipt.blockHash.toLowerCase() || transaction.blockNumber !== receipt.blockNumber || transaction.transactionIndex !== receipt.transactionIndex || !['success', 'reverted'].includes(receipt.status)) throw new Error('Reward receipt identity or commitment mismatch');
    const block = await this.client.getBlock({ blockNumber: receipt.blockNumber });
    const included = block.transactions[receipt.transactionIndex];
    if (block.hash?.toLowerCase() !== receipt.blockHash.toLowerCase() || !Number.isSafeInteger(receipt.transactionIndex) || receipt.transactionIndex < 0 || !included || included.toLowerCase() !== hash.toLowerCase()) throw new Error('Reward receipt is not canonical');
    if ((await this.client.getBlock({ blockNumber: head.number })).hash?.toLowerCase() !== head.hash.toLowerCase()) throw new Error('Committed reward head changed');
    this.current(pending);
    const final = this.restore();
    if (final?.attemptId !== pending.attemptId || final.hash?.toLowerCase() !== hash.toLowerCase()) throw new Error('Saved reward claim changed');
    this.storage.removeItem(this.key);
    return { record: pending, outcome: receipt.status === 'success' ? 'confirmed' : 'reverted' };
  }

  close() { this.closed = true; }
}

interface RecoveryState {
  scope: AccountSession;
  address: string;
  chainId: number;
  revision: string;
  router: string;
  client: RewardClaimClient;
  recovery?: RewardClaimRecovery;
  record?: RewardClaimRecord | null;
  error?: string;
}

export function useRewardClaimRecovery(context: RewardClaimContext, client: RewardClaimClient | undefined) {
  const latest = useRef(context);
  latest.current = context;
  const { scope, address, chainId, authority } = context;
  const revision = authority?.revision.revisionSha256;
  const available = !!authority && authority.expiresAt > Date.now();
  const router = authority?.network.network.addresses.FeesRouter;
  const [state, setState] = useState<RecoveryState>();
  useEffect(() => {
    if (!scope || !address || chainId !== 125 || !client || !revision || !router || !available) return;
    let recovery: RewardClaimRecovery | undefined;
    const identity = { scope, address, chainId, revision, router, client };
    try {
      recovery = new RewardClaimRecovery(window.localStorage, scope, () => latest.current, client);
      const record = recovery.restore();
      setState({ ...identity, recovery, record });
    } catch {
      recovery?.close();
      setState({ ...identity, error: "Your saved claim can't be checked right now. Reconnect the same wallet and network before trying again" });
    }
    return () => recovery?.close();
  }, [scope, address, chainId, client, revision, router, available]);
  const current = state?.scope === scope && state?.address === address && state?.chainId === chainId && state?.revision === revision && state?.router === router && state?.client === client && available ? state : undefined;
  return { recovery: current?.recovery, record: current?.record, error: current?.error, ready: !!current?.recovery };
}

// @vitest-environment jsdom
import { act, createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { encodeFunctionData, getAddress, keccak256, stringToHex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import FeesRouter from '@/core/network/abis/FeesRouter.json';
import { accountEpoch, clearAccountSession, installAccountSession, requireAccount, type AccountSession } from '../profile/account-transport';
import { prepareRewardClaim } from './rewards-view';
import { RewardClaimRecovery, useRewardClaimRecovery, type RewardClaimContext } from './reward-claim-recovery';
import { rewardClaimFixture, type RewardClaimFixture } from './reward-claim.fixture';

let fixture: RewardClaimFixture;
let scope: AccountSession;
let context: RewardClaimContext;
const roots = new Set<Root>();
const previousOrigin = process.env.NEXT_PUBLIC_PNL_API_URL;
const other = getAddress('0x1111111111111111111111111111111111111111');

function signIn(owner: string, id: string) {
  installAccountSession({
    wallet: owner, sessionId: id, deviceLabel: 'Web browser',
    accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(),
    refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(),
    revokedAt: null, scopes: ['account'],
  }, 'reward-recovery-access', accountEpoch());
  return requireAccount(owner);
}

beforeEach(async () => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  window.localStorage.clear();
  fixture = await rewardClaimFixture();
  process.env.NEXT_PUBLIC_PNL_API_URL = fixture.origin;
  scope = signIn(fixture.owner, crypto.randomUUID());
  context = { scope, address: fixture.owner, chainId: 125, authority: fixture.authority };
});

afterEach(async () => {
  for (const root of roots) await act(async () => root.unmount());
  roots.clear();
  clearAccountSession(false);
  if (previousOrigin === undefined) delete process.env.NEXT_PUBLIC_PNL_API_URL;
  else process.env.NEXT_PUBLIC_PNL_API_URL = previousOrigin;
  await fixture.close();
  expect(fixture.errors).toEqual([]);
  document.body.replaceChildren();
});

function recovery() {
  return new RewardClaimRecovery(window.localStorage, scope, () => context, fixture.client);
}

async function openComponent() {
  let state: ReturnType<typeof useRewardClaimRecovery> | undefined;
  function RecoveryComponent() {
    state = useRewardClaimRecovery(context, fixture.client);
    return createElement('output', { 'data-ready': state.ready }, state.record?.hash ?? state.error ?? '');
  }
  const element = document.createElement('div');
  document.body.append(element);
  const root = createRoot(element);
  roots.add(root);
  await act(async () => root.render(createElement(RecoveryComponent)));
  return {
    get state() { expect(state).toBeDefined(); return state!; },
    element,
    async render() { await act(async () => root.render(createElement(RecoveryComponent))); },
    async stop() { await act(async () => root.unmount()); roots.delete(root); element.remove(); },
  };
}

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>(done => { resolve = done; });
  return { promise, resolve };
}

async function reward_claim_recovery_reopens_with_saved_hash_and_committed_result() {
  const first = await openComponent();
  expect(first.state.ready).toBe(true);
  const producer = await prepareRewardClaim('creator:1', scope);
  expect(fixture.requests).toEqual([{
    path: '/v2/me/rewards:prepare-claim', body: { recipient: fixture.owner, rewardIds: ['creator:1'] },
    authorization: 'Bearer reward-recovery-access',
  }]);
  const original = first.state.recovery!;
  fixture.state.onSend = () => {
    const saved = original.restore();
    expect(saved).toMatchObject({ owner: fixture.owner.toLowerCase(), chainId: 125, rewardId: 'creator:1', prepared: producer });
    expect(saved?.hash).toBeUndefined();
  };
  const sent = await original.broadcast('creator:1', producer, fixture.provider);
  expect(sent.hash).toBe(fixture.lastHash);
  const raw = window.localStorage.getItem(original.key);
  expect(raw).not.toContain('reward-recovery-access');
  await first.stop();
  clearAccountSession(false);
  scope = signIn(fixture.owner, crypto.randomUUID());
  context = { ...context, scope };
  const reopened = await openComponent();
  expect(reopened.state.ready).toBe(true);
  expect(reopened.state.record).toEqual(sent);
  expect(reopened.element.textContent).toBe(fixture.lastHash);
  expect(reopened.state.recovery).not.toBe(original);
  await expect(original.reconcile()).rejects.toThrow();
  const active = reopened.state.recovery!;
  await expect(active.broadcast('creator:1', producer, fixture.provider)).rejects.toThrow('Reconcile');
  const result = await active.reconcile();
  expect(result).toEqual({ record: sent, outcome: 'confirmed' });
  expect(window.localStorage.getItem(active.key)).toBeNull();
  expect(fixture.provider.requests.filter(row => row.method === 'eth_sendTransaction')).toHaveLength(1);
  expect(fixture.provider.requests.filter(row => row.method === 'eth_getBlockByNumber')).toHaveLength(3);
}

async function reward_claim_recovery_unknown_broadcast_requires_explicit_matching_hash() {
  const first = await openComponent();
  const active = first.state.recovery!;
  const producer = await prepareRewardClaim('creator:1', scope);
  fixture.state.loseWalletResponse = true;
  await expect(active.broadcast('creator:1', producer, fixture.provider)).rejects.toThrow();
  const preserved = window.localStorage.getItem(active.key);
  expect(active.restore()?.hash).toBeUndefined();
  await first.stop();
  const reopened = await openComponent();
  const restored = reopened.state.recovery!;
  expect(reopened.state.record?.prepared).toEqual(producer);
  expect(reopened.state.record?.hash).toBeUndefined();
  await expect(restored.reconcile()).rejects.toThrow('Transaction hash required');
  await expect(restored.broadcast('creator:1', producer, fixture.provider)).rejects.toThrow('Reconcile');
  const wrongRequests = [
    { to: other },
    { data: encodeFunctionData({ abi: FeesRouter, functionName: 'claimFees', args: [2n] }) },
    { value: 1n },
    { chainId: 1 },
    { account: privateKeyToAccount(`0x${'02'.repeat(32)}`) },
  ];
  for (const change of wrongRequests) {
    const wrong = await fixture.otherTransaction(change);
    await expect(restored.reconcile(wrong)).rejects.toThrow('Transaction does not match');
    expect(window.localStorage.getItem(active.key)).toBe(preserved);
  }
  const result = await restored.reconcile(fixture.lastHash);
  expect(result.outcome).toBe('confirmed');
  expect(result.record.hash).toBe(fixture.lastHash);
  expect(fixture.provider.requests.filter(row => row.method === 'eth_sendTransaction')).toHaveLength(1);
}

async function reward_claim_recovery_keeps_late_hash_after_component_stops() {
  const first = await openComponent();
  const active = first.state.recovery!;
  const started = deferred();
  const response = deferred();
  fixture.state.onSend = started.resolve;
  fixture.state.walletResponse = response.promise;
  const producer = await prepareRewardClaim('creator:1', scope);
  const pending = active.broadcast('creator:1', producer, fixture.provider).then(
    () => { throw new Error('Closed component must not publish a broadcast result'); },
    error => error as Error,
  );
  await started.promise;
  expect(active.restore()?.hash).toBeUndefined();
  await first.stop();
  response.resolve();
  expect(await pending).toBeInstanceOf(Error);
  const reopened = await openComponent();
  expect(reopened.state.record?.hash).toBe(fixture.lastHash);
  expect(reopened.state.record?.prepared).toEqual(producer);
  expect(fixture.provider.requests.filter(row => row.method === 'eth_sendTransaction')).toHaveLength(1);
}

async function reward_claim_recovery_revalidates_account_chain_authority_and_storage() {
  const active = recovery();
  await active.broadcast('creator:1', await prepareRewardClaim('creator:1', scope), fixture.provider);
  const raw = window.localStorage.getItem(active.key)!;
  const first = await openComponent();
  expect(first.state.record?.hash).toBe(fixture.lastHash);
  context = { ...context, address: other };
  await first.render();
  expect(first.state.ready).toBe(false);
  expect(first.state.record).toBeUndefined();
  expect(first.element.textContent).not.toContain(fixture.lastHash);
  await expect(active.reconcile()).rejects.toThrow('account changed');
  context = { ...context, address: fixture.owner, chainId: 1 };
  await first.render();
  expect(first.state.ready).toBe(false);
  await expect(active.reconcile()).rejects.toThrow('account changed');
  context = { ...context, chainId: 125, authority: { ...fixture.authority, expiresAt: Date.now() - 1 } };
  await first.render();
  expect(first.state.ready).toBe(false);
  await expect(active.reconcile()).rejects.toThrow('authority unavailable');
  context = { ...context, authority: { ...fixture.authority, revision: { ...fixture.authority.revision, revisionSha256: 'a'.repeat(64) } } };
  await first.render();
  expect(first.state.ready).toBe(false);
  expect(first.state.error).toBeTruthy();
  await expect(active.reconcile()).rejects.toThrow('authority changed');
  context = { ...context, authority: { ...fixture.authority, network: { ...fixture.authority.network, network: { ...fixture.authority.network.network, addresses: { FeesRouter: other } } } } };
  await first.render();
  expect(first.state.ready).toBe(false);
  expect(first.state.record).toBeUndefined();
  await expect(active.reconcile()).rejects.toThrow('authority mismatch');
  context = { ...context, authority: undefined };
  await first.render();
  expect(first.state.ready).toBe(false);
  expect(window.localStorage.getItem(active.key)).toBe(raw);
  await first.stop();
  context = { ...context, authority: fixture.authority };
  clearAccountSession(false);
  const otherScope = signIn(other, crypto.randomUUID());
  context = { ...context, scope: otherScope, address: other };
  const otherComponent = await openComponent();
  expect(otherComponent.state.ready).toBe(true);
  expect(otherComponent.state.record).toBeNull();
  expect(window.localStorage.getItem(active.key)).toBe(raw);
  await expect(active.reconcile()).rejects.toThrow();
  await otherComponent.stop();
  scope = signIn(fixture.owner, crypto.randomUUID());
  context = { ...context, scope, address: fixture.owner };
  window.localStorage.setItem(active.key, '{corrupt');
  const corrupt = await openComponent();
  expect(corrupt.state.ready).toBe(false);
  expect(corrupt.state.error).toBeTruthy();
  expect(window.localStorage.getItem(active.key)).toBe('{corrupt');
  await expect(recovery().broadcast('creator:1', fixture.prepared, fixture.provider)).rejects.toThrow();
  expect(fixture.provider.requests.filter(row => row.method === 'eth_sendTransaction')).toHaveLength(1);
}

async function reward_claim_recovery_rejects_uncommitted_or_mismatched_receipts() {
  const active = recovery();
  const pending = await active.broadcast('creator:1', await prepareRewardClaim('creator:1', scope), fixture.provider);
  const { transaction, receipt } = fixture.entry;
  const stored = window.localStorage.getItem(active.key);
  const wrongHash = keccak256(stringToHex('foreign transaction or fork'));
  const faults: [Record<string, unknown>, string, unknown][] = [
    [transaction, 'hash', wrongHash],
    [transaction, 'chainId', '0x1'],
    [transaction, 'from', other],
    [transaction, 'to', other],
    [transaction, 'input', '0x'],
    [transaction, 'value', '0x1'],
    [transaction, 'gas', '0x1'],
    [receipt, 'transactionHash', wrongHash],
    [receipt, 'from', other],
    [receipt, 'to', other],
    [receipt, 'blockHash', wrongHash],
    [receipt, 'blockNumber', '0x2a'],
    [receipt, 'transactionIndex', '0x1'],
  ];
  for (const [target, key, value] of faults) {
    const before = target[key];
    target[key] = value;
    await expect(active.reconcile()).rejects.toThrow();
    expect(window.localStorage.getItem(active.key)).toBe(stored);
    target[key] = before;
  }
  fixture.state.chainId = '0x1';
  await expect(active.reconcile()).rejects.toThrow('not chain 125');
  fixture.state.chainId = '0x7d';
  const originalHead = { ...fixture.state.head };
  fixture.state.head.number = (BigInt(originalHead.number) - 1n).toString();
  await expect(active.reconcile()).rejects.toThrow('commitment mismatch');
  fixture.state.head = originalHead;
  fixture.state.canonicalHash = wrongHash;
  await expect(active.reconcile()).rejects.toThrow('not canonical');
  fixture.state.canonicalHash = originalHead.hash;
  expect(window.localStorage.getItem(active.key)).toBe(stored);
  active.close();
  const reopened = recovery();
  expect(reopened.restore()).toEqual(pending);
  expect((await reopened.reconcile()).outcome).toBe('confirmed');
}

async function reward_claim_recovery_preserves_record_when_account_changes_during_receipt_read() {
  const active = recovery();
  await active.broadcast('creator:1', await prepareRewardClaim('creator:1', scope), fixture.provider);
  const stored = window.localStorage.getItem(active.key);
  const started = deferred();
  const response = deferred();
  fixture.state.onReceipt = started.resolve;
  fixture.state.receiptResponse = response.promise;
  const pending = active.reconcile().then(
    () => { throw new Error('Changed account must not accept the receipt'); },
    error => error as Error,
  );
  await started.promise;
  clearAccountSession(false);
  response.resolve();
  expect(await pending).toBeInstanceOf(Error);
  expect(window.localStorage.getItem(active.key)).toBe(stored);
}

async function reward_claim_recovery_reopens_reverted_transaction_without_rebroadcast() {
  fixture.state.receiptStatus = '0x0';
  const active = recovery();
  const saved = await active.broadcast('creator:1', await prepareRewardClaim('creator:1', scope), fixture.provider);
  active.close();
  const reopened = await openComponent();
  expect(reopened.state.record).toEqual(saved);
  expect((await reopened.state.recovery!.reconcile()).outcome).toBe('reverted');
  expect(window.localStorage.getItem(active.key)).toBeNull();
  expect(fixture.provider.requests.filter(row => row.method === 'eth_sendTransaction')).toHaveLength(1);
}

async function reward_claim_recovery_requires_durable_storage_before_wallet_invocation() {
  const active = recovery();
  const producer = await prepareRewardClaim('creator:1', scope);
  window.localStorage.setItem('quota', 'x'.repeat(4_999_900));
  await expect(active.broadcast('creator:1', producer, fixture.provider)).rejects.toMatchObject({ name: 'QuotaExceededError' });
  expect(window.localStorage.getItem(active.key)).toBeNull();
  expect(fixture.provider.requests.filter(row => row.method === 'eth_sendTransaction')).toHaveLength(0);
}

test(reward_claim_recovery_reopens_with_saved_hash_and_committed_result.name, reward_claim_recovery_reopens_with_saved_hash_and_committed_result);
test(reward_claim_recovery_unknown_broadcast_requires_explicit_matching_hash.name, reward_claim_recovery_unknown_broadcast_requires_explicit_matching_hash);
test(reward_claim_recovery_keeps_late_hash_after_component_stops.name, reward_claim_recovery_keeps_late_hash_after_component_stops);
test(reward_claim_recovery_revalidates_account_chain_authority_and_storage.name, reward_claim_recovery_revalidates_account_chain_authority_and_storage);
test(reward_claim_recovery_rejects_uncommitted_or_mismatched_receipts.name, reward_claim_recovery_rejects_uncommitted_or_mismatched_receipts);
test(reward_claim_recovery_preserves_record_when_account_changes_during_receipt_read.name, reward_claim_recovery_preserves_record_when_account_changes_during_receipt_read);
test(reward_claim_recovery_reopens_reverted_transaction_without_rebroadcast.name, reward_claim_recovery_reopens_reverted_transaction_without_rebroadcast);
test(reward_claim_recovery_requires_durable_storage_before_wallet_invocation.name, reward_claim_recovery_requires_durable_storage_before_wallet_invocation);

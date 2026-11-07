import { createRequire } from 'node:module';
import { resolveObjectURL } from 'node:buffer';
import { act, createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { gatewayUrl } from '@/core/sdk-config';
import { getPublicClient } from '@/core/wallet-sdk/client';
import { accountEpoch, clearAccountSession, installAccountSession } from '@/widgets/profile/account-transport';
import { PendingAttemptStore } from '@/widgets/trading/transaction-controls/recovery';
import { useBountyAttemptRecovery, useEvidenceUploads } from './use-bounty-mutations';
import { BountyTransactionStatus } from './funding-step';
import { BountyTransactionAttempt, bountyIntentId, freezeBountyReview, sendReviewedBountyTransaction } from './bounty-transaction-attempt';
import { createFundingIntent, createSettlementIntent, validatePreparedEnvelope, type ValidatedTransaction } from './prepared-transaction';
import { BountyAssetGateway, BountyRpcTranscript, transcriptProvider, bounty, creator, winner, deployment, broadcastHash, receiptTranscript, preparedTokenRequest, selection } from './repair-22-33-985.fixtures';
// The focused gate also executes the updated preparation-to-review refusal suite.
import './prepared-transaction.test';

const dom = new (createRequire(import.meta.url)('jsdom').JSDOM)('', { url: 'http://localhost' });
const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
const originalNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');
const roots = new Set<Root>();
const assetGateway = new BountyAssetGateway();

beforeAll(async () => {
  Object.defineProperty(globalThis, 'window', { configurable: true, value: dom.window });
  Object.defineProperty(globalThis, 'document', { configurable: true, value: dom.window.document });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  await assetGateway.open(gatewayUrl(''));
});
afterEach(() => {
  act(() => { for (const root of roots) root.unmount(); clearAccountSession(false); });
  roots.clear();
  dom.window.localStorage.clear();
  assetGateway.release();
  assetGateway.calls.splice(0);
  assetGateway.assets.clear();
  dom.window.document.body.replaceChildren();
});
afterAll(async () => {
  await assetGateway.close();
  dom.window.close();
  for (const [name, descriptor] of [['window', originalWindow], ['document', originalDocument], ['navigator', originalNavigator]] as const) {
    if (descriptor) Object.defineProperty(globalThis, name, descriptor);
    else Reflect.deleteProperty(globalThis, name);
  }
});

function install(wallet: string) {
  installAccountSession({ sessionId: crypto.randomUUID(), wallet, accessExpiresAt: new Date(Date.now() + 600000).toISOString(), refreshExpiresAt: new Date(Date.now() + 86400000).toISOString(), revokedAt: null, deviceLabel: 'test browser', scopes: [] }, `fixture:${wallet}`, accountEpoch());
}

function renderHook<T>(read: () => T) {
  let current: T;
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  roots.add(root);
  function Probe() { current = read(); return null; }
  act(() => root.render(createElement(Probe)));
  return { get value() { return current; }, container, close() { act(() => root.unmount()); roots.delete(root); container.remove(); } };
}

async function until(condition: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!condition()) {
    if (Date.now() >= deadline) throw new Error('Bounty fixture condition did not arrive');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
}

function evidence_crosses_session_clears_previews_approvals_and_pending_uploads() {
  return (async () => {
    install(creator);
    const hook = renderHook(useEvidenceUploads);
    assetGateway.hold = 'PUT';
    act(() => { expect(hook.value.add([new File(['first evidence'], 'first.png', { type: 'image/png' })])).toBeNull(); });
    const oldId = hook.value.items[0].id;
    const preview = hook.value.items[0].previewUrl;
    expect(resolveObjectURL(preview)).toBeDefined();
    await until(() => assetGateway.calls.some(call => call.id === oldId));
    act(() => install(winner));
    expect(hook.value.items).toEqual([]);
    expect(hook.value.evidenceUrls).toEqual([]);
    expect(resolveObjectURL(preview)).toBeUndefined();
    assetGateway.approve(oldId);
    await act(async () => { assetGateway.release(); await new Promise(resolve => setTimeout(resolve, 20)); });
    expect(hook.value.items).toEqual([]);
    expect(hook.value.evidenceUrls).toEqual([]);
    act(() => { hook.value.add([new File(['new account'], 'new.png', { type: 'image/png' })]); });
    await until(() => hook.value.items[0]?.status === 'review');
    const newId = hook.value.items[0].id;
    expect(assetGateway.assets.get(newId)?.ownerWallet).toBe(winner);
    assetGateway.approve(newId);
    await act(async () => hook.value.refresh(newId));
    expect(hook.value.evidenceUrls).toEqual([assetGateway.assets.get(newId)!.publicUrl]);
    const newPreview = hook.value.items[0].previewUrl;
    act(() => clearAccountSession(false));
    expect(hook.value.evidenceUrls).toEqual([]);
    expect(hook.value.items).toEqual([]);
    expect(resolveObjectURL(newPreview)).toBeUndefined();
  })();
}

async function evidence_crosses_session_fences_moderation_refresh_and_same_wallet_relogin() {
  install(creator);
  const hook = renderHook(useEvidenceUploads);
  act(() => { hook.value.add([new File(['moderation'], 'review.png', { type: 'image/png' })]); });
  await until(() => hook.value.items[0]?.status === 'review');
  const id = hook.value.items[0].id;
  const preview = hook.value.items[0].previewUrl;
  assetGateway.hold = 'GET';
  let refresh!: Promise<void>;
  act(() => { refresh = hook.value.refresh(id); });
  await until(() => assetGateway.calls.some(call => call.method === 'GET'));
  act(() => install(winner));
  assetGateway.approve(id);
  await act(async () => { assetGateway.release(); await refresh; });
  expect(hook.value.items).toEqual([]);
  expect(hook.value.evidenceUrls).toEqual([]);
  expect(resolveObjectURL(preview)).toBeUndefined();
  await act(async () => hook.value.refresh(id));
  expect(assetGateway.calls.filter(call => call.method === 'GET')).toEqual([{ method: 'GET', wallet: creator, id }]);
  act(() => { hook.value.add([new File(['winner'], 'winner.png', { type: 'image/png' })]); });
  await until(() => hook.value.items[0]?.status === 'review');
  const nextPreview = hook.value.items[0].previewUrl;
  act(() => install(winner));
  expect(hook.value.items).toEqual([]);
  expect(resolveObjectURL(nextPreview)).toBeUndefined();
}

async function reviewed_fee_not_enforced_refuses_every_changed_or_omitted_signing_field() {
  const transcript = new BountyRpcTranscript();
  const client = getPublicClient(await transcript.open());
  const provider = transcriptProvider(client);
  try {
    const request = preparedTokenRequest({ maxPriorityFeePerGasRaw: '100000000' });
    const tx = validatePreparedEnvelope(request, { wallet: creator, deployment });
    const review = freezeBountyReview(tx, tx.gas!, tx.maxFeePerGas!);
    const changes: Partial<ValidatedTransaction>[] = [
      { gas: null }, { gas: review.gas + 1n }, { maxFeePerGas: null }, { maxFeePerGas: review.feePerGas + 1n },
      { maxPriorityFeePerGas: null }, { maxPriorityFeePerGas: 999999999n }, { value: 1n }, { from: winner }, { to: deployment.Router }, { data: '0xdeadbeef' },
    ];
    for (const change of changes) await expect(sendReviewedBountyTransaction(review, { ...tx, ...change }, provider)).rejects.toThrow('review again');
    expect(transcript.calls).toEqual([]);
    expect(Object.isFrozen(review)).toBe(true);
    expect(Object.isFrozen(review.tx)).toBe(true);
    tx.maxFeePerGas = 9_000_000_000n;
    expect(review.tx.maxFeePerGas).toBe(1_000_000_000n);
    await expect(sendReviewedBountyTransaction(review, tx, provider)).rejects.toThrow('review again');
    expect(transcript.calls).toEqual([]);
  } finally { await transcript.close(); }
}

async function reviewed_fee_not_enforced_sends_exact_estimated_caps_when_prepared_fields_are_null() {
  const transcript = new BountyRpcTranscript();
  const client = getPublicClient(await transcript.open());
  const provider = transcriptProvider(client);
  try {
    const tx = validatePreparedEnvelope(preparedTokenRequest({ gasLimitRaw: null, maxFeePerGasRaw: null, maxPriorityFeePerGasRaw: null }), { wallet: creator, deployment });
    const gas = await client.estimateGas({ account: tx.from, to: tx.to, data: tx.data, value: tx.value });
    const fee = await client.getGasPrice();
    const review = freezeBountyReview(tx, gas, fee);
    expect(review.costRaw).toBe(65_000n * 1_000_000_000n);
    expect(await sendReviewedBountyTransaction(review, tx, provider)).toBe(broadcastHash);
    const sends = transcript.calls.filter(call => call.method === 'eth_sendTransaction');
    expect(sends).toHaveLength(1);
    expect(sends[0].params[0]).toMatchObject({ from: tx.from, to: tx.to, gas: '0xfde8', maxFeePerGas: '0x3b9aca00', maxPriorityFeePerGas: '0x0', data: tx.data, value: '0x0' });
    expect(BigInt((sends[0].params[0] as { gas: string }).gas) * BigInt((sends[0].params[0] as { maxFeePerGas: string }).maxFeePerGas)).toBe(review.costRaw);
    expect(() => freezeBountyReview({ ...tx, maxPriorityFeePerGas: fee + 1n }, gas, fee)).toThrow('Priority fee');
  } finally { await transcript.close(); }
}

async function recoverAfterReload(status: 'success' | 'reverted') {
  const transcript = new BountyRpcTranscript();
  const client = getPublicClient(await transcript.open());
  const provider = transcriptProvider(client);
  const intent = status === 'success' ? createFundingIntent(bounty) : createSettlementIntent(bounty, selection);
  const tx = validatePreparedEnvelope(preparedTokenRequest(), { wallet: creator, deployment });
  let hook = renderHook(() => useBountyAttemptRecovery(creator, intent, client));
  try {
    const review = freezeBountyReview(tx, tx.gas!, tx.maxFeePerGas!);
    let hash!: string;
    await act(async () => { hash = await hook.value.attempt!.broadcastReviewed(review, tx, provider); });
    expect(hash).toBe(broadcastHash);
    const loaded = new PendingAttemptStore(dom.window.localStorage, creator).load();
    expect(loaded.corrupt).toBe(false);
    expect(loaded.records).toHaveLength(1);
    expect(loaded.records[0]).toMatchObject({ from: creator, chainId: 125, txHash: broadcastHash, intentId: bountyIntentId(intent), action: `bounty-${intent.action}` });
    hook.close();
    transcript.receiptFailure = true;
    hook = renderHook(() => useBountyAttemptRecovery(creator, intent, client));
    await until(() => hook.value.state?.kind === 'uncertain');
    expect(hook.value.state).toMatchObject({ hash: broadcastHash, kind: 'uncertain' });
    expect(() => hook.value.attempt!.assertCanSubmit()).toThrow('existing bounty transaction');
    expect(new PendingAttemptStore(dom.window.localStorage, winner).load().records).toEqual([]);
    expect(transcript.calls.filter(call => call.method === 'eth_sendTransaction')).toHaveLength(1);
    hook.close();
    transcript.receiptFailure = false;
    transcript.receipt = receiptTranscript(status);
    hook = renderHook(() => useBountyAttemptRecovery(creator, intent, client));
    await until(() => hook.value.state?.kind === (status === 'success' ? 'confirmed' : 'reverted'));
    expect(hook.value.state?.hash).toBe(broadcastHash);
    expect(() => hook.value.attempt!.assertCanSubmit()).toThrow('existing bounty transaction');
    expect(transcript.calls.filter(call => call.method === 'eth_sendTransaction')).toHaveLength(1);
    if (status === 'reverted') {
      const container = document.createElement('div');
      const root = createRoot(container); roots.add(root);
      act(() => root.render(createElement(BountyTransactionStatus, { state: hook.value.state!, recoveryHref: `/bounties/${bounty.bountyId}` })));
      expect(container.textContent).toContain('Funding or payout reverted');
      expect(container.textContent).toContain('network gas may have been charged');
      expect(container.textContent).toContain(broadcastHash);
      expect(container.querySelector('a')?.getAttribute('href')).toBe(`/bounties/${bounty.bountyId}`);
      expect(container.textContent).not.toContain('nothing was sent');
    }
  } finally { hook.close(); await transcript.close(); }
}

async function bounty_broadcast_not_persisted_recovers_success_after_reload_and_timeout() { await recoverAfterReload('success'); }
async function revert_fee_message_and_bounty_broadcast_recover_reverted_receipt() { await recoverAfterReload('reverted'); }

async function bounty_broadcast_not_persisted_saves_hash_even_when_component_closes_during_wallet_prompt() {
  const transcript = new BountyRpcTranscript();
  const client = getPublicClient(await transcript.open());
  const provider = transcriptProvider(client);
  const intent = createFundingIntent(bounty);
  const tx = validatePreparedEnvelope(preparedTokenRequest(), { wallet: creator, deployment });
  const attempt = new BountyTransactionAttempt(dom.window.localStorage, creator, intent, client);
  try {
    const pending = attempt.broadcastReviewed(freezeBountyReview(tx, tx.gas!, tx.maxFeePerGas!), tx, provider);
    attempt.close();
    expect(await pending).toBe(broadcastHash);
    transcript.receipt = receiptTranscript('success');
    const hook = renderHook(() => useBountyAttemptRecovery(creator, intent, client));
    await until(() => hook.value.state?.kind === 'confirmed');
    expect(hook.value.state?.hash).toBe(broadcastHash);
    expect(transcript.calls.filter(call => call.method === 'eth_sendTransaction')).toHaveLength(1);
  } finally { attempt.close(); await transcript.close(); }
}

async function bounty_broadcast_not_persisted_storage_failure_preserves_hash_and_blocks_resubmission() {
  const { JSDOM } = createRequire(import.meta.url)('jsdom');
  const quota = new JSDOM('', { url: 'https://quota.example.test', storageQuota: 0 });
  const transcript = new BountyRpcTranscript();
  const client = getPublicClient(await transcript.open());
  const provider = transcriptProvider(client);
  const tx = validatePreparedEnvelope(preparedTokenRequest(), { wallet: creator, deployment });
  const attempt = new BountyTransactionAttempt(quota.window.localStorage, creator, createFundingIntent(bounty), client);
  try {
    await expect(attempt.broadcastReviewed(freezeBountyReview(tx, tx.gas!, tx.maxFeePerGas!), tx, provider)).rejects.toThrow('could not be persisted');
    expect(attempt.state).toMatchObject({ kind: 'uncertain', hash: broadcastHash });
    expect(attempt.state && 'error' in attempt.state ? attempt.state.error : '').toContain('storage failed');
    expect(() => attempt.assertCanSubmit()).toThrow('existing bounty transaction');
    expect(transcript.calls.filter(call => call.method === 'eth_sendTransaction')).toHaveLength(1);
  } finally { attempt.close(); quota.window.close(); await transcript.close(); }
}

test(evidence_crosses_session_clears_previews_approvals_and_pending_uploads.name, evidence_crosses_session_clears_previews_approvals_and_pending_uploads);
test(evidence_crosses_session_fences_moderation_refresh_and_same_wallet_relogin.name, evidence_crosses_session_fences_moderation_refresh_and_same_wallet_relogin);
test(reviewed_fee_not_enforced_refuses_every_changed_or_omitted_signing_field.name, reviewed_fee_not_enforced_refuses_every_changed_or_omitted_signing_field);
test(reviewed_fee_not_enforced_sends_exact_estimated_caps_when_prepared_fields_are_null.name, reviewed_fee_not_enforced_sends_exact_estimated_caps_when_prepared_fields_are_null);
test(bounty_broadcast_not_persisted_recovers_success_after_reload_and_timeout.name, bounty_broadcast_not_persisted_recovers_success_after_reload_and_timeout);
test(revert_fee_message_and_bounty_broadcast_recover_reverted_receipt.name, revert_fee_message_and_bounty_broadcast_recover_reverted_receipt);
test(bounty_broadcast_not_persisted_saves_hash_even_when_component_closes_during_wallet_prompt.name, bounty_broadcast_not_persisted_saves_hash_even_when_component_closes_during_wallet_prompt);

test(bounty_broadcast_not_persisted_storage_failure_preserves_hash_and_blocks_resubmission.name, bounty_broadcast_not_persisted_storage_failure_preserves_hash_and_blocks_resubmission);

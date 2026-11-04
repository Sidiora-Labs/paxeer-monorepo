'use client';
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useAccount, usePublicClient } from 'wagmi';
import { parseUnits, type Hex } from 'viem';
import type { V2ContractTypes } from '@kindle/sdk';
import { useGatewaySessionOperations } from '@/core/market/operations';
import { useRuntimeAuthority } from '@/core/runtime-authority';
import { connectorProvider, isWalletProvider, PaxeerWallet } from '@/core/wallet-sdk/client';
import { accountSession, readAccountAsset, subscribeAccount, uploadAccountAsset, type AccountAsset } from '@/widgets/profile/account-transport';
import { queryKeys } from '@/core/query-keys';
import { bountiesContract, type Bounty, type BountyFailure } from './bounties-contract';
import { validateSubmission, type BountySubmission } from './use-bounty';
import { validatePreparedTransaction, type BountyTransactionIntent, type TransactionRequest, type ValidatedTransaction } from './prepared-transaction';

import { assertBountyReview, bountyIntentId, BountyTransactionAttempt, prepareBountyReview, type BountyFeeReview } from './bounty-transaction-attempt';

export type BountyCreateBody = V2ContractTypes['BountyCreateBody'];
export type BountyCreateResponse = V2ContractTypes['BountyCreateResponse'];
export type BountySubmitBody = V2ContractTypes['BountySubmitBody'];
export type BountyAcceptResponse = V2ContractTypes['BountyAcceptResponse'];

export const MUTATION_OPERATIONS = { create: 'IF_BOUNTY_CREATE', submit: 'IF_BOUNTY_SUBMIT', accept: 'IF_BOUNTY_ACCEPT' } as const;
export const TITLE_MAX = 120;
export const DESCRIPTION_MAX = 4000;
export const SUBMISSION_MAX = 2000;
export const EVIDENCE_MAX_FILES = 4;
export const EVIDENCE_MAX_BYTES = 6 * 1024 * 1024;
export const EVIDENCE_TYPES: readonly string[] = ['image/png', 'image/jpeg', 'image/webp'];

const DECIMAL = /^(0|[1-9][0-9]*)(?:\.([0-9]+))?$/;
const UINT256_LIMIT = 1n << 256n;

/** Raw integer for a typed decimal amount, or null when the text is not a plain positive decimal within `decimals`. */
export function parseDecimalAmount(input: string, decimals: number): bigint | null {
  const text = input.trim();
  const match = DECIMAL.exec(text);
  if (!match || !Number.isSafeInteger(decimals) || decimals < 0 || decimals > 36) return null;
  if ((match[2]?.length ?? 0) > decimals) return null;
  const raw = parseUnits(text, decimals);
  return raw > 0n && raw < UINT256_LIMIT ? raw : null;
}

/** Problem with a chosen evidence file, or null when the gateway asset rules accept it. */
export function evidenceFileProblem(file: Pick<File, 'type' | 'size'>): string | null {
  if (!EVIDENCE_TYPES.includes(file.type)) return 'Use a PNG, JPEG or WebP image';
  if (file.size === 0) return 'That file is empty';
  if (file.size > EVIDENCE_MAX_BYTES) return 'Images can be up to 6 MB';
  return null;
}

function quarantined(response: { quality: string; watermark: { integrity?: string } | null }): boolean {
  return response.quality === 'quarantined' || response.watermark?.integrity === 'quarantined';
}

/** The created bounty must be the one requested, owned by the signed-in wallet. */
export function validateCreated(response: BountyCreateResponse, body: BountyCreateBody, wallet: string): BountyCreateResponse {
  if (quarantined(response)) throw new Error('Bounty creation integrity quarantined');
  const bounty = bountiesContract.validateBounty(response.data.bounty);
  if (!bountiesContract.isCreator(bounty, wallet)) throw new Error('Created bounty belongs to another wallet');
  if (bounty.title !== body.title || bounty.budgetRaw !== body.budgetRaw || bounty.assetAddress.toLowerCase() !== body.assetAddress.toLowerCase()) throw new Error('Created bounty differs from the request');
  if ((body.poolAddress?.toLowerCase() ?? null) !== (bounty.poolAddress?.toLowerCase() ?? null)) throw new Error('Created bounty pool differs from the request');
  return response;
}

export function validateAccepted(response: BountyAcceptResponse, bountyId: string, submissionId: string, awardRaw: string): BountyAcceptResponse {
  if (quarantined(response)) throw new Error('Acceptance integrity quarantined');
  const submission = validateSubmission(response.data.submission, bountyId);
  if (submission.submissionId !== submissionId || submission.state !== 'accepted' || submission.awardRaw !== awardRaw) throw new Error('Accepted submission differs from the request');
  return response;
}

const noSession = () => null;

/** Wallet of the installed account session (bearer for the authenticated bounty operations), or null when signed out. */
export function useSessionWallet(): string | null {
  const session = useSyncExternalStore(subscribeAccount, accountSession, noSession);
  return session?.session.wallet ?? null;
}

export class SignedOutError extends Error {
  constructor() { super('Sign in to continue'); }
}

function failureOf(operation: string, error: unknown): BountyFailure {
  return bountiesContract.mapError(operation, error);
}

/** IF_BOUNTY_CREATE `POST /v2/bounties`; answers the created bounty plus its funding transaction. */
export function useCreateBounty() {
  const operations = useGatewaySessionOperations();
  const wallet = useSessionWallet();
  const client = useQueryClient();
  const mutation = useMutation<BountyCreateResponse, BountyFailure, BountyCreateBody>({
    mutationFn: async body => {
      if (!operations || !wallet) throw failureOf(MUTATION_OPERATIONS.create, new SignedOutError());
      try {
        return validateCreated(await operations.call('IF_BOUNTY_CREATE', { body }), body, wallet);
      } catch (error) {
        throw failureOf(MUTATION_OPERATIONS.create, error);
      }
    },
    onSuccess: () => { void client.invalidateQueries({ queryKey: queryKeys.bounties('', null).slice(0, 2) }); },
  });
  return { signedIn: operations !== null && wallet !== null, wallet, mutation };
}

/** IF_BOUNTY_SUBMIT `POST /v2/bounties/{bountyId}/submissions` with a client id stable across retries of one draft. */
export function useSubmitBounty(bountyId: string) {
  const operations = useGatewaySessionOperations();
  const wallet = useSessionWallet();
  const client = useQueryClient();
  const mutation = useMutation<BountySubmission, BountyFailure, BountySubmitBody>({
    mutationFn: async body => {
      if (!operations || !wallet) throw failureOf(MUTATION_OPERATIONS.submit, new SignedOutError());
      try {
        const response = await operations.call('IF_BOUNTY_SUBMIT', { path: { bountyId }, body });
        if (quarantined(response)) throw new Error('Submission integrity quarantined');
        const submission = validateSubmission(response.data.submission, bountyId);
        if (submission.authorWallet.toLowerCase() !== wallet.toLowerCase()) throw new Error('Submission belongs to another wallet');
        return submission;
      } catch (error) {
        throw failureOf(MUTATION_OPERATIONS.submit, error);
      }
    },
    onSuccess: () => { void client.invalidateQueries({ queryKey: queryKeys.bounty(bountyId) }); },
  });
  return { signedIn: operations !== null && wallet !== null, wallet, mutation };
}

export interface AcceptInput { submissionId: string; winnerWallet: string; awardRaw: string; expectedBountyRevision: string }

/** IF_BOUNTY_ACCEPT `POST .../submissions/{submissionId}:accept` (creator only); answers the settlement transaction. */
export function useAcceptSubmission(bounty: Bounty) {
  const operations = useGatewaySessionOperations();
  const wallet = useSessionWallet();
  const client = useQueryClient();
  const mutation = useMutation<BountyAcceptResponse, BountyFailure, AcceptInput>({
    mutationFn: async input => {
      if (!operations || !wallet) throw failureOf(MUTATION_OPERATIONS.accept, new SignedOutError());
      if (!bountiesContract.isCreator(bounty, wallet)) throw failureOf(MUTATION_OPERATIONS.accept, new Error('Only the bounty creator can pick winners'));
      try {
        const response = await operations.call('IF_BOUNTY_ACCEPT', {
          path: { bountyId: bounty.bountyId, submissionId: input.submissionId },
          body: { awardRaw: input.awardRaw, expectedBountyRevision: input.expectedBountyRevision },
        });
        if (response.data.submission.authorWallet.toLowerCase() !== input.winnerWallet.toLowerCase()) throw new Error('Accepted winner differs from the reviewed submission');
        return validateAccepted(response, bounty.bountyId, input.submissionId, input.awardRaw);
      } catch (error) {
        throw failureOf(MUTATION_OPERATIONS.accept, error);
      }
    },
    onSuccess: () => { void client.invalidateQueries({ queryKey: queryKeys.bounty(bounty.bountyId) }); },
  });
  return { signedIn: operations !== null && wallet !== null, wallet, mutation };
}

/* ------------------------------------------------------------ evidence */

export interface EvidenceItem {
  id: string;
  file: File;
  previewUrl: string;
  status: 'uploading' | 'review' | 'approved' | 'rejected' | 'failed';
  asset: AccountAsset | null;
  error: string | null;
}

function evidenceStatus(asset: AccountAsset): EvidenceItem['status'] {
  if (asset.state === 'approved') return asset.publicUrl ? 'approved' : 'review';
  if (asset.state === 'rejected') return 'rejected';
  return 'review';
}

function uploadError(error: unknown): string {
  if (error && typeof error === 'object' && 'status' in error && typeof (error as { status: unknown }).status === 'number') {
    const status = (error as { status: number }).status;
    if (status === 404 || status === 501 || status === 503) return "Uploads aren't available yet";
    if (status === 401) return 'Sign in again to upload';
    if (status === 413) return 'That file is too big';
    if (status === 415) return "That file type isn't supported";
    return 'Upload failed, try again';
  }
  return 'Upload failed, try again';
}

/**
 * Evidence attachments through the authenticated account asset PUT
 * (`/v2/accounts/{wallet}/assets/{assetId}`, the same transport the profile
 * editor uses). Only approved assets with a public URL become evidenceUrls.
 */
export function useEvidenceUploads() {
  const session = useSyncExternalStore(subscribeAccount, accountSession, noSession);
  const [items, setItems] = useState<EvidenceItem[]>([]);
  const itemsRef = useRef(items);
  const owner = useRef(session);
  const mounted = useRef(true);
  const clear = useCallback(() => {
    for (const item of itemsRef.current) URL.revokeObjectURL(item.previewUrl);
    itemsRef.current = [];
    setItems([]);
  }, []);
  const syncOwner = useCallback(() => {
    const next = accountSession();
    if (owner.current !== next) { owner.current = next; clear(); }
    return next;
  }, [clear]);
  useEffect(() => {
    mounted.current = true;
    syncOwner();
    const stop = subscribeAccount(syncOwner);
    return () => {
      mounted.current = false;
      stop();
      for (const item of itemsRef.current) URL.revokeObjectURL(item.previewUrl);
      itemsRef.current = [];
    };
  }, [syncOwner]);
  const patch = useCallback((id: string, next: Partial<EvidenceItem>, scope: ReturnType<typeof accountSession>) => {
    if (!mounted.current || scope !== accountSession() || scope !== owner.current || scope?.signal.aborted) return;
    itemsRef.current = itemsRef.current.map(item => item.id === id ? { ...item, ...next } : item);
    setItems(itemsRef.current);
  }, []);

  const transfer = useCallback(async (id: string, file: File) => {
    const scope = syncOwner();
    if (!scope) { patch(id, { status: 'failed', error: 'Sign in to upload images' }, scope); return; }
    patch(id, { status: 'uploading', error: null }, scope);
    try {
      const asset = await uploadAccountAsset(scope.session.wallet, id, file);
      if (asset.ownerWallet.toLowerCase() !== scope.session.wallet.toLowerCase() || asset.assetId !== id) throw new Error('Evidence identity mismatch');
      const status = evidenceStatus(asset);
      patch(id, { asset, status, error: status === 'rejected' ? "This image wasn't approved. Remove it or pick another" : null }, scope);
    } catch (error) {
      patch(id, { status: 'failed', error: uploadError(error) }, scope);
    }
  }, [patch, syncOwner]);

  const add = useCallback((files: readonly File[]): string | null => {
    if (!syncOwner()) return 'Sign in to upload images';
    const room = EVIDENCE_MAX_FILES - itemsRef.current.length;
    if (room <= 0) return `Up to ${EVIDENCE_MAX_FILES} images per submission`;
    const accepted: EvidenceItem[] = [];
    let problem: string | null = files.length > room ? `You can add ${room} more image${room === 1 ? '' : 's'}` : null;
    for (const file of files.slice(0, room)) {
      const issue = evidenceFileProblem(file);
      if (issue) { problem = `${file.name}: ${issue}`; continue; }
      accepted.push({ id: crypto.randomUUID(), file, previewUrl: URL.createObjectURL(file), status: 'uploading', asset: null, error: null });
    }
    if (accepted.length > 0) {
      itemsRef.current = [...itemsRef.current, ...accepted];
      setItems(itemsRef.current);
      for (const item of accepted) void transfer(item.id, item.file);
    }
    return problem;
  }, [transfer, syncOwner]);

  const remove = useCallback((id: string) => {
    syncOwner();
    const target = itemsRef.current.find(item => item.id === id);
    if (target) URL.revokeObjectURL(target.previewUrl);
    itemsRef.current = itemsRef.current.filter(item => item.id !== id);
    setItems(itemsRef.current);
  }, [syncOwner]);

  const retry = useCallback((id: string) => {
    syncOwner();
    const item = itemsRef.current.find(entry => entry.id === id);
    if (item) void transfer(id, item.file);
  }, [transfer, syncOwner]);

  const refresh = useCallback(async (id: string) => {
    const scope = syncOwner();
    if (!scope || !itemsRef.current.some(item => item.id === id)) return;
    try {
      const asset = await readAccountAsset(scope.session.wallet, id);
      if (asset.ownerWallet.toLowerCase() !== scope.session.wallet.toLowerCase() || asset.assetId !== id) throw new Error('Evidence identity mismatch');
      const status = evidenceStatus(asset);
      patch(id, { asset, status, error: status === 'rejected' ? "This image wasn't approved. Remove it or pick another" : null }, scope);
    } catch (error) {
      patch(id, { error: uploadError(error) }, scope);
    }
  }, [patch, syncOwner]);

  const visible = owner.current === session ? items : [];
  const evidenceUrls = visible.flatMap(item => (item.status === 'approved' && item.asset?.publicUrl ? [item.asset.publicUrl] : []));
  const ready = visible.every(item => item.status === 'approved');
  return { items: visible, evidenceUrls, ready, add, remove, retry, refresh, clear };
}

/* ------------------------------------------------- prepared transaction */

export type SigningState =
  | { kind: 'idle' }
  | { kind: 'invalid'; error: string }
  | { kind: 'estimating'; tx: ValidatedTransaction }
  | ({ kind: 'review' } & BountyFeeReview)
  | { kind: 'gas-unavailable'; tx: ValidatedTransaction; error: string }
  | { kind: 'wallet'; tx: ValidatedTransaction; costRaw: bigint }
  | { kind: 'pending'; tx: ValidatedTransaction; hash: Hex }
  | { kind: 'confirmed'; tx: ValidatedTransaction; hash: Hex }
  | { kind: 'reverted'; tx: ValidatedTransaction; hash: Hex }
  | { kind: 'failed'; tx: ValidatedTransaction; error: string }
  | { kind: 'uncertain'; tx: ValidatedTransaction; hash: Hex | null; error: string };

function message(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

export function useBountyAttemptRecovery(account: string | null, intent: BountyTransactionIntent, publicClient: import('./bounty-transaction-attempt').BountyReceiptClient | undefined) {
  const [attempt, setAttempt] = useState<BountyTransactionAttempt | null>(null);
  const [error, setError] = useState<string | null>(null);
  const key = bountyIntentId(intent);
  const intentRef = useRef(intent);
  intentRef.current = intent;
  useEffect(() => {
    setAttempt(null);
    setError(null);
    if (!account || !publicClient) return;
    let opened: BountyTransactionAttempt;
    try {
      opened = new BountyTransactionAttempt(window.localStorage, account, intentRef.current, publicClient);
      setAttempt(opened);
    } catch (failure) { setError(message(failure, 'Local recovery unavailable; check wallet activity')); return; }
    return () => opened.close();
  }, [account, key, publicClient]);
  const subscribe = useCallback((notify: () => void) => attempt?.subscribe(notify) ?? (() => {}), [attempt]);
  const snapshot = useCallback(() => attempt?.state ?? null, [attempt]);
  const state = useSyncExternalStore(subscribe, snapshot, noSession);
  const matches = attempt !== null && account !== null && attempt.account.toLowerCase() === account.toLowerCase() && bountyIntentId(attempt.intent) === key;
  return { attempt: matches ? attempt : null, state: matches ? state : null, error };
}

/**
 * Review and signing state for one server-prepared transaction. Validates it
 * against the connected wallet and the runtime deployment authority, estimates
 * PAX gas before anything is signed, then sends exactly the prepared call and
 * waits for its receipt. A failure after the wallet was invoked is reported as
 * uncertain, never as success or as a clean failure.
 */
export function usePreparedTransaction(request: TransactionRequest | null, intent: BountyTransactionIntent) {
  const intentKey = JSON.stringify(intent);
  const reviewedIntent = useMemo(() => JSON.parse(intentKey) as BountyTransactionIntent, [intentKey]);
  const wallet = useAccount();
  const publicClient = usePublicClient({ chainId: 125 });
  const { authority, unavailableReason } = useRuntimeAuthority();
  const published = authority?.network.network.addresses ?? null;
  const deploymentKey = published ? JSON.stringify(published) : null;
  const deployment = useMemo(() => (deploymentKey ? JSON.parse(deploymentKey) as Record<string, string> : null), [deploymentKey]);
  const [state, setState] = useState<SigningState>({ kind: 'idle' });
  const [attempt, setAttempt] = useState(0);
  const busy = useRef(false);
  /** Once the wallet was asked, the outcome must stay on screen whatever the authority or wallet does next. */
  const settled = useRef(false);
  useEffect(() => { settled.current = ['wallet', 'pending', 'confirmed', 'reverted', 'uncertain'].includes(state.kind); }, [state.kind]);
  const address = wallet.address ?? null;
  const chainId = wallet.chainId ?? null;
  const recovery = useBountyAttemptRecovery(address, reviewedIntent, publicClient);
  const currentInputs = useRef({ request, intentKey, address, chainId, authority });
  currentInputs.current = { request, intentKey, address, chainId, authority };

  useEffect(() => {
    if (busy.current || settled.current) return;
    if (!request) { setState({ kind: 'idle' }); return; }
    if (!address) { setState({ kind: 'invalid', error: 'Connect your wallet to continue' }); return; }
    if (chainId !== 125) { setState({ kind: 'invalid', error: 'Switch your wallet to Paxeer' }); return; }
    if (!deployment) { setState({ kind: 'invalid', error: "Can't reach the network right now, try again" }); return; }
    let tx: ValidatedTransaction;
    try {
      tx = validatePreparedTransaction(request, { wallet: address, deployment, intent: reviewedIntent });
    } catch (error) {
      console.error('[bounty transaction check]', error);
      setState({ kind: 'invalid', error: "This transaction doesn't look right, so it wasn't sent to your wallet" });
      return;
    }
    if (!publicClient) { setState({ kind: 'gas-unavailable', tx, error: "Couldn't estimate the network fee, try again" }); return; }
    let cancelled = false;
    setState({ kind: 'estimating', tx });
    void (async () => {
      try {
        const review = await prepareBountyReview(request, { wallet: address, deployment, intent: reviewedIntent }, publicClient);
        if (!cancelled) setState({ kind: 'review', ...review });
      } catch (error) {
        console.error('[bounty fee estimate]', error);
        if (!cancelled) setState({ kind: 'gas-unavailable', tx, error: "Couldn't estimate the network fee, try again" });
      }
    })();
    return () => { cancelled = true; };
  }, [request, reviewedIntent, address, chainId, deployment, publicClient, unavailableReason, attempt]);

  const sign = useCallback(async () => {
    if (state.kind !== 'review' || busy.current || !publicClient || !recovery.attempt || recovery.state || recovery.error) return;
    busy.current = true;
    const reviewed = state;
    let invoked = false;
    let hash: Hex | null = null;
    try {
      if (!request || !wallet.connector || !authority || authority.expiresAt <= Date.now()) throw new Error(unavailableReason ?? 'Network authority expired; refresh before signing');
      const tx = validatePreparedTransaction(request, { wallet: wallet.address ?? '', deployment: authority.network.network.addresses, intent: reviewedIntent });
      assertBountyReview(reviewed, tx);
      recovery.attempt.assertCanSubmit();
      const provider = await connectorProvider(wallet.connector);
      if (!isWalletProvider(provider)) throw new Error('Wallet provider unavailable');
      const signer = new PaxeerWallet(provider);
      await signer.assertChain();
      const accounts = await signer.getAccounts();
      if (accounts[0]?.toLowerCase() !== tx.from.toLowerCase()) throw new Error('The prepared signer is not selected in this wallet');
      const latest = currentInputs.current;
      if (!latest.request || latest.intentKey !== intentKey || latest.address !== address || latest.chainId !== 125 || !latest.authority || latest.authority.expiresAt <= Date.now()) throw new Error('Signing context changed; review again');
      assertBountyReview(reviewed, validatePreparedTransaction(latest.request, { wallet: latest.address ?? '', deployment: latest.authority.network.network.addresses, intent: reviewedIntent }));
      recovery.attempt.assertCanSubmit();
      setState({ kind: 'wallet', tx, costRaw: reviewed.costRaw });
      invoked = true;
      hash = await recovery.attempt.broadcastReviewed(reviewed, tx, provider);
      setState({ kind: 'pending', tx, hash });
    } catch (error) {
      console.error('[bounty signing]', error);
      const text = message(error, 'Signing failed');
      const rejected = /reject|denied|cancel/i.test(text) && hash === null;
      if (invoked && !rejected) setState({ kind: 'uncertain', tx: reviewed.tx, hash, error: 'Something went wrong. Check your wallet activity before trying again' });
      else setState({ kind: 'failed', tx: reviewed.tx, error: rejected ? 'You declined in your wallet' : 'Something went wrong, try again' });
    } finally {
      busy.current = false;
    }
  }, [state, publicClient, request, reviewedIntent, wallet.connector, wallet.address, authority, unavailableReason, recovery.attempt, recovery.state, recovery.error, intentKey, address]);

  /** Re-validate and re-estimate after a clean failure (never after an uncertain broadcast). */
  const retry = useCallback(() => {
    if (recovery.state) { recovery.attempt?.retry(); return; }
    if (state.kind === 'failed' || state.kind === 'gas-unavailable' || state.kind === 'invalid') setAttempt(value => value + 1);
  }, [state.kind, recovery.state, recovery.attempt]);

  return { state: recovery.state ?? (recovery.error ? { kind: 'invalid' as const, error: recovery.error } : state), sign, retry };
}

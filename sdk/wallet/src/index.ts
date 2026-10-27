import {
  createClient,
  type AuthChangeEvent,
  type Session,
  type SupabaseClient,
  type User,
} from '@supabase/supabase-js';
import { keccak256, serializeTransaction, type Hex, type TransactionSerializable } from 'viem';
import { JsonRpcClient } from './rpc.js';
import { decodeSequencerReceipt, readSequencerReceipt } from './receipt.js';
import {
  PaxeerWalletError,
  FundedUnsupportedError,
  type ChainInfo,
  type FundedProvisionResponse,
  type FundedSelfResponse,
  type FundedTier,
  type PaxeerWalletConfig,
  type PublicWallet,
  type SendTxResponse,
  type SignMessageResponse,
  type SignTxResponse,
  type TxRequest,
} from './types.js';

export * from './types.js';
export * from './rpc.js';
export * from './endpoint.js';
export * from './kernel.js';
export * from './human.js';
export * from './ladder.js';
export * from './receipt.js';
export * from './modules/index.js';
export * from './errors.js';
export * from './provider.js';
export * from './eip6963.js';
export * from './wallet.js';
export * from './agent.js';

export interface EvmTransactionReview {
  readonly id: string;
  readonly state: 'reviewed';
  readonly to: Hex;
  readonly value: string;
  readonly data: Hex;
  readonly signing_hash: Hex;
  readonly transaction: Hex;
  readonly principal: string;
  readonly chain_id: number;
  readonly method: string;
  readonly destination: string;
  readonly key_id: string;
  readonly public_key: string;
  readonly epoch: number;
  readonly session_id: string;
  readonly expires_at: string;
  readonly participants: readonly string[];
}

export type EvmApproval = Omit<EvmTransactionReview, 'state'> & {
  readonly state: 'approved' | 'signing_unknown' | 'signed';
  readonly approval: {
    readonly version: 1;
    readonly principal: string;
    readonly key_id: string;
    readonly session_id: string;
    readonly signing_hash: Hex;
    readonly expires_at: string;
  };
};

export interface EvmReviewStorage {
  getItem(key: string): string | null | Promise<string | null>;
  setItem(key: string, value: string): void | Promise<void>;
}

export type EvmReviewWalletConfig = PaxeerWalletConfig & {
  readonly evmReview?: { readonly storage: EvmReviewStorage };
};

/**
 * `PaxeerWallet` — drop-in client for any Paxeer-network app.
 *
 * Lifecycle:
 *   1. `signInWith{Email,OAuth}()` runs the Supabase auth flow
 *   2. After the user returns with a valid session, the SDK auto-provisions a
 *      wallet on first call to `getWallet()` / `sendTransaction()` if needed
 *   3. All signing happens on the API server; the SDK never sees key material
 */
export class PaxeerWallet {
  readonly supabase: SupabaseClient;
  private readonly apiUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly evmLocks = new Map<string, Promise<unknown>>();
  private readonly evmStorage?: EvmReviewStorage;

  constructor(config: EvmReviewWalletConfig) {
    if (!config.apiUrl) throw new Error('PaxeerWallet: apiUrl required');
    if (!config.supabaseUrl) throw new Error('PaxeerWallet: supabaseUrl required');
    if (!config.supabaseAnonKey) throw new Error('PaxeerWallet: supabaseAnonKey required');

    this.apiUrl = config.apiUrl.replace(/\/$/, '');
    this.fetchImpl = config.fetch ?? globalThis.fetch.bind(globalThis);
    if (
      config.evmReview &&
      (typeof config.evmReview.storage?.getItem !== 'function' ||
        typeof config.evmReview.storage?.setItem !== 'function')
    )
      throw new TypeError('EVM review storage is required');
    this.evmStorage = config.evmReview?.storage;
    this.supabase = createClient(config.supabaseUrl, config.supabaseAnonKey, {
      auth: {
        persistSession: true,
        autoRefreshToken: true,
        detectSessionInUrl: true,
      },
    });
  }

  // ---------------------------------------------------------------------
  // Auth
  // ---------------------------------------------------------------------

  /** Send a magic-link email to start passwordless sign-in. */
  signInWithEmail(email: string, redirectTo?: string): Promise<{ error: Error | null }> {
    return this.supabase.auth
      .signInWithOtp({
        email,
        ...(redirectTo ? { options: { emailRedirectTo: redirectTo } } : {}),
      })
      .then((r) => ({ error: r.error }));
  }

  /** Begin OAuth sign-in. Browser will redirect to the provider. */
  async signInWithOAuth(
    provider: 'google' | 'apple' | 'twitter' | 'github' | 'discord',
    redirectTo?: string
  ): Promise<void> {
    const { error } = await this.supabase.auth.signInWithOAuth({
      provider,
      ...(redirectTo ? { options: { redirectTo } } : {}),
    });
    if (error) throw new PaxeerWalletError(error.message, 'OAUTH_FAILED');
  }

  signOut(): Promise<void> {
    return this.supabase.auth.signOut().then(() => undefined);
  }

  async getSession(): Promise<Session | null> {
    const { data } = await this.supabase.auth.getSession();
    return data.session ?? null;
  }

  async getUser(): Promise<User | null> {
    const { data } = await this.supabase.auth.getUser();
    return data.user ?? null;
  }

  /**
   * Subscribe to auth state changes. Returns an unsubscribe function.
   * Forward this to whatever framework reactivity layer you use.
   */
  onAuthStateChange(cb: (event: AuthChangeEvent, session: Session | null) => void): () => void {
    const { data } = this.supabase.auth.onAuthStateChange(cb);
    return () => data.subscription.unsubscribe();
  }

  // ---------------------------------------------------------------------
  // Wallet
  // ---------------------------------------------------------------------

  /**
   * Get the current user's wallet. Auto-provisions on first call.
   * Throws if the user is not signed in.
   *
   * NOTE: this composes the two primitives below. Prefer calling
   * `getStandardSelf()` + `provisionStandardWallet()` directly when you want
   * the user to explicitly choose embedded provisioning.
   */
  async getWallet(): Promise<{ wallet: PublicWallet; chain: ChainInfo }> {
    const me = await this.getStandardSelf();
    if (me) return me;

    // No wallet yet — provision then fetch.
    await this.provisionStandardWallet();
    const after = await this.getStandardSelf();
    if (!after) throw new PaxeerWalletError('provision_lost', 'PROVISION_LOST', 500);
    return after;
  }

  /**
   * Read the current user's standard wallet without auto-provisioning.
   * Returns `null` if the user is signed in but hasn't created a wallet yet.
   *
   * Use this when the calling UI must NOT silently create a wallet — e.g.
   * an embedded wallet creation screen.
   */
  async getStandardSelf(): Promise<{ wallet: PublicWallet; chain: ChainInfo } | null> {
    try {
      return await this.callJson<{ wallet: PublicWallet; chain: ChainInfo }>(
        'GET',
        '/v1/wallet/me'
      );
    } catch (err) {
      if (err instanceof PaxeerWalletError && err.status === 404) return null;
      throw err;
    }
  }

  /**
   * Explicitly provision a embedded threshold-custody wallet for the authenticated
   * user. Idempotent on the server — calling twice returns the same wallet.
   */
  provisionStandardWallet(): Promise<{ wallet: PublicWallet }> {
    return this.callJson<{ wallet: PublicWallet }>('POST', '/v1/wallet/provision');
  }

  /** Sign a transaction without broadcasting. Returns serialized signed tx hex. */
  signTransaction(tx: TxRequest): Promise<SignTxResponse> {
    return this.callJson<SignTxResponse>('POST', '/v1/wallet/sign', { tx: serialize(tx) });
  }

  /** Sign + broadcast and return the tx hash with its available sequencer receipt. */
  async sendTransaction(tx: TxRequest): Promise<SendTxResponse> {
    const response = await this.callJson<SendTxResponse>('POST', '/v1/wallet/send', {
      tx: serialize(tx),
    });
    let receipt = response.receipt ? decodeSequencerReceipt(response.receipt) : null;
    if (!receipt) {
      const { chain } = await this.getWallet();
      const rpc = new JsonRpcClient({ url: chain.rpc_url, fetch: this.fetchImpl });
      try {
        receipt = await readSequencerReceipt(
          (method, params) => rpc.call(method, params),
          response.tx_hash
        );
      } catch (error) {
        if (
          error instanceof TypeError &&
          error.message === 'sequencer receipt txHash differs from the asked hash'
        ) {
          throw new TypeError('receipt does not match the sent transaction');
        }
        throw error;
      }
    }
    if (receipt) {
      if (receipt.txHash.toLowerCase() !== response.tx_hash.toLowerCase()) {
        throw new TypeError('receipt does not match the sent transaction');
      }
      response.receipt = receipt;
    }
    return response;
  }

  /** EIP-191 personal_sign over a message string. */
  signMessage(message: string): Promise<SignMessageResponse> {
    return this.callJson<SignMessageResponse>('POST', '/v1/wallet/sign-message', { message });
  }

  async reviewEvmTransaction(
    unsignedTransaction: TransactionSerializable
  ): Promise<EvmTransactionReview> {
    if (
      unsignedTransaction.type !== 'eip1559' ||
      !unsignedTransaction.to ||
      unsignedTransaction.r !== undefined ||
      unsignedTransaction.s !== undefined ||
      unsignedTransaction.v !== undefined ||
      unsignedTransaction.yParity !== undefined
    ) {
      throw new TypeError('unsigned EIP-1559 transaction required');
    }
    const transaction = serializeTransaction(unsignedTransaction);
    const expected = {
      to: unsignedTransaction.to.toLowerCase(),
      value: (unsignedTransaction.value ?? 0n).toString(),
      data: (unsignedTransaction.data ?? '0x').toLowerCase(),
      signing_hash: keccak256(transaction),
      chainId: unsignedTransaction.chainId,
    };
    const principal = await this.evmPrincipal();
    const review = await this.callJson<EvmTransactionReview>('POST', '/v1/wallet/evm/review', {
      transaction,
    });
    if (
      !review ||
      review.state !== 'reviewed' ||
      review.principal !== principal ||
      typeof review.to !== 'string' ||
      review.to.toLowerCase() !== expected.to ||
      review.value !== expected.value ||
      typeof review.data !== 'string' ||
      review.data.toLowerCase() !== expected.data ||
      review.signing_hash !== expected.signing_hash ||
      review.chain_id !== expected.chainId ||
      typeof review.id !== 'string' ||
      !review.id
    ) {
      throw new TypeError('review does not match the transaction');
    }
    await this.evmSave(principal, review);
    return freezeEvmReview(review);
  }

  async approveEvmTransaction(reviewId: string): Promise<EvmApproval> {
    return this.evmExclusive(`evm:${reviewId}`, async () => {
      const principal = await this.evmPrincipal();
      const review = (await this.evmLoad(principal)).find((entry) => entry.id === reviewId);
      if (!review) throw new TypeError('original EVM review is required');
      const approval = await this.callJson<EvmApproval>('POST', '/v1/wallet/evm/approve', {
        review_id: reviewId,
        signing_hash: review.signing_hash,
        decision: 'approve',
      });
      this.evmMatch(review, approval);
      if (
        !['approved', 'signing_unknown', 'signed'].includes(approval.state) ||
        approval.approval?.version !== 1 ||
        approval.approval.signing_hash !== review.signing_hash ||
        approval.approval.principal !== principal ||
        approval.approval.key_id !== review.key_id ||
        approval.approval.session_id !== review.session_id ||
        approval.approval.expires_at !== review.expires_at
      ) {
        throw new TypeError('approval does not match the review');
      }
      await this.evmSave(principal, approval);
      return freezeEvmReview(approval);
    });
  }

  async evmApprovalStatus(reviewId: string): Promise<EvmTransactionReview | EvmApproval> {
    const principal = await this.evmPrincipal();
    const artifact = await this.callJson<EvmTransactionReview | EvmApproval>(
      'GET',
      `/v1/wallet/evm/approvals/${encodeURIComponent(reviewId)}`
    );
    if (
      !artifact ||
      artifact.id !== reviewId ||
      artifact.principal !== principal ||
      !['reviewed', 'approved', 'signing_unknown', 'signed'].includes(artifact.state)
    ) {
      throw new TypeError('approval does not match the review');
    }
    const prior = (await this.evmLoad(principal)).find((entry) => entry.id === reviewId);
    if (prior) {
      this.evmMatch(prior, artifact);
      await this.evmSave(principal, artifact);
    }
    return freezeEvmReview(artifact);
  }

  private evmStorageKey(principal: string): string {
    return `paxeer:evm-review:v1:${principal}`;
  }

  private async evmPrincipal(): Promise<string> {
    if (!this.evmStorage) throw new TypeError('EVM review storage is required');
    const session = await this.getSession();
    if (!session?.access_token) throw new PaxeerWalletError('not_authenticated', 'NO_SESSION', 401);
    return session.user.id;
  }

  private async evmLoad(principal: string): Promise<(EvmTransactionReview | EvmApproval)[]> {
    const raw = await this.evmStorage!.getItem(this.evmStorageKey(principal));
    if (raw === null) return [];
    const saved: unknown = JSON.parse(raw);
    if (
      !Array.isArray(saved) ||
      saved.some(
        (entry) =>
          !entry ||
          entry.principal !== principal ||
          typeof entry.id !== 'string' ||
          typeof entry.signing_hash !== 'string'
      )
    ) {
      throw new TypeError('invalid retained EVM reviews');
    }
    return saved;
  }

  private evmMatch(
    review: EvmTransactionReview | EvmApproval,
    artifact: EvmTransactionReview | EvmApproval
  ): void {
    if (
      !artifact ||
      [
        'id',
        'principal',
        'to',
        'value',
        'data',
        'signing_hash',
        'chain_id',
        'key_id',
        'session_id',
        'expires_at',
      ].some(
        (field) =>
          review[field as keyof EvmTransactionReview] !==
          artifact[field as keyof EvmTransactionReview]
      )
    ) {
      throw new TypeError('approval does not match the review');
    }
  }

  private async evmSave(
    principal: string,
    artifact: EvmTransactionReview | EvmApproval
  ): Promise<void> {
    if ((await this.evmPrincipal()) !== principal)
      throw new TypeError('authenticated principal changed');
    await this.evmExclusive(`evm-storage:${principal}`, async () => {
      const saved = await this.evmLoad(principal);
      await this.evmStorage!.setItem(
        this.evmStorageKey(principal),
        JSON.stringify([...saved.filter((entry) => entry.id !== artifact.id), artifact])
      );
    });
  }

  private async evmExclusive<T>(id: string, work: () => Promise<T>): Promise<T> {
    const prior = this.evmLocks.get(id) ?? Promise.resolve();
    const next = prior.catch(() => undefined).then(work);
    this.evmLocks.set(id, next);
    try {
      return await next;
    } finally {
      if (this.evmLocks.get(id) === next) this.evmLocks.delete(id);
    }
  }

  listFundedTiers(): Promise<{ tiers: FundedTier[] }> {
    return Promise.reject(new FundedUnsupportedError());
  }

  getFundedSelf(): Promise<FundedSelfResponse | null> {
    return Promise.reject(new FundedUnsupportedError());
  }

  provisionFundedAccount(tier_id = 'starter_25k'): Promise<FundedProvisionResponse> {
    void tier_id;
    return Promise.reject(new FundedUnsupportedError());
  }

  signFundedTransaction(tx: TxRequest): Promise<SignTxResponse> {
    void tx;
    return Promise.reject(new FundedUnsupportedError());
  }

  sendFundedTransaction(tx: TxRequest): Promise<SendTxResponse> {
    void tx;
    return Promise.reject(new FundedUnsupportedError());
  }

  signFundedMessage(message: string): Promise<SignMessageResponse> {
    void message;
    return Promise.reject(new FundedUnsupportedError());
  }

  private async callJson<T>(
    method: 'GET' | 'POST',
    path: string,
    body?: unknown,
    options: { auth?: boolean } = {}
  ): Promise<T> {
    const headers: Record<string, string> = {};
    if (options.auth !== false) {
      const session = await this.getSession();
      if (!session?.access_token) {
        throw new PaxeerWalletError('not_authenticated', 'NO_SESSION', 401);
      }
      headers.Authorization = `Bearer ${session.access_token}`;
    }
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }

    const res = await this.fetchImpl(`${this.apiUrl}${path}`, {
      method,
      headers,
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });

    let payload: unknown;
    try {
      payload = await res.json();
    } catch {
      payload = null;
    }

    if (!res.ok) {
      const code =
        typeof payload === 'object' && payload && 'error' in payload
          ? String((payload as { error: unknown }).error)
          : `HTTP_${res.status}`;
      const message =
        typeof payload === 'object' && payload && 'message' in payload
          ? String((payload as { message: unknown }).message)
          : `request failed: ${res.status}`;
      throw new PaxeerWalletError(message, code, res.status, payload);
    }
    return payload as T;
  }
}

function serialize(tx: TxRequest): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (tx.to !== undefined) out.to = tx.to;
  if (tx.value !== undefined) out.value = bigintToString(tx.value);
  if (tx.data !== undefined) out.data = tx.data;
  if (tx.gas !== undefined) out.gas = bigintToString(tx.gas);
  if (tx.maxFeePerGas !== undefined) out.maxFeePerGas = bigintToString(tx.maxFeePerGas);
  if (tx.maxPriorityFeePerGas !== undefined)
    out.maxPriorityFeePerGas = bigintToString(tx.maxPriorityFeePerGas);
  if (tx.nonce !== undefined) out.nonce = tx.nonce;
  if (tx.chainId !== undefined) out.chainId = tx.chainId;
  return out;
}

function bigintToString(v: string | bigint): string {
  return typeof v === 'bigint' ? v.toString() : v;
}

function freezeEvmReview<T>(value: T): T {
  const copy: T = JSON.parse(JSON.stringify(value));
  const freeze = (entry: unknown): void => {
    if (entry && typeof entry === 'object') {
      for (const child of Object.values(entry)) freeze(child);
      Object.freeze(entry);
    }
  };
  freeze(copy);
  return copy;
}

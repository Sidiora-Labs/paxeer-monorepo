import { EndpointClient } from './endpoint.js';
import { readSequencerReceipt } from './receipt.js';
import { JsonRpcError, isRecord } from './rpc.js';
import type {
  Balances,
  ChainAccount,
  KernelAvailabilityState,
  KernelBackendName,
  KernelRead,
  KernelStatus,
  KernelUnavailableState,
  SequencerReceiptAnswer,
} from './types.js';

export const KERNEL_UNAVAILABLE_CODE = -32010;
export const DEFAULT_KERNEL_AVAILABILITY_TTL_MS = 5_000;

const KERNEL_BACKENDS: readonly KernelBackendName[] = [
  'core_agent_boundary',
  'public_core',
  'independent_receipt_authority',
  'identity',
  'program_registry',
];

export interface KernelAvailabilityOptions {
  ttlMs?: number;
  now?: () => number;
}

export function availabilityFromStatus(status: KernelStatus): KernelAvailabilityState {
  if (status.reason === 'available') return { available: true, reason: 'available' };
  return { available: false, reason: status.reason, backend: null };
}

export function kernelUnavailableFrom(error: unknown): KernelUnavailableState | null {
  if (!(error instanceof JsonRpcError) || error.code !== KERNEL_UNAVAILABLE_CODE) return null;
  const data = error.data;
  if (!isRecord(data) || data.code !== 'kernel_unavailable') return null;
  if (data.reason !== 'not_configured' && data.reason !== 'unreachable') return null;
  const backend =
    typeof data.backend === 'string' && (KERNEL_BACKENDS as readonly string[]).includes(data.backend)
      ? (data.backend as KernelBackendName)
      : null;
  return { available: false, reason: data.reason, backend };
}

export class KernelAvailability {
  private readonly endpoint: EndpointClient;
  private readonly ttlMs: number;
  private readonly now: () => number;
  private cached: { state: KernelAvailabilityState; at: number } | null = null;
  private inflight: Promise<KernelAvailabilityState> | null = null;

  constructor(endpoint: EndpointClient, options: KernelAvailabilityOptions = {}) {
    const ttl = options.ttlMs ?? DEFAULT_KERNEL_AVAILABILITY_TTL_MS;
    if (!Number.isInteger(ttl) || ttl < 0) throw new RangeError('ttlMs must be a non-negative integer');
    this.endpoint = endpoint;
    this.ttlMs = ttl;
    this.now = options.now ?? Date.now;
  }

  async current(): Promise<KernelAvailabilityState> {
    if (this.cached !== null && this.now() - this.cached.at < this.ttlMs) return this.cached.state;
    if (this.inflight !== null) return this.inflight;
    this.inflight = this.endpoint
      .getNetwork()
      .then((network) => {
        const state = availabilityFromStatus(network.kernel);
        this.cached = { state, at: this.now() };
        return state;
      })
      .finally(() => {
        this.inflight = null;
      });
    return this.inflight;
  }

  record(state: KernelAvailabilityState): void {
    this.cached = { state, at: this.now() };
  }

  invalidate(): void {
    this.cached = null;
  }
}

export class KernelClient {
  readonly endpoint: EndpointClient;
  readonly availability: KernelAvailability;

  constructor(endpoint: EndpointClient, availability: KernelAvailability = new KernelAvailability(endpoint)) {
    this.endpoint = endpoint;
    this.availability = availability;
  }

  getAccount(address: string): Promise<KernelRead<ChainAccount>> {
    if (!/^0x[0-9a-fA-F]{40}$/.test(address)) throw new RangeError('account address is malformed');
    return this.readEndpoint(() => this.endpoint.getChainAccount(address));
  }

  getBalances(address: string): Promise<KernelRead<Balances>> {
    if (!/^[A-Za-z0-9._:-]{1,255}$/.test(address)) throw new RangeError('account address is malformed');
    return this.readEndpoint(() => this.endpoint.getBankBalances(address));
  }

  getSequence(address: string): Promise<KernelRead<string>> {
    if (!/^0x[0-9a-fA-F]{40}$/.test(address)) throw new RangeError('account address is malformed');
    return this.readEndpoint(async () => {
      const result: unknown = await this.endpoint.rpc.call('eth_getTransactionCount', [
        address,
        'pending',
      ]);
      if (typeof result !== 'string' || !/^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/.test(result))
        throw new TypeError('eth_getTransactionCount answered a malformed nonce');
      return BigInt(result).toString(10);
    });
  }

  getReceipt(hash: string): Promise<KernelRead<SequencerReceiptAnswer | null>> {
    return this.readEndpoint(() =>
      readSequencerReceipt((method, params) => this.endpoint.rpc.call(method, params), hash)
    );
  }

  private async readEndpoint<T>(request: () => Promise<T>): Promise<KernelRead<T>> {
    const state = await this.availability.current();
    if (!state.available) return state;
    try {
      return { available: true, result: await request() };
    } catch (error) {
      const unavailable = kernelUnavailableFrom(error);
      if (unavailable === null) throw error;
      this.availability.record(unavailable);
      return unavailable;
    }
  }
}

export interface WalletBudgetCap {
  readonly id: string;
  readonly owner: string;
  readonly asset: string;
  readonly account: string;
  readonly source_account: string | null;
  readonly limit: string;
  readonly configured_limit: string;
  readonly spent: string;
  readonly remaining: string;
  readonly carry_cap: string;
  readonly carried: string;
  readonly period_start: string;
  readonly period_length: string;
  readonly expiry: string;
  readonly revocation_height: string;
  readonly closed: boolean;
  readonly revoked: boolean;
}
export interface WalletGrantCap {
  readonly id: string;
  readonly owner: string;
  readonly recipient: string;
  readonly asset: string;
  readonly per_draw_maximum: string;
  readonly allowance: string;
  readonly drawn_total: string;
  readonly drawn_this_period: string;
  readonly recurring: boolean;
  readonly window_length: string;
  readonly window_start: string;
  readonly expiration: string;
  readonly revocation_height: string;
  readonly revoked_at_height: string;
  readonly revoked: boolean;
  readonly invoice_settled: boolean;
}
export interface WalletCapsSnapshot {
  readonly state: 'ready' | 'empty';
  readonly did: string;
  readonly account_id: string;
  readonly network_id: number;
  readonly context: {
    readonly principal: string;
    readonly session_id: string;
    readonly address: string;
    readonly chain_id: number;
    readonly expires_at: string;
  };
  readonly observation: {
    readonly rung: 'receipted' | 'committed';
    readonly block_number: string;
    readonly state_root: string;
  };
  readonly budgets: readonly WalletBudgetCap[];
  readonly grants: readonly WalletGrantCap[];
  readonly history?: WalletCapsHistory;
}
export type WalletCapsHistory =
  | { readonly source: 'indexer'; readonly indexed_height: string | null; readonly chain_height: string }
  | { readonly source: 'chain_scan'; readonly floor: string; readonly head: string };
export type WalletCapsState =
  | { readonly state: 'loading' }
  | { readonly state: 'unavailable'; readonly reason: 'consensus_unavailable' }
  | { readonly state: 'refused'; readonly reason: string }
  | WalletCapsSnapshot;

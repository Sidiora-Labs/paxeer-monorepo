import { SdkClient, type RequestOptions } from './transport.js';
import { validateChainsResponse, validateChainDetailResponse, validateChainRevenueResponse } from './validate.js';

export interface LaunchedChain {
  chain: string;
  chainId: string;
  pool: string;
  token: string;
  nftId: string;
  creator: string;
  blockNumber: string;
  status: 'active' | 'retired';
}

export interface ChainRevenueEpoch {
  epoch: string;
  epochAmount: string;
  snapshotBlock: string;
  eligibleSupply: string;
}

export interface ChainRevenueClaim {
  epoch: string;
  holder: string;
  amount: string;
  blockNumber: string;
}

export interface ChainEnvelope<T> {
  requestId: string;
  data: T;
  revision: string | null;
  watermark: null;
  quality: 'complete' | 'partial' | 'stale' | 'quarantined';
  nextCursor: string | null;
  generatedAt: string;
}

export type ChainsResponse = ChainEnvelope<{ items: LaunchedChain[] }>;
export type ChainDetailResponse = ChainEnvelope<{ chain: LaunchedChain }>;
export type ChainRevenueResponse = ChainEnvelope<{
  chain: string;
  epochs: ChainRevenueEpoch[];
  holder?: string;
  claims?: ChainRevenueClaim[];
}>;

export interface ChainPageQuery {
  limit?: number;
  cursor?: string;
}

export interface ChainRevenueQuery extends ChainPageQuery {
  holder?: string;
}

export type ChainRequestOptions = Pick<RequestOptions, 'signal' | 'timeoutMs' | 'requestId'>;

function chainPath(chain: string): string {
  if (!/^[1-9][0-9]*$/.test(chain) || BigInt(chain) > 4294967295n) throw new TypeError('Invalid chain');
  return `/v2/chains/${chain}`;
}

function pagePath(path: string, query: ChainRevenueQuery): string {
  const params = new URLSearchParams();
  if (query.limit !== undefined) {
    if (!Number.isSafeInteger(query.limit) || query.limit < 1 || query.limit > 100) throw new RangeError('Invalid chain page limit');
    params.set('limit', String(query.limit));
  }
  if (query.cursor !== undefined) {
    if (!query.cursor || !/^[A-Za-z0-9_-]+$/.test(query.cursor)) throw new TypeError('Invalid chain page cursor');
    params.set('cursor', query.cursor);
  }
  if (query.holder !== undefined) {
    if (!/^0x[0-9a-f]{40}$/.test(query.holder)) throw new TypeError('Invalid chain holder');
    params.set('holder', query.holder);
  }
  return params.size ? `${path}?${params}` : path;
}

export class Chains {
  constructor(readonly client: SdkClient) {}

  list(query: ChainPageQuery = {}, options: ChainRequestOptions = {}): Promise<ChainsResponse> {
    return this.client.requestV2Validated(pagePath('/v2/chains', query), validateChainsResponse, { ...options, method: 'GET', idempotentRead: true });
  }

  get(chain: string, options: ChainRequestOptions = {}): Promise<ChainDetailResponse> {
    return this.client.requestV2Validated(chainPath(chain), validateChainDetailResponse, { ...options, method: 'GET', idempotentRead: true });
  }

  revenue(chain: string, query: ChainRevenueQuery = {}, options: ChainRequestOptions = {}): Promise<ChainRevenueResponse> {
    return this.client.requestV2Validated(pagePath(`${chainPath(chain)}/revenue`, query), validateChainRevenueResponse, { ...options, method: 'GET', idempotentRead: true });
  }
}

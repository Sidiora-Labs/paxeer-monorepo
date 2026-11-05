/** Blockscout API v2 — HTTP client with configurable base URL */

import {
  walletReadUrl,
  assertWalletReadResponse,
  type WalletReadOptions,
} from '../wallet-read-contract';

const DEFAULT_TIMEOUT_MS = 30_000;

export interface BlockscoutClientConfig {
  baseUrl: string;
  timeoutMs?: number;
}

export class BlockscoutApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly statusText: string,
    public readonly body: string,
    public readonly url: string
  ) {
    super(`Blockscout API ${status} ${statusText}: ${body.slice(0, 200)}`);
    this.name = 'BlockscoutApiError';
  }
}

export class BlockscoutClient {
  private readonly baseUrl: string;
  private readonly timeoutMs: number;

  constructor(config: BlockscoutClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/+$/, '');
    this.timeoutMs = config.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  }

  async get<T>(
    path: string,
    params?: Record<string, string | number | boolean | undefined>,
    options: WalletReadOptions = {}
  ): Promise<T> {
    let url = `${this.baseUrl}/api${path}`;
    if (params) {
      const qs = Object.entries(params)
        .filter(([, v]) => v !== undefined)
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
        .join('&');
      if (qs) url += `?${qs}`;
    }

    url = walletReadUrl(url, options);
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);

    try {
      const res = await fetch(url, {
        method: 'GET',
        headers: {
          Accept: 'application/json',
          ...(options.fresh ? { 'Cache-Control': 'no-cache' } : {}),
        },
        cache: 'no-store',
        signal: controller.signal,
      });
      if (!res.ok) {
        const body = await res.text();
        throw new BlockscoutApiError(res.status, res.statusText, body, url);
      }
      assertWalletReadResponse(res, url);
      return (await res.json()) as T;
    } finally {
      clearTimeout(timer);
    }
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }
}

let _defaultClient: BlockscoutClient | null = null;

export function initBlockscout(config: BlockscoutClientConfig): BlockscoutClient {
  _defaultClient = new BlockscoutClient(config);
  return _defaultClient;
}

export function getBlockscoutClient(): BlockscoutClient {
  if (!_defaultClient) {
    throw new Error(
      '[@paxeer/wallet-data] BlockscoutClient not initialised. Call initBlockscout({ baseUrl }) first.'
    );
  }
  return _defaultClient;
}

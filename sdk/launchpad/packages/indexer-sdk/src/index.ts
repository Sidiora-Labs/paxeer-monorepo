export interface HttpClientConfig {
  baseUrl: string;
  headers?: Readonly<Record<string, string>>;
  fetch?: typeof globalThis.fetch;
  /** Total request deadline, including the response body (1–120000 ms). */
  timeoutMs?: number;
}

const DEFAULT_TIMEOUT_MS = 10_000;
const MAX_TIMEOUT_MS = 120_000;
// The transactions endpoint returns 50 trades by default; allow a generous page
// while keeping upstream errors substantially smaller than successful pages.
const MAX_RESPONSE_BYTES = 1024 * 1024;
const MAX_ERROR_RESPONSE_BYTES = 16 * 1024;

export class HttpError extends Error {
  constructor(public readonly status: number, public readonly body: string, public readonly url: string) {
    super(`Indexer data request failed (${status})`);
    this.name = 'HttpError';
  }
}

export class ResponseTooLargeError extends Error {
  constructor(public readonly status: number, public readonly limitBytes: number, public readonly url: string) {
    super(`Indexer response exceeds ${limitBytes} bytes`);
    this.name = 'ResponseTooLargeError';
  }
}

async function readBoundedBody(response: Response, limitBytes: number, url: string, controller: AbortController): Promise<string> {
  if (!response.body) return '';
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const parts: string[] = [];
  let bytes = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      bytes += value.byteLength;
      if (bytes > limitBytes) {
        const error = new ResponseTooLargeError(response.status, limitBytes, url);
        controller.abort(error);
        await reader.cancel(error);
        throw error;
      }
      parts.push(decoder.decode(value, { stream: true }));
    }
    parts.push(decoder.decode());
    return parts.join('');
  } finally {
    reader.releaseLock();
  }
}

export interface Trade {
  id: string;
  sender: string;
  isBuy: boolean;
  amountIn: string;
  amountOut: string;
  price: string;
  fee: string;
  blockTimestamp: number;
  txHash: string;
}
export interface RecentTrades { pool: string; trades: Trade[] | null }

export class HttpClient {
  private config?: HttpClientConfig;

  configure(config: HttpClientConfig): void {
    const url = new URL(config.baseUrl);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') throw new TypeError('baseUrl must use HTTP or HTTPS');
    if (url.search || url.hash || url.username || url.password) throw new TypeError('baseUrl must not contain query, fragment or credentials');
    const timeoutMs = config.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    if (!Number.isInteger(timeoutMs) || timeoutMs < 1 || timeoutMs > MAX_TIMEOUT_MS) {
      throw new TypeError('timeoutMs must be an integer between 1 and 120000');
    }
    this.config = { ...config, baseUrl: config.baseUrl.replace(/\/+$/, ''), headers: { ...config.headers }, timeoutMs };
  }

  async get(path: string, options: { signal?: AbortSignal } = {}): Promise<unknown> {
    const config = this.config;
    if (!config) throw new Error('Configure indexer-sdk.httpClient before requesting data');
    if (!path.startsWith('/') || path.startsWith('//') || path.includes('://')) throw new TypeError('Expected a resource path');
    const url = config.baseUrl + path;
    const controller = new AbortController();
    const signal = options.signal;
    const abort = () => controller.abort(signal?.reason);
    if (signal?.aborted) abort();
    else signal?.addEventListener('abort', abort, { once: true });
    const timer = setTimeout(() => controller.abort(new DOMException('Indexer request timed out', 'TimeoutError')), config.timeoutMs);
    try {
      controller.signal.throwIfAborted();
      const response = await (config.fetch ?? globalThis.fetch)(url, {
        method: 'GET', headers: { Accept: 'application/json', ...config.headers }, signal: controller.signal,
      });
      const text = await readBoundedBody(response, response.ok ? MAX_RESPONSE_BYTES : MAX_ERROR_RESPONSE_BYTES, url, controller);
      if (!response.ok) throw new HttpError(response.status, text, url);
      return JSON.parse(text) as unknown;
    } catch (error) {
      if (controller.signal.aborted) throw controller.signal.reason;
      throw error;
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', abort);
    }
  }

  async recentTrades(poolAddress: string, options: { signal?: AbortSignal } = {}): Promise<RecentTrades> {
    if (!/^0x[0-9a-fA-F]{40}$/.test(poolAddress)) throw new TypeError('Invalid pool address');
    const body = await this.get(`/stats/${poolAddress}/transactions`, options);
    if (!isRecord(body) || !Array.isArray(body.transactions) || !body.transactions.every(isTrade)) {
      throw new TypeError('Invalid recent trades response');
    }
    return { pool: poolAddress, trades: body.transactions };
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
function isTrade(value: unknown): value is Trade {
  if (!isRecord(value)) return false;
  return ['id', 'sender', 'amountIn', 'amountOut', 'price', 'fee', 'txHash'].every(key => typeof value[key] === 'string')
    && typeof value.isBuy === 'boolean' && typeof value.blockTimestamp === 'number' && Number.isSafeInteger(value.blockTimestamp);
}

export const httpClient = new HttpClient();
export const getRecentTrades = (poolAddress: string, options?: { signal?: AbortSignal }): Promise<RecentTrades> => httpClient.recentTrades(poolAddress, options);

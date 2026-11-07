export interface Socials {
  website: string | null;
  twitter: string | null;
  telegram: string | null;
  discord: string | null;
}
export interface PublicMetadata {
  token_address: string;
  pool_address: string | null;
  name: string | null;
  symbol: string | null;
  decimals: number;
  total_supply: string;
  creator: string | null;
  description: string | null;
  socials: Socials;
  tags: string[];
  images: { logo: string | null; banner: string | null };
  created_at: number | null;
  updated_at: number | null;
}
export interface UploadMetadata {
  name?: string;
  symbol?: string;
  description?: string;
  website?: string;
  twitter?: string;
  telegram?: string;
  discord?: string;
  tags?: string[];
  decimals?: number;
}
export interface UploadRequest {
  wallet: string;
  signature: string;
  message: string;
  metadata?: UploadMetadata;
  logo?: Blob;
  banner?: Blob;
}
export interface UploadResponse {
  success: boolean;
  metadata_updated: boolean;
  logo_url: string | null;
  banner_url: string | null;
  errors?: string[];
}
export class MetadataHttpError extends Error {
  readonly status: number;
  readonly body: string;
  constructor(status: number, body: string) {
    super(`Metadata HTTP ${status}: ${body}`);
    this.name = 'MetadataHttpError';
    this.status = status;
    this.body = body;
  }
}
export class MetadataUploadError extends Error {
  readonly response: UploadResponse;
  constructor(response: UploadResponse) {
    super(response.errors?.join('; ') || 'Metadata upload failed');
    this.name = 'MetadataUploadError';
    this.response = response;
  }
}
export class MetadataProtocolError extends TypeError {
  constructor(field: string) {
    super(`Invalid metadata response: ${field}`);
    this.name = 'MetadataProtocolError';
  }
}
function object(value: unknown, field: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new MetadataProtocolError(field);
  return value as Record<string, unknown>;
}
function string(value: unknown, field: string): string {
  if (typeof value !== 'string') throw new MetadataProtocolError(field);
  return value;
}
function nullableString(value: unknown, field: string): string | null {
  return value === null ? null : string(value, field);
}
function integer(value: unknown, field: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new MetadataProtocolError(field);
  return value;
}
function nullableInteger(value: unknown, field: string): number | null {
  return value === null ? null : integer(value, field);
}
function boolean(value: unknown, field: string): boolean {
  if (typeof value !== 'boolean') throw new MetadataProtocolError(field);
  return value;
}
function strings(value: unknown, field: string): string[] {
  if (!Array.isArray(value)) throw new MetadataProtocolError(field);
  return value.map((entry: unknown) => string(entry, field));
}
function parseMetadata(value: unknown): PublicMetadata {
  const data = object(value, 'metadata');
  const socials = object(data.socials, 'socials');
  const images = object(data.images, 'images');
  const decimals = integer(data.decimals, 'decimals');
  if (decimals < -2147483648 || decimals > 2147483647) throw new MetadataProtocolError('decimals');
  return {
    token_address: string(data.token_address, 'token_address'),
    pool_address: nullableString(data.pool_address, 'pool_address'),
    name: nullableString(data.name, 'name'),
    symbol: nullableString(data.symbol, 'symbol'),
    decimals,
    total_supply: string(data.total_supply, 'total_supply'),
    creator: nullableString(data.creator, 'creator'),
    description: nullableString(data.description, 'description'),
    socials: {
      website: nullableString(socials.website, 'socials.website'),
      twitter: nullableString(socials.twitter, 'socials.twitter'),
      telegram: nullableString(socials.telegram, 'socials.telegram'),
      discord: nullableString(socials.discord, 'socials.discord'),
    },
    tags: strings(data.tags, 'tags'),
    images: { logo: nullableString(images.logo, 'images.logo'), banner: nullableString(images.banner, 'images.banner') },
    created_at: nullableInteger(data.created_at, 'created_at'),
    updated_at: nullableInteger(data.updated_at, 'updated_at'),
  };
}
function parseBatch(value: unknown): Record<string, PublicMetadata> {
  const data = object(value, 'batch');
  return Object.fromEntries(Object.entries(data).map(([key, entry]) => [key, parseMetadata(entry)]));
}
function parseUpload(value: unknown): UploadResponse {
  const data = object(value, 'upload');
  return {
    success: boolean(data.success, 'success'),
    metadata_updated: boolean(data.metadata_updated, 'metadata_updated'),
    logo_url: nullableString(data.logo_url, 'logo_url'),
    banner_url: nullableString(data.banner_url, 'banner_url'),
    ...(data.errors === undefined ? {} : { errors: strings(data.errors, 'errors') }),
  };
}
export const METADATA_DEFAULT_TIMEOUT_MS = 10_000;
export const METADATA_MAX_TIMEOUT_MS = 120_000;
export const METADATA_RESPONSE_LIMITS = {
  metadata: 256 * 1024,
  batch: 4 * 1024 * 1024,
  upload: 64 * 1024,
  error: 16 * 1024,
} as const;
export class MetadataTimeoutError extends Error {
  constructor(readonly timeoutMs: number) {
    super(`Metadata request exceeded ${timeoutMs}ms deadline`);
    this.name = 'MetadataTimeoutError';
  }
}
export class MetadataResponseTooLargeError extends RangeError {
  constructor(readonly status: number, readonly limitBytes: number) {
    super(`Metadata HTTP ${status} response exceeds ${limitBytes} bytes`);
    this.name = 'MetadataResponseTooLargeError';
  }
}
export interface ClientConfig { baseUrl: string; timeoutMs?: number }
export interface RequestOptions { signal?: AbortSignal }

async function readBoundedBody(response: Response, limitBytes: number): Promise<string> {
  const declaredLength = response.headers.get('content-length');
  if (declaredLength !== null && /^\d+$/.test(declaredLength) && Number(declaredLength) > limitBytes) {
    throw new MetadataResponseTooLargeError(response.status, limitBytes);
  }
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
      if (bytes > limitBytes) throw new MetadataResponseTooLargeError(response.status, limitBytes);
      parts.push(decoder.decode(value, { stream: true }));
    }
    parts.push(decoder.decode());
    return parts.join('');
  } finally {
    reader.releaseLock();
  }
}

function address(value: string): string {
  if (!/^0x[0-9a-fA-F]{40}$/.test(value)) throw new TypeError('Invalid EVM address');
  return value.toLowerCase();
}
export function createMetadataClient(initial?: ClientConfig) {
  let baseUrl: string | undefined;
  let timeoutMs = METADATA_DEFAULT_TIMEOUT_MS;
  function configure(config: ClientConfig): void {
    const url = new URL(config.baseUrl);
    if (!['http:', 'https:'].includes(url.protocol) || url.search || url.hash || url.username || url.password) {
      throw new TypeError('Metadata baseUrl must be an HTTP(S) URL without credentials, query or fragment');
    }
    const deadline = config.timeoutMs ?? METADATA_DEFAULT_TIMEOUT_MS;
    if (!Number.isSafeInteger(deadline) || deadline < 1 || deadline > METADATA_MAX_TIMEOUT_MS) {
      throw new RangeError(`Metadata timeoutMs must be an integer between 1 and ${METADATA_MAX_TIMEOUT_MS}`);
    }
    baseUrl = config.baseUrl.replace(/\/+$/, '');
    timeoutMs = deadline;
  }
  if (initial) configure(initial);
  async function request<T>(path: string, init: RequestInit, parse: (value: unknown) => T, limitBytes: number): Promise<T> {
    if (!baseUrl) throw new Error('Metadata client is not configured');
    const controller = new AbortController();
    const caller = init.signal;
    const cancel = () => controller.abort(caller?.reason);
    if (caller?.aborted) cancel();
    else caller?.addEventListener('abort', cancel, { once: true });
    const deadline = timeoutMs;
    const timer = setTimeout(() => controller.abort(new MetadataTimeoutError(deadline)), deadline);
    try {
      controller.signal.throwIfAborted();
      const response = await fetch(baseUrl + path, { ...init, signal: controller.signal });
      const body = await readBoundedBody(response, response.ok ? limitBytes : METADATA_RESPONSE_LIMITS.error);
      controller.signal.throwIfAborted();
      if (!response.ok) throw new MetadataHttpError(response.status, body);
      return parse(JSON.parse(body));
    } catch (error) {
      if (controller.signal.aborted) throw controller.signal.reason;
      // Abort unread response bytes on refusal while preserving the original typed error.
      controller.abort(error);
      throw error;
    } finally {
      clearTimeout(timer);
      caller?.removeEventListener('abort', cancel);
    }
  }
  return {
    configure,
    getMetadata(tokenAddress: string, options: RequestOptions = {}): Promise<PublicMetadata> {
      return request(`/metadata/${address(tokenAddress)}`, { signal: options.signal }, parseMetadata, METADATA_RESPONSE_LIMITS.metadata);
    },
    getMetadataBatch(tokenAddresses: readonly string[], options: RequestOptions = {}): Promise<Record<string, PublicMetadata>> {
      const addresses = [...new Set(tokenAddresses.map(address))];
      if (addresses.length > 100) throw new RangeError('Metadata batch supports at most 100 distinct addresses');
      const query = new URLSearchParams({ addresses: addresses.join(',') });
      return request(`/metadata/batch?${query}`, { signal: options.signal }, value => {
        const result = parseBatch(value);
        for (const token of addresses) {
          if (!Object.hasOwn(result, token)) throw new MetadataProtocolError(`batch missing ${token}`);
          if (result[token]?.token_address !== token) throw new MetadataProtocolError(`batch token_address ${token}`);
        }
        return result;
      }, METADATA_RESPONSE_LIMITS.batch);
    },
    async uploadMetadata(tokenAddress: string, input: UploadRequest, options: RequestOptions = {}): Promise<UploadResponse> {
      const token = address(tokenAddress);
      const wallet = address(input.wallet);
      if (!input.signature || !input.message) throw new TypeError('Signature and message are required');
      const body = new FormData();
      body.append('wallet', wallet);
      body.append('signature', input.signature);
      body.append('message', input.message);
      if (input.metadata !== undefined) body.append('metadata', JSON.stringify(input.metadata));
      if (input.logo) body.append('logo', input.logo, 'logo');
      if (input.banner) body.append('banner', input.banner, 'banner');
      const result = await request<UploadResponse>(`/metadata/${token}`, { method: 'POST', body, signal: options.signal }, parseUpload, METADATA_RESPONSE_LIMITS.upload);
      if (!result.success) throw new MetadataUploadError(result);
      return result;
    },
  };
}
export const httpClient = createMetadataClient();

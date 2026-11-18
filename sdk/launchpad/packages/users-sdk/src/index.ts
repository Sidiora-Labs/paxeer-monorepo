export interface Socials { website: string | null; twitter: string | null; telegram: string | null; discord: string | null }
export interface Images { avatar: string | null; banner: string | null }
export interface CreatedPool { pool_address: string; token_address: string; created_at: number }
export interface PublicProfile {
  wallet_address: string; display_name: string | null; bio: string | null;
  socials: Socials; images: Images; created_pools: CreatedPool[];
  created_at: number | null; updated_at: number | null;
}
export interface ProfileData { display_name: string; bio: string; website: string; twitter: string; telegram: string; discord: string }
export interface WalletAuthorization { signature: string; message: string }
export interface OperationSigner { signMessage(message: string): Promise<string> }
export interface UpdateProfileRequest { data: ProfileData }
export interface WatchlistEntry { pool_address: string; added_at: number }
export interface WatchlistResponse { wallet_address: string; pools: WatchlistEntry[] }
export interface SuccessResponse { success: boolean }
export interface UploadResponse extends SuccessResponse { url: string }
export interface ErrorResponse { statusCode: number; error: string; message: string }
type Decoder<T> = (value: unknown) => T;
export class UsersResponseError extends Error {
  constructor() { super('Malformed Users API response'); this.name = 'UsersResponseError'; }
}
function record(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new UsersResponseError();
  return value as Record<string, unknown>;
}
function string(value: unknown): string {
  if (typeof value !== 'string') throw new UsersResponseError();
  return value;
}
function integer(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new UsersResponseError();
  return value;
}
function nullable<T>(value: unknown, decode: Decoder<T>): T | null { return value === null ? null : decode(value); }
function array<T>(value: unknown, decode: Decoder<T>): T[] {
  if (!Array.isArray(value)) throw new UsersResponseError();
  return value.map(decode);
}
function profile(value: unknown): PublicProfile {
  const v = record(value), socials = record(v.socials), images = record(v.images);
  return {
    wallet_address: string(v.wallet_address), display_name: nullable(v.display_name, string), bio: nullable(v.bio, string),
    socials: { website: nullable(socials.website, string), twitter: nullable(socials.twitter, string), telegram: nullable(socials.telegram, string), discord: nullable(socials.discord, string) },
    images: { avatar: nullable(images.avatar, string), banner: nullable(images.banner, string) },
    created_pools: array(v.created_pools, value => { const p = record(value); return { pool_address: string(p.pool_address), token_address: string(p.token_address), created_at: integer(p.created_at) }; }),
    created_at: nullable(v.created_at, integer), updated_at: nullable(v.updated_at, integer),
  };
}
function watchlist(value: unknown): WatchlistResponse {
  const v = record(value);
  return { wallet_address: string(v.wallet_address), pools: array(v.pools, value => { const p = record(value); return { pool_address: string(p.pool_address), added_at: integer(p.added_at) }; }) };
}
function success(value: unknown): SuccessResponse {
  const v = record(value);
  if (typeof v.success !== 'boolean') throw new UsersResponseError();
  return { success: v.success };
}
function upload(value: unknown): UploadResponse {
  return { ...success(value), url: string(record(value).url) };
}
export class UsersApiError extends Error {
  constructor(public readonly status: number, public readonly body: unknown) {
    super(`Users API request failed (${status})`);
    this.name = 'UsersApiError';
  }
}
export const USERS_DEFAULT_TIMEOUT_MS = 10_000;
export const USERS_MAX_TIMEOUT_MS = 120_000;
export const USERS_RESPONSE_LIMITS = Object.freeze({
  profile: 1024 * 1024,
  watchlist: 1024 * 1024,
  mutation: 16 * 1024,
  upload: 16 * 1024,
  error: 16 * 1024,
});
export class UsersResponseTooLargeError extends UsersResponseError {
  constructor(public readonly limit: number) {
    super();
    this.name = 'UsersResponseTooLargeError';
    this.message = `Users API response exceeds ${limit} bytes`;
  }
}
export interface HttpClientConfig { baseUrl: string; timeoutMs?: number }
export interface RequestOptions { signal?: AbortSignal }
async function boundedResponseText(response: Response, limit: number): Promise<string> {
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
      if (bytes > limit) throw new UsersResponseTooLargeError(limit);
      parts.push(decoder.decode(value, { stream: true }));
    }
    parts.push(decoder.decode());
    return parts.join('');
  } catch (error) {
    await reader.cancel().catch(() => {});
    throw error;
  } finally {
    reader.releaseLock();
  }
}
function address(value: string): string {
  const normalized = value.trim().toLowerCase();
  if (!/^0x[a-f0-9]{40}$/.test(normalized)) throw new TypeError('Invalid EVM address');
  return normalized;
}
function authorization(value: OperationSigner): void {
  if (typeof value.signMessage !== 'function') throw new TypeError('A wallet message signer is required');
}
function hex(bytes: Uint8Array): string { return Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join(''); }
async function digest(body: string | ArrayBuffer): Promise<string> {
  const bytes = typeof body === 'string' ? new TextEncoder().encode(body) : body;
  return hex(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)));
}
async function operationHeaders(wallet: string, action: string, method: string, url: string, body: string, signer: OperationSigner, options?: RequestOptions): Promise<Record<string, string>> {
  options?.signal?.throwIfAborted();
  const issuedAt = Math.floor(Date.now() / 1000);
  const envelope = JSON.stringify({
    version: 1, chainId: '125', environment: 'launchpad-user-legacy', actor: wallet,
    action, method, path: new URL(url).pathname, bodyHash: await digest(body),
    nonce: hex(crypto.getRandomValues(new Uint8Array(32))), issuedAt, expiresAt: issuedAt + 300,
  }).replace(/[<>&\u2028\u2029]/g, character => `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`);
  const signature = await signer.signMessage(`KindleLaunch signed operation v1\n${envelope}`);
  options?.signal?.throwIfAborted();
  if (Math.floor(Date.now() / 1000) >= issuedAt + 300) throw new Error('Signed operation expired');
  let binary = '';
  for (const byte of new TextEncoder().encode(envelope)) binary += String.fromCharCode(byte);
  return {
    'X-Kindle-Operation': btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, ''),
    'X-Kindle-Operation-Signature': signature,
  };
}
export class UsersHttpClient {
  private baseUrl: string | undefined;
  private timeoutMs = USERS_DEFAULT_TIMEOUT_MS;
  constructor(config?: HttpClientConfig) { if (config) this.configure(config); }
  configure(config: HttpClientConfig): void {
    const url = new URL(config.baseUrl);
    if (!['http:', 'https:'].includes(url.protocol) || url.search || url.hash || url.username || url.password) {
      throw new TypeError('baseUrl must be an HTTP(S) service root without credentials, query or fragment');
    }
    const timeoutMs = config.timeoutMs ?? USERS_DEFAULT_TIMEOUT_MS;
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0 || timeoutMs > USERS_MAX_TIMEOUT_MS) {
      throw new RangeError(`timeoutMs must be an integer between 1 and ${USERS_MAX_TIMEOUT_MS}`);
    }
    this.baseUrl = config.baseUrl.replace(/\/+$/, '');
    this.timeoutMs = timeoutMs;
  }
  private url(path: string): string {
    if (this.baseUrl === undefined) throw new Error('Users SDK baseUrl is not configured');
    return this.baseUrl + path;
  }
  private async request<T>(path: string, init: RequestInit, decode: Decoder<T>, limit: number, options: RequestOptions = {}, url = this.url(path)): Promise<T> {
    const controller = new AbortController();
    const caller = options.signal;
    const abort = () => controller.abort(caller?.reason);
    const timer = setTimeout(() => controller.abort(new DOMException('Users API request timed out', 'TimeoutError')), this.timeoutMs);
    try {
      if (caller?.aborted) abort();
      else caller?.addEventListener('abort', abort, { once: true });
      controller.signal.throwIfAborted();
      const response = await fetch(url, { ...init, signal: controller.signal });
      const text = await boundedResponseText(response, response.ok ? limit : USERS_RESPONSE_LIMITS.error);
      controller.signal.throwIfAborted();
      if (!response.ok) {
        let body: unknown = text;
        try { body = JSON.parse(text); } catch { /* Non-JSON proxy errors retain their original text. */ }
        throw new UsersApiError(response.status, body);
      }
      let value: unknown;
      try { value = JSON.parse(text); } catch { throw new UsersResponseError(); }
      return decode(value);
    } catch (error) {
      if (controller.signal.aborted) throw controller.signal.reason;
      controller.abort();
      throw error;
    } finally {
      clearTimeout(timer);
      caller?.removeEventListener('abort', abort);
    }
  }
  private async signed(path: string, method: string, wallet: string, action: string, body: string, signer: OperationSigner, options?: RequestOptions): Promise<SuccessResponse> {
    const url = this.url(path);
    const headers = await operationHeaders(wallet, action, method, url, body, signer, options);
    return this.request(path, { method, headers: { ...headers, 'Content-Type': 'application/json' }, body }, success, USERS_RESPONSE_LIMITS.mutation, options, url);
  }
  getProfile(wallet: string, options?: RequestOptions): Promise<PublicProfile> {
    return this.request(`/users/${address(wallet)}`, { method: 'GET' }, profile, USERS_RESPONSE_LIMITS.profile, options);
  }
  updateProfile(wallet: string, body: UpdateProfileRequest, signer: OperationSigner, options?: RequestOptions): Promise<SuccessResponse> {
    authorization(signer);
    const actor = address(wallet);
    return this.signed(`/users/${actor}`, 'POST', actor, 'profile.update', JSON.stringify({ data: body.data }), signer, options);
  }
  getWatchlist(wallet: string, options?: RequestOptions): Promise<WatchlistResponse> {
    return this.request(`/users/${address(wallet)}/watchlist`, { method: 'GET' }, watchlist, USERS_RESPONSE_LIMITS.watchlist, options);
  }
  addWatchlist(wallet: string, pool: string, signer: OperationSigner, options?: RequestOptions): Promise<SuccessResponse> {
    authorization(signer);
    const actor = address(wallet);
    return this.signed(`/users/${actor}/watchlist/${address(pool)}`, 'PUT', actor, 'watchlist.update', '{}', signer, options);
  }
  removeWatchlist(wallet: string, pool: string, signer: OperationSigner, options?: RequestOptions): Promise<SuccessResponse> {
    authorization(signer);
    const actor = address(wallet);
    return this.signed(`/users/${actor}/watchlist/${address(pool)}`, 'DELETE', actor, 'watchlist.remove', '{}', signer, options);
  }
  imageUrl(wallet: string, kind: 'avatar' | 'banner'): string {
    return this.url(`/users/${address(wallet)}/${kind}`);
  }
  async uploadImage(wallet: string, kind: 'avatar' | 'banner', file: Blob, signer: OperationSigner, options?: RequestOptions): Promise<UploadResponse> {
    authorization(signer);
    const actor = address(wallet);
    const path = `/users/${actor}/${kind}`;
    const url = this.url(path);
    const form = new FormData();
    form.append(kind, file);
    const encoded = new Request(url, { method: 'POST', body: form });
    const contentType = encoded.headers.get('Content-Type')!;
    const body = await encoded.arrayBuffer();
    const intent = JSON.stringify({ content_type: contentType, sha256: await digest(body) });
    const headers = await operationHeaders(actor, `image.${kind}.update`, 'POST', url, intent, signer, options);
    return this.request(path, { method: 'POST', headers: { ...headers, 'Content-Type': contentType }, body }, upload, USERS_RESPONSE_LIMITS.upload, options, url);
  }
}
export const httpClient = new UsersHttpClient();

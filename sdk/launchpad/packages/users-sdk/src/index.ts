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
export interface UpdateProfileRequest extends WalletAuthorization { data: ProfileData }
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
function authorization(value: WalletAuthorization): void {
  if (!value.signature || !value.message) throw new TypeError('Signature and message are required');
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
  private async request<T>(path: string, init: RequestInit, decode: Decoder<T>, limit: number, options: RequestOptions = {}): Promise<T> {
    const url = this.url(path);
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
  private signed(path: string, method: string, body: WalletAuthorization, options?: RequestOptions): Promise<SuccessResponse> {
    authorization(body);
    return this.request(path, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }, success, USERS_RESPONSE_LIMITS.mutation, options);
  }
  getProfile(wallet: string, options?: RequestOptions): Promise<PublicProfile> {
    return this.request(`/users/${address(wallet)}`, { method: 'GET' }, profile, USERS_RESPONSE_LIMITS.profile, options);
  }
  updateProfile(wallet: string, body: UpdateProfileRequest, options?: RequestOptions): Promise<SuccessResponse> {
    return this.signed(`/users/${address(wallet)}`, 'POST', body, options);
  }
  getWatchlist(wallet: string, options?: RequestOptions): Promise<WatchlistResponse> {
    return this.request(`/users/${address(wallet)}/watchlist`, { method: 'GET' }, watchlist, USERS_RESPONSE_LIMITS.watchlist, options);
  }
  addWatchlist(wallet: string, pool: string, body: WalletAuthorization, options?: RequestOptions): Promise<SuccessResponse> {
    return this.signed(`/users/${address(wallet)}/watchlist/${address(pool)}`, 'PUT', body, options);
  }
  removeWatchlist(wallet: string, pool: string, body: WalletAuthorization, options?: RequestOptions): Promise<SuccessResponse> {
    return this.signed(`/users/${address(wallet)}/watchlist/${address(pool)}`, 'DELETE', body, options);
  }
  imageUrl(wallet: string, kind: 'avatar' | 'banner'): string {
    return this.url(`/users/${address(wallet)}/${kind}`);
  }
  uploadImage(wallet: string, kind: 'avatar' | 'banner', file: Blob, auth: WalletAuthorization, options?: RequestOptions): Promise<UploadResponse> {
    authorization(auth);
    const form = new FormData();
    form.append(kind, file);
    form.append('signature', auth.signature);
    form.append('message', auth.message);
    return this.request(`/users/${address(wallet)}/${kind}`, { method: 'POST', body: form }, upload, USERS_RESPONSE_LIMITS.upload, options);
  }
}
export const httpClient = new UsersHttpClient();

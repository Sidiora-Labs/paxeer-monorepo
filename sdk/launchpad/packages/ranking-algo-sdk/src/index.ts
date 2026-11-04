export const rankingCategories = ['trending', 'breakout', 'volume', 'gainers', 'losers', 'movers', 'unusual', 'new'] as const;
export type RankingCategory = typeof rankingCategories[number];
export interface RankingWatermark {
  chainId: '125'; height: string; blockHash: string; projectionRevision: string;
  observedAt: string; integrity: 'healthy' | 'quarantined';
}
export interface RankingItem {
  poolAddress: string; category: RankingCategory; score: string; rank: number;
  modelVersion: string; computedAt: string;
}
export interface RankingSnapshot {
  snapshotId: string; category: RankingCategory; modelVersion: string; computedAt: string;
  watermark: RankingWatermark; items: RankingItem[]; totalEligible: string;
  truncated: boolean; replace: true;
}
export interface RankingFreshness {
  cutoffHeight: string; sourceObservedAt: string; generatedAt: string; ageSeconds: number;
  stale: boolean; staleReason: string | null; retryAfterSeconds: number;
}
export interface RankingsResponse {
  data: { snapshot: RankingSnapshot }; requestId: string; generatedAt: string;
  nextCursor: string | null; revision: string | null; watermark: RankingWatermark | null;
  quality: 'complete' | 'partial' | 'stale' | 'quarantined'; freshness?: RankingFreshness;
}
export interface RankingErrorResponse {
  code: string; message: string; requestId: string; retryable: boolean; retryAfterMs: number | null;
  fieldErrors: { pointer: string; rule: string }[];
  details: {
    capability?: string; dependency?: string; latestWatermark?: RankingWatermark | null;
    minimumVersion?: string; resource?: string;
  } | null;
  supported?: RankingCategory[]; restart?: boolean; retainLastResult?: boolean;
}
export interface RankingQuery { limit?: number; cursor?: string | null; signal?: AbortSignal }
export interface HttpClientConfig { baseUrl: string; headers?: HeadersInit }
export class RankingHttpError extends Error {
  readonly status: number;
  readonly body: string;
  readonly error: RankingErrorResponse | undefined;
  readonly retryAfter: string | null;
  constructor(status: number, body: string, retryAfter: string | null = null) {
    super(`Ranking request failed: HTTP ${status}`);
    this.name = 'RankingHttpError'; this.status = status; this.body = body;
    this.retryAfter = retryAfter;
    let value: unknown;
    try { value = JSON.parse(body); } catch (error) {
      if (!(error instanceof SyntaxError)) throw error;
    }
    this.error = rankingError(value) ? value : undefined;
  }
}
function object(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function nonnegativeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}
function category(value: unknown): value is RankingCategory {
  return typeof value === 'string' && rankingCategories.some((entry) => entry === value);
}
function uint256(value: unknown, positive = false): value is string {
  return typeof value === 'string' && /^(?:0|[1-9][0-9]{0,77})$/.test(value)
    && (!positive || value !== '0') && BigInt(value) < (1n << 256n);
}
function timestamp(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}Z$/.test(value)
    && !value.startsWith('0000') && Number.isFinite(Date.parse(value)) && new Date(value).toISOString() === value;
}
function uuid(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
}
function watermark(value: unknown): value is RankingWatermark {
  return object(value) && value.chainId === '125' && uint256(value.height)
    && typeof value.blockHash === 'string' && /^0x[0-9a-fA-F]{64}$/.test(value.blockHash)
    && uint256(value.projectionRevision, true) && timestamp(value.observedAt)
    && (value.integrity === 'healthy' || value.integrity === 'quarantined');
}
function item(value: unknown): value is RankingItem {
  return object(value) && typeof value.poolAddress === 'string' && /^0x[0-9a-fA-F]{40}$/.test(value.poolAddress)
    && category(value.category) && typeof value.score === 'string' && value.score.length <= 512
    && /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$/.test(value.score)
    && nonnegativeInteger(value.rank) && value.rank > 0
    && typeof value.modelVersion === 'string' && value.modelVersion.length <= 64 && timestamp(value.computedAt);
}
function snapshot(value: unknown, requested: RankingCategory): value is RankingSnapshot {
  return object(value) && uuid(value.snapshotId) && value.category === requested
    && typeof value.modelVersion === 'string' && value.modelVersion.length <= 64 && timestamp(value.computedAt)
    && watermark(value.watermark) && uint256(value.totalEligible)
    && typeof value.truncated === 'boolean' && value.replace === true
    && Array.isArray(value.items) && value.items.length <= 100
    && value.items.every((entry) => item(entry) && entry.category === requested && entry.modelVersion === value.modelVersion);
}
function freshness(value: unknown): value is RankingFreshness {
  return object(value) && uint256(value.cutoffHeight) && timestamp(value.sourceObservedAt) && timestamp(value.generatedAt)
    && nonnegativeInteger(value.ageSeconds) && typeof value.stale === 'boolean'
    && (value.staleReason === null || typeof value.staleReason === 'string') && nonnegativeInteger(value.retryAfterSeconds);
}
function rankings(value: unknown, requested: RankingCategory): value is RankingsResponse {
  return object(value) && object(value.data) && snapshot(value.data.snapshot, requested)
    && uuid(value.requestId) && timestamp(value.generatedAt)
    && (value.nextCursor === null || (typeof value.nextCursor === 'string' && value.nextCursor.length <= 2048))
    && (value.revision === null || uint256(value.revision, true))
    && (value.watermark === null || watermark(value.watermark))
    && ['complete', 'partial', 'stale', 'quarantined'].some((quality) => value.quality === quality)
    && (value.freshness === undefined || freshness(value.freshness));
}
function errorDetails(value: unknown): value is RankingErrorResponse['details'] {
  if (value === null) return true;
  if (!object(value) || Object.keys(value).some((key) => !['capability', 'dependency', 'latestWatermark', 'minimumVersion', 'resource'].includes(key))) return false;
  return (value.capability === undefined || (typeof value.capability === 'string' && value.capability.length <= 80))
    && (value.dependency === undefined || (typeof value.dependency === 'string' && value.dependency.length <= 80))
    && (value.resource === undefined || (typeof value.resource === 'string' && value.resource.length <= 160))
    && (value.minimumVersion === undefined || (typeof value.minimumVersion === 'string' && value.minimumVersion.length <= 32))
    && (value.latestWatermark === undefined || value.latestWatermark === null || watermark(value.latestWatermark));
}
function rankingError(value: unknown): value is RankingErrorResponse {
  return object(value) && typeof value.code === 'string' && value.code.length <= 64 && /^[A-Z][A-Z0-9_]*$/.test(value.code)
    && typeof value.message === 'string' && value.message.length <= 256 && uuid(value.requestId)
    && typeof value.retryable === 'boolean' && errorDetails(value.details)
    && (value.retryAfterMs === null || (nonnegativeInteger(value.retryAfterMs) && value.retryAfterMs <= 60000))
    && Array.isArray(value.fieldErrors) && value.fieldErrors.length <= 32
    && value.fieldErrors.every((entry) => object(entry) && typeof entry.pointer === 'string' && typeof entry.rule === 'string')
    && (value.supported === undefined || (Array.isArray(value.supported) && value.supported.every(category)))
    && (value.restart === undefined || typeof value.restart === 'boolean')
    && (value.retainLastResult === undefined || typeof value.retainLastResult === 'boolean');
}
function parseRankings(value: unknown, requested: RankingCategory): RankingsResponse {
  if (!rankings(value, requested)) throw new TypeError('Invalid rankings response');
  return value;
}
export class RankingHttpClient {
  private config: HttpClientConfig | undefined;
  configure(config: HttpClientConfig): void {
    const url = new URL(config.baseUrl);
    if (!['http:', 'https:'].includes(url.protocol) || url.search || url.hash || url.username || url.password) {
      throw new TypeError('Ranking baseUrl must be an HTTP(S) URL without credentials, query or fragment');
    }
    this.config = { baseUrl: url.href.replace(/\/$/, ''), headers: new Headers(config.headers) };
  }
  async getRankings(requested: RankingCategory, query: RankingQuery = {}): Promise<RankingsResponse> {
    if (!this.config) throw new Error('Ranking HTTP client is not configured');
    if (!category(requested)) throw new TypeError('Invalid ranking category');
    if ('offset' in query) throw new TypeError('Ranking snapshots require cursor pagination');
    const url = new URL(`${this.config.baseUrl}/rankings/${requested}`);
    if (query.limit !== undefined) {
      if (!nonnegativeInteger(query.limit) || query.limit < 1 || query.limit > 100) throw new RangeError('limit must be an integer from 1 to 100');
      url.searchParams.set('limit', String(query.limit));
    }
    if (query.cursor !== undefined && query.cursor !== null) {
      if (typeof query.cursor !== 'string' || query.cursor.length > 2048) throw new TypeError('cursor must be a string of at most 2048 characters');
      url.searchParams.set('cursor', query.cursor);
    }
    const headers = new Headers(this.config.headers);
    if (!headers.has('Accept')) headers.set('Accept', 'application/json');
    const response = await fetch(url, { headers, signal: query.signal });
    if (!response.ok) throw new RankingHttpError(response.status, await response.text(), response.headers.get('Retry-After'));
    const body: unknown = await response.json();
    return parseRankings(body, requested);
  }
}
export const httpClient = new RankingHttpClient();
export function getRankings(category: RankingCategory, query?: RankingQuery): Promise<RankingsResponse> {
  return httpClient.getRankings(category, query);
}

/**
 * Same-origin proxy for the wallet's Blockscout API.
 *
 * Why this exists:
 *   - Hides `BLOCKSCOUT_UPSTREAM_BASE` from clients (no NEXT_PUBLIC_*).
 *   - Same-origin requests → zero CORS friction for PWA + Capacitor.
 *   - Single chokepoint for rate limiting, auth, and server-side caching.
 *
 * Routing:
 *   Client calls `/api/wallet/api/v2/addresses/0x.../tokens?type=ERC-20`
 *   This handler forwards to
 *     `${BLOCKSCOUT_UPSTREAM_BASE}/api/v2/addresses/0x.../tokens?type=ERC-20`
 *   The `[...path]` segments are joined with `/` and the original query
 *   string is preserved.
 *
 * Methods:
 *   The Blockscout v2 API consumed by `@paxeer/wallet-data` is read-only,
 *   so its routes only allow GET / HEAD. Watch and webhook lifecycle routes
 *   forward bearer credentials to the portfolio producer for authentication
 *   and ownership checks; their responses are never shared or cached.
 *
 * Caching strategy (server-side, in-process):
 *   Root cause of upstream 429s: every user's request hits api.paxscan.io
 *   from the same server IP, burning a shared rate-limit bucket.
 *
 *   Two mechanisms protect the upstream:
 *   1. Response cache  — successful upstream responses are stored in memory
 *      for a TTL that scales with how frequently the data changes.
 *   2. In-flight deduplication — if 50 users simultaneously request the same
 *      address/token list, only ONE fetch goes upstream; all 50 share the
 *      result. This collapses thundering-herd bursts to a single call.
 *
 *   Cache TTLs (conservative — prefer freshness over upstream savings):
 *     transactions / token-transfers  20 s  (new txs land every few seconds)
 *     token holdings / NFTs           45 s  (balance changes less often)
 *     address info / stats / counters 90 s  (very stable)
 *     everything else                 30 s
 */

import { NextResponse, type NextRequest } from 'next/server';
import { walletLimiter } from '@/lib/rateLimit';
import { PAXEER_CONFIG } from '@/lib/constants';
import { parseWalletReadChainId, WALLET_CHAIN_HEADER } from '@/lib/wallet-read-contract';
import {
  boundaryErrorResponse,
  encodePathSegments,
  HttpBoundaryError,
  publicError,
  readBoundedUpstream,
  readBoundedJson,
  timeoutSignal,
  trustedClientIdentity,
  validateQuery,
} from '@/server/http';

export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

const UPSTREAM_BASE = (process.env.BLOCKSCOUT_UPSTREAM_BASE || 'https://api.paxscan.io').replace(
  /\/+$/,
  ''
);

// ── Server-side cache ─────────────────────────────────────────────────────────

interface CacheEntry {
  body: string;
  status: number;
  headers: Record<string, string>;
  expiresAt: number;
  cacheBytes: number;
}

// Both maps use the effective method and full URL (path + query string).
const responseCache = new Map<string, CacheEntry>();
let responseCacheBytes = 0;

// Duplicate requests share work, including when all upstream slots are occupied.
const inFlight = new Map<string, Promise<CacheEntry>>();

const MAX_CACHE_BYTES = 512 * 1024;
const MAX_CACHE_ENTRIES = 256;
const MAX_TOTAL_CACHE_BYTES = 8 * 1024 * 1024;
const MAX_IN_FLIGHT = 16;

function deleteCachedEntry(key: string): void {
  const entry = responseCache.get(key);
  if (!entry) return;
  responseCacheBytes -= entry.cacheBytes;
  responseCache.delete(key);
}

function pruneExpiredEntries(now: number): void {
  for (const [key, entry] of responseCache) {
    if (now >= entry.expiresAt) deleteCachedEntry(key);
  }
}

// Expiry cleanup supplements the budgets enforced on every insertion.
const pruneTimer = setInterval(() => {
  pruneExpiredEntries(Date.now());
}, 2 * 60_000);
if (pruneTimer.unref) pruneTimer.unref();

function cacheResponse(key: string, entry: CacheEntry, segments: string[]): void {
  if (Buffer.byteLength(entry.body, 'utf8') > MAX_CACHE_BYTES) return;
  entry.cacheBytes =
    Buffer.byteLength(key, 'utf8') +
    Buffer.byteLength(entry.body, 'utf8') +
    Buffer.byteLength(JSON.stringify(entry.headers), 'utf8');
  if (entry.cacheBytes > MAX_TOTAL_CACHE_BYTES) return;

  const now = Date.now();
  pruneExpiredEntries(now);
  deleteCachedEntry(key);
  // Map insertion order tracks least recently used entries.
  while (
    responseCache.size >= MAX_CACHE_ENTRIES ||
    responseCacheBytes + entry.cacheBytes > MAX_TOTAL_CACHE_BYTES
  ) {
    const oldest = responseCache.keys().next();
    if (oldest.done) break;
    deleteCachedEntry(oldest.value);
  }
  entry.expiresAt = now + cacheTtlMs(segments);
  responseCache.set(key, entry);
  responseCacheBytes += entry.cacheBytes;
}

function cacheTtlMs(segments: string[]): number {
  const path = segments.join('/').toLowerCase();
  if (path.includes('transactions') || path.includes('token-transfers')) return 20_000;
  if (path.includes('tokens') || path.includes('nft')) return 45_000;
  if (path.includes('stats') || path.includes('counters') || path.includes('addresses'))
    return 90_000;
  return 30_000;
}

// ── Header helpers ────────────────────────────────────────────────────────────

function buildUpstreamUrl(req: NextRequest, pathSegments: string[]): string {
  const path = encodePathSegments(pathSegments);
  validateQuery(req, { maxPairs: 32, maxValueLength: 512 });
  const search = req.nextUrl.search;
  return `${UPSTREAM_BASE}/${path}${search}`;
}

// ── Core fetch with cache + deduplication ─────────────────────────────────────

async function fetchWithCache(
  upstreamUrl: string,
  segments: string[],
  req: NextRequest
): Promise<CacheEntry> {
  const now = Date.now();
  const key = `${req.method} ${upstreamUrl}`;

  // 1. Cache hit
  const cached = responseCache.get(key);
  if (cached && now < cached.expiresAt) {
    responseCache.delete(key);
    responseCache.set(key, cached);
    return cached;
  }
  if (cached) deleteCachedEntry(key);

  // 2. Deduplicate before admission so hot URLs remain available at capacity.
  const existing = inFlight.get(key);
  if (existing) return existing;
  if (inFlight.size >= MAX_IN_FLIGHT) {
    throw new HttpBoundaryError(503, 'UPSTREAM_BUSY', 'Wallet data is busy; retry shortly');
  }

  // 3. New upstream fetch
  const fetchPromise: Promise<CacheEntry> = (async () => {
    const timed = timeoutSignal(req.signal, 15_000);
    try {
      const upstreamRes = await fetch(upstreamUrl, {
        method: req.method,
        headers: {
          Accept: 'application/json',
          'X-Forwarded-By': 'paxport-wallet-proxy',
        },
        signal: timed.signal,
        cache: 'no-store',
        redirect: 'error',
      });

      const upstream = await readBoundedUpstream(upstreamRes, {
        maxBytes: 2_097_152,
        allowedContentTypes: new Set(['application/json']),
      });
      const body = Buffer.from(upstream.body).toString('utf-8');
      const entry: CacheEntry = {
        body,
        status: upstreamRes.status,
        headers: {
          'content-type': 'application/json; charset=utf-8',
          'cache-control':
            upstreamRes.headers.get('cache-control') ??
            'private, max-age=10, stale-while-revalidate=30',
        },
        expiresAt: 0,
        cacheBytes: 0,
      };

      // Only cache successful responses within the size limit
      if (upstreamRes.status === 200) cacheResponse(key, entry, segments);

      return entry;
    } finally {
      timed.dispose();
      inFlight.delete(key);
    }
  })();

  inFlight.set(key, fetchPromise);
  return fetchPromise;
}

// ── Route handler ─────────────────────────────────────────────────────────────

type RouteContext = { params: Promise<{ path: string[] }> };

async function forward(req: NextRequest, ctx: RouteContext): Promise<Response> {
  try {
    const portfolioSegments = (await ctx.params).path;
    if (
      portfolioSegments?.[0] === 'api' &&
      portfolioSegments[1] === 'v2' &&
      (portfolioSegments[2] === 'watch' || portfolioSegments[2] === 'webhooks')
    ) {
      return forwardPortfolio(req, ctx);
    }
    const rl = await walletLimiter.check(trustedClientIdentity(req));
    if (!rl.ok) {
      return NextResponse.json(
        { error: { code: 'RATE_LIMITED', message: 'Request limit exceeded' } },
        {
          status: 429,
          headers: {
            'Retry-After': String(rl.retryAfter ?? 60),
            'Cache-Control': 'no-store',
          },
        }
      );
    }
    if (!UPSTREAM_BASE) {
      return publicError(
        503,
        'WALLET_UPSTREAM_UNAVAILABLE',
        'Wallet data is temporarily unavailable'
      );
    }
    const params = await ctx.params;
    const segments = Array.isArray(params.path) ? params.path : [];
    const upstreamUrl = buildUpstreamUrl(req, segments);
    const entry = await fetchWithCache(upstreamUrl, segments, req);
    const headers = new Headers(entry.headers);
    const fromCache = entry.expiresAt > 0 && Date.now() < entry.expiresAt;
    headers.set('x-cache', fromCache ? 'HIT' : 'MISS');
    return new Response(req.method === 'HEAD' ? null : entry.body, {
      status: entry.status,
      headers,
    });
  } catch (err) {
    if (err instanceof HttpBoundaryError) return boundaryErrorResponse(err);
    const aborted = err instanceof Error && err.name === 'AbortError';
    return publicError(
      aborted ? 504 : 502,
      aborted ? 'UPSTREAM_TIMEOUT' : 'UPSTREAM_UNAVAILABLE',
      aborted ? 'Upstream request timed out' : 'Upstream request failed'
    );
  }
}

export async function GET(req: NextRequest, ctx: RouteContext) {
  return walletResponse(await forward(req, ctx));
}

export async function HEAD(req: NextRequest, ctx: RouteContext) {
  return walletResponse(await forward(req, ctx));
}

function walletResponse(response: Response): Response {
  response.headers.set(WALLET_CHAIN_HEADER, String(PAXEER_CONFIG.chainId));
  return response;
}

function methodNotAllowed(): Response {
  return NextResponse.json({ error: 'method not allowed' }, { status: 405 });
}

let portfolioInFlight = 0;

async function forwardPortfolio(req: NextRequest, ctx: RouteContext): Promise<Response> {
  let admitted = false;
  const timed = timeoutSignal(req.signal, 15_000);
  try {
    const segments = (await ctx.params).path;
    const path = encodePathSegments(segments, { maxSegments: 5 });
    const watch = /^api\/v2\/watch\/0x[0-9a-fA-F]{40}$/.test(path);
    const webhook = /^api\/v2\/webhooks\/[A-Za-z0-9_-]{1,128}$/.test(path);
    const deliveries = /^api\/v2\/webhooks\/[A-Za-z0-9_-]{1,128}\/deliveries$/.test(path);
    const allowed =
      (path === 'api/v2/watch' && req.method === 'POST') ||
      (watch && ['GET', 'HEAD', 'DELETE'].includes(req.method)) ||
      (path === 'api/v2/webhooks' && ['GET', 'HEAD', 'POST'].includes(req.method)) ||
      (webhook && ['GET', 'HEAD', 'PATCH', 'DELETE'].includes(req.method)) ||
      (deliveries && ['GET', 'HEAD'].includes(req.method));
    if (!allowed) return methodNotAllowed();

    // Presence is checked before reading the body. The producer validates the
    // credential and resource ownership; no server credential is substituted.
    const authorization = req.headers.get('authorization');
    if (!authorization || !/^Bearer [A-Za-z0-9._~+/-]{16,512}={0,2}$/.test(authorization)) {
      return publicError(401, 'PORTFOLIO_UNAUTHORIZED', 'Portfolio authorization is required');
    }
    const query = new URL(req.nextUrl.href);
    const hasReadChain =
      (req.method === 'GET' || req.method === 'HEAD') && query.searchParams.has('chain_id');
    validateQuery(req, { maxPairs: hasReadChain ? 2 : 1, maxValueLength: 128 });
    if (hasReadChain) {
      try {
        if (parseWalletReadChainId(query) !== PAXEER_CONFIG.chainId) {
          throw new TypeError('Portfolio chain is unavailable');
        }
      } catch {
        throw new HttpBoundaryError(400, 'QUERY_INVALID', 'Query is invalid');
      }
      query.searchParams.delete('chain_id');
    }
    for (const [key, value] of query.searchParams) {
      if (
        !(req.method === 'GET' || req.method === 'HEAD') ||
        (!(path === 'api/v2/webhooks' && key === 'address' && /^0x[0-9a-fA-F]{40}$/.test(value)) &&
          !(
            deliveries &&
            key === 'limit' &&
            /^[1-9][0-9]{0,2}$/.test(value) &&
            Number(value) <= 100
          ))
      ) {
        throw new HttpBoundaryError(400, 'QUERY_INVALID', 'Query is invalid');
      }
    }
    const configured = process.env.PORTFOLIO_UPSTREAM_BASE;
    let origin: URL;
    try {
      origin = new URL(configured ?? '');
      if (
        origin.username ||
        origin.password ||
        origin.search ||
        origin.hash ||
        origin.pathname !== '/' ||
        !(
          origin.protocol === 'https:' ||
          (origin.protocol === 'http:' &&
            ['127.0.0.1', '[::1]', 'localhost'].includes(origin.hostname))
        )
      )
        throw new Error();
    } catch {
      return publicError(503, 'PORTFOLIO_UNAVAILABLE', 'Portfolio service is not configured');
    }
    const rl = await walletLimiter.check(trustedClientIdentity(req));
    if (!rl.ok) {
      return NextResponse.json(
        { error: { code: 'RATE_LIMITED', message: 'Request limit exceeded' } },
        {
          status: 429,
          headers: { 'Retry-After': String(rl.retryAfter ?? 60), 'Cache-Control': 'no-store' },
        }
      );
    }
    if (portfolioInFlight >= MAX_IN_FLIGHT) {
      return publicError(503, 'UPSTREAM_BUSY', 'Wallet data is busy; retry shortly');
    }
    portfolioInFlight += 1;
    admitted = true;
    let body: string | undefined;
    if (req.method === 'POST' || req.method === 'PATCH') {
      const json = await readBoundedJson(req, 16_384);
      if (!json || typeof json !== 'object' || Array.isArray(json)) {
        throw new HttpBoundaryError(400, 'BODY_INVALID', 'Request body must be an object');
      }
      body = JSON.stringify(json);
    } else if (req.body !== null) {
      throw new HttpBoundaryError(400, 'BODY_INVALID', 'This method does not accept a body');
    }
    const response = await fetch(`${origin.origin}/${path}${query.search}`, {
      method: req.method,
      headers: {
        Accept: 'application/json',
        Authorization: authorization,
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body,
      signal: timed.signal,
      redirect: 'error',
      cache: 'no-store',
      credentials: 'omit',
    });
    const headers = new Headers({ 'Cache-Control': 'no-store' });
    for (const name of ['content-type', 'retry-after', 'www-authenticate']) {
      const value = response.headers.get(name);
      if (value !== null) headers.set(name, value);
    }
    if (req.method === 'HEAD' || response.status === 204 || response.status === 304) {
      await response.body?.cancel();
      return new Response(null, { status: response.status, headers });
    }
    const upstream = await readBoundedUpstream(response, {
      maxBytes: 2_097_152,
      allowedContentTypes: new Set(['application/json']),
    });
    return new Response(upstream.body, { status: response.status, headers });
  } catch (error) {
    if (error instanceof HttpBoundaryError) return boundaryErrorResponse(error);
    return publicError(
      timed.signal.aborted ? 504 : 502,
      timed.signal.aborted ? 'UPSTREAM_TIMEOUT' : 'UPSTREAM_UNAVAILABLE',
      timed.signal.aborted ? 'Upstream request timed out' : 'Upstream request failed'
    );
  } finally {
    if (admitted) portfolioInFlight -= 1;
    timed.dispose();
  }
}

export function POST(): Response;
export function POST(req: NextRequest, ctx: RouteContext): Promise<Response>;
export function POST(req?: NextRequest, ctx?: RouteContext): Response | Promise<Response> {
  // Preserve explicit refusal for legacy callers that supply no route request.
  if (!req || !ctx) return walletResponse(methodNotAllowed());
  return forwardPortfolio(req, ctx).then(walletResponse);
}
export function PUT() {
  return walletResponse(methodNotAllowed());
}
export async function PATCH(req: NextRequest, ctx: RouteContext) {
  return walletResponse(await forwardPortfolio(req, ctx));
}
export async function DELETE(req: NextRequest, ctx: RouteContext) {
  return walletResponse(await forwardPortfolio(req, ctx));
}

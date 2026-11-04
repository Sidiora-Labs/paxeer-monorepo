import { cacheGet, cacheSet } from '@/lib/metaCache';
import { rewriteLogoUrl } from '@/lib/mediaProxy';
import type { SwapToken } from './constants';
import {
  getSwapNetwork,
  quoteNetwork,
  assertQuoteNetwork,
  resolveTokenCapability,
} from './network';
import type { QuoteNetwork } from './network';
import { swapStorage, writeSwapRecord, type SwapStorage } from './storage';

const METADATA_BATCH_SIZE = 40;
const META_TTL = 24 * 60 * 60_000; // 24 h — logos/names are static
const LIST_TTL = 60 * 60_000; // 1 h  — list of known tokens
const META_CACHE_KEY = 'sidiora-meta-map';

/**
 * Enrich tokens with Sidiora metadata (name / symbol / decimals / logo).
 * - Loads the persisted metadata map from localStorage (24 h TTL).
 * - Only fetches addresses not already in cache.
 * - Merges new entries into cache and persists.
 */
async function enrichWithSidioraMetadata(
  tokens: SwapToken[],
  source: QuoteNetwork
): Promise<SwapToken[]> {
  if (tokens.length === 0) return tokens;

  // ── Load cached metadata map ──────────────────────────────────────────
  const cached: Record<string, any> =
    cacheGet<Record<string, any>>(
      `${META_CACHE_KEY}:${source.chainId}:${source.endpoint}`,
      META_TTL
    ) ?? {};

  // Only request addresses that are missing from cache
  const missing = tokens.filter((t) => !cached[t.address.toLowerCase()]);

  if (missing.length > 0) {
    for (let i = 0; i < missing.length; i += METADATA_BATCH_SIZE) {
      const chunk = missing.slice(i, i + METADATA_BATCH_SIZE);
      const addrList = chunk.map((t) => t.address).join(',');
      try {
        const res = await fetch(`${getSwapNetwork().metadataUrl}?addresses=${addrList}`);
        if (!res.ok) continue;
        const data: Record<string, any> = await res.json();
        for (const [k, v] of Object.entries(data)) {
          cached[k.toLowerCase()] = v;
        }
      } catch {
        // enrichment is non-critical
      }
    }
    // Persist the merged map
    assertQuoteNetwork(source);
    cacheSet(`${META_CACHE_KEY}:${source.chainId}:${source.endpoint}`, cached);
  }

  return tokens.map((t) => {
    const meta = cached[t.address.toLowerCase()];
    if (!meta) return t;
    return {
      ...t,
      name: meta.name || t.name,
      symbol: meta.symbol || t.symbol,
      decimals: typeof meta.decimals === 'number' ? meta.decimals : t.decimals,
      iconUrl: rewriteLogoUrl(meta.images?.logo) || t.iconUrl,
    };
  });
}

interface WalletApiToken {
  address_hash: string;
  name: string;
  symbol: string;
  decimals: string | null;
  type: string | null;
}

export interface TokenDiscovery {
  tokens: SwapToken[];
  unsupported: SwapToken[];
  complete: boolean;
  status: 'complete' | 'truncated' | 'failed';
  continuation: Record<string, string> | null;
  pages: number;
  scanned: number;
  updatedAt: number;
  error?: string;
}

// An array keeps the existing picker interface, while consumers can inspect completeness.
export type LaunchpadTokens = SwapToken[] & { discovery: TokenDiscovery };

function listKey(): string {
  const network = getSwapNetwork();
  return `pax:swap:tokens:v1:${network.chainId}:${encodeURIComponent(network.endpoint)}`;
}

function readInventory(storage: SwapStorage): TokenDiscovery | null {
  const raw = storage.getItem(listKey());
  if (raw === null) return null;
  const value: TokenDiscovery = JSON.parse(raw);
  if (
    !Array.isArray(value.tokens) ||
    !Array.isArray(value.unsupported) ||
    value.status !== 'complete' ||
    value.complete !== true ||
    !Number.isSafeInteger(value.updatedAt)
  )
    throw new Error('Invalid swap discovery cache');
  return value;
}

export async function fetchLaunchpadTokens(
  options: {
    storage?: SwapStorage;
    continuation?: Record<string, string>;
  } = {}
): Promise<LaunchpadTokens> {
  const source = quoteNetwork(Date.now() + 300_000);
  const network = getSwapNetwork();
  const storage = options.storage ?? swapStorage();
  const previous = readInventory(storage);
  const tokens: SwapToken[] = [];
  const unsupported: SwapToken[] = [];
  const seen = new Set<string>();
  const visited = new Set<string>();
  let continuation: Record<string, string> | null = options.continuation ?? null;
  let pages = 0;
  let scanned = 0;
  let complete = false;
  let error: string | undefined;
  try {
    while (pages < 10 && scanned < 500) {
      assertQuoteNetwork(source);
      const qs = new URLSearchParams({ ...continuation, type: 'ERC-20' });
      const url = `${network.indexerUrl.replace(/\/$/, '')}/api/v2/tokens?${qs}`;
      if (visited.has(url)) throw new Error('Token index returned a repeated cursor');
      visited.add(url);
      const response = await fetch(url);
      if (!response.ok) throw new Error(`Token page failed: HTTP ${response.status}`);
      assertQuoteNetwork(source);
      const data: { items: WalletApiToken[]; next_page_params: Record<string, unknown> | null } =
        await response.json();
      if (
        !Array.isArray(data.items) ||
        data.items.length > 50 ||
        !(data.next_page_params === null || typeof data.next_page_params === 'object')
      ) {
        throw new Error('Invalid token page');
      }
      pages++;
      scanned += data.items.length;
      for (const item of data.items) {
        if (item.type !== 'ERC-20' || !/^0x[0-9a-fA-F]{40}$/.test(item.address_hash)) continue;
        const address = item.address_hash.toLowerCase();
        if (seen.has(address)) continue;
        seen.add(address);
        const decimals = item.decimals === null ? NaN : Number(item.decimals);
        if (!Number.isInteger(decimals) || decimals < 0 || decimals > 255) {
          throw new Error('Token decimals are unavailable');
        }
        const token: SwapToken = {
          address,
          symbol: item.symbol || address.slice(0, 6),
          name: item.name || address.slice(0, 8),
          decimals,
        };
        const capability = await resolveTokenCapability(token);
        assertQuoteNetwork(source);
        if (capability.state === 'launchpad') tokens.push({ ...token, isLaunchpad: true });
        else if (capability.state === 'unsupported') unsupported.push(token);
      }
      continuation =
        data.next_page_params === null
          ? null
          : Object.fromEntries(
              Object.entries(data.next_page_params).map(([key, value]) => [key, String(value)])
            );
      if (continuation === null) {
        complete = true;
        break;
      }
    }
  } catch (cause) {
    assertQuoteNetwork(source);
    error = cause instanceof Error ? cause.message : String(cause);
  }
  assertQuoteNetwork(source);
  const enriched = error ? tokens : await enrichWithSidioraMetadata(tokens, source);
  assertQuoteNetwork(source);
  const discovery: TokenDiscovery = {
    tokens: error && previous ? previous.tokens : enriched,
    unsupported: error && previous ? previous.unsupported : unsupported,
    complete: !error && complete,
    status: error ? 'failed' : complete ? 'complete' : 'truncated',
    continuation,
    pages,
    scanned,
    updatedAt: error && previous ? previous.updatedAt : Date.now(),
    ...(error ? { error } : {}),
  };
  // A continuation segment is not a complete inventory, even if its final page is terminal.
  if (!error && complete && !options.continuation) writeSwapRecord(storage, listKey(), discovery);
  return Object.assign([...discovery.tokens], { discovery });
}

export function getCachedLaunchpadTokens(storage?: SwapStorage): {
  tokens: SwapToken[];
  updatedAt: number;
  discovery: TokenDiscovery;
} | null {
  if (!storage && typeof window === 'undefined') return null;
  const discovery = readInventory(storage ?? swapStorage());
  if (!discovery || Date.now() - discovery.updatedAt > LIST_TTL) return null;
  return { tokens: discovery.tokens, updatedAt: discovery.updatedAt, discovery };
}

import { createRequire } from 'node:module';
import { randomUUID } from 'node:crypto';
import { createElement, act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import { NextRequest } from 'next/server';
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { HttpError, Operations, SdkClient, decodeV2Contract, type V2ContractTypes } from '@kindle/sdk';
import { middleware } from '@/middleware';
import { backendConfiguration, BackendOriginError } from '@/core/backend-origins';
import { dataApiUrl, gatewayUrl, metadataApiUrl, statsApiUrl } from '@/core/sdk-config';
import { BackendConfigurationError } from '@/ui/shared/backend-configuration-error';
import { serveRecorded, v2Reads, type Replay, type ReplayFallback } from '@/core/test-fixtures/market-production/v2-reads';
import { browserGatewayClient, browserGatewaySessionClient, browserMarketClient, browserSessionClient, browserStatsClient, browserUserClient, MarketClientContext } from './client';
import { fetchPoolDetailListings, fetchPoolListings, fetchPoolListingsWithStatus, usePoolDetailListings, usePoolListings, type PoolListingsQueryResult } from './pool-listings';

interface BrowserDom { window: Window & typeof globalThis & { close(): void } }
type JsdomModule = { JSDOM: new (html: string, options: { url: string }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000' });
const globals = globalThis as unknown as Record<string, unknown>;
const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
const previousDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
Object.defineProperty(globalThis, 'window', { configurable: true, value: dom.window });
Object.defineProperty(globalThis, 'document', { configurable: true, value: dom.window.document });
globals.IS_REACT_ACT_ENVIRONMENT = true;

const { real, realToken, example, stateless, unknown } = v2Reads.pools;
const recorded = v2Reads.responses;
const storageKey = 'sidiora.profile-absent.v1';
const wallet = (index: number) => `0x${index.toString(16).padStart(40, '0')}`;
const names = ['DATA_API_URL', 'GATEWAY_URL', 'METADATA_API_URL', 'PNL_API_URL', 'STATS_API_URL', 'STREAM_ORIGIN', 'PRIVATE_API_ORIGIN', 'PRIVATE_STREAM_ORIGIN', 'WALLET_IFRAME_ORIGIN'] as const;
const savedEnv = Object.fromEntries(names.map(name => [name, process.env[`NEXT_PUBLIC_${name}`]]));
let replay: Replay;
let override: ReplayFallback | undefined;
const clients: QueryClient[] = [];

function queryClient(): QueryClient {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  clients.push(client);
  return client;
}

function sdk(Client = SdkClient): SdkClient {
  return new Client({ apiOrigin: replay.origin, identityOrigin: replay.origin, mediaOrigin: replay.origin, streamOrigin: replay.origin.replace(/^http/, 'ws'), approvedStreamOrigins: [replay.origin.replace(/^http/, 'ws')], production: false });
}

beforeEach(async () => {
  override = undefined;
  replay = await serveRecorded(Object.values(recorded), undefined, (...args) => override?.(...args));
  for (const name of names) delete process.env[`NEXT_PUBLIC_${name}`];
  for (const name of ['DATA_API_URL', 'GATEWAY_URL', 'METADATA_API_URL', 'PNL_API_URL', 'STATS_API_URL']) process.env[`NEXT_PUBLIC_${name}`] = replay.origin;
  window.sessionStorage.clear();
  (await import('./profile-absence')).resetProfileAbsenceForSession();
});
afterEach(async () => {
  clients.splice(0).forEach(client => client.clear());
  await replay.close();
  for (const name of names) {
    const value = savedEnv[name];
    if (value === undefined) delete process.env[`NEXT_PUBLIC_${name}`];
    else process.env[`NEXT_PUBLIC_${name}`] = value;
  }
});
afterAll(() => {
  dom.window.close();
  if (previousWindow) Object.defineProperty(globalThis, 'window', previousWindow);
  else delete globals.window;
  if (previousDocument) Object.defineProperty(globalThis, 'document', previousDocument);
  else delete globals.document;
  delete globals.IS_REACT_ACT_ENVIRONMENT;
});

async function base_path_loss_rejects_prefixed_configuration_before_serving_consumers() {
  const builders = [dataApiUrl, gatewayUrl, metadataApiUrl, statsApiUrl];
  const factories = [browserMarketClient, browserUserClient, browserStatsClient, browserSessionClient, browserGatewayClient, browserGatewaySessionClient];
  for (const name of ['DATA_API_URL', 'GATEWAY_URL', 'METADATA_API_URL', 'STATS_API_URL']) {
    process.env[`NEXT_PUBLIC_${name}`] = `${replay.origin}/tenant/market/`;
    const configuration = backendConfiguration();
    expect(configuration.origins).toBeNull();
    expect(configuration.problems).toHaveLength(1);
    expect(configuration.problems[0]).toBeInstanceOf(BackendOriginError);
    expect(configuration.problems[0].variable).toBe(`NEXT_PUBLIC_${name}`);
    for (const build of builders) expect(() => build('/v2/pools')).toThrow('without a path prefix');
    for (const factory of factories) expect(factory).toThrow('without a path prefix');
    const response = middleware(new NextRequest('http://localhost:3000/explore'));
    expect(response.headers.get('Content-Security-Policy')).not.toContain(replay.origin);
    expect(renderToStaticMarkup(createElement(BackendConfigurationError, { problems: configuration.problems }))).toContain('role="alert"');
    process.env[`NEXT_PUBLIC_${name}`] = `${replay.origin}/`;
  }
  expect(replay.requests).toEqual([]);
  const listings = await fetchPoolListings(queryClient(), new Operations(browserMarketClient()), replay.origin, [real]);
  expect(listings[real].tokenAddress).toBe(realToken);
  expect(replay.requests).toEqual([`GET /v2/pools/${real}`]);
}
test(base_path_loss_rejects_prefixed_configuration_before_serving_consumers.name, base_path_loss_rejects_prefixed_configuration_before_serving_consumers);

function quarantinedDetail(): V2ContractTypes['PoolDetailResponse'] {
  const body = structuredClone(decodeV2Contract('PoolDetailResponse', recorded.poolDetailFirst.body));
  body.data.stats.watermark.integrity = 'quarantined';
  return body;
}
function mismatchedDetail(): V2ContractTypes['PoolDetailResponse'] {
  const body = structuredClone(decodeV2Contract('PoolDetailResponse', recorded.poolDetailFirst.body));
  body.data.token.address = realToken;
  return body;
}

async function assertFatalMixedDetail(body: V2ContractTypes['PoolDetailResponse'], message: string) {
  override = (_method, url) => url.pathname === `/v2/pools/${example}` ? { status: 200, body } : undefined;
  const operations = new Operations(sdk());
  await expect(fetchPoolListings(queryClient(), operations, replay.origin, [real, example])).rejects.toThrow(message);
  await expect(fetchPoolDetailListings(queryClient(), operations, replay.origin, [real, example])).rejects.toThrow(message);
  expect(replay.requests).toContain(`GET /v2/pools/${real}`);
  expect(replay.requests).toContain(`GET /v2/pools/${example}`);
}
async function partial_listing_failures_propagates_mixed_quarantine() {
  await assertFatalMixedDetail(quarantinedDetail(), 'Pool integrity quarantined');
}
test(partial_listing_failures_propagates_mixed_quarantine.name, partial_listing_failures_propagates_mixed_quarantine);
async function partial_listing_failures_propagates_mixed_identity_disagreement() {
  await assertFatalMixedDetail(mismatchedDetail(), 'Pool identity disagreement');
}
test(partial_listing_failures_propagates_mixed_identity_disagreement.name, partial_listing_failures_propagates_mixed_identity_disagreement);

async function partial_listing_failures_propagates_security_and_unexpected_http_errors() {
  const operations = new Operations(sdk());
  for (const status of [401, 403, 500, 503]) {
    override = (_method, url) => url.pathname === `/v2/pools/${example}` ? { status, body: { ...recorded.poolDetailNotFound.body as V2ContractTypes['Error'], code: status === 503 ? 'UNEXPECTED_FAILURE' : 'SCOPE_DENIED' } } : undefined;
    await expect(fetchPoolListings(queryClient(), operations, replay.origin, [real, example])).rejects.toSatisfy((error: unknown) => error instanceof HttpError && error.status === status);
    await expect(fetchPoolDetailListings(queryClient(), operations, replay.origin, [real, example])).rejects.toSatisfy((error: unknown) => error instanceof HttpError && error.status === status);
  }
}
test(partial_listing_failures_propagates_security_and_unexpected_http_errors.name, partial_listing_failures_propagates_security_and_unexpected_http_errors);

async function partial_listing_failures_rejects_quarantined_list_before_cache_seeding() {
  const body = structuredClone(decodeV2Contract('PoolListResponse', recorded.poolListNew.body));
  body.quality = 'quarantined';
  override = (_method, url) => url.pathname === '/v2/pools' ? { status: 200, body } : undefined;
  const qc = queryClient();
  await expect(fetchPoolListings(qc, new Operations(sdk()), replay.origin, [real, example], undefined, 'new')).rejects.toThrow('Pool list integrity quarantined');
  expect(replay.requests).toEqual(['GET /v2/pools?sort=new&limit=100']);
  expect(qc.getQueryCache().getAll()).toHaveLength(0);
}
test(partial_listing_failures_rejects_quarantined_list_before_cache_seeding.name, partial_listing_failures_rejects_quarantined_list_before_cache_seeding);

async function partial_listing_failures_retains_declared_omission_status() {
  // The committed unavailable transcript is retried by the real SDK transport.
  const operations = new Operations(sdk());
  const result = await fetchPoolListingsWithStatus(queryClient(), operations, replay.origin, [real, unknown, stateless]);
  expect(Object.keys(result.listings)).toEqual([real]);
  expect(result.omissions).toEqual({ [unknown]: { state: 'not_found', status: 404 }, [stateless]: { state: 'unavailable', status: 503 } });
  await expect(fetchPoolListings(queryClient(), operations, replay.origin, [unknown])).rejects.toSatisfy((error: unknown) => error instanceof HttpError && error.status === 404);
  expect(await fetchPoolListingsWithStatus(queryClient(), operations, replay.origin, [])).toEqual({ listings: {}, omissions: {} });
}
test(partial_listing_failures_retains_declared_omission_status.name, partial_listing_failures_retains_declared_omission_status);

async function waitFor(check: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 200 && !check(); attempt++) await act(() => new Promise(resolve => setTimeout(resolve, 10)));
  expect(check()).toBe(true);
}
async function partial_listing_failures_hooks_expose_omissions_and_integrity_errors() {
  for (const detail of [false, true]) {
    for (const failure of ['unserved', 'quarantine', 'identity'] as const) {
      const host = document.createElement('div');
      document.body.append(host);
      const root: Root = createRoot(host);
      let latest: PoolListingsQueryResult | undefined;
      const poolSet = [real, failure === 'unserved' ? unknown : example];
      const useListings = detail ? usePoolDetailListings : usePoolListings;
      override = (_method, url) => url.pathname === `/v2/pools/${example}` ? { status: 200, body: failure === 'quarantine' ? quarantinedDetail() : mismatchedDetail() } : undefined;
      function Probe() {
        const result = useListings(poolSet);
        latest = result;
        return createElement('span', null, result.status);
      }
      try {
        await act(() => root.render(createElement(QueryClientProvider, { client: queryClient() }, createElement(MarketClientContext.Provider, { value: sdk() }, createElement(Probe)))));
        if (failure === 'unserved') {
          await waitFor(() => latest?.isSuccess === true);
          expect(latest?.data?.[real].tokenAddress).toBe(realToken);
          expect(latest?.omissions).toEqual({ [unknown]: { state: 'not_found', status: 404 } });
        } else {
          await waitFor(() => latest?.isError === true);
          expect(latest?.error?.message).toBe(failure === 'quarantine' ? 'Pool integrity quarantined' : 'Pool identity disagreement');
          expect(latest?.data).toBeUndefined();
        }
      } finally {
        await act(() => root.unmount());
        host.remove();
      }
    }
  }
}
test(partial_listing_failures_hooks_expose_omissions_and_integrity_errors.name, partial_listing_failures_hooks_expose_omissions_and_integrity_errors);

async function reopenedAbsence() {
  vi.resetModules();
  return import('./profile-absence');
}
async function profile_absence_growth_evicts_lru_and_recovers_after_reload() {
  const cache = await reopenedAbsence();
  cache.rememberProfileAbsent(wallet(1));
  cache.rememberProfileAbsent(wallet(2));
  for (let index = 3; index <= cache.profileAbsenceCapacity; index++) cache.rememberProfileAbsent(wallet(index));
  expect(cache.isProfileAbsent(wallet(1).toUpperCase())).toBe(true);
  cache.rememberProfileAbsent(wallet(cache.profileAbsenceCapacity + 1));
  expect(cache.isProfileAbsent(wallet(2))).toBe(false);
  expect(cache.isProfileAbsent(wallet(1))).toBe(true);
  expect(window.sessionStorage.getItem(storageKey)!.length).toBeLessThanOrEqual(cache.profileAbsenceStorageLimit);
  const restarted = await reopenedAbsence();
  expect(restarted.isProfileAbsent(wallet(1))).toBe(true);
  expect(restarted.isProfileAbsent(wallet(2))).toBe(false);
  expect(restarted.isProfileAbsent(wallet(cache.profileAbsenceCapacity + 1))).toBe(true);
  const persisted = JSON.parse(window.sessionStorage.getItem(storageKey)!) as { entries: unknown[] };
  expect(persisted.entries).toHaveLength(cache.profileAbsenceCapacity);
}
test(profile_absence_growth_evicts_lru_and_recovers_after_reload.name, profile_absence_growth_evicts_lru_and_recovers_after_reload);

async function profile_absence_growth_expires_entries_without_sliding_ttl() {
  const cache = await reopenedAbsence();
  const savedAt = Date.now() - cache.profileAbsenceTtlMs + 250;
  window.sessionStorage.setItem(storageKey, JSON.stringify({ version: 2, entries: [[wallet(1), savedAt], [wallet(2), Date.now() - cache.profileAbsenceTtlMs], [wallet(3), Date.now() + 60_000]] }));
  expect(cache.isProfileAbsent(wallet(1))).toBe(true);
  expect(cache.isProfileAbsent(wallet(2))).toBe(false);
  expect(cache.isProfileAbsent(wallet(3))).toBe(false);
  await new Promise(resolve => setTimeout(resolve, 300));
  expect(cache.isProfileAbsent(wallet(1))).toBe(false);
  expect((await reopenedAbsence()).isProfileAbsent(wallet(1))).toBe(false);
}
test(profile_absence_growth_expires_entries_without_sliding_ttl.name, profile_absence_growth_expires_entries_without_sliding_ttl);

async function profile_absence_growth_caps_persisted_input_before_hydration() {
  let cache = await reopenedAbsence();
  const oversized = JSON.stringify(Array.from({ length: 2000 }, (_, index) => wallet(index + 1)));
  expect(oversized.length).toBeGreaterThan(cache.profileAbsenceStorageLimit);
  window.sessionStorage.setItem(storageKey, oversized);
  expect(cache.isProfileAbsent(wallet(1))).toBe(false);
  expect(window.sessionStorage.getItem(storageKey)!.length).toBeLessThanOrEqual(cache.profileAbsenceStorageLimit);
  cache = await reopenedAbsence();
  window.sessionStorage.setItem(storageKey, JSON.stringify({ version: 2, entries: Array.from({ length: cache.profileAbsenceCapacity + 1 }, (_, index) => [wallet(index + 1), Date.now()]) }));
  expect(cache.isProfileAbsent(wallet(1))).toBe(false);
  cache.rememberProfileAbsent('not-a-wallet');
  expect(cache.isProfileAbsent('not-a-wallet')).toBe(false);
}
test(profile_absence_growth_caps_persisted_input_before_hydration.name, profile_absence_growth_caps_persisted_input_before_hydration);

async function profile_absence_growth_preserves_absence_and_profile_save_invalidation() {
  const cache = await reopenedAbsence();
  const { fetchProfile } = await import('./use-profile');
  const { SdkClient: ProfileSdkClient } = await import('@kindle/sdk');
  const profiles = new Map<string, V2ContractTypes['ConnectedExtendedProfileResponse']>();
  override = (_method, url) => {
    if (url.pathname !== `/v3/profiles/${real}`) return undefined;
    const profile = profiles.get(real);
    return profile ? { status: 200, body: profile } : { status: 404, body: recorded.poolDetailNotFound.body };
  };
  const client = sdk(ProfileSdkClient);
  expect(await fetchProfile(real, client)).toBeNull();
  expect(await fetchProfile(real, client)).toBeNull();
  expect(replay.requests).toEqual([`GET /v3/profiles/${real}`]);
  profiles.set(real, { data: { profile: { wallet: real, displayName: 'Published', bio: '', avatarAssetId: null, bannerAssetId: null, website: null, socials: {}, creatorPools: null, revision: '1' } }, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: randomUUID(), revision: '1', watermark: null });
  cache.forgetProfileAbsence(real.toUpperCase());
  expect(await fetchProfile(real, client)).toMatchObject({ wallet: real, displayName: 'Published', source: 'v3' });
  expect(replay.requests).toEqual([`GET /v3/profiles/${real}`, `GET /v3/profiles/${real}`]);
  expect(cache.isProfileAbsent(real)).toBe(false);
  expect((await reopenedAbsence()).isProfileAbsent(real)).toBe(false);
}
test(profile_absence_growth_preserves_absence_and_profile_save_invalidation.name, profile_absence_growth_preserves_absence_and_profile_save_invalidation);

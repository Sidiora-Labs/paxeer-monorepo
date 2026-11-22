// @vitest-environment jsdom
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { EventEmitter } from 'node:events';
import { act, createElement, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Operations, SdkClient } from '@kindle/sdk';
import { createConfig, http, WagmiProvider, type Config } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import { injected } from 'wagmi/connectors';
import { BaseError, UserRejectedRequestError, createWalletClient, type EIP1193Provider } from 'viem';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { MarketClientContext } from '@/core/market/client';
import { queryKeys } from '@/core/query-keys';
import { paxeerWalletChain } from '@/core/wallet-sdk/client';
import { fetchStatsBatch } from '@/core/market/use-stats-batch';
import { fetchHolderRewardsPools, fetchHolderRewardsStats, type HolderRewardsPoolsResponse } from '@/widgets/holder-rewards/holder-rewards-contract';
import { buildRows, defaultOrder, filterRows } from '@/widgets/holder-rewards/holder-rewards-presentation';
import HolderRewardsTable from '@/widgets/holder-rewards/holder-rewards-table';
import HolderRewardsView from '@/widgets/holder-rewards/holder-rewards-view';
import { useHolderRewardsPools } from '@/widgets/holder-rewards/use-holder-rewards';
import { TradersBoard } from '@/widgets/leaderboard/traders-board';
import { CalloutsRail } from '@/widgets/social/callouts-rail';
import RewardsView from '@/widgets/rewards/rewards-view';
import { RuntimeAuthorityProvider } from '@/core/runtime-authority';
import { accountEpoch, clearAccountSession, installAccountSession } from '@/widgets/profile/account-transport';
import { isRewardClaimRejected } from '@/widgets/rewards/reward-claim-state';
import { releaseRejectedRewardClaim } from '@/widgets/rewards/reward-claim-recovery';
import ReferralRedirect from '@/app/r/[shortCode]/page';

interface JoinedConfiguration {
  api_origin: string;
  rpc_url: string;
  processes: { role: string; env?: { DATABASE_URL: string; REDIS_URL: string } }[];
}
interface RewardsServices { origin: string; unavailable: string; pool: string; discovery: string; discoveryPending: string }
let services: RewardsServices;
let server: ChildProcess;
let serverExit: Promise<number | null>;
let directory: string;
let output = '';
let client: SdkClient;
let operations: Operations;
let joined: JoinedConfiguration;
const views: { root: Root; container: HTMLDivElement; queries: QueryClient }[] = [];
const previousMetadata = process.env.NEXT_PUBLIC_METADATA_API_URL;
const previousGateway = process.env.NEXT_PUBLIC_GATEWAY_URL;
const previousPnl = process.env.NEXT_PUBLIC_PNL_API_URL;

function sdk(origin: string) {
  const streamOrigin = origin.replace('http:', 'ws:');
  return new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
}

beforeAll(async () => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  const path = process.env.JOINED_CONFIG_FILE;
  if (!path) throw new Error('Joined service configuration required');
  joined = JSON.parse(readFileSync(path, 'utf8'));
  const environment = joined.processes.find(process => process.role === 'api')?.env;
  if (!environment) throw new Error('Joined database configuration required');
  const redis = new URL(environment.REDIS_URL);
  redis.pathname = '/14';
  directory = mkdtempSync(resolve(tmpdir(), 'rewards-services-'));
  const moduleRoot = resolve(__dirname, '../../../../../../services/launchpad/api');
  const overlay = resolve(directory, 'overlay.json');
  writeFileSync(overlay, JSON.stringify({ Replace: {
    [resolve(moduleRoot, 'internal/rest/rewards_web_test.go')]: resolve(__dirname, 'rewards-services_test.go.txt'),
  } }));
  server = spawn('go', ['test', `-overlay=${overlay}`, '-count=1', '-v', '-run=^TestRewardsWebServices$', './internal/rest'], {
    cwd: moduleRoot, stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, GOWORK: 'off', API_TEST_DATABASE_URL: environment.DATABASE_URL, CORE_API_TEST_POSTGRES_URL: environment.DATABASE_URL, CORE_API_TEST_REDIS_URL: redis.toString() },
  });
  serverExit = new Promise((done, reject) => { server.once('error', reject); server.once('exit', done); });
  server.stderr!.on('data', chunk => { output += String(chunk); });
  const lines = createInterface({ input: server.stdout! });
  services = await new Promise<RewardsServices>((done, reject) => {
    lines.on('line', line => {
      output += line + '\n';
      if (line.startsWith('REWARDS_SERVICES ')) done(JSON.parse(line.slice('REWARDS_SERVICES '.length)));
    });
    void serverExit.then(code => reject(new Error(`Rewards services exited ${code}: ${output}`)), reject);
  });
  client = sdk(services.origin);
  operations = new Operations(client);
  process.env.NEXT_PUBLIC_METADATA_API_URL = joined.api_origin;
  process.env.NEXT_PUBLIC_GATEWAY_URL = joined.api_origin;
  process.env.NEXT_PUBLIC_PNL_API_URL = joined.api_origin;
}, 600_000);

afterEach(async () => {
  for (const view of views.splice(0)) {
    await act(async () => view.root.unmount());
    await view.queries.cancelQueries();
    view.queries.clear();
    view.container.remove();
  }
  window.localStorage.clear();
  clearAccountSession(false);
});

afterAll(async () => {
  if (previousMetadata === undefined) delete process.env.NEXT_PUBLIC_METADATA_API_URL;
  else process.env.NEXT_PUBLIC_METADATA_API_URL = previousMetadata;
  if (previousGateway === undefined) delete process.env.NEXT_PUBLIC_GATEWAY_URL;
  else process.env.NEXT_PUBLIC_GATEWAY_URL = previousGateway;
  if (previousPnl === undefined) delete process.env.NEXT_PUBLIC_PNL_API_URL;
  else process.env.NEXT_PUBLIC_PNL_API_URL = previousPnl;
  try {
    if (services) {
      expect((await fetch(services.origin + '/stop', { method: 'POST' })).status).toBe(204);
      expect(await serverExit, output).toBe(0);
    } else server?.kill('SIGTERM');
  } finally {
    if (directory) rmSync(directory, { recursive: true });
  }
});

function mount(node: ReactNode, queries = new QueryClient(), connectedConfig?: Config) {
  const config = connectedConfig ?? createConfig({ chains: [paxeerWalletChain], transports: { 125: http() }, storage: null, multiInjectedProviderDiscovery: false });
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  const render = (content: ReactNode) => act(() => root.render(createElement(WagmiProvider, { config, reconnectOnMount: false }, createElement(QueryClientProvider, { client: queries }, createElement(MarketClientContext.Provider, { value: client }, content)))));
  render(node);
  views.push({ root, container, queries });
  return { container, queries, render };
}

async function until(check: () => boolean) {
  const deadline = Date.now() + 15_000;
  while (!check() && Date.now() < deadline) await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  expect(check(), document.body.textContent ?? '').toBe(true);
}

function button(container: ParentNode, label: string) {
  const found = [...container.querySelectorAll('button')].find(node => node.textContent === label);
  expect(found, label).toBeDefined();
  return found!;
}

async function rewards_filtered_rows_keep_the_next_page_reachable() {
  const page = await fetchHolderRewardsPools(operations, { sort: 'rewards', cursor: null, limit: 1 });
  expect(page.nextCursor).not.toBeNull();
  expect(page.data.items[0].strategyActive).toBe(false);
  const rows = filterRows(buildRows(page.data.items, new Map(), undefined), { activeOnly: true, pricedOnly: false });
  expect(rows).toHaveLength(0);
  const view = mount(null);
  let nextPageLoaded = false;
  const load = async () => {
    const next = await fetchHolderRewardsPools(operations, { sort: 'rewards', cursor: page.nextCursor, limit: 1 });
    const shown = filterRows(buildRows(next.data.items, new Map(), undefined), { activeOnly: true, pricedOnly: false });
    expect(shown).toHaveLength(1);
    view.render(createElement(HolderRewardsTable, { rows: shown, order: defaultOrder, onOrderChange: () => {}, hasMore: next.nextCursor !== null, onLoadMore: () => void load() }));
    nextPageLoaded = true;
  };
  view.render(createElement(HolderRewardsTable, { rows, order: defaultOrder, onOrderChange: () => {}, filtered: true, hasMore: true, onLoadMore: () => void load() }));
  expect(view.container.textContent).toContain('No coins match these filters');
  act(() => button(view.container, 'Load more').click());
  await until(() => nextPageLoaded);
  expect(view.container.querySelectorAll('tbody tr')).toHaveLength(1);
  expect(view.container.querySelector('tbody a')?.getAttribute('href')).toContain(services.pool);
}

async function rewards_top_rail_failure_has_a_retry_and_recovers() {
  const queries = new QueryClient();
  const queryKey = queryKeys.holderRewardsPools('rewards', 5);
  queries.setQueryDefaults(queryKey, { enabled: false });
  await expect(queries.fetchInfiniteQuery({
    queryKey, initialPageParam: null as string | null,
    queryFn: () => fetchHolderRewardsPools(new Operations(sdk(services.unavailable)), { sort: 'rewards', cursor: null, limit: 5 }),
    getNextPageParam: (page: HolderRewardsPoolsResponse) => page.nextCursor,
  })).rejects.toMatchObject({ status: 503 });
  const view = mount(createElement(HolderRewardsView), queries);
  await until(() => view.container.querySelector('tbody a') !== null);
  const notice = [...view.container.querySelectorAll('[role="status"]')].find(node => node.textContent?.includes('Try again'));
  expect(notice).toBeDefined();
  expect([...view.container.querySelectorAll('h3')].filter(node => node.textContent === 'No holder rewards paid yet')).toHaveLength(0);
  act(() => button(notice!, 'Try again').click());
  await until(() => view.container.querySelector('#top-paying-title') !== null);
  await until(() => view.container.querySelector('[aria-labelledby="top-paying-title"] ol') !== null);
  expect(view.container.querySelector('[aria-labelledby="top-paying-title"] ol')).not.toBeNull();
}

async function rewards_market_cap_failure_ends_the_loading_state() {
  const view = mount(createElement(HolderRewardsView));
  await until(() => view.container.querySelector('tbody a') !== null);
  await act(async () => { await Promise.allSettled(view.queries.getQueryCache().getAll().map(query => query.promise)); });
  await until(() => view.container.querySelector('tbody a') !== null && view.queries.isFetching() === 0);
  await until(() => view.container.querySelectorAll('[aria-label="Loading market cap"]').length === 0);
  expect(view.container.querySelectorAll('[aria-label="Loading market cap"]')).toHaveLength(0);
  expect(view.container.querySelectorAll('[aria-label="Market cap not available yet"]')).toHaveLength(2);
  const page = await fetchHolderRewardsPools(operations, { sort: 'rewards', cursor: null, limit: 50 });
  await expect(fetchStatsBatch(operations, page.data.items.map(row => row.poolAddress))).rejects.toMatchObject({ status: 503 });
  const rows = buildRows(page.data.items, new Map(), undefined);
  const table = mount(createElement(HolderRewardsTable, { rows, order: defaultOrder, onOrderChange: () => {}, hasMore: false, onLoadMore: () => {}, pricesPending: true }));
  expect(table.container.querySelectorAll('[aria-label="Loading market cap"]')).toHaveLength(2);
  table.render(createElement(HolderRewardsTable, { rows, order: defaultOrder, onOrderChange: () => {}, hasMore: false, onLoadMore: () => {}, pricesPending: false }));
  expect(table.container.querySelectorAll('[aria-label="Loading market cap"]')).toHaveLength(0);
  expect(table.container.querySelectorAll('[aria-label="Market cap not available yet"]')).toHaveLength(2);
  const totals = await fetchHolderRewardsStats(operations);
  expect(totals.data.cumulativeDistributedRaw).toBe('6000000');
  expect(view.container.textContent).toContain('6');
}

async function rewards_trader_pages_follow_the_service_cursor_and_restart() {
  const first = await operations.call('IF_LEADERBOARDS', { path: { category: 'trader' }, query: { window: '24h', limit: 100 } });
  expect(first.data.snapshot.entries).toHaveLength(100);
  expect(first.nextCursor).not.toBeNull();
  const view = mount(createElement(TradersBoard, { period: '1d' }));
  await until(() => view.container.querySelectorAll('[data-rank]').length === 100);
  act(() => button(view.container, 'Next page').click());
  await until(() => view.container.querySelector('[data-rank="101"]') !== null);
  expect(view.container.querySelectorAll('[data-rank]')).toHaveLength(1);
  expect(view.container.querySelector('[data-rank="101"] a')?.getAttribute('href')).toMatch(/^\/profile\/0x/);
  act(() => button(view.container, 'First page').click());
  await until(() => view.container.querySelector('[data-rank="1"]') !== null);
  expect(view.container.querySelectorAll('[data-rank]')).toHaveLength(100);
  view.render(createElement(TradersBoard, { period: '1w' }));
  await until(() => view.container.querySelector('[data-rank="1"]') !== null && view.container.textContent?.includes('Last 7 days') === true);
  expect([...view.container.querySelectorAll('button')].filter(node => node.textContent === 'First page')).toHaveLength(0);
}

async function rewards_holder_page_retry_restarts_an_expired_snapshot() {
  let pools: ReturnType<typeof useHolderRewardsPools>;
  function PoolPages() {
    pools = useHolderRewardsPools('rewards', 1);
    return createElement('button', { onClick: pools.loadMore }, pools.loadMoreFailed ? 'Try again' : 'Load more');
  }
  const view = mount(createElement(PoolPages));
  await until(() => pools?.items.length === 1);
  expect(pools!.hasMore).toBe(true);
  expect((await fetch(services.origin + '/advance-head', { method: 'POST' })).status).toBe(204);
  act(() => button(view.container, 'Load more').click());
  await until(() => pools!.loadMoreFailed);
  expect(pools!.items).toHaveLength(1);
  act(() => button(view.container, 'Try again').click());
  await until(() => pools!.pages[0]?.watermark?.height === '901' && !pools!.loadingMore && !pools!.loadMoreFailed);
  expect(pools!.items).toHaveLength(1);
  act(() => button(view.container, 'Load more').click());
  await until(() => pools!.items.length === 2);
  expect(pools!.hasMore).toBe(false);
}

async function rewards_callouts_follow_the_discovery_cursor(poolReads: 'pending' | 'settled') {
  const discoveryClient = sdk(poolReads === 'pending' ? services.discoveryPending : services.discovery);
  const discovery = new Operations(discoveryClient);
  const page = await discovery.call('IF_CONNECTED_FEED', { query: { order: 'relevance', limit: 50 } });
  expect(page.Entries).toHaveLength(50);
  expect(page.Cursor).not.toBe('');
  const next = await discovery.call('IF_CONNECTED_FEED', { query: { order: 'relevance', limit: 50, cursor: page.Cursor } });
  expect(next.Entries).toHaveLength(1);
  const first = page.Entries[0];
  const last = next.Entries[0];
  expect(first.Body).toBe('Callout 1');
  expect(last.Body).toBe('Callout 51');
  expect(first.Source.Pools).not.toBeNull();
  const pool = first.Source.Pools![0];
  expect(pool).toMatch(/^0x[0-9a-f]{40}$/);
  const view = mount(createElement(MarketClientContext.Provider, { value: discoveryClient }, createElement(CalloutsRail)));

  async function assertPoolReadState() {
    const key = queryKeys.poolSnapshot(pool);
    if (poolReads === 'settled') {
      await until(() => {
        const state = view.queries.getQueryState(key);
        return state !== undefined && state.status !== 'pending' && state.fetchStatus === 'idle';
      });
      expect(view.queries.getQueryState(key)?.status).not.toBe('pending');
      expect(view.queries.getQueryState(key)?.fetchStatus).toBe('idle');
    } else {
      expect(view.queries.getQueryState(key)?.status).toBe('pending');
      expect(view.queries.getQueryState(key)?.fetchStatus).toBe('fetching');
    }
  }

  await until(() => view.container.querySelectorAll('[data-callout-id]').length === 50);
  if (poolReads === 'pending') expect((await fetch(services.origin + '/pool-read-started')).status).toBe(204);
  await assertPoolReadState();
  expect(view.container.querySelector('[data-callout-id]')?.getAttribute('data-callout-id')).toBe(first.Source.ID);
  expect(view.container.querySelector('[data-callout-id] > div > p')?.textContent).toBe(first.Body);
  act(() => button(view.container, 'Next page').click());
  await until(() => view.container.querySelectorAll('[data-callout-id]').length === 1);
  await assertPoolReadState();
  expect(view.container.querySelectorAll('[data-callout-id]')).toHaveLength(1);
  expect(view.container.querySelector('[data-callout-id]')?.getAttribute('data-callout-id')).toBe(last.Source.ID);
  expect(view.container.querySelector('[data-callout-id] > div > p')?.textContent).toBe(last.Body);
  act(() => button(view.container, 'First page').click());
  await until(() => view.container.querySelectorAll('[data-callout-id]').length === 50);
  await assertPoolReadState();
  expect(view.container.querySelector(`[data-callout-id="${last.Source.ID}"]`)).toBeNull();
  expect(view.container.querySelectorAll('[data-callout-id]')).toHaveLength(50);
  expect(view.container.querySelector('[data-callout-id]')?.getAttribute('data-callout-id')).toBe(first.Source.ID);
  expect(view.container.querySelector('[data-callout-id] > div > p')?.textContent).toBe(first.Body);
}

async function rewards_unavailable_claims_explain_the_state_and_keep_read_retry() {
  const rpc = createWalletClient({ chain: paxeerWalletChain, transport: http(joined.rpc_url) });
  const provider = Object.assign(new EventEmitter(), { request: rpc.request }) as EIP1193Provider;
  const config = createConfig({ chains: [paxeerWalletChain], connectors: [injected({ target: { id: 'paxeer', name: 'Paxeer', provider: () => provider } })], transports: { 125: http(joined.rpc_url) }, storage: null, multiInjectedProviderDiscovery: false });
  await connect(config, { connector: config.connectors[0]! });
  try {
    const [wallet] = await rpc.getAddresses();
    if (!wallet) throw new Error('Connected wallet required');
    installAccountSession({ wallet, sessionId: crypto.randomUUID(), deviceLabel: 'Web browser', accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(), refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), revokedAt: null, scopes: ['account'] }, crypto.randomUUID(), accountEpoch());
    const view = mount(createElement(RuntimeAuthorityProvider, null, createElement(RewardsView)), new QueryClient(), config);
    await until(() => view.container.textContent?.includes('Open the secure app to connect your wallet') === true);
    await until(() => view.container.textContent?.includes("Rewards can't load right now") === true);
    expect(view.container.querySelector('[role="status"]')?.textContent).toBe('Open the secure app to connect your wallet');
    expect(button(view.container, 'Try again').disabled).toBe(false);
    expect(view.container.querySelector('[data-claim-state]')).toBeNull();
  } finally {
    await disconnect(config);
  }
}

function rewards_wallet_rejection_releases_only_the_matching_reservation() {
  const storage = window.localStorage;
  const key = 'reward-claim';
  const reservation = JSON.stringify({ attemptId: crypto.randomUUID() });
  const rejection = new BaseError('Wallet request failed', { cause: new UserRejectedRequestError(new Error('Cancelled')) });
  expect(isRewardClaimRejected(rejection)).toBe(true);
  storage.setItem(key, reservation);
  releaseRejectedRewardClaim(storage, key, reservation, rejection);
  expect(storage.getItem(key)).toBeNull();
  storage.setItem(key, reservation);
  releaseRejectedRewardClaim(storage, key, reservation, new Error('Connection closed'));
  expect(storage.getItem(key)).toBe(reservation);
  const replacement = JSON.stringify({ attemptId: crypto.randomUUID() });
  storage.setItem(key, replacement);
  releaseRejectedRewardClaim(storage, key, reservation, rejection);
  expect(storage.getItem(key)).toBe(replacement);
  expect(isRewardClaimRejected(new Error('User rejected'))).toBe(false);
}

async function rewards_short_links_reach_home_and_shared_cards() {
  await expect(ReferralRedirect({ params: Promise.resolve({ shortCode: 'invite-code' }), searchParams: Promise.resolve({}) })).rejects.toMatchObject({ digest: 'NEXT_REDIRECT;replace;/?ref=invite-code;307;' });
  await expect(ReferralRedirect({ params: Promise.resolve({ shortCode: 'invite-code' }), searchParams: Promise.resolve({ cardId: 'shared-card' }) })).rejects.toMatchObject({ digest: 'NEXT_REDIRECT;replace;/pnl/shared-card?ref=invite-code;307;' });
  await expect(ReferralRedirect({ params: Promise.resolve({ shortCode: 'invite-code' }), searchParams: Promise.resolve({ cardId: ['a', 'b'] }) })).rejects.toMatchObject({ digest: 'NEXT_HTTP_ERROR_FALLBACK;404' });
}

test(rewards_filtered_rows_keep_the_next_page_reachable.name, rewards_filtered_rows_keep_the_next_page_reachable);
test(rewards_top_rail_failure_has_a_retry_and_recovers.name, rewards_top_rail_failure_has_a_retry_and_recovers, 30_000);
test(rewards_market_cap_failure_ends_the_loading_state.name, rewards_market_cap_failure_ends_the_loading_state, 30_000);
test(rewards_trader_pages_follow_the_service_cursor_and_restart.name, rewards_trader_pages_follow_the_service_cursor_and_restart, 30_000);
test(rewards_holder_page_retry_restarts_an_expired_snapshot.name, rewards_holder_page_retry_restarts_an_expired_snapshot, 30_000);
test.each(['pending', 'settled'] as const)(rewards_callouts_follow_the_discovery_cursor.name + ' (%s pool reads)', rewards_callouts_follow_the_discovery_cursor, 30_000);
test(rewards_unavailable_claims_explain_the_state_and_keep_read_retry.name, rewards_unavailable_claims_explain_the_state_and_keep_read_retry, 30_000);
test(rewards_wallet_rejection_releases_only_the_matching_reservation.name, rewards_wallet_rejection_releases_only_the_matching_reservation);
test(rewards_short_links_reach_home_and_shared_cards.name, rewards_short_links_reach_home_and_shared_cards);

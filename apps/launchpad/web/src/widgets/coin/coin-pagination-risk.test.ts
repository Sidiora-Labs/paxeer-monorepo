import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import React, { act, createElement } from 'react';
import type { Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SdkClient, decodeV2Contract, type V2ContractTypes } from '@kindle/sdk';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';
import { serveRecorded, v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { queryKeys } from '@/core/query-keys';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.Event = dom.window.Event;
browserGlobals.CustomEvent = dom.window.CustomEvent;
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const pool = v2Reads.pools.real;
const token = v2Reads.pools.realToken;
const wire = JSON.parse(readFileSync(new URL('../../core/test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8')) as {
  tradesFirstPage: { body: unknown };
  trades: { body: unknown };
};
const firstTrades = decodeV2Contract('TradesResponse', wire.tradesFirstPage.body);
const allTrades = decodeV2Contract('TradesResponse', wire.trades.body);
const detail = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body);
const tokenUnit = 10n ** BigInt(detail.data.token.decimals!);
const holder = allTrades.data.items[0]!.sender;
const oldHolder = '0x2222222222222222222222222222222222222222';
const newHolder = '0x3333333333333333333333333333333333333333';
const oldCursor = 'holders-snapshot-2-page-2';
const newCursor = 'holders-snapshot-3-page-2';
const tradeCursor = firstTrades.nextCursor!;

// TransferHolders' envelope and HolderEntry shape (stats-workers/internal/httpapi/pool_holders.go).
// The loopback store keeps immutable cursor snapshots; each response goes through the SDK validator.
function holderPage(wallet: string, balance: bigint, revision: string, nextCursor: string | null): V2ContractTypes['HoldersResponse'] {
  return decodeV2Contract('HoldersResponse', {
    ...allTrades,
    revision,
    nextCursor,
    watermark: { ...allTrades.watermark, projectionRevision: revision },
    data: {
      coverage: 'partial',
      excludedProtocolAddresses: [],
      exclusionPolicyVersion: 'inclusive_indexed_supply_v1',
      fromPosition: allTrades.data.items[1]!.position,
      toPosition: allTrades.data.items[0]!.position,
      items: [{ wallet, tokenAddress: token, balanceRaw: (balance * tokenUnit).toString(), share: null }],
    },
  });
}

// riskBreakdown's producer shape (stats-workers/internal/httpapi/pool_analytics.go).
const riskResponse = {
  coverage: 'legacy_incomplete', ownershipSource: 'swap_sender_legacy', poolAddress: pool,
  riskRating: 20, riskLevel: 'low', riskFactors: [],
  details: {
    holderCount: 20, top10ConcentrationPct: 25, creatorHoldingsPct: 5, creatorAddress: holder,
    hasCreatorSold: false, isNew: false, ageSeconds: 86400,
  },
};

let firstHolders = holderPage(holder, 1n, '2', oldCursor);
let continuationHolders = new Map<string, V2ContractTypes['HoldersResponse']>();
let failedHolderCursor: string | null = null;
let holderFailures = 0;
let tradeFailures = 0;
let servedTradeContinuation = allTrades;
let riskDetails: Record<string, unknown> = { ...riskResponse.details };
const failure = { code: 'INTERNAL_ERROR', message: 'Read failed', requestId: allTrades.requestId, retryable: false, retryAfterMs: null, fieldErrors: [], details: null };
const replay = await serveRecorded([v2Reads.responses.poolDetailReal], (_method, url) => {
  if (url.pathname === `/v1/pools/${pool}/holders`) {
    const cursor = url.searchParams.get('cursor');
    if (cursor === failedHolderCursor && holderFailures > 0) { holderFailures--; return { status: 500, body: failure }; }
    const page = cursor === null ? firstHolders : continuationHolders.get(cursor);
    return page ? { status: 200, body: page } : { status: 400, body: failure };
  }
  if (url.pathname === `/stats/${pool}/holders`) return { status: 404 };
  if (url.pathname === `/stats/${pool}/risk`) return { status: 200, body: {
    ...riskResponse,
    riskFactors: [riskDetails.hasCreatorSold === true ? 'creator_sold' : null, riskDetails.isNew === true ? 'new' : null].filter(value => value !== null),
    details: riskDetails,
  } };
  if (url.pathname === `/v2/pools/${pool}/trades`) {
    if (url.searchParams.get('cursor') === null) return { status: 200, body: firstTrades };
    if (url.searchParams.get('cursor') !== tradeCursor) return { status: 400, body: failure };
    if (tradeFailures > 0) { tradeFailures--; return { status: 500, body: failure }; }
    return { status: 200, body: servedTradeContinuation };
  }
  return { status: 404 };
});
const previousEnv = { ...process.env };
for (const name of ['DATA_API_URL', 'STATS_API_URL', 'GATEWAY_URL', 'USER_API_URL', 'METADATA_API_URL', 'PNL_API_URL', 'SOCIAL_API_URL', 'RPC_URL']) process.env[`NEXT_PUBLIC_${name}`] = replay.origin;
process.env.NEXT_PUBLIC_STREAM_ORIGIN = replay.origin.replace(/^http/, 'ws');

const { createRoot } = await import('react-dom/client');
const { MarketClientContext } = await import('@/core/market/client');
const { useMarketLive } = await import('@/page-modules/token/market-session-context');
const HoldersTable = (await import('./holders-table')).default;
const TradesTable = (await import('./trades-table')).default;
const StatsAuditPanel = (await import('./stats-audit-panel')).default;

interface Mounted { root: Root; container: HTMLDivElement; cache: QueryClient }
const mounted = new Set<Mounted>();
function mount(node: React.ReactElement): Mounted {
  const streamOrigin = replay.origin.replace(/^http/, 'ws');
  const client = new SdkClient({ apiOrigin: replay.origin, identityOrigin: replay.origin, mediaOrigin: replay.origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  const cache = new QueryClient();
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const view = { root, container, cache };
  mounted.add(view);
  act(() => root.render(createElement(QueryClientProvider, { client: cache }, createElement(MarketClientContext.Provider, { value: client }, node))));
  return view;
}
function unmount(view: Mounted) {
  act(() => view.root.unmount());
  view.cache.clear();
  view.container.remove();
  mounted.delete(view);
}
async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Condition not reached. DOM: ${document.body.textContent}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}
function button(view: Mounted, label: string) {
  return [...view.container.querySelectorAll<HTMLButtonElement>('button')].find(node => node.textContent === label);
}
function click(view: Mounted, label: string) {
  const control = button(view, label);
  expect(control, `Expected enabled ${label} control`).toBeDefined();
  expect(control!.disabled).toBe(false);
  act(() => control!.click());
}
function holderRow(view: Mounted, wallet: string) {
  return view.container.querySelector(`tbody a[href="/profile/${wallet}"]`)?.closest('tr') ?? null;
}
function requestsFor(path: string, cursor: string | null) {
  return replay.requests.filter(request => {
    const url = new URL(request.slice(request.indexOf(' ') + 1), replay.origin);
    return url.pathname === path && url.searchParams.get('cursor') === cursor;
  });
}
function TradesHarness() {
  const live = useMarketLive(pool, { enabled: false });
  return createElement(TradesTable, { poolAddress: pool, live });
}
function auditValue(view: Mounted, label: string) {
  const term = [...view.container.querySelectorAll('dt')].find(node => node.textContent === label);
  return term?.parentElement?.querySelector('dd') ?? null;
}

beforeEach(() => {
  replay.requests.length = 0;
  firstHolders = holderPage(holder, 1n, '2', oldCursor);
  continuationHolders = new Map([
    [oldCursor, holderPage(oldHolder, 2n, '2', null)],
    [newCursor, holderPage(newHolder, 3n, '3', null)],
  ]);
  failedHolderCursor = null;
  holderFailures = 0;
  tradeFailures = 0;
  servedTradeContinuation = allTrades;
  riskDetails = { ...riskResponse.details };
});
afterEach(() => { for (const view of mounted) unmount(view); });
afterAll(async () => {
  await replay.close();
  for (const key of Object.keys(process.env)) if (!(key in previousEnv)) delete process.env[key];
  Object.assign(process.env, previousEnv);
  dom.window.close();
});

async function holder_page_refresh_replaces_balances_and_cursor_chain() {
  const view = mount(createElement(HoldersTable, { poolAddress: pool }));
  await waitFor(() => holderRow(view, holder) !== null);
  expect(holderRow(view, holder)?.querySelectorAll('td')[1]?.textContent).toBe('1');
  click(view, 'Load more holders');
  await waitFor(() => holderRow(view, oldHolder) !== null);
  firstHolders = { ...firstHolders, generatedAt: '2026-10-13T08:20:00.000Z' };
  await act(async () => { await view.cache.refetchQueries({ queryKey: queryKeys.holders(replay.origin, pool, null), exact: true }); });
  expect(holderRow(view, oldHolder)).not.toBeNull();
  firstHolders = holderPage(holder, 9n, '3', newCursor);
  await act(async () => { await view.cache.refetchQueries({ queryKey: queryKeys.holders(replay.origin, pool, null), exact: true }); });
  await waitFor(() => holderRow(view, holder)?.querySelectorAll('td')[1]?.textContent === '9');
  expect(holderRow(view, oldHolder)).toBeNull();
  expect(holderRow(view, holder)?.querySelectorAll('td')[2]?.textContent).toBe('0.00%');
  click(view, 'Load more holders');
  await waitFor(() => holderRow(view, newHolder) !== null);
  expect(holderRow(view, oldHolder)).toBeNull();
  expect(requestsFor(`/v1/pools/${pool}/holders`, oldCursor)).toHaveLength(1);
  expect(requestsFor(`/v1/pools/${pool}/holders`, newCursor)).toHaveLength(1);
}
test(holder_page_refresh_replaces_balances_and_cursor_chain, holder_page_refresh_replaces_balances_and_cursor_chain);

async function holder_page_retry_click_requests_the_failed_continuation() {
  failedHolderCursor = oldCursor;
  holderFailures = 1;
  const view = mount(createElement(HoldersTable, { poolAddress: pool }));
  await waitFor(() => holderRow(view, holder) !== null);
  click(view, 'Load more holders');
  await waitFor(() => button(view, 'Try again') !== undefined);
  expect(holderRow(view, holder)).not.toBeNull();
  expect(holderRow(view, oldHolder)).toBeNull();
  expect(requestsFor(`/v1/pools/${pool}/holders`, oldCursor)).toHaveLength(1);
  click(view, 'Try again');
  await waitFor(() => holderRow(view, oldHolder) !== null);
  expect(requestsFor(`/v1/pools/${pool}/holders`, oldCursor)).toHaveLength(2);
  expect(view.container.textContent).toContain("That's everyone");
}
test(holder_page_retry_click_requests_the_failed_continuation, holder_page_retry_click_requests_the_failed_continuation);

async function risk_unknown_booleans_preserve_unknown_and_valid_verdicts() {
  const cases: Array<{ value: unknown; verdict: 'unknown' | 'pass' | 'flagged' }> = [
    { value: undefined, verdict: 'unknown' }, { value: null, verdict: 'unknown' },
    { value: 'false', verdict: 'unknown' }, { value: 'true', verdict: 'unknown' },
    { value: false, verdict: 'pass' }, { value: true, verdict: 'flagged' },
  ];
  for (const { value, verdict } of cases) {
    riskDetails = { ...riskResponse.details, hasCreatorSold: value, isNew: value };
    if (value === undefined) { delete riskDetails.hasCreatorSold; delete riskDetails.isNew; }
    const view = mount(createElement(StatsAuditPanel, { poolAddress: pool, liveStats: null }));
    click(view, 'Audit');
    await waitFor(() => auditValue(view, 'Creator sold') !== null);
    for (const label of ['Creator sold', 'New coin']) {
      const row = auditValue(view, label)!;
      expect(row.textContent).toContain(`${verdict}: `);
      if (verdict === 'unknown') {
        expect(row.textContent).toContain('status was not provided');
        expect(row.querySelector('svg.text-text-faint')).not.toBeNull();
        expect(row.querySelector('svg.text-accent')).toBeNull();
      } else {
        expect(row.textContent).toContain(value === true ? 'Yes' : 'No');
        expect(row.querySelector(value === true ? 'svg.text-danger' : 'svg.text-accent')).not.toBeNull();
      }
    }
    expect(auditValue(view, 'Age')?.textContent).toContain(`${verdict}: `);
    unmount(view);
  }
  expect(replay.requests.filter(request => request.includes(`/stats/${pool}/risk`))).toHaveLength(cases.length);
}
test(risk_unknown_booleans_preserve_unknown_and_valid_verdicts, risk_unknown_booleans_preserve_unknown_and_valid_verdicts);

async function trade_page_retry_without_data_preserves_rows_and_retries_cursor() {
  tradeFailures = 1;
  const view = mount(createElement(TradesHarness));
  await waitFor(() => view.container.querySelector('[data-market-history-size="1"]') !== null);
  click(view, 'Load older trades');
  await waitFor(() => button(view, 'Try again') !== undefined);
  expect(view.container.querySelector('[data-market-history-size="1"]')).not.toBeNull();
  expect(requestsFor(`/v2/pools/${pool}/trades`, tradeCursor)).toHaveLength(1);
  click(view, 'Try again');
  await waitFor(() => view.container.querySelector('[data-market-history-size="2"]') !== null);
  expect(requestsFor(`/v2/pools/${pool}/trades`, tradeCursor)).toHaveLength(2);
  expect(view.container.textContent).toContain('No older trades');
}
test(trade_page_retry_without_data_preserves_rows_and_retries_cursor, trade_page_retry_without_data_preserves_rows_and_retries_cursor);

async function trade_page_retry_after_cached_refetch_never_advances_cursor() {
  servedTradeContinuation = decodeV2Contract('TradesResponse', { ...allTrades, nextCursor: 'older-than-cached-page' });
  const view = mount(createElement(TradesHarness));
  await waitFor(() => view.container.querySelector('[data-market-history-size="1"]') !== null);
  click(view, 'Load older trades');
  await waitFor(() => view.container.querySelector('[data-market-history-size="2"]') !== null);
  tradeFailures = 1;
  await act(async () => { await view.cache.refetchQueries({ queryKey: queryKeys.trades(replay.origin, pool, tradeCursor), exact: true }); });
  await waitFor(() => button(view, 'Try again') !== undefined);
  expect(view.container.querySelector('[data-market-history-size="2"]')).not.toBeNull();
  click(view, 'Try again');
  await waitFor(() => requestsFor(`/v2/pools/${pool}/trades`, tradeCursor).length === 3 && button(view, 'Load older trades') !== undefined);
  expect(requestsFor(`/v2/pools/${pool}/trades`, 'older-than-cached-page')).toHaveLength(0);
  expect(view.container.querySelector('[data-market-history-size="2"]')).not.toBeNull();
}
test(trade_page_retry_after_cached_refetch_never_advances_cursor, trade_page_retry_after_cached_refetch_never_advances_cursor);

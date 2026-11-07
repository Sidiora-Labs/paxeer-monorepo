// @vitest-environment jsdom
import { createServer, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { act, createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { decodeV2Contract, SdkClient, type V2ContractTypes } from '@kindle/sdk';
import { afterEach, expect, test } from 'vitest';
import { MarketClientContext } from '@/core/market/client';
import { queryKeys } from '@/core/query-keys';
import { followStatsKey } from '@/widgets/social/use-follow';
import { CupResultsList } from './cup-results-list';
import { validateCup, type Cup, type CupStanding, type CupStandingsResponse } from './cups-contract';

const reactGlobals = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean };
reactGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const cup: Cup = validateCup({
  cupId: '12345678-1234-4234-8234-123456789abc',
  name: 'Trading cup',
  status: 'live',
  startsAt: '2026-11-01T00:00:00.000Z',
  endsAt: '2026-11-08T00:00:00.000Z',
  prizePoolRaw: '1000000',
  prizeLadder: [{ rank: 1, amountRaw: '1000000' }],
  rulesMarkdown: 'Ranked by profit',
  scoring: 'pnl',
  mode: 'solo',
});

function standing(rank: number): CupStanding {
  return { rank, wallet: `0x${rank.toString(16).padStart(40, '0')}`, metricSigned: String(1000000 - rank) };
}

function page(items: CupStanding[], nextCursor: string | null): CupStandingsResponse {
  return decodeV2Contract('CupStandingsResponse', {
    requestId: '12345678-1234-4234-8234-123456789abc',
    data: { items },
    nextCursor,
    generatedAt: '2026-11-04T00:00:00.000Z',
    watermark: null,
    revision: null,
    quality: 'complete',
  });
}

interface Reply {
  status: number;
  body: CupStandingsResponse | V2ContractTypes['Error'];
  held?: boolean;
}

function served(items: CupStanding[], nextCursor: string | null = null, held = false): Reply {
  return { status: 200, body: page(items, nextCursor), held };
}

function failure(status: number): Reply {
  return {
    status,
    body: decodeV2Contract('Error', {
      code: status === 503 ? 'DEPENDENCY_UNAVAILABLE' : 'INTERNAL_ERROR',
      message: 'Cup service unavailable',
      requestId: '12345678-1234-4234-8234-123456789abc',
      retryable: false,
      retryAfterMs: null,
      fieldErrors: [],
      details: null,
    }),
  };
}

const cleanups: Array<() => void | Promise<void>> = [];
afterEach(async () => {
  for (const cleanup of cleanups.splice(0).reverse()) await cleanup();
});

async function fixture(routes: Map<string, Reply[]>) {
  const requests: Array<{ cupId: string; cursor: string | null; limit: string | null; method: string | undefined }> = [];
  const pending: Array<() => void> = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    const match = /^\/v2\/cups\/([^/]+)\/standings$/.exec(url.pathname);
    if (!match) {
      response.writeHead(404).end();
      return;
    }
    const cursor = url.searchParams.get('cursor');
    requests.push({ cupId: match[1], cursor, limit: url.searchParams.get('limit'), method: request.method });
    const replies = routes.get(`${match[1]}:${cursor ?? ''}`);
    const reply = replies && (replies.length > 1 ? replies.shift() : replies[0]);
    if (!reply) {
      response.writeHead(500).end('Missing fixture page');
      return;
    }
    const send = () => writeReply(response, reply);
    if (reply.held) pending.push(send);
    else send();
  });
  const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => {
    resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`);
  }));
  cleanups.push(async () => {
    pending.splice(0).forEach(send => send());
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  });
  const streamOrigin = origin.replace(/^http/, 'ws');
  const client = new SdkClient({ apiOrigin: origin, mediaOrigin: origin, identityOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  return { client, requests, release: () => pending.splice(0).forEach(send => send()) };
}

function writeReply(response: ServerResponse, reply: Reply) {
  response.writeHead(reply.status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(reply.body));
}

function route(cursor: string | null, cupId = cup.cupId) {
  return `${cupId}:${cursor ?? ''}`;
}

function mount(client: SdkClient, initialCup = cup) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  // Real in-memory identity and social caches keep unrelated peers off the network.
  for (let rank = 1; rank <= 105; rank++) {
    const wallet = standing(rank).wallet;
    queryClient.setQueryData(queryKeys.walletIdentity(wallet), null);
    queryClient.setQueryData(followStatsKey(wallet, null), { wallet, followers: 0, following: 0, isFollowing: false, route: 'operation' });
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const render = (value: Cup) => act(() => root.render(
    createElement(QueryClientProvider, { client: queryClient },
      createElement(MarketClientContext.Provider, { value: client },
        createElement(CupResultsList, { cup: value }))),
  ));
  render(initialCup);
  cleanups.push(() => {
    act(() => root.unmount());
    queryClient.clear();
    container.remove();
  });
  return { container, render, queryClient };
}

async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 5000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Condition timed out: ${document.body.textContent}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
}

function button(container: HTMLElement, label: string) {
  const result = Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find(node => node.textContent === label);
  expect(result, `Expected button ${label}`).toBeDefined();
  return result!;
}

function ranks(container: HTMLElement) {
  return Array.from(container.querySelectorAll('ol[aria-label="Live standings"] li[data-rank]')).map(node => Number(node.getAttribute('data-rank')));
}

async function cup_standings_pagination_appends_beyond_one_hundred_and_exhausts_cursor() {
  const first = Array.from({ length: 100 }, (_, index) => standing(index + 1));
  const cursor = 'offset=100&sort=pnl+';
  const backend = await fixture(new Map([
    [route(null), [served(first, cursor)]],
    [route(cursor), [served([standing(101)], 'last-page', true)]],
    [route('last-page'), [served([standing(102)])]],
  ]));
  const view = mount(backend.client);
  await waitFor(() => ranks(view.container).length === 100);
  expect(backend.requests).toEqual([{ cupId: cup.cupId, cursor: null, limit: '100', method: 'GET' }]);
  act(() => button(view.container, 'Load more standings').click());
  await waitFor(() => backend.requests.length === 2);
  const loading = button(view.container, 'Loading standings');
  expect(loading.disabled).toBe(true);
  expect(loading.getAttribute('aria-busy')).toBe('true');
  act(() => loading.click());
  expect(ranks(view.container)).toEqual(first.map(row => row.rank));
  backend.release();
  await waitFor(() => ranks(view.container).length === 101);
  expect(backend.requests[1]).toEqual({ cupId: cup.cupId, cursor, limit: '100', method: 'GET' });
  act(() => button(view.container, 'Load more standings').click());
  await waitFor(() => ranks(view.container).length === 102);
  expect(ranks(view.container)).toEqual(Array.from({ length: 102 }, (_, index) => index + 1));
  expect(view.container.querySelector('a[href="/profile/' + standing(102).wallet + '"]')).not.toBeNull();
  expect(view.container.textContent).not.toContain('Load more standings');
  expect(backend.requests.map(request => request.cursor)).toEqual([null, cursor, 'last-page']);
}

async function cup_standings_pagination_retries_failed_continuation_without_losing_rows() {
  const backend = await fixture(new Map([
    [route(null), [served([standing(1)], 'next')]],
    [route('next'), [failure(500), served([standing(2)])]],
  ]));
  const view = mount(backend.client);
  await waitFor(() => ranks(view.container).length === 1);
  act(() => button(view.container, 'Load more standings').click());
  await waitFor(() => view.container.textContent!.includes("Couldn't load more standings"));
  expect(ranks(view.container)).toEqual([1]);
  act(() => button(view.container, 'Try again').click());
  await waitFor(() => ranks(view.container).length === 2);
  expect(ranks(view.container)).toEqual([1, 2]);
  expect(backend.requests.map(request => request.cursor)).toEqual([null, 'next', 'next']);
}

async function cup_standings_pagination_rejects_conflicting_rank_wallet_order_and_cursor() {
  const first = standing(10);
  const conflicts: Array<{ items: CupStanding[]; next: string | null; message: string }> = [
    { items: [{ ...standing(11), rank: 10 }], next: null, message: 'Cup standing rank disagreement' },
    { items: [{ ...standing(11), wallet: first.wallet.replace('a', 'A') }], next: null, message: 'Cup standing identity disagreement' },
    { items: [standing(9)], next: null, message: 'Cup standings page order disagreement' },
    { items: [standing(11)], next: 'next', message: 'Cup standings cursor disagreement' },
  ];
  for (const conflict of conflicts) {
    const backend = await fixture(new Map([
      [route(null), [served([first], 'next')]],
      [route('next'), [served(conflict.items, conflict.next), served([standing(11)])]],
    ]));
    const view = mount(backend.client);
    await waitFor(() => ranks(view.container).length === 1);
    act(() => button(view.container, 'Load more standings').click());
    await waitFor(() => view.container.textContent!.includes("Couldn't load more standings"));
    expect(ranks(view.container)).toEqual([10]);
    const failedQuery = view.queryClient.getQueryCache().getAll().find(query => query.state.error !== null);
    expect(failedQuery?.state.error?.message).toBe(conflict.message);
    act(() => button(view.container, 'Try again').click());
    await waitFor(() => ranks(view.container).length === 2);
    expect(ranks(view.container)).toEqual([10, 11]);
    expect(backend.requests.map(request => request.cursor)).toEqual([null, 'next', 'next']);
  }
}

async function cup_standings_pagination_preserves_real_unavailable_initial_response() {
  const backend = await fixture(new Map([[route(null), [failure(503), served([standing(1)])]]]));
  const view = mount(backend.client);
  await waitFor(() => view.container.textContent!.includes('Cup standings is not available yet'));
  expect(ranks(view.container)).toEqual([]);
  expect(view.container.textContent).not.toContain('No one has qualified yet');
  expect(view.container.textContent).not.toContain('Load more standings');
  const failedQuery = view.queryClient.getQueryCache().getAll().find(query => query.state.error !== null);
  expect(failedQuery?.state.error).toMatchObject({ status: 503 });
  act(() => button(view.container, 'Try again').click());
  await waitFor(() => ranks(view.container).length === 1);
  expect(backend.requests.map(request => request.cursor)).toEqual([null, null]);
}

async function cup_standings_pagination_preserves_rows_when_continuation_is_unavailable() {
  const backend = await fixture(new Map([
    [route(null), [served([standing(1)], 'next')]],
    [route('next'), [failure(503), served([standing(2)])]],
  ]));
  const view = mount(backend.client);
  await waitFor(() => ranks(view.container).length === 1);
  act(() => button(view.container, 'Load more standings').click());
  await waitFor(() => view.container.textContent!.includes('More cup standings is not available yet'));
  expect(ranks(view.container)).toEqual([1]);
  act(() => button(view.container, 'Try again').click());
  await waitFor(() => ranks(view.container).length === 2);
  expect(backend.requests.map(request => request.cursor)).toEqual([null, 'next', 'next']);
}

async function cup_standings_pagination_isolates_cup_changes_during_pending_continuation() {
  const otherCup: Cup = { ...cup, cupId: '12345678-1234-4234-8234-123456789abd' };
  const backend = await fixture(new Map([
    [route(null), [served([standing(1)], 'next')]],
    [route('next'), [served([standing(2)], null, true)]],
    [route(null, otherCup.cupId), [served([standing(3)])]],
  ]));
  const view = mount(backend.client);
  await waitFor(() => ranks(view.container).length === 1);
  act(() => button(view.container, 'Load more standings').click());
  await waitFor(() => backend.requests.length === 2);
  view.render(otherCup);
  await waitFor(() => ranks(view.container)[0] === 3);
  await act(async () => { backend.release(); await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(ranks(view.container)).toEqual([3]);
  expect(view.container.textContent).not.toContain('Load more standings');
  expect(backend.requests.map(request => request.cupId)).toEqual([cup.cupId, cup.cupId, otherCup.cupId]);
}

async function cup_standings_pagination_keeps_continuation_after_an_empty_page() {
  const backend = await fixture(new Map([
    [route(null), [served([], 'next')]],
    [route('next'), [served([standing(1)])]],
  ]));
  const view = mount(backend.client);
  await waitFor(() => view.container.textContent!.includes('Load more standings'));
  act(() => button(view.container, 'Load more standings').click());
  await waitFor(() => ranks(view.container).length === 1);
  expect(ranks(view.container)).toEqual([1]);
  expect(view.container.textContent).not.toContain('Load more standings');
}

test(cup_standings_pagination_appends_beyond_one_hundred_and_exhausts_cursor.name, cup_standings_pagination_appends_beyond_one_hundred_and_exhausts_cursor);
test(cup_standings_pagination_retries_failed_continuation_without_losing_rows.name, cup_standings_pagination_retries_failed_continuation_without_losing_rows);
test(cup_standings_pagination_rejects_conflicting_rank_wallet_order_and_cursor.name, cup_standings_pagination_rejects_conflicting_rank_wallet_order_and_cursor);
test(cup_standings_pagination_preserves_real_unavailable_initial_response.name, cup_standings_pagination_preserves_real_unavailable_initial_response);
test(cup_standings_pagination_preserves_rows_when_continuation_is_unavailable.name, cup_standings_pagination_preserves_rows_when_continuation_is_unavailable);
test(cup_standings_pagination_isolates_cup_changes_during_pending_continuation.name, cup_standings_pagination_isolates_cup_changes_during_pending_continuation);
test(cup_standings_pagination_keeps_continuation_after_an_empty_page.name, cup_standings_pagination_keeps_continuation_after_an_empty_page);

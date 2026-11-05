import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import { act, createElement } from 'react';
import { afterAll, expect, test } from 'vitest';
import { SdkClient } from '@kindle/sdk';
import type { FeedCard, FeedPage } from '@/core/market/use-feed';
import { queryKeys } from '@/core/query-keys';

// Retain Node's fetch/AbortController while giving the real React hooks a DOM.
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
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider, notifyManager } = await import('@tanstack/react-query');
const { MarketClientContext } = await import('@/core/market/client');
const { CalloutsWidget } = await import('./callouts-widget');
type Feed = NonNullable<Parameters<typeof CalloutsWidget>[0]['feed']>;

afterAll(() => { dom.window.close(); });

// In-memory discovery pages served over HTTP and decoded by the production SDK.
function card(id: number, removed = false): FeedCard {
  return {
    Body: `Callout ${id}`,
    PositionDisclosure: '', PositionDisclosureKind: 'declared', PositionVerified: false,
    State: removed ? 'removed' : 'visible',
    Source: {
      Authorized: true, Corrected: false, CreatorPublicID: 'account-1',
      EventID: `callout:c${id}:1`, ID: `c${id}`, Kind: 'callout', Pools: [],
      Public: !removed, PublishedAt: '2026-10-12T11:30:00Z', SourceRevision: 1,
      Tombstone: removed, VisibilityRevision: removed ? 2 : 1,
    },
  };
}

function fullPage(first: number): FeedCard[] {
  return Array.from({ length: 50 }, (_, index) => card(first + index));
}

async function waitFor(check: () => boolean, diagnostics: () => string): Promise<void> {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Feed did not reach the expected state; ${diagnostics()}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
}

async function openFeed(entries: FeedCard[]) {
  let revision = 0;
  let page: FeedPage = { Algorithm: 'chronological', Cursor: '', Entries: entries, FallbackReason: '', Personalized: false, Snapshot: String(revision) };
  const requests: URL[] = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://feed.invalid');
    requests.push(url);
    if (url.pathname !== '/v2/discovery/feed') { response.writeHead(404); response.end(); return; }
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(page));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: origin.replace('http:', 'ws:'), approvedStreamOrigins: [origin.replace('http:', 'ws:')], production: false });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const events: unknown[] = [];
  const stopRecording = queryClient.getQueryCache().subscribe(event => {
    if (event.query.queryKey[1] !== 'feed' || event.type !== 'updated' || event.action.type !== 'success') return;
    const received = event.query.state.data as FeedPage;
    events.push({ key: event.query.queryKey, snapshot: received.Snapshot, identities: received.Entries.map(entry => entry.Source.EventID), observers: event.query.getObserversCount() });
  });
  const diagnostics = () => `received events=${JSON.stringify(events)}`;
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  let feed: Feed = 'latest';
  function render(filter = '') {
    act(() => {
      root.render(createElement(QueryClientProvider, { client: queryClient },
        createElement(MarketClientContext.Provider, { value: client }, createElement(CalloutsWidget, { feed, filter, columnId: 'repair-callouts', density: 'comfortable', view: 'list' })),
      ));
    });
  }
  function pending() {
    const label = button().textContent;
    if (label === 'Live' || label === 'Paused') return 0;
    const match = /^Paused, (\d+) new$/.exec(label ?? '');
    if (!match) throw new Error(`Unexpected pause button: ${label}; ${diagnostics()}`);
    return Number(match[1]);
  }
  function visible() { return Array.from(container.querySelectorAll('article p.whitespace-pre-wrap'), row => row.textContent); }
  function button() {
    const result = container.querySelector<HTMLButtonElement>('button[aria-pressed]');
    if (!result) throw new Error(`Missing production pause button; ${diagnostics()}`);
    return result;
  }
  function feedKey() { return queryKeys.feed(feed === 'latest' ? 'chronological' : 'relevance', null, 50); }
  async function receivePage() {
    await waitFor(() => queryClient.getQueryData<FeedPage>(feedKey())?.Snapshot === page.Snapshot && !queryClient.isFetching(), diagnostics);
    // HTTP completion precedes the batched notification to the widget's real query observer.
    // Drain that queue inside act before checking the rendered counter or freezing another page.
    await act(async () => { await new Promise<void>(resolve => notifyManager.schedule(resolve)); });
    expect(queryClient.getQueryCache().find({ queryKey: feedKey() })?.getObserversCount(), diagnostics()).toBe(1);
  }
  async function close() {
    act(() => root.unmount());
    await queryClient.cancelQueries();
    stopRecording();
    queryClient.clear();
    container.remove();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
  try {
    render();
    await receivePage();
    await waitFor(() => visible().length === entries.length, diagnostics);
    expect(requests).toHaveLength(1);
    expect(requests[0].searchParams.get('limit')).toBe('50');
    expect(requests[0].searchParams.get('order')).toBe('chronological');
  } catch (error) { await close(); throw error; }
  return {
    pending, visible, button, render, close, diagnostics,
    toggle() { act(() => button().click()); },
    async refresh(next: FeedCard[]) {
      page = { ...page, Entries: next, Snapshot: String(++revision) };
      const before = requests.length;
      await act(async () => {
        await queryClient.invalidateQueries({ queryKey: feedKey() });
      });
      await receivePage();
      expect(requests.length, diagnostics()).toBeGreaterThan(before);
      expect(events.at(-1), diagnostics()).toMatchObject({ snapshot: page.Snapshot, identities: next.map(entry => entry.Source.EventID), observers: 1 });
    },
    async switchFeed(next: Feed) {
      feed = next;
      render();
      await receivePage();
    },
  };
}

async function pausedFeedCountsArrivalsOnFullPage() {
  const initial = fullPage(0);
  const feed = await openFeed(initial);
  try {
    expect(feed.visible()).toHaveLength(50);
    feed.toggle();
    await feed.refresh(fullPage(1));
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    expect(feed.button().textContent, feed.diagnostics()).toBe('Paused, 1 new');
    expect(feed.visible()).toEqual(initial.map(entry => entry.Body));
    await feed.refresh(fullPage(50));
    expect(feed.pending(), feed.diagnostics()).toBe(50);
    await feed.refresh(fullPage(100));
    expect(feed.pending(), feed.diagnostics()).toBe(100);
    // Previously evicted arrivals and frozen identities must not count twice.
    await feed.refresh(fullPage(50));
    expect(feed.pending(), feed.diagnostics()).toBe(100);
    await feed.refresh([...initial].reverse());
    expect(feed.pending(), feed.diagnostics()).toBe(100);
    expect(feed.visible()).toEqual(initial.map(entry => entry.Body));
    feed.toggle();
    expect(feed.pending(), feed.diagnostics()).toBe(0);
    expect(feed.button().textContent, feed.diagnostics()).toBe('Live');
    expect(feed.visible()).toEqual([...initial].reverse().map(entry => entry.Body));
    feed.toggle();
    await feed.refresh(fullPage(1));
    expect(feed.pending(), feed.diagnostics()).toBe(1);
  } finally { await feed.close(); }
}

async function pausedFeedModerationNeverSubtractsArrivals() {
  const initial = fullPage(0);
  const feed = await openFeed(initial);
  try {
    feed.toggle();
    await feed.refresh(initial.slice(1));
    expect(feed.pending(), feed.diagnostics()).toBe(0);
    expect(feed.button().textContent, feed.diagnostics()).toBe('Paused');
    expect(feed.visible()).toEqual(initial.map(entry => entry.Body));
    await feed.refresh([card(50), ...initial.slice(2)]);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    await feed.refresh([card(50, true), ...initial.slice(2)]);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    await feed.refresh([]);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    await feed.refresh([card(50), card(51, true), ...initial.slice(2)]);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    expect(feed.visible()).toEqual(initial.map(entry => entry.Body));
  } finally { await feed.close(); }
}

async function pausedFeedDeduplicatesRevisionsAndIgnoresFilterChanges() {
  const feed = await openFeed([card(0)]);
  try {
    feed.toggle();
    const corrected = card(0);
    corrected.Source.Corrected = true;
    corrected.Source.SourceRevision = 2;
    corrected.Source.EventID = 'callout:c0:2';
    corrected.Body = 'Corrected body';
    await feed.refresh([corrected]);
    expect(feed.pending(), feed.diagnostics()).toBe(0);
    expect(feed.visible()).toEqual(['Callout 0']);
    await feed.refresh([card(1), card(1), corrected]);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    feed.render('no matching callouts');
    expect(feed.visible()).toEqual([]);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    feed.render();
    expect(feed.visible()).toEqual(['Callout 0']);
    expect(feed.pending(), feed.diagnostics()).toBe(1);
  } finally { await feed.close(); }
}

async function pausedFeedSwitchingOrderResetsSnapshot() {
  const feed = await openFeed(fullPage(0));
  try {
    feed.toggle();
    await feed.refresh(fullPage(1));
    expect(feed.pending(), feed.diagnostics()).toBe(1);
    await feed.switchFeed('hot');
    expect(feed.pending(), feed.diagnostics()).toBe(0);
    expect(feed.button().getAttribute('aria-pressed')).toBe('false');
    expect(feed.visible()).toEqual(fullPage(1).map(entry => entry.Body));
    feed.toggle();
    await feed.refresh(fullPage(2));
    expect(feed.pending(), feed.diagnostics()).toBe(1);
  } finally { await feed.close(); }
}

test(pausedFeedCountsArrivalsOnFullPage.name, pausedFeedCountsArrivalsOnFullPage);
test(pausedFeedModerationNeverSubtractsArrivals.name, pausedFeedModerationNeverSubtractsArrivals);
test(pausedFeedDeduplicatesRevisionsAndIgnoresFilterChanges.name, pausedFeedDeduplicatesRevisionsAndIgnoresFilterChanges);
test(pausedFeedSwitchingOrderResetsSnapshot.name, pausedFeedSwitchingOrderResetsSnapshot);

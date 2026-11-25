import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import React, { act, useState } from 'react';
import { afterAll, expect, test } from 'vitest';
import { SdkClient, type V2ContractTypes } from '@kindle/sdk';
import type { TrendingCategory } from './trending-widget';

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
afterAll(() => dom.window.close());

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { defineChain } = await import('viem');
const { MarketClientContext } = await import('@/core/market/client');
const { useExploreRows } = await import('@/widgets/explore/use-explore-rows');
const { TrendingWidget, TrendingColumn, trendingEmptyTitles } = await import('./trending-widget');

type RankingsResponse = V2ContractTypes['RankingsResponse'];
type Quality = RankingsResponse['quality'];
const paxeer = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: ['http://127.0.0.1:1'] } } });
const wagmiConfig = createConfig({ chains: [paxeer], transports: { [paxeer.id]: http() }, ssr: true, multiInjectedProviderDiscovery: false });
const pool = '0x' + '1'.repeat(40);
const notice = 'Rankings may be a little behind right now';
const props = { columnId: 'home-1', filter: '', density: 'comfortable' as const, view: 'list' as const };
const watermark: V2ContractTypes['Watermark'] = { chainId: '125', height: '4200', blockHash: '0x' + '6'.repeat(64), projectionRevision: '7', integrity: 'healthy', observedAt: '2026-10-12T15:00:00.000Z' };

function rankingPage(quality: Quality, withRows: boolean): RankingsResponse {
  return {
    requestId: '0f0e2a8e-5b3c-4d8a-9a1e-2c6d7b8f9a01', generatedAt: '2026-10-12T15:00:01.000Z', nextCursor: null, quality, revision: '7', watermark,
    data: { snapshot: {
      snapshotId: '7b6f2b4e-3c1d-4e2f-9a8b-1c2d3e4f5a6b', category: 'trending', modelVersion: 'dev-v1', computedAt: watermark.observedAt, watermark,
      items: withRows ? [{ category: 'trending', modelVersion: 'dev-v1', computedAt: watermark.observedAt, poolAddress: pool, rank: 1, score: '90' }] : [],
      totalEligible: withRows ? '1' : '0', truncated: false, replace: true,
    } },
  };
}

function QuarantinedColumn({ filter }: { filter: string }) {
  const [category, setCategory] = useState<TrendingCategory>('trending');
  const explore = useExploreRows(category);
  const snapshot: RankingsResponse | null = explore.snapshot ? { ...explore.snapshot, quality: 'quarantined' } : null;
  return <TrendingColumn {...props} filter={filter} category={category} setCategory={setCategory} explore={{ ...explore, snapshot }} />;
}

async function assertNotice(quality: Quality, withRows: boolean, expected: boolean, filter = '') {
  const paths: string[] = [];
  const server = createServer((request, response) => {
    paths.push(request.url ?? '');
    if (request.url === '/v2/rankings/trending') {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify(rankingPage(quality === 'quarantined' ? 'partial' : quality, withRows)));
    } else {
      response.statusCode = 404;
      response.end();
    }
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: 'ws://127.0.0.1:1', approvedStreamOrigins: ['ws://127.0.0.1:1'], production: false });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  try {
    await act(async () => {
      root.render(
        <WagmiProvider config={wagmiConfig}>
          <QueryClientProvider client={queryClient}>
            <MarketClientContext.Provider value={client}>
              {quality === 'quarantined' ? <QuarantinedColumn filter={filter} /> : <TrendingWidget {...props} filter={filter} />}
            </MarketClientContext.Provider>
          </QueryClientProvider>
        </WagmiProvider>,
      );
    });
    const visibleRows = withRows && !filter;
    const deadline = Date.now() + 4000;
    while (visibleRows ? !container.querySelector(`li[data-pool="${pool}"]`) : !container.textContent?.includes(filter ? 'No coins match this filter' : trendingEmptyTitles.trending)) {
      if (Date.now() > deadline) throw new Error(`Trending did not render its ${visibleRows ? 'rows' : 'empty state'}`);
      await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
    }
    expect(paths).toContain('/v2/rankings/trending');
    expect(container.querySelectorAll('ol > li')).toHaveLength(visibleRows ? 1 : 0);
    expect(container.querySelector('[aria-label="Ranking"]')).not.toBeNull();
    if (expected) expect(container.textContent).toContain(notice);
    else expect(container.textContent).not.toContain(notice);
  } finally {
    act(() => root.unmount());
    queryClient.clear();
    container.remove();
    server.closeAllConnections();
    await new Promise<void>(resolve => server.close(() => resolve()));
  }
}

async function trendingPartialRowsHaveNoLateNotice() {
  await assertNotice('partial', true, false);
}

async function trendingStaleRowsShowLateNotice() {
  await assertNotice('stale', true, true);
}

async function trendingQuarantinedRowsShowLateNotice() {
  await assertNotice('quarantined', true, true);
}

async function trendingCompleteRowsHaveNoLateNotice() {
  await assertNotice('complete', true, false);
}

async function trendingEmptyRankingsHaveNoLateNotice() {
  for (const quality of ['partial', 'stale', 'quarantined', 'complete'] as const) {
    await assertNotice(quality, false, false);
  }
}

async function trendingFilteredRowsHaveNoLateNotice() {
  for (const quality of ['partial', 'stale', 'quarantined', 'complete'] as const) {
    await assertNotice(quality, true, false, 'no matching coin');
  }
}

test(trendingPartialRowsHaveNoLateNotice.name, trendingPartialRowsHaveNoLateNotice);
test(trendingStaleRowsShowLateNotice.name, trendingStaleRowsShowLateNotice);
test(trendingQuarantinedRowsShowLateNotice.name, trendingQuarantinedRowsShowLateNotice);
test(trendingCompleteRowsHaveNoLateNotice.name, trendingCompleteRowsHaveNoLateNotice);
test(trendingEmptyRankingsHaveNoLateNotice.name, trendingEmptyRankingsHaveNoLateNotice);
test(trendingFilteredRowsHaveNoLateNotice.name, trendingFilteredRowsHaveNoLateNotice);

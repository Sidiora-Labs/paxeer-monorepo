import { createRequire } from 'node:module';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { decodeV2Contract, SdkClient } from '@kindle/sdk';
import { afterEach, expect, test } from 'vitest';
import { MarketClientContext } from '@/core/market/client';
import type { PoolSnapshot, PoolStats } from '@/core/market/pool-resolver';
import { queryKeys } from '@/core/query-keys';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import PoolInfoCard from './pool-info-card';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as typeof import('jsdom');
const detail = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body).data;
const snapshot: PoolSnapshot = {
  pool: { chainId: '125', poolAddress: detail.pool.poolAddress, tokenAddress: detail.pool.tokenAddress },
  token: detail.token,
  metadata: null,
  stats: { ...detail.stats, realUsdlBalanceRaw: '100000000' },
  creator: detail.pool.creator,
  createdAt: '2026-10-13T00:00:00.000Z',
  generatedAt: '2026-10-13T00:00:00.000Z',
  resolvedBy: 'detail',
  unavailable: { stats: null, metadata: null, registry: null },
};
const liveStats: PoolStats = {
  ...detail.stats,
  realUsdlBalanceRaw: '250000000',
  circulatingMarketCapUsdlRaw: '300000000',
  indicativeTvlUsdlRaw: '500000000',
};
const sdk = new SdkClient({
  apiOrigin: 'http://localhost:3000', identityOrigin: 'http://localhost:3000',
  mediaOrigin: 'http://localhost:3000', streamOrigin: 'ws://localhost:3000',
  approvedStreamOrigins: ['ws://localhost:3000'], production: false,
});
const clients: QueryClient[] = [];

afterEach(() => {
  for (const client of clients.splice(0)) client.clear();
});

function renderCard(stats?: PoolStats | null) {
  const client = new QueryClient();
  clients.push(client);
  client.setQueryData(queryKeys.poolSnapshot(snapshot.pool.poolAddress), snapshot);
  return JSDOM.fragment(renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <MarketClientContext.Provider value={sdk}>
        <PoolInfoCard poolAddress={snapshot.pool.poolAddress} liveStats={stats} />
      </MarketClientContext.Provider>
    </QueryClientProvider>,
  ));
}

function row(card: DocumentFragment, label: string) {
  const term = Array.from(card.querySelectorAll('dt')).find(node => node.textContent === label);
  expect(term, `${label} row`).toBeDefined();
  return term!.nextElementSibling!;
}

function cardShowsTheLiveLiquidity() {
  const card = renderCard(liveStats);
  expect(row(card, 'Total liquidity').textContent).toBe('$250.00');
  expect(row(card, 'Circulating market cap').textContent).toBe('$300.00');
  expect(row(card, 'Total value locked').textContent).toBe('$500.00');
}
test(cardShowsTheLiveLiquidity.name, cardShowsTheLiveLiquidity);

function cardKeepsTheSnapshotWithoutLiveStats() {
  for (const stats of [undefined, null]) {
    expect(row(renderCard(stats), 'Total liquidity').textContent).toBe('$100.00');
  }
}
test(cardKeepsTheSnapshotWithoutLiveStats.name, cardKeepsTheSnapshotWithoutLiveStats);

function cardKeepsItsIdentityRows() {
  const card = renderCard(liveStats);
  for (const [label, address] of [['Pool', snapshot.pool.poolAddress], ['Coin', snapshot.pool.tokenAddress]]) {
    const value = row(card, label);
    expect(value.querySelector('[title]')?.getAttribute('title')).toBe(address);
    expect(value.querySelector('.sr-only')?.textContent).toBe(address);
    expect(value.querySelector('a')?.getAttribute('href')).toBe(`https://paxscan.paxeer.app/address/${address}`);
  }
  expect(row(card, 'Created').querySelector('time')?.getAttribute('datetime')).toBe(snapshot.createdAt);
}
test(cardKeepsItsIdentityRows.name, cardKeepsItsIdentityRows);

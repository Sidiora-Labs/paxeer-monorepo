// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import type { V2ContractTypes } from '@kindle/sdk';
import { queryKeys } from '@/core/query-keys';
import { validateLeaderboard } from '@/core/market/use-leaderboard';
import TickerStrip from './ticker-strip';
import { useEarnersTicker, type TickerFeed } from './use-ticker';

type Entry = V2ContractTypes['LeaderboardEntry'];
const wallets = [
  '0x1111111111111111111111111111111111111111',
  '0x2222222222222222222222222222222222222222',
  '0x3333333333333333333333333333333333333333',
];
const mounted: Array<{ root: Root; queries: QueryClient; container: HTMLDivElement }> = [];
const actEnvironment = globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean };
const previousActEnvironment = actEnvironment.IS_REACT_ACT_ENVIRONMENT;

beforeAll(() => { actEnvironment.IS_REACT_ACT_ENVIRONMENT = true; });
afterAll(() => { actEnvironment.IS_REACT_ACT_ENVIRONMENT = previousActEnvironment; });
afterEach(() => {
  for (const { root, queries, container } of mounted.splice(0)) {
    try { act(() => root.unmount()); }
    finally { queries.clear(); container.remove(); }
  }
  window.localStorage.clear();
});

function entry(rank: number, metricSigned?: string, quality: Entry['quality'] = 'complete'): Entry {
  return {
    rank,
    wallet: wallets[rank - 1],
    metricRaw: metricSigned?.replace(/^-/, '') ?? '0',
    ...(metricSigned === undefined ? {} : { metricSigned }),
    quality,
  };
}

function mount(entries: Entry[]) {
  const queries = new QueryClient();
  const watermark: V2ContractTypes['Watermark'] = {
    chainId: '125', height: '1', blockHash: `0x${'1'.repeat(64)}`,
    projectionRevision: '1', observedAt: '2026-11-26T00:00:00Z', integrity: 'healthy',
  };
  const board: V2ContractTypes['LeaderboardsResponse'] = {
    data: { snapshot: {
      category: 'trader', window: '24h', entries, watermark,
      methodologyVersion: 'dev-indexer-swaps-v1', snapshotId: '2881b873-06ec-5544-a753-30a471366389',
    } },
    generatedAt: '2026-11-26T00:00:00Z', nextCursor: null, quality: 'complete',
    requestId: '0f0e2a8e-5b3c-4d8a-9a1e-2c6d7b8f9a01', revision: '1', watermark,
  };
  queries.setQueryData(queryKeys.leaderboard('trader', '24h', 20), validateLeaderboard(board, 'trader', '24h', 20));
  for (const row of entries) queries.setQueryData(queryKeys.walletIdentity(row.wallet), null);
  window.localStorage.setItem('sidiora-ticker-mode', 'earners');
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  mounted.push({ root, queries, container });
  const observed: { feed?: TickerFeed } = {};
  function Probe() { observed.feed = useEarnersTicker(); return null; }
  act(() => {
    root.render(<QueryClientProvider client={queries}><Probe /><TickerStrip /></QueryClientProvider>);
  });
  const { feed } = observed;
  if (!feed || feed.state !== 'ready') throw new Error('Top earners feed is not ready');
  const chips = Array.from(container.querySelectorAll('ul:not([aria-hidden="true"]) a[href^="/profile/"]'));
  return { feed, container, chips };
}

function stripLeavesOutALoss() {
  const { feed, chips } = mount([entry(1, '5000000'), entry(2, '-2000000'), entry(3, '1000000')]);
  expect(feed.chips.map(chip => chip.key)).toEqual([wallets[0], wallets[2]]);
  expect(chips.map(chip => chip.getAttribute('href'))).toEqual([`/profile/${wallets[0]}`, `/profile/${wallets[2]}`]);
  expect(feed.chips.map(chip => chip.value)).toEqual(['+5.00 USDL', '+1.00 USDL']);
  expect(feed.emptyText).toBeNull();
}

function stripLeavesOutAZeroFigure() {
  const { feed, chips } = mount([entry(1, '0'), entry(2, '-0')]);
  expect(feed.chips).toEqual([]);
  expect(chips).toHaveLength(0);
}

function stripKeepsAnEntryWithoutASignedFigure() {
  const { feed, chips } = mount([entry(1)]);
  expect(feed.chips).toMatchObject([{ key: wallets[0], value: '0.00 USDL', tone: 'flat' }]);
  expect(chips).toHaveLength(1);
  expect(chips[0].getAttribute('href')).toBe(`/profile/${wallets[0]}`);
}

function stripShowsItsEmptyTextWithoutAGain() {
  const { feed, container, chips } = mount([entry(1, '-2000000')]);
  expect(feed.chips).toEqual([]);
  expect(chips).toHaveLength(0);
  expect(feed.emptyText).toBe('No top earners in the last 24h');
  expect(feed.emptyShort).toBe('None in 24h');
  const empty = container.querySelector('[data-ticker-empty]');
  expect(empty).not.toBeNull();
  expect(empty?.querySelector('.sm\\:hidden')?.textContent).toBe('None in 24h');
  expect(empty?.querySelector('.sm\\:inline')?.textContent).toBe('No top earners in the last 24h');
}

function stripKeepsUnavailableFigures() {
  const { feed, chips } = mount([entry(1, '-2000000', 'unavailable'), entry(2, '0', 'unavailable')]);
  expect(feed.chips.map(chip => ({ key: chip.key, value: chip.value, tone: chip.tone }))).toEqual([
    { key: wallets[0], value: null, tone: 'neutral' },
    { key: wallets[1], value: null, tone: 'neutral' },
  ]);
  expect(chips.map(chip => chip.getAttribute('href'))).toEqual([`/profile/${wallets[0]}`, `/profile/${wallets[1]}`]);
}

test(stripLeavesOutALoss.name, stripLeavesOutALoss);
test(stripLeavesOutAZeroFigure.name, stripLeavesOutAZeroFigure);
test(stripKeepsAnEntryWithoutASignedFigure.name, stripKeepsAnEntryWithoutASignedFigure);
test(stripShowsItsEmptyTextWithoutAGain.name, stripShowsItsEmptyTextWithoutAGain);
test(stripKeepsUnavailableFigures.name, stripKeepsUnavailableFigures);

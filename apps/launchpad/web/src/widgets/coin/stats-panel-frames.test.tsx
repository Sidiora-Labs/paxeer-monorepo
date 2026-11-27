// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, onlineManager } from '@tanstack/react-query';
import { decodeV2Contract } from '@kindle/sdk';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import wire from '@/core/test-fixtures/market-production/core-api-wire.json';
import type { PoolSnapshot } from '@/core/market/pool-resolver';
import { queryKeys } from '@/core/query-keys';
import StatsAuditPanel from './stats-audit-panel';

const detail = decodeV2Contract('PoolDetailResponse', wire.v2Reads.responses.poolDetailReal.body);
const candles = decodeV2Contract('CandlesResponse', wire.candles.body);
const pool = detail.data.pool.poolAddress;
const now = Date.parse('2026-10-13T07:15:00.000Z');
const snapshot: PoolSnapshot = {
  pool: { chainId: '125', poolAddress: pool, tokenAddress: detail.data.token.address },
  token: detail.data.token,
  stats: detail.data.stats,
  metadata: null,
  creator: detail.data.pool.creator,
  createdAt: detail.data.pool.createdAt ?? null,
  generatedAt: detail.generatedAt,
  resolvedBy: 'detail',
  unavailable: { stats: null, metadata: null, registry: null },
};
const mounted: Array<{ root: Root; host: HTMLDivElement; client: QueryClient }> = [];
let wasOnline: boolean;

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(now);
  wasOnline = onlineManager.isOnline();
  onlineManager.setOnline(false);
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

afterEach(() => {
  for (const { root, host, client } of mounted.splice(0)) {
    act(() => root.unmount());
    client.clear();
    host.remove();
  }
  onlineManager.setOnline(wasOnline);
  vi.useRealTimers();
});

function mount(ready = true, stats: PoolSnapshot['stats'] = snapshot.stats) {
  const client = new QueryClient();
  if (ready) client.setQueryData(queryKeys.poolSnapshot(pool), { ...snapshot, stats });
  for (const [interval, milliseconds, limit] of [
    ['1m', 5 * 60_000, 10],
    ['5m', 60 * 60_000, 20],
    ['1h', 24 * 60 * 60_000, 40],
  ] as const) {
    client.setQueryData(queryKeys.candles(pool, interval, new Date(now - milliseconds).toISOString(), new Date(now).toISOString(), limit), {
      ...candles,
      data: { ...candles.data, items: candles.data.items.map(candle => ({ ...candle, interval })) },
    });
  }
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  mounted.push({ root, host, client });
  act(() => root.render(<QueryClientProvider client={client}><StatsAuditPanel poolAddress={pool} liveStats={null} /></QueryClientProvider>));
  return { host, client };
}

function selectWindow(host: HTMLElement, label: string) {
  const button = Array.from(host.querySelectorAll<HTMLButtonElement>('[role="radio"]')).find(node => node.textContent === label);
  expect(button).toBeDefined();
  act(() => button!.click());
  expect(button!.getAttribute('aria-checked')).toBe('true');
}

function panelShowsNoBuyersAndSellersFrame() {
  const { host } = mount();
  for (const window of ['24h', '1h', '5m']) {
    selectWindow(host, window);
    expect(host.querySelector('[role="tabpanel"]')).not.toBeNull();
    expect(host.textContent).not.toMatch(/buyers versus sellers/i);
    expect(host.querySelector('[data-split-frame="buyers"]')).toBeNull();
  }
}
test(panelShowsNoBuyersAndSellersFrame.name, panelShowsNoBuyersAndSellersFrame);

function shortWindowShowsTradesWithoutAFrame() {
  const { host } = mount();
  for (const window of ['1h', '5m']) {
    selectWindow(host, window);
    const trades = Array.from(host.querySelectorAll('dt')).find(node => node.textContent === 'Trades');
    expect(trades).toBeDefined();
    expect(trades!.nextElementSibling?.textContent).toBe('2');
    expect(host.querySelector('[data-split-frame="buys"]')).toBeNull();
    expect(host.querySelector('[role="img"][aria-label*="Buys"]')).toBeNull();
    expect(host.querySelector('[role="img"][aria-label*="Buy vol"]')).not.toBeNull();
  }
}
test(shortWindowShowsTradesWithoutAFrame.name, shortWindowShowsTradesWithoutAFrame);

async function dayWindowKeepsItsBuysAndSells() {
  const { host, client } = mount(false);
  const frame = host.querySelector('[data-split-frame="buys"]');
  expect(frame?.textContent).toContain('Loading buy and sell counts for 24h');
  expect(frame?.querySelector('.sl-skeleton')).not.toBeNull();
  expect(host.querySelector('[data-split-frame="buyers"]')).toBeNull();
  selectWindow(host, '1h');
  expect(host.querySelector('[data-split-frame="buys"]')).toBeNull();
  selectWindow(host, '24h');
  await act(async () => {
    client.setQueryData(queryKeys.poolSnapshot(pool), snapshot);
    await vi.advanceTimersByTimeAsync(0);
  });
  const bar = host.querySelector('[role="img"][aria-label="1 Buys versus 1 Sells, 50 percent Buys"]');
  expect(bar).not.toBeNull();
  expect((bar!.firstElementChild as HTMLElement).style.width).toBe('50%');
  expect(host.querySelector('[data-split-frame="buys"]')).toBeNull();
  expect(host.textContent).toContain('Traders1');
  expect(host.textContent).toContain('Txns2');
  const unavailable = mount(true, null).host;
  expect(unavailable.querySelector('[data-split-frame="buys"]')).toBeNull();
  expect(unavailable.querySelector('[role="img"][aria-label*="Buys"]')).toBeNull();
  expect(unavailable.textContent).toContain('Trades2');
}
test(dayWindowKeepsItsBuysAndSells.name, dayWindowKeepsItsBuysAndSells);

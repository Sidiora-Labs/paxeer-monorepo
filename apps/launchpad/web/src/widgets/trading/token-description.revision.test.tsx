// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, onlineManager } from '@tanstack/react-query';
import { decodeV2Contract } from '@kindle/sdk';
import { afterEach, expect, test } from 'vitest';
import wire from '@/core/test-fixtures/market-production/core-api-wire.json';
import type { PoolSnapshot } from '@/core/market/pool-resolver';
import { queryKeys } from '@/core/query-keys';
import TokenDescription from './token-description';

const detail = decodeV2Contract('PoolDetailResponse', wire.v2Reads.responses.poolDetailReal.body);
const pool = detail.data.pool.poolAddress;
const description = 'A coin for the community.';
const snapshot: PoolSnapshot = {
  pool: { chainId: '125', poolAddress: pool, tokenAddress: detail.data.token.address },
  token: detail.data.token,
  stats: detail.data.stats,
  metadata: {
    token: detail.data.token,
    description,
    displayName: detail.data.token.name,
    displaySymbol: detail.data.token.symbol,
    revision: '3',
    socialLinks: [],
    tags: ['community'],
  },
  creator: detail.data.pool.creator,
  createdAt: detail.data.pool.createdAt ?? null,
  generatedAt: detail.generatedAt,
  resolvedBy: 'detail',
  unavailable: { stats: null, metadata: null, registry: null },
};
let view: { root: Root; host: HTMLDivElement; client: QueryClient; wasOnline: boolean } | undefined;

afterEach(() => {
  if (!view) return;
  act(() => view!.root.unmount());
  view.client.clear();
  view.host.remove();
  onlineManager.setOnline(view.wasOnline);
  view = undefined;
});

function descriptionShowsNoRevisionLine() {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const wasOnline = onlineManager.isOnline();
  onlineManager.setOnline(false);
  const client = new QueryClient();
  client.setQueryData(queryKeys.poolSnapshot(pool), snapshot);
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  view = { root, host, client, wasOnline };
  act(() => root.render(<QueryClientProvider client={client}><TokenDescription poolAddress={pool} /></QueryClientProvider>));
  const section = host.querySelector('section[aria-label="Description"]');
  expect(section?.querySelector('p')?.textContent).toBe(description);
  expect(section?.textContent).toContain('community');
  expect(host.textContent).not.toMatch(/revision/i);
  expect(client.getQueryData<PoolSnapshot>(queryKeys.poolSnapshot(pool))?.metadata?.revision).toBe('3');
}
test(descriptionShowsNoRevisionLine.name, descriptionShowsNoRevisionLine);

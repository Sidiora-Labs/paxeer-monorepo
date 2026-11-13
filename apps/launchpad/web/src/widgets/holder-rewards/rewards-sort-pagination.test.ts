import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import React, { act } from 'react';
import type { Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createConfig, createStorage, http, WagmiProvider } from 'wagmi';
import { SdkClient, type V2ContractTypes } from '@kindle/sdk';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { MarketClientContext } from '@/core/market/client';
import type { TokenMetadata } from '@/widgets/home/types';
import type { HolderRewardPool, HolderRewardsPoolsResponse, HolderRewardsStatsResponse } from './holder-rewards-contract';
import { holderRewardsPageSize } from './holder-rewards-contract';

// Keep Node's real fetch while giving the production view a browser DOM.
type BrowserDom = { window: Window & typeof globalThis };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom;
};
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
const originalEventDescriptors = new Map<string, PropertyDescriptor | undefined>();
// Wallet discovery dispatches on this window, so its events must use the
// same DOM realm even when Node already exposes its own Event constructors.
for (const key of ['Event', 'CustomEvent'] as const) {
  originalEventDescriptors.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
  Object.defineProperty(globalThis, key, { configurable: true, writable: true, value: dom.window[key] });
}
const installedGlobals: string[] = [];
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
  installedGlobals.push(key);
}
globals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const HolderRewardsView = (await import('./holder-rewards-view')).default;

const requestId = '0f0e2a8e-5b3c-4d8a-9a1e-2c6d7b8f9a01';
const generatedAt = '2026-10-12T15:00:01.000Z';
const envelope = { requestId, generatedAt, nextCursor: null, quality: 'complete', revision: '7', watermark: null } as const;
const address = (id: number) => `0x${id.toString(16).padStart(40, '0')}`;

// A committed in-memory read model: a full first page, then two amounts
// below its entire range. Both routes and metadata read the same identities.
const population: HolderRewardPool[] = Array.from({ length: holderRewardsPageSize + 2 }, (_, index) => ({
  poolAddress: address(index + 1),
  tokenAddress: address(index + 1001),
  distributedRaw: String(BigInt(index < holderRewardsPageSize ? 100 - index : holderRewardsPageSize + 2 - index) * 1_000_000n),
  holdersPaid: 100 - index,
  strategyActive: index !== 0,
  createdAt: '2026-09-14T10:00:00.000Z',
}));
const metadata: Record<string, TokenMetadata> = Object.fromEntries(population.map((pool, index) => [pool.tokenAddress, {
  token_address: pool.tokenAddress, name: `Coin ${index + 1}`, symbol: `C${index + 1}`,
}]));
const summary: HolderRewardsStatsResponse = {
  ...envelope,
  data: { cumulativeDistributedRaw: population.reduce((sum, pool) => sum + BigInt(pool.distributedRaw), 0n).toString(), walletsRewarded: 100, generatedAt, watermark: null },
};
const dependencyFailure: V2ContractTypes['Error'] = {
  code: 'DEPENDENCY_UNAVAILABLE', message: 'Read model unavailable', requestId,
  retryable: false, retryAfterMs: null, fieldErrors: [], details: null,
};

let server: Server;
let origin: string;
let requests: URL[];
let unexpected: string[];
let failNextPage: boolean;
let releasePage: (() => void) | undefined;
let holdPage: Promise<void> | undefined;
let view: { root: Root; container: HTMLDivElement; queries: QueryClient } | undefined;
const originalMetadataOrigin = process.env.NEXT_PUBLIC_METADATA_API_URL;

beforeAll(async () => {
  server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    requests.push(url);
    const send = (status: number, body: unknown) => {
      response.writeHead(status, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(body));
    };
    request.resume();
    request.on('end', () => {
      if (request.method === 'GET' && url.pathname === '/v2/holder-rewards/stats') return send(200, summary);
      if (request.method === 'GET' && url.pathname === '/v2/holder-rewards/pools') {
        const limit = Number(url.searchParams.get('limit'));
        const cursor = url.searchParams.get('cursor');
        if (url.searchParams.get('sort') !== 'rewards' || ![5, holderRewardsPageSize].includes(limit) || (cursor !== null && cursor !== 'page-2')) {
          unexpected.push(url.pathname + url.search);
          return send(400, dependencyFailure);
        }
        if (cursor && failNextPage) {
          failNextPage = false;
          return send(503, dependencyFailure);
        }
        const offset = cursor ? holderRewardsPageSize : 0;
        const end = Math.min(offset + limit, population.length);
        const page: HolderRewardsPoolsResponse = {
          ...envelope,
          nextCursor: limit === holderRewardsPageSize && end < population.length ? 'page-2' : null,
          data: { items: population.slice(offset, end) },
        };
        if (cursor && holdPage) { void holdPage.then(() => send(200, page)); return; }
        return send(200, page);
      }
      if (request.method === 'GET' && url.pathname === '/metadata/batch') {
        const tokens = (url.searchParams.get('addresses') ?? '').split(',');
        if (tokens.every(token => Object.hasOwn(metadata, token))) return send(200, Object.fromEntries(tokens.map(token => [token, metadata[token]])));
      }
      if (request.method === 'POST' && url.pathname === '/v2/pools/stats:batch') return send(503, dependencyFailure);
      unexpected.push(`${request.method} ${url.pathname}`);
      send(404, dependencyFailure);
    });
  });
  origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
  process.env.NEXT_PUBLIC_METADATA_API_URL = origin;
});

beforeEach(() => {
  dom.window.localStorage.clear();
  requests = [];
  unexpected = [];
  failNextPage = false;
  holdPage = undefined;
  releasePage = undefined;
});

afterEach(async () => {
  releasePage?.();
  if (view) {
    const mounted = view;
    act(() => mounted.root.unmount());
    await mounted.queries.cancelQueries();
    mounted.queries.clear();
    mounted.container.remove();
    view = undefined;
  }
  expect(unexpected).toEqual([]);
});

afterAll(async () => {
  await new Promise<void>(resolve => server.close(() => resolve()));
  if (originalMetadataOrigin === undefined) delete process.env.NEXT_PUBLIC_METADATA_API_URL;
  else process.env.NEXT_PUBLIC_METADATA_API_URL = originalMetadataOrigin;
  dom.window.close();
  for (const key of installedGlobals) delete globals[key];
  for (const [key, descriptor] of originalEventDescriptors) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor);
    else delete globals[key];
  }
  delete globals.IS_REACT_ACT_ENVIRONMENT;
});

function mount() {
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: 'ws://127.0.0.1:1', approvedStreamOrigins: ['ws://127.0.0.1:1'], production: false });
  const chain = { id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [origin] } } } as const;
  const config = createConfig({ chains: [chain], transports: { 125: http(origin, { retryCount: 0 }) }, storage: createStorage({ storage: dom.window.localStorage }), ssr: true });
  const queries = new QueryClient();
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(React.createElement(WagmiProvider, { config },
    React.createElement(QueryClientProvider, { client: queries },
      React.createElement(MarketClientContext.Provider, { value: client }, React.createElement(HolderRewardsView))))));
  view = { root, container, queries };
  const section = () => {
    const node = container.querySelector<HTMLElement>('section[aria-labelledby="all-holder-rewards-title"]');
    if (!node) throw new Error('Holder rewards list is missing');
    return node;
  };
  return { section };
}

async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 8000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error('Holder rewards did not reach the expected state');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

function clickButton(section: HTMLElement, text: string) {
  const button = Array.from(section.querySelectorAll('button')).find(node => node.textContent === text || node.getAttribute('aria-label') === text);
  expect(button, `Button ${text}`).toBeDefined();
  act(() => button!.click());
  return button!;
}

function poolLinks(section: HTMLElement, presentation: 'table' | 'grid') {
  return Array.from(section.querySelectorAll<HTMLAnchorElement>(presentation === 'table' ? 'tbody a[href^="/coin/"]' : 'ol a[href^="/coin/"]')).map(node => node.getAttribute('href'));
}

function pageRequests() {
  return requests.filter(url => url.pathname === '/v2/holder-rewards/pools' && url.searchParams.get('limit') === String(holderRewardsPageSize));
}

async function reward_sort_scope() {
  const { section } = mount();
  await waitFor(() => poolLinks(section(), 'table').length === holderRewardsPageSize && section().textContent!.includes('Coin 1'));
  const scope = () => section().querySelector('#holder-rewards-scope')?.textContent;
  expect(scope()).toContain(`Showing ${holderRewardsPageSize} of ${holderRewardsPageSize} loaded coins.`);
  expect(scope()).toContain('Sorting, filters and CSV export apply only to loaded coins.');
  expect(scope()).toContain('More coins are available; loading them may change the order.');
  const rewardHeader = section().querySelector<HTMLButtonElement>('th[aria-sort="descending"] button');
  expect(rewardHeader).not.toBeNull();
  act(() => rewardHeader!.click());
  expect(section().querySelector('th[aria-sort="ascending"]')).not.toBeNull();
  expect(poolLinks(section(), 'table')[0]).toBe(`/coin/${population[holderRewardsPageSize - 1].poolAddress}`);
  expect(pageRequests()).toHaveLength(1);
  expect(scope()).toContain('apply only to loaded coins');

  clickButton(section(), 'Load more');
  await waitFor(() => poolLinks(section(), 'table').length === population.length);
  expect(poolLinks(section(), 'table')).toEqual([...population].reverse().map(pool => `/coin/${pool.poolAddress}`));
  expect(scope()).toContain(`Showing ${population.length} of ${population.length} loaded coins.`);
  expect(scope()).not.toContain('More coins are available');
  expect(pageRequests().map(url => url.searchParams.get('cursor'))).toEqual([null, 'page-2']);

  clickButton(section(), 'Grid view');
  expect(poolLinks(section(), 'grid')).toEqual([...population].reverse().map(pool => `/coin/${pool.poolAddress}`));
  expect(scope()).toContain('apply only to loaded coins');
  clickButton(section(), 'Filters');
  const activeOnly = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitemradio"], [role="menuitem"]')).find(node => node.textContent?.includes('Still paying holders'));
  expect(activeOnly).toBeDefined();
  act(() => activeOnly!.click());
  expect(scope()).toContain(`Showing ${population.length - 1} of ${population.length} loaded coins.`);
  expect(poolLinks(section(), 'grid')).toHaveLength(population.length - 1);
}

async function rewards_grid_pagination() {
  const { section } = mount();
  await waitFor(() => poolLinks(section(), 'table').length === holderRewardsPageSize);
  clickButton(section(), 'Grid view');
  const firstPage = population.slice(0, holderRewardsPageSize).map(pool => `/coin/${pool.poolAddress}`);
  expect(poolLinks(section(), 'grid')).toEqual(firstPage);
  failNextPage = true;
  clickButton(section(), 'Load more');
  await waitFor(() => section().querySelector('[role="alert"]') !== null);
  expect(section().querySelector('[role="alert"]')?.textContent).toBe("Couldn't load more, try again");
  expect(poolLinks(section(), 'grid')).toEqual(firstPage);
  expect(section().querySelector('table')).toBeNull();
  expect(pageRequests().map(url => url.searchParams.get('cursor'))).toEqual([null, 'page-2']);

  holdPage = new Promise<void>(resolve => { releasePage = resolve; });
  const retry = clickButton(section(), 'Try again');
  await waitFor(() => pageRequests().length === 3);
  expect(retry.disabled).toBe(true);
  expect(poolLinks(section(), 'grid')).toEqual(firstPage);
  act(() => retry.click());
  expect(pageRequests()).toHaveLength(3);
  releasePage!();
  await waitFor(() => poolLinks(section(), 'grid').length === population.length);
  expect(poolLinks(section(), 'grid')).toEqual(population.map(pool => `/coin/${pool.poolAddress}`));
  expect(section().querySelector('[role="alert"]')).toBeNull();
  expect(section().textContent).toContain("You're up to date");
  expect(Array.from(section().querySelectorAll('button')).some(node => node.textContent === 'Load more')).toBe(false);
  expect(pageRequests().map(url => url.searchParams.get('cursor'))).toEqual([null, 'page-2', 'page-2']);
  expect(section().querySelector('#holder-rewards-scope')?.textContent).toContain(`Showing ${population.length} of ${population.length} loaded coins.`);

  clickButton(section(), 'Table view');
  expect(poolLinks(section(), 'table')).toEqual(population.map(pool => `/coin/${pool.poolAddress}`));
  expect(pageRequests()).toHaveLength(3);
}

test(reward_sort_scope.name, reward_sort_scope, 20_000);
test(rewards_grid_pagination.name, rewards_grid_pagination);

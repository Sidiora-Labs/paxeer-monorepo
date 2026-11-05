// @vitest-environment jsdom

import { onlineManager, useQueryClient } from '@tanstack/react-query';
import BigNumber from 'bignumber.js';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import React from 'react';

import type { SocketMessage } from 'lib/socket/types';
import type { FeedMarketsResponse, MarketUpdate, MarketUpdatesResponse } from 'types/api/paxeerXFeed';
import type { TokenInfo } from 'types/api/token';

import { getResourceKey } from 'lib/api/useApiQuery';
import { SocketProvider } from 'lib/socket/context';
import { TOKEN_INFO_APPLICATION } from 'mocks/account/verifiedAddresses';
import * as addressMock from 'mocks/address/address';
import * as feedMock from 'mocks/paxeerX/feed';
import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { PAXEER_X_RECEIPTS_ITEM } from 'stubs/paxeerXLists';
import applyPriceUpdates from 'ui/paxeerX/feed/applyPriceUpdates';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { afterAll, afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { act, cleanup, fireEvent, waitFor, within } from 'vitest/lib';
import { createTestSocket } from 'vitest/utils/socketServer';

import Address from './Address';
import PaxeerXFeed from './PaxeerXFeed';
import PaxeerXFeedMarket, { FEED_HISTORY_WINDOW_LIMIT } from './PaxeerXFeedMarket';
import PaxeerXReceipt from './PaxeerXReceipt';
import { addressDetailFixtures, tokenDetailFixtures, tokenSupplyTranscript } from './repair-22-33-314.fixtures';
import Token from './Token';

// Execute the repaired layout, buffering, rejoin and archive regression cases in the declared gate.
import './Address.spec';
import './PaxeerXFeedLive.spec';
import './PaxeerXFeedMarketLive.spec';
import './PaxeerXReceipt.spec';

const peer = await vi.hoisted(async() => {
  const { createServer } = await import('node:http');
  const documents = new Map<string, unknown | (() => Promise<unknown>)>();
  const requests: Array<{ target: string; body: unknown }> = [];
  const unexpected: Array<string> = [];
  const server = createServer(async(request, response) => {
    const target = request.url ?? '';
    const url = new URL(target, 'http://127.0.0.1');
    const key = documents.has(target) ? target : url.pathname;
    if (request.method !== 'GET' || !documents.has(key)) {
      unexpected.push(`${ request.method } ${ target }`);
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ message: 'Unexpected fixture request', target }));
      return;
    }
    const document = documents.get(key);
    const body = typeof document === 'function' ? await document() : document;
    requests.push({ target, body });
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify(body));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback HTTP address');
  }
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_CONTRACT_INFO_API_HOST: `http://127.0.0.1:${ address.port }`,
    NEXT_PUBLIC_CLUSTERS_API_HOST: `http://127.0.0.1:${ address.port }`,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
  };
  return { server, documents, requests, unexpected };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

const marketPath = '/api/v2/paxeer-x/feed/markets';
const historyPath = `${ marketPath }/0/updates`;
const tokenPath = `/api/v2/tokens/${ tokenInfo.address_hash }`;
const tokenKey = getResourceKey('general:token', { pathParams: { hash: tokenInfo.address_hash } });
const historyKey = getResourceKey('general:paxeer_x_feed_market_updates', { pathParams: { id: '0' } });
const root = path.resolve(__dirname, '../../../..');
const archive = {
  ...PAXEER_X_RECEIPTS_ITEM,
  verification_status: 'checkpoint_finalised',
  payload_hash: feedMock.updateHash,
  transaction_hash: feedMock.olderUpdateHash,
  timestamp: '2023-05-22T18:00:36.000000Z',
};
const archivePath = `/api/v2/paxeer-x/archive/receipts/${ archive.id }`;
const wireSupplies: ReadonlyArray<Parameters<SocketMessage.TokenTotalSupply['handler']>[0]> = tokenSupplyTranscript;
let client: ReturnType<typeof useQueryClient>;

function CacheProbe() {
  client = useQueryClient();
  return null;
}

function openPage(page: React.ReactElement, url?: string) {
  return render(React.createElement(React.Fragment, null,
    React.createElement(CacheProbe),
    React.createElement(SocketProvider, { url, children: page }),
  ));
}

function hashes(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[data-update]')).map(row => row.getAttribute('data-update'));
}

function marketPrice(container: HTMLElement, id: number) {
  return container.querySelector(`[data-market="${ id }"] [data-price]`)?.getAttribute('data-price');
}

function readCount(target: string) {
  return peer.requests.filter(request => request.target === target).length;
}

async function addressFixtureShapeReachesLoadedDetailOverHttp() {
  routerState.pathname = '/address/[hash]';
  routerState.query = { hash: addressMock.hash };
  addressDetailFixtures.forEach((value, key) => peer.documents.set(key, value));
  const { container, rerender } = openPage(React.createElement(Address));
  await waitFor(() => {
    expect(container.querySelector('[data-section-group="ADDRESS INFO"]')).not.toBeNull();
    expect(container.querySelector(`a[href="/tx/${ addressMock.contract.creation_transaction_hash }"]`)).not.toBeNull();
  });
  expect(container.querySelectorAll('[data-address-card]')).toHaveLength(2);
  expect(container.querySelector('[data-address-balances]')).not.toBeNull();
  routerState.query = { hash: addressMock.hash, tab: 'analytics' };
  rerender(React.createElement(
    React.Fragment, null, React.createElement(CacheProbe), React.createElement(SocketProvider, { children: React.createElement(Address) }),
  ));
  await waitFor(() => expect(container.querySelector('[data-stat-tile="Transactions"]')?.textContent).toContain('5,462'));
  expect(peer.requests.find(request => request.target === `/api/v2/addresses/${ addressMock.hash }`)?.body).toEqual(addressMock.contract);
  expect(peer.requests.find(request => request.target.endsWith('/counters'))?.body)
    .toEqual(addressDetailFixtures.get(`/api/v2/addresses/${ addressMock.hash }/counters`));
}

async function feedOverviewRestoresAfterStopAndReopenWithoutEvents() {
  routerState.pathname = '/feed';
  peer.documents.set(marketPath, feedMock.markets);
  const { container, rerender } = openPage(React.createElement(PaxeerXFeed));
  await waitFor(() => expect(marketPrice(container, 0)).toBe(feedMock.btcIndexPrice));
  rerender(React.createElement(CacheProbe));
  // The committed submission is reflected in REST while the consumer is stopped. No event is replayed.
  const recovered = applyPriceUpdates(feedMock.refusalMarkets, [ feedMock.refusedPriceUpdate ]);
  peer.documents.set(marketPath, recovered);
  rerender(React.createElement(SocketProvider, { children: React.createElement(PaxeerXFeed) }));
  await waitFor(() => expect(marketPrice(container, 1)).toBe(feedMock.refusedPriceUpdate.price_update.observations[1].index_price));
  expect(readCount(marketPath)).toBe(2);
  const socket = await createTestSocket();
  try {
    rerender(React.createElement(SocketProvider, { url: socket.url, children: React.createElement(PaxeerXFeed) }));
    await socket.join('paxeer_x_feed:new_update');
    await waitFor(() => {
      expect(marketPrice(container, 1)).toBe(feedMock.refusedPriceUpdate.price_update.observations[1].index_price);
      expect(client.getQueryData<FeedMarketsResponse>(getResourceKey('general:paxeer_x_feed_markets'))).toEqual(recovered);
      expect(readCount(marketPath)).toBe(3);
    });
  } finally {
    cleanup();
    await socket.close();
  }
}

async function unknownAcceptedMarketReconcilesThroughRest() {
  routerState.pathname = '/feed';
  peer.documents.set(marketPath, feedMock.markets);
  const socket = await createTestSocket();
  try {
    const { container } = openPage(React.createElement(PaxeerXFeed), socket.url);
    await socket.join('paxeer_x_feed:new_update');
    await waitFor(() => {
      expect(marketPrice(container, 0)).toBe(feedMock.btcIndexPrice);
      expect(client.isFetching()).toBe(0);
    });
    expect(container.querySelector('[data-market="1"]')).toBeNull();
    const reads = readCount(marketPath);
    const recovered = applyPriceUpdates(feedMock.refusalMarkets, [ feedMock.priceUpdate ]);
    peer.documents.set(marketPath, recovered);
    socket.send('paxeer_x_feed:new_update', 'new_update', feedMock.priceUpdate);
    await waitFor(() => expect(marketPrice(container, 1)).toBe(feedMock.ethIndexPrice), { timeout: 10_000 });
    expect(readCount(marketPath)).toBe(reads + 1);
    expect(client.getQueryData<FeedMarketsResponse>(getResourceKey('general:paxeer_x_feed_markets'))).toEqual(recovered);
  } finally {
    cleanup();
    await socket.close();
  }
}

async function longSessionBoundsHistoryAndPreservesExclusiveCursorContinuity() {
  routerState.pathname = '/feed/market/[id]';
  routerState.query = { id: '0' };
  const history = [ ...feedMock.marketUpdates ];
  peer.documents.set(marketPath, feedMock.markets);
  const serveHistory = async() => {
    const request = pendingHistoryTarget;
    const cursor = new URL(request, 'http://127.0.0.1').searchParams;
    const block = cursor.get('block_number');
    const index = cursor.get('index');
    const remaining = history.filter(item => block === null || item.block_number < Number(block) ||
      (item.block_number === Number(block) && item.index < Number(index)));
    const items = remaining.slice(0, 50);
    const last = items[items.length - 1];
    return {
      items,
      next_page_params: remaining.length > items.length ? { block_number: last.block_number, index: last.index } : null,
    };
  };
  // Install exact cursor documents before each turn; the store retains all committed rows.
  let pendingHistoryTarget = historyPath;
  peer.documents.set(historyPath, serveHistory);
  const socket = await createTestSocket();
  try {
    const { container } = openPage(React.createElement(PaxeerXFeedMarket), socket.url);
    await socket.join('paxeer_x_feed:market:0');
    await waitFor(() => {
      expect(hashes(container)).toEqual(history.map(item => item.update_hash));
      expect(client.isFetching()).toBe(0);
    });
    for (let batch = 0; batch < 4; batch++) {
      const updates = Array.from({ length: batch === 3 ? 41 : 40 }, (_, offset): MarketUpdate => {
        const ordinal = batch * 40 + offset;
        return {
          ...feedMock.marketUpdates[0],
          update_hash: `0x${ (ordinal + 1).toString(16).padStart(64, '0') }`,
          block_number: feedMock.marketUpdates[0].block_number + 1 + Math.floor(ordinal / 2),
          index: ordinal % 2,
          sequence: 10000 + ordinal,
        };
      });
      updates.forEach(item => {
        history.unshift(item);
        socket.send('paxeer_x_feed:market:0', 'new_update', item);
      });
      socket.send('paxeer_x_feed:market:0', 'new_update', updates[updates.length - 1]);
      await waitFor(() => {
        expect(hashes(container)).toEqual(history.slice(0, FEED_HISTORY_WINDOW_LIMIT).map(item => item.update_hash));
      }, { timeout: 15_000 });
      const cached = client.getQueryData<MarketUpdatesResponse>(historyKey)!;
      expect(cached.items.length).toBeLessThanOrEqual(FEED_HISTORY_WINDOW_LIMIT);
      expect(new Set(hashes(container)).size).toBe(hashes(container).length);
    }
    expect(hashes(container)).toHaveLength(FEED_HISTORY_WINDOW_LIMIT);
    const reached = [ ...hashes(container) ];
    let cursor = client.getQueryData<MarketUpdatesResponse>(historyKey)!.next_page_params;
    expect(cursor).toEqual({ block_number: history[FEED_HISTORY_WINDOW_LIMIT - 1].block_number, index: history[FEED_HISTORY_WINDOW_LIMIT - 1].index });
    expect(cursor?.index).toBe(1);
    let offset = FEED_HISTORY_WINDOW_LIMIT;
    let nextPage = 2;
    while (cursor) {
      pendingHistoryTarget = `${ historyPath }?block_number=${ cursor.block_number }&index=${ cursor.index }`;
      peer.documents.set(pendingHistoryTarget, serveHistory);
      const expected = history.slice(offset, offset + 50).map(item => item.update_hash);
      routerState.query = { id: '0', page: String(nextPage++), next_page_params: encodeURIComponent(JSON.stringify(cursor)) };
      const footerPagination = container.querySelector<HTMLElement>('[data-footer-pagination]');
      expect(footerPagination).not.toBeNull();
      fireEvent.click(within(footerPagination!).getByRole('button', { name: 'Next page' }));
      await waitFor(() => expect(hashes(container)).toEqual(expected));
      expect(peer.requests.some(request => request.target === pendingHistoryTarget)).toBe(true);
      reached.push(...hashes(container));
      offset += expected.length;
      cursor = (peer.requests.findLast(request => request.target === pendingHistoryTarget)?.body as MarketUpdatesResponse).next_page_params;
    }
    expect(reached).toEqual(history.map(item => item.update_hash));
    expect(new Set(reached).size).toBe(history.length);
    expect(reached.slice(-3)).toEqual(feedMock.marketUpdates.map(item => item.update_hash));
    await waitFor(() => {
      const cachedPages = client.getQueriesData<MarketUpdatesResponse>({ queryKey: [ 'general:paxeer_x_feed_market_updates' ] });
      expect(cachedPages).toHaveLength(1);
      expect(cachedPages[0][1]?.items).toHaveLength(14);
    });
  } finally {
    cleanup();
    await socket.close();
  }
}

async function frozenArchiveRemainsStableWithoutPollingOrVisibilityRefresh() {
  routerState.pathname = '/paxeer-x/receipts/[id]';
  routerState.query = { id: archive.id };
  peer.documents.set(archivePath, archive);
  const { container } = openPage(React.createElement(PaxeerXReceipt));
  await waitFor(() => expect(container.querySelector('[data-receipt-freshness]')?.getAttribute('data-receipt-freshness')).toBe('complete'));
  expect(peer.requests.filter(request => request.target === archivePath).map(request => request.body)).toEqual([ archive ]);
  act(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' });
    document.dispatchEvent(new Event('visibilitychange'));
  });
  act(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
    document.dispatchEvent(new Event('visibilitychange'));
  });
  await new Promise(resolve => setTimeout(resolve, 5_500));
  expect(readCount(archivePath)).toBe(1);
  expect(container.querySelector('[data-verification-status]')?.getAttribute('data-verification-status')).toBe('checkpoint_finalised');
  expect(container.querySelector('[data-receipt-freshness]')?.textContent).toContain('Recorded archive verification status.');
  expect(container.querySelector('[data-receipt-freshness]')?.textContent).not.toMatch(/Checking|Anchored/);
}

async function invalidArchiveEvidenceIsRefusedOverHttp() {
  routerState.pathname = '/paxeer-x/receipts/[id]';
  routerState.query = { id: archive.id };
  for (const invalid of [ { ...archive, frozen: false }, { ...archive, id: 'wrong-id' }, { ...archive, verification_status: 'unsupported' } ]) {
    peer.documents.set(archivePath, invalid);
    const { container, unmount } = openPage(React.createElement(PaxeerXReceipt));
    await waitFor(() => expect(container.querySelector('[data-receipt-refresh-error]')).not.toBeNull());
    expect(container.querySelector('[data-receipt-details-card]')).toBeNull();
    expect(peer.requests.at(-1)?.body).toEqual(invalid);
    unmount();
  }
  expect(readCount(archivePath)).toBe(3);
}

async function tokenSupplyStringsCrossPhoenixLosslesslyIncludingBufferedZero() {
  routerState.pathname = '/token/[hash]';
  routerState.query = { hash: tokenInfo.address_hash, tab: 'metadata' };
  tokenDetailFixtures.forEach((value, key) => peer.documents.set(key, value));
  peer.documents.set(`/api/v1/chains/1/token-infos/${ tokenInfo.address_hash }`, TOKEN_INFO_APPLICATION.APPROVED);
  let release: () => void = () => undefined;
  const responseReady = new Promise<void>(resolve => {
    release = resolve;
  });
  peer.documents.set(tokenPath, async() => {
    await responseReady;
    return tokenInfo;
  });
  const socket = await createTestSocket();
  try {
    const { container } = openPage(React.createElement(Token), socket.url);
    await socket.join(`tokens:${ tokenInfo.address_hash }`);
    expect(client.getQueryData(tokenKey)).toBeUndefined();
    await act(async() => {
      socket.send(`tokens:${ tokenInfo.address_hash }`, 'total_supply', wireSupplies[1]);
      socket.send(`tokens:${ tokenInfo.address_hash }`, 'total_supply', wireSupplies[2]);
      await new Promise(resolve => setTimeout(resolve, 50));
    });
    release();
    await waitFor(() => expect(client.getQueryData<TokenInfo>(tokenKey)?.total_supply).toBe('0'));
    for (const payload of wireSupplies) {
      socket.send(`tokens:${ tokenInfo.address_hash }`, 'total_supply', payload);
      await waitFor(() => {
        expect(client.getQueryData<TokenInfo>(tokenKey)?.total_supply).toBe(payload.total_supply);
        const displayed = new BigNumber(payload.total_supply).shiftedBy(-Number(tokenInfo.decimals)).dp(3).toFormat();
        expect(container.querySelector('[data-token-card="TOKEN INFO"]')?.textContent).toContain(`${ displayed } ARIA`);
      });
    }
    const notifier = readFileSync(path.join(root, 'services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex'), 'utf8');
    expect(notifier).toContain('total_supply: to_string(total_supply)');
    const wireType = readFileSync(path.join(root, 'apps/explorer/lib/socket/types.ts'), 'utf8');
    expect(wireType).toContain('SocketMessageParamsGeneric<\'total_supply\', { total_supply: string }>');
    const visualCases = readFileSync(path.join(__dirname, 'Token.pw.tsx'), 'utf8');
    expect(visualCases).toContain('\'100000000000000000000\'');
    expect(visualCases).toContain('\'9007199254740993000000000000000001\'');
    expect(visualCases).toContain('\'0\'');
    expect(visualCases).not.toContain('total_supply: 10 ** 20');
  } finally {
    release();
    cleanup();
    await socket.close();
  }
}

describe('repair 22.33.314 real HTTP and Phoenix consumers', () => {
  beforeEach(() => {
    onlineManager.setOnline(true);
    fetchMock.disableMocks();
    peer.documents.clear();
    peer.requests.length = 0;
    peer.unexpected.length = 0;
    routerState.query = {};
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
  });
  afterEach(() => {
    cleanup();
    Reflect.deleteProperty(document, 'visibilityState');
    fetchMock.enableMocks();
    expect(peer.unexpected).toEqual([]);
  });
  test(addressFixtureShapeReachesLoadedDetailOverHttp.name, addressFixtureShapeReachesLoadedDetailOverHttp);
  test(feedOverviewRestoresAfterStopAndReopenWithoutEvents.name, feedOverviewRestoresAfterStopAndReopenWithoutEvents);
  test(unknownAcceptedMarketReconcilesThroughRest.name, unknownAcceptedMarketReconcilesThroughRest);
  test(longSessionBoundsHistoryAndPreservesExclusiveCursorContinuity.name, longSessionBoundsHistoryAndPreservesExclusiveCursorContinuity);
  test(frozenArchiveRemainsStableWithoutPollingOrVisibilityRefresh.name, frozenArchiveRemainsStableWithoutPollingOrVisibilityRefresh);
  test(invalidArchiveEvidenceIsRefusedOverHttp.name, invalidArchiveEvidenceIsRefusedOverHttp);
  test(tokenSupplyStringsCrossPhoenixLosslesslyIncludingBufferedZero.name, tokenSupplyStringsCrossPhoenixLosslesslyIncludingBufferedZero);
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => {
    peer.server.close(error => error ? reject(error) : resolve());
    peer.server.closeAllConnections();
  });
});

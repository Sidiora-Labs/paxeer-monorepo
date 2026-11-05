// @vitest-environment jsdom

import { onlineManager, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import BigNumber from 'bignumber.js';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import React from 'react';

import type { TokenInfo } from 'types/api/token';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { getResourceKey } from 'lib/api/useApiQuery';
import { MultichainContext } from 'lib/contexts/multichain';
import { SocketProvider } from 'lib/socket/context';
import { TOKEN_INFO_APPLICATION } from 'mocks/account/verifiedAddresses';
import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { nextRouterModule, render, routerState } from 'ui/shared/layout/testWrapper';
import { afterAll, afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { act, cleanup, waitFor } from 'vitest/lib';
import { createTestSocket } from 'vitest/utils/socketServer';

import { tokenDetailFixtures, tokenSupplyTranscript } from './repair-22-33-314.fixtures';
import Token from './Token';

const peer = await vi.hoisted(async function() {
  const { createServer } = await import('node:http');
  const documents = new Map<string, unknown | (() => Promise<unknown>)>();
  const requests: Array<string> = [];
  const unexpected: Array<string> = [];
  const server = createServer(async(request, response) => {
    const target = request.url ?? '';
    const pathname = new URL(target, 'http://127.0.0.1').pathname;
    const key = documents.has(target) ? target : pathname;
    requests.push(target);
    if (request.method !== 'GET' || !documents.has(key)) {
      unexpected.push(`${ request.method } ${ target }`);
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ message: 'Unexpected fixture request', target }));
      return;
    }
    const document = documents.get(key);
    const body = typeof document === 'function' ? await document() : document;
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify(body));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback HTTP address');
  }
  const endpoint = `http://127.0.0.1:${ address.port }`;
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_CONTRACT_INFO_API_HOST: endpoint,
    NEXT_PUBLIC_CLUSTERS_API_HOST: endpoint,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
  };
  return { server, endpoint, documents, requests, unexpected };
});

function chainFixture(id: string, slug: string): ClusterChainConfig {
  if (!config.apis.general) {
    throw new Error('Token fixtures require the general API configuration');
  }
  return {
    id,
    slug,
    name: slug,
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        general: { ...config.apis.general, endpoint: peer.endpoint, basePath: `/${ slug }` },
      },
    },
  };
}

const chainA = chainFixture('125', 'alpha');
const chainB = chainFixture('126', 'beta');
const tokenA: TokenInfo = { ...tokenInfo, total_supply: '11000000000000000000' };
const tokenB: TokenInfo = { ...tokenInfo, total_supply: '22000000000000000000' };
const tokenPath = `/api/v2/tokens/${ tokenInfo.address_hash }`;
const topic = `tokens:${ tokenInfo.address_hash.toLowerCase() }`;
const rootKey = tokenKey();
let client: QueryClient;
let view: ReturnType<typeof render>;
let sockets: Array<Awaited<ReturnType<typeof createTestSocket>>>;
let pendingResponse: ReturnType<typeof holdTokenResponse> | undefined;

function tokenKey(chain?: ClusterChainConfig) {
  return getResourceKey('general:token', { pathParams: { hash: tokenInfo.address_hash }, chainId: chain?.id });
}

function tokenPage(chain: ClusterChainConfig, url: string) {
  return React.createElement('section', { 'data-chain': chain.id },
    React.createElement(MultichainContext.Provider, { value: { chain } },
      React.createElement(SocketProvider, { url, children: React.createElement(Token) }),
    ),
  );
}

function queryTree(...pages: Array<React.ReactElement>) {
  return React.createElement(RouterContext.Provider, { value: nextRouterModule().useRouter() },
    React.createElement(QueryClientProvider, { client }, ...pages),
  );
}

function expectSupply(container: HTMLElement, chain: ClusterChainConfig, supply: string) {
  expect(client.getQueryData<TokenInfo>(tokenKey(chain))?.total_supply).toBe(supply);
  const displayed = new BigNumber(supply).shiftedBy(-Number(tokenInfo.decimals)).dp(3).toFormat();
  expect(container.querySelector(`[data-chain="${ chain.id }"] [data-token-card="Overview"]`)?.textContent)
    .toContain(`${ displayed } ${ tokenInfo.symbol }`);
}

function holdTokenResponse(chain: ClusterChainConfig, token: TokenInfo) {
  let release: () => void = () => undefined;
  const ready = new Promise<void>(resolve => {
    release = resolve;
  });
  const path = `/${ chain.slug }${ tokenPath }`;
  peer.documents.set(path, async() => {
    await ready;
    return token;
  });
  return { release, path };
}

async function tokenChainCacheKeyLiveUpdatesIsolateTwoChainsWithTheSameHash(payload: typeof tokenSupplyTranscript[number]) {
  const { container } = view;
  await act(async() => {
    sockets[0].send(topic, 'total_supply', tokenSupplyTranscript[0]);
    await waitFor(() => expect(client.getQueryData<TokenInfo>(tokenKey(chainA))?.total_supply)
      .toBe(tokenSupplyTranscript[0].total_supply));
  });
  await waitFor(() => expectSupply(container, chainA, tokenSupplyTranscript[0].total_supply));
  expect(client.getQueryData(tokenKey(chainB))).toEqual(tokenB);
  expect(client.getQueryData(rootKey)).toEqual(tokenInfo);
  await act(async() => {
    sockets[1].send(topic, 'total_supply', payload);
    await waitFor(() => expect(client.getQueryData<TokenInfo>(tokenKey(chainB))?.total_supply).toBe(payload.total_supply));
  });
  await waitFor(() => expectSupply(container, chainB, payload.total_supply));
  expect(client.getQueryData(tokenKey(chainA))).toEqual({ ...tokenA, total_supply: tokenSupplyTranscript[0].total_supply });
  expect(client.getQueryData(rootKey)).toEqual(tokenInfo);
  expect(peer.requests).toContain(`/${ chainA.slug }${ tokenPath }`);
  expect(peer.requests).toContain(`/${ chainB.slug }${ tokenPath }`);
}

async function tokenChainCacheKeyBuffersLatestSupplyUntilItsOwnHttpResponse() {
  const { container } = view;
  expect(client.getQueryData(tokenKey(chainA))).toBeUndefined();
  await act(async() => {
    sockets[0].send(topic, 'total_supply', tokenSupplyTranscript[1]);
    sockets[0].send(topic, 'total_supply', tokenSupplyTranscript[2]);
    await new Promise(resolve => setTimeout(resolve, 50));
  });
  expect(client.getQueryData(tokenKey(chainA))).toBeUndefined();
  expect(client.getQueryData(rootKey)).toBeUndefined();
  pendingResponse!.release();
  await waitFor(() => expectSupply(container, chainA, '0'));
  expect(client.getQueryData(tokenKey(chainB))).toEqual(tokenB);
  expectSupply(container, chainB, tokenB.total_supply!);
  expect(client.getQueryData(rootKey)).toBeUndefined();
}

async function tokenChainCacheKeyDiscardsPendingSupplyWhenTheActiveChainChanges() {
  const { container, rerender } = view;
  await act(async() => {
    sockets[0].send(topic, 'total_supply', tokenSupplyTranscript[1]);
    await new Promise(resolve => setTimeout(resolve, 50));
  });
  expect(client.getQueryData(tokenKey(chainA))).toBeUndefined();
  rerender(queryTree(tokenPage(chainB, sockets[0].url)));
  await waitFor(() => {
    expectSupply(container, chainB, tokenB.total_supply!);
    expect(client.isFetching()).toBe(0);
  });
  expect(client.getQueryData(tokenKey(chainB))).toEqual(tokenB);
  expect(client.getQueryData(rootKey)).toBeUndefined();
}

describe('repair 22.33.315 token-chain-cache-key', () => {
  beforeEach(async({ task }) => {
    onlineManager.setOnline(true);
    fetchMock.disableMocks();
    client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
    peer.documents.clear();
    peer.requests.length = 0;
    peer.unexpected.length = 0;
    sockets = [];
    pendingResponse = undefined;
    routerState.pathname = '/chain/[chain_slug]/token/[hash]';
    routerState.query = { hash: tokenInfo.address_hash, tab: 'token_transfers', chain_slug: chainA.slug };
    for (const chain of [ chainA, chainB ]) {
      tokenDetailFixtures.forEach((value, path) => peer.documents.set(`/${ chain.slug }${ path }`, value));
      peer.documents.set(`/${ chain.slug }${ tokenPath }`, chain === chainA ? tokenA : tokenB);
      client.setQueryData(getResourceKey('general:token_transfers', {
        pathParams: { hash: tokenInfo.address_hash },
        chainId: chain.id,
      }), tokenDetailFixtures.get(`${ tokenPath }/transfers`));
    }
    peer.documents.set(`/api/v1/chains/${ config.chain.id }/token-infos/${ tokenInfo.address_hash }`, TOKEN_INFO_APPLICATION.APPROVED);
    const liveCase = task.name === tokenChainCacheKeyLiveUpdatesIsolateTwoChainsWithTheSameHash.name;
    const navigationCase = task.name === tokenChainCacheKeyDiscardsPendingSupplyWhenTheActiveChainChanges.name;
    if (liveCase) {
      client.setQueryData(rootKey, tokenInfo);
    } else {
      pendingResponse = holdTokenResponse(chainA, tokenA);
    }
    sockets.push(await createTestSocket());
    if (!navigationCase) {
      sockets.push(await createTestSocket());
    }
    const pages = [ tokenPage(chainA, sockets[0].url) ];
    if (!navigationCase) {
      pages.push(tokenPage(chainB, sockets[1].url));
    }
    view = render(queryTree(...pages));
    await Promise.all(sockets.map(socket => socket.join(topic)));
    await waitFor(() => {
      if (liveCase) {
        expectSupply(view.container, chainA, tokenA.total_supply!);
        expect(client.isFetching()).toBe(0);
      } else {
        expect(peer.requests).toContain(pendingResponse!.path);
      }
      if (!navigationCase) {
        expectSupply(view.container, chainB, tokenB.total_supply!);
      }
    });
  });
  afterEach(async() => {
    cleanup();
    await client.cancelQueries();
    pendingResponse?.release();
    await Promise.all(sockets.map(socket => socket.close()));
    client.clear();
    expect(peer.unexpected).toEqual([]);
  });
  test.each(tokenSupplyTranscript)(
    tokenChainCacheKeyLiveUpdatesIsolateTwoChainsWithTheSameHash.name,
    tokenChainCacheKeyLiveUpdatesIsolateTwoChainsWithTheSameHash,
  );
  test(tokenChainCacheKeyBuffersLatestSupplyUntilItsOwnHttpResponse.name, tokenChainCacheKeyBuffersLatestSupplyUntilItsOwnHttpResponse);
  test(tokenChainCacheKeyDiscardsPendingSupplyWhenTheActiveChainChanges.name, tokenChainCacheKeyDiscardsPendingSupplyWhenTheActiveChainChanges);
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => {
    peer.server.close(error => error ? reject(error) : resolve());
    peer.server.closeAllConnections();
  });
  fetchMock.enableMocks();
});

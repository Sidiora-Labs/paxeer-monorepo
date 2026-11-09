// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import NextApp from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { NextRouter } from 'next/dist/shared/lib/router/router';
import Router from 'next/dist/shared/lib/router/router';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import { afterAll, afterEach, expect, test } from 'vitest';

import type TokensPage from './Tokens';

let Tokens: typeof TokensPage;
let Wrapper: React.ComponentType<{ children: React.ReactNode }>;
let endpoint: string;
let router: Router;
let client: QueryClient;
const requests: Array<URL> = [];
const originalEnvs = window.__envs;
const server = createServer((request, response) => {
  const url = new URL(request.url || '/', endpoint);
  requests.push(url);
  if (![ '/api/v2/tokens', '/api/v2/tokens/bridged' ].includes(url.pathname)) {
    response.writeHead(404);
    response.end();
    return;
  }
  response.writeHead(200, { 'Content-Type': 'application/json' });
  response.end(JSON.stringify({ items: [], next_page_params: null }));
});

async function prepareTokenHistoryPage() {
  fetchMock.disableMocks();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const port = String((server.address() as AddressInfo).port);
  endpoint = `http://127.0.0.1:${ port }`;
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: port,
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_BASE_PATH: '',
    NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES: JSON.stringify([ { id: 'ERC-7984', name: 'ERC-7984' } ]),
    NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS: JSON.stringify([
      { id: '1', title: 'Ethereum', short_title: 'ETH', base_url: 'https://ethereum.org' },
    ]),
    NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES: JSON.stringify([ { type: 'omni', title: 'Omni', short_title: 'Omni' } ]),
  };
  Object.assign(window, {
    __NEXT_DATA__: { props: { pageProps: {} }, page: '/tokens', query: {}, buildId: 'tokens-history' },
    __BUILD_MANIFEST: { sortedPages: [ '/tokens' ], __rewrites: { beforeFiles: [], afterFiles: [], fallback: [] }, '/tokens': [] },
    __DEV_PAGES_MANIFEST: { pages: [ '/tokens' ] },
    __DEV_MIDDLEWARE_MATCHERS: [],
    __MIDDLEWARE_MATCHERS: [],
    __SSG_MANIFEST: new Set(),
  });
  ({ Wrapper } = await import('ui/shared/layout/testWrapper'));
  ({ 'default': Tokens } = await import('./Tokens'));
}

await prepareTokenHistoryPage();

afterEach(() => {
  cleanup();
  client?.clear();
  if (router) {
    window.removeEventListener('popstate', router.onPopState);
  }
});

afterAll(async() => {
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});

async function tokenCategoryHistoryRestoresFilters() {
  const nft = 'ERC-721,ERC-1155,ERC-404';
  const fungible = 'ERC-20,ERC-7984';
  const initialType = `${ nft },invalid`;
  const query = { q: 'history search', sort: 'holders_count', order: 'asc', campaign: 'keep me', type: initialType };
  const initialPath = `/tokens?${ new URLSearchParams(query) }`;
  window.history.replaceState(null, '', initialPath);
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false } } });
  let publish: (() => void) | undefined;
  router = new Router('/tokens', query, initialPath, {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('tokens-history', ''),
    App: ({ Component, router }) => React.createElement(NextApp, { Component, router, pageProps: {} }),
    Component: Tokens,
    wrapApp: App => App,
    isFallback: false,
    subscription: async() => {
      publish?.();
    },
  });
  await router._initialMatchesMiddlewarePromise;
  router.changeState('replaceState', initialPath, initialPath, { shallow: true });
  const initialHistoryLength = window.history.length;
  function getRouterSnapshot(): NextRouter {
    return {
      route: router.route,
      pathname: router.pathname,
      query: router.query,
      asPath: router.asPath,
      basePath: router.basePath,
      isLocaleDomain: router.isLocaleDomain,
      isFallback: router.isFallback,
      isReady: router.isReady,
      isPreview: router.isPreview,
      events: router.events,
      push: router.push.bind(router),
      replace: router.replace.bind(router),
      reload: router.reload.bind(router),
      back: router.back.bind(router),
      forward: router.forward.bind(router),
      prefetch: router.prefetch.bind(router),
      beforePopState: router.beforePopState.bind(router),
    };
  }
  function Providers({ children }: { children: React.ReactNode }) {
    const [ snapshot, setSnapshot ] = React.useState(getRouterSnapshot);
    React.useEffect(() => {
      publish = () => setSnapshot(getRouterSnapshot());
      return () => {
        publish = undefined;
      };
    }, []);
    return (
      <Wrapper>
        <QueryClientProvider client={ client }>
          <RouterContext.Provider value={ snapshot }>{ children }</RouterContext.Provider>
        </QueryClientProvider>
      </Wrapper>
    );
  }
  render(<Tokens/>, { wrapper: Providers });

  async function assertSelection(
    label: string,
    type: string | null,
    count: number,
    historyLength: number,
    { urlType = type, chain = null, page = null }: { urlType?: string | null; chain?: string | null; page?: string | null } = {},
  ) {
    await waitFor(() => {
      expect(requests).toHaveLength(count);
      expect(client.isFetching()).toBe(0);
      expect(screen.getByText('No results')).toBeDefined();
    }, { timeout: 10_000 });
    await act(async() => {
      await new Promise(resolve => setTimeout(resolve, 150));
    });
    const url = new URL(window.location.href);
    const request = requests.at(-1)!;
    const group = within(screen.getByRole('group', { name: 'Token category' }));
    for (const name of [ 'All', 'Fungible', 'NFT', 'Bridged' ]) {
      expect(group.getByRole('button', { name }).getAttribute('aria-pressed')).toBe(String(name === label));
    }
    expect(url.searchParams.get('type')).toBe(urlType);
    expect(router.query.type ?? null).toBe(urlType);
    expect(request.searchParams.get('type')).toBe(type);
    expect(request.pathname).toBe(label === 'Bridged' ? '/api/v2/tokens/bridged' : '/api/v2/tokens');
    expect(request.searchParams.get('chain_ids')).toBe(chain);
    expect(url.searchParams.get('page')).toBe(page);
    expect(request.searchParams.get('q')).toBe(query.q);
    expect(request.searchParams.get('sort')).toBe(query.sort);
    expect(request.searchParams.get('order')).toBe(query.order);
    for (const key of [ 'q', 'sort', 'order', 'campaign' ] as const) {
      expect(url.searchParams.get(key)).toBe(query[key]);
    }
    expect((screen.getByPlaceholderText('Token name or symbol') as HTMLInputElement).value).toBe(query.q);
    expect(requests).toHaveLength(count);
    expect(window.history.length).toBe(historyLength);
  }
  async function click(label: string) {
    await act(async() => {
      fireEvent.click(within(screen.getByRole('group', { name: 'Token category' })).getByRole('button', { name: label }));
    });
  }
  async function traverse(direction: 'back' | 'forward', expectedPath: string) {
    await act(async() => window.history[direction]());
    await waitFor(() => expect(router.asPath).toBe(expectedPath));
  }

  await assertSelection('NFT', nft, 1, initialHistoryLength, { urlType: initialType });
  await click('Fungible');
  const fungiblePath = router.asPath;
  await assertSelection('Fungible', fungible, 2, initialHistoryLength + 1);
  await click('NFT');
  const nftPath = router.asPath;
  await assertSelection('NFT', nft, 3, initialHistoryLength + 2);
  await traverse('back', fungiblePath);
  await assertSelection('Fungible', fungible, 4, initialHistoryLength + 2);
  await traverse('forward', nftPath);
  await assertSelection('NFT', nft, 5, initialHistoryLength + 2);
  await click('All');
  await assertSelection('All', null, 6, initialHistoryLength + 3);
  await act(async() => {
    await router.push({ pathname: '/tokens', query: {
      ...router.query, tab: 'bridged', chain_ids: '1,unknown', page: '2',
      next_page_params: encodeURIComponent(JSON.stringify({ items_count: 50, marker: 'bridge-page' })),
    } }, undefined, { shallow: true });
  });
  const bridgedPath = router.asPath;
  await assertSelection('Bridged', null, 7, initialHistoryLength + 4, { chain: '1', page: '2' });
  expect(requests.at(-1)?.searchParams.get('marker')).toBe('bridge-page');
  await click('All');
  const allPath = router.asPath;
  await assertSelection('All', null, 8, initialHistoryLength + 5);
  expect(router.query.tab).toBe('all');
  expect(router.query.chain_ids).toBeUndefined();
  expect(router.query.next_page_params).toBeUndefined();
  expect(requests.at(-1)?.searchParams.get('marker')).toBeNull();
  await traverse('back', bridgedPath);
  await assertSelection('Bridged', null, 9, initialHistoryLength + 5, { chain: '1', page: '2' });
  expect(router.query.chain_ids).toBe('1,unknown');
  expect(requests.at(-1)?.searchParams.get('marker')).toBe('bridge-page');
  await traverse('forward', allPath);
  await assertSelection('All', null, 10, initialHistoryLength + 5);
  await click('Fungible');
  await assertSelection('Fungible', fungible, 11, initialHistoryLength + 6);
  await click('Bridged');
  await assertSelection('Bridged', null, 12, initialHistoryLength + 7);
  expect(router.query.type).toBeUndefined();
  expect(router.query.chain_ids).toBeUndefined();
}

test('tokenCategoryHistoryRestoresFilters', tokenCategoryHistoryRestoresFilters, 60_000);

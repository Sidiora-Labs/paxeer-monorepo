// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import NextApp from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { NextRouter } from 'next/dist/shared/lib/router/router';
import Router from 'next/dist/shared/lib/router/router';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { ClusterChainConfig } from 'types/multichain';

import { BLOCK } from 'stubs/block';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import type useQueryWithPagesType from './useQueryWithPages';

let useQueryWithPages: typeof useQueryWithPagesType;
let endpoint: string;
let chains: Array<ClusterChainConfig>;
const requests: Array<URL> = [];
const clients: Array<QueryClient> = [];
const routers: Array<Router> = [];
const originalEnvs = window.__envs;
const originalMultichainConfig = window.__multichainConfig;

const server = createServer((request, response) => {
  const url = new URL(request.url || '/', endpoint);
  requests.push(url);
  const chain = url.pathname.split('/')[1];
  const start = chain === 'a' ? 100 : 200;
  const cursor = url.searchParams.get('block_number');
  const page = cursor ? Number(cursor) - start : 1;
  if (!/^\/(a|b)\/api\/v2\/blocks$/.test(url.pathname) || !Number.isInteger(page) || page < 1 || page > 10) {
    response.writeHead(400, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ error: 'Cursor does not belong to this chain' }));
    return;
  }
  response.writeHead(200, { 'Content-Type': 'application/json' });
  response.end(JSON.stringify({
    items: [ { ...BLOCK, height: start + page } ],
    next_page_params: { block_number: start + page + 1, items_count: 50 },
  }));
});

beforeAll(async() => {
  fetchMock.disableMocks();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  endpoint = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_MULTICHAIN_ENABLED: 'true',
    NEXT_PUBLIC_MULTICHAIN_CLUSTER: 'pagination',
    NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST: endpoint,
    NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST: endpoint,
  };
  const { 'default': config } = await import('configs/app');
  chains = [ 'a', 'b' ].map((id) => ({
    id,
    slug: id,
    name: id,
    app_config: {
      ...config,
      apis: { ...config.apis, general: { ...config.apis.general!, endpoint, basePath: `/${ id }` } },
    },
  }));
  window.__multichainConfig = { chains };
  Object.assign(window, {
    __NEXT_DATA__: { props: { pageProps: {} }, page: '/blocks', query: {}, buildId: 'pagination' },
    __BUILD_MANIFEST: { sortedPages: [ '/blocks' ], __rewrites: { beforeFiles: [], afterFiles: [], fallback: [] }, '/blocks': [] },
    __DEV_PAGES_MANIFEST: { pages: [ '/blocks' ] },
    __DEV_MIDDLEWARE_MATCHERS: [],
    __MIDDLEWARE_MATCHERS: [],
    __SSG_MANIFEST: new Set(),
  });
  ({ 'default': useQueryWithPages } = await import('./useQueryWithPages'));
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  routers.splice(0).forEach(router => window.removeEventListener('popstate', router.onPopState));
  requests.length = 0;
});

afterAll(async() => {
  window.__envs = originalEnvs;
  window.__multichainConfig = originalMultichainConfig;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});

async function openPagination(search: string) {
  window.history.replaceState(null, '', `/blocks?${ search }`);
  const query: Record<string, string | Array<string>> = {};
  const searchParams = new URLSearchParams(search);
  searchParams.forEach((value, key) => {
    const values = searchParams.getAll(key);
    query[key] = values.length === 1 ? value : values;
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  clients.push(client);
  let publish: (() => void) | undefined;
  const router = new Router('/blocks', query, window.location.pathname + window.location.search, {
    initialProps: { pageProps: {} },
    pageLoader: new PageLoader('pagination', ''),
    App: ({ Component, router }) => React.createElement(NextApp, { Component, router, pageProps: {} }),
    Component: React.Fragment,
    wrapApp: App => App,
    isFallback: false,
    subscription: async() => {
      publish?.();
    },
  });
  routers.push(router);
  await router._initialMatchesMiddlewarePromise;
  // All entries in this fixture use the same loaded page, just like the hook's shallow pushes.
  router.changeState('replaceState', window.location.pathname + window.location.search, window.location.pathname + window.location.search, {
    shallow: true,
  });
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
  function PaginationProviders({ children }: { children: React.ReactNode }) {
    const [ snapshot, setSnapshot ] = React.useState(getRouterSnapshot);
    React.useEffect(() => {
      publish = () => setSnapshot(getRouterSnapshot());
      return () => {
        publish = undefined;
      };
    }, []);
    return React.createElement(QueryClientProvider, { client },
      React.createElement(RouterContext.Provider, { value: snapshot }, children));
  }
  const hook = renderHook(() => useQueryWithPages({
    resourceName: 'general:blocks', isMultichain: true, noScroll: true,
  }), { wrapper: PaginationProviders });
  return { ...hook, router, client };
}

async function paginationInvalidPage() {
  const invalidPages = [
    'page=abc', 'page=0', 'page=-2', 'page=1.5', 'page=Infinity', 'page=NaN',
    'page=1e309', 'page=', 'page=9007199254740992', 'page=2&page=3',
  ];
  for (const invalidPage of invalidPages) {
    const cursor = encodeURIComponent(JSON.stringify({ block_number: 109 }));
    const { result, router, unmount, client } = await openPagination(`chain_id=a&${ invalidPage }&next_page_params=${ cursor }`);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.pagination.page).toBe(1);
    expect(result.current.pagination.hasPages).toBe(false);
    expect(result.current.pagination.canGoBackwards).toBe(false);
    expect(result.current.data?.items[0].height).toBe(101);
    expect(requests.at(-1)?.search).toBe('');
    const asPath = router.asPath;
    await act(async() => result.current.pagination.onPrevPageClick());
    expect(router.asPath).toBe(asPath);
    await act(async() => result.current.pagination.onNextPageClick());
    await waitFor(() => expect(result.current.data?.items[0].height).toBe(102));
    expect(result.current.pagination.page).toBe(2);
    expect(router.query.page).toBe('2');
    expect(requests.at(-1)?.searchParams.get('block_number')).toBe('102');
    await act(async() => result.current.pagination.onPrevPageClick());
    await waitFor(() => expect(result.current.data?.items[0].height).toBe(101));
    expect(result.current.pagination.page).toBe(1);
    expect(router.query.page).toBeUndefined();
    unmount();
    client.clear();
    window.removeEventListener('popstate', router.onPopState);
  }
}

async function paginationInvalidPageHistory() {
  const { result, router } = await openPagination('chain_id=a');
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(101));
  await act(async() => {
    await router.push('/blocks?chain_id=a&page=-3&next_page_params=' + encodeURIComponent(JSON.stringify({ block_number: 109 })),
      undefined, { shallow: true });
  });
  await waitFor(() => expect(result.current.pagination.page).toBe(1));
  expect(result.current.data?.items[0].height).toBe(101);
  await act(async() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(102));
  expect(router.query.page).toBe('2');
}

async function paginationChainHistory() {
  const { result, router } = await openPagination('chain_id=a');
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(101));
  await act(async() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(102));
  await act(async() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(103));
  await act(async() => result.current.onChainValueChange({ value: [ 'b' ] }));
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(201));
  expect(result.current.chainValue).toEqual([ 'b' ]);
  expect(result.current.pagination.page).toBe(1);
  await act(async() => router.back());
  await waitFor(() => expect(router.query.chain_id).toBe('a'));
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(103));
  expect(result.current.chainValue).toEqual([ 'a' ]);
  expect(result.current.pagination.page).toBe(3);
  expect(result.current.pagination.canGoBackwards).toBe(false);
  await act(async() => router.forward());
  await waitFor(() => expect(router.query.chain_id).toBe('b'));
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(201));
  expect(result.current.chainValue).toEqual([ 'b' ]);
  expect(result.current.pagination.page).toBe(1);
  await act(async() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(202));
  expect(result.current.pagination.canGoBackwards).toBe(true);
  expect(requests.filter(url => url.pathname.startsWith('/b/')).every(url => {
    const cursor = url.searchParams.get('block_number');
    return cursor === null || Number(cursor) >= 202;
  })).toBe(true);
}

async function paginationValidatedChainHistory() {
  const { result, router } = await openPagination('chain_id=b');
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(201));
  await act(async() => {
    await router.push('/blocks?chain_id=unknown', undefined, { shallow: true });
  });
  await waitFor(() => expect(result.current.chainValue).toEqual([ 'a' ]));
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(101));
  await act(async() => router.back());
  await waitFor(() => expect(router.query.chain_id).toBe('b'));
  await waitFor(() => expect(result.current.data?.items[0].height).toBe(201));
  expect(result.current.chainValue).toEqual([ 'b' ]);
}

test('paginationInvalidPage', paginationInvalidPage);
test('paginationInvalidPageHistory', paginationInvalidPageHistory);
test('paginationChainHistory', paginationChainHistory);
test('paginationValidatedChainHistory', paginationValidatedChainHistory);

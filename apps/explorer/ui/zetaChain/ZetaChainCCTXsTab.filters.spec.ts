// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { makePublicRouterInstance } from 'next/dist/client/router';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import type { NextRouter } from 'next/dist/shared/lib/router/router';
import { once } from 'node:events';
import { createServer } from 'node:http';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { CctxStatus, CctxStatusReduced, type ListCctxsResponse } from '@blockscout/zetachain-cctx-types';

import { zetaChainCCTXItem } from 'mocks/zetaChain/zetaChainCCTX';
import type { RoutedTabsProps } from 'toolkit/components/RoutedTabs/RoutedTabs';
import type RoutedTabsComponent from 'toolkit/components/RoutedTabs/RoutedTabs';
import type { Wrapper as LayoutWrapper } from 'ui/shared/layout/testWrapper';
import { afterAll, beforeAll, expect, test } from 'vitest';

import type ZetaChainCCTxs from './cctxs/ZetaChainCCTxs';
import type ZetaChainFilterTags from './filters/ZetaChainFilterTags';
import type CctxTabComponent from './ZetaChainCCTXsTab';

// Next exports this bootstrap function at runtime but omits it from its declarations.
declare module 'next/dist/client/router' {
  export function makePublicRouterInstance(router: Router): NextRouter;
}

type ListProps = React.ComponentProps<typeof ZetaChainCCTxs>;
type FilterTagsProps = React.ComponentProps<typeof ZetaChainFilterTags>;

let ZetaChainCCTXsTab: typeof CctxTabComponent;
let RoutedTabs: typeof RoutedTabsComponent;
let Wrapper: typeof LayoutWrapper;
const initialEnvs = window.__envs;
const requests: Array<URL> = [];
const sender = '0x1234567890123456789012345678901234567890';

function transcriptRows(pending: boolean) {
  const filteredRow = {
    ...zetaChainCCTXItem,
    sender_address: sender,
    status: pending ? CctxStatus.PENDING_OUTBOUND : CctxStatus.OUTBOUND_MINED,
    status_reduced: pending ? CctxStatusReduced.PENDING : CctxStatusReduced.SUCCESS,
  };
  return {
    filteredRow,
    laterRow: { ...filteredRow, index: '0x' + '2'.repeat(64) },
    firstRow: { ...filteredRow, index: '0x' + '3'.repeat(64) },
  };
}

const server = createServer((request, response) => {
  const url = new URL(request.url || '/', 'http://localhost');
  requests.push(url);
  if (url.pathname !== '/api/v1/CctxInfo:list') {
    response.writeHead(404).end();
    return;
  }
  const rows = transcriptRows(url.searchParams.get('status_reduced') === 'Pending');
  let row = rows.firstRow;
  if (url.searchParams.has('page_key')) {
    row = rows.laterRow;
  } else if (url.searchParams.has('sender_address')) {
    row = rows.filteredRow;
  }
  const payload: ListCctxsResponse = { items: [ row ], next_page_params: undefined };
  response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(payload));
});

beforeAll(async() => {
  fetchMock.disableMocks();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('The CCTX transcript server did not bind a port');
  }
  window.__envs = {
    ...initialEnvs,
    NEXT_PUBLIC_ZETACHAIN_SERVICE_API_HOST: `http://127.0.0.1:${ address.port }`,
  };
  const [ component, tabs, layout ] = await Promise.all([
    import('./ZetaChainCCTXsTab'),
    import('toolkit/components/RoutedTabs/RoutedTabs'),
    import('ui/shared/layout/testWrapper'),
  ]);
  ZetaChainCCTXsTab = component.default;
  RoutedTabs = tabs.default;
  Wrapper = layout.Wrapper;
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
  window.__envs = initialEnvs;
});

function tabChildren(tree: React.ReactElement) {
  return React.Children.toArray((tree as React.ReactElement<{ children: React.ReactNode }>).props.children);
}

function listProps(tree: React.ReactElement, tab: string): ListProps {
  const tabs = tabChildren(tree).find((child) => React.isValidElement<RoutedTabsProps>(child) && child.type === RoutedTabs);
  if (!React.isValidElement<RoutedTabsProps>(tabs)) {
    throw new Error('The production component did not render RoutedTabs');
  }
  const component = tabs.props.tabs.find((item) => item.id === (tab === 'cctx' ? 'cctx_mined' : tab))?.component;
  if (!React.isValidElement<ListProps>(component)) {
    throw new Error('The requested production CCTX list is missing');
  }
  return component.props;
}

function RouteFixture() {
  return React.createElement('main', null, 'Cross chain transactions');
}

async function filterRequestCase(tab: 'cctx' | 'cctx_mined' | 'cctx_pending') {
  requests.length = 0;
  const pending = tab === 'cctx_pending';
  const statuses = pending ? 'Pending' : 'Success,Failed';
  const { filteredRow, laterRow, firstRow } = transcriptRows(pending);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  const initialNextData = window.__NEXT_DATA__;
  const middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  const initialBuildManifest = window.__BUILD_MANIFEST;
  const initialPagesManifest = window.__DEV_PAGES_MANIFEST;
  let unmount: (() => void) | undefined;
  let router: Router | undefined;

  try {
    const query = { tab, page: '2', next_page_params: encodeURIComponent(JSON.stringify({ page_key: 50 })) };
    window.__NEXT_DATA__ = { props: {}, page: '/txs', query, buildId: 'cctx-filter' };
    Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });
    Object.defineProperty(window, '__BUILD_MANIFEST', {
      configurable: true,
      writable: true,
      value: {
        __rewrites: { beforeFiles: [], afterFiles: [], fallback: [] },
        sortedPages: [ '/txs' ],
        '/txs': [],
      },
    });
    window.__DEV_PAGES_MANIFEST = { pages: [ '/txs' ] };
    const route = new Router('/txs', query, `/txs?${ new URLSearchParams(query) }`, {
      initialProps: { pageProps: {} },
      pageLoader: new PageLoader('cctx-filter', ''),
      App,
      Component: RouteFixture,
      wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => React.createElement(WrappedApp, props),
      subscription: async(data, CurrentApp) => {
        renderToStaticMarkup(React.createElement(CurrentApp, { ...data.props, Component: data.Component, router: route }));
      },
      isFallback: false,
    });
    router = route;
    await route._initialMatchesMiddlewarePromise;

    // Keep Next's public route snapshot for this mount: the active query must reset its own
    // cursor when its filter callback completes, independently of the next route render.
    const publicRouter = makePublicRouterInstance(route);
    const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(Wrapper, null,
      React.createElement(QueryClientProvider, { client },
        React.createElement(RouterContext.Provider, { value: publicRouter }, children)),
    );
    // Execute the real component and its hooks; inspect the props it supplies to the actual
    // list and filter-tag consumers without mounting their unrelated sockets and menus.
    const mounted = renderHook(() => ZetaChainCCTXsTab(), { wrapper });
    unmount = mounted.unmount;
    const visibleList = () => listProps(mounted.result.current, tab);
    await waitFor(() => {
      expect(visibleList().isPlaceholderData).toBe(false);
      expect(visibleList().items).toEqual([ laterRow ]);
    });
    expect(visibleList().pagination.page).toBe(2);
    expect(requests.at(-1)?.searchParams.get('page_key')).toBe('50');
    expect(requests.at(-1)?.searchParams.get('status_reduced')).toBe(statuses);

    await act(async() => {
      visibleList().onFilterChange('sender_address', [ sender ]);
    });
    await waitFor(() => {
      expect(visibleList().pagination.page).toBe(1);
      expect(visibleList().isPlaceholderData).toBe(false);
      expect(visibleList().items).toEqual([ filteredRow ]);
      expect(requests.at(-1)?.searchParams.get('sender_address')).toBe(sender);
      expect(requests.at(-1)?.searchParams.has('page_key')).toBe(false);
    });
    expect(requests.every((url) => url.searchParams.get('status_reduced') === statuses)).toBe(true);
    expect(router.query).toMatchObject({ tab, sender_address: sender });
    expect(router.query).not.toHaveProperty('next_page_params');
    expect(router.query).not.toHaveProperty('page');

    const tags = tabChildren(mounted.result.current)[1];
    if (!React.isValidElement<FilterTagsProps>(tags)) {
      throw new Error('The production filter tags are missing');
    }
    await act(async() => tags.props.onClearAll());
    await waitFor(() => {
      expect(visibleList().items).toEqual([ firstRow ]);
      expect(visibleList().isPlaceholderData).toBe(false);
      expect(requests.at(-1)?.searchParams.has('sender_address')).toBe(false);
      expect(requests.at(-1)?.searchParams.has('page_key')).toBe(false);
    });
    expect(visibleList().pagination.page).toBe(1);
    expect(requests.at(-1)?.searchParams.get('status_reduced')).toBe(statuses);
  } finally {
    unmount?.();
    client.clear();
    if (router) {
      window.removeEventListener('popstate', router.onPopState);
    }
    window.__BUILD_MANIFEST = initialBuildManifest;
    window.__DEV_PAGES_MANIFEST = initialPagesManifest;
    window.__NEXT_DATA__ = initialNextData;
    if (middlewareMatchers) {
      Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', middlewareMatchers);
    } else {
      Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
    }
  }
}

async function defaultCctxFiltersResetTheValidatedRequest() {
  await filterRequestCase('cctx');
}

async function minedCctxFiltersResetTheValidatedRequest() {
  await filterRequestCase('cctx_mined');
}

async function pendingCctxFiltersResetThePendingRequest() {
  await filterRequestCase('cctx_pending');
}

test(defaultCctxFiltersResetTheValidatedRequest.name, defaultCctxFiltersResetTheValidatedRequest);
test(minedCctxFiltersResetTheValidatedRequest.name, minedCctxFiltersResetTheValidatedRequest);
test(pendingCctxFiltersResetThePendingRequest.name, pendingCctxFiltersResetThePendingRequest);

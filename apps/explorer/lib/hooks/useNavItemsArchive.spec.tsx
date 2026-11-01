// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { expect, test, vi } from 'vitest';

vi.hoisted(() => {
  window.__envs = { ...window.__envs, NEXT_PUBLIC_PAXEER_X_ENABLED: 'true' };
});

import useNavItems, { isGroupItem, isInternalItem } from './useNavItems';

test('nav_groups_kernel_pages_under_archive', async() => {
  const initialData = window.__NEXT_DATA__;
  const middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  window.__NEXT_DATA__ = { props: {}, page: '/', query: {}, buildId: 'navigation-archive' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });

  try {
    for (const pathname of [ '/', '/paxeer-x/anchors', '/paxeer-x/receipts', '/blocks' ]) {
      const queryClient = new QueryClient();
      let navigation: ReturnType<typeof useNavItems> | undefined;

      function NavigationProbe() {
        navigation = useNavItems();
        return null;
      }

      const router = new Router(pathname, {}, pathname, {
        initialProps: { pageProps: {} },
        pageLoader: new PageLoader('navigation-archive', ''),
        App,
        Component: NavigationProbe,
        wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => (
          <WrappedApp { ...props } router={ router }/>
        ),
        subscription: async(data, CurrentApp) => {
          renderToStaticMarkup(<CurrentApp { ...data.props } Component={ data.Component } router={ router }/>);
        },
        isFallback: false,
      });

      try {
        await router._initialMatchesMiddlewarePromise;
        renderToStaticMarkup(
          <QueryClientProvider client={ queryClient }>
            <RouterContext.Provider value={ router }>
              <NavigationProbe/>
            </RouterContext.Provider>
          </QueryClientProvider>,
        );

        if (!navigation) {
          throw new Error('Navigation hook did not render');
        }

        const archives = navigation.mainNavItems.filter((item) => item.text === 'Kernel archive');
        expect(archives).toHaveLength(1);
        expect(navigation.mainNavItems.map((item) => item.text)).not.toContain('Kernel');
        const archive = archives[0];
        if (!archive || !isGroupItem(archive)) {
          throw new Error('Kernel archive must be a navigation group');
        }

        const children = archive.subItems.flat();
        expect(children.map((item) => item.text)).toEqual([ 'Anchor checkpoints', 'Kernel receipts' ]);
        expect(children.map((item) => isInternalItem(item) && item.nextRoute)).toEqual([
          { pathname: '/paxeer-x/anchors' },
          { pathname: '/paxeer-x/receipts' },
        ]);
        expect(children.map((item) => isInternalItem(item) && item.isActive)).toEqual([
          pathname === '/paxeer-x/anchors',
          pathname === '/paxeer-x/receipts',
        ]);
        expect(archive.isActive).toBe(pathname === '/paxeer-x/anchors' || pathname === '/paxeer-x/receipts');
        expect(navigation.mainNavItems.filter((item) => item.text === 'Anchor checkpoints' || item.text === 'Kernel receipts')).toEqual([]);
      } finally {
        window.removeEventListener('popstate', router.onPopState);
        queryClient.clear();
      }
    }
  } finally {
    window.__NEXT_DATA__ = initialData;
    if (middlewareMatchers) {
      Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', middlewareMatchers);
    } else {
      Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
    }
  }
});

// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import useNavItems, { isGroupItem } from 'lib/hooks/useNavItems';
import { allEnabled } from 'mocks/paxeerX/capabilities';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test, vi } from 'vitest';
import { fireEvent, waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
  };
});

import NavLinkGroup from './NavLinkGroup';

async function assertNativeNavigation(pathname: string) {
  const initialData = window.__NEXT_DATA__;
  const middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  window.__NEXT_DATA__ = { props: {}, page: '/', query: {}, buildId: 'navigation-retirement' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });

  try {
    const queryClient = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } });
    queryClient.setQueryData([ 'general:paxeer_x_capabilities' ], allEnabled);

    function NavigationGroups() {
      const { mainNavItems } = useNavItems();
      return React.createElement('nav', { 'aria-label': 'Merged network navigation' },
        mainNavItems.filter(isGroupItem).map((item) => React.createElement(NavLinkGroup, { key: item.text, item })),
      );
    }

    const router = new Router(pathname, {}, pathname, {
      initialProps: { pageProps: {} },
      pageLoader: new PageLoader('navigation-retirement', ''),
      App,
      Component: NavigationGroups,
      wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) =>
        React.createElement(WrappedApp, { ...props, router }),
      subscription: async(data, CurrentApp) => {
        renderToStaticMarkup(React.createElement(CurrentApp, { ...data.props, Component: data.Component, router }));
      },
      isFallback: false,
    });

    try {
      await router._initialMatchesMiddlewarePromise;
      const view = render(React.createElement(QueryClientProvider, { client: queryClient },
        React.createElement(RouterContext.Provider, { value: router }, React.createElement(NavigationGroups)),
      ));

      try {
        const groups = Array.from(view.container.querySelectorAll<HTMLElement>('[data-label="nav-group"]'));
        const blockchain = groups.find((group) => group.textContent?.trim() === 'Blockchain');
        expect(blockchain).toBeDefined();
        expect(groups.map((group) => group.textContent?.trim())).not.toContain('Kernel archive');
        expect(groups.map((group) => group.textContent?.trim())).not.toContain('Kernel');

        if (!blockchain) {
          throw new Error('Native Blockchain navigation must be visible');
        }

        fireEvent.click(blockchain);
        expect(blockchain.hasAttribute('data-active')).toBe(true);
        const panel = await waitFor(() => {
          const content = document.getElementById(blockchain.getAttribute('aria-describedby') || '');
          expect(content).not.toBeNull();
          return content;
        });

        for (const [ text, href ] of [ [ 'Blocks', '/blocks' ], [ 'Transactions', '/txs' ], [ 'Top accounts', '/accounts' ] ]) {
          const link = panel?.querySelector(`a[href="${ href }"]`);
          expect(link?.textContent?.trim()).toBe(text);
          expect(link?.hasAttribute('data-selected')).toBe(pathname === href);
        }

        expect(blockchain.hasAttribute('data-selected')).toBe([ '/blocks', '/txs', '/accounts' ].includes(pathname));
        expect(document.querySelectorAll('a[href^="/paxeer-x/anchors"], a[href^="/paxeer-x/receipts"]')).toHaveLength(0);
        expect(document.body.textContent).not.toContain('Anchor checkpoints');
        expect(document.body.textContent).not.toContain('Kernel receipts');

        fireEvent.click(blockchain);
        expect(blockchain.hasAttribute('data-active')).toBe(false);
      } finally {
        view.unmount();
      }
    } finally {
      window.removeEventListener('popstate', router.onPopState);
      queryClient.clear();
    }
  } finally {
    window.__NEXT_DATA__ = initialData;
    if (middlewareMatchers) {
      Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', middlewareMatchers);
    } else {
      Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
    }
  }
}

function retiredNavigationContractUsesNativeDestinations() {
  return assertNativeNavigation('/');
}

function nativeBlockNavigationSelectsBlockchain() {
  return assertNativeNavigation('/blocks');
}

function nativeTransactionNavigationSelectsBlockchain() {
  return assertNativeNavigation('/txs');
}

function nativeAccountNavigationSelectsBlockchain() {
  return assertNativeNavigation('/accounts');
}

function retiredAnchorNavigationIsAbsent() {
  return assertNativeNavigation('/paxeer-x/anchors');
}

function retiredReceiptNavigationIsAbsent() {
  return assertNativeNavigation('/paxeer-x/receipts');
}

test('retiredNavigationContractUsesNativeDestinations', retiredNavigationContractUsesNativeDestinations);
test('nativeBlockNavigationSelectsBlockchain', nativeBlockNavigationSelectsBlockchain);
test('nativeTransactionNavigationSelectsBlockchain', nativeTransactionNavigationSelectsBlockchain);
test('nativeAccountNavigationSelectsBlockchain', nativeAccountNavigationSelectsBlockchain);
test('retiredAnchorNavigationIsAbsent', retiredAnchorNavigationIsAbsent);
test('retiredReceiptNavigationIsAbsent', retiredReceiptNavigationIsAbsent);

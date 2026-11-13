// @vitest-environment jsdom

import dotenv from 'dotenv';
import React, { type ComponentProps } from 'react';

import { expect, test, vi } from 'vitest';

import './testWrapper';

function setViewport(width: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width });
  window.matchMedia = (media: string) => {
    const target = new EventTarget();
    const matches = Array.from(media.matchAll(/\((min|max)-width:\s*([\d.]+)(px|r?em)\)/g))
      .every(([ , bound, amount, unit ]) => {
        const limit = Number(amount) * (unit === 'px' ? 1 : 16);
        return bound === 'min' ? width >= limit : width <= limit;
      });
    return {
      media,
      matches,
      onchange: null,
      addListener: (listener) => listener && target.addEventListener('change', listener as EventListener),
      removeListener: (listener) => listener && target.removeEventListener('change', listener as EventListener),
      addEventListener: target.addEventListener.bind(target),
      removeEventListener: target.removeEventListener.bind(target),
      dispatchEvent: target.dispatchEvent.bind(target),
    };
  };
}

function responsiveDisplay(element: HTMLElement) {
  let display = '';
  function visit(rules: CSSRuleList) {
    for (const rule of Array.from(rules)) {
      if (rule instanceof CSSMediaRule && !window.matchMedia(rule.conditionText).matches) continue;
      if (rule instanceof CSSStyleRule && rule.style.display && element.matches(rule.selectorText)) {
        display = rule.style.display;
      }
      if ('cssRules' in rule) visit((rule as CSSGroupingRule).cssRules);
    }
  }
  for (const sheet of Array.from(document.styleSheets)) visit(sheet.cssRules);
  return display;
}

async function searchResultsRoutesKeepOneHeaderAndLiveSearch() {
  const originalEnvs = window.__envs;
  const originalReact = Object.getOwnPropertyDescriptor(globalThis, 'React');
  const originalLocation = window.location.href;
  const originalConfig = window.__multichainConfig;
  const originalNextData = window.__NEXT_DATA__;
  const originalMedia = window.matchMedia;
  const originalWidth = window.innerWidth;
  const originalScrollTo = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollTo');
  const bootKeys = [ '__DEV_PAGES_MANIFEST', '__DEV_MIDDLEWARE_MATCHERS', '__BUILD_MANIFEST' ];
  const bootDescriptors = bootKeys.map(key => Object.getOwnPropertyDescriptor(window, key));
  const envs = dotenv.config({ path: './configs/envs/.env.vitest' });
  if (envs.error || !envs.parsed) throw envs.error || new Error('Missing explorer test environment');
  fetchMock.disableMocks();
  if (!Element.prototype.scrollTo) {
    Object.defineProperty(Element.prototype, 'scrollTo', {
      configurable: true,
      value: function scrollTo(this: Element, options?: ScrollToOptions | number, y?: number) {
        if (typeof options === 'number') {
          this.scrollLeft = options;
          this.scrollTop = y ?? 0;
        } else {
          if (options?.left !== undefined) this.scrollLeft = options.left;
          if (options?.top !== undefined) this.scrollTop = options.top;
        }
      },
    });
  }

  try {
    for (const multichain of [ false, true ]) {
      vi.resetModules();
      window.__envs = {
        ...envs.parsed,
        NEXT_PUBLIC_NAVIGATION_LAYOUT: 'horizontal',
        NEXT_PUBLIC_API_HOST: 'eth.blockscout.com',
        NEXT_PUBLIC_API_PROTOCOL: 'https',
        NEXT_PUBLIC_API_PORT: '',
        NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
        NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
        NEXT_PUBLIC_MULTICHAIN_ENABLED: String(multichain),
        NEXT_PUBLIC_MULTICHAIN_CLUSTER: 'superchain',
        NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST: 'https://multichain-aggregator.services.blockscout.com',
        NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST: 'https://multichain-stats.services.blockscout.com',
        NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'true',
        NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER: 'auth0',
      };
      const React = await import('react');
      Object.defineProperty(globalThis, 'React', { configurable: true, value: React });
      const { render, cleanup, fireEvent, getRoles, screen, waitFor, within } = await import('@testing-library/react');
      const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
      const { GrowthBookProvider } = await import('@growthbook/growthbook-react');
      const { 'default': App } = await import('next/app');
      const { 'default': PageLoader } = await import('next/dist/client/page-loader');
      const { 'default': Router } = await import('next/dist/shared/lib/router/router');
      const { makePublicRouterInstance } = await import('next/router');
      const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
      const { Provider } = await import('toolkit/chakra/provider');
      const { AppContextProvider } = await import('lib/contexts/app');
      const { SettingsContextProvider } = await import('lib/contexts/settings');
      const { MarketplaceContextProvider } = await import('lib/contexts/marketplace');
      const { SocketProvider } = await import('lib/socket/context');
      const { 'default': config } = await import('configs/app');
      const { 'default': SearchRoute } = await import('pages/search-results');
      const { 'default': Results } = multichain ?
        await import('ui/multichain/searchResults/SearchResults') : await import('ui/pages/SearchResults');
      expect(config.features.multichain.isEnabled).toBe(multichain);
      expect(config.features.account.isEnabled).toBe(true);
      window.__multichainConfig = {
        chains: [
          { id: '10', slug: 'optimism', name: 'Optimism', explorer_url: 'https://optimism.blockscout.com', app_config: config },
          { id: '8453', slug: 'base', name: 'Base', explorer_url: 'https://base.blockscout.com', app_config: config },
        ],
      };

      for (const width of [ 1440, 390 ]) {
        setViewport(width);
        const initialQuery: Record<string, string> = { q: 'headerregressioninitial', ...(multichain ? { tab: 'all', chain_id: 'all' } : {}) };
        const asPath = `/search-results?${ new URLSearchParams(initialQuery) }`;
        window.history.replaceState(null, '', asPath);
        window.__NEXT_DATA__ = { props: {}, page: '/search-results', query: initialQuery, buildId: 'search-results', gip: true };
        Object.assign(window, {
          __DEV_PAGES_MANIFEST: { pages: [ '/search-results' ] },
          __DEV_MIDDLEWARE_MATCHERS: [],
          __BUILD_MANIFEST: { '/search-results': [], sortedPages: [ '/search-results' ], __rewrites: { beforeFiles: [], afterFiles: [], fallback: [] } },
        });
        const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
        let updateRouter: (() => void) | undefined;
        const router = new Router('/search-results', initialQuery, asPath, {
          initialProps: { pageProps: {}, gip: true },
          pageLoader: new PageLoader('search-results', ''),
          App,
          Component: Results,
          wrapApp: (CurrentApp) => (props: ComponentProps<typeof App>) => <CurrentApp { ...props } router={ router }/>,
          subscription: async() => {
            updateRouter?.();
          },
          isFallback: false,
        });
        // The route is already loaded by this composition, as after Next's initial hydration.
        Reflect.deleteProperty(router.components['/search-results'], 'initial');
        function RoutedResults() {
          const [ , refresh ] = React.useReducer(value => value + 1, 0);
          updateRouter = refresh;
          return (
            <RouterContext.Provider value={ makePublicRouterInstance(router) }>
              <Provider>
                <QueryClientProvider client={ client }>
                  <AppContextProvider
                    pageProps={{ cookies: '', referrer: '', query: initialQuery, adBannerProvider: null, apiData: null, uuid: 'search-results' }}
                  >
                    <GrowthBookProvider>
                      <SocketProvider>
                        <MarketplaceContextProvider>
                          <SettingsContextProvider>
                            { SearchRoute.getLayout!(<Results/>) }
                          </SettingsContextProvider>
                        </MarketplaceContextProvider>
                      </SocketProvider>
                    </GrowthBookProvider>
                  </AppContextProvider>
                </QueryClientProvider>
              </Provider>
            </RouterContext.Provider>
          );
        }
        const mounted = render(<RoutedResults/>);
        try {
          await screen.findByRole('heading', { name: 'Search results' });
          const banners = within(mounted.container).getAllByRole('banner', { hidden: true })
            .filter(header => responsiveDisplay(header) !== 'none');
          expect(banners).toHaveLength(1);
          const desktopHeaders = mounted.container.querySelectorAll<HTMLElement>('[data-label="desktop-header"]');
          const mobileHeaders = mounted.container.querySelectorAll<HTMLElement>('[data-label="mobile-header"]');
          expect(desktopHeaders).toHaveLength(1);
          expect(mobileHeaders).toHaveLength(1);
          const desktop = desktopHeaders[0];
          const mobile = mobileHeaders[0];
          expect(banners[0]).toBe(width >= 1024 ? desktop : mobile);
          expect(responsiveDisplay(desktop)).toBe(width >= 1024 ? 'block' : 'none');
          expect(responsiveDisplay(mobile)).toBe(width >= 1024 ? 'none' : 'block');
          expect(within(desktop).getByLabelText('Main navigation')).toBeTruthy();
          expect(within(desktop).getByLabelText('Link to main page')).toBeTruthy();
          expect(desktop.querySelector('[data-label="header-brand"]')?.textContent).toContain(config.chain.name);
          expect(within(desktop).getByLabelText('User settings')).toBeTruthy();
          expect(within(desktop).getByText('Log in').closest('button')).toBeTruthy();
          const term = `headerregression${ multichain ? 'multi' : 'single' }${ width }`;
          if (width >= 1024) {
            const input = within(desktop).getByLabelText('Search', { selector: 'input' });
            fireEvent.change(input, { target: { value: term } });
            fireEvent.submit(input.closest('form')!);
          } else {
            fireEvent.click(within(mobile).getByRole('button', { name: 'Search' }));
            const search = await screen.findByRole('dialog', { name: 'Search' });
            const input = within(search).getByLabelText('Search', { selector: 'input' });
            fireEvent.change(input, { target: { value: term } });
            fireEvent.submit(input.closest('form')!);
            await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Search' })).toBeNull());
          }
          await waitFor(() => expect(router.query.q).toBe(term));
          expect(new URLSearchParams(window.location.search).get('q')).toBe(term);
          expect((within(desktop).getByLabelText('Search', { selector: 'input' }) as HTMLInputElement).value).toBe(term);
          await waitFor(() => {
            const searches = client.getQueryCache().findAll().filter(query =>
              (multichain ? String(query.queryKey[0]).startsWith('multichainAggregator:search_') : query.queryKey[0] === 'general:search') &&
              query.queryKey.some(part => typeof part === 'object' && part !== null && 'q' in part && part.q === term),
            );
            expect(searches, `Search query keys for ${ term }: ${ JSON.stringify(searches.map(query => query.queryKey)) }`)
              .toHaveLength(multichain ? 7 : 1);
            searches.forEach(query => expect(query.state.status).toBe('success'));
          }, { timeout: 15000 });
          if (multichain) {
            fireEvent.click(screen.getByRole('tab', { name: 'Tokens (ERC-20)' }));
            await waitFor(() => expect(router.query.tab).toBe('tokens'));
            const filters = width >= 1024 ? screen.getByRole('tablist') : screen.getByRole('tabpanel', { name: 'Tokens (ERC-20)' });
            const chainSelect = await waitFor(() => {
              const triggers = within(filters).queryAllByRole('combobox', { name: 'Chain' });
              const roles = Object.fromEntries(Object.entries(getRoles(filters)).map(([ role, elements ]) => [
                role,
                elements.map(element => ({ label: element.getAttribute('aria-label'), text: element.textContent })),
              ]));
              expect(triggers, `Chain select roles inside filters: ${ JSON.stringify(roles) }`).toHaveLength(1);
              return triggers[0];
            });
            fireEvent.click(chainSelect);
            fireEvent.click(await screen.findByRole('option', { name: /Optimism/ }));
            await waitFor(() => expect(router.query.chain_id).toBe('10'));
            expect(router.query.q).toBe(term);
            expect(router.query.tab).toBe('tokens');
            expect(screen.getByRole('tab', { name: 'Tokens (ERC-20)' }).getAttribute('aria-selected')).toBe('true');
            await waitFor(() => expect(client.getQueryCache().findAll().some(query =>
              query.queryKey[0] === 'multichainAggregator:search_tokens' && query.queryKey.some(part =>
                typeof part === 'object' && part !== null && 'chain_id' in part && part.chain_id === '10' && 'q' in part && part.q === term,
              ) && query.state.status === 'success',
            )).toBe(true), { timeout: 15000 });
          } else {
            await screen.findByText(term, { exact: true });
          }
          if (width < 1024) {
            fireEvent.click(within(mobile).getByRole('button', { name: 'Menu button' }));
            const navigation = await screen.findByRole('navigation', { name: 'Mobile navigation' });
            const explore = within(navigation).getByRole('button', { name: 'Explore' });
            fireEvent.click(explore);
            expect(explore.getAttribute('aria-expanded')).toBe('true');
            expect(within(navigation).getByRole('link', { name: /Browse the latest blocks/ }).getAttribute('href')).toBe('/blocks');
          }
          expect(mounted.container.querySelectorAll('[data-label="desktop-header"]')).toHaveLength(1);
          expect(mounted.container.querySelectorAll('[data-label="mobile-header"]')).toHaveLength(1);
        } finally {
          mounted.unmount();
          cleanup();
          client.clear();
          window.removeEventListener('popstate', router.onPopState);
        }
      }
    }
  } finally {
    if (originalReact) Object.defineProperty(globalThis, 'React', originalReact);
    else Reflect.deleteProperty(globalThis, 'React');
    window.__envs = originalEnvs;
    window.__multichainConfig = originalConfig;
    window.__NEXT_DATA__ = originalNextData;
    window.history.replaceState(null, '', originalLocation);
    window.matchMedia = originalMedia;
    if (originalScrollTo) Object.defineProperty(Element.prototype, 'scrollTo', originalScrollTo);
    else Reflect.deleteProperty(Element.prototype, 'scrollTo');
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth });
    bootKeys.forEach((key, index) => {
      const descriptor = bootDescriptors[index];
      if (descriptor) Object.defineProperty(window, key, descriptor);
      else Reflect.deleteProperty(window, key);
    });
    fetchMock.enableMocks();
  }
}

test(searchResultsRoutesKeepOneHeaderAndLiveSearch.name, searchResultsRoutesKeepOneHeaderAndLiveSearch, 120000);

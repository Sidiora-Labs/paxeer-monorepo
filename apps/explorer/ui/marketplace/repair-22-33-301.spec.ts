// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import { getRouteMatcher } from 'next/dist/shared/lib/router/utils/route-matcher';
import { getRouteRegex } from 'next/dist/shared/lib/router/utils/route-regex';
import React from 'react';

import config from 'configs/app';
import { apps } from 'mocks/apps/apps';
import { Provider } from 'toolkit/chakra/provider';
// Reuse the committed jsdom browser-host fixtures; the components and router remain real.
import 'ui/shared/layout/testWrapper';
import { afterEach, beforeEach, expect, test } from 'vitest';

import getMarketplaceAppShareUrl from './getMarketplaceAppShareUrl';
import MarketplaceAppCard from './MarketplaceAppCard';
import MarketplaceAppModal from './MarketplaceAppModal';

class InMemoryClipboard implements Pick<Clipboard, 'writeText' | 'readText'> {
  private text = '';

  async writeText(text: string) {
    this.text = text;
  }

  async readText() {
    return this.text;
  }
}

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
const clients: Array<QueryClient> = [];
const routers: Array<Router> = [];

beforeEach(() => {
  window.history.replaceState({}, '', '/explorer/apps?category=Bridge#list');
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: new InMemoryClipboard() });
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
  routers.splice(0).forEach((router) => window.removeEventListener('popstate', router.onPopState));
  if (originalClipboard) {
    Object.defineProperty(navigator, 'clipboard', originalClipboard);
  } else {
    Reflect.deleteProperty(navigator, 'clipboard');
  }
});

function renderMarketplace(component: React.ReactElement) {
  function MarketplacePage() {
    return component;
  }
  window.__NEXT_DATA__ = { props: {}, page: '/apps', query: {}, buildId: 'repair-22-33-301' };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  const router = new Router('/apps', {}, '/apps', {
    initialProps: {},
    pageLoader: new PageLoader('repair-22-33-301', ''),
    App,
    Component: MarketplacePage,
    wrapApp: (component) => component,
    subscription: async() => {},
    isFallback: false,
  });
  routers.push(router);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  clients.push(client);
  return render(React.createElement(RouterContext.Provider, { value: router },
    React.createElement(QueryClientProvider, { client },
      React.createElement(Provider, null, React.createElement(MarketplacePage)))));
}

async function copyShareLink(id: string) {
  const button = await screen.findByRole('button', { name: 'copy' });
  fireEvent.click(button);
  const expected = `${ window.location.origin }/explorer/apps/${ encodeURIComponent(id) }`;
  await waitFor(async() => {
    expect(await navigator.clipboard.readText()).toBe(expected);
  });
  const copied = await navigator.clipboard.readText();
  const destination = new URL(copied);
  expect(destination.origin).toBe(window.location.origin);
  expect(destination.pathname).toBe(`/explorer/apps/${ encodeURIComponent(id) }`);
  expect(destination.search).toBe('');
  expect(destination.hash).toBe('');
  expect(config.app.basePath).toBe('/explorer');
  const match = getRouteMatcher(getRouteRegex('/apps/[id]'))(destination.pathname.slice(config.app.basePath.length));
  expect(match).toEqual({ id });
  expect(button.querySelector('use')?.getAttribute('href')?.endsWith('#check')).toBe(true);
  return copied;
}

test('marketplaceCardSharePathCopiesMountedAbsoluteUrl', async function marketplaceCardSharePathCopiesMountedAbsoluteUrl() {
  for (const app of apps) {
    const view = renderMarketplace(React.createElement(MarketplaceAppCard, {
      ...app,
      isFavorite: false,
      isLoading: false,
      onInfoClick: () => {},
      onFavoriteClick: () => {},
      onAppClick: () => {},
    }));
    expect(await copyShareLink(app.id)).toBe(getMarketplaceAppShareUrl(app.id, window.location.origin));
    view.unmount();
  }
});

test('marketplaceModalSharePathMatchesCardAndReopensSelectedApp', async function marketplaceModalSharePathMatchesCardAndReopensSelectedApp() {
  for (const app of apps) {
    const modal = renderMarketplace(React.createElement(MarketplaceAppModal, {
      data: app,
      isFavorite: false,
      onFavoriteClick: () => {},
      onClose: () => {},
    }));
    const modalLink = await copyShareLink(app.id);
    modal.unmount();
    const card = renderMarketplace(React.createElement(MarketplaceAppCard, {
      ...app,
      isFavorite: false,
      isLoading: false,
      onInfoClick: () => {},
      onFavoriteClick: () => {},
      onAppClick: () => {},
    }));
    expect(await copyShareLink(app.id)).toBe(modalLink);
    card.unmount();
  }
});

test('marketplaceShareUrlEncodesIdAndUsesCurrentOrigin', function marketplaceShareUrlEncodesIdAndUsesCurrentOrigin() {
  const id = 'bridge /?#%';
  const origin = 'https://explorer.example:8443';
  const copied = getMarketplaceAppShareUrl(id, origin);
  expect(copied).toBe(`${ origin }/explorer/apps/bridge%20%2F%3F%23%25`);
  const destination = new URL(copied);
  expect(getRouteMatcher(getRouteRegex('/apps/[id]'))(destination.pathname.slice(config.app.basePath.length))).toEqual({ id });
});

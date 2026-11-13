// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, waitFor as waitForDom } from '@testing-library/react';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { NextRouter } from 'next/router';
import { EventEmitter, once } from 'node:events';
import { createServer } from 'node:http';
import { format } from 'node:url';
import React from 'react';

import type { TokenHolder } from 'types/api/token';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import { tokenHoldersERC20 } from 'mocks/tokens/tokenHolders';
import { tokenInfo } from 'mocks/tokens/tokenInfo';
import { Provider } from 'toolkit/chakra/provider';
import 'ui/shared/layout/testWrapper';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import TokenHolders from './TokenHolders';

beforeAll(() => fetchMock.disableMocks());
afterAll(() => fetchMock.enableMocks());
afterEach(cleanup);

function waitFor(assertion: () => void) {
  return waitForDom(assertion, { timeout: 10000 });
}

const records: Array<TokenHolder> = Array.from({ length: 57 }, (_, index) => ({
  ...tokenHoldersERC20.items[0],
  address: {
    ...tokenHoldersERC20.items[0].address,
    hash: `0x${ (index + 1).toString(16).padStart(40, '0') }`,
    name: `Holder ${ index + 1 }`,
  },
}));

// A route history held in memory supplies Next's real context; navigation updates the query
// consumed by useQueryWithPages without needing a Next server or a mocked module.
function RouteHistory({ children }: { children: React.ReactNode }) {
  const [ entries, setEntries ] = React.useState([ `/token/[hash]?hash=${ tokenInfo.address_hash }&tab=holders` ]);
  const [ position, setPosition ] = React.useState(0);
  const [ events ] = React.useState(() => new EventEmitter());
  const current = new URL(entries[position], 'http://localhost');
  const hash = current.searchParams.get('hash');
  if (!hash) {
    throw new Error('The token route requires a hash');
  }
  const navigate: NextRouter['push'] = async(url, _as, options) => {
    const target = typeof url === 'string' ? url : format(url);
    events.emit('routeChangeStart', target, options);
    setEntries(previous => [ ...previous.slice(0, position + 1), target ]);
    setPosition(position + 1);
    events.emit('routeChangeComplete', target, options);
    return true;
  };
  const router: NextRouter = {
    pathname: '/token/[hash]',
    route: '/token/[hash]',
    query: { ...Object.fromEntries(current.searchParams), hash },
    asPath: current.pathname + current.search,
    basePath: '',
    isReady: true,
    isFallback: false,
    isPreview: false,
    isLocaleDomain: false,
    push: navigate,
    replace: async(url) => {
      const target = typeof url === 'string' ? url : format(url);
      setEntries(previous => previous.map((entry, index) => index === position ? target : entry));
      return true;
    },
    back: () => setPosition(index => Math.max(0, index - 1)),
    forward: () => setPosition(index => Math.min(entries.length - 1, index + 1)),
    reload: () => {
      throw new Error('A reload is not a cursor navigation');
    },
    prefetch: async() => {
      throw new Error('These tests do not prefetch routes');
    },
    beforePopState: () => {
      throw new Error('These tests do not install popstate handlers');
    },
    events: {
      on: (name, handler) => {
        events.on(name, handler);
      },
      off: (name, handler) => {
        events.off(name, handler);
      },
      emit: (name, ...args) => {
        events.emit(name, ...args);
      },
    },
  };
  return React.createElement(RouterContext.Provider, { value: router }, children);
}

async function openHolders(total = records.length, holdersCount?: number) {
  const requests: Array<number> = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    if (url.pathname !== `/api/v2/tokens/${ tokenInfo.address_hash }/holders`) {
      response.writeHead(404);
      response.end();
      return;
    }
    const offset = Number(url.searchParams.get('items_count') ?? 0);
    requests.push(offset);
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify({
      items: records.slice(offset, Math.min(offset + 50, total)),
      next_page_params: offset + 50 < total ? { items_count: offset + 50, value: records[offset + 49].value } : null,
    }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP address');
  }
  const chain: ClusterChainConfig = {
    id: '125',
    name: 'Holders fixture',
    slug: 'holders-fixture',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        general: {
          endpoint: `http://127.0.0.1:${ address.port }`,
          host: '127.0.0.1',
          protocol: 'http',
          port: String(address.port),
          socketEndpoint: `ws://127.0.0.1:${ address.port }`,
          basePath: '',
        },
      },
    },
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  function HoldersConsumer() {
    const holdersQuery = useQueryWithPages({
      resourceName: 'general:token_holders',
      pathParams: { hash: tokenInfo.address_hash },
      noScroll: true,
    });
    return React.createElement(TokenHolders, { holdersQuery, token: tokenInfo, holdersCount });
  }
  const view = render(React.createElement(Provider, null,
    React.createElement(QueryClientProvider, { client },
      React.createElement(RouteHistory, null,
        React.createElement(MultichainContext.Provider, { value: { chain } }, React.createElement(HoldersConsumer)),
      ),
    ),
  ));
  const close = async() => {
    view.unmount();
    client.clear();
    await new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  };
  try {
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(0, Math.min(total, 50)).map(item => item.address.name)));
  } catch (error) {
    await close();
    throw error;
  }
  return { ...view, requests, close };
}

function visibleHolders(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[data-token-holders-row]')).map(row =>
    Array.from(row.querySelectorAll('[data-entity-kind="address"] [data-entity-content]')).map(entity => entity.textContent).join(''),
  );
}

function control(container: HTMLElement, name: string) {
  const button = container.querySelector<HTMLButtonElement>(`[data-footer-pagination] [data-control="${ name }"]`);
  if (!button) {
    throw new Error(`Missing ${ name } pagination control`);
  }
  return button;
}

async function selectRows(container: HTMLElement, size: number) {
  const select = container.querySelector<HTMLSelectElement>('[data-scan-show-rows] select');
  if (!select) {
    throw new Error('Missing row selector');
  }
  fireEvent.change(select, { target: { value: String(size) } });
  await waitFor(() => expect(select.value).toBe(String(size)));
}

async function rowsHoldersTraversesLocalSubpagesBeforeAdvancingTheCursor() {
  const view = await openHolders();
  try {
    await selectRows(view.container, 25);
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(0, 25).map(item => item.address.name)));
    const visited = [ ...visibleHolders(view.container) ];
    fireEvent.click(control(view.container, 'next'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(25, 50).map(item => item.address.name)));
    visited.push(...visibleHolders(view.container));
    expect(view.requests).toEqual([ 0 ]);
    expect(control(view.container, 'page').textContent).toBe('Page 2');
    fireEvent.click(control(view.container, 'next'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(50).map(item => item.address.name)));
    visited.push(...visibleHolders(view.container));
    expect(visited).toEqual(records.map(item => item.address.name));
    expect(view.requests).toEqual([ 0, 50 ]);
    expect(control(view.container, 'next').disabled).toBe(true);
    fireEvent.click(control(view.container, 'prev'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(25, 50).map(item => item.address.name)));
    fireEvent.click(control(view.container, 'prev'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(0, 25).map(item => item.address.name)));
    expect(control(view.container, 'prev').disabled).toBe(true);
    expect(control(view.container, 'first').disabled).toBe(true);
  } finally {
    await view.close();
  }
}

async function rowsHoldersPaginatesASingleServerPageAndResetsTheSelection() {
  const view = await openHolders(50);
  try {
    await selectRows(view.container, 25);
    await waitFor(() => expect(visibleHolders(view.container)).toHaveLength(25));
    fireEvent.click(control(view.container, 'next'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(25, 50).map(item => item.address.name)));
    expect(control(view.container, 'next').disabled).toBe(true);
    fireEvent.click(control(view.container, 'first'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(0, 25).map(item => item.address.name)));
    fireEvent.click(control(view.container, 'next'));
    await selectRows(view.container, 50);
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(0, 50).map(item => item.address.name)));
    const options = Array.from(view.container.querySelectorAll('[data-scan-show-rows] select option')).map(option => option.getAttribute('value'));
    expect(options.filter(Boolean)).toEqual([ '25', '50' ]);
    expect(view.requests).toEqual([ 0 ]);
  } finally {
    await view.close();
  }
}

async function totalHolderLabelsTheFinalPageWithoutInventingAGlobalTotal() {
  const view = await openHolders();
  try {
    expect(view.container.querySelector('[data-title]')?.textContent).toBe('50 holders on this page');
    fireEvent.click(control(view.container, 'next'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(50).map(item => item.address.name)));
    expect(view.container.querySelector('[data-title]')?.textContent).toBe('7 holders on this page');
    expect(view.requests).toEqual([ 0, 50 ]);
    expect(control(view.container, 'next').disabled).toBe(true);
  } finally {
    await view.close();
  }
}

async function totalHolderPreservesAnAuthoritativeTotalAcrossPages() {
  const view = await openHolders(records.length, records.length);
  try {
    expect(view.container.querySelector('[data-title]')?.textContent).toBe('Latest 50 from a total of 57 holders');
    fireEvent.click(control(view.container, 'next'));
    await waitFor(() => expect(visibleHolders(view.container)).toEqual(records.slice(50).map(item => item.address.name)));
    expect(view.container.querySelector('[data-title]')?.textContent).toBe('Latest 7 from a total of 57 holders');
  } finally {
    await view.close();
  }
}

test(rowsHoldersTraversesLocalSubpagesBeforeAdvancingTheCursor, { timeout: 20000 });
test(rowsHoldersPaginatesASingleServerPageAndResetsTheSelection, { timeout: 20000 });
test(totalHolderLabelsTheFinalPageWithoutInventingAGlobalTotal, { timeout: 20000 });
test(totalHolderPreservesAnAuthoritativeTotalAcrossPages, { timeout: 20000 });

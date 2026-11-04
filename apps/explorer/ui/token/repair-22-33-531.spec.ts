// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, waitFor as waitForDom } from '@testing-library/react';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { NextRouter } from 'next/router';
import { EventEmitter, once } from 'node:events';
import { createServer } from 'node:http';
import { format } from 'node:url';
import React from 'react';

import type { TokenInstance, TokenInventoryResponse } from 'types/api/token';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { MultichainContext } from 'lib/contexts/multichain';
import { tokenInfoERC721a as tokenInfo } from 'mocks/tokens/tokenInfo';
import { base as tokenInstance } from 'mocks/tokens/tokenInstance';
import { Provider } from 'toolkit/chakra/provider';
import 'ui/shared/layout/testWrapper';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import TokenInventory from './TokenInventory';

beforeAll(() => {
  fetchMock.disableMocks();
});
afterAll(() => {
  fetchMock.enableMocks();
});
afterEach(cleanup);

function waitFor(assertion: () => void) {
  return waitForDom(assertion, { timeout: 10000 });
}

const records: Array<TokenInstance> = Array.from({ length: 107 }, (_, index) => ({
  ...tokenInstance,
  id: String(index + 1),
}));

// A route history held in memory supplies Next's real context; navigation updates the query
// consumed by useQueryWithPages without needing a Next server or a mocked module.
function RouteHistory({ children }: { children: React.ReactNode }) {
  const [ entries, setEntries ] = React.useState([ `/token/[hash]?hash=${ tokenInfo.address_hash }&tab=inventory` ]);
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

async function openInventory(total = records.length) {
  const requests: Array<number> = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    if (url.pathname === `/api/v2/tokens/${ tokenInfo.address_hash }`) {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify(tokenInfo));
      return;
    }
    if (url.pathname !== `/api/v2/tokens/${ tokenInfo.address_hash }/instances`) {
      response.writeHead(404);
      response.end();
      return;
    }
    const offset = Number(url.searchParams.get('unique_token') ?? 0);
    requests.push(offset);
    response.writeHead(200, { 'content-type': 'application/json' });
    const page: TokenInventoryResponse = {
      items: records.slice(offset, Math.min(offset + 50, total)),
      next_page_params: offset + 50 < total ? { unique_token: offset + 50 } : null,
    };
    response.end(JSON.stringify(page));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP address');
  }
  const chain: ClusterChainConfig = {
    id: '125',
    name: 'Inventory fixture',
    slug: 'inventory-fixture',
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
  function InventoryConsumer() {
    const inventoryQuery = useQueryWithPages({
      resourceName: 'general:token_inventory',
      pathParams: { hash: tokenInfo.address_hash },
      noScroll: true,
    });
    const tokenQuery = useApiQuery('general:token', { pathParams: { hash: tokenInfo.address_hash } });
    return React.createElement(TokenInventory, { inventoryQuery, tokenQuery });
  }
  const view = render(React.createElement(Provider, null,
    React.createElement(QueryClientProvider, { client },
      React.createElement(RouteHistory, null,
        React.createElement(MultichainContext.Provider, { value: { chain } }, React.createElement(InventoryConsumer)),
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
    await waitFor(() => expect(visibleInventory(view.container)).toEqual(records.slice(0, Math.min(total, 50)).map(item => item.id)));
  } catch (error) {
    await close();
    throw error;
  }
  return { ...view, requests, close };
}

function visibleInventory(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[data-inventory-item]')).map(row => row.getAttribute('data-inventory-item'));
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

async function expectInventoryPage(container: HTMLElement, start: number, end: number, page: number) {
  await waitFor(() => {
    expect(visibleInventory(container)).toEqual(records.slice(start, end).map(item => item.id));
    expect(container.querySelector('[data-footer-pagination] [data-control="page"]')?.textContent).toBe(`Page ${ page }`);
  });
}

async function rowsNftTraversesEveryLocalSubpageBeforeAdvancingTheServerCursor() {
  const view = await openInventory();
  try {
    await selectRows(view.container, 25);
    const visited: Array<string | null> = [];
    for (let start = 0; start < records.length; start += 25) {
      const page = start / 25 + 1;
      await expectInventoryPage(view.container, start, Math.min(start + 25, records.length), page);
      visited.push(...visibleInventory(view.container));
      expect(view.requests).toEqual(Array.from({ length: Math.floor(start / 50) + 1 }, (_, index) => index * 50));
      if (start + 25 < records.length) {
        expect(control(view.container, 'next').disabled).toBe(false);
        fireEvent.click(control(view.container, 'next'));
      }
    }
    expect(visited).toEqual(records.map(item => item.id));
    expect(new Set(visited).size).toBe(records.length);
    expect(control(view.container, 'next').disabled).toBe(true);
    for (let start = 75; start >= 0; start -= 25) {
      fireEvent.click(control(view.container, 'prev'));
      await expectInventoryPage(view.container, start, start + 25, start / 25 + 1);
    }
    expect(control(view.container, 'prev').disabled).toBe(true);
    expect(control(view.container, 'first').disabled).toBe(true);
  } finally {
    await view.close();
  }
}

async function rowsNftPaginatesATerminalFiftyRecordResponse() {
  const view = await openInventory(50);
  try {
    await selectRows(view.container, 25);
    await expectInventoryPage(view.container, 0, 25, 1);
    fireEvent.click(control(view.container, 'next'));
    await expectInventoryPage(view.container, 25, 50, 2);
    expect(control(view.container, 'next').disabled).toBe(true);
    fireEvent.click(control(view.container, 'first'));
    await expectInventoryPage(view.container, 0, 25, 1);
    expect(control(view.container, 'prev').disabled).toBe(true);
    fireEvent.click(control(view.container, 'next'));
    await expectInventoryPage(view.container, 25, 50, 2);
    await selectRows(view.container, 50);
    await waitFor(() => expect(visibleInventory(view.container)).toEqual(records.slice(0, 50).map(item => item.id)));
    expect(view.requests).toEqual([ 0 ]);
    const options = Array.from(view.container.querySelectorAll('[data-scan-show-rows] select option')).map(option => option.getAttribute('value'));
    expect(options.filter(Boolean)).toEqual([ '25', '50' ]);
  } finally {
    await view.close();
  }
}

async function rowsNftResizesAndReturnsToFirstFromALaterServerPage() {
  const view = await openInventory();
  try {
    await selectRows(view.container, 25);
    await expectInventoryPage(view.container, 0, 25, 1);
    fireEvent.click(control(view.container, 'next'));
    await expectInventoryPage(view.container, 25, 50, 2);
    fireEvent.click(control(view.container, 'next'));
    await expectInventoryPage(view.container, 50, 75, 3);
    fireEvent.click(control(view.container, 'next'));
    await expectInventoryPage(view.container, 75, 100, 4);
    await selectRows(view.container, 50);
    await expectInventoryPage(view.container, 50, 100, 2);
    await selectRows(view.container, 25);
    await expectInventoryPage(view.container, 50, 75, 3);
    expect(view.requests).toEqual([ 0, 50 ]);
    fireEvent.click(control(view.container, 'first'));
    await expectInventoryPage(view.container, 0, 25, 1);
    expect(control(view.container, 'first').disabled).toBe(true);
    expect(control(view.container, 'prev').disabled).toBe(true);
  } finally {
    await view.close();
  }
}

test(
  rowsNftTraversesEveryLocalSubpageBeforeAdvancingTheServerCursor,
  { timeout: 30000 },
  rowsNftTraversesEveryLocalSubpageBeforeAdvancingTheServerCursor,
);
test(
  rowsNftPaginatesATerminalFiftyRecordResponse,
  { timeout: 30000 },
  rowsNftPaginatesATerminalFiftyRecordResponse,
);
test(
  rowsNftResizesAndReturnsToFirstFromALaterServerPage,
  { timeout: 30000 },
  rowsNftResizesAndReturnsToFirstFromALaterServerPage,
);

// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { TokenTransferResponse } from 'types/api/tokenTransfer';
import type { ClusterChainConfig } from 'types/multichain';
import type { PaginationParams } from 'ui/shared/pagination/types';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { erc20 } from 'mocks/tokens/tokenTransfer';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';

import useTokenTransferPages, { getTokenTransferTitle } from './useTokenTransferPages';

const records = Array.from({ length: 103 }, (_, index) => ({ ...erc20, log_index: String(index + 1) }));
const hash = erc20.token!.address_hash;
const requests: Array<string> = [];
const clients: Array<QueryClient> = [];
let chain: ClusterChainConfig;

const server = createServer((request, response) => {
  const url = new URL(request.url!, 'http://localhost');
  requests.push(url.pathname + url.search);
  response.setHeader('Content-Type', 'application/json');

  if (url.pathname === `/api/v2/tokens/${ hash }`) {
    response.end(JSON.stringify(erc20.token));
    return;
  }

  if (url.pathname !== `/api/v2/tokens/${ hash }/transfers`) {
    response.writeHead(404).end(JSON.stringify({ error: 'unknown resource' }));
    return;
  }

  const index = Number(url.searchParams.get('index') ?? 0);
  if (![ 0, 50, 100 ].includes(index)) {
    response.writeHead(400).end(JSON.stringify({ error: 'unknown cursor' }));
    return;
  }

  const payload: TokenTransferResponse = {
    items: records.slice(index, index + 50),
    next_page_params: index + 50 < records.length ? { block_number: 12345, index: index + 50, items_count: 50 } : null,
  };
  response.end(JSON.stringify(payload));
});

beforeAll(async() => {
  fetchMock.disableMocks();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const port = String((server.address() as AddressInfo).port);
  const endpoint = `http://127.0.0.1:${ port }`;
  chain = {
    id: '1',
    name: 'Transfer fixture',
    slug: 'transfer-fixture',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        general: { endpoint, host: '127.0.0.1', port, protocol: 'http', socketEndpoint: `ws://127.0.0.1:${ port }` },
      },
    },
  };
});

beforeEach(() => {
  requests.length = 0;
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

function QueryProvider({ children }: { children: React.ReactNode }) {
  const [ client ] = React.useState(() => {
    const value = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false } } });
    clients.push(value);
    return value;
  });
  return React.createElement(QueryClientProvider, { client, children });
}

// The cursor history is an in-memory store; the production API hook transports each response
// through React Query into the exact paging hook consumed by TokenTransfer.
function useTransferFixture() {
  const [ cursors, setCursors ] = React.useState<Array<TokenTransferResponse['next_page_params']>>([ null ]);
  const [ page, setPage ] = React.useState(1);
  const query = useApiQuery('general:token_transfers', {
    chain,
    pathParams: { hash },
    queryParams: cursors[page - 1] ?? undefined,
  });
  const token = useApiQuery('general:token', { chain, pathParams: { hash } });
  const pagination: PaginationParams = {
    page,
    hasPages: page > 1,
    hasNextPage: Boolean(query.data?.next_page_params),
    canGoBackwards: page > 1,
    isLoading: query.isPending,
    isVisible: page > 1 || Boolean(query.data?.next_page_params),
    onNextPageClick: () => {
      const nextPageParams = query.data?.next_page_params;
      if (nextPageParams) {
        setCursors((previous) => [ ...previous.slice(0, page), nextPageParams ]);
        setPage(page + 1);
      }
    },
    onPrevPageClick: () => setPage(page - 1),
    resetPage: () => setPage(1),
  };
  const view = useTokenTransferPages(query.data?.items, pagination);
  return { ...view, query, token, serverPage: page };
}

async function openTransfers() {
  const view = renderHook(useTransferFixture, { wrapper: QueryProvider });
  await waitFor(() => {
    expect(view.result.current.query.isSuccess).toBe(true);
    expect(view.result.current.token.isSuccess).toBe(true);
  });
  return view;
}

async function rowsTokenPreservesEveryRecord() {
  const { result } = await openTransfers();
  expect(result.current.items).toEqual(records.slice(0, 50));
  act(() => result.current.onRowsChange(25));

  const seen: Array<string> = [];
  for (let page = 1; page <= 5; page++) {
    await waitFor(() => expect(result.current.query.isSuccess).toBe(true));
    expect(result.current.pagination.page).toBe(page);
    const expected = records.slice((page - 1) * 25, page * 25);
    expect(result.current.items).toEqual(expected);
    seen.push(...result.current.items!.map((item) => item.log_index));
    if (page < 5) {
      act(() => result.current.pagination.onNextPageClick());
    }
    if (page === 1) {
      expect(requests.filter((url) => url.includes('/transfers'))).toHaveLength(1);
    }
  }

  expect(seen).toEqual(records.map((item) => item.log_index));
  expect(new Set(seen).size).toBe(103);
  expect(result.current.pagination.hasNextPage).toBe(false);
  expect(requests.filter((url) => url.includes('/transfers'))).toEqual([
    `/api/v2/tokens/${ hash }/transfers`,
    `/api/v2/tokens/${ hash }/transfers?block_number=12345&index=50&items_count=50`,
    `/api/v2/tokens/${ hash }/transfers?block_number=12345&index=100&items_count=50`,
  ]);

  act(() => result.current.pagination.onPrevPageClick());
  await waitFor(() => expect(result.current.items).toEqual(records.slice(75, 100)));
  expect(result.current.pagination.page).toBe(4);
  act(() => result.current.pagination.onPrevPageClick());
  expect(result.current.items).toEqual(records.slice(50, 75));
  act(() => result.current.pagination.onPrevPageClick());
  await waitFor(() => expect(result.current.items).toEqual(records.slice(25, 50)));
  act(() => result.current.pagination.resetPage());
  expect(result.current.items).toEqual(records.slice(0, 25));
  expect(result.current.pagination.page).toBe(1);
  expect(result.current.pagination.canGoBackwards).toBe(false);
}

test('rowsTokenPreservesEveryRecord', rowsTokenPreservesEveryRecord);

async function totalTransferLabelsTheFinalPage() {
  const { result } = await openTransfers();
  expect(getTokenTransferTitle(result.current.items!.length)).toBe('50 token transfers on this page');
  act(() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.items).toEqual(records.slice(50, 100)));
  act(() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.items).toEqual(records.slice(100)));
  expect(result.current.serverPage).toBe(3);
  expect(result.current.pagination.hasNextPage).toBe(false);
  expect(getTokenTransferTitle(result.current.items!.length)).toBe('3 token transfers on this page');
  expect(getTokenTransferTitle(result.current.items!.length, 103)).toBe('Latest 3 from a total of 103 token transfers');
  expect(getTokenTransferTitle(0)).toBe('0 token transfers on this page');
  expect(getTokenTransferTitle(3, 3)).toBe('A total of 3 token transfers found');
}

test('totalTransferLabelsTheFinalPage', totalTransferLabelsTheFinalPage);

async function rowsTokenLargerSelectionKeepsServerRowsReachable() {
  const { result } = await openTransfers();
  act(() => result.current.onRowsChange(100));
  expect(result.current.items).toEqual(records.slice(0, 50));
  act(() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.items).toEqual(records.slice(50, 100)));
  act(() => result.current.pagination.onNextPageClick());
  await waitFor(() => expect(result.current.items).toEqual(records.slice(100)));
  expect(result.current.pagination.hasNextPage).toBe(false);
  act(() => result.current.onRowsChange(25));
  await waitFor(() => expect(result.current.items).toEqual(records.slice(0, 25)));
  expect(result.current.pagination.page).toBe(1);
}

test('rowsTokenLargerSelectionKeepsServerRowsReachable', rowsTokenLargerSelectionKeepsServerRowsReachable);

async function launchpadEventProofStopsAtTheExplorerApiBoundary() {
  const { result } = await openTransfers();
  expect(result.current.token.data).toEqual(erc20.token);
  expect(result.current.items).toEqual(records.slice(0, 50));
  expect(result.current.items![0]).toMatchObject({
    from: erc20.from,
    to: erc20.to,
    token: erc20.token,
    total: erc20.total,
    timestamp: erc20.timestamp,
    block_number: erc20.block_number,
    transaction_hash: erc20.transaction_hash,
  });
  expect(requests).toContain(`/api/v2/tokens/${ hash }`);
  expect(requests).toContain(`/api/v2/tokens/${ hash }/transfers`);
  act(() => result.current.onRowsChange(25));
  act(() => result.current.pagination.onNextPageClick());
  expect(result.current.items).toEqual(records.slice(25, 50));
  expect(getTokenTransferTitle(result.current.items!.length)).toBe('25 token transfers on this page');
}

test('launchpadEventProofStopsAtTheExplorerApiBoundary', launchpadEventProofStopsAtTheExplorerApiBoundary);

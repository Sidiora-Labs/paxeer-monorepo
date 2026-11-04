// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { retry as retryPolicy } from 'lib/api/useQueryClientConfig';
import { campNetworkClusterByName } from 'mocks/clusters/cluster';
import { afterAll, afterEach, beforeAll, expect, test as vitestTest } from 'vitest';

import type useSearchWithClustersType from './useSearchWithClusters';

let useSearchWithClusters: typeof useSearchWithClustersType;
let retry: typeof retryPolicy;
let origin: string;
const originalEnvs = window.__envs;
const requests = new Map<string, number>();
const closed = new Set<string>();
const clients: Array<QueryClient> = [];
const errorCodes = { NOT_FOUND: -32004, UNAUTHORIZED: -32001, SERVICE_UNAVAILABLE: -32603 } as const;

function test(proof: () => Promise<void>) {
  vitestTest(proof.name, proof);
}

function errorPayload(code: keyof typeof errorCodes, path = 'names.get') {
  return { error: { message: code, code: errorCodes[code], data: { code, path } } };
}

const server = createServer((request, response) => {
  const url = new URL(request.url || '/', origin);
  response.setHeader('Content-Type', 'application/json');

  if (url.pathname === '/api/v2/search/quick') {
    response.end('[]');
    return;
  }
  if (url.pathname === '/api/v2/search/check-redirect') {
    response.end(JSON.stringify({ redirect: false }));
    return;
  }
  if (url.pathname !== '/v1/trpc/names.get') {
    response.writeHead(404).end(JSON.stringify({ message: 'Unexpected route' }));
    return;
  }

  const { name } = JSON.parse(url.searchParams.get('input') || '{}') as { name: string };
  const count = (requests.get(name) || 0) + 1;
  requests.set(name, count);

  if (name.startsWith('abort/')) {
    response.on('close', () => closed.add(name));
    return;
  }
  if (name === 'network/failure') {
    request.socket.destroy();
    return;
  }
  if (name === 'unknown/name') {
    response.writeHead(404).end(JSON.stringify(errorPayload('NOT_FOUND')));
    return;
  }
  if (name === 'wrong/procedure') {
    response.writeHead(404).end(JSON.stringify(errorPayload('NOT_FOUND', 'names.other')));
    return;
  }
  if (name === 'wrong/code') {
    response.writeHead(404).end(JSON.stringify(errorPayload('UNAUTHORIZED')));
    return;
  }
  if (name === 'missing/route') {
    response.writeHead(404).end(JSON.stringify({ message: 'Route not found' }));
    return;
  }
  if (name === 'unauthorized/name') {
    response.writeHead(401).end(JSON.stringify(errorPayload('UNAUTHORIZED')));
    return;
  }
  if (name === 'unavailable/name' || (name === 'retry/name' && count < 3)) {
    response.setHeader('x-ratelimit-reset', '12345');
    response.writeHead(503).end(JSON.stringify(errorPayload('SERVICE_UNAVAILABLE')));
    return;
  }
  response.end(JSON.stringify(campNetworkClusterByName));
});

beforeAll(async() => {
  fetchMock.disableMocks();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  origin = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_PORT: String((server.address() as AddressInfo).port),
    NEXT_PUBLIC_API_BASE_PATH: '',
    NEXT_PUBLIC_CLUSTERS_API_HOST: origin,
    NEXT_PUBLIC_APP_ENV: 'testing',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
    NEXT_PUBLIC_MULTICHAIN_ENABLED: 'false',
    NEXT_PUBLIC_ZETACHAIN_SERVICE_API_HOST: '',
  };
  ({ 'default': useSearchWithClusters } = await import('./useSearchWithClusters'));
  ({ retry } = await import('lib/api/useQueryClientConfig'));
});

afterEach(() => {
  cleanup();
  clients.forEach((client) => client.clear());
  clients.length = 0;
  requests.clear();
  closed.clear();
});

afterAll(async() => {
  window.__envs = originalEnvs;
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

function search(name: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry, retryDelay: 0, refetchOnWindowFocus: false } },
  });
  clients.push(client);
  function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client }, children);
  }
  const hook = renderHook(() => useSearchWithClusters(), { wrapper: Wrapper });
  act(() => hook.result.current.handleSearchTermChange(`${ name }/`));
  return { ...hook, client };
}

async function clusterUnavailableRemainsTypedAfterRetries() {
  const { result, client } = search('unavailable/name');
  await waitFor(() => expect(result.current.query.isError).toBe(true));
  expect(result.current.query.error).toEqual({
    status: 503,
    statusText: 'Service Unavailable',
    payload: errorPayload('SERVICE_UNAVAILABLE'),
    rateLimits: { bypassOptions: null, reset: '12345' },
  });
  expect(result.current.query.data).toBeUndefined();
  expect(result.current.query.failureCount).toBe(3);
  expect(requests.get('unavailable/name')).toBe(3);
  expect(client.getQueryState([ 'clusters:get_cluster_by_name', { input: 'unavailable/name' } ])?.status).toBe('error');
}

async function clusterUnauthorizedIsRefusedWithoutRetry() {
  const { result } = search('unauthorized/name');
  await waitFor(() => expect(result.current.query.isError).toBe(true));
  expect(result.current.query.error).toMatchObject({
    status: 401, statusText: 'Unauthorized', payload: errorPayload('UNAUTHORIZED'),
  });
  expect(result.current.query.data).toBeUndefined();
  expect(result.current.query.failureCount).toBe(1);
  expect(requests.get('unauthorized/name')).toBe(1);
}

async function clusterUnknownNameIsAnEmptySuccess() {
  const { result, client } = search('unknown/name');
  await waitFor(() => expect(result.current.debouncedSearchTerm).toBe('unknown/name/'));
  await waitFor(() => expect(result.current.query.isSuccess).toBe(true));
  expect(result.current.query.data).toEqual([]);
  expect(result.current.query.error).toBeNull();
  expect(result.current.query.failureCount).toBe(0);
  expect(requests.get('unknown/name')).toBe(1);
  expect(client.getQueryData([ 'clusters:get_cluster_by_name', { input: 'unknown/name' } ])).toBeNull();
}

async function clusterOtherNotFoundResponsesRemainErrors() {
  for (const name of [ 'missing/route', 'wrong/procedure', 'wrong/code' ]) {
    const { result, unmount } = search(name);
    await waitFor(() => expect(result.current.query.isError).toBe(true));
    expect(result.current.query.error).toMatchObject({ status: 404 });
    expect(result.current.query.data).toBeUndefined();
    expect(requests.get(name)).toBe(1);
    unmount();
  }
}

async function clusterRetryRecoversThroughTheRealSelector() {
  const { result } = search('retry/name');
  await waitFor(() => expect(result.current.debouncedSearchTerm).toBe('retry/name/'));
  await waitFor(() => expect(result.current.query.isSuccess).toBe(true));
  const cluster = campNetworkClusterByName.result.data;
  expect(requests.get('retry/name')).toBe(3);
  expect(result.current.query.error).toBeNull();
  expect(result.current.query.data).toEqual([ {
    type: 'cluster',
    name: cluster.name,
    address_hash: cluster.owner,
    is_smart_contract_verified: false,
    is_smart_contract_address: false,
    cluster_info: {
      cluster_id: cluster.clusterId,
      name: cluster.name,
      owner: cluster.owner,
      created_at: cluster.createdAt,
      expires_at: cluster.expiresAt,
      total_wei_amount: cluster.backingWei,
      is_testnet: cluster.isTestnet,
    },
  } ]);
}

async function clusterNetworkFailureIsRetriedAndPropagated() {
  const { result } = search('network/failure');
  await waitFor(() => expect(result.current.query.isError).toBe(true));
  expect(result.current.query.error).toMatchObject({ name: 'TypeError', message: 'fetch failed' });
  expect(result.current.query.data).toBeUndefined();
  expect(result.current.query.failureCount).toBe(3);
  expect(requests.get('network/failure')).toBe(3);
}

async function clusterCancellationAbortsTransportWithoutCachingEmptiness() {
  const name = 'abort/cancel';
  const { client, result } = search(name);
  await waitFor(() => expect(requests.get(name)).toBe(1));
  const queryKey = [ 'clusters:get_cluster_by_name', { input: name } ];
  await act(async() => {
    await client.cancelQueries({ queryKey });
  });
  await waitFor(() => expect(closed.has(name)).toBe(true));
  expect(client.getQueryState(queryKey)).toMatchObject({ status: 'pending', fetchStatus: 'idle', data: undefined, error: null });
  expect(result.current.query.isSuccess).toBe(false);
  expect(requests.get(name)).toBe(1);
}

async function clusterQueryFunctionPreservesAbortRejection() {
  const name = 'abort/rejection';
  const { client } = search(name);
  await waitFor(() => expect(requests.get(name)).toBe(1));
  const queryKey = [ 'clusters:get_cluster_by_name', { input: name } ];
  const queryFn = client.getQueryCache().find({ queryKey })?.options.queryFn;
  if (typeof queryFn !== 'function') throw new Error('Production cluster queryFn was not registered');
  const controller = new AbortController();
  const pending = queryFn({ queryKey, signal: controller.signal, meta: undefined });
  const rejection = (async() => {
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  })();
  await waitFor(() => expect(requests.get(name)).toBe(2));
  controller.abort();
  await rejection;
  await act(async() => {
    await client.cancelQueries({ queryKey });
  });
  await waitFor(() => expect(closed.has(name)).toBe(true));
  expect(client.getQueryData(queryKey)).toBeUndefined();
}

test(clusterUnavailableRemainsTypedAfterRetries);
test(clusterUnauthorizedIsRefusedWithoutRetry);
test(clusterUnknownNameIsAnEmptySuccess);
test(clusterOtherNotFoundResponsesRemainErrors);
test(clusterRetryRecoversThroughTheRealSelector);
test(clusterNetworkFailureIsRetriedAndPropagated);
test(clusterCancellationAbortsTransportWithoutCachingEmptiness);
test(clusterQueryFunctionPreservesAbortRejection);

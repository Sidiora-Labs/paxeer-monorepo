// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { Address, AddressCounters } from 'types/api/address';
import type { ClusterChainConfig } from 'types/multichain';

import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

type Activity = { from: string; to: string; transfers: number };

// Both HTTP indexed totals and JSON-RPC reads are produced from this account store.
class AddressProducer {
  activities: Array<Activity> = [];
  balances = new Map<string, bigint>();
  httpStatus = 200;
  rpcUnavailable = false;
  requests: Array<{ path: string; method?: string; address?: string }> = [];

  counters(hash: string): AddressCounters {
    const related = this.activities.filter(item => item.from === hash || item.to === hash);
    return {
      transactions_count: String(related.length),
      token_transfers_count: String(related.reduce((sum, item) => sum + item.transfers, 0)),
      gas_usage_count: null,
      validations_count: null,
    };
  }

  nonce(hash: string) {
    return this.activities.filter(item => item.from === hash).length;
  }

  address(hash: string): Address {
    return {
      hash, coin_balance: (this.balances.get(hash) ?? BigInt(0)).toString(),
      block_number_balance_updated_at: 123, creator_address_hash: null,
      creation_transaction_hash: null, creation_status: null, exchange_rate: null,
      ens_domain_name: null, has_logs: false,
      has_token_transfers: this.activities.some(item => item.transfers > 0 && (item.from === hash || item.to === hash)),
      has_tokens: false, has_validated_blocks: false, implementations: null,
      is_contract: false, is_verified: false, name: 'Indexed account', token: null,
      watchlist_address_id: null, private_tags: [], public_tags: [], watchlist_names: [],
    };
  }

  reset() {
    this.activities = [];
    this.balances.clear();
    this.requests = [];
    this.httpStatus = 200;
    this.rpcUnavailable = false;
  }
}

const HASH = '0x1e7a5b0b0d3f4d5e6f708192a3b4c5d6e7f80912';
const OTHER = '0x2e7a5b0b0d3f4d5e6f708192a3b4c5d6e7f80912';
const root = new AddressProducer();
const first = new AddressProducer();
const second = new AddressProducer();
const producers = { root, first, second };
const clients = new Set<QueryClient>();
const server = createServer(async(request, response) => {
  response.setHeader('Content-Type', 'application/json');
  response.setHeader('Connection', 'close');
  const path = new URL(request.url!, 'http://localhost').pathname;
  const tenant = path.split('/')[1];
  const producer = producers[tenant as keyof typeof producers];
  if (!producer) {
    response.statusCode = 404;
    response.end(JSON.stringify({ error: 'Unknown tenant' }));
    return;
  }
  if (request.method === 'POST' && path.endsWith('/rpc')) {
    let body = '';
    for await (const chunk of request) {
      body += chunk;
    }
    const call = JSON.parse(body) as { id: number; method: string; params: [string, string] };
    const hash = call.params[0];
    producer.requests.push({ path, method: call.method, address: hash });
    if (producer.rpcUnavailable) {
      response.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, error: { code: -32000, message: 'Account store unavailable' } }));
      return;
    }
    const value = call.method === 'eth_getTransactionCount' ? BigInt(producer.nonce(hash)) : producer.balances.get(hash) ?? BigInt(0);
    if (call.method !== 'eth_getBalance' && call.method !== 'eth_getTransactionCount') {
      response.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, error: { code: -32601, message: 'Unsupported method' } }));
      return;
    }
    response.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, result: `0x${ value.toString(16) }` }));
    return;
  }
  producer.requests.push({ path });
  const route = path.match(/^\/(?:root|first|second)\/api\/v2\/addresses\/(0x[0-9a-fA-F]{40})(?:\/counters)?$/);
  if (!route) {
    response.statusCode = 404;
    response.end(JSON.stringify({ error: 'Unknown address route' }));
    return;
  }
  response.statusCode = producer.httpStatus;
  const hash = route[1];
  const data = path.endsWith('/counters') ? producer.counters(hash) : producer.address(hash);
  response.end(JSON.stringify(producer.httpStatus === 200 ? data : { message: 'Indexer unavailable' }));
});
await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;
window.__envs = {
  ...window.__envs,
  NEXT_PUBLIC_APP_ENV: 'testing',
  NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String((server.address() as AddressInfo).port),
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_BASE_PATH: '/root',
  NEXT_PUBLIC_NETWORK_ID: '125',
  NEXT_PUBLIC_NETWORK_RPC_URL: `${ origin }/root/rpc`,
};
fetchMock.disableMocks();
const { 'default': config } = await import('configs/app');
const { MultichainContext } = await import('lib/contexts/multichain');
const { 'default': useAddressCountersQuery } = await import('./useAddressCountersQuery');
const { 'default': useAddressQuery } = await import('./useAddressQuery');

function tenantChain(id: string, name: string, rpc = true): ClusterChainConfig {
  return {
    id, name, slug: name,
    app_config: {
      ...config,
      chain: { ...config.chain, id, rpcUrls: rpc ? [ `${ origin }/${ name }/rpc` ] : [] },
      apis: {
        ...config.apis,
        general: { endpoint: origin, basePath: `/${ name }`, host: '127.0.0.1', protocol: 'http', socketEndpoint: origin },
      },
    },
  };
}

function provider(chain?: ClusterChainConfig, client = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
})) {
  clients.add(client);
  return {
    client,
    wrapper: ({ children }: { children: React.ReactNode }) => React.createElement(QueryClientProvider, { client },
      React.createElement(MultichainContext.Provider, { value: chain ? { chain } : null }, children)),
  };
}

beforeEach(() => {
  Object.values(producers).forEach(producer => producer.reset());
});
afterEach(async() => {
  cleanup();
  await Promise.all(Array.from(clients, client => client.cancelQueries()));
  clients.forEach(client => client.clear());
  clients.clear();
});
afterAll(async() => {
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

async function counterFallbackTenantIsolation() {
  first.httpStatus = second.httpStatus = 501;
  first.activities = [ { from: HASH, to: OTHER, transfers: 0 } ];
  second.activities = [ ...first.activities, ...first.activities ];
  const shared = provider(tenantChain('126', 'first'));
  const a = renderHook(() => useAddressCountersQuery({ hash: HASH }), shared);
  const b = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('127', 'second') }), shared);
  await waitFor(() => {
    expect(a.result.current.outgoingTransactionCount).toBe('1');
    expect(b.result.current.outgoingTransactionCount).toBe('2');
  });
  expect(a.result.current.data).toBeUndefined();
  expect(b.result.current.data).toBeUndefined();
  const rpcKeys = shared.client.getQueryCache().findAll({ queryKey: [ 'RPC', 'address_counters' ] });
  expect(rpcKeys).toHaveLength(2);
  expect(root.requests).toEqual([]);
  expect(first.requests.some(item => item.method === 'eth_getTransactionCount' && item.address === HASH)).toBe(true);
  expect(second.requests.some(item => item.method === 'eth_getTransactionCount' && item.address === HASH)).toBe(true);
}
test(counterFallbackTenantIsolation, counterFallbackTenantIsolation);

async function counterFallbackRefusesMissingChainRpc() {
  first.httpStatus = 501;
  const view = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('126', 'first', false) }), provider());
  await waitFor(() => expect(view.result.current.rpcError?.message).toBe('No public RPC configured for chain 126'));
  expect(view.result.current.outgoingTransactionCount).toBeUndefined();
  expect(view.result.current.data).toBeUndefined();
  expect(root.requests).toEqual([]);
  expect(first.requests.filter(item => item.method)).toEqual([]);
}
test(counterFallbackRefusesMissingChainRpc, counterFallbackRefusesMissingChainRpc);

async function counterFallbackPreservesIncomingActivityAndTransfers() {
  first.activities = [ { from: OTHER, to: HASH, transfers: 2 } ];
  const view = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('126', 'first') }), provider());
  await waitFor(() => expect(view.result.current.data).toEqual(first.counters(HASH)));
  expect(view.result.current.data?.transactions_count).toBe('1');
  expect(view.result.current.data?.token_transfers_count).toBe('2');
  first.httpStatus = 501;
  await act(async() => {
    await view.result.current.refetch();
  });
  await waitFor(() => expect(view.result.current.outgoingTransactionCount).toBe('0'));
  // Retained indexed data remains an indexed observation; nonce cannot overwrite either total.
  expect(view.result.current.data).toEqual(first.counters(HASH));
  expect(view.result.current.isError).toBe(true);
  view.unmount();
  const outage = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('126', 'first') }), provider());
  await waitFor(() => expect(outage.result.current.outgoingTransactionCount).toBe('0'));
  expect(outage.result.current.data).toBeUndefined();
  expect(outage.result.current.isDegradedData).toBe(true);
}
test(counterFallbackPreservesIncomingActivityAndTransfers, counterFallbackPreservesIncomingActivityAndTransfers);

async function counterFallbackSurfacesRpcFailure() {
  first.httpStatus = 501;
  first.rpcUnavailable = true;
  const view = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('126', 'first') }), provider());
  await waitFor(() => expect(view.result.current.rpcError?.message).toContain('Account store unavailable'));
  expect(view.result.current.data).toBeUndefined();
  expect(view.result.current.outgoingTransactionCount).toBeUndefined();
  expect(view.result.current.isError).toBe(true);
}
test(counterFallbackSurfacesRpcFailure, counterFallbackSurfacesRpcFailure);

async function counterFallbackDistinguishesTrueZero() {
  const view = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('126', 'first') }), provider());
  await waitFor(() => expect(view.result.current.data).toEqual(first.counters(HASH)));
  expect(view.result.current.data?.transactions_count).toBe('0');
  expect(view.result.current.data?.token_transfers_count).toBe('0');
  expect(view.result.current.isDegradedData).toBe(false);
  first.httpStatus = 501;
  const outage = renderHook(() => useAddressCountersQuery({ hash: HASH, chain: tenantChain('126', 'first') }), provider());
  await waitFor(() => expect(outage.result.current.outgoingTransactionCount).toBe('0'));
  expect(outage.result.current.data).toBeUndefined();
}
test(counterFallbackDistinguishesTrueZero, counterFallbackDistinguishesTrueZero);

async function zeroBalanceFallbackAndHttpRecovery() {
  root.httpStatus = 501;
  root.balances.set(HASH, BigInt(0));
  const view = renderHook(() => useAddressQuery({ hash: HASH }), provider());
  await waitFor(() => {
    expect(view.result.current.data?.coin_balance).toBe('0');
    expect(view.result.current.isDegradedData).toBe(true);
  });
  expect(view.result.current.rpcError).toBeNull();
  expect(root.requests.some(item => item.method === 'eth_getBalance' && item.address === HASH)).toBe(true);
  root.balances.set(HASH, BigInt(55));
  root.httpStatus = 200;
  await waitFor(() => {
    expect(view.result.current.data).toEqual(root.address(HASH));
    expect(view.result.current.isDegradedData).toBe(false);
  }, { timeout: 20_000 });
}
test(zeroBalanceFallbackAndHttpRecovery, zeroBalanceFallbackAndHttpRecovery, 25_000);

async function nonzeroBalanceFallback() {
  root.httpStatus = 501;
  root.balances.set(HASH, BigInt('9007199254740993'));
  const view = renderHook(() => useAddressQuery({ hash: HASH }), provider());
  await waitFor(() => expect(view.result.current.data?.coin_balance).toBe('9007199254740993'));
  expect(view.result.current.isDegradedData).toBe(true);
  expect(view.result.current.rpcError).toBeNull();
}
test(nonzeroBalanceFallback, nonzeroBalanceFallback);

async function balanceRpcFailureStillRecoversHttp() {
  root.httpStatus = 501;
  root.rpcUnavailable = true;
  const view = renderHook(() => useAddressQuery({ hash: HASH }), provider());
  await waitFor(() => expect(view.result.current.rpcError?.message).toContain('Account store unavailable'), { timeout: 5_000 });
  expect(view.result.current.data).toBeUndefined();
  expect(view.result.current.isError).toBe(true);
  root.httpStatus = 200;
  root.balances.set(HASH, BigInt(7));
  await waitFor(() => {
    expect(view.result.current.data).toEqual(root.address(HASH));
    expect(view.result.current.isDegradedData).toBe(false);
  }, { timeout: 20_000 });
}
test(balanceRpcFailureStillRecoversHttp, balanceRpcFailureStillRecoversHttp, 30_000);

async function disabledAndForbiddenAddressesDoNotUseRpc() {
  const disabled = renderHook(() => useAddressQuery({ hash: HASH, isEnabled: false }), provider());
  expect(disabled.result.current.isFetching).toBe(false);
  expect(root.requests).toEqual([]);
  root.httpStatus = 403;
  const view = renderHook(() => useAddressQuery({ hash: HASH }), provider());
  await waitFor(() => expect(view.result.current.isError).toBe(true));
  expect(view.result.current.error?.status).toBe(403);
  expect(view.result.current.isDegradedData).toBe(false);
  expect(root.requests.filter(item => item.method)).toEqual([]);
}
test(disabledAndForbiddenAddressesDoNotUseRpc, disabledAndForbiddenAddressesDoNotUseRpc);

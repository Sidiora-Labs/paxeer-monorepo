import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { once } from 'node:events';
import type { IncomingHttpHeaders } from 'node:http';
import { createServer } from 'node:http';
import React from 'react';
import { renderToString } from 'react-dom/server';
import { object, parse, string } from 'valibot';

import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { afterAll, beforeAll, expect, test } from 'vitest';

import buildUrl from './buildUrl';
import getResourceParams from './getResourceParams';
import { PAXEER_X_CHAIN_RESOURCES } from './resources';
import useApiFetch from './useApiFetch';

beforeAll(() => fetchMock.disableMocks());
afterAll(() => fetchMock.enableMocks());

const tenant: ClusterChainConfig = {
  id: '125',
  name: 'Root network',
  slug: 'root',
  app_config: {
    ...config,
    apis: {
      ...config.apis,
      paxeerXIndexer: {
        endpoint: 'https://gateway.example',
        basePath: '/tenant/demo/indexer',
      },
    },
  },
};

function recoverFeedCursor(saved: string) {
  return parse(object({ cursor: string(), id: string() }), JSON.parse(saved));
}

function chainApiUsesDedicatedIndexerForEveryExistingConsumer() {
  for (const [ name, definition ] of Object.entries(PAXEER_X_CHAIN_RESOURCES)) {
    const resourceName = `general:${ name as keyof typeof PAXEER_X_CHAIN_RESOURCES }` as const;
    const { api, apiName, resource } = getResourceParams(resourceName, tenant);
    expect(apiName).toBe('paxeerXIndexer');
    expect(api).toBe(tenant.app_config.apis.paxeerXIndexer);
    expect(api).not.toBe(tenant.app_config.apis.general);
    expect(resource).toBe(definition);
    expect(buildUrl(resourceName, { id: '7' }, undefined, true, tenant))
      .toBe(`https://gateway.example/tenant/demo/indexer${ definition.path.replace(':id', '7') }`);
  }
}

function chainApiSupportsItsExplicitNamespace() {
  for (const name of Object.keys(PAXEER_X_CHAIN_RESOURCES) as Array<keyof typeof PAXEER_X_CHAIN_RESOURCES>) {
    const existing = getResourceParams(`general:${ name }`, tenant);
    const explicit = getResourceParams(`paxeerXIndexer:${ name }`, tenant);
    expect(explicit).toEqual(existing);
    expect(buildUrl(`paxeerXIndexer:${ name }`, { id: '7' }, undefined, true, tenant))
      .toBe(buildUrl(`general:${ name }`, { id: '7' }, undefined, true, tenant));
  }
}

function chainApiRefusesMissingIndexerWithoutUsingBlockscout() {
  const { paxeerXIndexer, ...apis } = tenant.app_config.apis;
  expect(paxeerXIndexer).toBeDefined();
  const unconfigured = { ...tenant, app_config: { ...tenant.app_config, apis } };
  for (const name of Object.keys(PAXEER_X_CHAIN_RESOURCES) as Array<keyof typeof PAXEER_X_CHAIN_RESOURCES>) {
    expect(() => getResourceParams(`general:${ name }`, unconfigured))
      .toThrow('API config for paxeerXIndexer not found');
  }
  expect(getResourceParams('general:block', unconfigured).api).toBe(apis.general);
}

function chainApiPreservesTenantIdentityAcrossAllReads() {
  const otherTenant: ClusterChainConfig = {
    ...tenant,
    id: '126',
    app_config: {
      ...tenant.app_config,
      apis: {
        ...tenant.app_config.apis,
        paxeerXIndexer: { endpoint: 'https://other-gateway.example', basePath: '/tenant/other/indexer' },
      },
    },
  };
  for (const name of Object.keys(PAXEER_X_CHAIN_RESOURCES) as Array<keyof typeof PAXEER_X_CHAIN_RESOURCES>) {
    const resourceName = `general:${ name }` as const;
    const first = new URL(buildUrl(resourceName, { id: '7' }, undefined, true, tenant));
    const second = new URL(buildUrl(resourceName, { id: '7' }, undefined, true, otherTenant));
    expect(first.origin).toBe('https://gateway.example');
    expect(first.pathname).toMatch(/^\/tenant\/demo\/indexer\/v1\/chains/);
    expect(second.origin).toBe('https://other-gateway.example');
    expect(second.pathname).toMatch(/^\/tenant\/other\/indexer\/v1\/chains/);
  }
}

function chainApiReconstructsFeedRequestsFromPersistedCursorUrls() {
  for (const name of [ 'paxeer_x_chain_transfers', 'paxeer_x_chain_events' ] as const) {
    const resourceName = `general:${ name }` as const;
    const initial = new URL(buildUrl(resourceName, { id: '7' }, { cursor: '42', limit: 2, kind: 'transfer' }, true, tenant));
    expect(initial.searchParams.get('cursor')).toBe('42');
    expect(initial.searchParams.get('limit')).toBe('2');
    expect(initial.searchParams.get('kind')).toBe('transfer');
    const saved = JSON.stringify({ cursor: initial.searchParams.get('cursor'), id: '7' });
    const recovered = recoverFeedCursor(saved);
    const resumed = new URL(buildUrl(resourceName, { id: recovered.id }, { cursor: recovered.cursor, limit: 2, kind: 'transfer' }, true, tenant));
    expect(resumed.toString()).toBe(initial.toString());
    expect(resumed.pathname).toBe(`/tenant/demo/indexer/v1/chains/7/${ name.endsWith('events') ? 'events' : 'transfers' }`);
  }
}

function chainApiRoutesUnknownChainIdsToTheSameUpstream() {
  for (const name of [ 'paxeer_x_chain', 'paxeer_x_chain_summary', 'paxeer_x_chain_transfers', 'paxeer_x_chain_events' ] as const) {
    const url = new URL(buildUrl(`general:${ name }`, { id: '4294967295' }, undefined, true, tenant));
    expect(url.origin).toBe('https://gateway.example');
    expect(url.pathname).toBe(`/tenant/demo/indexer${ PAXEER_X_CHAIN_RESOURCES[name].path.replace(':id', '4294967295') }`);
  }
}

const chainDocument = {
  id: 1,
  name: 'demo',
  operator: '0x2222222222222222222222222222222222222222',
  status: 'active',
  chain_id: 125000001,
  created_height: 42,
  bond: '100000000000000000007',
  gas_allowance: 500000,
  byte_allowance: 65536,
  fee_share_bps: 100,
};

const summaryDocument = { id: 1, tx_count: 2, gas_used: 42005, last_height: 44 };

async function openChainTransport(responses: Record<string, { status: number; body: unknown }>) {
  const requests: Array<{ method: string | undefined; target: string | undefined; headers: IncomingHttpHeaders }> = [];
  const server = createServer((request, response) => {
    requests.push({ method: request.method, target: request.url, headers: request.headers });
    const recorded = responses[request.url ?? ''];
    response.writeHead(recorded?.status ?? 500, { 'content-type': 'application/json' });
    response.end(JSON.stringify(recorded?.body ?? { error: 'unexpected_request' }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP address');
  }
  const configured: ClusterChainConfig = {
    ...tenant,
    app_config: {
      ...tenant.app_config,
      apis: {
        ...tenant.app_config.apis,
        paxeerXIndexer: { endpoint: `http://127.0.0.1:${ address.port }`, basePath: '/tenant/demo/indexer' },
      },
    },
  };
  return {
    chain: configured,
    requests,
    close: () => new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

function openChainReader() {
  const client = new QueryClient();
  const state: { read?: ReturnType<typeof useApiFetch> } = {};
  function ChainReader() {
    state.read = useApiFetch();
    return null;
  }
  renderToString(React.createElement(QueryClientProvider, { client, children: React.createElement(ChainReader) }));
  if (!state.read) {
    throw new Error('Chain reader did not mount');
  }
  return { read: state.read, close: () => client.clear() };
}

async function chainApiReadsDirectoryDetailAndSummaryThroughRealTransport() {
  const transport = await openChainTransport({
    '/tenant/demo/indexer/v1/chains': { status: 200, body: { chains: [ chainDocument ] } },
    '/tenant/demo/indexer/v1/chains/1': { status: 200, body: chainDocument },
    '/tenant/demo/indexer/v1/chains/1/summary': { status: 200, body: summaryDocument },
    '/tenant/other/indexer/v1/chains/1': { status: 200, body: { ...chainDocument, name: 'other-demo' } },
  });
  const reader = openChainReader();
  try {
    await expect(reader.read('general:paxeer_x_chains', { chain: transport.chain }))
      .resolves.toEqual({ chains: [ chainDocument ] });
    await expect(reader.read('general:paxeer_x_chain', { chain: transport.chain, pathParams: { id: '1' } }))
      .resolves.toEqual(chainDocument);
    await expect(reader.read('general:paxeer_x_chain_summary', { chain: transport.chain, pathParams: { id: '1' } }))
      .resolves.toEqual(summaryDocument);
    const otherTenant: ClusterChainConfig = {
      ...transport.chain,
      id: '126',
      app_config: {
        ...transport.chain.app_config,
        apis: {
          ...transport.chain.app_config.apis,
          paxeerXIndexer: {
            endpoint: transport.chain.app_config.apis.paxeerXIndexer!.endpoint,
            basePath: '/tenant/other/indexer',
          },
        },
      },
    };
    await expect(reader.read('paxeerXIndexer:paxeer_x_chain', { chain: otherTenant, pathParams: { id: '1' } }))
      .resolves.toEqual({ ...chainDocument, name: 'other-demo' });
    expect(transport.requests.map(request => [ request.method, request.target ])).toEqual([
      [ 'GET', '/tenant/demo/indexer/v1/chains' ],
      [ 'GET', '/tenant/demo/indexer/v1/chains/1' ],
      [ 'GET', '/tenant/demo/indexer/v1/chains/1/summary' ],
      [ 'GET', '/tenant/other/indexer/v1/chains/1' ],
    ]);
    for (const request of transport.requests) {
      expect(request.headers.authorization).toBeUndefined();
      expect(request.headers.cookie).toBeUndefined();
      expect(request.headers['api-v2-temp-token']).toBeUndefined();
    }
  } finally {
    reader.close();
    await transport.close();
  }
}

async function chainApiPropagatesUnknownChainErrorsThroughRealTransport() {
  const names = [ 'paxeer_x_chain', 'paxeer_x_chain_summary', 'paxeer_x_chain_transfers', 'paxeer_x_chain_events' ] as const;
  const transport = await openChainTransport(Object.fromEntries(names.map(name => [
    `/tenant/demo/indexer${ PAXEER_X_CHAIN_RESOURCES[name].path.replace(':id', '999') }`,
    { status: 404, body: { error: 'unknown_chain' } },
  ])));
  const reader = openChainReader();
  try {
    for (const name of names) {
      await expect(reader.read(`general:${ name }`, { chain: transport.chain, pathParams: { id: '999' } }))
        .rejects.toMatchObject({ status: 404, payload: { error: 'unknown_chain' } });
    }
    expect(transport.requests).toHaveLength(names.length);
  } finally {
    reader.close();
    await transport.close();
  }
}

async function chainApiResumesFeedCursorAfterReopeningTheConsumer() {
  for (const name of [ 'paxeer_x_chain_transfers', 'paxeer_x_chain_events' ] as const) {
    const path = `/tenant/demo/indexer${ PAXEER_X_CHAIN_RESOURCES[name].path.replace(':id', '1') }`;
    const firstPage = { version: 1, items: [ { id: '42', chain: 'pax:1' } ], next_cursor: '42' };
    const lastPage = { version: 1, items: [ { id: '43', chain: 'pax:1' } ], next_cursor: null };
    const transport = await openChainTransport({
      [`${ path }?limit=1`]: { status: 200, body: firstPage },
      [`${ path }?limit=1&cursor=42`]: { status: 200, body: lastPage },
    });
    let reader = openChainReader();
    try {
      const first = await reader.read<`general:${ typeof name }`, typeof firstPage>(`general:${ name }`, {
        chain: transport.chain, pathParams: { id: '1' }, queryParams: { limit: 1 },
      });
      expect(first).toEqual(firstPage);
      if (!('next_cursor' in first)) {
        throw new Error('Expected a chain feed page');
      }
      const saved = JSON.stringify({ cursor: first.next_cursor, id: '1' });
      reader.close();
      reader = openChainReader();
      const recovered = recoverFeedCursor(saved);
      await expect(reader.read(`general:${ name }`, {
        chain: transport.chain, pathParams: { id: recovered.id }, queryParams: { limit: 1, cursor: recovered.cursor },
      })).resolves.toEqual(lastPage);
      expect(transport.requests.map(request => request.target)).toEqual([ `${ path }?limit=1`, `${ path }?limit=1&cursor=42` ]);
    } finally {
      reader.close();
      await transport.close();
    }
  }
}

test(chainApiUsesDedicatedIndexerForEveryExistingConsumer);
test(chainApiSupportsItsExplicitNamespace);
test(chainApiRefusesMissingIndexerWithoutUsingBlockscout);
test(chainApiPreservesTenantIdentityAcrossAllReads);
test(chainApiReconstructsFeedRequestsFromPersistedCursorUrls);
test(chainApiRoutesUnknownChainIdsToTheSameUpstream);
test(chainApiReadsDirectoryDetailAndSummaryThroughRealTransport);
test(chainApiPropagatesUnknownChainErrorsThroughRealTransport);
test(chainApiResumesFeedCursorAfterReopeningTheConsumer);

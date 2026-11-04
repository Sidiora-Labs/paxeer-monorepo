import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { once } from 'node:events';
import { createServer } from 'node:http';
import React from 'react';
import { renderToString } from 'react-dom/server';
import { array, literal, nullable, number, parse, strictObject, string, union } from 'valibot';

import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import buildUrl from 'lib/api/buildUrl';
import useApiFetch from 'lib/api/useApiFetch';
import { evmAddress, nativeUnifiedAccount } from 'mocks/paxeerX/unifiedAccount';
import { expect, test } from 'vitest';

import { GENERAL_API_PAXEER_X_RESOURCES } from './paxeerX';
import './paxeerX.spec';
import './paxeerXSignature.spec';

const assetSchema = strictObject({
  id: string(),
  denom: string(),
  symbol: nullable(string()),
  decimals: nullable(number()),
});

const nativeAccountSchema = strictObject({
  identities: strictObject({ evm: string(), pax: nullable(string()), did: nullable(string()) }),
  balances: array(strictObject({ asset: assetSchema, total: string() })),
  activity: array(strictObject({
    kind: literal('transaction'),
    hash: string(),
    block_number: number(),
    ordinal: number(),
    status: literal('committed'),
    result: nullable(union([ literal('ok'), literal('error') ])),
    timestamp: string(),
    asset: assetSchema,
    amount: string(),
    counterparty: nullable(string()),
  })),
  next_page_params: nullable(strictObject({ cursor: string() })),
  page_cursor: string(),
  first_page_cursor: string(),
  page_number: number(),
  activity_total: nullable(number()),
});

async function unifiedAccountResourcePreservesNativePayloadAndOpaqueCursor() {
  expect(GENERAL_API_PAXEER_X_RESOURCES.paxeer_x_unified_account.filterFields).toEqual([ 'cursor' ]);
  const requests: Array<{ method: string | undefined; target: string | undefined }> = [];
  const pathname = `/explorer/backend/api/v2/addresses/${ evmAddress }/unified`;
  const replayTarget = `${ pathname }?cursor=${ encodeURIComponent(nativeUnifiedAccount.page_cursor) }`;
  const server = createServer((request, response) => {
    requests.push({ method: request.method, target: request.url });
    if (request.method !== 'GET' || (request.url !== pathname && request.url !== replayTarget)) {
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ message: 'Unexpected native account replay request' }));
      return;
    }
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify(nativeUnifiedAccount));
  });
  const client = new QueryClient();
  const consumer: { read?: ReturnType<typeof useApiFetch> } = {};
  function UnifiedAccountConsumer() {
    consumer.read = useApiFetch();
    return null;
  }

  fetchMock.disableMocks();
  try {
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('Expected a loopback TCP address');
    }
    const chain: ClusterChainConfig = {
      id: '125', name: 'Native account fixture', slug: 'native-account-fixture',
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
            basePath: '/explorer/backend',
          },
        },
      },
    };
    renderToString(React.createElement(QueryClientProvider, { client, children: React.createElement(UnifiedAccountConsumer) }));
    if (!consumer.read) {
      throw new Error('Native account consumer did not mount');
    }
    const first = parse(nativeAccountSchema, await consumer.read('general:paxeer_x_unified_account', {
      chain, pathParams: { hash: evmAddress },
    }));
    expect(first).toEqual(nativeUnifiedAccount);
    expect(first.balances.map(({ asset, total }) => [ asset.id, total ])).toEqual([
      [ '0x2170Ed0880ac9A755fd29B2688956BD959F933F8', '2500000' ],
      [ 'native', '1000000000000000000' ],
    ]);
    expect(first.activity.map(({ kind, status, result }) => ({ kind, status, result }))).toEqual([
      { kind: 'transaction', status: 'committed', result: 'error' },
      { kind: 'transaction', status: 'committed', result: 'ok' },
    ]);
    const queryParams = { cursor: first.page_cursor };
    const url = new URL(buildUrl('general:paxeer_x_unified_account', { hash: evmAddress }, queryParams, true, chain));
    expect(url.pathname).toBe(pathname);
    expect(url.searchParams.get('cursor')).toBe(nativeUnifiedAccount.page_cursor);
    const replayed = parse(nativeAccountSchema, await consumer.read('general:paxeer_x_unified_account', {
      chain, pathParams: { hash: evmAddress }, queryParams,
    }));
    expect(replayed).toEqual(first);
    expect(requests).toEqual([
      { method: 'GET', target: pathname },
      { method: 'GET', target: replayTarget },
    ]);
  } finally {
    client.clear();
    fetchMock.enableMocks();
    await new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  }
}

test(unifiedAccountResourcePreservesNativePayloadAndOpaqueCursor);

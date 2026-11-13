// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { once } from 'node:events';
import { createServer } from 'node:http';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { PaxeerXNativeActivityItem, PaxeerXNativeUnifiedAccount } from 'types/api/paxeerX';
import type { FeedMarketsResponse, PriceUpdateItem } from 'types/api/paxeerXFeed';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import useApiFetch from 'lib/api/useApiFetch';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import ActivityListItem from 'ui/paxeerX/account/ActivityListItem';
import AssetListItem from 'ui/paxeerX/account/AssetListItem';
import { activityKindLabel, formatAmount, listIdentities } from 'ui/paxeerX/account/utils';
import applyPriceUpdates from 'ui/paxeerX/feed/applyPriceUpdates';
import PaxeerXFeedUpdateDetails from 'ui/paxeerX/feed/PaxeerXFeedUpdateDetails';
import { Wrapper } from 'ui/shared/layout/testWrapper';
import { afterAll, beforeAll, expect, expectTypeOf, test } from 'vitest';

import { failedPriceUpdate, priceUpdate, refusalMarkets, refusedPriceUpdate } from './feed';
import {
  evmAddress,
  nativeAsset,
  nativeUnifiedAccount,
  nativeUnifiedAccountEvmOnly,
  nativeUnifiedAccountUnknownResult,
} from './unifiedAccount';

import 'lib/setLocale';

beforeAll(() => fetchMock.disableMocks());
afterAll(() => fetchMock.enableMocks());

async function openTransport(documents: Record<string, unknown>) {
  const requests: Array<string> = [];
  const server = createServer((request, response) => {
    const target = request.url ?? '';
    requests.push(`${ request.method } ${ target }`);
    if (!Object.hasOwn(documents, target)) {
      response.writeHead(404, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ message: 'Not found' }));
      return;
    }
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify(documents[target]));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP address');
  }
  const chain: ClusterChainConfig = {
    id: '125',
    name: 'Paxeer X',
    slug: 'paxeer-x',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        general: {
          host: '127.0.0.1',
          protocol: 'http',
          port: String(address.port),
          endpoint: `http://127.0.0.1:${ address.port }`,
          basePath: '',
          socketEndpoint: `ws://127.0.0.1:${ address.port }`,
        },
      },
    },
  };
  return {
    chain,
    requests,
    close: () => new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

function openReader() {
  const client = new QueryClient();
  const state: { read?: ReturnType<typeof useApiFetch> } = {};
  function Reader() {
    state.read = useApiFetch();
    return null;
  }
  renderToStaticMarkup(React.createElement(QueryClientProvider, { client, children: React.createElement(Reader) }));
  if (!state.read) {
    throw new Error('API reader did not render');
  }
  return { read: state.read, close: () => client.clear() };
}

function renderConsumer(element: React.ReactElement) {
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(React.createElement(Wrapper, { children: element }));
  return container;
}

async function readUpdate(reader: ReturnType<typeof openReader>, chain: ClusterChainConfig, hash: string) {
  const data = await reader.read<'general:paxeer_x_feed_update', PriceUpdateItem>(
    'general:paxeer_x_feed_update', { chain, pathParams: { hash } },
  );
  if (!('price_update' in data)) {
    throw new Error('Expected a price update response');
  }
  return data;
}

async function readMarkets(reader: ReturnType<typeof openReader>, chain: ClusterChainConfig) {
  const data = await reader.read<'general:paxeer_x_feed_markets', FeedMarketsResponse>('general:paxeer_x_feed_markets', { chain });
  if (!('items' in data)) {
    throw new Error('Expected a markets response');
  }
  return data;
}

async function failedObservationsStayVisibleInDetailAndNeverAdvanceLiveMarkets() {
  const updatePath = `/api/v2/paxeer-x/feed/updates/${ failedPriceUpdate.hash }`;
  const marketPath = '/api/v2/paxeer-x/feed/markets';
  const transport = await openTransport({ [updatePath]: failedPriceUpdate, [marketPath]: refusalMarkets });
  const reader = openReader();
  try {
    const update = await readUpdate(reader, transport.chain, failedPriceUpdate.hash);
    const previous = await readMarkets(reader, transport.chain);
    expect(update.status).toBe('error');
    expect(update.result).toBe('failed');
    expect(update.price_update.observation_count).toBe(2);
    expect(update.price_update.observations).toHaveLength(2);
    expect(update.price_update.observations.map(observation => observation.accepted)).toEqual([ false, false ]);
    expect(update.price_update.observations.map(observation => observation.market_id)).toEqual([ 0, 1 ]);
    expect(update.block_number).toBeGreaterThan(previous.chain_head);

    const detail = renderConsumer(React.createElement(PaxeerXFeedUpdateDetails, { data: update }));
    expect(detail.querySelector('[data-status="error"]')?.textContent).toBe('Failed');
    expect(detail.querySelector('[data-field="hash"]')?.textContent).toBe(update.hash);
    expect(Array.from(detail.querySelectorAll('[data-observation]')).map(row => row.getAttribute('data-observation'))).toEqual([ '0', '1' ]);
    expect(detail.querySelector('[data-observation="0"] [data-price]')?.getAttribute('data-price')).toBe('67012500000000000000000');

    const live = applyPriceUpdates(previous, [ update ]);
    expect(live).toBe(previous);
    expect(live.items[0].latest?.update_hash).toBe(priceUpdate.hash);
    expect(live.items[2].latest).toBeNull();
    expect(transport.requests).toEqual([ `GET ${ updatePath }`, `GET ${ marketPath }` ]);
  } finally {
    reader.close();
    await transport.close();
  }
}

async function successfulSubmissionKeepsRefusedObservationInDetailAndAppliesOnlyAcceptedMarket() {
  const updatePath = `/api/v2/paxeer-x/feed/updates/${ refusedPriceUpdate.hash }`;
  const transport = await openTransport({
    [updatePath]: refusedPriceUpdate,
    '/api/v2/paxeer-x/feed/markets': refusalMarkets,
  });
  const reader = openReader();
  try {
    const update = await readUpdate(reader, transport.chain, refusedPriceUpdate.hash);
    const previous = await readMarkets(reader, transport.chain);
    expect(update.status).toBe('ok');
    expect(update.result).toBe('success');
    expect(update.price_update.observations.map(observation => [ observation.market_id, observation.accepted ]))
      .toEqual([ [ 0, false ], [ 1, true ] ]);
    expect(update.block_number).toBeGreaterThan(previous.chain_head);

    const detail = renderConsumer(React.createElement(PaxeerXFeedUpdateDetails, { data: update }));
    expect(detail.querySelector('[data-status="ok"]')?.textContent).toBe('Success');
    expect(detail.querySelectorAll('[data-observation]')).toHaveLength(2);
    expect(detail.querySelector('[data-observation="0"] [data-price]')?.getAttribute('data-price')).toBe('67500000000000000000000');
    expect(detail.querySelector('[data-observation="1"] [data-price]')?.getAttribute('data-price')).toBe('3500000000000000000000');

    const live = applyPriceUpdates(previous, [ update ]);
    expect(live.items[0]).toBe(previous.items[0]);
    expect(live.items[0].latest?.index_price).toBe('67012500000000000000000');
    expect(live.items[1]).toBe(previous.items[1]);
    expect(live.items[2].latest).toEqual({
      index: update.index,
      index_price: '3500000000000000000000',
      mark_price: '3501000000000000000000',
      source_timestamp_ms: 1767323057500,
      sequence: 9002,
      update_hash: update.hash,
      block_number: update.block_number,
      timestamp: update.timestamp,
    });
    expect(live.chain_head).toBe(update.block_number);
    expect(applyPriceUpdates(live, [ update ])).toBe(live);
    expect(transport.requests).toEqual([ `GET ${ updatePath }`, 'GET /api/v2/paxeer-x/feed/markets' ]);
  } finally {
    reader.close();
    await transport.close();
  }
}

async function readAccount(document: PaxeerXNativeUnifiedAccount) {
  const path = `/api/v2/addresses/${ evmAddress }/unified`;
  const transport = await openTransport({ [path]: document });
  const reader = openReader();
  try {
    const account = await reader.read<'general:paxeer_x_unified_account', PaxeerXNativeUnifiedAccount>(
      'general:paxeer_x_unified_account', { chain: transport.chain, pathParams: { hash: evmAddress } },
    );
    if (!('activity' in account)) {
      throw new Error('Expected a unified account response');
    }
    expect(transport.requests).toEqual([ `GET ${ path }` ]);
    return account;
  } finally {
    reader.close();
    await transport.close();
  }
}

async function nativeAccountTransactionsPreserveExecutionResultsThroughTransportAndConsumer() {
  expectTypeOf<PaxeerXNativeActivityItem['kind']>().toEqualTypeOf<'transaction'>();
  expectTypeOf<PaxeerXNativeActivityItem['status']>().toEqualTypeOf<'committed'>();
  expectTypeOf<Pick<PaxeerXNativeActivityItem, 'result'>>().toEqualTypeOf<{ result: 'ok' | 'error' | null }>();
  const account = await readAccount(nativeUnifiedAccount);
  expect(account.activity_total).toBe(2);
  expect(account.activity.map(item => [ item.kind, item.status, item.result, item.asset.id ]))
    .toEqual([ [ 'transaction', 'committed', 'error', 'native' ], [ 'transaction', 'committed', 'ok', 'native' ] ]);
  expect(account.activity.map(item => activityKindLabel(item.kind))).toEqual([ 'Transaction', 'Transaction' ]);
  const rows = account.activity.map(item => React.createElement(ActivityListItem, { key: item.hash, item }));
  const content = renderConsumer(React.createElement(TableRoot, { children: React.createElement(TableBody, { children: rows }) }));
  expect(content.querySelectorAll('[data-activity]')).toHaveLength(2);
  expect(Array.from(content.querySelectorAll('[data-scan-method]')).map(chip => chip.getAttribute('data-scan-method')))
    .toEqual([ 'Transaction', 'Transaction' ]);
  expect(content.querySelectorAll('[data-rung="committed"]')).toHaveLength(2);
  expect(content.querySelector('[data-rung="receipted"]')).toBeNull();
  expect(Array.from(content.querySelectorAll('[data-activity]')).map(row => row.querySelectorAll('td')[3]?.textContent))
    .toEqual([ '0 HPX', '1.5 HPX' ]);
  for (const item of account.activity) {
    expect(content.querySelector(`a[href="/tx/${ item.hash }"]`)).toBeTruthy();
    expect(item).toHaveProperty('result');
  }

  const unknown = await readAccount(nativeUnifiedAccountUnknownResult);
  expect(unknown.activity).toHaveLength(1);
  expect(unknown.activity[0]).toHaveProperty('result', null);
  expect(unknown.activity[0].status).toBe('committed');
}

async function emptyNativeAccountIncludesAndRendersZeroBalance() {
  const account = await readAccount(nativeUnifiedAccountEvmOnly);
  expect(account.balances).toEqual([ { asset: nativeAsset, total: '0' } ]);
  expect(account.activity).toEqual([]);
  expect(account.activity_total).toBe(0);
  expect(account.next_page_params).toBeNull();
  expect(listIdentities(account.identities)).toEqual([ { kind: 'evm', label: 'EVM address', value: evmAddress } ]);
  expect(formatAmount(account.balances[0].total, account.balances[0].asset)).toBe('0');
  const content = renderConsumer(React.createElement(TableRoot, {
    children: React.createElement(TableBody, { children: React.createElement(AssetListItem, { item: account.balances[0] }) }),
  }));
  expect(content.querySelectorAll('[data-asset]')).toHaveLength(1);
  expect(content.querySelector('[data-asset="native"] [data-label="total"]')?.textContent).toBe('0');
  expect(content.textContent).toContain('HPX');
}

test(failedObservationsStayVisibleInDetailAndNeverAdvanceLiveMarkets);
test(successfulSubmissionKeepsRefusedObservationInDetailAndAppliesOnlyAcceptedMarket);
test(nativeAccountTransactionsPreserveExecutionResultsThroughTransportAndConsumer);
test(emptyNativeAccountIncludesAndRendersZeroBalance);

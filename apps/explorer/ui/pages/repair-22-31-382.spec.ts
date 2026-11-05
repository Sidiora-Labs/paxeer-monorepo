// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import path from 'node:path';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { FeedMarketsResponse, MarketUpdatesResponse, PriceUpdateItem } from 'types/api/paxeerXFeed';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import buildUrl from 'lib/api/buildUrl';
import { GENERAL_API_PAXEER_X_FEED_RESOURCES } from 'lib/api/services/general/paxeerXFeed';
import useApiFetch from 'lib/api/useApiFetch';
import { marketUpdates, markets, priceUpdate, refusedPriceUpdate } from 'mocks/paxeerX/feed';
import PaxeerXFeedMarketsTable from 'ui/paxeerX/feed/PaxeerXFeedMarketsTable';
import PaxeerXFeedMarketUpdatesTable from 'ui/paxeerX/feed/PaxeerXFeedMarketUpdatesTable';
import PaxeerXFeedUpdateDetails from 'ui/paxeerX/feed/PaxeerXFeedUpdateDetails';
import { Wrapper } from 'ui/shared/layout/testWrapper';
import { afterAll, beforeAll, expect, test } from 'vitest';

import 'lib/setLocale';

const root = path.resolve(__dirname, '../../../..');
const gateway = readFileSync(path.join(root, 'services/platform/hosted/gateway/src/explorer_proxy.rs'), 'utf8');
const router = readFileSync(path.join(root, 'services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex'), 'utf8');
const routeDeclarations = Array.from(gateway.slice(gateway.indexOf('static ROUTES:')).matchAll(
  /Route\s*\{\s*method:\s*"([A-Z]+)",\s*path:\s*"([^"]+)",\s*session_csrf:\s*(true|false),\s*\}/g,
));
const prefix = '/explorer/backend';
const marketPath = '/api/v2/paxeer-x/feed/markets';
const historyPath = `${ marketPath }/0/updates`;
const cursor = { block_number: marketUpdates[1].block_number, index: marketUpdates[1].index };
const firstPage: MarketUpdatesResponse = { items: marketUpdates.slice(0, 2), next_page_params: cursor };
const nextPage: MarketUpdatesResponse = { items: marketUpdates.slice(2), next_page_params: null };

beforeAll(() => {
  fetchMock.disableMocks();
});
afterAll(() => {
  fetchMock.enableMocks();
});

function assertFeedRoute(resource: keyof typeof GENERAL_API_PAXEER_X_FEED_RESOURCES, template: string, producer: string) {
  const definition = GENERAL_API_PAXEER_X_FEED_RESOURCES[resource];
  expect(definition.path.replace(':id', '{market_id}').replace(':hash', '{hash}')).toBe(template);
  expect(routeDeclarations.filter(route => route[2] === template).map(route => [ route[1], route[3] ]))
    .toEqual([ [ 'GET', 'false' ] ]);
  expect(router).toContain(producer);
}

async function openFeedPeer() {
  const documents = new Map<string, unknown>([
    [ `${ prefix }${ marketPath }`, markets ],
    [ `${ prefix }${ historyPath }`, firstPage ],
    [ `${ prefix }${ historyPath }?block_number=${ cursor.block_number }&index=${ cursor.index }`, nextPage ],
    [ `${ prefix }/api/v2/paxeer-x/feed/updates/${ priceUpdate.hash }`, priceUpdate ],
    [ `${ prefix }/api/v2/paxeer-x/feed/updates/${ refusedPriceUpdate.hash }`, refusedPriceUpdate ],
  ]);
  const requests: Array<string> = [];
  // This is a backend response peer, not a replacement for either Rust gateway handler.
  const server = createServer((request, response) => {
    const target = request.url ?? '';
    requests.push(`${ request.method } ${ target }`);
    if (request.method !== 'GET' || !documents.has(target)) {
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ message: 'Unexpected fixture request' }));
      return;
    }
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify(documents.get(target)));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP address');
  }
  const chain: ClusterChainConfig = {
    id: '125', name: 'Feed response fixture', slug: 'feed-fixture',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        general: {
          endpoint: `http://127.0.0.1:${ address.port }`,
          host: '127.0.0.1', protocol: 'http', port: String(address.port),
          socketEndpoint: `ws://127.0.0.1:${ address.port }`, basePath: prefix,
        },
      },
    },
  };
  return {
    chain, requests,
    close: () => new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

function openFeedConsumer() {
  const client = new QueryClient();
  const state: { read?: ReturnType<typeof useApiFetch> } = {};
  function FeedConsumer() {
    state.read = useApiFetch();
    return null;
  }
  renderToStaticMarkup(React.createElement(QueryClientProvider, { client, children: React.createElement(FeedConsumer) }));
  if (!state.read) {
    throw new Error('Feed API consumer did not mount');
  }
  return { read: state.read, close: () => client.clear() };
}

function renderConsumer(element: React.ReactElement) {
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(React.createElement(Wrapper, { children: element }));
  return container;
}

async function feedOverviewGatewayContractAndConsumer() {
  assertFeedRoute('paxeer_x_feed_markets', marketPath, 'get("/feed/markets", V2.PaxeerX.FeedController, :markets)');
  const peer = await openFeedPeer();
  const consumer = openFeedConsumer();
  try {
    const received = await consumer.read<'general:paxeer_x_feed_markets', FeedMarketsResponse>(
      'general:paxeer_x_feed_markets', { chain: peer.chain },
    );
    if (!('items' in received)) {
      throw new Error('Expected FeedView markets.json');
    }
    expect(received).toEqual(markets);
    expect(received.chain_head).toBe(34800135);
    const rendered = renderConsumer(React.createElement(PaxeerXFeedMarketsTable, { items: received.items }));
    expect(rendered.querySelectorAll('[data-market]')).toHaveLength(2);
    expect(rendered.querySelector('a[href="/feed/market/0"]')?.textContent).toBe('BTC-USD');
    expect(rendered.querySelector(`a[href="/feed/update/${ priceUpdate.hash }"]`)).toBeTruthy();
    expect(rendered.querySelector('[data-market="0"] [data-price]')?.getAttribute('data-price')).toBe('67012500000000000000000');
    expect(rendered.querySelector('[data-market="17"] [data-price]')).toBeNull();
    expect(peer.requests).toEqual([ `GET ${ prefix }${ marketPath }` ]);
  } finally {
    consumer.close();
    await peer.close();
  }
}

async function feedMarketGatewayContractAndCursorConsumer() {
  assertFeedRoute('paxeer_x_feed_market_updates', `${ marketPath }/{market_id}/updates`,
    'get("/feed/markets/:market_id/updates", V2.PaxeerX.FeedController, :market_updates)');
  const peer = await openFeedPeer();
  const consumer = openFeedConsumer();
  try {
    const received = await consumer.read<'general:paxeer_x_feed_market_updates', MarketUpdatesResponse>(
      'general:paxeer_x_feed_market_updates', { chain: peer.chain, pathParams: { id: '0' } },
    );
    if (!('items' in received) || !received.next_page_params) {
      throw new Error('Expected a market page with a FeedView cursor');
    }
    expect(received).toEqual(firstPage);
    const url = new URL(buildUrl('general:paxeer_x_feed_market_updates', { id: '0' }, received.next_page_params, true, peer.chain));
    expect(url.pathname).toBe(`${ prefix }${ historyPath }`);
    expect(url.search).toBe('?block_number=34800122&index=0');
    const next = await consumer.read<'general:paxeer_x_feed_market_updates', MarketUpdatesResponse>(
      'general:paxeer_x_feed_market_updates', { chain: peer.chain, pathParams: { id: '0' }, queryParams: received.next_page_params },
    );
    if (!('items' in next)) {
      throw new Error('Expected the next market page');
    }
    expect(next).toEqual(nextPage);
    expect(next.next_page_params).toBeNull();
    expect([ ...received.items, ...next.items ]).toEqual(marketUpdates);
    const rendered = renderConsumer(React.createElement(PaxeerXFeedMarketUpdatesTable, { items: received.items }));
    expect(rendered.querySelectorAll('[data-update]')).toHaveLength(2);
    for (const item of received.items) {
      expect(rendered.querySelector(`a[href="/feed/update/${ item.update_hash }"]`)).toBeTruthy();
      expect(rendered.querySelector(`[data-update="${ item.update_hash }"] [data-price]`)?.getAttribute('data-price')).toBe(item.index_price);
    }
    expect(peer.requests).toEqual([ `GET ${ prefix }${ historyPath }`, `GET ${ url.pathname }${ url.search }` ]);
  } finally {
    consumer.close();
    await peer.close();
  }
}

async function feedUpdateGatewayContractAndNativeSubmissionConsumer() {
  assertFeedRoute('paxeer_x_feed_update', '/api/v2/paxeer-x/feed/updates/{hash}',
    'get("/feed/updates/:hash", V2.PaxeerX.FeedController, :update)');
  const peer = await openFeedPeer();
  const consumer = openFeedConsumer();
  try {
    for (const submission of [ priceUpdate, refusedPriceUpdate ]) {
      const received = await consumer.read<'general:paxeer_x_feed_update', PriceUpdateItem>(
        'general:paxeer_x_feed_update', { chain: peer.chain, pathParams: { hash: submission.hash } },
      );
      if (!('price_update' in received)) {
        throw new Error('Expected FeedView update.json');
      }
      expect(received).toEqual(submission);
      expect(received.type).toBe(127);
      expect(received.method).toBe('submitPrices');
      expect(received.to).toBeNull();
      expect(received.price_update.observations.map(item => item.accepted))
        .toEqual(submission.price_update.observations.map(item => item.accepted));
      const rendered = renderConsumer(React.createElement(PaxeerXFeedUpdateDetails, { data: received }));
      expect(rendered.querySelector('[data-field="hash"]')?.textContent).toBe(submission.hash);
      expect(rendered.querySelector('[data-field="observation_count"]')?.textContent).toBe('2');
      for (const observation of submission.price_update.observations) {
        expect(rendered.querySelector(`[data-observation="${ observation.market_id }"] [data-price]`)?.getAttribute('data-price'))
          .toBe(observation.index_price);
        expect(typeof observation.index_price).toBe('string');
        expect(typeof observation.mark_price).toBe('string');
      }
    }
    expect(peer.requests).toEqual([ priceUpdate, refusedPriceUpdate ].map(item => `GET ${ prefix }/api/v2/paxeer-x/feed/updates/${ item.hash }`));
  } finally {
    consumer.close();
    await peer.close();
  }
}

function feedGatewayDeclaresOnlyTheThreeRequiredReadRoutes() {
  expect(routeDeclarations.filter(route => route[2].includes('/feed/')).map(route => [ route[1], route[2], route[3] ]))
    .toEqual([
      [ 'GET', marketPath, 'false' ],
      [ 'GET', `${ marketPath }/{market_id}/updates`, 'false' ],
      [ 'GET', '/api/v2/paxeer-x/feed/updates/{hash}', 'false' ],
    ]);
}

test(feedOverviewGatewayContractAndConsumer.name, feedOverviewGatewayContractAndConsumer);
test(feedMarketGatewayContractAndCursorConsumer.name, feedMarketGatewayContractAndCursorConsumer);
test(feedUpdateGatewayContractAndNativeSubmissionConsumer.name, feedUpdateGatewayContractAndNativeSubmissionConsumer);
test(feedGatewayDeclaresOnlyTheThreeRequiredReadRoutes.name, feedGatewayDeclaresOnlyTheThreeRequiredReadRoutes);

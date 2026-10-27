import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type ContractTypes,
  type QuoteBody,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import * as market from '../../src/runtime/routes/market.ts';
import * as account from '../../src/runtime/routes/account.ts';
import * as notifications from '../../src/runtime/routes/notifications.ts';
import * as chat from '../../src/runtime/routes/chat.ts';
import * as activity from '../../src/runtime/routes/activity.ts';

async function routeModulesPreserveHeadPaths() {
  const address = `0x${'Ab'.repeat(20)}`;
  const hash = `0x${'Cd'.repeat(32)}`;
  const id = '123e4567-e89b-72d3-a456-426614174000';
  const room = '123e4567-e89b-42d3-a456-426614174001';
  const time = '2026-01-01T00:00:00.000Z';
  const position = {
    blockHash: hash,
    chainId: '125',
    height: '1',
    logIndex: '0',
    transactionHash: hash,
    transactionIndex: '0',
  } satisfies ContractTypes['ChainPosition'];
  const watermark = {
    blockHash: hash,
    chainId: '125',
    height: '1',
    integrity: 'healthy',
    observedAt: time,
    projectionRevision: '1',
  } satisfies ContractTypes['Watermark'];
  const envelope = {
    generatedAt: time,
    nextCursor: null,
    quality: 'complete',
    requestId: id,
    revision: '1',
    watermark,
  } as const;
  const pool = {
    chainId: '125',
    createdPosition: position,
    creator: address,
    effectivePricingUsdlReserveRaw: '2',
    feeBps: 30,
    poolAddress: address,
    quoteAssetAddress: address,
    quoteAssetDecimals: 6,
    realUsdlBalanceRaw: '1',
    tokenAddress: address,
    tokenReserveRaw: '1',
    virtualUsdlReserveRaw: '1',
  } satisfies ContractTypes['Pool'];
  const message = {
    authorWallet: address,
    body: 'Hello, route!',
    createdAt: time,
    deletedAt: null,
    editedAt: null,
    messageId: id,
    revision: '1',
    roomId: room,
  } satisfies ContractTypes['Message'];
  const quoteBody: QuoteBody = {
    inputRaw: '1',
    poolAddress: address,
    side: 'buy',
    slippageBps: 50,
  };
  const fixtures = {
    PoolListResponse: { ...envelope, data: { items: [pool] } },
    PoolDetailResponse: {
      ...envelope,
      data: {
        pool,
        stats: {
          buys24h: '0',
          changeBaselineQuality: 'unavailable',
          circulatingMarketCapUsdlRaw: null,
          holderCount: null,
          indicativeTvlUsdlRaw: null,
          poolAddress: address,
          priceChangeBps24h: null,
          realUsdlBalanceRaw: '1',
          sells24h: '0',
          sourceUpperPosition: position,
          supplyValuationQuality: 'unavailable',
          totalSupplyValuationUsdlRaw: null,
          valuationChangeUsdlRaw: null,
          valuationMethod: null,
          volume24hRaw: '0',
          watermark,
          windowEnd: time,
          windowStart: '2025-12-31T00:00:00.000Z',
        },
        token: {
          address,
          decimals: null,
          decimalsQuality: 'unavailable',
          logoAssetId: null,
          metadataRevision: '1',
          name: 'Route token',
          sourcePosition: position,
          supplyQuality: 'unavailable',
          symbol: 'ROUTE',
          totalSupplyRaw: null,
        },
      },
    },
    SearchResponse: { ...envelope, data: { items: [] } },
    PortfolioResponse: {
      ...envelope,
      data: {
        items: [],
        summary: {
          basisQuality: 'complete',
          completeValueUsdlRaw: '0',
          knownRealizedSubtotalUsdlRaw: '0',
          positionCount: 0,
          pricedSubtotalUsdlRaw: '0',
          pricingQuality: 'complete',
          realizedPnlRaw: '0',
          snapshotId: id,
          unpricedAssetCount: 0,
          wallet: address,
          watermark,
        },
      },
    },
    NotificationsResponse: {
      ...envelope,
      data: {
        items: [
          {
            createdAt: time,
            deliveryState: 'sent',
            entityId: address,
            kind: 'price_alert',
            notificationId: id,
            readAt: null,
            wallet: address,
          },
        ],
      },
    },
    NotificationReadResponse: { ...envelope, data: { readAt: time } },
    DmListResponse: {
      ...envelope,
      data: {
        items: [
          {
            conversationId: room,
            kind: 'direct',
            lastMessageId: id,
            muted: false,
            participantWallets: [address, `0x${'12'.repeat(20)}`],
            unreadCount: 1,
          },
        ],
      },
    },
    MessagesListResponse: { ...envelope, data: { items: [message] } },
    MessageSendResponse: { ...envelope, data: { message } },
    StreamListResponse: {
      ...envelope,
      data: {
        items: [
          {
            creatorWallet: address,
            playbackUrl: null,
            poolAddress: address,
            status: 'live',
            streamId: id,
            title: 'Route stream',
            viewerCount: 1,
          },
        ],
      },
    },
    QuoteResponse: {
      ...envelope,
      data: {
        quote: {
          feeRaw: '0',
          inputRaw: '1',
          minOutputRaw: '0',
          outputRaw: '1',
          poolAddress: address,
          quoteId: id,
          side: 'buy',
          validUntil: time,
          watermark,
        },
      },
    },
    TxStatusResponse: {
      ...envelope,
      data: { receipt: null, status: 'unknown' },
    },
  } satisfies {
    [N in
      | 'PoolListResponse'
      | 'PoolDetailResponse'
      | 'SearchResponse'
      | 'PortfolioResponse'
      | 'NotificationsResponse'
      | 'NotificationReadResponse'
      | 'DmListResponse'
      | 'MessagesListResponse'
      | 'MessageSendResponse'
      | 'StreamListResponse'
      | 'QuoteResponse'
      | 'TxStatusResponse']: ContractTypes[N];
  };

  const expectedPairs = [
    ['GET', '/v1/pools?limit=50'],
    ['GET', `/v1/pools/${address}`],
    ['GET', '/v1/search?type=pool&limit=50&q=pool%20%26%20token%2FUSDL'],
    ['GET', '/v1/me/portfolio?limit=50'],
    ['GET', '/v1/me/notifications?limit=50'],
    ['PUT', `/v1/me/notifications/${id}/read`],
    ['GET', '/v1/me/conversations?limit=50'],
    ['GET', `/v1/rooms/${room}/messages?limit=50`],
    ['POST', `/v1/rooms/${room}/messages`],
    ['GET', '/v1/streams?status=live&limit=50'],
    ['POST', '/v1/quotes'],
    ['GET', `/v1/transactions/${hash}`],
  ];
  const responses = Object.values(fixtures);
  const observed: {
    method: string | undefined;
    path: string | undefined;
    csrf: string | string[] | undefined;
    body: string;
  }[] = [];
  const server = createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => {
      body += chunk;
    });
    request.on('end', () => {
      const index = expectedPairs.findIndex(
        ([method, path]) => method === request.method && path === request.url,
      );
      observed.push({
        method: request.method,
        path: request.url,
        csrf: request.headers['x-csrf-token'],
        body,
      });
      if (index < 0) {
        response.writeHead(400);
        response.end();
        return;
      }
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(responses[index]));
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Loopback port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = `ws://127.0.0.1:${bound.port}`;
    const sdk = new SdkClient({
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
      session: {
        mode: 'cookie',
        applicationOrigin: origin,
        csrfToken: () => 'route-csrf',
      },
    });
    const signal = new AbortController().signal;
    const calls = [
      () => market.pools(sdk, signal),
      () => market.pool(sdk, address, signal),
      () => market.search(sdk, 'pool & token/USDL', signal),
      () => account.portfolio(sdk, signal),
      () => notifications.notifications(sdk, signal),
      () => notifications.readNotification(sdk, id, signal),
      () => chat.conversations(sdk, signal),
      () => chat.messages(sdk, room, signal),
      () => chat.sendMessage(sdk, room, message.body, id, signal),
      () => activity.streams(sdk, signal),
      () => activity.quote(sdk, quoteBody, signal),
      () => activity.receipt(sdk, hash, signal),
    ];
    for (const [index, call] of calls.entries()) {
      expect(await call()).toEqual(responses[index]);
    }
    expect(observed.map(({ method, path }) => [method, path])).toEqual(
      expectedPairs,
    );
    expect(observed.map(({ body }) => body)).toEqual([
      '',
      '',
      '',
      '',
      '',
      '',
      '',
      '',
      JSON.stringify({ body: message.body, clientMessageId: id }),
      '',
      JSON.stringify(quoteBody),
      '',
    ]);
    expect(observed.map(({ csrf }) => csrf)).toEqual([
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      'route-csrf',
      undefined,
      undefined,
      'route-csrf',
      undefined,
      undefined,
      undefined,
    ]);
    expect(() => market.pool(sdk, 'invalid', signal)).toThrow(
      'Invalid wallet or pool address',
    );
    expect(() =>
      notifications.readNotification(sdk, '../invalid', signal),
    ).toThrow('Invalid resource identifier');
    expect(() => chat.messages(sdk, '../invalid', signal)).toThrow(
      'Invalid resource identifier',
    );
    expect(() =>
      chat.sendMessage(sdk, '../invalid', message.body, id, signal),
    ).toThrow('Invalid resource identifier');
    expect(() => activity.receipt(sdk, '0x123', signal)).toThrow(
      'Invalid transaction hash',
    );
    const cancelled = AbortSignal.abort();
    await expect(market.pools(sdk, cancelled)).rejects.toThrow();
    expect(observed).toHaveLength(expectedPairs.length);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

test(routeModulesPreserveHeadPaths, routeModulesPreserveHeadPaths);

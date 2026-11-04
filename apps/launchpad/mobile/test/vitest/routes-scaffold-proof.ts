import { createServer } from 'node:http';
import { expect } from 'vitest';
import { readFileSync } from 'node:fs';
import {
  SdkClient,
  decodeV2Contract,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import * as market from '../../src/runtime/routes/market.ts';
import * as account from '../../src/runtime/routes/account.ts';
import * as notifications from '../../src/runtime/routes/notifications.ts';
import * as chat from '../../src/runtime/routes/chat.ts';
import * as activity from '../../src/runtime/routes/activity.ts';

export async function routeModulesPreserveHeadPaths() {
  const recording = JSON.parse(
    readFileSync(
      new URL('./fixtures/repair-22-33-929-v2.json', import.meta.url),
      'utf8',
    ),
  ) as { poolList: unknown; poolDetail: unknown };
  const poolList = decodeV2Contract('PoolListResponse', recording.poolList);
  const poolDetail = decodeV2Contract(
    'PoolDetailResponse',
    recording.poolDetail,
  );
  const address = poolDetail.data.pool.poolAddress;
  const hash = `0x${'Cd'.repeat(32)}`;
  const id = '123e4567-e89b-72d3-a456-426614174000';
  const notificationId = 'ab'.repeat(32);
  const room = '123e4567-e89b-42d3-a456-426614174001';
  const time = '2026-01-01T00:00:00.000Z';
  const watermark = {
    blockHash: hash,
    chainId: '125',
    height: '1',
    integrity: 'healthy',
    observedAt: time,
    projectionRevision: '1',
  } satisfies V2ContractTypes['Watermark'];
  const envelope = {
    generatedAt: time,
    nextCursor: null,
    quality: 'complete',
    requestId: id,
    revision: '1',
    watermark,
  } as const;
  const message = {
    authorWallet: address,
    body: 'Hello, route!',
    createdAt: time,
    deletedAt: null,
    editedAt: null,
    messageId: id,
    revision: '1',
    roomId: room,
  } satisfies V2ContractTypes['Message'];
  const notification = {
    ID: notificationId,
    Identity: {
      AccountID: id,
      TriggerEventID: 'price-trigger',
      TemplateVersion: '1',
    },
    Category: 'price',
    Summary: 'Your price alert was triggered',
    ResourceID: address,
    CreatedAt: time,
    ExpiresAt: '2026-01-02T00:00:00.000Z',
    Read: false,
  } satisfies V2ContractTypes['ConnectedNotificationsInboxItem'];
  const conversation = {
    id: room,
    peer: address,
    last_message_at: 1767225600,
    unread_count: 1,
    read_sequence: '3',
    muted: false,
  } satisfies V2ContractTypes['ConnectedChatConversationsResponse']['conversations'][number];
  const streams = decodeV2Contract(
    'StreamListResponse',
    JSON.parse(
      readFileSync(
        new URL('../fixtures/live-v2-response.json', import.meta.url),
        'utf8',
      ),
    ),
  );
  const fixtures = {
    PoolListResponse: poolList,
    PoolDetailResponse: poolDetail,
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
    ConnectedNotificationListResponse: [notification],
    ConnectedReadResponse: { read: true },
    ConnectedChatConversationsResponse: { conversations: [conversation] },
    MessagesListResponse: { ...envelope, data: { items: [message] } },
    MessageSendResponse: { ...envelope, data: { message } },
    StreamListResponse: streams,
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
      | 'ConnectedNotificationListResponse'
      | 'ConnectedReadResponse'
      | 'ConnectedChatConversationsResponse'
      | 'MessagesListResponse'
      | 'MessageSendResponse'
      | 'StreamListResponse'
      | 'TxStatusResponse']: V2ContractTypes[N];
  };

  const expectedPairs = [
    ['GET', '/v2/pools?sort=new&limit=50'],
    ['GET', `/v2/pools/${address}`],
    ['GET', '/v2/search?q=pool+%26+token%2FUSDL&type=pool&limit=25'],
    ['GET', '/v2/me/portfolio'],
    ['GET', '/v2/discovery/notifications'],
    ['PATCH', `/v2/discovery/notifications/${notificationId}`],
    ['GET', '/v2/chat/conversations'],
    ['GET', `/v2/rooms/${room}/messages?limit=50`],
    ['POST', `/v2/rooms/${room}/messages`],
    ['GET', '/v2/streams?status=live&limit=50'],
    ['GET', `/v2/transactions/${hash}`],
  ];
  for (const name of Object.keys(fixtures) as (keyof typeof fixtures)[]) {
    decodeV2Contract(name, fixtures[name]);
  }
  const responses = Object.values(fixtures);
  const expectedResponses: unknown[] = [...responses];
  expectedResponses[4] = [
    {
      notificationId: notification.ID,
      category: notification.Category,
      summary: notification.Summary,
      resourceId: notification.ResourceID,
      createdAt: notification.CreatedAt,
      expiresAt: notification.ExpiresAt,
      read: notification.Read,
    },
  ];
  expectedResponses[6] = {
    data: {
      items: [
        {
          conversationId: conversation.id,
          peer: conversation.peer,
          unreadCount: conversation.unread_count,
          muted: conversation.muted,
          lastMessageAt: conversation.last_message_at,
          readSequence: conversation.read_sequence,
        },
      ],
    },
  };
  const observed: {
    method: string | undefined;
    path: string | undefined;
    csrf: string | string[] | undefined;
    idempotencyKey: string | string[] | undefined;
    authorization: string | undefined;
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
        ([method, path]) =>
          method === request.method && path === request.url,
      );
      observed.push({
        method: request.method,
        path: request.url,
        csrf: request.headers['x-csrf-token'],
        idempotencyKey: request.headers['idempotency-key'],
        authorization: request.headers.authorization,
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
    const credentials: (RequestCredentials | undefined)[] = [];
    const sdk = new SdkClient(
      {
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
      },
      (input, init) => {
        credentials.push(init?.credentials);
        return fetch(input, init);
      },
    );
    const signal = new AbortController().signal;
    const calls = [
      () => market.pools(sdk, signal),
      () => market.pool(sdk, address, signal),
      () => market.search(sdk, 'pool & token/USDL', signal),
      () => account.portfolio(sdk, signal),
      () => notifications.notifications(sdk, signal),
      () => notifications.readNotification(sdk, notificationId, signal),
      () => chat.conversations(sdk, signal),
      () => chat.messages(sdk, room, signal),
      () => chat.sendMessage(sdk, room, message.body, id, signal),
      () => activity.streams(sdk, signal),
      () => activity.receipt(sdk, hash, signal),
    ];
    for (const [index, call] of calls.entries()) {
      expect(await call()).toEqual(expectedResponses[index]);
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
      JSON.stringify({ read: true }),
      '',
      '',
      JSON.stringify({ body: message.body, clientMessageId: id }),
      '',
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
    ]);
    expect(() => market.pool(sdk, 'invalid', signal)).toThrow(
      'Invalid wallet or pool address',
    );
    expect(() =>
      notifications.readNotification(sdk, '../invalid', signal),
    ).toThrow('Invalid resource identifier');
    await expect(chat.messages(sdk, '../invalid', signal)).rejects.toThrow(
      'Invalid IF_MESSAGES_LIST path',
    );
    await expect(
      chat.sendMessage(sdk, '../invalid', message.body, id, signal),
    ).rejects.toThrow('Invalid IF_MESSAGE_SEND path');
    expect(() => activity.receipt(sdk, '0x123', signal)).toThrow(
      'Invalid transaction hash',
    );
    expect(observed.map(({ idempotencyKey }) => idempotencyKey)).toEqual(
      expectedPairs.map((_, index) => (index === 8 ? id : undefined)),
    );
    expect(observed.map(({ authorization }) => authorization)).toEqual(
      expectedPairs.map(() => undefined),
    );
    expect(credentials).toEqual([
      'omit',
      'omit',
      'omit',
      'same-origin',
      'same-origin',
      'same-origin',
      'same-origin',
      'same-origin',
      'same-origin',
      'omit',
      'omit',
    ]);
    const anonymous = new SdkClient({ ...sdk.config, session: undefined });
    const privateCalls = [
      () => account.portfolio(anonymous, signal),
      () => notifications.notifications(anonymous, signal),
      () =>
        notifications.readNotification(anonymous, notificationId, signal),
      () => chat.conversations(anonymous, signal),
      () => chat.messages(anonymous, room, signal),
      () => chat.sendMessage(anonymous, room, message.body, id, signal),
    ];
    for (const call of privateCalls)
      await expect(call()).rejects.toThrow('Session required');
    const noCsrf = new SdkClient({
      ...sdk.config,
      session: {
        mode: 'cookie',
        applicationOrigin: origin,
        csrfToken: () => '',
      },
    });
    await expect(
      notifications.readNotification(noCsrf, notificationId, signal),
    ).rejects.toThrow('CSRF token required');
    await expect(
      chat.sendMessage(noCsrf, room, message.body, id, signal),
    ).rejects.toThrow('CSRF token required');
    const cancelled = AbortSignal.abort();
    const cancelledCalls = [
      () => market.pools(sdk, cancelled),
      () => market.pool(sdk, address, cancelled),
      () => market.search(sdk, 'pool & token/USDL', cancelled),
      () => account.portfolio(sdk, cancelled),
      () => notifications.notifications(sdk, cancelled),
      () => notifications.readNotification(sdk, notificationId, cancelled),
      () => chat.conversations(sdk, cancelled),
      () => chat.messages(sdk, room, cancelled),
      () => chat.sendMessage(sdk, room, message.body, id, cancelled),
      () => activity.streams(sdk, cancelled),
      () => activity.receipt(sdk, hash, cancelled),
    ];
    for (const call of cancelledCalls)
      await expect(call()).rejects.toThrow();
    expect(observed).toHaveLength(expectedPairs.length);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

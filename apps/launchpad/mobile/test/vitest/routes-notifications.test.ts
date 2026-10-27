import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import {
  IF_CONNECTED_NOTIFICATION_READ,
  notificationRoutes,
} from '../../src/runtime/routes/notifications.ts';

async function notificationsUseDiscoveryRoutes() {
  const id = 'ab'.repeat(32);
  const item: V2ContractTypes['ConnectedNotificationsInboxItem'] = {
    ID: id,
    Identity: {
      AccountID: '123e4567-e89b-42d3-a456-426614174000',
      TriggerEventID: 'price-trigger',
      TemplateVersion: '1',
    },
    Category: 'price',
    Summary: 'Your price alert was triggered',
    ResourceID: `0x${'cd'.repeat(20)}`,
    CreatedAt: '2026-01-01T00:00:00Z',
    ExpiresAt: '2026-01-02T00:00:00Z',
    Read: false,
  };
  const observed: {
    method: string | undefined;
    path: string | undefined;
    csrf: string | string[] | undefined;
    contentType: string | undefined;
    body: string;
  }[] = [];
  let status = 200;
  let malformed = false;
  const server = createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => {
      body += chunk;
    });
    request.on('end', () => {
      observed.push({
        method: request.method,
        path: request.url,
        csrf: request.headers['x-csrf-token'],
        contentType: request.headers['content-type'],
        body,
      });
      response.setHeader('Content-Type', 'application/json');
      if (status !== 200) {
        response.writeHead(status);
        response.end('{}');
      } else if (
        request.method === 'GET' &&
        request.url === '/v2/discovery/notifications'
      ) {
        response.end(
          JSON.stringify(malformed ? [{ ...item, Read: 'false' }] : [item]),
        );
      } else if (
        request.method === 'PATCH' &&
        request.url === `/v2/discovery/notifications/${id}` &&
        body === JSON.stringify({ read: true })
      ) {
        item.Read = true;
        response.end(
          JSON.stringify({
            read: item.Read,
          } satisfies V2ContractTypes['ConnectedReadResponse']),
        );
      } else {
        response.writeHead(400);
        response.end('{}');
      }
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
        csrfToken: () => 'notification-csrf',
      },
    });
    const signal = new AbortController().signal;
    const mapped = {
      notificationId: id,
      category: item.Category,
      summary: item.Summary,
      resourceId: item.ResourceID,
      createdAt: item.CreatedAt,
      expiresAt: item.ExpiresAt,
      read: false,
    };
    expect(IF_CONNECTED_NOTIFICATION_READ).toBe(
      'IF_CONNECTED_NOTIFICATION_READ',
    );
    expect(await notificationRoutes.notifications(sdk, signal)).toEqual([
      mapped,
    ]);
    expect(await notificationRoutes.readNotification(sdk, id, signal)).toEqual({
      read: true,
    });
    expect(await notificationRoutes.notifications(sdk, signal)).toEqual([
      { ...mapped, read: true },
    ]);
    expect(observed).toEqual([
      {
        method: 'GET',
        path: '/v2/discovery/notifications',
        csrf: undefined,
        contentType: undefined,
        body: '',
      },
      {
        method: 'PATCH',
        path: `/v2/discovery/notifications/${id}`,
        csrf: 'notification-csrf',
        contentType: 'application/json',
        body: '{"read":true}',
      },
      {
        method: 'GET',
        path: '/v2/discovery/notifications',
        csrf: undefined,
        contentType: undefined,
        body: '',
      },
    ]);
    for (const invalid of ['n1', '../invalid', id.toUpperCase(), `${id}0`]) {
      expect(() =>
        notificationRoutes.readNotification(sdk, invalid, signal),
      ).toThrow('Invalid resource identifier');
    }
    const cancelled = AbortSignal.abort();
    await expect(
      notificationRoutes.notifications(sdk, cancelled),
    ).rejects.toThrow();
    await expect(
      notificationRoutes.readNotification(sdk, id, cancelled),
    ).rejects.toThrow();
    expect(observed).toHaveLength(3);
    malformed = true;
    await expect(notificationRoutes.notifications(sdk, signal)).rejects.toThrow(
      'Invalid v2 contract ConnectedNotificationListResponse',
    );
    malformed = false;
    status = 403;
    await expect(
      notificationRoutes.notifications(sdk, signal),
    ).rejects.toMatchObject({ status: 403 });
    await expect(
      notificationRoutes.readNotification(sdk, id, signal),
    ).rejects.toMatchObject({ status: 403 });
    expect(observed).toHaveLength(6);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

test(notificationsUseDiscoveryRoutes, notificationsUseDiscoveryRoutes);

// @vitest-environment jsdom
// @vitest-environment-options {"url":"http://127.0.0.1:32235"}

import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { NextRequest } from 'next/server';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { PortfolioApiError } from './client';
import { getWatchStatus, unwatchAddress, watchAddress } from './watch';
import {
  createWebhook,
  deleteWebhook,
  getWebhook,
  getWebhookDeliveries,
  listWebhooks,
  updateWebhook,
} from './webhooks';

const ADDRESS = '0x1111111111111111111111111111111111111111';
const WEBHOOK_ID = '22-31-235';
let walletRoute: typeof import('../../app/api/wallet/[...path]/route');
let server: Server;
let indexer: Server;
let directory: string;
const TOKEN = 'recorded-portfolio-owner-credential';
const OTHER_TOKEN = 'recorded-other-owner-credential';
const originalEnv = ['PORTFOLIO_UPSTREAM_BASE', 'RATE_LIMIT_STORE_PATH'].map(
  (name) => [name, process.env[name]] as const
);
interface Exchange {
  method: string;
  path: string;
  request?: unknown;
  token?: string;
  status: number;
  response?: unknown;
  headers?: Record<string, string>;
}
// In-process transcript peer permitted by decision 2520. It never implements
// watch/webhook logic: each exchange must exactly match the recorded contract.
const transcript: Exchange[] = [];
const producerReceived: {
  method: string;
  path: string;
  authorization: string | undefined;
  body: unknown;
}[] = [];
const fixtureErrors: unknown[] = [];
const responses: { status: number; cacheControl: string | null }[] = [];

async function startIndexer() {
  indexer = createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const text = Buffer.concat(chunks).toString('utf8');
      const body: unknown = text ? JSON.parse(text) : undefined;
      producerReceived.push({
        method: request.method ?? '',
        path: request.url ?? '',
        authorization: request.headers.authorization,
        body,
      });
      const exchange = transcript.shift();
      expect(exchange).toBeDefined();
      if (!exchange) throw new Error('Unexpected producer request');
      expect(request.method).toBe(exchange.method);
      expect(request.url).toBe(exchange.path);
      expect(request.headers.authorization).toBe(`Bearer ${exchange.token ?? TOKEN}`);
      expect(body).toEqual(exchange.request);
      expect(request.headers.cookie).toBeUndefined();
      if (body !== undefined) expect(request.headers['content-type']).toBe('application/json');
      response.writeHead(exchange.status, {
        'Content-Type': 'application/json',
        ...exchange.headers,
      });
      response.end(exchange.response === undefined ? undefined : JSON.stringify(exchange.response));
    } catch (error) {
      fixtureErrors.push(error);
      response.writeHead(500, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify({ error: 'transcript mismatch' }));
    }
  });
  indexer.listen(0, '127.0.0.1');
  await once(indexer, 'listening');
  const address = indexer.address();
  if (!address || typeof address === 'string') throw new Error('Expected loopback listener');
  process.env.PORTFOLIO_UPSTREAM_BASE = `http://127.0.0.1:${address.port}`;
}

async function stopServer(listener: Server) {
  if (!listener?.listening) return;
  await new Promise<void>((resolve, reject) => {
    listener.close((error) => (error ? reject(error) : resolve()));
    listener.closeAllConnections();
  });
}

async function restartIndexer() {
  await stopServer(indexer);
  await startIndexer();
}
const received: { method: string; path: string; contentType: string | null; body: unknown }[] = [];

beforeEach(() => {
  received.length = 0;
  producerReceived.length = 0;
  responses.length = 0;
});

afterEach(() => {
  expect(fixtureErrors).toEqual([]);
  expect(transcript).toHaveLength(0);
});

beforeAll(async () => {
  directory = await mkdtemp(path.join(tmpdir(), 'portfolio-repair-'));
  process.env.RATE_LIMIT_STORE_PATH = path.join(directory, 'limits.json');
  await startIndexer();
  walletRoute = await import('../../app/api/wallet/[...path]/route');
  // Adapt Node HTTP to the actual Next handlers; all client requests use native fetch.
  server = createServer(async (incoming, outgoing) => {
    try {
      const url = new URL(incoming.url ?? '/', window.location.origin);
      const chunks: Buffer[] = [];
      for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString('utf8');
      const request = new NextRequest(url, {
        method: incoming.method,
        headers: new Headers(
          Object.entries(incoming.headers).flatMap(([key, value]) =>
            value === undefined
              ? []
              : (Array.isArray(value) ? value : [value]).map((entry): [string, string] => [
                  key,
                  entry,
                ])
          )
        ),
        ...(body ? { body } : {}),
      });
      received.push({
        method: request.method,
        path: request.nextUrl.pathname,
        contentType: request.headers.get('content-type'),
        body: body ? JSON.parse(body) : null,
      });
      const context = {
        params: Promise.resolve({
          path: url.pathname.replace('/wallet/api/wallet/', '').split('/').map(decodeURIComponent),
        }),
      };
      let response: Response;
      switch (request.method) {
        case 'GET':
          response = await walletRoute.GET(request, context);
          break;
        case 'POST':
          response = await walletRoute.POST(request, context);
          break;
        case 'PATCH':
          response = await walletRoute.PATCH(request, context);
          break;
        case 'DELETE':
          response = await walletRoute.DELETE(request, context);
          break;
        default:
          throw new Error(`Unexpected method: ${request.method}`);
      }
      responses.push({
        status: response.status,
        cacheControl: response.headers.get('cache-control'),
      });
      outgoing.writeHead(response.status, Object.fromEntries(response.headers));
      outgoing.end(Buffer.from(await response.arrayBuffer()));
    } catch (error) {
      outgoing.destroy(error instanceof Error ? error : new Error(String(error)));
    }
  });
  server.listen(Number(window.location.port), window.location.hostname);
  await once(server, 'listening');
});

afterAll(async () => {
  await stopServer(server);
  await stopServer(indexer);
  for (const [name, value] of originalEnv) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  if (directory) await rm(directory, { recursive: true, force: true });
});

async function read_only_proxy_explicitly_refuses_watch_and_webhook_mutations() {
  // Non-lifecycle paths still refuse every formerly rejected mutation method.
  for (const [method, handler] of [
    ['POST', walletRoute.POST],
    ['PATCH', walletRoute.PATCH],
    ['DELETE', walletRoute.DELETE],
  ] as const) {
    const request = new NextRequest('http://localhost/wallet/api/wallet/api/v2/addresses', {
      method,
    });
    const response = await handler(request, {
      params: Promise.resolve({ path: ['api', 'v2', 'addresses'] }),
    });
    expect(response.status).toBe(405);
    expect(await response.json()).toEqual({ error: 'method not allowed' });
  }
  expect(received).toHaveLength(0);
  expect(producerReceived).toHaveLength(0);
}

async function finding_2_watch_mutations_require_authorization_at_the_real_proxy() {
  const results = await Promise.allSettled([
    watchAddress({ address: ADDRESS }),
    unwatchAddress(ADDRESS),
  ]);
  expect(received).toHaveLength(2);
  expect(received).toEqual(
    expect.arrayContaining([
      {
        method: 'POST',
        path: '/wallet/api/wallet/api/v2/watch',
        contentType: 'application/json',
        body: { address: ADDRESS },
      },
      {
        method: 'DELETE',
        path: `/wallet/api/wallet/api/v2/watch/${ADDRESS}`,
        contentType: null,
        body: null,
      },
    ])
  );
  for (const result of results) {
    expect
      .soft(result)
      .toMatchObject({ status: 'rejected', reason: expect.any(PortfolioApiError) });
    expect.soft(result).toMatchObject({ status: 'rejected', reason: { status: 401 } });
  }
}

async function finding_3_webhook_mutations_require_authorization_at_the_real_proxy() {
  const results = await Promise.allSettled([
    createWebhook({
      address: ADDRESS,
      callbackUrl: 'https://example.com/events',
      secret: 'test-webhook-signing-secret',
      events: ['balance_change'],
      minDeltaUsd: '1.25',
    }),
    updateWebhook(WEBHOOK_ID, {
      callbackUrl: 'https://example.com/updated-events',
      events: ['balance_change'],
      minDeltaUsd: '2.50',
      isActive: false,
    }),
    deleteWebhook(WEBHOOK_ID),
  ]);
  expect(received).toHaveLength(3);
  expect(received).toEqual(
    expect.arrayContaining([
      {
        method: 'POST',
        path: '/wallet/api/wallet/api/v2/webhooks',
        contentType: 'application/json',
        body: {
          address: ADDRESS,
          callback_url: 'https://example.com/events',
          secret: 'test-webhook-signing-secret',
          events: ['balance_change'],
          min_delta_usd: '1.25',
        },
      },
      {
        method: 'PATCH',
        path: `/wallet/api/wallet/api/v2/webhooks/${WEBHOOK_ID}`,
        contentType: 'application/json',
        body: {
          callback_url: 'https://example.com/updated-events',
          events: ['balance_change'],
          min_delta_usd: '2.50',
          is_active: false,
        },
      },
      {
        method: 'DELETE',
        path: `/wallet/api/wallet/api/v2/webhooks/${WEBHOOK_ID}`,
        contentType: null,
        body: null,
      },
    ])
  );
  for (const result of results) {
    expect
      .soft(result)
      .toMatchObject({ status: 'rejected', reason: expect.any(PortfolioApiError) });
    expect.soft(result).toMatchObject({ status: 'rejected', reason: { status: 401 } });
  }
}

test(
  read_only_proxy_explicitly_refuses_watch_and_webhook_mutations.name,
  read_only_proxy_explicitly_refuses_watch_and_webhook_mutations
);
test(
  finding_2_watch_mutations_require_authorization_at_the_real_proxy.name,
  finding_2_watch_mutations_require_authorization_at_the_real_proxy
);
test(
  finding_3_webhook_mutations_require_authorization_at_the_real_proxy.name,
  finding_3_webhook_mutations_require_authorization_at_the_real_proxy
);

async function watch_lifecycle_round_trips_and_reads_after_indexer_restart() {
  const watched = { status: 'watching', address: ADDRESS, watched_at: '2026-01-01T00:00:00Z' };
  const active = { address: ADDRESS, watching: true, webhook_count: 0 };
  const inactive = { address: ADDRESS, watching: false, webhook_count: 0 };
  transcript.push(
    {
      method: 'POST',
      path: '/api/v2/watch',
      request: { address: ADDRESS },
      status: 201,
      response: watched,
    },
    { method: 'GET', path: `/api/v2/watch/${ADDRESS}`, status: 200, response: active },
    { method: 'GET', path: `/api/v2/watch/${ADDRESS}`, status: 200, response: active },
    { method: 'DELETE', path: `/api/v2/watch/${ADDRESS}`, status: 204 },
    { method: 'GET', path: `/api/v2/watch/${ADDRESS}`, status: 200, response: inactive },
    { method: 'GET', path: `/api/v2/watch/${ADDRESS}`, status: 200, response: inactive }
  );
  expect(await watchAddress({ address: ADDRESS }, undefined, TOKEN)).toEqual(watched);
  expect(await getWatchStatus(ADDRESS, undefined, TOKEN)).toEqual(active);
  await restartIndexer();
  expect(await getWatchStatus(ADDRESS, undefined, TOKEN)).toEqual(active);
  expect(await unwatchAddress(ADDRESS, undefined, TOKEN)).toBeNull();
  expect(await getWatchStatus(ADDRESS, undefined, TOKEN)).toEqual(inactive);
  await restartIndexer();
  expect(await getWatchStatus(ADDRESS, undefined, TOKEN)).toEqual(inactive);
  expect(producerReceived).toHaveLength(6);
  expect(responses.map(({ status }) => status)).toEqual([201, 200, 200, 204, 200, 200]);
  expect(responses.every(({ cacheControl }) => cacheControl === 'no-store')).toBe(true);
}

async function webhook_lifecycle_preserves_snake_case_ownership_and_restart_reads() {
  const create = {
    address: ADDRESS,
    callbackUrl: 'https://example.com/events',
    secret: 'test-webhook-signing-secret',
    events: ['balance_change'],
    minDeltaUsd: '1.25',
  };
  const created = {
    id: WEBHOOK_ID,
    address: ADDRESS,
    callback_url: create.callbackUrl,
    events: create.events,
    min_delta_usd: '1.25',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
  };
  const patch = {
    callbackUrl: 'https://example.com/updated-events',
    events: ['balance_change'],
    minDeltaUsd: '2.50',
    isActive: false,
  };
  const updated = {
    ...created,
    callback_url: patch.callbackUrl,
    min_delta_usd: '2.50',
    is_active: false,
  };
  const deliveries = {
    deliveries: [
      {
        id: 'delivery-1',
        webhook_id: WEBHOOK_ID,
        status: 'delivered',
        attempted_at: '2026-01-01T00:00:01Z',
        response_status: 204,
      },
    ],
  };
  const forbidden = {
    error: { code: 'WEBHOOK_NOT_OWNED', message: 'Webhook belongs to another principal' },
  };
  const invalidCallback = {
    error: {
      code: 'CALLBACK_URL_INVALID',
      message: 'Callback URL must be a public HTTPS endpoint',
    },
  };
  const absent = { error: { code: 'WEBHOOK_NOT_FOUND', message: 'Webhook does not exist' } };
  const base = `/api/v2/webhooks/${WEBHOOK_ID}`;
  transcript.push(
    {
      method: 'POST',
      path: '/api/v2/webhooks',
      request: {
        address: ADDRESS,
        callback_url: create.callbackUrl,
        secret: create.secret,
        events: create.events,
        min_delta_usd: '1.25',
      },
      status: 201,
      response: created,
    },
    {
      method: 'GET',
      path: `/api/v2/webhooks?address=${ADDRESS}`,
      status: 200,
      response: { webhooks: [created] },
    },
    { method: 'GET', path: base, status: 200, response: created },
    {
      method: 'PATCH',
      path: base,
      request: {
        callback_url: patch.callbackUrl,
        events: patch.events,
        min_delta_usd: '2.50',
        is_active: false,
      },
      status: 200,
      response: updated,
    },
    { method: 'GET', path: base, status: 200, response: updated },
    { method: 'GET', path: `${base}/deliveries?limit=10`, status: 200, response: deliveries },
    {
      method: 'PATCH',
      path: base,
      token: OTHER_TOKEN,
      request: { is_active: true },
      status: 403,
      response: forbidden,
    },
    {
      method: 'PATCH',
      path: base,
      request: { callback_url: 'http://127.0.0.1/private' },
      status: 422,
      response: invalidCallback,
    },
    { method: 'GET', path: base, status: 200, response: updated },
    { method: 'DELETE', path: base, token: OTHER_TOKEN, status: 403, response: forbidden },
    { method: 'DELETE', path: base, status: 200, response: { status: 'deleted' } },
    { method: 'GET', path: base, status: 404, response: absent }
  );
  expect(await createWebhook(create, undefined, TOKEN)).toEqual(created);
  expect(await listWebhooks(ADDRESS, undefined, TOKEN)).toEqual({ webhooks: [created] });
  expect(await getWebhook(WEBHOOK_ID, undefined, TOKEN)).toEqual(created);
  expect(await updateWebhook(WEBHOOK_ID, patch, undefined, TOKEN)).toEqual(updated);
  await restartIndexer();
  expect(await getWebhook(WEBHOOK_ID, undefined, TOKEN)).toEqual(updated);
  expect(await getWebhookDeliveries(WEBHOOK_ID, 10, undefined, TOKEN)).toEqual(deliveries);
  await expect(
    updateWebhook(WEBHOOK_ID, { isActive: true }, undefined, OTHER_TOKEN)
  ).rejects.toMatchObject({ status: 403, body: forbidden });
  await expect(
    updateWebhook(WEBHOOK_ID, { callbackUrl: 'http://127.0.0.1/private' }, undefined, TOKEN)
  ).rejects.toMatchObject({ status: 422, body: invalidCallback });
  expect(await getWebhook(WEBHOOK_ID, undefined, TOKEN)).toEqual(updated);
  await expect(deleteWebhook(WEBHOOK_ID, undefined, OTHER_TOKEN)).rejects.toMatchObject({
    status: 403,
    body: forbidden,
  });
  expect(await deleteWebhook(WEBHOOK_ID, undefined, TOKEN)).toEqual({ status: 'deleted' });
  await restartIndexer();
  await expect(getWebhook(WEBHOOK_ID, undefined, TOKEN)).rejects.toMatchObject({
    status: 404,
    body: absent,
  });
  expect(producerReceived).toHaveLength(12);
  expect(responses.map(({ status }) => status)).toEqual([
    201, 200, 200, 200, 200, 200, 403, 422, 200, 403, 200, 404,
  ]);
  expect(responses.every(({ cacheControl }) => cacheControl === 'no-store')).toBe(true);
}

async function portfolio_authorization_precedes_body_handling_and_producer_errors_survive() {
  for (const endpoint of ['watch', 'webhooks']) {
    const request = new NextRequest(`http://localhost/wallet/api/wallet/api/v2/${endpoint}`, {
      method: 'POST',
      body: '{invalid JSON',
      headers: { 'Content-Length': '99999999', 'Content-Type': 'application/json' },
    });
    const response = await walletRoute.POST(request, {
      params: Promise.resolve({ path: ['api', 'v2', endpoint] }),
    });
    expect(response.status).toBe(401);
    expect(request.bodyUsed).toBe(false);
    await request.body?.cancel();
  }
  expect(producerReceived).toHaveLength(0);
  const denied = {
    error: { code: 'CREDENTIAL_EXPIRED', message: 'Credential expired' },
    request_id: 'recorded-auth-error',
  };
  const busy = { error: { code: 'WATCH_CAPACITY', message: 'Tracking capacity exceeded' } };
  const unauthorizedWebhook = {
    error: { code: 'CREDENTIAL_REVOKED', message: 'Credential revoked' },
  };
  transcript.push(
    {
      method: 'POST',
      path: '/api/v2/watch',
      request: { address: ADDRESS },
      token: OTHER_TOKEN,
      status: 401,
      response: denied,
    },
    {
      method: 'POST',
      path: '/api/v2/watch',
      request: { address: ADDRESS },
      status: 429,
      response: busy,
      headers: { 'Retry-After': '23' },
    },
    {
      method: 'DELETE',
      path: `/api/v2/webhooks/${WEBHOOK_ID}`,
      token: OTHER_TOKEN,
      status: 401,
      response: unauthorizedWebhook,
    }
  );
  await expect(watchAddress({ address: ADDRESS }, undefined, OTHER_TOKEN)).rejects.toMatchObject({
    status: 401,
    body: denied,
  });
  const limited = await fetch(`${window.location.origin}/wallet/api/wallet/api/v2/watch`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${TOKEN}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ address: ADDRESS }),
  });
  expect(limited.status).toBe(429);
  expect(limited.headers.get('retry-after')).toBe('23');
  expect(await limited.json()).toEqual(busy);
  await expect(deleteWebhook(WEBHOOK_ID, undefined, OTHER_TOKEN)).rejects.toMatchObject({
    status: 401,
    body: unauthorizedWebhook,
  });
}

async function portfolio_mutations_enforce_route_body_query_and_response_bounds() {
  async function invoke(
    method: 'POST' | 'PATCH' | 'DELETE',
    segments: string[],
    body?: string,
    contentType = 'application/json',
    query = ''
  ) {
    const request = new NextRequest(
      `http://localhost/wallet/api/wallet/${segments.join('/')}${query}`,
      {
        method,
        headers: { Authorization: `Bearer ${TOKEN}`, 'Content-Type': contentType },
        ...(body === undefined ? {} : { body }),
      }
    );
    return walletRoute[method](request, { params: Promise.resolve({ path: segments }) });
  }
  const watch = ['api', 'v2', 'watch'];
  expect(
    (await invoke('POST', watch, JSON.stringify({ address: ADDRESS, padding: 'x'.repeat(16_384) })))
      .status
  ).toBe(413);
  expect((await invoke('POST', watch, '{}', 'text/plain')).status).toBe(415);
  expect((await invoke('POST', watch, '{')).status).toBe(400);
  expect((await invoke('POST', watch, '[]')).status).toBe(400);
  expect((await invoke('POST', watch, '{}', 'application/json', '?admin=true')).status).toBe(400);
  expect((await invoke('PATCH', watch, '{}')).status).toBe(405);
  expect((await invoke('POST', ['api', 'v2', 'watch', ADDRESS], '{}')).status).toBe(405);
  expect((await invoke('DELETE', ['api', 'v2', 'webhooks', WEBHOOK_ID], '{}')).status).toBe(400);
  expect((await invoke('POST', ['api', 'v2', '..'], '{}')).status).toBe(400);
  expect(producerReceived).toHaveLength(0);
  const oldOrigin = process.env.PORTFOLIO_UPSTREAM_BASE;
  try {
    delete process.env.PORTFOLIO_UPSTREAM_BASE;
    expect((await invoke('POST', watch, '{}')).status).toBe(503);
    process.env.PORTFOLIO_UPSTREAM_BASE = 'http://untrusted.example';
    expect((await invoke('POST', watch, '{}')).status).toBe(503);
  } finally {
    process.env.PORTFOLIO_UPSTREAM_BASE = oldOrigin;
  }
  transcript.push(
    {
      method: 'POST',
      path: '/api/v2/watch',
      request: {},
      status: 200,
      response: { padding: 'x'.repeat(2_097_152) },
    },
    {
      method: 'POST',
      path: '/api/v2/watch',
      request: {},
      status: 200,
      response: {},
      headers: { 'Content-Type': 'text/html' },
    },
    {
      method: 'POST',
      path: '/api/v2/watch',
      request: {},
      status: 307,
      headers: { Location: 'http://127.0.0.1:1/credential-leak' },
    }
  );
  const oversize = await invoke('POST', watch, '{}');
  expect(oversize.status).toBe(502);
  expect(await oversize.json()).toMatchObject({ error: { code: 'UPSTREAM_TOO_LARGE' } });
  const wrongType = await invoke('POST', watch, '{}');
  expect(wrongType.status).toBe(502);
  expect(await wrongType.json()).toMatchObject({ error: { code: 'UPSTREAM_TYPE_INVALID' } });
  expect((await invoke('POST', watch, '{}')).status).toBe(502);
  expect(producerReceived).toHaveLength(3);
}

test(
  watch_lifecycle_round_trips_and_reads_after_indexer_restart.name,
  watch_lifecycle_round_trips_and_reads_after_indexer_restart
);
test(
  webhook_lifecycle_preserves_snake_case_ownership_and_restart_reads.name,
  webhook_lifecycle_preserves_snake_case_ownership_and_restart_reads
);
test(
  portfolio_authorization_precedes_body_handling_and_producer_errors_survive.name,
  portfolio_authorization_precedes_body_handling_and_producer_errors_survive
);
test(
  portfolio_mutations_enforce_route_body_query_and_response_bounds.name,
  portfolio_mutations_enforce_route_body_query_and_response_bounds
);

// @vitest-environment node

import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import nodeFetch from 'node-fetch';
import type { IncomingHttpHeaders, Server } from 'node:http';
import { createServer } from 'node:http';

import { afterAll, beforeAll, beforeEach, expect, test, vi } from 'vitest';

interface ReceivedRequest {
  url: string;
  method: string | undefined;
  headers: IncomingHttpHeaders;
  body: string;
}

const servers: Array<Server> = [];
const approvedRequests: Array<ReceivedRequest> = [];
const serviceRequests: Array<ReceivedRequest> = [];
const refusedRequests: Array<ReceivedRequest> = [];
const accountHeaders = {
  cookie: '_explorer_key=account-secret; rewards_api_token=rewards-secret; unrelated_session=foreign-secret',
  authorization: 'Bearer account-secret',
  'x-csrf-token': 'csrf-secret',
  'content-type': 'application/json',
};
let approvedOrigin: string;
let serviceOrigin: string;
let refusedOrigin: string;
let proxyOrigin: string;

async function listen(server: Server) {
  servers.push(server);
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Fixture must listen on a TCP port');
  }
  return `http://127.0.0.1:${ address.port }`;
}

function accountService(requests: Array<ReceivedRequest>) {
  return createServer((req, res) => {
    let body = '';
    req.setEncoding('utf8');
    req.on('data', (chunk: string) => body += chunk);
    req.on('end', () => {
      const received = {
        url: req.url || '/',
        method: req.method,
        headers: req.headers,
        body,
      };
      requests.push(received);
      const url = new URL(received.url, 'http://fixture');
      if (url.pathname === '/redirect') {
        res.writeHead(Number(url.searchParams.get('status') || 302), {
          location: url.searchParams.get('location') || '/account',
          'set-cookie': '_explorer_key=redirect-secret; Path=/',
        });
        res.end();
        return;
      }
      if (url.pathname === '/loop') {
        res.writeHead(302, { location: '/loop' });
        res.end();
        return;
      }
      res.writeHead(201, {
        'content-type': 'application/json',
        'x-request-id': 'account-fixture',
        'set-cookie': [
          '_explorer_key=renewed; Domain=fixture; Path=/api; HttpOnly; SameSite=Lax',
          'api_temp_token=renewed-temp; Path=/; Secure',
        ],
      });
      res.end(JSON.stringify({ path: received.url, method: received.method, body: received.body }));
    });
  });
}

beforeAll(async() => {
  approvedOrigin = await listen(accountService(approvedRequests));
  serviceOrigin = await listen(accountService(serviceRequests));
  refusedOrigin = await listen(accountService(refusedRequests));
  const upstream = new URL(approvedOrigin);
  vi.stubEnv('NEXT_PUBLIC_USE_NEXT_JS_PROXY', 'true');
  vi.stubEnv('NEXT_PUBLIC_API_PROTOCOL', upstream.protocol.slice(0, -1));
  vi.stubEnv('NEXT_PUBLIC_API_HOST', upstream.hostname);
  vi.stubEnv('NEXT_PUBLIC_API_PORT', upstream.port);
  vi.stubEnv('NEXT_PUBLIC_STATS_API_HOST', serviceOrigin);
  vi.resetModules();
  await import('configs/app');
  const { 'default': handler } = await import('../../../pages/api/proxy');
  proxyOrigin = await listen(createServer((req, res) => {
    void apiResolver(req, res, {}, { 'default': handler }, {
      dev: false,
      previewModeId: 'repair-fixture',
      previewModeEncryptionKey: 'repair-fixture',
      previewModeSigningKey: 'repair-fixture',
    }, false);
  }));
});

beforeEach(() => {
  approvedRequests.length = 0;
  serviceRequests.length = 0;
  refusedRequests.length = 0;
});

afterAll(async() => {
  vi.unstubAllEnvs();
  vi.resetModules();
  await Promise.all(servers.map((server) => new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
  })));
});

test('proxy-upstream-trust forwards account credentials only to configured upstreams', async function approvedAccountForwarding() {
  for (const selector of [ undefined, 'general', approvedOrigin ]) {
    const response = await nodeFetch(`${ proxyOrigin }/explorer/node-api/proxy/api/account?view=private`, {
      method: 'POST',
      headers: { ...accountHeaders, ...(selector ? { 'x-endpoint': selector } : {}) },
      body: JSON.stringify({ name: 'private tag' }),
    });
    expect(response.status).toBe(201);
    expect(await response.json()).toEqual({ path: '/api/account?view=private', method: 'POST', body: '{"name":"private tag"}' });
    expect(response.headers.get('x-request-id')).toBe('account-fixture');
    expect(response.headers.raw()['set-cookie']).toEqual([
      '_explorer_key=renewed; HttpOnly; SameSite=Lax; Path=/explorer',
      'api_temp_token=renewed-temp; Secure; Path=/explorer',
    ]);
  }
  expect(approvedRequests).toHaveLength(3);
  for (const request of approvedRequests) {
    expect(request.headers.cookie).toBe('_explorer_key=account-secret; rewards_api_token=rewards-secret');
    expect(request.headers.authorization).toBe('Bearer account-secret');
    expect(request.headers['x-csrf-token']).toBe('csrf-secret');
    expect(request.headers['x-endpoint']).toBeUndefined();
  }
  expect(serviceRequests).toHaveLength(0);
  expect(refusedRequests).toHaveLength(0);
});

test('proxy-upstream-trust resolves service identifiers and existing configured endpoint selectors', async function approvedServiceSelection() {
  for (const selector of [ 'stats', serviceOrigin ]) {
    const response = await nodeFetch(`${ proxyOrigin }/node-api/proxy/account`, {
      headers: { ...accountHeaders, 'x-endpoint': selector },
    });
    expect(response.status).toBe(201);
    expect(await response.json()).toEqual({ path: '/account', method: 'GET', body: '' });
  }
  expect(serviceRequests).toHaveLength(2);
  expect(serviceRequests.map(({ headers }) => headers.cookie)).toEqual([
    '_explorer_key=account-secret; rewards_api_token=rewards-secret',
    '_explorer_key=account-secret; rewards_api_token=rewards-secret',
  ]);
  expect(approvedRequests).toHaveLength(0);
  expect(refusedRequests).toHaveLength(0);
});

test('proxy-upstream-trust refuses arbitrary origins and unknown or ambiguous selectors before sending credentials', async function refusedAccountForwarding() {
  for (const selector of [
    refusedOrigin,
    `${ approvedOrigin }.attacker.invalid`,
    `${ approvedOrigin }@attacker.invalid`,
    `${ approvedOrigin }/unapproved-path`,
    'http://169.254.169.254',
    'file:///etc/passwd',
    'unknown',
    'toString',
    'general, stats',
    '',
  ]) {
    const response = await nodeFetch(`${ proxyOrigin }/explorer/node-api/proxy/account`, {
      headers: { ...accountHeaders, 'x-endpoint': selector },
    });
    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: 'Unapproved upstream' });
  }
  expect(approvedRequests).toHaveLength(0);
  expect(serviceRequests).toHaveLength(0);
  expect(refusedRequests).toHaveLength(0);
});

test('proxy-upstream-trust refuses a request path that replaces the configured origin', async function refusedPathAuthority() {
  const response = await nodeFetch(`${ proxyOrigin }/explorer/node-api/proxy//${ new URL(refusedOrigin).host }/account`, {
    headers: accountHeaders,
  });
  expect(response.status).toBe(400);
  expect(await response.json()).toEqual({ error: 'Invalid proxy destination' });
  expect(approvedRequests).toHaveLength(0);
  expect(serviceRequests).toHaveLength(0);
  expect(refusedRequests).toHaveLength(0);
});

test('proxy-upstream-trust refuses unsafe redirects without leaking account credentials or redirect cookies', async function refusedRedirectForwarding() {
  for (const location of [
    `${ refusedOrigin }/account`,
    `//${ new URL(refusedOrigin).host }/account`,
    `${ serviceOrigin }/account`,
    approvedOrigin.replace('http://', 'http://user:password@') + '/account',
    'file:///etc/passwd',
    'http://[invalid',
  ]) {
    const query = new URLSearchParams({ location });
    const response = await nodeFetch(`${ proxyOrigin }/explorer/node-api/proxy/redirect?${ query }`, {
      headers: accountHeaders,
    });
    expect(response.status).toBe(502);
    expect(await response.json()).toEqual({ error: 'Upstream request refused or failed' });
    expect(response.headers.get('set-cookie')).toBeNull();
    expect(response.headers.get('location')).toBeNull();
  }
  expect(approvedRequests).toHaveLength(6);
  expect(approvedRequests.every(({ headers }) => headers.cookie?.includes('_explorer_key=account-secret'))).toBe(true);
  expect(serviceRequests).toHaveLength(0);
  expect(refusedRequests).toHaveLength(0);
});

test('proxy-upstream-trust follows same-origin redirects with the correct account request method and body', async function approvedRedirectForwarding() {
  for (const status of [ 302, 303, 307, 308 ]) {
    const query = new URLSearchParams({ location: '/account?view=private', status: String(status) });
    const response = await nodeFetch(`${ proxyOrigin }/explorer/node-api/proxy/redirect?${ query }`, {
      method: 'POST', headers: accountHeaders, body: '{"name":"private tag"}',
    });
    expect(response.status).toBe(201);
    expect(await response.json()).toEqual({
      path: '/account?view=private',
      method: status === 307 || status === 308 ? 'POST' : 'GET',
      body: status === 307 || status === 308 ? '{"name":"private tag"}' : '',
    });
    expect(response.headers.get('set-cookie')).not.toContain('redirect-secret');
  }
  expect(approvedRequests).toHaveLength(8);
  expect(approvedRequests.every(({ headers }) => headers.authorization === 'Bearer account-secret')).toBe(true);
  expect(refusedRequests).toHaveLength(0);
});

test('proxy-upstream-trust bounds same-origin redirect loops', async function boundedRedirects() {
  const response = await nodeFetch(`${ proxyOrigin }/explorer/node-api/proxy/loop`, { headers: accountHeaders });
  expect(response.status).toBe(502);
  expect(await response.json()).toEqual({ error: 'Upstream request refused or failed' });
  expect(approvedRequests).toHaveLength(6);
  expect(serviceRequests).toHaveLength(0);
  expect(refusedRequests).toHaveLength(0);
});

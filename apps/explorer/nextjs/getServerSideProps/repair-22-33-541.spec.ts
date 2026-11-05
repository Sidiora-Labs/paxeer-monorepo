// @vitest-environment node

import type { GetServerSidePropsContext } from 'next';
import { createServer as createHttpServer } from 'node:http';
import type { Server } from 'node:http';
import { createServer as createHttpsServer } from 'node:https';
import { Writable } from 'node:stream';
import pino from 'pino-http';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { Agent, buildConnector, fetch as realFetch, getGlobalDispatcher, setGlobalDispatcher } from 'undici';

import { httpLogger } from 'nextjs/utils/logger';

import { expect, test } from 'vitest';

import { base } from './handlers';

const serviceKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQghm+MoR39/uGW6J1X
gUXs2fnrmq5urNIEuWuZDMJJyM+hRANCAAQHcThF/6Jc2jLMZPj2aul7ckF+O5H+
g3IHoxNVNCo0FC1OmQoP7m62JQ09s7OsogPh/9J8pCzmpICtx3oHtHeH
-----END PRIVATE KEY-----`;
const serviceCertificate = `-----BEGIN CERTIFICATE-----
MIIBmjCCAUGgAwIBAgIUcUsiWHoEANe3sN6KIGVF9l2w0DgwCgYIKoZIzj0EAwIw
FDESMBAGA1UEAwwJbG9jYWxob3N0MCAXDTI2MTEwNDAyMDgyOFoYDzIxMjYxMDEx
MDIwODI4WjAUMRIwEAYDVQQDDAlsb2NhbGhvc3QwWTATBgcqhkjOPQIBBggqhkjO
PQMBBwNCAAQHcThF/6Jc2jLMZPj2aul7ckF+O5H+g3IHoxNVNCo0FC1OmQoP7m62
JQ09s7OsogPh/9J8pCzmpICtx3oHtHeHo28wbTAdBgNVHQ4EFgQUADailhgC4Gua
DQs5MbYmz0pZPx0wHwYDVR0jBBgwFoAUADailhgC4GuaDQs5MbYmz0pZPx0wDwYD
VR0TAQH/BAUwAwEB/zAaBgNVHREEEzARhwR/AAABgglsb2NhbGhvc3QwCgYIKoZI
zj0EAwIDRwAwRAIgOn0td5ASGoYdiiUyPSIUrBtpLe/BLujDHXpb6+8/c8QCIFzO
mcFTO67rnXHZS4t1cwASvrDsZIGYms6yIRMIqFPV
-----END CERTIFICATE-----`;

type MonitorMode = 'success' | 'unavailable' | 'reset' | 'failure' | 'stall' | 'stream';

const browserHeaders = {
  host: 'explorer.test',
  'user-agent': 'Mozilla/5.0 AppleWebKit/537.36 Chrome/130.0.0.0 Safari/537.36',
  'accept-language': 'en-US',
  'sec-fetch-mode': 'navigate',
  referer: 'https://explorer.test/explorer/',
};

async function listen(server: Server): Promise<number> {
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('The fixture must listen on a TCP port');
  }
  return address.port;
}

async function waitUntil(condition: () => boolean): Promise<void> {
  const deadline = performance.now() + 2_500;
  while (!condition()) {
    if (performance.now() >= deadline) {
      throw new Error('The reporting request did not finish within its bounded lifetime');
    }
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

async function withMonitor(mode: MonitorMode, run: (fixture: {
  pageUrl: string;
  requests: Array<{ method: string | undefined; url: string | undefined; host: string | undefined; contentType: string | undefined; body: string }>;
  logs: () => string;
  closed: () => boolean;
}) => Promise<void>): Promise<void> {
  const originalFetch = globalThis.fetch;
  const originalLogger = httpLogger.logger;
  const originalDispatcher = getGlobalDispatcher();
  const originalTracking = process.env.DISABLE_TRACKING;
  const requests: Array<{ method: string | undefined; url: string | undefined; host: string | undefined; contentType: string | undefined; body: string }> = [];
  const unhandled: Array<unknown> = [];
  const onUnhandled = (error: unknown) => unhandled.push(error);
  let logs = '';
  let closed = false;
  const output = new Writable({
    write(chunk, _encoding, callback) {
      logs += chunk.toString();
      callback();
    },
  });
  const monitor = createHttpsServer({ key: serviceKey, cert: serviceCertificate }, async(req, res) => {
    req.socket.once('close', () => {
      closed = true;
    });
    req.setEncoding('utf8');
    let body = '';
    for await (const chunk of req) {
      body += chunk;
    }
    requests.push({ method: req.method, url: req.url, host: req.headers.host, contentType: req.headers['content-type'], body });
    if (mode === 'reset') {
      req.socket.destroy();
    } else if (mode === 'failure') {
      res.writeHead(503, { 'Content-Type': 'text/plain' });
      res.end('Monitor unavailable');
    } else if (mode === 'stream') {
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.write('No useful response body');
    } else if (mode === 'success') {
      res.writeHead(204);
      res.end();
    }
  });
  const monitorPort = await listen(monitor);
  if (mode === 'unavailable') {
    await new Promise<void>((resolve, reject) => monitor.close((error) => error ? reject(error) : resolve()));
  }

  // Route the real HTTPS client to a TLS-verified loopback peer. No response or transport is mocked.
  const connect = buildConnector({ ca: serviceCertificate });
  const dispatcher = new Agent({
    connect(options, callback) {
      if (options.hostname === 'monitor.blockscout.com') {
        connect({ ...options, hostname: '127.0.0.1', host: '127.0.0.1', port: String(monitorPort), servername: 'localhost' }, callback);
      } else if (options.hostname === 'explorer.test') {
        connect({ ...options, hostname: '127.0.0.1', host: '127.0.0.1', port: String(pagePort) }, callback);
      } else if (options.hostname === '127.0.0.1') {
        connect(options, callback);
      } else {
        callback(new Error(`Unexpected external connection: ${ options.hostname }`), null);
      }
    },
  });
  const page = createHttpServer(async(req, res) => {
    try {
      const context: GetServerSidePropsContext = {
        req: Object.assign(req, { cookies: {} }),
        res,
        query: { page: '1' },
        resolvedUrl: '/explorer/?page=1',
      };
      const result = await base(context);
      if (!('props' in result)) {
        throw new Error('Expected renderable page props');
      }
      const props = await result.props;
      if (!props.query) {
        throw new Error('Expected the SSR query in the page props');
      }
      res.setHeader('Content-Type', 'text/html');
      res.end(renderToStaticMarkup(React.createElement('main', { 'data-uuid': props.uuid }, `Page ${ props.query.page }`)));
    } catch (error) {
      res.statusCode = 500;
      res.end(String(error));
    }
  });
  const pagePort = await listen(page);
  process.on('unhandledRejection', onUnhandled);
  delete process.env.DISABLE_TRACKING;
  Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: realFetch });
  httpLogger.logger = pino({}, output).logger;
  setGlobalDispatcher(dispatcher);

  try {
    await run({ pageUrl: 'http://explorer.test/explorer/?page=1', requests, logs: () => logs, closed: () => closed });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(unhandled).toEqual([]);
  } finally {
    await dispatcher.destroy();
    monitor.closeAllConnections();
    page.closeAllConnections();
    await new Promise<void>((resolve, reject) => page.close((error) => error ? reject(error) : resolve()));
    if (mode !== 'unavailable') {
      await new Promise<void>((resolve, reject) => monitor.close((error) => error ? reject(error) : resolve()));
    }
    setGlobalDispatcher(originalDispatcher);
    Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: originalFetch });
    httpLogger.logger = originalLogger;
    if (originalTracking === undefined) {
      delete process.env.DISABLE_TRACKING;
    } else {
      process.env.DISABLE_TRACKING = originalTracking;
    }
    process.off('unhandledRejection', onUnhandled);
    output.end();
  }
}

async function renderPage(pageUrl: string, headers = browserHeaders) {
  const response = await realFetch(pageUrl, { headers });
  expect(response.status).toBe(200);
  expect(response.headers.get('content-type')).toBe('text/html');
  const html = await response.text();
  expect(html).toMatch(/^<main data-uuid="[0-9a-f-]{36}">Page 1<\/main>$/);
  const cookie = response.headers.get('set-cookie');
  expect(cookie).toMatch(/^uuid=[0-9a-f-]{36}; Path=\/explorer(?:; Secure)?$/);
  const uuid = cookie?.split(';')[0].slice('uuid='.length);
  expect(html).toContain(`data-uuid="${ uuid }"`);
  return uuid;
}

async function repairTelemetryRejectionKeepsPageRenderable() {
  for (const mode of [ 'unavailable', 'reset' ] as const) {
    await withMonitor(mode, async({ pageUrl, requests, logs }) => {
      await renderPage(pageUrl);
      await waitUntil(() => logs().includes('Pageview reporting failed'));
      expect(logs()).toContain('fetch failed');
      expect(requests).toHaveLength(mode === 'reset' ? 1 : 0);
    });
  }
}

async function repairTelemetryHttpFailureIsHandled() {
  await withMonitor('failure', async({ pageUrl, requests, logs }) => {
    await renderPage(pageUrl);
    await waitUntil(() => logs().includes('Pageview reporting failed'));
    expect(logs()).toContain('"code":503');
    expect(requests).toHaveLength(1);
  });
}

async function repairTelemetryTimeoutAbortsWithoutBlockingPage() {
  await withMonitor('stall', async({ pageUrl, requests, logs, closed }) => {
    await renderPage(pageUrl);
    await waitUntil(() => requests.length === 1);
    expect(logs()).toBe('');
    expect(closed()).toBe(false);
    await waitUntil(() => closed() && logs().includes('Pageview reporting failed'));
    expect(logs()).toContain('AbortError');
    expect(requests).toHaveLength(1);
  });
}

async function repairTelemetrySuccessPreservesPayloadAndReleasesBody() {
  for (const mode of [ 'success', 'stream' ] as const) {
    await withMonitor(mode, async({ pageUrl, requests, logs, closed }) => {
      const uuid = await renderPage(pageUrl);
      await waitUntil(() => requests.length === 1);
      const request = requests[0];
      expect(request).toMatchObject({ method: 'POST', url: '/count', host: 'monitor.blockscout.com', contentType: 'application/json' });
      const payload: unknown = JSON.parse(request.body);
      expect(payload).toEqual({
        hostname: 'explorer.test',
        timestamp: expect.any(String),
        chainId: process.env.NEXT_PUBLIC_NETWORK_ID,
        chainName: process.env.NEXT_PUBLIC_NETWORK_NAME,
        publicRPC: process.env.NEXT_PUBLIC_NETWORK_RPC_URL,
        uuid,
      });
      if (mode === 'stream') {
        await waitUntil(closed);
      }
      expect(logs()).toBe('');
    });
  }
}

async function repairTelemetryRespectsTrackingGuards() {
  await withMonitor('success', async({ pageUrl, requests, logs }) => {
    for (const headers of [
      { ...browserHeaders, 'x-app-profile': 'private' },
      { ...browserHeaders, cookie: 'app_profile=private' },
      { ...browserHeaders, 'user-agent': 'Googlebot' },
    ]) {
      const response = await realFetch(pageUrl, { headers });
      expect(response.status).toBe(200);
      expect(await response.text()).toContain('Page 1');
    }
    process.env.DISABLE_TRACKING = 'true';
    await renderPage(pageUrl);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(requests).toHaveLength(0);
    expect(logs()).toBe('');
  });
}

test(repairTelemetryRejectionKeepsPageRenderable.name, repairTelemetryRejectionKeepsPageRenderable);
test(repairTelemetryHttpFailureIsHandled.name, repairTelemetryHttpFailureIsHandled);
test(repairTelemetryTimeoutAbortsWithoutBlockingPage.name, repairTelemetryTimeoutAbortsWithoutBlockingPage);
test(repairTelemetrySuccessPreservesPayloadAndReleasesBody.name, repairTelemetrySuccessPreservesPayloadAndReleasesBody);
test(repairTelemetryRespectsTrackingGuards.name, repairTelemetryRespectsTrackingGuards);

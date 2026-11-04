// @vitest-environment node

import type { GetServerSidePropsContext } from 'next';
import { NextRequest } from 'next/server';
import nodeFetch from 'node-fetch';
import { subscribe, unsubscribe } from 'node:diagnostics_channel';
import type { ClientRequest, Server } from 'node:http';
import { createServer } from 'node:http';

import { afterAll, beforeAll, expect, test } from 'vitest';

const originalFetch = globalThis.fetch;
const fixtureEnvs = {
  NEXT_PUBLIC_GOOGLE_ANALYTICS_PROPERTY_ID: 'G-REPAIR-FIXTURE',
  NEXT_PUBLIC_MIXPANEL_PROJECT_TOKEN: 'repair-fixture',
  NEXT_PUBLIC_MARKETPLACE_ENABLED: 'false',
  NEXT_PUBLIC_MULTICHAIN_ENABLED: 'false',
  DISABLE_TRACKING: 'false',
};
const originalEnvs = Object.fromEntries(Object.keys(fixtureEnvs).map((key) => [ key, process.env[key] ]));
const outgoingRequests: Array<ClientRequest> = [];
let server: Server;
let origin: string;

function observeTelemetry(message: unknown) {
  const { request } = message as { request: ClientRequest };
  outgoingRequests.push(request);
}

beforeAll(async() => {
  Object.assign(process.env, fixtureEnvs);
  Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: nodeFetch });
  subscribe('http.client.request.start', observeTelemetry);
  const { base } = await import('nextjs/getServerSideProps/handlers');

  server = createServer(async(req, res) => {
    try {
      const url = new URL(req.url || '/', origin);
      const requestHeaders = new Headers();
      for (const [ name, value ] of Object.entries(req.headers)) {
        if (value !== undefined) {
          requestHeaders.set(name, Array.isArray(value) ? value.join(', ') : value);
        }
      }
      const parsedRequest = new NextRequest(url, { headers: requestHeaders });
      const context: GetServerSidePropsContext = {
        req: Object.assign(req, { cookies: Object.fromEntries(parsedRequest.cookies.getAll().map(({ name, value }) => [ name, value ])) }),
        res,
        query: Object.fromEntries(url.searchParams),
        resolvedUrl: url.pathname + url.search,
      };
      const result = await base(context);
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(result));
    } catch (error) {
      res.statusCode = 500;
      res.end(String(error));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('SSR fixture must listen on a TCP port');
  }
  origin = `http://127.0.0.1:${ address.port }`;
});

afterAll(async() => {
  unsubscribe('http.client.request.start', observeTelemetry);
  Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: originalFetch });
  for (const [ key, value ] of Object.entries(originalEnvs)) {
    if (value === undefined) {
      delete process.env[key];
    } else {
      process.env[key] = value;
    }
  }
  if (server) {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
});

async function navigate(path: string, headers: Record<string, string>, expectedProfile: 'private' | 'default') {
  const { middleware } = await import('middleware');
  const cookies = await import('lib/cookies');
  const request = new NextRequest(`${ origin }${ path }`, {
    headers: {
      accept: 'text/html',
      'accept-language': 'en-US',
      'sec-fetch-mode': 'navigate',
      'user-agent': expectedProfile === 'private' ? 'Mozilla/5.0 AppleWebKit/537.36 Chrome/130.0 Safari/537.36' : 'repair-bot',
      ...headers,
    },
  });
  const response = await middleware(request);
  if (!response) {
    throw new Error('Page request must produce a middleware response');
  }

  expect(request.headers.get('x-app-profile')).toBe(expectedProfile);
  const overrideHeader = response.headers.get('x-middleware-override-headers');
  if (!overrideHeader) {
    throw new Error('SSR must receive the resolved profile through Next.js request overrides');
  }
  const overrides = overrideHeader.split(',').map((name) => name.trim());
  expect(overrides).toContain('x-app-profile');
  const forwardedHeaders: Record<string, string> = {};
  for (const name of overrides) {
    const value = response.headers.get(`x-middleware-request-${ name }`);
    if (value === null) {
      throw new Error(`Missing Next.js override for ${ name }`);
    }
    forwardedHeaders[name] = value;
  }
  expect(forwardedHeaders['x-app-profile']).toBe(expectedProfile);
  expect(forwardedHeaders.accept).toBe('text/html');

  const profileCookie = response.cookies.get(cookies.NAMES.APP_PROFILE);
  expect(profileCookie?.path).toBe(cookies.getDefaultAttributes().path);
  if (expectedProfile === 'private') {
    expect(profileCookie?.value).toBe('private');
    expect(profileCookie?.expires).toBeUndefined();
  } else {
    expect(profileCookie?.value).toBe('');
    const expires = profileCookie?.expires;
    if (!(expires instanceof Date)) {
      throw new Error('Clearing a profile must emit a Date expiry');
    }
    expect(expires.getTime()).toBe(0);
  }

  const policy = response.headers.get('Content-Security-Policy');
  expect(policy).toBeTruthy();
  for (const source of [ '*.google-analytics.com', 'https://www.googletagmanager.com', '*.mixpanel.com' ]) {
    if (expectedProfile === 'private') {
      expect(policy).not.toContain(source);
    } else {
      expect(policy).toContain(source);
    }
  }

  const requestOffset = outgoingRequests.length;
  const ssr = await nodeFetch(`${ origin }${ path }`, { headers: forwardedHeaders });
  expect(ssr.status).toBe(200);
  const result = await ssr.json();
  expect(result).toHaveProperty('props');
  if (expectedProfile === 'private') {
    expect(result).not.toHaveProperty('props.uuid');
    expect(ssr.headers.get('set-cookie')).toBeNull();
  } else {
    expect(ssr.headers.get('set-cookie')).toContain(`${ cookies.NAMES.UUID }=`);
    expect(result).toHaveProperty('props.uuid', expect.stringMatching(/^[0-9a-f-]{36}$/));
  }
  expect(outgoingRequests.slice(requestOffset).map((outgoing) => outgoing.getHeader('host'))).toEqual([ new URL(origin).host ]);

  return { response, cookie: `${ cookies.NAMES.APP_PROFILE }=${ profileCookie?.value }` };
}

async function repairPrivateProfileQueryAndCookieNavigation() {
  const { 'default': config } = await import('configs/app');
  expect(config.features.googleAnalytics.isEnabled).toBe(true);
  expect(config.features.mixpanel.isEnabled).toBe(true);
  const entry = await navigate('/explorer/blocks?app-profile=private', {}, 'private');
  const next = await navigate('/explorer/txs', { cookie: entry.cookie }, 'private');
  await navigate('/explorer/blocks', { cookie: next.cookie }, 'private');
}

async function repairPrivateProfileHeaderOverrides() {
  const cookies = await import('lib/cookies');
  const cookie = `${ cookies.NAMES.APP_PROFILE }=private`;
  await navigate('/explorer/blocks?app-profile=default', { 'x-app-profile': 'private' }, 'private');
  await navigate('/explorer/blocks?app-profile=private', { 'x-app-profile': 'default', cookie }, 'default');
  await navigate('/explorer/blocks?app-profile=default', { cookie }, 'default');
  await navigate('/explorer/blocks', { 'x-app-profile': 'private', cookie: `${ cookies.NAMES.APP_PROFILE }=default` }, 'private');
}

async function repairPrivateProfileMissingAndInvalidCookie() {
  const cookies = await import('lib/cookies');
  await navigate('/explorer/blocks', {}, 'default');
  await navigate('/explorer/blocks', { cookie: `${ cookies.NAMES.APP_PROFILE }=invalid` }, 'default');
}

test(repairPrivateProfileQueryAndCookieNavigation, repairPrivateProfileQueryAndCookieNavigation);
test(repairPrivateProfileHeaderOverrides, repairPrivateProfileHeaderOverrides);
test(repairPrivateProfileMissingAndInvalidCookie, repairPrivateProfileMissingAndInvalidCookie);

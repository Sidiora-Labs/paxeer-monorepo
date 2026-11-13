// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import { readFileSync } from 'node:fs';
import type { ServerResponse } from 'node:http';
import path from 'node:path';
import React from 'react';

import ts from 'typescript';

import * as queries from 'lib/api/useApiQuery';
import * as cookies from 'lib/cookies';
import * as profile from 'mocks/user/profile';
import * as fixtures from 'ui/shared/layout/testWrapper';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest';
import * as testing from 'vitest/lib';

import NavigationDesktopAuthScene from './NavigationDesktopAuthScene';

const { server, requests } = await vi.hoisted(async() => {
  const { createServer } = await import('node:http');
  const requests: Array<{ url: string | undefined; method: string | undefined; response: ServerResponse }> = [];
  const server = createServer((request, response) => {
    requests.push({ url: request.url, method: request.method, response });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Profile fixture must listen on a loopback TCP port');
  }

  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'true',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
  return { server, requests };
});

let client: QueryClient;
const fetchMocker = globalThis.fetchMock;

beforeAll(() => {
  fetchMocker.disableMocks();
});

beforeEach(() => {
  requests.length = 0;
  fixtures.routerState.pathname = '/blocks';
  fixtures.routerState.query = {};
  cookies.set(cookies.NAMES.API_TOKEN, 'foo', { path: '/' });
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
});

afterEach(async() => {
  testing.cleanup();
  await client.cancelQueries();
  client.clear();
  cookies.remove(cookies.NAMES.API_TOKEN, { path: '/' });
});

afterAll(async() => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMocker.enableMocks();
});

function renderAuthenticatedNavigation() {
  return fixtures.render(
    React.createElement(QueryClientProvider, { client },
      React.createElement(RouterContext.Provider, { value: fixtures.nextRouterModule().useRouter() },
        React.createElement(NavigationDesktopAuthScene),
      ),
    ),
  );
}

async function authNavigationWaitsForProfile() {
  const { container } = renderAuthenticatedNavigation();
  const root = container.querySelector('[aria-busy]');
  const key = queries.getResourceKey('general:user_info');

  expect(root).not.toBeNull();
  await testing.waitFor(() => expect(requests).toHaveLength(1));
  expect(requests[0].url).toBe('/api/account/v2/user/info');
  expect(requests[0].method).toBe('GET');
  expect(root?.getAttribute('aria-busy')).toBe('true');
  expect(client.getQueryState(key)?.status).toBe('pending');
  expect(client.getQueryData(key)).toBeUndefined();

  requests[0].response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(profile.base));

  await testing.waitFor(() => expect(root?.getAttribute('aria-busy')).toBe('false'));
  expect(client.getQueryData(key)).toEqual(profile.base);
  expect(client.getQueryState(key)?.fetchStatus).toBe('idle');
  expect(container.querySelector('header')).not.toBeNull();
  for (const [ name, href ] of [
    [ 'Watch list', '/account/watchlist' ],
    [ 'Private tags', '/account/tag-address' ],
    [ 'API keys', '/account/api-key' ],
    [ 'Custom ABI', '/account/custom-abi' ],
  ]) {
    const link = testing.within(container).getByLabelText(`${ name } link`, { exact: true });
    expect(link.tagName).toBe('A');
    expect(link.getAttribute('href')).toBe(href);
  }
}

async function authNavigationRejectsFailedProfile() {
  const { container } = renderAuthenticatedNavigation();
  const root = container.querySelector('[aria-busy]');
  const key = queries.getResourceKey('general:user_info');

  expect(root).not.toBeNull();
  await testing.waitFor(() => expect(requests).toHaveLength(1));
  requests[0].response.writeHead(401, { 'Content-Type': 'application/json' }).end(JSON.stringify(profile.base));

  await testing.waitFor(() => expect(client.getQueryState(key)?.status).toBe('error'));
  expect(client.getQueryState(key)?.fetchStatus).toBe('idle');
  expect(client.getQueryData(key)).toBeUndefined();
  expect(root?.getAttribute('aria-busy')).toBe('true');
}

function authNavigationScreenshotsRemainEnabled() {
  const source = readFileSync(path.join(__dirname, 'NavigationDesktop.pw.tsx'), 'utf8');
  const file = ts.createSourceFile('NavigationDesktop.pw.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const calls: Array<ts.CallExpression> = [];
  function visit(node: ts.Node) {
    if (ts.isCallExpression(node)) {
      calls.push(node);
    }
    ts.forEachChild(node, visit);
  }
  visit(file);

  const authSuite = calls.find((call) =>
    call.arguments[0] && ts.isStringLiteral(call.arguments[0]) && call.arguments[0].text === 'auth',
  );
  expect(authSuite?.expression.getText(file)).toBe('authTest.describe');
  if (!authSuite) {
    throw new Error('Authenticated screenshot suite must be declared');
  }

  const suiteCalls = calls.filter((call) => call.pos > authSuite.pos && call.end < authSuite.end);
  const cases = suiteCalls.filter((call) => call.expression.getText(file) === 'authTest');
  expect(cases).toHaveLength(2);
  expect(cases.map((call) => call.arguments[0].getText(file))).toEqual([ '\'+@dark-mode\'', '\'+@dark-mode\'' ]);
  for (const screenshotCase of cases) {
    expect(screenshotCase.getText(file)).toContain('await expect(component).toHaveScreenshot()');
  }
  expect(suiteCalls.some((call) => call.getText(file) === 'authTest.use({ viewport: pwConfig.viewport.xl })')).toBe(true);
  expect(suiteCalls.some((call) => /\.(?:skip|fixme|fail|only)$/.test(call.expression.getText(file)))).toBe(false);
  expect(source).toContain('context: contextWithAuth');
  expect(authSuite.getText(file)).toContain('await context.cookies()');
  expect(authSuite.getText(file)).toContain('await mockApiResponse(\'general:user_info\', profileMock.base)');
  expect(authSuite.getText(file)).toContain('<NavigationDesktopAuthScene/>');
  expect(authSuite.getText(file)).toContain('await expect(component).toHaveAttribute(\'aria-busy\', \'false\')');
  expect(authSuite.getText(file)).toContain('await document.fonts.ready');
}

test(authNavigationWaitsForProfile, authNavigationWaitsForProfile);
test(authNavigationRejectsFailedProfile, authNavigationRejectsFailedProfile);
test(authNavigationScreenshotsRemainEnabled, authNavigationScreenshotsRemainEnabled);

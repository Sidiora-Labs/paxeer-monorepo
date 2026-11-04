// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, render, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { GetActivityRewardsResponse } from '@blockscout/points-types';

import { base as populatedActivity } from 'mocks/rewards/activity';
import { base as balances } from 'mocks/rewards/balance';
import { base as dailyReward } from 'mocks/rewards/dailyReward';
import { base as referrals } from 'mocks/rewards/referrals';
import { base as rewardsConfig } from 'mocks/rewards/rewardsConfig';
import { afterAll, afterEach, expect, test } from 'vitest';

const address = '0x1111111111111111111111111111111111111111';
const token = `${ btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' })) }.${ btoa(JSON.stringify({ sub: address })) }.fixture`;
const emptyActivity: GetActivityRewardsResponse = { items: [], last_week: [] };
let activity = emptyActivity;
const activityRequests: Array<{ authorization: string | undefined }> = [];
const unexpectedRequests: Array<string> = [];
const responses = new Map<string, unknown>([
  [ '/api/account/v2/user/info', { email: null, address_hash: address } ],
  [ '/api/v1/config', rewardsConfig ],
  [ '/api/v1/user/balances', balances ],
  [ '/api/v1/user/daily/check', dailyReward ],
  [ '/api/v1/user/referrals', referrals ],
  [ '/api/v1/activity/check-pass', { is_valid: true } ],
  [ '/api/v1/instances', { items: [] } ],
]);
const server = createServer((request, response) => {
  const path = new URL(request.url || '/', 'http://localhost').pathname;
  response.setHeader('content-type', 'application/json');
  if (path === '/api/v1/user/activity/rewards') {
    activityRequests.push({ authorization: request.headers.authorization });
    response.end(JSON.stringify(activity));
  } else if (responses.has(path)) {
    response.end(JSON.stringify(responses.get(path)));
  } else {
    unexpectedRequests.push(path);
    response.writeHead(404).end(JSON.stringify({ message: 'Unexpected fixture request' }));
  }
});
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
const { port } = server.address() as AddressInfo;
const previousEnvs = window.__envs;
window.__envs = {
  ...previousEnvs,
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String(port),
  NEXT_PUBLIC_REWARDS_SERVICE_API_HOST: `http://127.0.0.1:${ port }`,
  NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID: 'local-rewards-fixture',
};
fetchMock.disableMocks();
const previousReact = Object.getOwnPropertyDescriptor(globalThis, 'React');
Object.defineProperty(globalThis, 'React', { configurable: true, value: React });

const { 'default': ActivityTab } = await import('./ActivityTab');
const { RewardsContextProvider, useRewardsContext } = await import('lib/contexts/rewards');
const { getResourceKey } = await import('lib/api/useApiQuery');
const cookies = await import('lib/cookies');
const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': App } = await import('next/app');
const { ChakraProvider } = await import('@chakra-ui/react');
const { 'default': theme } = await import('toolkit/theme/theme');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { mainnet } = await import('wagmi/chains');
await import('ui/shared/layout/testWrapper');

function AuthenticatedActivity() {
  const { isAuth } = useRewardsContext();
  return React.createElement(React.Fragment, null,
    React.createElement('output', { 'data-testid': 'rewards-auth' }, String(isAuth)),
    React.createElement(ActivityTab));
}

function mountActivity() {
  activityRequests.length = 0;
  unexpectedRequests.length = 0;
  cookies.set(cookies.NAMES.API_TOKEN, 'local-profile-fixture', { path: '/' });
  cookies.set(cookies.NAMES.REWARDS_API_TOKEN, token, { path: '/' });
  window.__NEXT_DATA__ = { props: {}, page: '/account/merits', query: {}, buildId: 'repair-22-33-452' };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  const router = new Router('/account/merits', {}, '/account/merits', {
    initialProps: {},
    pageLoader: new PageLoader('repair-22-33-452', ''),
    App,
    Component: ActivityTab,
    wrapApp: (component) => component,
    subscription: async() => {},
    isFallback: false,
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  const wagmiConfig = createConfig({ chains: [ mainnet ], transports: { [mainnet.id]: http(`http://127.0.0.1:${ port }`) } });
  const view = render(React.createElement(RouterContext.Provider, { value: router },
    React.createElement(WagmiProvider, { config: wagmiConfig },
      React.createElement(QueryClientProvider, { client },
        React.createElement(ChakraProvider, {
          value: theme,
          children: React.createElement(RewardsContextProvider, { children: React.createElement(AuthenticatedActivity) }),
        })))));
  return {
    client,
    view,
    dispose: () => {
      view.unmount();
      client.clear();
      window.removeEventListener('popstate', router.onPopState);
    },
  };
}

afterEach(() => {
  cleanup();
  cookies.remove(cookies.NAMES.API_TOKEN, { path: '/' });
  cookies.remove(cookies.NAMES.REWARDS_API_TOKEN, { path: '/' });
});

afterAll(async() => {
  window.__envs = previousEnvs;
  if (previousReact) {
    Object.defineProperty(globalThis, 'React', previousReact);
  } else {
    Reflect.deleteProperty(globalThis, 'React');
  }
  fetchMock.enableMocks();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

test('empty rewards activity renders the empty period and zero activity metrics', async function emptyRewardsActivity() {
  activity = emptyActivity;
  const { client, view, dispose } = mountActivity();
  try {
    await waitFor(() => {
      expect(view.getByTestId('rewards-auth').textContent).toBe('true');
      expect(client.getQueryState(getResourceKey('rewards:user_activity'))?.status).toBe('success');
      expect(client.getQueryState(getResourceKey('rewards:user_check_activity_pass', { queryParams: { address } }))?.status).toBe('success');
      expect(view.getByText('Period: —')).toBeTruthy();
      expect(view.getByText('—')).toBeTruthy();
      expect(view.getAllByText('0%')).toHaveLength(3);
      expect(view.getAllByRole('heading', { level: 3 }).filter((heading) => heading.textContent === '0')).toHaveLength(3);
      expect(view.getAllByText('+0% vs previous week')).toHaveLength(3);
      expect(view.getAllByText('+0 vs previous week')).toHaveLength(3);
    });
    expect(client.getQueryData(getResourceKey('rewards:user_activity'))).toEqual(emptyActivity);
    expect(activityRequests).toEqual([ { authorization: `Bearer ${ token }` } ]);
    expect(unexpectedRequests).toEqual([]);
    expect(view.getByText('Your activity')).toBeTruthy();
    expect(view.getByText('Blockscout activity')).toBeTruthy();
    expect(view.getByText('Contracts verification')).toBeTruthy();
    expect(view.getByText('Blockscout usage')).toBeTruthy();
    expect(view.container.textContent).not.toMatch(/undefined|Invalid Date|NaN/);
  } finally {
    dispose();
  }
});

test('a populated rewards period remains correct and can become empty on refresh', async function rewardsPeriodBecomesEmpty() {
  activity = populatedActivity;
  const { client, view, dispose } = mountActivity();
  try {
    await waitFor(() => {
      expect(view.getByTestId('rewards-auth').textContent).toBe('true');
      expect(client.getQueryState(getResourceKey('rewards:user_activity'))?.status).toBe('success');
      expect(view.getByText('Period: Mar 10 - Mar 16, 2025')).toBeTruthy();
      expect(view.getByText('50%')).toBeTruthy();
      expect(view.getByText('30%')).toBeTruthy();
      expect(view.getByText('80%')).toBeTruthy();
    });
    activity = emptyActivity;
    await act(async() => {
      await client.invalidateQueries({ queryKey: getResourceKey('rewards:user_activity') });
    });
    await waitFor(() => {
      expect(view.getByText('Period: —')).toBeTruthy();
      expect(view.getAllByText('0%')).toHaveLength(3);
    });
    expect(client.getQueryData(getResourceKey('rewards:user_activity'))).toEqual(emptyActivity);
    expect(activityRequests).toEqual([ { authorization: `Bearer ${ token }` }, { authorization: `Bearer ${ token }` } ]);
    expect(unexpectedRequests).toEqual([]);
  } finally {
    dispose();
  }
});

// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { once } from 'node:events';
import { createServer } from 'node:http';
import React from 'react';
import type { Address } from 'viem';
import { mainnet } from 'viem/chains';
import { createConfig, http, useAccount, WagmiProvider } from 'wagmi';
import { injected } from 'wagmi/connectors';

import type { UserInfo } from 'types/api/account';

import type useProfileQueryHook from 'ui/snippets/auth/useProfileQuery';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';

import useLoginAddressMismatch from './useLoginAddressMismatch';

const checksumAddress = '0x52908400098527886E0F7030069857D2E4169EE7';
const lowercaseAddress = '0x52908400098527886e0f7030069857d2e4169ee7';
const otherAddress = '0xde709f2102306220921060314715629080e2fb77';
let profile: UserInfo;
const exchanges: Array<{ path: string; profile: UserInfo }> = [];
const clients: Array<QueryClient> = [];
let origin: string;
let previousEnvs: typeof window.__envs;
let useProfileQuery: typeof useProfileQueryHook;

const server = createServer((request, response) => {
  const path = new URL(request.url || '/', 'http://localhost').pathname;
  response.setHeader('content-type', 'application/json');
  if (request.method !== 'GET' || path !== '/api/account/v2/user/info') {
    response.writeHead(404);
    response.end(JSON.stringify({ message: 'Unexpected fixture resource' }));
    return;
  }
  exchanges.push({ path, profile: { ...profile } });
  response.end(JSON.stringify(profile));
});

beforeAll(async() => {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Account fixture server did not bind a port');
  }
  origin = `http://127.0.0.1:${ address.port }`;
  previousEnvs = window.__envs;
  window.__envs = {
    ...previousEnvs,
    NEXT_PUBLIC_APP_ENV: 'testing',
    NEXT_PUBLIC_APP_INSTANCE: 'vitest',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'true',
    NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY: 'account-fixture',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_API_BASE_PATH: '/',
    NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID: '',
  };
  fetchMock.disableMocks();
  ({ 'default': useProfileQuery } = await import('ui/snippets/auth/useProfileQuery'));
});

beforeEach(() => {
  profile = { name: 'Wallet account', email: null, address_hash: lowercaseAddress };
  exchanges.length = 0;
  document.cookie = '_explorer_key=account-session; Path=/';
});

afterEach(() => {
  cleanup();
  clients.forEach((client) => client.clear());
  clients.length = 0;
  document.cookie = '_explorer_key=; Max-Age=0; Path=/';
});

afterAll(async() => {
  fetchMock.enableMocks();
  window.__envs = previousEnvs;
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function openLoginAddressCheck(walletAddress: Address = checksumAddress) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const wallet = createConfig({
    chains: [ mainnet ],
    connectors: [ injected() ],
    transports: { [mainnet.id]: http(origin) },
    storage: null,
    multiInjectedProviderDiscovery: false,
  });
  const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(QueryClientProvider, { client },
    React.createElement(WagmiProvider, { config: wallet, reconnectOnMount: false }, children),
  );
  const hook = renderHook(() => {
    const account = useAccount();
    const accountProfile = useProfileQuery();
    const isAddressMismatch = useLoginAddressMismatch(account.address, accountProfile.data?.address_hash);
    return { account, accountProfile, isAddressMismatch };
  }, { wrapper });
  const connector = wallet.connectors[0];
  act(() => wallet.setState({
    chainId: mainnet.id,
    connections: new Map([ [ connector.uid, {
      accounts: [ walletAddress ],
      chainId: mainnet.id,
      connector,
    } ] ]),
    current: connector.uid,
    status: 'connected',
  }));
  return { ...hook, client, wallet };
}

test('walletAddressCaseAcceptsLowercaseProfileAndChecksumConnector', async function walletAddressCaseAcceptsLowercaseProfileAndChecksumConnector() {
  const login = openLoginAddressCheck();
  await waitFor(() => expect(login.result.current.accountProfile.isSuccess).toBe(true));
  expect(login.result.current.account.isConnected).toBe(true);
  expect(login.result.current.account.address).toBe(checksumAddress);
  expect(login.result.current.accountProfile.data?.address_hash).toBe(lowercaseAddress);
  expect(login.result.current.isAddressMismatch).toBe(false);
  expect(exchanges).toEqual([ { path: '/api/account/v2/user/info', profile } ]);
});

test('walletAddressCaseRefusesDifferentWallet', async function walletAddressCaseRefusesDifferentWallet() {
  const login = openLoginAddressCheck(otherAddress);
  await waitFor(() => expect(login.result.current.accountProfile.isSuccess).toBe(true));
  expect(login.result.current.account.isConnected).toBe(true);
  expect(login.result.current.account.address).toBe(otherAddress);
  expect(login.result.current.accountProfile.data?.address_hash).toBe(lowercaseAddress);
  expect(login.result.current.isAddressMismatch).toBe(true);
  expect(exchanges).toEqual([ { path: '/api/account/v2/user/info', profile } ]);
});

test('walletAddressCaseRechecksUpdatedProfile', async function walletAddressCaseRechecksUpdatedProfile() {
  const login = openLoginAddressCheck();
  await waitFor(() => expect(login.result.current.accountProfile.isSuccess).toBe(true));
  expect(login.result.current.isAddressMismatch).toBe(false);
  profile = { ...profile, address_hash: otherAddress };
  await act(async() => {
    await login.result.current.accountProfile.refetch();
  });
  await waitFor(() => expect(login.result.current.accountProfile.data?.address_hash).toBe(otherAddress));
  expect(login.result.current.isAddressMismatch).toBe(true);
  profile = { ...profile, address_hash: checksumAddress };
  await act(async() => {
    await login.result.current.accountProfile.refetch();
  });
  await waitFor(() => expect(login.result.current.accountProfile.data?.address_hash).toBe(checksumAddress));
  expect(login.result.current.isAddressMismatch).toBe(false);
  expect(exchanges.map((exchange) => exchange.profile.address_hash)).toEqual([ lowercaseAddress, otherAddress, checksumAddress ]);
});

test('walletAddressCaseRejectsMalformedProfile', async function walletAddressCaseRejectsMalformedProfile() {
  profile = { ...profile, address_hash: '0x52908400098527886e0f7030069857d2e4169eeg' };
  const login = openLoginAddressCheck();
  await waitFor(() => expect(login.result.current.accountProfile.isSuccess).toBe(true));
  expect(login.result.current.accountProfile.data?.address_hash).toBe(profile.address_hash);
  expect(login.result.current.isAddressMismatch).toBe(true);
  expect(exchanges).toEqual([ { path: '/api/account/v2/user/info', profile } ]);
});

test('walletAddressCaseValidatesBothInputsBeforeComparing', function walletAddressCaseValidatesBothInputsBeforeComparing() {
  const invalidAddresses = [ '0x1234', `${ lowercaseAddress }00`, '0x52908400098527886e0f7030069857d2e4169eeg' ];
  for (const address of invalidAddresses) {
    const hook = renderHook(() => useLoginAddressMismatch(address, address));
    expect(hook.result.current).toBe(true);
    hook.unmount();
  }
  const hook = renderHook(({ address, profileAddress }) => useLoginAddressMismatch(address, profileAddress), {
    initialProps: { address: checksumAddress, profileAddress: lowercaseAddress as string | null },
  });
  expect(hook.result.current).toBe(false);
  hook.rerender({ address: checksumAddress, profileAddress: null });
  expect(hook.result.current).toBe(false);
  hook.unmount();
});

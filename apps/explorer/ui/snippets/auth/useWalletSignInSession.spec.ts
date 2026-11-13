// @vitest-environment jsdom
/// <reference types="vitest/jsdom" />

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { randomBytes, randomUUID } from 'node:crypto';
import { once } from 'node:events';
import type { ServerResponse } from 'node:http';
import { createServer } from 'node:http';
import React from 'react';
import { object, optional, parse, string } from 'valibot';
import { isHex, recoverMessageAddress } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { createSiweMessage, parseSiweMessage } from 'viem/siwe';

import type { UserInfo } from 'types/api/account';

import type useApiFetchHook from 'lib/api/useApiFetch';
import type { getResourceKey as resourceKeyFunction } from 'lib/api/useApiQuery';
import type useReCaptchaHook from 'ui/shared/reCaptcha/useReCaptcha';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';

import type useProfileQueryHook from './useProfileQuery';
import type useWalletSignInSessionHook from './useWalletSignInSession';

let useWalletSignInSession: typeof useWalletSignInSessionHook;
let useProfileQuery: typeof useProfileQueryHook;
let useReCaptcha: typeof useReCaptchaHook;
let useApiFetch: typeof useApiFetchHook;
let getResourceKey: typeof resourceKeyFunction;

const wallet = privateKeyToAccount(generatePrivateKey());
const sessions = new Map<string, UserInfo>();
const nonces = new Set<string>();
let accountRequests = 0;
let profileRequests = 0;
let rewardsRequests = 0;
let rewardsAvailable = false;
let rejectAccount = false;
let invalidProfile = false;
let holdRewards = false;
let heldRewards: ServerResponse | undefined;
let origin: string;
let restoreFetch: typeof fetch;
let previousEnvs: typeof window.__envs;
let previousUrl: string;
const clients: Array<QueryClient> = [];

function respond(response: ServerResponse, status: number, body: unknown) {
  response.writeHead(status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(body));
}

const server = createServer((request, response) => {
  void (async() => {
    const path = new URL(request.url || '/', 'http://localhost').pathname;
    if (request.method === 'GET' && path === '/api/v1/auth/nonce') {
      const nonce = randomBytes(16).toString('hex');
      nonces.add(nonce);
      respond(response, 200, { nonce, merits_login_nonce: randomBytes(16).toString('hex') });
      return;
    }
    if (request.method === 'GET' && path === '/api/account/v2/user/info') {
      profileRequests++;
      const token = request.headers.cookie?.match(/(?:^|;\s*)_explorer_key=([^;]+)/)?.[1];
      const profile = token ? sessions.get(token) : undefined;
      respond(response, profile ? 200 : 401, profile || { message: 'No account session' });
      return;
    }
    if (request.method !== 'POST' || ![ '/api/account/v2/authenticate_via_wallet', '/api/v1/auth/login' ].includes(path)) {
      respond(response, 404, { message: 'Unexpected resource' });
      return;
    }
    let data = '';
    for await (const chunk of request) {
      data += chunk;
    }
    const proof = parse(object({ message: string(), signature: string(), nonce: optional(string()) }), JSON.parse(data));
    if (!isHex(proof.signature)) {
      respond(response, 401, { message: 'Invalid signature encoding' });
      return;
    }
    const address = await recoverMessageAddress({ message: proof.message, signature: proof.signature });
    const siwe = parseSiweMessage(proof.message);
    if (
      address !== wallet.address || siwe.address !== address || !siwe.nonce || !nonces.has(siwe.nonce) ||
      siwe.domain !== new URL(origin).hostname || siwe.uri !== origin || siwe.chainId !== 1
    ) {
      respond(response, 401, { message: 'Invalid signed challenge' });
      return;
    }
    if (path === '/api/account/v2/authenticate_via_wallet') {
      accountRequests++;
      if (rejectAccount) {
        respond(response, 401, { message: 'Account verification rejected' });
        return;
      }
      if (invalidProfile) {
        respond(response, 200, { message: 'Missing account profile' });
        return;
      }
      const profile: UserInfo = { name: `Wallet ${ address }`, address_hash: address, email: null };
      const session = randomUUID();
      sessions.set(session, profile);
      response.setHeader('set-cookie', `_explorer_key=${ session }; Path=/; SameSite=Lax`);
      respond(response, 200, profile);
      return;
    }
    rewardsRequests++;
    if (request.headers.cookie) {
      respond(response, 400, { message: 'Account cookies must not be sent to rewards' });
    } else if (proof.nonce !== siwe.nonce) {
      respond(response, 401, { message: 'Wrong rewards nonce' });
    } else if (holdRewards) {
      heldRewards = response;
    } else if (!rewardsAvailable) {
      respond(response, 503, { message: 'Rewards unavailable' });
    } else {
      respond(response, 200, { token: randomUUID(), created: false });
    }
  })().catch((error: Error) => respond(response, 500, { message: error.message }));
});

beforeAll(async() => {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Loopback server did not bind a port');
  }
  origin = `http://127.0.0.1:${ address.port }`;
  previousUrl = window.location.href;
  jsdom.reconfigure({ url: `${ origin }/explorer` });
  previousEnvs = window.__envs;
  window.__envs = {
    ...previousEnvs,
    NEXT_PUBLIC_APP_PROTOCOL: 'http',
    NEXT_PUBLIC_APP_HOST: '127.0.0.1',
    NEXT_PUBLIC_APP_PORT: String(address.port),
    NEXT_PUBLIC_APP_ENV: 'testing',
    NEXT_PUBLIC_APP_INSTANCE: 'vitest',
    NEXT_PUBLIC_NETWORK_ID: '1',
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'true',
    NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY: 'loopback-test',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_API_BASE_PATH: '/',
    NEXT_PUBLIC_REWARDS_SERVICE_API_HOST: origin,
  };
  fetchMock.disableMocks();
  restoreFetch = globalThis.fetch;
  // jsdom has a cookie store but no browser fetch. Carry its cookies over real HTTP.
  globalThis.fetch = async(input, init) => {
    const headers = new Headers(init?.headers);
    const url = typeof input === 'string' || input instanceof URL ? new URL(input, window.location.href) : new URL(input.url);
    const withCookies = init?.credentials === 'same-origin' && url.origin === window.location.origin;
    if (withCookies && document.cookie) {
      headers.set('cookie', document.cookie);
    }
    const response = await restoreFetch(input, { ...init, headers });
    const cookie = response.headers.get('set-cookie');
    if (withCookies && cookie) {
      document.cookie = cookie;
    }
    return response;
  };
  [
    { 'default': useWalletSignInSession },
    { 'default': useProfileQuery },
    { 'default': useReCaptcha },
    { 'default': useApiFetch },
    { getResourceKey },
  ] = await Promise.all([
    import('./useWalletSignInSession'),
    import('./useProfileQuery'),
    import('ui/shared/reCaptcha/useReCaptcha'),
    import('lib/api/useApiFetch'),
    import('lib/api/useApiQuery'),
  ]);
});

beforeEach(() => {
  sessions.clear();
  nonces.clear();
  document.cookie = '_explorer_key=; Max-Age=0; Path=/';
  accountRequests = 0;
  profileRequests = 0;
  rewardsRequests = 0;
  rewardsAvailable = false;
  rejectAccount = false;
  invalidProfile = false;
  holdRewards = false;
  heldRewards = undefined;
});

afterEach(() => {
  heldRewards?.end();
  cleanup();
  clients.forEach((client) => client.clear());
  clients.length = 0;
});

afterAll(async() => {
  globalThis.fetch = restoreFetch;
  fetchMock.enableMocks();
  window.__envs = previousEnvs;
  jsdom.reconfigure({ url: previousUrl });
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function openSession() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  clients.push(client);
  const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(QueryClientProvider, { client }, children);
  const hook = renderHook(() => {
    const recaptcha = useReCaptcha();
    const authenticate = useWalletSignInSession({ fetchProtectedResource: recaptcha.fetchProtectedResource });
    const profile = useProfileQuery();
    const apiFetch = useApiFetch();
    return { authenticate, profile, apiFetch };
  }, { wrapper });
  return { ...hook, client };
}

async function signedChallenge(session: ReturnType<typeof openSession>) {
  const challenge = await session.result.current.apiFetch<'rewards:nonce', { nonce: string; merits_login_nonce: string }>('rewards:nonce');
  if (!('nonce' in challenge)) {
    throw new Error('Nonce producer did not return a challenge');
  }
  const message = createSiweMessage({
    address: wallet.address,
    chainId: 1,
    domain: new URL(origin).hostname,
    uri: origin,
    version: '1',
    nonce: challenge.nonce,
    statement: `Sign in/up to Blockscout Account V2 & Blockscout Merits program. Merits nonce: ${ challenge.merits_login_nonce }.`,
  });
  return { message, signature: await wallet.signMessage({ message }), authNonce: challenge.nonce };
}

async function sharedLoginCommitsAccountBeforeRewardsAndRecoversAfterRestart() {
  holdRewards = true;
  const session = openSession();
  const proof = await signedChallenge(session);
  const login = session.result.current.authenticate(proof);
  await waitFor(() => expect(rewardsRequests).toBe(1));
  const committedProfile = session.client.getQueryData<UserInfo>(getResourceKey('general:user_info'));
  expect(committedProfile?.address_hash).toBe(wallet.address);
  expect(document.cookie).toMatch(/_explorer_key=/);
  expect(sessions.size).toBe(1);
  if (!heldRewards) {
    throw new Error('Rewards request did not reach the producer');
  }
  respond(heldRewards, 503, { message: 'Rewards unavailable' });
  await act(async() => {
    expect(await login).toEqual({ profile: committedProfile, rewardsToken: undefined, rewardsAuthFailed: true });
  });
  await waitFor(() => expect(session.result.current.profile.data).toEqual(committedProfile));
  expect(accountRequests).toBe(1);
  session.unmount();
  session.client.clear();
  const profileRequestsBeforeRestart = profileRequests;

  const reopened = openSession();
  await waitFor(() => expect(reopened.result.current.profile.data).toEqual(committedProfile));
  expect(reopened.result.current.profile.isSuccess).toBe(true);
  expect(profileRequests).toBe(profileRequestsBeforeRestart + 1);
  expect(accountRequests).toBe(1);
  expect(rewardsRequests).toBe(1);
  expect(sessions.size).toBe(1);
}

async function sharedLoginPreservesRewardsSuccess() {
  rewardsAvailable = true;
  const session = openSession();
  const proof = await signedChallenge(session);
  await act(async() => {
    const result = await session.result.current.authenticate(proof);
    expect(result.rewardsAuthFailed).toBe(false);
    expect(result.rewardsToken).toBeTypeOf('string');
    expect(result.profile.address_hash).toBe(wallet.address);
    expect(session.client.getQueryData(getResourceKey('general:user_info'))).toEqual(result.profile);
  });
  expect(accountRequests).toBe(1);
  expect(rewardsRequests).toBe(1);
}

async function accountOnlyLoginDoesNotContactRewards() {
  const session = openSession();
  const { message, signature } = await signedChallenge(session);
  await act(async() => {
    const result = await session.result.current.authenticate({ message, signature });
    expect(result.rewardsAuthFailed).toBe(false);
    expect(result.rewardsToken).toBeUndefined();
    expect(result.profile.address_hash).toBe(wallet.address);
  });
  expect(accountRequests).toBe(1);
  expect(rewardsRequests).toBe(0);
}

async function accountRejectionDoesNotCommitProfileOrAttemptRewards() {
  rejectAccount = true;
  const session = openSession();
  const proof = await signedChallenge(session);
  await expect(session.result.current.authenticate(proof)).rejects.toMatchObject({ status: 401 });
  expect(session.client.getQueryData(getResourceKey('general:user_info'))).toBeUndefined();
  expect(rewardsRequests).toBe(0);
  expect(sessions.size).toBe(0);
  expect(document.cookie).toBe('');
}

async function invalidAccountResponseDoesNotCommitProfileOrAttemptRewards() {
  invalidProfile = true;
  const session = openSession();
  const proof = await signedChallenge(session);
  await expect(session.result.current.authenticate(proof)).rejects.toThrow('Something went wrong');
  expect(session.client.getQueryData(getResourceKey('general:user_info'))).toBeUndefined();
  expect(rewardsRequests).toBe(0);
}

test(sharedLoginCommitsAccountBeforeRewardsAndRecoversAfterRestart.name, sharedLoginCommitsAccountBeforeRewardsAndRecoversAfterRestart);
test(sharedLoginPreservesRewardsSuccess.name, sharedLoginPreservesRewardsSuccess);
test(accountOnlyLoginDoesNotContactRewards.name, accountOnlyLoginDoesNotContactRewards);
test(accountRejectionDoesNotCommitProfileOrAttemptRewards.name, accountRejectionDoesNotCommitProfileOrAttemptRewards);
test(invalidAccountResponseDoesNotCommitProfileOrAttemptRewards.name, invalidAccountResponseDoesNotCommitProfileOrAttemptRewards);

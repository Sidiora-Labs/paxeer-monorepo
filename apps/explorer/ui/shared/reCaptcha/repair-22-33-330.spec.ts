// @vitest-environment jsdom

import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import type { Server } from 'node:http';
import { createServer } from 'node:http';
import type { ReCAPTCHA } from 'react-google-recaptcha';

import useFetch from 'lib/hooks/useFetch';
import { afterEach, beforeEach, expect, test } from 'vitest';

import renderReCaptcha from './repair-22-33-330.fixture';
import useReCaptcha from './useReCaptcha';

const SOLVED_TOKEN = 'solved-captcha-transcript-token';
const PROTECTED_RESOURCE = { hash: 'protected-resource' };
const RATE_LIMIT_TRANSCRIPT = {
  initial: { reset: '5000', payload: { message: 'Captcha bypass available' } },
  afterToken: { reset: '42000', payload: { message: 'Limit still applies after captcha' } },
};

let server: Server;
let endpoint: string;
let receivedTokens: Array<string | undefined>;

beforeEach(async() => {
  // Restore real HTTP transport installed before the shared Vitest setup enabled fetch interception.
  fetchMock.disableMocks();
  receivedTokens = [];
  server = createServer((request, response) => {
    const token = request.headers['x-recaptcha-token'];
    receivedTokens.push(typeof token === 'string' ? token : undefined);
    if (request.url === '/recover' && token === SOLVED_TOKEN) {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify(PROTECTED_RESOURCE));
      return;
    }
    const entry = token === SOLVED_TOKEN ? RATE_LIMIT_TRANSCRIPT.afterToken : RATE_LIMIT_TRANSCRIPT.initial;
    response.writeHead(429, {
      'content-type': 'application/json',
      'bypass-429-option': 'recaptcha',
      'x-ratelimit-reset': entry.reset,
    });
    response.end(JSON.stringify(entry.payload));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP listener');
  }
  endpoint = `http://127.0.0.1:${ address.port }/protected`;
});

afterEach(async() => {
  cleanup();
  await new Promise<void>((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
});

function renderProtectedResource() {
  const hook = renderHook(() => ({ captcha: useReCaptcha(), fetch: useFetch() }));
  // Mount the real component without its external script loader. Replay its solved-token callback below.
  renderReCaptcha(hook.result.current.captcha.ref);
  const captcha = hook.result.current.captcha.ref.current;
  if (!captcha) {
    throw new Error('Expected the mounted reCAPTCHA instance');
  }
  // The package exposes this runtime callback but omits it from its public TypeScript declaration.
  const callbacks = captcha as ReCAPTCHA & { handleChange: (token: string | null) => void };
  return { ...hook, callbacks };
}

async function assertBoundedBypass(wrapInError: boolean) {
  const { result, callbacks } = renderProtectedResource();
  const failures: Array<unknown> = [];
  let outcome: { value?: unknown; error?: unknown } | undefined;

  const fetcher = async(token?: string) => {
    try {
      return await result.current.fetch(endpoint, { headers: token ? { 'x-recaptcha-token': token } : undefined });
    } catch (cause) {
      const error = wrapInError ? new Error('Protected resource rate limited', { cause }) : cause;
      failures.push(error);
      throw error;
    }
  };

  await act(async() => {
    void result.current.captcha.fetchProtectedResource(fetcher).then(
      value => {
        outcome = { value };
      },
      error => {
        outcome = { error };
      },
    );
  });
  await waitFor(() => expect(callbacks).toHaveProperty('executionResolve', expect.any(Function)));
  await act(async() => callbacks.handleChange(SOLVED_TOKEN));
  await waitFor(() => expect(outcome).toEqual({ error: failures[1] }));

  expect(receivedTokens).toEqual([ undefined, SOLVED_TOKEN ]);
  expect(failures).toHaveLength(2);
  expect(outcome?.error).toBe(failures[1]);
  const responseError = wrapInError ? (outcome?.error as Error).cause : outcome?.error;
  expect(responseError).toMatchObject({
    status: 429,
    rateLimits: { bypassOptions: 'recaptcha', reset: RATE_LIMIT_TRANSCRIPT.afterToken.reset },
    payload: RATE_LIMIT_TRANSCRIPT.afterToken.payload,
  });
  expect(callbacks).not.toHaveProperty('executionResolve');
}

async function captchaRetryUnbounded() {
  await assertBoundedBypass(false);
}

async function captchaRetryUnboundedErrorCause() {
  await assertBoundedBypass(true);
}

async function captchaSuppliedTokenDoesNotPromptAgain() {
  const { result, callbacks } = renderProtectedResource();
  let failure: unknown;
  const fetcher = async(token?: string) => {
    try {
      return await result.current.fetch(endpoint, { headers: { 'x-recaptcha-token': token || '' } });
    } catch (error) {
      failure = error;
      throw error;
    }
  };
  await expect(result.current.captcha.fetchProtectedResource(fetcher, SOLVED_TOKEN)).rejects.toMatchObject({
    status: 429,
    rateLimits: { reset: RATE_LIMIT_TRANSCRIPT.afterToken.reset },
  });
  expect(failure).toMatchObject({ status: 429 });
  expect(receivedTokens).toEqual([ SOLVED_TOKEN ]);
  expect(callbacks).not.toHaveProperty('executionResolve');
}

async function captchaSuccessPreservesResultAndNewOperationBudget() {
  const { result, callbacks } = renderProtectedResource();
  const successfulFetcher = async(token?: string) => {
    return result.current.fetch(endpoint.replace('/protected', '/recover'), {
      headers: token ? { 'x-recaptcha-token': token } : undefined,
    });
  };

  for (let operation = 0; operation < 2; operation++) {
    let value: unknown;
    await act(async() => {
      void result.current.captcha.fetchProtectedResource(successfulFetcher).then(response => {
        value = response;
      });
    });
    await waitFor(() => expect(callbacks).toHaveProperty('executionResolve', expect.any(Function)));
    await act(async() => callbacks.handleChange(SOLVED_TOKEN));
    await waitFor(() => expect(value).toEqual(PROTECTED_RESOURCE));
  }
  expect(receivedTokens).toEqual([ undefined, SOLVED_TOKEN, undefined, SOLVED_TOKEN ]);
  expect(callbacks).not.toHaveProperty('executionResolve');
}

test(captchaRetryUnbounded);
test(captchaRetryUnboundedErrorCause);
test(captchaSuppliedTokenDoesNotPromptAgain);
test(captchaSuccessPreservesResultAndNewOperationBudget);

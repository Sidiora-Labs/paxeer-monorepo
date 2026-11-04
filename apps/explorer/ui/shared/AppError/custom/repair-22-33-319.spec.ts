// @vitest-environment jsdom

import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { ClusterChainConfig } from 'types/multichain';

import 'lib/hooks/useFetch';
import { afterAll, afterEach, beforeAll, expect, test, vi } from 'vitest';

import './AppErrorTooManyRequests';

const originalEnvs = window.__envs;
const responses: Record<string, Record<string, string>> = {
  '/no-bypass': { 'bypass-429-option': 'no_bypass' },
  '/no-bypass-reset': { 'bypass-429-option': 'no_bypass', 'x-ratelimit-reset': '42001' },
  '/temporary-token': { 'bypass-429-option': 'temporary_token' },
  '/temporary-token-reset': { 'bypass-429-option': 'temporary_token', 'x-ratelimit-reset': '3661000' },
  '/unspecified': {},
};

const origin = 'https://rate-limit.test';

beforeAll(() => {
  fetchMock.mockResponse(async(request) => {
    const url = new URL(request.url);
    const headers = responses[url.pathname];
    if (url.origin !== origin || !headers) {
      throw new Error(`Unexpected rate-limit fixture request: ${ request.url }`);
    }
    return {
      status: 429,
      headers: { 'content-type': 'application/json', ...headers },
      body: JSON.stringify({ message: 'Too many requests' }),
    };
  });
});

afterEach(() => {
  window.__envs = originalEnvs;
  document.cookie = 'app_profile=; Max-Age=0; Path=/';
});

afterAll(() => {
  fetchMock.resetMocks();
});

async function renderRateLimit(path: string, { siteKey = '', privateMode = false, chainSiteKey }: {
  siteKey?: string;
  privateMode?: boolean;
  chainSiteKey?: string;
} = {}) {
  window.__envs = { ...originalEnvs, NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY: siteKey };
  document.cookie = `app_profile=${ privateMode ? 'private' : 'default' }; Path=/`;
  vi.resetModules();

  const { ChakraProvider, defaultSystem } = await import('@chakra-ui/react');
  const { 'default': config } = await import('configs/app');
  const { MultichainContext } = await import('lib/contexts/multichain');
  const { 'default': getErrorProp } = await import('lib/errors/getErrorProp');
  const { 'default': useFetch } = await import('lib/hooks/useFetch');
  const { 'default': AppErrorTooManyRequests } = await import('./AppErrorTooManyRequests');

  expect(config.services.reCaptchaV2.siteKey).toBe(siteKey);
  expect(config.app.isPrivateMode).toBe(privateMode);

  const producer: { fetchResource?: ReturnType<typeof useFetch> } = {};
  function FetchProducer() {
    producer.fetchResource = useFetch();
    return null;
  }
  renderToStaticMarkup(React.createElement(FetchProducer));
  if (!producer.fetchResource) {
    throw new Error('Production fetch hook did not initialize');
  }
  const error: unknown = await producer.fetchResource(`${ origin }${ path }`).then(
    () => {
      throw new Error('Expected the server to reject the request');
    },
    (error: unknown) => error,
  );
  expect(getErrorProp(error, 'status')).toBe(429);
  const rateLimits = getErrorProp<unknown>(error, 'rateLimits');
  if (typeof rateLimits !== 'object' || !rateLimits || !('bypassOptions' in rateLimits) || !('reset' in rateLimits)) {
    throw new Error('Production fetch hook did not decode rate-limit headers');
  }
  expect(rateLimits.bypassOptions).toBe(responses[path]['bypass-429-option'] ?? null);
  expect(rateLimits.reset).toBe(responses[path]['x-ratelimit-reset'] ?? null);

  const chain: ClusterChainConfig = {
    id: '125',
    name: 'Test chain',
    slug: 'test-chain',
    app_config: {
      ...config,
      services: { ...config.services, reCaptchaV2: { siteKey: chainSiteKey } },
    },
  };
  const content = React.createElement(AppErrorTooManyRequests, {
    bypassOptions: typeof rateLimits.bypassOptions === 'string' ? rateLimits.bypassOptions : undefined,
    reset: typeof rateLimits.reset === 'string' ? rateLimits.reset : undefined,
  });
  const markup = renderToStaticMarkup(React.createElement(ChakraProvider, {
    value: defaultSystem,
    children: React.createElement(MultichainContext.Provider, { value: chainSiteKey === undefined ? null : { chain } }, content),
  }));
  const screen = document.createElement('section');
  screen.innerHTML = markup;
  expect(screen.querySelector('h1')?.textContent).toBe('Too many requests');
  expect(screen.querySelector('use')?.getAttribute('href')).toContain('#error-pages/429');
  return screen;
}

test('rateLimitWithoutCaptchaShowsNoBypassExplanation', async function rateLimitWithoutCaptchaShowsNoBypassExplanation() {
  const screen = await renderRateLimit('/no-bypass');
  expect(screen.querySelector('p')?.textContent).toBe('Rate limit exceeded.');
  expect(screen.querySelector('button')).toBeNull();
  expect(screen.querySelectorAll('div')).toHaveLength(0);
});

test('rateLimitWithoutCaptchaPreservesServerResetCountdown', async function rateLimitWithoutCaptchaPreservesServerResetCountdown() {
  const screen = await renderRateLimit('/no-bypass-reset');
  expect(screen.querySelector('p')?.textContent).toBe('Rate limit exceeded. Please wait 00h 00m 43s before making another request.');
  expect(screen.querySelector('button')).toBeNull();
  expect(screen.querySelectorAll('div')).toHaveLength(0);
});

test('rateLimitWithoutConfiguredBypassDoesNotOfferVerification', async function rateLimitWithoutConfiguredBypassDoesNotOfferVerification() {
  for (const path of [ '/temporary-token', '/unspecified' ]) {
    const screen = await renderRateLimit(path);
    expect(screen.querySelector('p')?.textContent).toBe('Rate limit exceeded.');
    expect(screen.querySelector('button')).toBeNull();
    expect(screen.querySelectorAll('div')).toHaveLength(0);
  }
  const screen = await renderRateLimit('/temporary-token-reset');
  expect(screen.querySelector('p')?.textContent).toBe('Rate limit exceeded. Please wait 01h 01m 01s before making another request.');
  expect(screen.querySelector('button')).toBeNull();
});

test('rateLimitNoBypassNeverRendersConfiguredCaptcha', async function rateLimitNoBypassNeverRendersConfiguredCaptcha() {
  const screen = await renderRateLimit('/no-bypass', { siteKey: 'configured-test-site-key' });
  expect(screen.querySelector('p')?.textContent).toBe('Rate limit exceeded.');
  expect(screen.querySelector('button')).toBeNull();
  expect(screen.querySelectorAll('div')).toHaveLength(0);
});

test('rateLimitConfiguredPermittedBypassOffersCaptcha', async function rateLimitConfiguredPermittedBypassOffersCaptcha() {
  const screen = await renderRateLimit('/temporary-token-reset', { siteKey: 'configured-test-site-key' });
  expect(screen.querySelector('p')?.textContent)
    .toBe('Rate limit exceeded. Please wait 01h 01m 01s OR verify you\'re human before making another request.');
  expect(screen.querySelector('button')?.textContent).toBe('I\'m not a robot');
  expect(screen.querySelector('button')?.disabled).toBe(false);
  expect(screen.querySelectorAll('div')).toHaveLength(1);
});

test('rateLimitPrivateModeDoesNotOfferDisabledCaptcha', async function rateLimitPrivateModeDoesNotOfferDisabledCaptcha() {
  const screen = await renderRateLimit('/temporary-token-reset', { siteKey: 'configured-test-site-key', privateMode: true });
  expect(screen.querySelector('p')?.textContent).toBe('Rate limit exceeded. Please wait 01h 01m 01s before making another request.');
  expect(screen.querySelector('button')).toBeNull();
  expect(screen.querySelectorAll('div')).toHaveLength(0);
});

test('rateLimitUsesTheSameChainConfigurationAsCaptcha', async function rateLimitUsesTheSameChainConfigurationAsCaptcha() {
  const unavailable = await renderRateLimit('/temporary-token', { siteKey: 'global-test-site-key', chainSiteKey: '' });
  expect(unavailable.querySelector('p')?.textContent).toBe('Rate limit exceeded.');
  expect(unavailable.querySelector('button')).toBeNull();
  expect(unavailable.querySelectorAll('div')).toHaveLength(0);

  const available = await renderRateLimit('/temporary-token', { chainSiteKey: 'chain-test-site-key' });
  expect(available.querySelector('p')?.textContent).toBe('Rate limit exceeded. Please verify you\'re human before making another request.');
  expect(available.querySelector('button')?.textContent).toBe('I\'m not a robot');
  expect(available.querySelectorAll('div')).toHaveLength(1);
});

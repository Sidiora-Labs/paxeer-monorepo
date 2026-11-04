// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { NextRequest } from 'next/server';
import nodeFetch from 'node-fetch';
import { Agent, createServer } from 'node:https';

import React from 'react';
import { afterEach, expect, test, vi } from 'vitest';

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
const originalEnvs = window.__envs;
const originalFetch = globalThis.fetch;

function configureSafe(serviceUrl: string) {
  vi.resetModules();
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_SAFE_TX_SERVICE_URL: serviceUrl,
  };
}

function readDirectives(policy: string) {
  return Object.fromEntries(policy.split(';').map((directive) => {
    const [ name, ...sources ] = directive.trim().split(/\s+/);
    return [ name, sources ];
  }));
}

afterEach(() => {
  window.__envs = originalEnvs;
  Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: originalFetch });
  vi.resetModules();
});

async function repairSafeOriginConfigurationResponseAndRequest() {
  const hash = '0x1234567890123456789012345678901234567890';
  const requests: Array<{ method: string | undefined; url: string | undefined; host: string | undefined }> = [];
  const service = createServer({ key: serviceKey, cert: serviceCertificate }, (req, res) => {
    requests.push({ method: req.method, url: req.url, host: req.headers.host });
    res.setHeader('Content-Type', 'application/json');
    if (req.method === 'GET' && req.url === `/api/v1/safes/${ hash }`) {
      res.end(JSON.stringify({ address: hash }));
    } else {
      res.statusCode = 404;
      res.end(JSON.stringify({ detail: 'Not found' }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    service.once('error', reject);
    service.listen(0, '127.0.0.1', resolve);
  });
  const agent = new Agent({ ca: serviceCertificate });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  let unmount: (() => void) | undefined;

  try {
    const address = service.address();
    if (!address || typeof address === 'string') {
      throw new Error('HTTPS Safe service must listen on a TCP port');
    }
    const origin = `https://127.0.0.1:${ address.port }`;
    configureSafe(`${ origin }/configured/path?ignored=true#fragment`);

    const { 'default': config } = await import('configs/app');
    const { safe } = await import('./safe');
    const { middleware } = await import('middleware');
    const { 'default': useIsSafeAddress } = await import('lib/hooks/useIsSafeAddress');
    expect(config.features.safe).toEqual({ title: 'Safe address tags', isEnabled: true, apiUrl: `${ origin }/api/v1/safes` });
    expect(safe()).toEqual({ 'connect-src': [ origin ] });

    for (const profile of [ 'default', 'private' ]) {
      const response = await middleware(new NextRequest('https://explorer.example/', {
        headers: { accept: 'text/html', 'x-app-profile': profile },
      }));
      expect(response).toBeDefined();
      const header = response?.headers.get('Content-Security-Policy');
      expect(header).toBeTruthy();
      const directives = readDirectives(header || '');
      expect(directives['connect-src']).toContain(origin);
      expect(directives['connect-src']).toContain('\'self\'');
      for (const broadSource of [ '*', 'https:', 'http:', '*.safe.global' ]) {
        expect(directives['connect-src']).not.toContain(broadSource);
      }
      expect(directives['default-src']).toEqual([ '\'self\'' ]);
      expect(directives['object-src']).toEqual([ '\'none\'' ]);
      expect(directives['base-uri']).toEqual([ '\'none\'' ]);
    }

    // Use the real HTTP client with the local service's CA, replacing the suite's fetch mock.
    Object.defineProperty(globalThis, 'fetch', {
      configurable: true,
      writable: true,
      value: (url: Parameters<typeof nodeFetch>[0], options?: Parameters<typeof nodeFetch>[1]) => nodeFetch(url, { ...options, agent }),
    });
    function QueryProvider({ children }: { children: React.ReactNode }) {
      return React.createElement(QueryClientProvider, { client: queryClient }, children);
    }
    const hook = renderHook(() => useIsSafeAddress(hash), { wrapper: QueryProvider });
    unmount = hook.unmount;
    expect(hook.result.current).toBe(false);
    await waitFor(() => expect(hook.result.current).toBe(true));
    expect(requests).toEqual([ { method: 'GET', url: `/api/v1/safes/${ hash }`, host: `127.0.0.1:${ address.port }` } ]);
    expect(queryClient.getQueryData([ 'external:safe_transaction_api', hash ])).toEqual({ address: hash });
  } finally {
    unmount?.();
    queryClient.clear();
    agent.destroy();
    await new Promise<void>((resolve, reject) => service.close((error) => error ? reject(error) : resolve()));
  }
}

async function repairSafeOriginUsesOnlyConfiguredOrigin() {
  for (const [ configuredUrl, expectedOrigin ] of [
    [ 'https://service.example:8443/custom?value=1#section', 'https://service.example:8443' ],
    [ 'https://user:password@SERVICE.example:443/custom', 'https://service.example' ],
    [ 'https://safe-transaction-mainnet.safe.global', 'https://safe-transaction-mainnet.safe.global' ],
    [ 'http://service.example:8080/custom', 'http://service.example:8080' ],
  ]) {
    configureSafe(configuredUrl);
    const { 'default': feature } = await import('configs/app/features/safe');
    const { safe } = await import('./safe');
    expect(feature.isEnabled).toBe(true);
    if (!feature.isEnabled) {
      throw new Error('A valid Safe URL must enable the real configuration producer');
    }
    expect(new URL(feature.apiUrl).origin).toBe(expectedOrigin);
    expect(safe()).toEqual({ 'connect-src': [ expectedOrigin ] });
  }
}

async function repairSafeOriginDisabledForMissingOrInvalidConfiguration() {
  for (const configuredUrl of [ '', 'not a URL' ]) {
    configureSafe(configuredUrl);
    const { 'default': feature } = await import('configs/app/features/safe');
    const { safe } = await import('./safe');
    const { 'default': generateCspPolicy } = await import('nextjs/csp/generateCspPolicy');
    expect(feature.isEnabled).toBe(false);
    expect(safe()).toEqual({});
    const directives = readDirectives(generateCspPolicy());
    expect(directives['connect-src']).not.toContain('*.safe.global');
    expect(directives['default-src']).toEqual([ '\'self\'' ]);
  }
}

test(repairSafeOriginConfigurationResponseAndRequest.name, repairSafeOriginConfigurationResponseAndRequest);
test(repairSafeOriginUsesOnlyConfiguredOrigin.name, repairSafeOriginUsesOnlyConfiguredOrigin);
test(repairSafeOriginDisabledForMissingOrInvalidConfiguration.name, repairSafeOriginDisabledForMissingOrInvalidConfiguration);

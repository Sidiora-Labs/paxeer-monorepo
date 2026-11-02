/**
 * Proves: csp.ts contentSecurityPolicy -> proxy.ts config/proxy and NextResponse.headers -> proof_security_headers.spec.ts consumer.
 * Deployment state comes from lib/bridge/config.ts readConfig.
 * Next.js's real matcher consumes the exported config; no browser caller exists yet.
 * Repairs csp.ts contentSecurityPolicy accepting non-HTTP origins, credentials and
 * semicolons in hostnames that could introduce another CSP directive.
 * Each request produces fresh headers; changed deployment inputs must reach the
 * next response. Malformed configuration is refused at the CSP URL decoder.
 */
import { unstable_doesMiddlewareMatch } from 'next/experimental/testing/server';
import { NextRequest, NextResponse } from 'next/server';

import { expect, test } from 'vitest';

import { bridgeConfig, readConfig } from '../lib/bridge/config';
import { contentSecurityPolicy } from '../lib/bridge/csp';
import { config, proxy } from '../proxy';

function securityHeadersReachTheConsumerTwice() {
  const original = { ...bridgeConfig };
  const deployment = readConfig({
    NEXT_PUBLIC_PAXEER_RPC_URL: ' https://PAXEER.example:443/evm?key=paxeer-secret ',
    NEXT_PUBLIC_BRIDGE_API_URL: 'https://attest.example:8443/v1?key=api-secret',
    NEXT_PUBLIC_PAXEER_EXPLORER_URL: 'https://explorer.example',
    NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM: 'https://ethereum.example/rpc?key=eth-secret',
    NEXT_PUBLIC_BRIDGE_RPC_BASE: 'https://ethereum.example/another-rpc',
    NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM: 'https://arbitrum.example/rpc',
    NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM: 'https://optimism.example/rpc',
    NEXT_PUBLIC_BRIDGE_RPC_BNB: 'https://bnb.example/rpc',
    NEXT_PUBLIC_BRIDGE_RPC_POLYGON: 'https://polygon.example/rpc',
    NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE: 'https://avalanche.example/rpc',
    NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM: 'https://hyperevm.example/rpc',
    NEXT_PUBLIC_BRIDGE_RPC_SOLANA: 'http://localhost:8899/solana',
  });
  const expectedPolicy = [
    'default-src \'self\'',
    'script-src \'self\' \'unsafe-inline\'',
    'style-src \'self\' \'unsafe-inline\'',
    'img-src \'self\' data: blob:',
    'connect-src \'self\' https://paxeer.example https://attest.example:8443 https://ethereum.example ' +
      'https://arbitrum.example https://optimism.example https://bnb.example https://polygon.example ' +
      'https://avalanche.example https://hyperevm.example http://localhost:8899',
    'frame-ancestors \'none\'',
  ].join('; ');

  try {
    // Install real readConfig output in the deployment object consumed by proxy.
    Object.assign(bridgeConfig, deployment);
    expect(contentSecurityPolicy(deployment)).toBe(expectedPolicy);
    let previous: NextResponse | undefined;
    for (const path of [ '/', '/api/health' ]) {
      const request = new NextRequest(new URL(path, 'https://bridge.example'), {
        headers: { 'Content-Security-Policy': 'connect-src *; frame-ancestors *' },
      });
      expect(unstable_doesMiddlewareMatch({ config, url: request.url })).toBe(true);
      const response = proxy(request);
      expect(response).toBeInstanceOf(NextResponse);
      expect(response).not.toBe(previous);
      expect(response.headers).not.toBe(previous?.headers);
      expect(response.status).toBe(200);
      expect(response.headers.get('x-middleware-next')).toBe('1');
      expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
      expect(response.headers.get('X-Frame-Options')).toBe('DENY');
      expect(response.headers.get('Referrer-Policy')).toBe('strict-origin-when-cross-origin');
      expect(response.headers.get('Permissions-Policy')).toBe('camera=(), microphone=(), geolocation=()');
      const observed = response.headers.get('Content-Security-Policy');
      expect(observed).toBe(expectedPolicy);
      expect(observed).not.toContain('secret');
      expect(observed).not.toContain('explorer.example');
      // A consumer mutating its response must not affect the next invocation.
      response.headers.set('Content-Security-Policy', 'connect-src *');
      previous = response;
    }

    Object.assign(bridgeConfig, readConfig({ NEXT_PUBLIC_PAXEER_RPC_URL: 'https://replacement.example/evm' }));
    const updatedPolicy = [
      ...expectedPolicy.split('; ').slice(0, 4),
      'connect-src \'self\' https://replacement.example',
      'frame-ancestors \'none\'',
    ].join('; ');
    for (let attempt = 0; attempt < 2; attempt++) {
      const response = proxy(new NextRequest('https://bridge.example/'));
      expect(response.headers.get('Content-Security-Policy')).toBe(updatedPolicy);
    }
  } finally {
    Object.assign(bridgeConfig, original);
  }
}

function exportedMatcherIsConsumedByNextRouting() {
  expect(config.matcher).toEqual([ '/((?!_next/static|_next/image|favicon.ico).*)' ]);
  for (const url of [ '/', '/transfers', '/api/health', '/assets/bridge.svg' ]) {
    expect(unstable_doesMiddlewareMatch({ config, url }), url).toBe(true);
  }
  for (const url of [ '/_next/static/chunks/app.js', '/_next/image?url=%2Fbridge.png&w=64&q=75', '/favicon.ico' ]) {
    expect(unstable_doesMiddlewareMatch({ config, url }), url).toBe(false);
  }
}

function malformedDeploymentOriginsAreRefusedAtTheDecoder() {
  const original = { ...bridgeConfig };
  const malformed = [
    'not a URL',
    'data:text/plain,bridge',
    'javascript:alert(1)',
    'ftp://rpc.example',
    'https://user:password@rpc.example',
    'https://rpc.example;upgrade-insecure-requests',
  ];

  try {
    for (const endpoint of malformed) {
      // Corrupt each config producer result at its typed module boundary. The
      // real CSP decoder and proxy must refuse it before emitting a response.
      for (const invalid of [
        { ...readConfig({}), paxeerRpcUrl: endpoint },
        { ...readConfig({}), attestationApiUrl: endpoint },
        { ...readConfig({}), foreignRpcUrls: { ...readConfig({}).foreignRpcUrls, solana: endpoint } },
      ]) {
        Object.assign(bridgeConfig, invalid);
        for (let attempt = 0; attempt < 2; attempt++) {
          expect(() => contentSecurityPolicy(bridgeConfig)).toThrow(TypeError);
          expect(() => proxy(new NextRequest('https://bridge.example/'))).toThrow(TypeError);
        }
      }
    }
  } finally {
    Object.assign(bridgeConfig, original);
  }
}

function invalidEnvironmentEndpointsNeverReachResponseSources() {
  const original = { ...bridgeConfig };
  const rejected = readConfig({
    NEXT_PUBLIC_PAXEER_RPC_URL: 'javascript:alert(1)',
    NEXT_PUBLIC_BRIDGE_API_URL: 'https://user:password@attest.example',
    NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM: 'not a URL',
    NEXT_PUBLIC_BRIDGE_RPC_SOLANA: 'wss://solana.example',
  });
  expect(rejected.paxeerRpcUrl).toBeUndefined();
  expect(rejected.attestationApiUrl).toBeUndefined();
  expect(rejected.foreignRpcUrls.ethereum).toBeUndefined();
  expect(rejected.foreignRpcUrls.solana).toBeUndefined();
  try {
    Object.assign(bridgeConfig, rejected);
    for (let attempt = 0; attempt < 2; attempt++) {
      const response = proxy(new NextRequest('https://bridge.example/'));
      const observed = response.headers.get('Content-Security-Policy');
      expect(observed).toBe(contentSecurityPolicy(readConfig({})));
      expect(observed?.split('; ')).toContain('connect-src \'self\'');
      expect(observed?.split('; ')).toContain('frame-ancestors \'none\'');
    }
  } finally {
    Object.assign(bridgeConfig, original);
  }
}

test(securityHeadersReachTheConsumerTwice.name, securityHeadersReachTheConsumerTwice);
test(exportedMatcherIsConsumedByNextRouting.name, exportedMatcherIsConsumedByNextRouting);
test(malformedDeploymentOriginsAreRefusedAtTheDecoder.name, malformedDeploymentOriginsAreRefusedAtTheDecoder);
test(invalidEnvironmentEndpointsNeverReachResponseSources.name, invalidEnvironmentEndpointsNeverReachResponseSources);

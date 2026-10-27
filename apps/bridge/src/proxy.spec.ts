import { NextRequest } from 'next/server';
import { expect, test } from 'vitest';

import { bridgeConfig, readConfig } from './lib/bridge/config';
import { contentSecurityPolicy } from './lib/bridge/csp';
import { config, proxy } from './proxy';

function proxySetsTheSecurityHeaders() {
  for (const path of [ '/', '/transfers', '/api/health' ]) {
    const request = new NextRequest(new URL(path, 'https://bridge.example'), {
      headers: { 'Content-Security-Policy': 'connect-src *; frame-ancestors *' },
    });
    const response = proxy(request);

    expect(response.status).toBe(200);
    expect(response.headers.get('x-middleware-next')).toBe('1');
    expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
    expect(response.headers.get('X-Frame-Options')).toBe('DENY');
    expect(response.headers.get('Referrer-Policy')).toBe('strict-origin-when-cross-origin');
    expect(response.headers.get('Permissions-Policy')).toBe('camera=(), microphone=(), geolocation=()');
    expect(response.headers.get('Content-Security-Policy')).toBe(contentSecurityPolicy(bridgeConfig));
    expect(response.headers.get('Content-Security-Policy')).toContain('frame-ancestors \'none\'');
  }

  expect(config.matcher).toEqual([ '/((?!_next/static|_next/image|favicon.ico).*)' ]);
}

function contentSecurityPolicyAllowsOnlyConfiguredOrigins() {
  const configured = readConfig({
    NEXT_PUBLIC_PAXEER_RPC_URL: ' https://PAXEER.example:443/evm?key=paxeer-secret ',
    NEXT_PUBLIC_BRIDGE_API_URL: 'https://attest.example:8443/v1/attestations?key=api-secret',
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
  const policy = contentSecurityPolicy(configured);
  const directives = Object.fromEntries(policy.split('; ').map((directive) => {
    const [ name, ...sources ] = directive.split(' ');
    return [ name, sources ];
  }));

  expect(directives).toEqual({
    'default-src': [ '\'self\'' ],
    'script-src': [ '\'self\'', '\'unsafe-inline\'' ],
    'style-src': [ '\'self\'', '\'unsafe-inline\'' ],
    'img-src': [ '\'self\'', 'data:', 'blob:' ],
    'connect-src': [
      '\'self\'',
      'https://paxeer.example',
      'https://attest.example:8443',
      'https://ethereum.example',
      'https://arbitrum.example',
      'https://optimism.example',
      'https://bnb.example',
      'https://polygon.example',
      'https://avalanche.example',
      'https://hyperevm.example',
      'http://localhost:8899',
    ],
    'frame-ancestors': [ '\'none\'' ],
  });
  expect(policy).not.toContain('secret');
  expect(policy).not.toContain('explorer.example');
  expect(contentSecurityPolicy(readConfig({})).split('; ')).toContain('connect-src \'self\'');
  expect(contentSecurityPolicy(readConfig({
    NEXT_PUBLIC_PAXEER_RPC_URL: 'javascript:alert(1)',
    NEXT_PUBLIC_BRIDGE_API_URL: 'https://user:password@attest.example',
    NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM: 'not a URL',
    NEXT_PUBLIC_BRIDGE_RPC_SOLANA: 'wss://solana.example',
  })).split('; ')).toContain('connect-src \'self\'');
}

test(proxySetsTheSecurityHeaders.name, proxySetsTheSecurityHeaders);
test(contentSecurityPolicyAllowsOnlyConfiguredOrigins.name, contentSecurityPolicyAllowsOnlyConfiguredOrigins);

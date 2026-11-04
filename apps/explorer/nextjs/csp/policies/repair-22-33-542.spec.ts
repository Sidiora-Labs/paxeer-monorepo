// @vitest-environment jsdom

import { NextRequest } from 'next/server';
import nodeFetch from 'node-fetch';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { URL as NodeURL } from 'node:url';
import { PostHog } from 'posthog-js';

import type { EssentialDappsConfig } from 'types/client/marketplace';
import ts from 'typescript';

import { afterEach, expect, test, vi } from 'vitest';

const originalEnvs = window.__envs;
const originalFetch = globalThis.fetch;

function configureMarketplace(essentialDapps: EssentialDappsConfig | undefined, enabled = true, port = '3000') {
  vi.resetModules();
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_APP_HOST: '127.0.0.1',
    NEXT_PUBLIC_APP_PORT: port,
    NEXT_PUBLIC_APP_PROTOCOL: 'http',
    NEXT_PUBLIC_MARKETPLACE_ENABLED: String(enabled),
    NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG: JSON.stringify(essentialDapps),
    NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID: 'repair-marketplace-config',
  };
}

function readDirectives(policy: string) {
  return Object.fromEntries(policy.split(';').map((directive) => {
    const [ name, ...sources ] = directive.trim().split(/\s+/);
    return [ name, sources ];
  }));
}

async function assertMultisendPropWiring() {
  const source = ts.createSourceFile(
    'Multisend.tsx',
    await readFile(new NodeURL('../../../ui/marketplace/essentialDapps/multisend/Multisend.tsx', import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const declarations: Record<string, string | undefined> = {};
  const widgetProps: Record<string, string> = {};
  const imports: Record<string, string> = {};
  function visit(node: ts.Node) {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier) && node.importClause?.name) {
      imports[node.importClause.name.text] = node.moduleSpecifier.text;
    }
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name)) {
      declarations[node.name.text] = node.initializer?.getText(source);
    }
    if (ts.isJsxSelfClosingElement(node) && node.tagName.getText(source) === 'MultisenderWidget') {
      for (const attribute of node.attributes.properties) {
        if (ts.isJsxAttribute(attribute) && attribute.initializer && ts.isJsxExpression(attribute.initializer)) {
          widgetProps[attribute.name.getText(source)] = attribute.initializer.expression?.getText(source) || '';
        }
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  expect(imports.config).toBe('configs/app');
  expect(declarations.feature).toBe('getFeaturePayload(config.features.marketplace)');
  expect(declarations.dappConfig).toBe('feature?.essentialDapps?.multisend');
  expect(widgetProps.posthogHost).toBe('dappConfig?.posthogHost');
  expect(widgetProps.posthogKey).toBe('dappConfig?.posthogKey');
}

afterEach(() => {
  window.__envs = originalEnvs;
  Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: originalFetch });
  vi.unstubAllEnvs();
  vi.resetModules();
});

async function repairMarketplaceAnalyticsConfigurationAndResponse() {
  const requests: Array<{ method: string | undefined; url: string | undefined }> = [];
  const chains = { chains: [] };
  const service = createServer((req, res) => {
    requests.push({ method: req.method, url: req.url });
    res.setHeader('Content-Type', 'application/json');
    if (req.method === 'GET' && req.url === '/explorer/assets/essential-dapps/chains.json') {
      res.end(JSON.stringify(chains));
    } else {
      res.statusCode = 404;
      res.end(JSON.stringify({ error: 'Not found' }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    service.once('error', reject);
    service.listen(0, '127.0.0.1', resolve);
  });

  try {
    const address = service.address();
    if (!address || typeof address === 'string') {
      throw new Error('The configuration fixture must listen on a TCP port');
    }
    const origin = `http://127.0.0.1:${ address.port }`;
    const multisend = { chains: [], posthogHost: `${ origin }/analytics`, posthogKey: 'self-hosted-analytics' };
    configureMarketplace({ multisend }, true, String(address.port));
    vi.stubEnv('NEXT_RUNTIME', 'edge');
    vi.stubEnv('HOSTNAME', '127.0.0.1');
    Object.defineProperty(globalThis, 'fetch', { configurable: true, writable: true, value: nodeFetch });

    const { 'default': config } = await import('configs/app');
    const { getFeaturePayload } = await import('configs/app/features/types');
    const { marketplace } = await import('./marketplace');
    const { middleware } = await import('middleware');
    const { getValue } = await import('configs/essential-dapps-chains/config.edge');
    const feature = getFeaturePayload(config.features.marketplace);
    const dappConfig = feature?.essentialDapps?.multisend;
    expect(dappConfig).toEqual(multisend);
    if (!dappConfig?.posthogHost) {
      throw new Error('The configuration producer must carry the self-hosted analytics host');
    }
    await assertMultisendPropWiring();
    const analytics = new PostHog();
    analytics.set_config({ api_host: dappConfig.posthogHost });
    expect(new URL(analytics.requestRouter.endpointFor('api', '/e/')).origin).toBe(origin);
    expect(new URL(analytics.requestRouter.endpointFor('assets', '/static/array.js')).origin).toBe(origin);

    const descriptor = marketplace();
    expect(descriptor['connect-src']).toContain(origin);
    expect(descriptor['script-src']).toEqual([ origin ]);
    expect(descriptor['connect-src']).not.toContain('*.posthog.com');

    for (const profile of [ 'default', 'private' ]) {
      const response = await middleware(new NextRequest('https://explorer.example/', {
        headers: { accept: 'text/html', 'x-app-profile': profile },
      }));
      const header = response?.headers.get('Content-Security-Policy');
      expect(header).toBeTruthy();
      if (!header) {
        throw new Error('The page response must carry its CSP header');
      }
      const directives = readDirectives(header);
      for (const directive of [ 'connect-src', 'script-src' ]) {
        if (profile === 'default') {
          expect(directives[directive]).toContain(origin);
        } else {
          expect(directives[directive]).not.toContain(origin);
        }
        for (const broadSource of [ '*.posthog.com', '*', 'https:', 'http:' ]) {
          expect(directives[directive]).not.toContain(broadSource);
        }
        expect(directives[directive]).not.toContain(multisend.posthogHost);
      }
      expect(directives['default-src']).toEqual([ '\'self\'' ]);
      expect(directives['object-src']).toEqual([ '\'none\'' ]);
      expect(directives['base-uri']).toEqual([ '\'none\'' ]);
    }
    expect(getValue()).toEqual(chains);
    expect(requests).toEqual([ { method: 'GET', url: '/explorer/assets/essential-dapps/chains.json' } ]);
  } finally {
    await new Promise<void>((resolve, reject) => service.close((error) => error ? reject(error) : resolve()));
  }
}

async function repairMarketplaceAnalyticsUsesOnlyConfiguredOrigin() {
  for (const [ host, origin ] of [
    [ 'https://analytics.example:8443/path?query=yes#fragment', 'https://analytics.example:8443' ],
    [ 'https://ANALYTICS.example:443/', 'https://analytics.example' ],
    [ 'http://localhost:8123/analytics', 'http://localhost:8123' ],
  ]) {
    configureMarketplace({ multisend: { chains: [], posthogHost: host, posthogKey: 'analytics' } });
    const { marketplace } = await import('./marketplace');
    const { 'default': generateCspPolicy } = await import('nextjs/csp/generateCspPolicy');
    expect(marketplace()['script-src']).toEqual([ origin ]);
    const directives = readDirectives(generateCspPolicy());
    for (const directive of [ 'connect-src', 'script-src' ]) {
      expect(directives[directive]).toContain(origin);
      expect(directives[directive]).not.toContain('*.posthog.com');
      if (origin !== 'https://us.i.posthog.com') {
        expect(directives[directive]).not.toContain('https://us.i.posthog.com');
      }
    }
  }
}

async function repairMarketplaceAnalyticsPermitsActualCloudEndpoints() {
  for (const host of [
    'https://app.posthog.com', 'https://us.i.posthog.com', 'https://eu.i.posthog.com',
    'https://eu.posthog.com:8443/analytics',
  ]) {
    configureMarketplace({ multisend: { chains: [], posthogHost: host, posthogKey: 'analytics' } });
    const { marketplace } = await import('./marketplace');
    const { 'default': config } = await import('configs/app');
    const feature = config.features.marketplace;
    if (!feature.isEnabled || !feature.essentialDapps?.multisend?.posthogHost) {
      throw new Error('The real config loader must supply the analytics host');
    }
    const analytics = new PostHog();
    analytics.set_config({ api_host: feature.essentialDapps.multisend.posthogHost });
    const apiOrigin = new URL(analytics.requestRouter.endpointFor('api', '/e/')).origin;
    const scriptOrigin = new URL(analytics.requestRouter.endpointFor('assets', '/static/array.js')).origin;
    const expectedSources = Array.from(new Set([ new URL(host).origin, apiOrigin, scriptOrigin ]));
    const descriptor = marketplace();
    expect(descriptor['script-src']).toEqual(expectedSources);
    expect(descriptor['connect-src']).toEqual([ '', '', '*.multisender.app', ...expectedSources ]);
    expect(expectedSources).not.toContain('*.posthog.com');
  }
}

async function repairMarketplaceAnalyticsRejectsInvalidHosts() {
  for (const host of [
    undefined, '', 'not a URL', 'analytics.example', '//analytics.example',
    'javascript:alert(1)', 'data:text/javascript,alert(1)', 'file:///tmp/analytics',
    'ftp://analytics.example', 'https://*.posthog.com', 'https://analytics.example;script-src',
    'https://analytics.example,other.example',
  ]) {
    configureMarketplace({ multisend: { chains: [], posthogHost: host, posthogKey: 'analytics' } });
    const { marketplace } = await import('./marketplace');
    const { 'default': generateCspPolicy } = await import('nextjs/csp/generateCspPolicy');
    const descriptor = marketplace();
    expect(descriptor['script-src']).toEqual([]);
    expect(descriptor['connect-src']).toEqual([ '', '', '*.multisender.app' ]);
    const directives = readDirectives(generateCspPolicy());
    for (const directive of [ 'connect-src', 'script-src' ]) {
      expect(directives[directive]).not.toContain('*.posthog.com');
      expect(directives[directive]).not.toContain('https://us.i.posthog.com');
      if (host) {
        expect(directives[directive]).not.toContain(host);
      }
    }
    expect(directives['default-src']).toEqual([ '\'self\'' ]);
  }
}

async function repairMarketplaceAnalyticsRequiresEnabledMultisend() {
  const cases: Array<[EssentialDappsConfig | undefined, boolean]> = [
    [ undefined, true ],
    [ { multisend: { chains: [], posthogHost: 'https://analytics.example' } }, false ],
  ];
  for (const [ essentialDapps, enabled ] of cases) {
    configureMarketplace(essentialDapps, enabled);
    const { marketplace } = await import('./marketplace');
    if (enabled) {
      expect(marketplace()['script-src']).toEqual([]);
    } else {
      expect(marketplace()).toEqual({});
    }
  }
}

test(repairMarketplaceAnalyticsConfigurationAndResponse.name, repairMarketplaceAnalyticsConfigurationAndResponse);
test(repairMarketplaceAnalyticsUsesOnlyConfiguredOrigin.name, repairMarketplaceAnalyticsUsesOnlyConfiguredOrigin);
test(repairMarketplaceAnalyticsPermitsActualCloudEndpoints.name, repairMarketplaceAnalyticsPermitsActualCloudEndpoints);
test(repairMarketplaceAnalyticsRejectsInvalidHosts.name, repairMarketplaceAnalyticsRejectsInvalidHosts);
test(repairMarketplaceAnalyticsRequiresEnabledMultisend.name, repairMarketplaceAnalyticsRequiresEnabledMultisend);

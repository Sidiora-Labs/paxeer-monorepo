import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { allowedDevOrigins, backendRewrites, TRANSPILE_PACKAGES } from './next-build-config';

function transpilePackagesAreDeclaredWorkspaceDependencies() {
  const manifest: { dependencies: Record<string, string> } = JSON.parse(
    readFileSync(new URL('../../package.json', import.meta.url), 'utf8'),
  );

  expect(TRANSPILE_PACKAGES).toEqual([
    'candles-sdk',
    'indexer-sdk',
    'metadata-sdk',
    'ranking-algo-sdk',
    'stats-sdk',
    'users-sdk',
    'sidiora-launchpad-v3',
  ]);
  expect(Object.isFrozen(TRANSPILE_PACKAGES)).toBe(true);
  expect(new Set(TRANSPILE_PACKAGES).size).toBe(TRANSPILE_PACKAGES.length);
  for (const name of TRANSPILE_PACKAGES) {
    expect(Object.hasOwn(manifest.dependencies, name), name).toBe(true);
    expect(manifest.dependencies[name], name).toBe('workspace:*');
  }
}

test(transpilePackagesAreDeclaredWorkspaceDependencies);

function allowedDevOriginsDefaultToLoopback() {
  expect(allowedDevOrigins({})).toEqual(['localhost', '127.0.0.1']);
  expect(allowedDevOrigins({ NEXT_DEV_ALLOWED_ORIGINS: '' })).toEqual(['localhost', '127.0.0.1']);
  expect(allowedDevOrigins({ NEXT_DEV_ALLOWED_ORIGINS: ' , \t, ' })).toEqual(['localhost', '127.0.0.1']);
  expect(allowedDevOrigins({ NEXT_DEV_ALLOWED_ORIGINS: ' dev.example.com, ,preview.example.com , 192.0.2.1 ' })).toEqual([
    'localhost',
    '127.0.0.1',
    'dev.example.com',
    'preview.example.com',
    '192.0.2.1',
  ]);
}

function allowedDevOriginsRejectSchemeOrPath() {
  for (const entry of [
    'https://dev.example.com',
    'http://dev.example.com',
    'dev.example.com:3050',
    'dev.example.com/path',
    'dev.example.com/',
    'dev.example.com\\path',
    '*.example.com',
    'dev.*.com',
    '*',
  ]) {
    expect(() => allowedDevOrigins({ NEXT_DEV_ALLOWED_ORIGINS: entry }), entry).toThrow(TypeError);
    expect(() => allowedDevOrigins({ NEXT_DEV_ALLOWED_ORIGINS: `valid.example.com, ${entry} ` }), entry).toThrow(TypeError);
  }
}

test(allowedDevOriginsDefaultToLoopback);
test(allowedDevOriginsRejectSchemeOrPath);

async function backendRewritesRefuseMissingProductionUpstream() {
  const env = {
    NODE_ENV: 'production',
    WEB_GATEWAY_UPSTREAM: 'https://gateway.example.com',
    WEB_API_UPSTREAM: 'https://api.example.com',
    WEB_SOCKETS_UPSTREAM: 'https://sockets.example.com',
    WEB_CHARTS_UPSTREAM: 'https://charts.example.com',
  };

  await expect(backendRewrites({ NODE_ENV: 'production' })).rejects.toThrow(
    new Error('missing production upstream: WEB_GATEWAY_UPSTREAM'),
  );
  for (const name of [
    'WEB_GATEWAY_UPSTREAM',
    'WEB_API_UPSTREAM',
    'WEB_SOCKETS_UPSTREAM',
    'WEB_CHARTS_UPSTREAM',
  ] as const) {
    const missing: Record<string, string | undefined> = { ...env };
    delete missing[name];
    await expect(backendRewrites(missing), name).rejects.toThrow(new Error(`missing production upstream: ${name}`));
    await expect(backendRewrites({ ...env, [name]: '' }), name).rejects.toThrow(
      new Error(`missing production upstream: ${name}`),
    );
  }
}

async function backendRewritesEmptyInDevelopment() {
  const emptyRewrites = { beforeFiles: [], afterFiles: [], fallback: [] };
  expect(await backendRewrites({ NODE_ENV: 'development' })).toEqual(emptyRewrites);
  expect(await backendRewrites({})).toEqual(emptyRewrites);
  expect(
    await backendRewrites({
      NODE_ENV: 'development',
      WEB_GATEWAY_UPSTREAM: '',
      WEB_API_UPSTREAM: '',
      WEB_SOCKETS_UPSTREAM: '',
      WEB_CHARTS_UPSTREAM: '',
    }),
  ).toEqual(emptyRewrites);
}

async function backendRewritesKeepRouteTemplates() {
  const upstreams = {
    WEB_GATEWAY_UPSTREAM: 'https://gateway.example.com///',
    WEB_API_UPSTREAM: 'https://api.example.com/',
    WEB_SOCKETS_UPSTREAM: 'https://sockets.example.com//',
    WEB_CHARTS_UPSTREAM: 'https://charts.example.com/',
  };
  const expected = {
    beforeFiles: [
      { source: '/v2/auth/:path*', destination: 'https://gateway.example.com/v2/auth/:path*' },
      { source: '/v2/stream/tickets', destination: 'https://gateway.example.com/v2/stream/tickets' },
      { source: '/v2/control/:path*', destination: 'https://gateway.example.com/v2/control/:path*' },
      { source: '/v2/network/authority', destination: 'https://api.example.com/v2/network/authority' },
      { source: '/v2/preflight/:path*', destination: 'https://api.example.com/v2/preflight/:path*' },
      { source: '/v2/snapshots', destination: 'https://api.example.com/v2/snapshots' },
      { source: '/v2/stream', destination: 'https://sockets.example.com/v2/stream' },
      { source: '/v2/stream/sse', destination: 'https://sockets.example.com/v2/stream/sse' },
      { source: '/v2/udf/config', destination: 'https://charts.example.com/v2/udf/config' },
      { source: '/v2/udf/symbols', destination: 'https://charts.example.com/v2/udf/symbols' },
      { source: '/v2/udf/history', destination: 'https://charts.example.com/v2/udf/history' },
      { source: '/v2/udf/time', destination: 'https://charts.example.com/v2/udf/time' },
    ],
    afterFiles: [],
    fallback: [],
  };

  for (const NODE_ENV of ['production', 'development']) {
    expect(await backendRewrites({ ...upstreams, NODE_ENV })).toEqual(expected);
  }
}

test(backendRewritesRefuseMissingProductionUpstream);
test(backendRewritesEmptyInDevelopment);
test(backendRewritesKeepRouteTemplates);

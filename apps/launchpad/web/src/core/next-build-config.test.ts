import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { allowedDevOrigins, TRANSPILE_PACKAGES } from './next-build-config';

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

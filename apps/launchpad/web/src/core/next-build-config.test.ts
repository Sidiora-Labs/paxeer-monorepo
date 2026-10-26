import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { TRANSPILE_PACKAGES } from './next-build-config';

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

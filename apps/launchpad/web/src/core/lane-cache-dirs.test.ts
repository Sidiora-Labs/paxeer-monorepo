import { resolve } from 'node:path';
import { expect, test, vi } from 'vitest';

async function laneCacheDirsStayInWorkspace() {
  const overrideNames = [
    'L25_VITEST_CACHE_DIR',
    'L26_CACHE_DIR',
    'L28_PRIVATE_CACHE_DIRECTORY',
  ] as const;
  const overrides = overrideNames.map(
    (name) => [name, process.env[name]] as const,
  );

  try {
    for (const name of overrideNames) delete process.env[name];
    vi.resetModules();

    const [{ default: l25 }, { default: l26 }, { default: l28 }] =
      await Promise.all([
        import('./wallet-sdk/vitest.l25.config'),
        import('../widgets/trading/transaction-controls/vitest.phase2.config'),
        import('../widgets/meta-ag/vitest.l28.config'),
      ]);
    const workspaceCacheDir = resolve(__dirname, '../../node_modules/.vite');

    for (const [lane, config] of [
      ['l25', l25],
      ['l26', l26],
      ['l28', l28],
    ] as const) {
      expect(config.cacheDir).toBe(resolve(workspaceCacheDir, lane));
    }
  } finally {
    for (const [name, value] of overrides) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
    vi.resetModules();
  }
}

test(laneCacheDirsStayInWorkspace);

import { defineConfig, mergeConfig } from 'vitest/config';
import { resolve } from 'node:path';
import baseline from '../../../vitest.config';
export default mergeConfig(
  baseline,
  defineConfig({
    cacheDir:
      process.env.L25_VITEST_CACHE_DIR ??
      resolve(process.cwd(), 'node_modules/.vite/l25'),
    test: { maxWorkers: 1, minWorkers: 1 },
  }),
);

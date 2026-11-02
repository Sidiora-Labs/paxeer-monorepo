import { defineConfig, mergeConfig } from 'vitest/config';
import base from '../../../vitest.config';
import { fileURLToPath } from 'node:url';
export default mergeConfig(
  base,
  defineConfig({
    cacheDir:
      process.env.L28_PRIVATE_CACHE_DIRECTORY ??
      fileURLToPath(
        new URL('../../../node_modules/.vite/l28', import.meta.url),
      ),
    test: { maxWorkers: 1, minWorkers: 1 },
  }),
);

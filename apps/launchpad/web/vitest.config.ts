import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  esbuild: { jsx: 'automatic', jsxImportSource: 'react' },
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  test: {
    environment: 'node',
    // Wallet suites share one private EVM and exercise chain/account changes.
    fileParallelism: false,
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    globals: false,
    // Suites run against loopback backend origins by default and override
    // them per suite as needed.
    env: {
      NEXT_PUBLIC_DATA_API_URL: 'http://127.0.0.1:18081',
      NEXT_PUBLIC_GATEWAY_URL: 'http://127.0.0.1:18082',
      NEXT_PUBLIC_METADATA_API_URL: 'http://127.0.0.1:18083',
      NEXT_PUBLIC_PNL_API_URL: 'http://127.0.0.1:18084',
    },
    // @kindle/sdk dist validators read `.default` off CJS ajv runtime modules;
    // vitest's default interop unwraps it and breaks every SDK-importing suite.
    deps: { interopDefault: false },
  },
});

import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  resolve: {
    alias: [
      {
        find: /^\.\/(validators-(?:v1|v2|operations)\.gen\.js)$/,
        replacement: fileURLToPath(
          new URL('../../../sdk/launchpad/packages/sdk/dist/$1', import.meta.url),
        ),
      },
    ],
  },
  test: {
    environment: 'node',
    include: ['test/vitest/**/*.test.ts'],
    deps: { interopDefault: false },
  },
});

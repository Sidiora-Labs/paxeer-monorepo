import svgr from 'vite-plugin-svgr';
import tsconfigPaths from 'vite-tsconfig-paths';

import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [
    tsconfigPaths({
      ignoreConfigErrors: true,
    }),
    svgr({
      exportAsDefault: true,
    }),
  ],
  test: {
    environment: 'node',
    include: [ '**/*.spec.ts', '**/*.spec.tsx' ],
    exclude: [ '**/node_modules/**', '**/node_modules_linux/**' ],
  },
});

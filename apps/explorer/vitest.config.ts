import { fileURLToPath } from 'node:url';
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
    // Discover from the repository so both repository and app-relative CLI filters match.
    dir: fileURLToPath(new URL('../..', import.meta.url)),
    globalSetup: [ './vitest/global-setup.ts' ],
    setupFiles: [ './vitest/setup.ts' ],
    include: [ 'apps/explorer/**/*.spec.ts', 'apps/explorer/**/*.spec.tsx' ],
    exclude: [ '**/node_modules/**', '**/node_modules_linux/**' ],
  },
});

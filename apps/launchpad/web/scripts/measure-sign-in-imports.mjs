import { createServer } from 'vite';
import { JSDOM } from 'jsdom';
import { writeFileSync } from 'node:fs';

const output = process.argv[2];
if (!output) throw new Error('A private measurement output file is required');
const browser = new JSDOM('', { url: 'http://127.0.0.1:1' });
for (const name of [
  'window',
  'document',
  'navigator',
  'localStorage',
  'HTMLElement',
  'HTMLMediaElement',
])
  Object.defineProperty(globalThis, name, {
    configurable: true,
    value: browser.window[name],
  });
process.env.NEXT_PUBLIC_SUPABASE_URL = 'http://127.0.0.1:1';
process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY = 'sb_publishable_closed_local_boundary';
process.env.NEXT_PUBLIC_DATA_API_URL = 'http://127.0.0.1:18081';
process.env.NEXT_PUBLIC_GATEWAY_URL = 'http://127.0.0.1:18082';
const server = await createServer({
  configFile: 'vitest.config.ts',
  server: { middlewareMode: true },
  appType: 'custom',
});
const imports = [];
const started = performance.now();
try {
  for (const name of [
    '/src/shell/sign-in-modal.tsx',
    '/src/core/wagmi-config.ts',
    '/src/core/wallet-sdk/runtime.tsx',
    '/src/shell/auth-modal-store.ts',
  ]) {
    const before = performance.now();
    await server.ssrLoadModule(name);
    imports.push({ name, durationMs: performance.now() - before });
  }
  const result = {
    loader: 'Vite SSR module loader; no tests executed',
    imports,
    durationMs: performance.now() - started,
  };
  writeFileSync(output, JSON.stringify(result, null, 2) + '\n', { mode: 0o600, flag: 'wx' });
  console.info(JSON.stringify(result));
} finally {
  await server.close();
  browser.window.close();
}
process.exit(0);

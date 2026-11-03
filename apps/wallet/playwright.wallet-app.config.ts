import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { defineConfig } from '@playwright/test';
import { APP_ENV } from './e2e/environment';

const serverLog = join(tmpdir(), 'paxeer-wallet-app-webserver.log');
const server = `
import next from "next";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
const configURL = new URL("./next.config.mjs", import.meta.url).href;
registerHooks({
  load(url, context, nextLoad) {
    if (url !== configURL) return nextLoad(url, context);
    return {
      format: "module",
      source: \`import config from "\${configURL}?wallet-e2e"; export default { ...config, basePath: "" };\`,
      shortCircuit: true,
    };
  },
});
const { default: config } = await import(configURL);
const app = next({
  dev: true,
  webpack: true,
  hostname: "127.0.0.1",
  port: 3099,
  conf: { ...config, basePath: "" },
});
await app.prepare();
const handle = app.getRequestHandler();
createServer((request, response) => {
  if (request.url === "/wallet") {
    response.writeHead(308, { location: "/wallet/" });
    response.end();
    return;
  }
  if (request.url?.startsWith("/wallet/")) {
    request.url = request.url.slice("/wallet".length);
  }
  return handle(request, response);
}).listen(3099, "127.0.0.1");
`;

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  globalSetup: './e2e/global-setup.ts',
  timeout: 120_000,
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:3099/wallet/',
    browserName: 'chromium',
    headless: true,
  },
  webServer: {
    command: `node --input-type=module -e '${server}' 2>&1 | tee "$WALLET_APP_SERVER_LOG"`,
    url: 'http://127.0.0.1:3099/wallet/',
    reuseExistingServer: false,
    stdout: 'pipe',
    timeout: 600_000,
    env: {
      ...APP_ENV,
      NODE_ENV: 'development',
      WALLET_APP_SERVER_LOG: serverLog,
    },
  },
});

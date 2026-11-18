import { createServer } from 'node:http';
import { createRequire, enableCompileCache, flushCompileCache } from 'node:module';
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const directory = fileURLToPath(new URL('../../../../apps/explorer/', import.meta.url));
const compileCache = enableCompileCache();
if (!compileCache.directory) throw new Error(compileCache.message);
const require = createRequire(directory + 'package.json');
const environment = JSON.parse(await readFile(new URL('./environment.json', import.meta.url), 'utf8'));
const zetaChainCCTX = await readFile(new URL('./zeta-chain.json', import.meta.url));
Object.assign(process.env, environment, { NODE_ENV: 'production', DISABLE_TRACKING: 'true', NEXT_TELEMETRY_DISABLED: '1' });
const next = require('next');
const { WebSocketServer } = require('ws');
const staticAssets = new Map();
async function loadStaticAssets(relative = '') {
  const root = join(directory, '.next/static', relative);
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const filename = join(relative, entry.name);
    if (entry.isDirectory()) {
      await loadStaticAssets(filename);
    } else if (entry.name.endsWith('.js') || entry.name.endsWith('.css')) {
      const body = await readFile(join(root, entry.name));
      staticAssets.set('/explorer/_next/static/' + filename, {
        body,
        compressed: gzipSync(body),
        type: entry.name.endsWith('.js') ? 'application/javascript' : 'text/css',
      });
    }
  }
}
await loadStaticAssets();
const application = next({ dir: directory, dev: false, hostname: 'localhost', port: 4173 });
await application.prepare();
const handle = application.getRequestHandler();
let ready = false;
const server = createServer(async (request, response) => {
  const url = new URL(request.url, 'http://localhost:4173');
  try {
    if (url.pathname === '/__surface_ready') {
      response.writeHead(ready ? 200 : 503, { 'content-type': 'text/plain' });
      response.end('Explorer');
      return;
    }
    const asset = staticAssets.get(url.pathname);
    if (asset && (request.method === 'GET' || request.method === 'HEAD')) {
      const compressed = /\bgzip\b/.test(request.headers['accept-encoding'] ?? '');
      const body = compressed ? asset.compressed : asset.body;
      response.writeHead(200, {
        'content-type': asset.type,
        'content-length': body.length,
        'cache-control': 'public, max-age=31536000, immutable',
        vary: 'Accept-Encoding',
        ...(compressed ? { 'content-encoding': 'gzip' } : {}),
      });
      response.end(request.method === 'HEAD' ? undefined : body);
      return;
    }
    if (url.pathname === '/__surface_pending') return;
    if (url.pathname === '/api/v1/CctxInfo:get' || url.pathname === '/explorer/node-api/proxy/api/v1/CctxInfo:get') {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(zetaChainCCTX);
      return;
    }
    if (url.pathname === '/explorer/icons/sprite.svg') {
      response.writeHead(200, { 'content-type': 'image/svg+xml' });
      response.end(await readFile('/tmp/explorer-icons/sprite.svg'));
      return;
    }
    if (url.pathname === '/explorer/icons/registry.json') {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(await readFile('/tmp/explorer-icons/registry.json'));
      return;
    }
    if (url.pathname === '/explorer/assets/envs.js') {
      response.writeHead(200, { 'content-type': 'application/javascript' });
      response.end('window.__envs = ' + JSON.stringify(environment) + ';');
      return;
    }
    const apiPath = url.pathname.replace(/^\/explorer\/node-api\/proxy(?=\/api\/)/, '');
    if (apiPath.startsWith('/api/v2/')) {
      response.writeHead(307, { location: 'https://explorer.example.test' + apiPath + url.search });
      response.end();
      return;
    }
    await handle(request, response);
  } catch (error) {
    console.error(error);
    if (!response.headersSent) response.writeHead(500, { 'content-type': 'text/plain' });
    response.end('Cannot serve explorer');
  }
});
const sockets = new WebSocketServer({ noServer: true });
server.on('upgrade', (request, socket, head) => {
  if (!request.url.startsWith('/socket/')) {
    socket.destroy();
    return;
  }
  sockets.handleUpgrade(request, socket, head, (client) => sockets.emit('connection', client, request));
});
sockets.on('connection', (client) => {
  client.on('message', (message) => {
    const [join, reference, topic, event] = JSON.parse(message.toString());
    if (event === 'phx_join' || event === 'heartbeat') {
      client.send(JSON.stringify([join, reference, topic, 'phx_reply', { status: 'ok', response: {} }]));
    }
  });
});
server.listen(4173, 'localhost', () => {
  prepare().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
});
async function prepare() {
  const preparing = process.argv.includes('--prepare');
  try {
    const response = await fetch('http://localhost:4173/explorer/404', { signal: AbortSignal.timeout(10000) });
    await response.arrayBuffer();
    if (response.status !== 404) throw new Error('Explorer not-found route must answer with status 404');
    ready = true;
    if (preparing) {
      const { assertRollupPages } = await import('./rollup.mjs');
      await assertRollupPages();
      flushCompileCache();
    }
  } finally {
    if (preparing || !ready) await close();
  }
}
async function close() {
  for (const client of sockets.clients) client.terminate();
  sockets.close();
  server.closeAllConnections();
  server.close();
  await application.close();
}
process.on('SIGTERM', () => void close());
process.on('SIGINT', () => void close());

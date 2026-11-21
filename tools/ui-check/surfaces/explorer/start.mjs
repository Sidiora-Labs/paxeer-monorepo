import { execFileSync } from 'node:child_process';
import { createServer } from 'node:http';
import { createRequire, enableCompileCache, flushCompileCache } from 'node:module';
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

import { watchSurfaceOwner } from './lifecycle.mjs';
import { tokenServerAnswer } from './tokens.mjs';
import { readBlockResponses } from './block-responses.mjs';
import { prepareAddressResponses, readAddressResponses } from './address-responses.mjs';

const preparing = process.argv.includes('--prepare');
const directory = fileURLToPath(new URL('../../../../apps/explorer/', import.meta.url));
const compileCache = enableCompileCache();
if (!compileCache.directory) throw new Error(compileCache.message);
process.env.NODE_COMPILE_CACHE = compileCache.directory;
const require = createRequire(directory + 'package.json');
const environment = JSON.parse(await readFile(new URL('./environment.json', import.meta.url), 'utf8'));
const zetaChainCCTX = await readFile(new URL('./zeta-chain.json', import.meta.url));
const statistics = JSON.parse(await readFile(new URL('./statistics-service.json', import.meta.url), 'utf8'));
const transactionAnswers = JSON.parse(await readFile(join(directory, '.next/cache/explorer-transaction-answers.json'), 'utf8'));
if (transactionAnswers.buildId !== (await readFile(join(directory, '.next/BUILD_ID'), 'utf8')).trim()) throw new Error('Transaction answers must match the explorer build');
const transactionSurface = { answers: transactionAnswers.answers.map((answer) => ({ ...answer, pattern: new RegExp(answer.pattern.source, answer.pattern.flags), body: Buffer.from(answer.body, 'base64') })) };
const productionConfig = JSON.parse(await readFile(join(directory, '.next/required-server-files.json'), 'utf8')).config;
productionConfig.experimental.preloadEntriesOnStart = false;
Object.assign(process.env, environment, { NODE_ENV: 'production', DISABLE_TRACKING: 'true', NEXT_TELEMETRY_DISABLED: '1' });
process.env.__NEXT_PRIVATE_STANDALONE_CONFIG = JSON.stringify(productionConfig);
const next = require('next');
const { WebSocketServer } = require('ws');
const staticAssets = new Map();
let blockResponses = new Map();
let addressResponses = new Map();
async function loadStaticAssets(relative = '') {
  const root = join(directory, '.next/static', relative);
  const entries = await readdir(root, { withFileTypes: true });
  await Promise.all(entries.map(async (entry) => {
    const filename = join(relative, entry.name);
    if (entry.isDirectory()) {
      await loadStaticAssets(filename);
    } else if (entry.name.endsWith('.js') || entry.name.endsWith('.css')) {
      staticAssets.set('/explorer/_next/static/' + filename, {
        filename: join(root, entry.name),
        body: null,
        compressed: null,
        type: entry.name.endsWith('.js') ? 'application/javascript' : 'text/css',
      });
    }
  }));
}
async function readAssetBody(asset) {
  if (!asset.body) asset.body = await readFile(asset.filename);
  return asset.body;
}
const application = next({ dir: directory, dev: false, hostname: 'localhost', port: 4173 });
const [, , iconSprite] = await Promise.all([
  application.prepare(),
  loadStaticAssets(),
  readFile('/tmp/explorer-icons/sprite.svg'),
]);
const compressedIconSprite = gzipSync(iconSprite);
const notFoundPage = preparing ? null : await readFile(join(directory, '.next/server/pages/404.html'));
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
    if (notFoundPage && url.pathname === '/explorer/404' && (request.method === 'GET' || request.method === 'HEAD')) {
      response.writeHead(404, { 'content-type': 'text/html; charset=utf-8', 'content-length': notFoundPage.length });
      response.end(request.method === 'HEAD' ? undefined : notFoundPage);
      return;
    }
    const asset = staticAssets.get(decodeURI(url.pathname));
    if (asset && (request.method === 'GET' || request.method === 'HEAD')) {
      const compressed = /\bgzip\b/.test(request.headers['accept-encoding'] ?? '');
      await readAssetBody(asset);
      if (compressed && !asset.compressed) asset.compressed = gzipSync(asset.body);
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
      const compressed = /\bgzip\b/.test(request.headers['accept-encoding'] ?? '');
      const body = compressed ? compressedIconSprite : iconSprite;
      response.writeHead(200, {
        'content-type': 'image/svg+xml',
        'content-length': body.length,
        vary: 'Accept-Encoding',
        ...(compressed ? { 'content-encoding': 'gzip' } : {}),
      });
      response.end(request.method === 'HEAD' ? undefined : body);
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
    const documentData = {
      '/api/v2/stats': 'stats.json',
      '/api/v2/main-page/blocks': 'home-blocks.json',
    }[url.pathname];
    if (documentData) {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(await readFile(new URL('./' + documentData, import.meta.url)));
      return;
    }
    const apiPath = url.pathname.replace(/^\/explorer\/node-api\/proxy(?=\/api(?:\/|$))/, '');
    if (apiPath === '/api/v1/clusters/render/chain-metrics') {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(await readFile(new URL('./services/chain-metrics.json', import.meta.url)));
      return;
    }
    if (url.pathname.startsWith('/api/v2/')) {
      const address = 'https://explorer.example.test' + apiPath + url.search;
      const answer = transactionSurface.answers.find((entry) => entry.method === request.method && entry.pattern.test(address));
      if (answer) {
        response.writeHead(answer.status ?? 200, answer.headers);
        response.end(answer.body);
        return;
      }
    }
    if (apiPath.startsWith('/api/v2/') || apiPath === '/api/v1/solidity:visualize-contracts' || (apiPath === '/api' && url.searchParams.get('action') === 'getblockcountdown')) {
      const answer = !request.headers['user-agent']?.startsWith('Mozilla/') && tokenServerAnswer(request.method, apiPath, url.search);
      if (answer) {
        response.writeHead(answer.status ?? 200, answer.headers);
        response.end(answer.body);
        return;
      }
      response.writeHead(307, { location: 'https://explorer.example.test' + apiPath + url.search });
      response.end();
      return;
    }
    if (apiPath.startsWith('/api/v1/lines/') || ['/api/v1/docs/swagger.yaml', '/api/v1/lines', '/api/v1/counters', '/api/v1/pages/main', '/api/v1/pages/transactions', '/api/v1/pages/contracts'].includes(apiPath)) {
      const chart = apiPath.startsWith('/api/v1/lines/') ? statistics.charts[decodeURIComponent(apiPath.slice('/api/v1/lines/'.length))] : undefined;
      const payload = apiPath === '/api/v1/docs/swagger.yaml' ? JSON.parse(await readFile(new URL('./statistics-openapi.json', import.meta.url), 'utf8')) :
        apiPath === '/api/v1/lines' ? statistics.lines :
        apiPath === '/api/v1/counters' ? statistics.counters :
        apiPath === '/api/v1/pages/main' ? statistics.main :
        apiPath === '/api/v1/pages/transactions' ? JSON.parse(await readFile(new URL('./stats-txs.json', import.meta.url), 'utf8')) :
        apiPath === '/api/v1/pages/contracts' ? statistics.contracts :
        chart ? { info: chart.info, chart: chart.chart ?? statistics.series } : undefined;
      response.writeHead(payload ? 200 : 404, { 'content-type': 'application/json' });
      response.end(JSON.stringify(payload ?? { message: 'Chart not found' }));
      return;
    }
    const blockResponse = !url.search && (blockResponses.get(url.pathname) ?? addressResponses.get(url.pathname));
    if (blockResponse && (request.method === 'GET' || request.method === 'HEAD')) {
      const compressed = /\bgzip\b/.test(request.headers['accept-encoding'] ?? '');
      const body = compressed ? blockResponse.compressed : blockResponse.body;
      response.writeHead(200, {
        'content-type': blockResponse.type,
        'content-length': body.length,
        vary: 'Accept-Encoding',
        ...(compressed ? { 'content-encoding': 'gzip' } : {}),
      });
      response.end(request.method === 'HEAD' ? undefined : body);
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
  try {
    const response = await fetch('http://localhost:4173/explorer/404', { signal: AbortSignal.timeout(10000) });
    await response.arrayBuffer();
    if (response.status !== 404) throw new Error('Explorer not-found route must answer with status 404');
    if (preparing) {
      const { assertRollupPages } = await import('./rollup.mjs');
      await assertRollupPages();
      const { assertTokenPages } = await import('./tokens.mjs');
      await assertTokenPages();
      for (const path of ['/api-docs', '/gas-tracker', '/stats/coinPrice', '/stats']) {
        const document = await fetch('http://localhost:4173/explorer' + path, { signal: AbortSignal.timeout(10000) });
        await document.arrayBuffer();
        if (document.status !== 200) throw new Error(`${path}: statistics document must answer with status 200`);
      }
      await prepareAddressResponses();
      await (await import('./blocks.mjs')).prepareBlockResponses();
      blockResponses = await readBlockResponses();
      await (await import('./blocks.mjs')).assertBlockPages();
      await (await import('./transactions.mjs')).assertTransactionPages();
      flushCompileCache();
    } else {
      blockResponses = await readBlockResponses();
    }
    addressResponses = await readAddressResponses();
    for (const response of [...blockResponses.values(), ...addressResponses.values()]) {
      for (const [, path] of response.body.toString().matchAll(/(?:src|href)="(\/explorer\/_next\/static\/[^"?]+\.(?:js|css))(?:\?[^\"]*)?"/g)) {
        const asset = staticAssets.get(decodeURI(path));
        if (!asset) throw new Error(`Prepared block asset must exist: ${path}`);
        if (!asset.compressed) asset.compressed = gzipSync(await readAssetBody(asset));
      }
    }
    ready = true;
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
if (!process.argv.includes('--prepare')) {
  const ownerPid = Number(execFileSync('ps', ['-o', 'ppid=', '-p', String(process.ppid)], { encoding: 'utf8' }).trim());
  watchSurfaceOwner(ownerPid, () => void close());
}
process.on('SIGTERM', () => void close());
process.on('SIGINT', () => void close());

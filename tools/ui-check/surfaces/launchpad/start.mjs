import { spawn } from 'node:child_process';
import { cp, readFile, readdir, realpath } from 'node:fs/promises';
import { resolve, relative, extname } from 'node:path';
import { createServer } from 'node:https';
import { request } from 'node:http';
import { createRequire } from 'node:module';
import { gzipSync } from 'node:zlib';
import { origin, applicationEnvironment } from './environment.mjs';
import { certificate, privateKey } from './prepare.mjs';

const require = createRequire(new URL('../../../../apps/launchpad/web/package.json', import.meta.url));
const { WebSocketServer } = require('ws');
const directory = new URL('./answers/', import.meta.url);
const answerNames = [
  'pool', 'coin-pool', 'pools', 'stats', 'pool-stats', 'holder-rewards-stats', 'holder-rewards-stats-empty', 'holder-rewards-pools', 'items-empty', 'feed-empty',
  'streams-empty', 'platform', 'metadata', 'metadata-batch', 'metadata-batch-empty', 'metadata-v2', 'metadata-media', 'legacy-holders-empty', 'candles-empty', 'holders-empty', 'profile',
  'profile-empty', 'follow-stats', 'follow-stats-empty', 'portfolio', 'portfolio-empty', 'bounties', 'bounty', 'bounties-stats', 'unavailable',
  'rpc-unavailable', 'chart-config', 'chart-symbol', 'chart-time', 'chart-history-empty', 'not-found', 'explorer-empty', 'snapshot', 'stream-control', 'search', 'search-token', 'authority', 'public-share', 'cup-current-empty', 'comments-empty',
  ...['trending', 'new', 'volume', 'marketcap', 'gainers', 'losers', 'graduating', 'graduated', 'movers', 'breakout', 'unusual']
    .flatMap(category => [`rankings-${category}`, `rankings-${category}-empty`]),
  ...['trader', 'creator', 'volume'].flatMap(category =>
    ['24h', '7d', '30d'].flatMap(window => [
      `leaderboard-${category}-${window}`, `leaderboard-${category}-${window}-empty`,
    ])),
];
const answers = new Map(await Promise.all(answerNames.map(async name =>
  [name, await readFile(new URL(`${name}.json`, directory))])));
const returning = await readFile(new URL('returning.js', directory));
const loading = await readFile(new URL('loading.js', directory));
const surface = JSON.parse(await readFile(new URL('./parts/explore.json', import.meta.url), 'utf8'));
const rewardsSurface = JSON.parse(await readFile(new URL('./parts/rewards.json', import.meta.url), 'utf8'));
const loadingPaths = new Set([...surface.views, ...rewardsSurface.views].filter(view => {
  const url = new URL(view.path, origin);
  return url.searchParams.get('render') === 'loading';
}).map(view => view.path));

function documentScripts(path) {
  return '<head><script src="/__surface/returning.js"></script>'
    + (loadingPaths.has(path) ? '<script src="/__surface/loading.js"></script>' : '');
}
const scriptDecoder = await readFile(new URL('./assets/fflate.min.js', import.meta.url), 'utf8');
const standaloneDirectory = resolve('apps/launchpad/web/.next/standalone/web');
await cp('apps/launchpad/web/public', resolve(standaloneDirectory, 'public'), { recursive: true });
const app = spawn(process.execPath, [resolve(standaloneDirectory, 'server.js')], {
  env: { ...applicationEnvironment(), HOSTNAME: '127.0.0.1', PORT: '4174' }, stdio: 'inherit',
});

function scenario(incoming) {
  const referrer = new URL(incoming.headers.referer ?? '/', origin);
  const value = referrer.searchParams.get('render');
  return ['empty', 'loading', 'error'].includes(value) ? value : 'filled';
}

function answerFor(url, state, pagePath) {
  const path = url.pathname;
  const empty = state === 'empty';
  if (/^\/v1\/pools\/0x[0-9a-f]{40}\/holders$/.test(path)) return { name: 'holders-empty' };
  if (/^\/stats\/0x[0-9a-f]{40}\/holders$/.test(path)) return { name: 'legacy-holders-empty' };
  if (path === '/v2/network/authority') return { name: 'authority' };
  if (path === '/v3/portfolio/shares/7f3b2a10-5c4d-4e8f-9a1b-2c3d4e5f6a7b') return { name: 'public-share' };
  if (path === '/v3/portfolio/shares/11111111-1111-4111-8111-111111111111') return { name: 'unavailable', status: 410 };
  if (path === '/v3/portfolio/shares/22222222-2222-4222-8222-222222222222') return { name: 'not-found', status: 404 };
  if (path === '/v3/portfolio/shares/33333333-3333-4333-8333-333333333333') return { name: 'unavailable', status: 503 };
  if (path.startsWith('/v2/control/'))
    return { name: 'unavailable', status: 503 };
  if (/^\/users\/0x[0-9a-f]{40}\/comments$/.test(path)) return { name: 'comments-empty' };
  if (state === 'error' && !path.startsWith('/v2/udf/') && !path.endsWith('/candles') && path !== '/v2/snapshots') return { name: 'unavailable', status: 503 };
  if (path === '/v2/cups/current') return { name: 'cup-current-empty' };
  const ranking = /^\/v2\/rankings\/([a-z]+)$/.exec(path);
  if (ranking) return { name: `rankings-${ranking[1]}${empty ? '-empty' : ''}` };
  const board = /^\/v2\/leaderboards\/(trader|creator|volume)$/.exec(path);
  if (board) return { name: `leaderboard-${board[1]}-${url.searchParams.get('window') ?? '24h'}${empty ? '-empty' : ''}` };
  if (path === '/v2/udf/config') return { name: 'chart-config' };
  if (path === '/v2/udf/symbols') return { name: 'chart-symbol' };
  if (path === '/v2/udf/time') return { name: 'chart-time' };
  if (path === '/v2/udf/history') return { name: 'chart-history-empty' };
  if (path === '/v2/platform/metrics') return { name: 'platform' };
  if (path === '/v2/pools/stats:batch') return { name: empty ? 'items-empty' : 'stats' };
  if (path === '/v2/pools') return { name: empty ? 'items-empty' : 'pools' };
  if (path === '/v2/pools/0x1111111111111111111111111111111111111111') return { name: /^\/(coin|token)\/0x[0-9a-f]{40}$/i.test(pagePath) ? 'coin-pool' : 'pool' };
  if (/^\/v2\/pools\/0x[0-9a-f]{40}\/stats$/.test(path)) return { name: 'pool-stats' };
  if (/^\/v2\/pools\/0x[0-9a-f]{40}\/candles$/.test(path)) return { name: 'candles-empty' };
  if (/^\/v2\/pools\/0x[0-9a-f]{40}\/holders$/.test(path)) return { name: 'holders-empty' };
  if (/^\/v2\/pools\/0x[0-9a-f]{40}\/callouts$/.test(path)) return { name: 'items-empty' };
  if (/^\/v2\/pools\/0x[0-9a-f]{40}\/trades$/.test(path)) return { name: 'items-empty' };
  if (path === '/v2/tokens/0x3333333333333333333333333333333333333333/media') return { name: 'metadata-media' };
  if (path === '/v2/tokens/0x3333333333333333333333333333333333333333/metadata') return { name: 'metadata-v2' };
  if (/^\/v2\/pools\/0x[0-9a-f]{40}$/.test(path)) return { name: 'not-found', status: 404 };
  if (path === '/v2/discovery/feed') return { name: 'feed-empty' };
  if (path === '/v2/snapshots') return { name: 'snapshot' };
  if (/^\/v2\/streams\/(live|pool\/0x[0-9a-f]{40})$/.test(path)) return { name: 'streams-empty' };
  if (path === '/v2/bounties/stats') return { name: 'bounties-stats' };
  if (path === '/v2/bounties') return { name: empty || url.searchParams.has('state') && url.searchParams.get('state') !== 'open' ? 'items-empty' : 'bounties' };
  if (path === '/v2/bounties/8e894db0-87e7-4d2a-9f1c-3a192a239ba0') return { name: 'bounty' };
  if (path === '/v2/bounties/8e894db0-87e7-4d2a-9f1c-3a192a239ba0/submissions') return { name: 'items-empty' };
  if (path === '/metadata/batch') return { name: empty ? 'metadata-batch-empty' : 'metadata-batch' };
  if (path === '/metadata/0x3333333333333333333333333333333333333333') return { name: 'metadata' };
  if (/^\/v2\/profiles\/0x[0-9a-f]{40}\/follow-stats$/.test(path)) return { name: empty ? 'follow-stats-empty' : 'follow-stats' };
  if (/^\/v3\/wallets\/0x[0-9a-f]{40}\/portfolio$/.test(path)) return { name: empty ? 'portfolio-empty' : 'portfolio' };
  if (/^\/v3\/profiles\/0x[0-9a-f]{40}$/.test(path)) return { name: empty ? 'profile-empty' : 'profile' };
  if (path === '/v2/search') return { name: empty || url.searchParams.get('type') === 'profile' ? 'items-empty' : url.searchParams.get('type') === 'token' ? 'search-token' : 'search' };
  if (path === '/v2/holder-rewards/stats') return { name: empty ? 'holder-rewards-stats-empty' : 'holder-rewards-stats' };
  if (path === '/v2/holder-rewards/pools') return { name: empty ? 'items-empty' : 'holder-rewards-pools' };
  if (path === '/v2/profiles:batch') return { name: 'items-empty' };
  if (path.startsWith('/explorer/api/v2/')) return { name: 'explorer-empty' };
  return { name: 'unavailable', status: 503 };
}

const chartAssets = await realpath('apps/launchpad/web/public/charting_library');
const nextAssets = await realpath('apps/launchpad/web/.next/static');
const fontAssets = await realpath('apps/launchpad/web/public/fonts');
const compressedAssets = new Map();
let pagesReady = false;
const assetTypes = { '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff': 'font/woff', '.woff2': 'font/woff2', '.otf': 'font/otf', '.ttf': 'font/ttf', '.png': 'image/png', '.html': 'text/html' };

function cacheAsset(file) {
  if (!compressedAssets.has(file)) {
    compressedAssets.set(file, readFile(file).then(original => {
      let body = original;
      if (file.startsWith(`${nextAssets}/`) && extname(file) === '.js' && original.length > 1024 * 1024) {
        const packed = gzipSync(original, { level: 9 }).toString('base64');
        body = Buffer.from(`(() => {
          const nonce = document.currentScript.nonce;
          var module = { exports: {} }, exports = module.exports;
          ${scriptDecoder};
          const packed = atob('${packed}');
          const bytes = Uint8Array.from(packed, character => character.charCodeAt(0));
          const script = document.createElement('script');
          script.nonce = nonce;
          script.textContent = new TextDecoder('utf-8', { fatal: true }).decode(module.exports.gunzipSync(bytes));
          document.head.append(script);
          script.remove();
        })();`);
      }
      return { body, gzip: gzipSync(body, { level: 1 }) };
    }));
  }
  return compressedAssets.get(file);
}

for (const root of [nextAssets, chartAssets, fontAssets]) {
  const entries = await readdir(root, { recursive: true, withFileTypes: true });
  await Promise.all(entries.filter(entry => entry.isFile()).map(entry => cacheAsset(resolve(entry.parentPath, entry.name))));
}

const server = createServer({ cert: await readFile(certificate), key: await readFile(privateKey), keepAliveTimeout: 60_000, headersTimeout: 65_000 }, async (incoming, outgoing) => {
  const url = new URL(incoming.url, origin);
  if (url.pathname === '/' && incoming.method === 'GET' && !pagesReady) {
    outgoing.writeHead(503, { 'retry-after': '1' });
    outgoing.end();
    return;
  }
  const assets = [
    ['/_next/static/', nextAssets],
    ['/charting_library/', chartAssets],
    ['/fonts/', fontAssets],
  ].find(([prefix]) => url.pathname.startsWith(prefix));
  if (assets) {
    try {
      const [prefix, root] = assets;
      const file = await realpath(resolve(root, decodeURIComponent(url.pathname.slice(prefix.length))));
      const within = relative(root, file);
      if (!within || within.startsWith('..') || within.startsWith('/')) throw new Error('Invalid asset path');
      const asset = await cacheAsset(file);
      const compressed = /\bgzip\b/.test(incoming.headers['accept-encoding'] ?? '');
      const body = compressed ? asset.gzip : asset.body;
      outgoing.writeHead(200, {
        'content-type': assetTypes[extname(file)] ?? 'application/octet-stream',
        'cache-control': 'public, max-age=31536000, immutable',
        'content-length': body.length,
        'vary': 'Accept-Encoding',
        ...(root === chartAssets ? { 'permissions-policy': 'unload=(self)' } : {}),
        ...(compressed ? { 'content-encoding': 'gzip' } : {}),
      });
      outgoing.end(incoming.method === 'HEAD' ? undefined : body);
    } catch {
      outgoing.writeHead(404); outgoing.end();
    }
    return;
  }
  if (url.pathname === '/' && incoming.method === 'POST') {
    const chunks = [];
    for await (const chunk of incoming) chunks.push(chunk);
    const payload = JSON.parse(Buffer.concat(chunks).toString());
    const refusal = JSON.parse(answers.get('rpc-unavailable'));
    const respond = call => ({ ...refusal, id: call.id });
    outgoing.writeHead(200, { 'content-type': 'application/json', 'cache-control': 'no-store' });
    outgoing.end(JSON.stringify(Array.isArray(payload) ? payload.map(respond) : respond(payload)));
    return;
  }
  if (url.pathname === '/__surface/returning.js') {
    outgoing.writeHead(200, { 'content-type': 'text/javascript', 'cache-control': 'no-store' });
    outgoing.end(returning);
    return;
  }
  if (url.pathname === '/__surface/loading.js') {
    outgoing.writeHead(200, { 'content-type': 'text/javascript', 'cache-control': 'no-store' });
    outgoing.end(loading);
    return;
  }
  if (/^\/(v[123]\/|users\/|stats\/|metadata\/|explorer\/api\/)/.test(url.pathname)) {
    const state = scenario(incoming);
    const pagePath = new URL(incoming.headers.referer ?? '/', origin).pathname;
    const homeLoading = state === 'loading' && pagePath === '/';
    const coinLoading = state === 'loading' && /^\/coin\/0x[0-9a-f]{40}$/.test(pagePath);
    const rewardsLoading = state === 'loading' && ['/holder-rewards', '/leaderboard'].includes(pagePath);
    const liveLoading = state === 'loading' && (['/live', '/live/all'].includes(pagePath) || /^\/profile\/0x[0-9a-f]{40}$/i.test(pagePath));
    const bountyLoading = state === 'loading' && pagePath === '/bounties';
    const bountyProbe = url.pathname === '/v2/bounties' && url.searchParams.get('limit') === '1';
    if (homeLoading && url.pathname === '/v2/discovery/feed') {
      await new Promise(resolve => setTimeout(resolve, 7500));
    } else if (coinLoading && /^\/v2\/pools\/0x[0-9a-f]{40}$/.test(url.pathname)) {
      await new Promise(resolve => setTimeout(resolve, 750));
    } else if ((rewardsLoading || liveLoading) && !url.pathname.startsWith('/v2/network/') && url.pathname !== '/v2/snapshots') {
      await new Promise(resolve => setTimeout(resolve, 350));
    } else if (bountyLoading && bountyProbe) {
      await new Promise(resolve => setTimeout(resolve, 100));
    } else if (state === 'loading' && !homeLoading && !coinLoading && !bountyLoading && !url.pathname.startsWith('/v2/network/') && url.pathname !== '/v2/snapshots') {
      await new Promise(resolve => outgoing.once('close', resolve));
      return;
    }
    const answer = answerFor(url, state, pagePath);
    let body = answers.get(answer.name);
    if (answer.name === 'snapshot') {
      const snapshot = JSON.parse(body);
      const requested = new Set((url.searchParams.get('topics') ?? '').split(','));
      snapshot.data.snapshot.topics = snapshot.data.snapshot.topics.filter(entry => requested.has(entry.topic));
      if (state !== 'error' && state !== 'loading') {
        for (const topic of requested) {
          if (!topic.startsWith('market.rankings.')) continue;
          const category = topic.slice('market.rankings.'.length);
          const file = answers.get(`rankings-${category}${state === 'empty' ? '-empty' : ''}`);
          if (!file) throw new Error('Ranking response is unavailable');
          const ranking = JSON.parse(file);
          const payload = ranking.data.snapshot;
          snapshot.data.snapshot.topics.push({ topic, sequence: '0', revision: ranking.revision, watermark: payload.watermark, payload });
        }
      }
      body = JSON.stringify(snapshot);
    }
    if (answer.name === 'authority') {
      const authority = JSON.parse(body);
      const now = new Date().toISOString();
      authority.attestation.observedAt = now;
      authority.attestation.blockTime = now;
      body = JSON.stringify(authority);
    }
    if (!body) {
      outgoing.writeHead(500, { 'content-type': 'application/json' });
      outgoing.end(JSON.stringify({ message: `No response file for ${answer.name}` }));
      return;
    }
    outgoing.writeHead(answer.status ?? 200, {
      'content-type': 'application/json', 'cache-control': 'no-store', 'x-schema-version': '2',
    });
    outgoing.end(body);
    return;
  }
  const upstream = request({
    hostname: '127.0.0.1', port: 4174, method: incoming.method, path: incoming.url,
    headers: {
      ...incoming.headers,
      connection: 'close',
      host: new URL(origin).host,
      'x-forwarded-host': new URL(origin).host,
      'x-forwarded-port': new URL(origin).port,
      'x-forwarded-proto': 'https',
      'accept-encoding': url.pathname.startsWith('/_next/static/') ? incoming.headers['accept-encoding'] ?? 'gzip, deflate, br' : 'identity',
    },
  }, response => {
    const headers = { ...response.headers };
    if (headers.location) headers.location = headers.location.replace('https://localhost:4174', origin);
    const html = headers['content-type']?.startsWith('text/html');
    if (!html || url.searchParams.get('render') === 'first-visit') {
      outgoing.writeHead(response.statusCode, headers);
      response.pipe(outgoing);
      return;
    }
    const chunks = [];
    response.on('data', chunk => chunks.push(chunk));
    response.on('end', () => {
      const body = Buffer.concat(chunks).toString().replace('<head>', documentScripts(incoming.url));
      delete headers['content-length'];
      delete headers['transfer-encoding'];
      outgoing.writeHead(response.statusCode, headers);
      outgoing.end(body);
    });
  });
  upstream.on('error', error => {
    if (!outgoing.headersSent) outgoing.writeHead(502, { 'content-type': 'text/plain' });
    outgoing.end(error.message);
  });
  incoming.pipe(upstream);
});
server.keepAliveTimeout = 30_000;
const sockets = new WebSocketServer({ noServer: true, handleProtocols: protocols => protocols.has('sidiora.v2') ? 'sidiora.v2' : false });
const control = JSON.parse(answers.get('stream-control'));
server.on('upgrade', (incoming, socket, head) => {
  if (new URL(incoming.url, origin).pathname !== '/v2/stream') { socket.destroy(); return; }
  sockets.handleUpgrade(incoming, socket, head, connection => {
    connection.on('message', bytes => {
      try {
        const message = JSON.parse(String(bytes));
        if (message.op === 'subscribe' || message.op === 'ping') {
          connection.send(JSON.stringify({ ...control, op: message.op === 'ping' ? 'heartbeat' : 'subscribed', subscriptionId: message.subscriptionId ?? null }));
        } else connection.close(1008, 'Unsupported operation');
      } catch { connection.close(1007, 'Invalid JSON'); }
    });
  });
});
async function warmPage(path) {
  const deadline = Date.now() + 15000;
  while (true) {
    try {
      await new Promise((resolve, reject) => {
        const warming = request({ hostname: '127.0.0.1', port: 4174, path, headers: {
          host: new URL(origin).host,
          'x-forwarded-host': new URL(origin).host,
          'x-forwarded-port': new URL(origin).port,
          'x-forwarded-proto': 'https',
        } }, response => {
          response.resume();
          response.on('end', () => {
            if (response.statusCode === 200) resolve();
            else if ([307, 308].includes(response.statusCode) && response.headers.location) {
              const destination = new URL(response.headers.location.replace('https://localhost:4174', origin), origin);
              if (destination.origin === origin) warmPage(destination.pathname + destination.search).then(resolve, reject);
              else reject(new Error('Page redirected outside the surface'));
            } else reject(new Error(`Page returned ${response.statusCode}`));
          });
          response.on('error', reject);
        });
        warming.setTimeout(15000, () => warming.destroy(new Error('Page did not answer')));
        warming.on('error', reject);
        warming.end();
      });
      return;
    } catch (error) {
      if (Date.now() >= deadline) throw error;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
  }
}

server.keepAliveTimeoutBuffer = 5000;

server.listen(4173, '203.0.113.1');
const warmPaths = [
  '/holder-rewards',
  '/leaderboard',
  '/meta-ag/orders',
  '/meta-ag/swap',
  '/swap',
  '/terminal',
  '/terminal/0x1111111111111111111111111111111111111111',
  '/terminal/portfolio',
  '/terminal/rewards',
  '/terminal/track',
  '/terminal/trenches',
  '/terminal/trending',
];
const warmDirectory = new URL('./warm/', import.meta.url);
for (const file of (await readdir(warmDirectory)).sort()) {
  warmPaths.push(...JSON.parse(await readFile(new URL(file, warmDirectory), 'utf8')));
}
await Promise.all([...new Set(warmPaths)].map(path => warmPage(path)));
pagesReady = true;
function close() {
  for (const socket of sockets.clients) socket.terminate();
  sockets.close(); server.close(); app.kill('SIGTERM');
}
process.on('SIGTERM', close);
process.on('SIGINT', close);
app.on('error', error => { console.error(error); close(); process.exitCode = 1; });
app.on('exit', code => { if (code) { close(); process.exitCode = code; } });

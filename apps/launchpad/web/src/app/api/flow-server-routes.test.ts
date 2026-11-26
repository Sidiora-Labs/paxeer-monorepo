import Sentry from '@sentry/nextjs';
import { NextRequest } from 'next/server';
import { createHash, randomUUID } from 'node:crypto';
import { execFile, spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { createWriteStream, existsSync } from 'node:fs';
import { copyFile, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer, request as httpRequest, type Server, type ServerResponse } from 'node:http';
import type { Socket } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { promisify } from 'node:util';
import { gunzipSync } from 'node:zlib';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { backendRewrites } from '@/core/next-build-config';
import { redactedErrorPayload } from '@/core/report-error';
import { GET as health } from './health/route';
import { POST as errorBeacon } from './errors/route';
import { GET as cardImage } from './og/pnl/[cardId]/route';

interface JoinedConfiguration {
  browser_origin: string;
  stream_application_origin: string;
  api_origin: string;
  rpc_url: string;
  processes: Array<{ role: string; ready_url: string; env?: Record<string, string> }>;
}
interface PublishedCard {
  origin: string;
  id: string;
  revokedId: string;
  imageSha256: string;
  imageBytes: number;
}

const web = process.cwd();
const root = resolve(web, '../../..');
const savedEnvironment = new Map<string, string | undefined>();
let joined: JoinedConfiguration;
let pool: string;
let pnl: ChildProcess | undefined;
let published: PublishedCard;
let directory: string;
let receiver: Server;
let acknowledge = true;
const pendingEnvelopes = new Set<ServerResponse>();
const receivedEvents: Array<Record<string, unknown>> = [];
let sentryClient: NonNullable<ReturnType<typeof Sentry.init>>;

function environment(name: string, value: string) {
  if (!savedEnvironment.has(name)) savedEnvironment.set(name, process.env[name]);
  process.env[name] = value;
}

const serverConnections = new WeakMap<Server, Set<Socket>>();

function trackConnection(server: Server, socket: Socket) {
  const connections = serverConnections.get(server) ?? new Set<Socket>();
  serverConnections.set(server, connections);
  connections.add(socket);
  socket.once('close', () => connections.delete(socket));
}

async function listen(server: Server) {
  server.on('connection', socket => trackConnection(server, socket));
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('TCP listener required');
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: Server) {
  for (const socket of serverConnections.get(server) ?? []) socket.destroy();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}

function requestBeacon(body: string) {
  return new NextRequest('http://localhost/api/errors', { method: 'POST', body });
}

function imageRequest(id: string, signal?: AbortSignal) {
  return cardImage(new NextRequest(`http://localhost/api/og/pnl/${encodeURIComponent(id)}`, { signal }), {
    params: Promise.resolve({ cardId: id }),
  });
}

async function assertImageError(response: Response, status: number, error: string) {
  expect(response.status).toBe(status);
  expect(response.headers.get('Cache-Control')).toBe('no-store');
  expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
  expect(await response.json()).toEqual({ error });
}

beforeAll(async () => {
  const configuration = process.env.JOINED_CONFIG_FILE;
  const chainFile = process.env.JOINED_CHAIN_FILE;
  if (!configuration || !chainFile) throw new Error('Joined services and chain are required');
  joined = JSON.parse(await readFile(configuration, 'utf8'));
  pool = JSON.parse(await readFile(chainFile, 'utf8')).pool.toLowerCase();
  const api = joined.processes.find(process => process.role === 'api');
  const gateway = joined.processes.find(process => process.role === 'gateway');
  const charts = joined.processes.find(process => process.role === 'trading-charts');
  if (!api?.env || !gateway?.env || !charts) throw new Error('API, gateway and charts are required');
  environment('NEXT_PUBLIC_RPC_URL', joined.rpc_url);
  environment('NEXT_PUBLIC_DATA_API_URL', joined.api_origin);
  environment('NEXT_PUBLIC_GATEWAY_URL', new URL(gateway.ready_url).origin);
  environment('NEXT_PUBLIC_METADATA_API_URL', new URL(gateway.ready_url).origin);
  environment('WEB_GATEWAY_UPSTREAM', new URL(gateway.ready_url).origin);
  environment('WEB_API_UPSTREAM', joined.api_origin);
  environment('WEB_SOCKETS_UPSTREAM', joined.api_origin);
  environment('WEB_CHARTS_UPSTREAM', new URL(charts.ready_url).origin);

  directory = await mkdtemp(join(tmpdir(), 'web-shares-'));
  const service = join(root, 'services/launchpad/pnl-tracker');
  const manifest = (await readFile(join(service, 'go.mod'), 'utf8'))
    .replace(/^module (\S+)$/m, 'module $1/web-shares')
    .replace(/=> \.\.\/\.\.\/\.\.\/sdk\/launchpad\//g, `=> ${root}/sdk/launchpad/`);
  await writeFile(join(directory, 'go.mod'), `${manifest}\nrequire github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker v0.0.0\nreplace github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker => ${service}\n`);
  await copyFile(join(service, 'go.sum'), join(directory, 'go.sum'));
  await writeFile(join(directory, 'main.go'), (await readFile(join(web, 'src/app/api/fixtures/pnl.go'), 'utf8')).replace(/^\/\/go:build ignore\n\n/, ''));
  const output = createWriteStream(join(directory, 'pnl.txt'));
  pnl = spawn('go', ['run', '-mod=mod', '.'], { cwd: directory, env: { ...process.env, GOWORK: 'off' }, stdio: ['pipe', 'pipe', 'pipe'], detached: true });
  pnl.stdout!.pipe(output, { end: false });
  pnl.stderr!.pipe(output, { end: false });
  pnl.once('close', () => output.end());
  pnl.stdin!.end(JSON.stringify({ database: api.env.DATABASE_URL, rpc: joined.rpc_url,
    identity: new URL(gateway.ready_url).origin, token: gateway.env.IDENTITY_SERVICE_TOKEN,
    fontDir: join(service, 'internal/ogrender/fonts'), output: directory }));
  const ready = join(directory, 'ready.json');
  const deadline = Date.now() + 120_000;
  while (!existsSync(ready)) {
    if (pnl.exitCode !== null || Date.now() > deadline) {
      throw new Error(`PnL service did not start: ${await readFile(join(directory, 'pnl.txt'), 'utf8')}`);
    }
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  published = JSON.parse(await readFile(ready, 'utf8'));
  environment('NEXT_PUBLIC_PNL_API_URL', published.origin);

  receiver = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const bytes = Buffer.concat(chunks);
    const raw = request.headers['content-encoding'] === 'gzip' ? gunzipSync(bytes) : bytes;
    const lines = raw.toString('utf8').split('\n');
    for (let i = 1; i + 1 < lines.length; i += 2) {
      if (JSON.parse(lines[i]).type === 'event') receivedEvents.push(JSON.parse(lines[i + 1]));
    }
    if (acknowledge) response.writeHead(200).end();
    else pendingEnvelopes.add(response);
  });
  const origin = await listen(receiver);
  const client = Sentry.init({ dsn: `${origin.replace('://', '://public@')}/1`,
    defaultIntegrations: false, sendClientReports: false });
  if (!client) throw new Error('Sentry client is required');
  sentryClient = client;
}, 150_000);

afterAll(async () => {
  acknowledge = true;
  for (const response of pendingEnvelopes) response.writeHead(200).end();
  pendingEnvelopes.clear();
  if (sentryClient) await sentryClient.close(2000);
  if (receiver) await close(receiver);
  if (pnl?.pid && pnl.exitCode === null) {
    process.kill(-pnl.pid, 'SIGCONT');
    process.kill(-pnl.pid, 'SIGTERM');
    await once(pnl, 'exit');
  }
  for (const [name, value] of savedEnvironment) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  if (directory) await rm(directory, { recursive: true });
});

async function healthRouteAnswersGoodRequests() {
  const response = await fetch(`${joined.browser_origin}/api/health`);
  expect(response.status).toBe(200);
  expect(response.headers.get('Cache-Control')).toBe('no-store');
  expect(await response.json()).toEqual({ status: 'ok' });
}

async function healthRouteRejectsUnsupportedMethods() {
  const response = await fetch(`${joined.browser_origin}/api/health`, { method: 'POST', body: '{}' });
  expect(response.status).toBe(405);
  expect(await response.text()).toBe('');
}

async function healthRouteIsIndependentOfUpstreams() {
  if (!pnl?.pid) throw new Error('Owned PnL process required');
  process.kill(-pnl.pid, 'SIGSTOP');
  try {
    const response = health();
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ status: 'ok' });
  } finally { process.kill(-pnl.pid, 'SIGCONT'); }
}

async function errorRouteDeliversAClientBeacon() {
  const payload = redactedErrorPayload(new TypeError('render failed'), { area: 'widget', code: 'render_failed' });
  const before = receivedEvents.length;
  const response = await errorBeacon(requestBeacon(JSON.stringify(payload)));
  expect(response.status).toBe(204);
  expect(await response.text()).toBe('');
  expect(await sentryClient.flush(2000)).toBe(true);
  expect(receivedEvents.slice(before)).toHaveLength(1);
  expect(receivedEvents[before]).toMatchObject({
    exception: { values: [{ type: 'Error', value: payload.message }] },
    tags: { source: 'client-beacon' }, extra: payload.context,
  });
}

async function errorRouteRejectsInvalidBeacons() {
  const before = receivedEvents.length;
  for (const body of ['{', 'null', '[]', '{}', '{"message":""}', '{"message":"event","stack":2}', '{"message":"event","context":[]}']) {
    const response = await errorBeacon(requestBeacon(body));
    expect(response.status).toBe(400);
    expect(await response.text()).toBe('');
  }
  const oversized = await errorBeacon(requestBeacon(JSON.stringify({ message: 'x'.repeat(64 * 1024) })));
  expect(oversized.status).toBe(413);
  expect(await sentryClient.flush(2000)).toBe(true);
  expect(receivedEvents).toHaveLength(before);
}

async function errorRouteFinishesWhenIngestionDoesNotAnswer() {
  acknowledge = false;
  try {
    const response = await errorBeacon(requestBeacon(JSON.stringify({ message: `Error: unavailable ${randomUUID()}` })));
    expect(response.status).toBe(204);
    expect(await response.text()).toBe('');
    expect(await sentryClient.flush(200)).toBe(false);
  } finally {
    acknowledge = true;
    for (const response of pendingEnvelopes) response.writeHead(200).end();
    pendingEnvelopes.clear();
    expect(await sentryClient.flush(2000)).toBe(true);
  }
}

async function cardRouteReturnsThePublishedServiceImage() {
  const response = await imageRequest(published.id);
  expect(response.status).toBe(200);
  expect(response.headers.get('Content-Type')).toBe('image/png');
  expect(response.headers.get('Cache-Control')).toBe('no-store');
  expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
  const image = Buffer.from(await response.arrayBuffer());
  expect(image.length).toBe(published.imageBytes);
  expect(createHash('sha256').update(image).digest('hex')).toBe(published.imageSha256);
  expect([...image.subarray(0, 8)]).toEqual([137, 80, 78, 71, 13, 10, 26, 10]);
  expect(image.readUInt32BE(16)).toBe(1200);
  expect(image.readUInt32BE(20)).toBe(630);
}

async function cardRouteRejectsBadMissingAndWithdrawnCards() {
  for (const id of ['invalid', '../auth', `${published.id}/image.png`, `${published.id}?wallet=other`]) {
    await assertImageError(await imageRequest(id), 404, 'Card not found');
  }
  await assertImageError(await imageRequest(randomUUID()), 404, 'Card unavailable');
  await assertImageError(await imageRequest(published.revokedId), 410, 'Card withdrawn or expired');
}

const processStatus = promisify(execFile);

async function suspendProcessGroup(pid: number) {
  process.kill(-pid, 'SIGSTOP');
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    const { stdout } = await processStatus('ps', ['-s', String(pid), '-L', '-o', 'stat='], { encoding: 'utf8', timeout: 1000 });
    const states = stdout.trim().split(/\s+/);
    if (states.length > 0 && states.every(state => state.startsWith('T'))) return;
    await new Promise(resolve => setTimeout(resolve, 5));
  }
  throw new Error('Stopped PnL service required');
}

async function cardRouteFinishesWhenTheServiceDoesNotAnswer() {
  if (!pnl?.pid) throw new Error('Owned PnL process required');
  try {
    await suspendProcessGroup(pnl.pid);
    const started = Date.now();
    await assertImageError(await imageRequest(published.id), 502, 'Card image service unavailable');
    expect(Date.now() - started).toBeLessThan(6500);
  } finally { process.kill(-pnl.pid, 'SIGCONT'); }
}

async function cardRouteFinishesWhenTheImageBodyStops() {
  const proxy = createServer((request, response) => {
    const upstream = httpRequest(new URL(request.url!, published.origin), source => {
      response.writeHead(source.statusCode!, source.headers);
      response.flushHeaders();
      source.pause();
    });
    upstream.on('socket', socket => trackConnection(proxy, socket));
    response.once('close', () => upstream.destroy());
    upstream.on('error', error => response.destroy(error));
    request.pipe(upstream);
  });
  const origin = await listen(proxy);
  environment('NEXT_PUBLIC_PNL_API_URL', origin);
  const started = Date.now();
  try {
    await assertImageError(await imageRequest(published.id), 502, 'Card image service unavailable');
    expect(Date.now() - started).toBeLessThan(6500);
  } finally {
    environment('NEXT_PUBLIC_PNL_API_URL', published.origin);
    await close(proxy);
  }
}

async function cardRouteFinishesWhenTheImageBodyIgnoresCancellation() {
  const originalFetch = globalThis.fetch;
  let cancelled = false;
  let timeout: ReturnType<typeof setTimeout> | undefined;
  const body = new ReadableStream<Uint8Array>({
    start(controller) { controller.enqueue(new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])); },
    cancel() {
      cancelled = true;
      return new Promise<void>(() => { });
    },
  });
  const started = Date.now();
  try {
    globalThis.fetch = async () => new Response(body, { headers: { 'Content-Type': 'image/png' } });
    const response = await Promise.race([
      imageRequest(published.id),
      new Promise<never>((_, reject) => {
        timeout = setTimeout(() => reject(new Error('Card image response exceeded 6500 ms')), 6500);
      }),
    ]);
    await assertImageError(response, 502, 'Card image service unavailable');
    expect(Date.now() - started).toBeLessThan(6500);
    expect(cancelled).toBe(true);
  } finally {
    clearTimeout(timeout);
    globalThis.fetch = originalFetch;
  }
}

async function cardRouteHonorsClientCancellation() {
  const cancellation = new AbortController();
  cancellation.abort();
  await assertImageError(await imageRequest(published.id, cancellation.signal), 502, 'Card image service unavailable');
}

async function rewriteTableReachesEachRegisteredServiceRoute() {
  const table = await backendRewrites(process.env);
  const { default: config } = await import('../../../next.config');
  if (typeof config === 'function' || !config.rewrites) throw new Error('Web rewrite table required');
  const configured = await config.rewrites();
  if (Array.isArray(configured) || !configured.beforeFiles) throw new Error('Ordered rewrite groups required');
  expect(configured.beforeFiles.slice(0, -table.beforeFiles.length)).toMatchObject([
    { source: '/monitoring(/?)', destination: 'https://o:orgid.ingest.:region.sentry.io/api/:projectid/envelope/?hsts=0',
      has: [{ type: 'query', key: 'o' }, { type: 'query', key: 'p' }, { type: 'query', key: 'r' }] },
    { source: '/monitoring(/?)', destination: 'https://o:orgid.ingest.sentry.io/api/:projectid/envelope/?hsts=0',
      has: [{ type: 'query', key: 'o' }, { type: 'query', key: 'p' }] },
  ]);
  expect(configured.beforeFiles.slice(-table.beforeFiles.length)).toEqual(table.beforeFiles);
  expect(configured.afterFiles).toEqual([]);
  expect(configured.fallback).toEqual([]);

  expect(table.beforeFiles.map(row => row.source)).toEqual(rewriteProbes().map(probe => probe.source));
}

function rewriteProbes() {
  const now = Date.now();
  const candles = new URLSearchParams({ interval: '1d', from: new Date(now - 3 * 86400_000).toISOString(), to: new Date(now + 60_000).toISOString(), limit: '50' });
  const history = new URLSearchParams({ symbol: pool, resolution: '1D', from: String(Math.floor(now / 1000) - 3 * 86400), to: String(Math.floor(now / 1000) + 60) });
  return [
    { source: '/v2/auth/:path*', upstream: 'WEB_GATEWAY_UPSTREAM', path: '/v2/auth/sessions', status: 401 },
    { source: '/v2/stream/tickets', upstream: 'WEB_GATEWAY_UPSTREAM', path: '/v2/stream/tickets', method: 'POST', status: 401 },
    { source: '/v2/control/:path*', upstream: 'WEB_GATEWAY_UPSTREAM', path: '/v2/control/caps/create', status: 404 },
    { source: '/v2/network/authority', upstream: 'WEB_API_UPSTREAM', path: '/v2/network/authority' },
    { source: '/v2/preflight/:path*', upstream: 'WEB_API_UPSTREAM', path: '/v2/preflight/create', method: 'POST', status: 403 },
    { source: '/v2/snapshots', upstream: 'WEB_API_UPSTREAM', path: `/v2/snapshots?topics=market.trades.${pool}`, status: 200 },
    { source: '/v2/pools/:pool/candles', upstream: 'WEB_API_UPSTREAM', path: `/v2/pools/${pool}/candles?${candles}`, status: 200 },
    { source: '/v2/stream', upstream: 'WEB_SOCKETS_UPSTREAM', path: '/v2/stream', status: 401 },
    { source: '/v2/stream/sse', upstream: 'WEB_SOCKETS_UPSTREAM', path: '/v2/stream/sse', status: 422 },
    { source: '/v2/udf/config', upstream: 'WEB_CHARTS_UPSTREAM', path: '/v2/udf/config', status: 200 },
    { source: '/v2/udf/symbols', upstream: 'WEB_CHARTS_UPSTREAM', path: `/v2/udf/symbols?symbol=${pool}`, status: 200 },
    { source: '/v2/udf/history', upstream: 'WEB_CHARTS_UPSTREAM', path: `/v2/udf/history?${history}`, status: 200 },
    { source: '/v2/udf/time', upstream: 'WEB_CHARTS_UPSTREAM', path: '/v2/udf/time', status: 200 },
  ];
}

async function rewriteReachesRegisteredServiceRoute(source: string) {
  const table = await backendRewrites(process.env);
  const probe = rewriteProbes().find(probe => probe.source === source);
  if (!probe) throw new Error(`Service probe required: ${source}`);
  const row = table.beforeFiles.find(row => row.source === probe.source);
  if (!row) throw new Error(`Rewrite required: ${probe.source}`);
  expect(row.destination, probe.source).toBe(`${process.env[probe.upstream]}${probe.source}`);
  const init = { method: probe.method ?? 'GET', headers: { Origin: joined.stream_application_origin, 'X-Schema-Version': '2', 'Content-Type': 'application/json' },
    ...(probe.method === 'POST' ? { body: '{}' } : {}), signal: AbortSignal.timeout(30_000) };
  const [direct, proxied] = await Promise.all([
    fetch(`${process.env[probe.upstream]}${probe.path}`, init),
    fetch(`${joined.browser_origin}${probe.path}`, { ...init, signal: AbortSignal.timeout(30_000) }),
  ]);
  if (probe.source === '/v2/network/authority') {
    expect(proxied.status, `${probe.source} web`).toBe(direct.status);
  } else {
    expect(direct.status, `${probe.source} service`).toBe(probe.status);
    expect(proxied.status, `${probe.source} web`).toBe(probe.status);
  }
  expect(proxied.headers.get('Content-Type'), probe.source).toBe(direct.headers.get('Content-Type'));
  const [directBody, webBody] = await Promise.all([direct.text(), proxied.text()]);
  expect(webBody.length, probe.source).toBeGreaterThan(0);
  if (direct.headers.get('Content-Type')?.includes('application/json')) {
    const original = JSON.parse(directBody), result = JSON.parse(webBody);
    expect(typeof result, probe.source).toBe(typeof original);
    if (typeof original === 'number') expect(Math.abs(original - result)).toBeLessThanOrEqual(5);
    else {
      expect(Object.keys(result).sort(), probe.source).toEqual(Object.keys(original).sort());
      if (direct.status >= 400) expect(result.error?.code ?? result.code, probe.source).toBe(original.error?.code ?? original.code);
      if (probe.source === '/v2/network/authority') {
        if (direct.status >= 400) {
          for (const [response, body] of [[direct, original], [proxied, result]] as const) {
            expect(body.requestId).toBe(response.headers.get('X-Request-ID'));
            expect(body.requestId).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
            expect(body.retryAfterMs).toBe(Number(response.headers.get('Retry-After')) * 1000);
          }
          expect({ ...result, requestId: original.requestId }).toEqual(original);
        } else expect(result).toEqual(original);
      }
      if (probe.source === '/v2/snapshots') expect(result.data.snapshot.topics[0].topic).toBe(`market.trades.${pool}`);
      if (probe.source === '/v2/udf/config') expect(result).toEqual(original);
      if (probe.source === '/v2/udf/symbols') expect(result.poolAddress).toBe(pool);
      if (probe.source === '/v2/udf/history') expect(result.t.length).toBeGreaterThan(0);
    }
  } else expect(webBody, probe.source).toBe(directBody);
}

async function rewriteTableRejectsUnusableUpstreamOrigins() {
  for (const name of ['WEB_GATEWAY_UPSTREAM', 'WEB_API_UPSTREAM', 'WEB_SOCKETS_UPSTREAM', 'WEB_CHARTS_UPSTREAM']) {
    for (const value of ['relative', 'ftp://localhost', 'http://localhost/prefix', 'http://user:password@localhost', 'http://localhost?query=1', 'http://localhost#fragment']) {
      await expect(backendRewrites({ ...process.env, [name]: value })).rejects.toThrow(`${name} must be an HTTP(S) origin`);
    }
  }
}

test(healthRouteAnswersGoodRequests, healthRouteAnswersGoodRequests, 35_000);
test(healthRouteRejectsUnsupportedMethods, healthRouteRejectsUnsupportedMethods);
test(healthRouteIsIndependentOfUpstreams, healthRouteIsIndependentOfUpstreams);
test(errorRouteDeliversAClientBeacon, errorRouteDeliversAClientBeacon);
test(errorRouteRejectsInvalidBeacons, errorRouteRejectsInvalidBeacons);
test(errorRouteFinishesWhenIngestionDoesNotAnswer, errorRouteFinishesWhenIngestionDoesNotAnswer);
test(cardRouteReturnsThePublishedServiceImage, cardRouteReturnsThePublishedServiceImage);
test(cardRouteRejectsBadMissingAndWithdrawnCards, cardRouteRejectsBadMissingAndWithdrawnCards);
test(cardRouteFinishesWhenTheServiceDoesNotAnswer, cardRouteFinishesWhenTheServiceDoesNotAnswer, 15_000);
test(cardRouteFinishesWhenTheImageBodyStops, cardRouteFinishesWhenTheImageBodyStops, 15_000);
test(cardRouteFinishesWhenTheImageBodyIgnoresCancellation, cardRouteFinishesWhenTheImageBodyIgnoresCancellation, 15_000);
test(cardRouteHonorsClientCancellation, cardRouteHonorsClientCancellation);
test(rewriteTableReachesEachRegisteredServiceRoute, rewriteTableReachesEachRegisteredServiceRoute, 120_000);
test.each(rewriteProbes().map(probe => probe.source))('rewriteReachesRegisteredServiceRoute %s', rewriteReachesRegisteredServiceRoute, 35_000);
test(rewriteTableRejectsUnusableUpstreamOrigins, rewriteTableRejectsUnusableUpstreamOrigins);

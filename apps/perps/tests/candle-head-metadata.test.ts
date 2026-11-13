import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer, type Server } from 'node:http';
import { createServer as createHttpsServer } from 'node:https';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { promisify } from 'node:util';
import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import handler from '../src/pages/api/candle/[...path]';

// Repair record 22.31.7: f004 is proved by candleHeadPreservesUpstreamMetadata.
// Baseline was not run: the bounded gate order requires implementation first.
// No out-of-path wiring is required; hf-done records the qualified revision.
const execFileAsync = promisify(execFile);
const payload = { s: 'ok', t: [1], c: [10] };
const wireBody = JSON.stringify(payload);
const cases = [
  { endpoint: 'history', status: 200 },
  { endpoint: 'config', status: 202 },
  { endpoint: 'symbols', status: 404 },
  { endpoint: 'time', status: 503 },
];
const origin = 'https://perps.test.example';

async function listen(server: Server): Promise<string> {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  return `127.0.0.1:${(server.address() as AddressInfo).port}`;
}

async function close(server: Server): Promise<void> {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
  });
}

async function candleHeadClient(): Promise<void> {
  const relay = createServer((req, res) => {
    const url = new URL(req.url!, 'http://localhost');
    void apiResolver(req, res, {
      ...Object.fromEntries(url.searchParams),
      path: url.pathname.slice('/api/candle/'.length).split('/'),
    }, { default: handler }, {
      dev: false,
      previewModeId: '',
      previewModeEncryptionKey: '',
      previewModeSigningKey: '',
    }, false);
  });
  const address = await listen(relay);
  try {
    for (const { endpoint, status } of cases) {
      const response = await fetch(`http://${address}/api/candle/${endpoint}?symbol=PERP_PAX_USDC`, { method: 'HEAD' });
      assert.equal(response.status, status, `HEAD ${endpoint} must preserve upstream status`);
      assert.equal(response.headers.get('content-type'), 'application/json; charset=utf-8');
      assert.equal(response.headers.get('content-length'), String(Buffer.byteLength(wireBody)));
      assert.equal(response.headers.get('cache-control'), 'public, max-age=17');
      assert.equal(response.headers.get('etag'), '"candle-metadata"');
      assert.equal(response.headers.get('last-modified'), 'Wed, 21 Oct 2015 07:28:00 GMT');
      assert.equal(response.headers.get('x-candle-upstream'), endpoint);
      assert.equal(response.headers.get('x-connection-only'), null);
      assert.equal(await response.text(), '', 'HEAD must have no response body');
    }

    const response = await fetch(`http://${address}/api/candle/history?symbol=PERP_PAX_USDC`);
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), payload);
    assert.equal(response.headers.get('cache-control'), 'public, s-maxage=2, stale-while-revalidate=5');
  } finally {
    await close(relay);
  }
}

async function candleHeadPreservesUpstreamMetadata(): Promise<void> {
  const directory = await mkdtemp(join(tmpdir(), 'candle-head-'));
  const keyPath = join(directory, 'key.pem');
  const certPath = join(directory, 'cert.pem');
  let upstream: Server | undefined;
  const requests: { method: string | undefined; path: string | undefined; origin: string | undefined; accept: string | undefined }[] = [];
  try {
    await execFileAsync('openssl', [
      'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
      '-subj', '/CN=localhost', '-addext', 'subjectAltName=IP:127.0.0.1',
      '-keyout', keyPath, '-out', certPath,
    ]);
    upstream = createHttpsServer({
      key: await readFile(keyPath),
      cert: await readFile(certPath),
    }, (req, res) => {
      requests.push({ method: req.method, path: req.url, origin: req.headers.origin, accept: req.headers.accept });
      const endpoint = req.url?.split('?')[0].split('/').at(-1);
      const selected = cases.find(entry => entry.endpoint === endpoint);
      assert.ok(selected, `Unexpected upstream path: ${req.url}`);
      res.writeHead(selected.status, {
        'Content-Type': 'application/json; charset=utf-8',
        'Content-Length': Buffer.byteLength(wireBody),
        'Cache-Control': 'public, max-age=17',
        ETag: '"candle-metadata"',
        'Last-Modified': 'Wed, 21 Oct 2015 07:28:00 GMT',
        'X-Candle-Upstream': selected.endpoint,
        Connection: 'keep-alive, x-connection-only',
        'X-Connection-Only': 'upstream-hop',
      });
      // Node's real HTTP server suppresses these representation bytes for HEAD.
      res.end(wireBody);
    });
    const address = await listen(upstream);
    await execFileAsync(process.execPath, ['--import', 'tsx', __filename, '--candle-head-client'], {
      env: {
        ...process.env,
        NODE_EXTRA_CA_CERTS: certPath,
        MARKET_PROXY_HOST: address,
        MARKET_PROXY_ORIGIN: origin,
      },
      timeout: 30_000,
    });
    assert.deepEqual(requests, [
      ...cases.map(({ endpoint }) => ({ method: 'HEAD', path: `/api/pax/${endpoint}?symbol=PAX`, origin, accept: 'application/json' })),
      { method: 'GET', path: '/api/pax/history?symbol=PAX', origin, accept: 'application/json' },
    ]);
  } finally {
    if (upstream) await close(upstream);
    await rm(directory, { recursive: true, force: true });
  }
}

if (process.argv.includes('--candle-head-client')) {
  candleHeadClient().catch(error => {
    console.error(error);
    process.exitCode = 1;
  });
} else {
  test(candleHeadPreservesUpstreamMetadata);
}

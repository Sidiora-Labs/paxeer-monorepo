import { createServer, request as httpRequest } from 'node:http';
import { readFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const directory = dirname(fileURLToPath(import.meta.url));
const manifest = await readFile(join(directory, 'manifest.json'));
const proof = JSON.parse(await readFile(join(directory, 'proof.json'), 'utf8'));
const index = await readFile(join(directory, 'index.html'));
const stylesheet = await readFile(join(directory, 'style.css'));
const objectByDigest = new Map([
  [proof.digest.slice(2), manifest],
  ['c66cfeb545835d7c9aec762da213dd4bc7727a416372ab31c3ce1a7f4fe29306', index],
  ['72f1c620e46737a4e387d2ac194e3beaac047e190c39dbad907733785240b44c', stylesheet],
]);

function json(response, value, status = 200) {
  const body = Buffer.from(JSON.stringify(value));
  response.writeHead(status, { 'content-type': 'application/json', 'content-length': body.length });
  response.end(body);
}

const fixture = createServer(async (request, response) => {
  const url = new URL(request.url, 'http://fixture');
  if (request.method === 'GET' && url.pathname === '/commit') {
    json(response, { jsonrpc: '2.0', id: 1, result: { signed_header: { header: { height: '9', app_hash: proof.appHash.slice(2).toUpperCase() } } } });
    return;
  }
  if (request.method === 'GET' && url.pathname.startsWith('/sites/manifests/')) {
    response.writeHead(200, { 'content-type': 'application/json', 'content-length': manifest.length });
    response.end(manifest);
    return;
  }
  if (request.method === 'GET' && url.pathname.startsWith('/sites/objects/')) {
    const body = objectByDigest.get(url.pathname.slice('/sites/objects/'.length));
    if (!body) { response.writeHead(404).end(); return; }
    response.writeHead(200, { 'content-type': 'application/octet-stream', 'content-length': body.length });
    response.end(body);
    return;
  }
  if (request.method === 'POST' && url.pathname === '/rpc') {
    let body = '';
    for await (const chunk of request) body += chunk;
    const payload = JSON.parse(body);
    if (payload.method === 'eth_blockNumber') {
      json(response, { jsonrpc: '2.0', id: payload.id, result: '0x8' });
      return;
    }
    if (payload.method === 'eth_getProof') {
      json(response, {
        jsonrpc: '2.0',
        id: payload.id,
        result: {
          address: proof.address,
          hexValues: [proof.digest],
          storageProof: [{ ops: proof.proofOps.map((op) => ({ type: op.type, key: Buffer.from(op.key.slice(2), 'hex').toString('base64'), data: Buffer.from(op.data.slice(2), 'hex').toString('base64') })) }],
        },
      });
      return;
    }
    json(response, { jsonrpc: '2.0', id: payload.id, error: { code: -32601, message: 'Method not found' } }, 404);
    return;
  }
  response.writeHead(404).end();
});

await new Promise((resolve) => fixture.listen(0, '127.0.0.1', resolve));
const fixturePort = fixture.address().port;
const next = spawn(join(directory, '../../../../apps/names/node_modules/.bin/next'), ['start', '--hostname', '127.0.0.1', '--port', '4300'], {
  cwd: join(directory, '../../../../apps/names'),
  env: {
    ...process.env,
    NAMES_SHELL_DOMAIN: 'names.test',
    NAMES_RPC_URL: `http://127.0.0.1:${fixturePort}/rpc`,
    NAMES_RESOLVER: proof.address,
    NAMES_CONSENSUS_RPC_URL: `http://127.0.0.1:${fixturePort}`,
    NAMES_BUCKET_BASE_URL: `http://127.0.0.1:${fixturePort}/sites`,
  },
  stdio: ['ignore', 'pipe', 'pipe'],
});
const proxy = createServer(async (request, response) => {
  const incoming = new URL(request.url, 'http://surface');
  const published = incoming.pathname === '/published' || incoming.pathname.startsWith('/published/') || (incoming.pathname.startsWith('/_next/') ? false : incoming.pathname !== '/');
  const targetPath = incoming.pathname === '/published' ? '/' : incoming.pathname.startsWith('/published/') ? incoming.pathname.slice('/published'.length) || '/' : incoming.pathname;
  const headers = new Headers(request.headers);
  headers.set('host', published ? 'example.pax.names.test' : 'names.test');
  headers.set('accept-encoding', 'identity');
  const upstream = httpRequest({ hostname: '127.0.0.1', port: 4300, path: `${targetPath}${incoming.search}`, method: request.method, headers: Object.fromEntries(headers) }, (upstreamResponse) => {
    const responseHeaders = { ...upstreamResponse.headers };
    delete responseHeaders['content-encoding'];
    delete responseHeaders['content-length'];
    delete responseHeaders['transfer-encoding'];
    response.writeHead(upstreamResponse.statusCode ?? 502, responseHeaders);
    upstreamResponse.pipe(response);
  });
  upstream.on('error', () => response.writeHead(502).end());
  if (['GET', 'HEAD'].includes(request.method)) upstream.end();
  else request.pipe(upstream);
});
await new Promise((resolve) => proxy.listen(4173, '127.0.0.1', resolve));

async function close() {
  proxy.closeAllConnections();
  fixture.closeAllConnections();
  await Promise.all([
    new Promise((resolve) => proxy.close(resolve)),
    new Promise((resolve) => fixture.close(resolve)),
  ]);
  next.kill('SIGTERM');
}
process.once('SIGTERM', () => void close().finally(() => process.exit(0)));
process.once('SIGINT', () => void close().finally(() => process.exit(0)));

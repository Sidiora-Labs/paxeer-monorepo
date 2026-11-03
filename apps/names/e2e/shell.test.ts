import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test, { type TestContext } from 'node:test';
import {
  parseManifest,
  ProofError,
  resolveContent as sdkResolveContent,
  tendermintHeaders as sdkTendermintHeaders,
} from '@paxeer/names';
import { NextRequest } from 'next/server.js';
import { proxy } from '../src/proxy.ts';
import { publish } from '../../../tools/names/src/publish.ts';
import {
  privateKey,
  resolver,
  startRpc,
  startS3,
  type StorageProofVector,
} from '../../../tools/names/test/doubles.ts';
import { nameForHost, serve, type ServeDependencies } from '../src/lib/serve.ts';

const resolveContent = sdkResolveContent;
const tendermintHeaders = sdkTendermintHeaders;

const index =
  '<!doctype html><link rel="stylesheet" href="/style.css"><h1>Published Paxeer site</h1>';
const stylesheet = 'body { color: navy; }';
const expectedCsp =
  "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'";

async function fixture(t: TestContext) {
  const vector: StorageProofVector = JSON.parse(
    await readFile(
      new URL('../../../tests/vectors/names/storage_proof_v1.json', import.meta.url),
      'utf8'
    )
  );
  const s3 = await startS3(t);
  const rpc = await startRpc(t, vector);
  const dir = await mkdtemp(join(tmpdir(), 'pax-shell-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await writeFile(join(dir, 'index.html'), index);
  await writeFile(join(dir, 'style.css'), stylesheet);
  const published = await publish({
    dir,
    name: 'example.pax',
    bucket: {
      endpoint: s3.endpoint,
      region: 'us-east-1',
      bucket: 'sites',
      publicBaseUrl: `${s3.endpoint}/sites`,
      credentials: { accessKeyId: 'test-access', secretAccessKey: 'test-secret' },
    },
    rpcUrl: rpc.rpcUrl,
    resolver,
    privateKey,
  });
  assert.equal(published.uploaded, 2);
  assert.equal(rpc.transactions.length, 1);
  const headers = tendermintHeaders(rpc.rpcUrl);
  const proven = await resolveContent(vector.name, {
    rpcUrl: rpc.rpcUrl,
    resolver: vector.address,
    headers,
  });
  assert.deepEqual(proven, {
    digest: vector.value,
    height: BigInt(vector.height),
    verified: true,
  });
  assert.deepEqual(rpc.headerHeights, [BigInt(vector.height) + 1n]);
  assert.ok(rpc.calls.includes('eth_getProof'));
  const proofResponse = await fetch(rpc.rpcUrl, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 3,
      method: 'eth_getProof',
      params: [vector.address, [vector.slot], `0x${vector.height.toString(16)}`],
    }),
  });
  assert.equal(proofResponse.status, 200);
  const proof = await proofResponse.json();
  assert.equal(proof.jsonrpc, '2.0');
  assert.equal(proof.id, 3);
  assert.deepEqual(Object.keys(proof.result).sort(), [
    'address',
    'hexValues',
    'storageProof',
  ]);
  assert.equal(proof.result.address, vector.address);
  assert.deepEqual(proof.result.hexValues, [vector.value]);
  for (const tag of [
    'latest',
    'pending',
    String(vector.height),
    `0x${(vector.height + 1).toString(16)}`,
    null,
  ]) {
    const response = await fetch(rpc.rpcUrl, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: 4,
        method: 'eth_getProof',
        params: [vector.address, [vector.slot], tag],
      }),
    });
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), {
      jsonrpc: '2.0',
      id: 4,
      error: { code: -32602, message: 'Expected the explicit proof height tag' },
    });
  }
  const commitResponse = await fetch(
    `${rpc.rpcUrl}/commit?height=${BigInt(vector.height) + 1n}`
  );
  assert.equal(commitResponse.status, 200);
  const commit = await commitResponse.json();
  assert.equal(
    commit.result.signed_header.header.height,
    String(BigInt(vector.height) + 1n)
  );
  assert.match(commit.result.signed_header.header.app_hash, /^[0-9A-F]{64}$/);
  assert.equal(
    commit.result.signed_header.header.app_hash,
    vector.app_hash.slice(2).toUpperCase()
  );
  assert.ok(proven);
  assert.notEqual(published.digest, proven.digest);
  const resolvedNames: string[] = [];
  // The committed proof proves only its fixed vector value. Exercise publication and serving
  // with the published digest separately; never claim that proof covers this new digest.
  const deps: ServeDependencies = {
    async resolve(name) {
      resolvedNames.push(name);
      return { digest: published.digest, height: proven.height, verified: proven.verified };
    },
    bucketBaseUrl: `${s3.endpoint}/sites`,
    fetch: globalThis.fetch,
  };
  const manifestKey = `manifests/${published.digest.slice(2)}.json`;
  const manifestBytes = s3.stored.get(manifestKey);
  assert.ok(manifestBytes);
  const manifest = parseManifest(manifestBytes);
  assert.deepEqual(Object.keys(manifest.entries).sort(), ['/index.html', '/style.css']);
  return {
    s3,
    rpc,
    vector,
    headers,
    proven,
    published,
    deps,
    resolvedNames,
    manifest,
    manifestKey,
    manifestBytes,
  };
}

async function shell_serves_a_published_site_after_proof_verified_resolution(t: TestContext) {
  const f = await fixture(t);
  assert.equal(nameForHost('example.pax.names.test', 'names.test'), 'example.pax');
  assert.equal(nameForHost('other.pax.names.test', 'names.test'), 'other.pax');
  for (const host of [
    'names.test',
    'example.pax.evil.test',
    'example.pax.names.test.evil',
    'nested.example.pax.names.test',
  ]) {
    assert.equal(nameForHost(host, 'names.test'), null);
  }
  f.s3.requests.length = 0;
  const response = await serve(new Request('https://example.pax.names.test/'), f.deps);
  assert.equal(response.status, 200);
  assert.equal(await response.text(), index);
  assert.equal(response.headers.get('Content-Type'), 'text/html');
  assert.equal(response.headers.get('Content-Security-Policy'), expectedCsp);
  assert.equal(response.headers.get('Cache-Control'), 'public, max-age=60');
  assert.equal(response.headers.get('X-Content-Type-Options'), 'nosniff');
  assert.equal(response.headers.get('X-PNS-Name'), 'example.pax');
  assert.equal(response.headers.get('X-PNS-Manifest'), f.published.digest);
  assert.equal(response.headers.get('X-PNS-Height'), f.proven.height.toString());
  const css = await serve(
    new Request('https://example.pax.names.test/style.css?version=1'),
    f.deps
  );
  assert.equal(css.status, 200);
  assert.equal(css.headers.get('Content-Type'), 'text/css');
  assert.equal(await css.text(), stylesheet);
  assert.deepEqual(f.resolvedNames, ['example.pax', 'example.pax']);
  assert.deepEqual(
    f.s3.requests.map(({ method, key }) => ({ method, key })),
    [
      { method: 'GET', key: f.manifestKey },
      { method: 'GET', key: `objects/${f.manifest.entries['/index.html']!.sha256}` },
      { method: 'GET', key: f.manifestKey },
      { method: 'GET', key: `objects/${f.manifest.entries['/style.css']!.sha256}` },
    ]
  );
  const head = await serve(
    new Request('https://example.pax.names.test/', { method: 'HEAD' }),
    f.deps
  );
  assert.equal(head.status, 200);
  assert.equal(await head.text(), '');
  assert.equal(head.headers.get('Content-Length'), String(Buffer.byteLength(index)));
  const missing = await serve(new Request('https://example.pax.names.test/missing'), f.deps);
  assert.equal(missing.status, 404);
  assert.deepEqual(await missing.json(), { code: 'site_path_not_found' });
  const noSite = await serve(new Request('https://example.pax.names.test/'), {
    ...f.deps,
    resolve: async () => null,
  });
  assert.equal(noSite.status, 404);
  assert.deepEqual(await noSite.json(), { code: 'name_has_no_site' });

  const environment = {
    NAMES_SHELL_DOMAIN: 'names.test',
    NAMES_RPC_URL: f.rpc.rpcUrl,
    NAMES_RESOLVER: f.vector.address,
    NAMES_CONSENSUS_RPC_URL: f.rpc.rpcUrl,
    NAMES_BUCKET_BASE_URL: f.deps.bucketBaseUrl,
  };
  const previous = Object.fromEntries(
    Object.keys(environment).map((key) => [key, process.env[key]])
  );
  Object.assign(process.env, environment);
  try {
    const passthrough = await proxy(
      new NextRequest('https://names.test/', { headers: { host: 'names.test' } })
    );
    assert.equal(passthrough.headers.get('x-middleware-next'), '1');
    // The real adapter resolves the fixed vector digest and must not serve the new site's bytes.
    f.s3.requests.length = 0;
    const routed = await proxy(
      new NextRequest('https://example.pax.names.test/', {
        headers: { host: 'example.pax.names.test' },
      })
    );
    assert.equal(routed.status, 502);
    assert.deepEqual(await routed.json(), { code: 'content_unavailable' });
    assert.deepEqual(
      f.s3.requests.map(({ key }) => key),
      [`manifests/${f.vector.value.slice(2)}.json`]
    );
  } finally {
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}

async function shell_refuses_an_object_whose_digest_differs_from_the_manifest(t: TestContext) {
  const f = await fixture(t);
  const entry = f.manifest.entries['/index.html']!;
  const corrupt = Buffer.alloc(entry.size, 0);
  assert.equal(corrupt.length, Buffer.byteLength(index));
  f.s3.stored.set(`objects/${entry.sha256}`, corrupt);
  const response = await serve(new Request('https://example.pax.names.test/'), f.deps);
  assert.equal(response.status, 502);
  assert.deepEqual(await response.json(), { code: 'content_digest_mismatch' });
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.equal(response.headers.get('X-PNS-Manifest'), null);
  assert.equal(response.headers.get('Content-Security-Policy'), expectedCsp);

  f.s3.stored.set(f.manifestKey, Buffer.from('corrupted manifest'));
  f.s3.requests.length = 0;
  const badManifest = await serve(new Request('https://example.pax.names.test/'), f.deps);
  assert.equal(badManifest.status, 502);
  assert.deepEqual(await badManifest.json(), { code: 'content_digest_mismatch' });
  assert.deepEqual(
    f.s3.requests.map(({ key }) => key),
    [f.manifestKey]
  );

  f.rpc.setProofValue(f.published.digest);
  const options = { rpcUrl: f.rpc.rpcUrl, resolver: f.vector.address, headers: f.headers };
  await assert.rejects(resolveContent(f.vector.name, options), ProofError);
  f.s3.requests.length = 0;
  const failedProof = await serve(new Request('https://example.pax.names.test/'), {
    ...f.deps,
    resolve: (name) => resolveContent(name, options),
  });
  assert.equal(failedProof.status, 502);
  assert.deepEqual(await failedProof.json(), { code: 'resolution_failed' });
  assert.equal(f.s3.requests.length, 0);
}

async function shell_answers_invalid_site_path_for_a_malformed_escape(t: TestContext) {
  const f = await fixture(t);
  const environment = {
    NAMES_SHELL_DOMAIN: 'names.test',
    NAMES_RPC_URL: f.rpc.rpcUrl,
    NAMES_RESOLVER: f.vector.address,
    NAMES_CONSENSUS_RPC_URL: f.rpc.rpcUrl,
    NAMES_BUCKET_BASE_URL: f.deps.bucketBaseUrl,
  };
  const previous = Object.fromEntries(
    Object.keys(environment).map((key) => [key, process.env[key]])
  );
  Object.assign(process.env, environment);
  try {
    f.rpc.calls.length = 0;
    f.s3.requests.length = 0;
    const response = await proxy(
      new NextRequest('https://example.pax.names.test/%E0%A4%A', {
        headers: { host: 'example.pax.names.test' },
      })
    );
    assert.equal(response.status, 400);
    assert.deepEqual(await response.json(), { code: 'invalid_site_path' });
    assert.equal(response.headers.get('Cache-Control'), 'no-store');
    assert.equal(response.headers.get('Content-Security-Policy'), expectedCsp);
    assert.equal(response.headers.get('X-Content-Type-Options'), 'nosniff');
    assert.deepEqual(f.rpc.calls, []);
    assert.deepEqual(f.s3.requests, []);
  } finally {
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}

test(shell_serves_a_published_site_after_proof_verified_resolution);
test(shell_refuses_an_object_whose_digest_differs_from_the_manifest);
test(shell_answers_invalid_site_path_for_a_malformed_escape);

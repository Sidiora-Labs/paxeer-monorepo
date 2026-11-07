import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test, { type TestContext } from 'node:test';
import {
  canonicalManifest,
  encodeContenthash,
  manifestDigest,
  namehash,
  parseManifest,
} from '@paxeer/names';
import { encodeFunctionData } from 'viem';
import { abi, privateKey, resolver, startRpc, startS3 } from './doubles.ts';
import { immutableCacheControl, publish } from '../src/publish.ts';
import { verify } from '../src/verify.ts';

async function fixture(t: TestContext) {
  const dir = await mkdtemp(join(tmpdir(), 'paxnames-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await mkdir(join(dir, 'assets'));
  await writeFile(join(dir, 'index.html'), '<!doctype html><h1>Paxeer</h1>');
  await writeFile(join(dir, 'assets/site.css'), 'body{color:blue}');
  const s3 = await startS3(t);
  const rpc = await startRpc(t);
  const { endpoint } = s3;
  const { rpcUrl } = rpc;
  const options = {
    dir,
    name: 'example.pax',
    bucket: {
      endpoint,
      region: 'us-east-1',
      bucket: 'sites',
      publicBaseUrl: `${endpoint}/sites`,
      credentials: { accessKeyId: 'test-access', secretAccessKey: 'test-secret' },
    },
    rpcUrl,
    resolver,
    privateKey,
  };
  const verifyOptions = {
    name: options.name,
    rpcUrl,
    resolver,
    publicBaseUrl: options.bucket.publicBaseUrl,
  };
  return { dir, ...s3, ...rpc, options, verifyOptions };
}

async function publish_uploads_only_missing_objects_and_writes_the_manifest(t: TestContext) {
  const f = await fixture(t);
  const result = await publish(f.options);
  assert.equal(result.uploaded, 2);
  assert.equal(result.skipped, 0);
  const manifestKey = `manifests/${result.digest.slice(2)}.json`;
  const manifestBytes = f.stored.get(manifestKey);
  assert.ok(manifestBytes);
  assert.equal(createHash('sha256').update(manifestBytes).digest('hex'), result.digest.slice(2));
  const manifest = parseManifest(manifestBytes);
  assert.equal(manifest.name, 'example.pax');
  assert.equal(manifest.version, 1);
  assert.equal(manifestDigest(manifest), result.digest);
  assert.equal(canonicalManifest(manifest), manifestBytes.toString());
  assert.deepEqual(Object.keys(manifest.entries).sort(), ['/assets/site.css', '/index.html']);
  for (const [path, type] of [
    ['/assets/site.css', 'text/css'],
    ['/index.html', 'text/html'],
  ]) {
    const bytes = await readFile(join(f.dir, path!));
    const sha256 = createHash('sha256').update(bytes).digest('hex');
    assert.deepEqual(manifest.entries[path!], { sha256, size: bytes.length, type });
    assert.deepEqual(f.stored.get(`objects/${sha256}`), bytes);
  }
  assert.equal(f.stored.size, 3);
  for (const [index, request] of f.requests.entries()) {
    if (request.method !== 'PUT') continue;
    assert.equal(request.conditional, '*');
    assert.equal(request.cache, immutableCacheControl);
    assert.ok(
      f.requests
        .slice(0, index)
        .some((prior) => prior.key === request.key && prior.method === 'HEAD')
    );
    const expectedType =
      request.key === manifestKey
        ? 'application/json'
        : Object.values(manifest.entries).find(
            (entry) => request.key === `objects/${entry.sha256}`
          )!.type;
    assert.equal(request.type, expectedType);
  }
  f.requests.length = 0;
  const second = await publish(f.options);
  assert.equal(second.digest, result.digest);
  assert.equal(second.uploaded, 0);
  assert.equal(second.skipped, 2);
  assert.equal(f.requests.filter((request) => request.method === 'HEAD').length, 3);
  assert.equal(f.requests.filter((request) => request.method === 'PUT').length, 0);
  assert.equal(f.transactions.length, 2);

  const html = manifest.entries['/index.html']!;
  f.stored.delete(`objects/${html.sha256}`);
  f.raceKeys.add(`objects/${html.sha256}`);
  const concurrent = await publish(f.options);
  assert.equal(concurrent.uploaded, 0);
  assert.equal(concurrent.skipped, 2);
  assert.deepEqual(await verify(f.verifyOptions), []);
  f.setHeadStatus(403);
  await assert.rejects(publish(f.options));
  assert.equal(f.transactions.length, 3);
}

async function publish_sets_the_contenthash_to_the_manifest_digest(t: TestContext) {
  const f = await fixture(t);
  const result = await publish(f.options);
  assert.equal(f.transactions.length, 1);
  const transaction = f.transactions[0]!;
  assert.equal(transaction.to.toLowerCase(), resolver.toLowerCase());
  assert.equal(transaction.hash, result.txHash);
  assert.equal(
    transaction.data,
    encodeFunctionData({
      abi,
      functionName: 'setContenthash',
      args: [namehash('example.pax'), `0x70617831${result.digest.slice(2)}`],
    })
  );
  assert.equal(f.records.get(namehash('example.pax')), encodeContenthash(result.digest));
  assert.ok(f.calls.includes('eth_getTransactionReceipt'));
  f.setReverted();
  await assert.rejects(publish(f.options), /Contenthash transaction reverted/);
}

async function verify_refuses_an_object_whose_hash_differs(t: TestContext) {
  const f = await fixture(t);
  await assert.rejects(verify(f.verifyOptions), /no supported contenthash/);
  const result = await publish(f.options);
  assert.deepEqual(await verify(f.verifyOptions), []);
  const manifestKey = `manifests/${result.digest.slice(2)}.json`;
  const originalManifest = f.stored.get(manifestKey)!;
  const manifest = parseManifest(originalManifest);
  const entry = manifest.entries['/index.html']!;
  const corrupt = Buffer.alloc(entry.size, 0);
  assert.equal(corrupt.length, entry.size);
  f.stored.set(`objects/${entry.sha256}`, corrupt);
  assert.deepEqual(await verify(f.verifyOptions), [
    {
      path: '/index.html',
      expected: entry.sha256,
      actual: createHash('sha256').update(corrupt).digest('hex'),
    },
  ]);
  f.stored.delete(`objects/${entry.sha256}`);
  assert.deepEqual(await verify(f.verifyOptions), [
    { path: '/index.html', expected: entry.sha256, actual: null },
  ]);
  f.stored.set(manifestKey, Buffer.from('invalid JSON'));
  const start = f.requests.length;
  assert.deepEqual(await verify(f.verifyOptions), [
    {
      path: manifestKey,
      expected: result.digest.slice(2),
      actual: createHash('sha256').update('invalid JSON').digest('hex'),
    },
  ]);
  assert.deepEqual(
    f.requests.slice(start).map((request) => request.key),
    [manifestKey]
  );
}

test(publish_uploads_only_missing_objects_and_writes_the_manifest);
test(publish_sets_the_contenthash_to_the_manifest_digest);
test(verify_refuses_an_object_whose_hash_differs);

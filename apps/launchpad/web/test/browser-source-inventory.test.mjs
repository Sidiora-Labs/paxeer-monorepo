import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import {
  inventoryBrowserSources,
  readBrowserManifest,
} from '../src/page-modules/live/production-measurement.mjs';

const sourceRoot = fileURLToPath(new URL('../../../../', import.meta.url));
const sourcePaths = [
  'apps/launchpad/web/src/page-modules/live/production-measurement.mjs',
  'apps/launchpad/web/src/widgets/trading/live-stream.tsx',
  'apps/launchpad/web/src/page-modules/live/index.tsx',
  'apps/launchpad/web/src/core/clients/livestream-api.ts',
];
const sha256 = raw => createHash('sha256').update(raw).digest('hex');

async function browserManifestFixture(t) {
  const root = await mkdtemp(join(tmpdir(), 'browser-source-inventory-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  async function artifact(name, value) {
    const path = join(root, name);
    const raw = JSON.stringify(value);
    await writeFile(path, raw);
    return { path, sha256: sha256(raw) };
  }
  const identity = {
    runId: 'source-inventory-qualification',
    candidate: '1'.repeat(64),
    interface: '2'.repeat(64),
    topology: '3'.repeat(64),
    generator: '4'.repeat(64),
  };
  const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
  const streamId = '11111111-1111-4111-8111-111111111111';
  const authority = await artifact('authority.json', { ...identity, lane: 'L40', expiresAt });
  const license = await artifact('license.json', {
    streamId, rightsReference: 'recorded-license-admission', validUntil: expiresAt,
    regions: ['qualification-region'],
  });
  const networkProfile = await artifact('network-profile.json', {
    authorityReference: 'recorded-network-admission', region: 'qualification-region',
    downstreamBitsPerSecond: 10000000, latencyMS: 40,
  });
  // Runtime references remain unopened during source admission; this gate does not run playback.
  const config = {
    version: 1, ...identity, sourceRoot,
    sourceFiles: await inventoryBrowserSources(sourceRoot), outputRoot: root, authority,
    applicationOrigin: 'https://reviewed.example',
    poolPage: 'https://reviewed.example/pool/1',
    directoryPage: 'https://reviewed.example/live', streamId, revision: '7',
    license, networkProfile,
    playwrightPackage: { path: join(root, 'runtime/playwright/package.json'), sha256: '5'.repeat(64) },
    playwrightVersion: '1.51.1',
    axeSource: { path: join(root, 'runtime/axe.min.js'), sha256: '6'.repeat(64) },
    browserExecutable: join(root, 'runtime/chromium'), browserExecutableSHA256: '7'.repeat(64),
    samples: 100, parallelism: 20, durationSeconds: 30,
  };
  return { root, config, artifact };
}

async function browser_source_inventory_roundtrip(t) {
  const fixture = await browserManifestFixture(t);
  const requests = [];
  const server = createServer(async (request, response) => {
    requests.push({ method: request.method, url: request.url, accept: request.headers.accept });
    try {
      // The manifest producer uses the same inventory function as the measurement consumer.
      const config = { ...fixture.config, sourceFiles: await inventoryBrowserSources(sourceRoot) };
      const raw = JSON.stringify(config);
      response.writeHead(200, { 'content-type': 'application/json', 'x-manifest-sha256': sha256(raw) });
      response.end(raw);
    } catch (error) {
      response.writeHead(500);
      response.end(error.message);
    }
  });
  t.after(() => new Promise((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
    server.closeAllConnections();
  }));
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const response = await fetch(`http://127.0.0.1:${server.address().port}/l40/browser-manifest`, {
    headers: { accept: 'application/json' },
  });
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('content-type'), 'application/json');
  assert.deepEqual(requests, [{ method: 'GET', url: '/l40/browser-manifest', accept: 'application/json' }]);
  const raw = await response.text();
  assert.equal(response.headers.get('x-manifest-sha256'), sha256(raw));
  const manifest = JSON.parse(raw);
  assert.deepEqual(Object.keys(manifest).sort(), Object.keys(fixture.config).sort());
  assert.deepEqual(Object.keys(manifest.sourceFiles), sourcePaths);
  for (const path of sourcePaths) {
    assert.equal(manifest.sourceFiles[path], sha256(await readFile(resolve(sourceRoot, path))));
  }
  assert.equal(Object.keys(manifest.sourceFiles).some(path => path.startsWith('client/')), false);
  const path = join(fixture.root, 'transported-manifest.json');
  await writeFile(path, raw);
  const admitted = await readBrowserManifest({ path, sha256: response.headers.get('x-manifest-sha256') });
  assert.deepEqual(admitted.config, manifest);
  assert.equal(admitted.origin.origin, manifest.applicationOrigin);
  assert.deepEqual(admitted.profile, JSON.parse(await readFile(manifest.networkProfile.path, 'utf8')));
}

test(browser_source_inventory_roundtrip);

async function browser_source_inventory_rejects_drift(t) {
  const { config, artifact } = await browserManifestFixture(t);
  for (const path of sourcePaths) {
    await t.test(`requires current path ${path}`, async () => {
      const sourceFiles = { ...config.sourceFiles };
      delete sourceFiles[path];
      sourceFiles[path.replace('apps/launchpad/web/', 'client/apps/web/')] = config.sourceFiles[path];
      await assert.rejects(readBrowserManifest(await artifact('missing.json', { ...config, sourceFiles })), /Current source inventory required/);
    });
    await t.test(`binds digest for ${path}`, async () => {
      const sourceFiles = { ...config.sourceFiles, [path]: '0'.repeat(64) };
      await assert.rejects(readBrowserManifest(await artifact('mismatch.json', { ...config, sourceFiles })), /Current browser source mismatch/);
    });
  }
  await assert.rejects(inventoryBrowserSources('relative-candidate'), /Absolute candidate source root required/);
  await assert.rejects(readBrowserManifest(await artifact('relative.json', { ...config, sourceRoot: '.' })), /Current source inventory required/);
  const invalid = { ...config.sourceFiles, [sourcePaths[0]]: 'not-a-digest' };
  await assert.rejects(readBrowserManifest(await artifact('invalid.json', { ...config, sourceFiles: invalid })), /Source SHA256 binding missing/);
  for (const path of ['../outside-candidate', '/outside-candidate']) {
    const sourceFiles = { ...config.sourceFiles, [path]: '8'.repeat(64) };
    await assert.rejects(readBrowserManifest(await artifact('escape.json', { ...config, sourceFiles })), /Source path escapes candidate/);
  }
  const extraPath = 'apps/launchpad/web/src/page-modules/live/all.tsx';
  const extraFiles = { ...config.sourceFiles, [extraPath]: sha256(await readFile(resolve(sourceRoot, extraPath))) };
  const extra = await artifact('extra.json', { ...config, sourceFiles: extraFiles });
  assert.deepEqual((await readBrowserManifest(extra)).config.sourceFiles, extraFiles);
  extraFiles[extraPath] = '0'.repeat(64);
  await assert.rejects(readBrowserManifest(await artifact('extra-mismatch.json', { ...config, sourceFiles: extraFiles })), /Current browser source mismatch/);
}

test(browser_source_inventory_rejects_drift);

async function browser_source_inventory_retains_evidence_checks(t) {
  const { config, artifact } = await browserManifestFixture(t);
  const manifest = await artifact('manifest.json', config);
  await assert.rejects(readBrowserManifest({ ...manifest, sha256: '0'.repeat(64) }), /Artifact digest or size mismatch/);
  for (const field of ['authority', 'license', 'networkProfile']) {
    await t.test(`checks ${field} digest`, async () => {
      const changed = { ...config, [field]: { ...config[field], sha256: '0'.repeat(64) } };
      await assert.rejects(readBrowserManifest(await artifact('artifact-digest.json', changed)), /Artifact digest or size mismatch/);
    });
  }
  const changes = [
    [{ ...config, unexpected: true }, /Exact browser manifest schema required/],
    [{ ...config, candidate: 'invalid' }, /Browser evidence identity missing/],
    [{ ...config, samples: 99 }, /Bounded real browser sample population required/],
    [{ ...config, samples: 1001 }, /Bounded real browser sample population required/],
    [{ ...config, parallelism: 0 }, /Bounded real browser sample population required/],
    [{ ...config, parallelism: 21 }, /Bounded real browser sample population required/],
    [{ ...config, durationSeconds: 29 }, /Bounded real browser sample population required/],
    [{ ...config, durationSeconds: 121 }, /Bounded real browser sample population required/],
    [{ ...config, applicationOrigin: 'http://reviewed.example' }, /Exact TLS application origin required/],
    [{ ...config, poolPage: 'https://other.example/pool/1' }, /Browser journey outside application origin/],
    [{ ...config, directoryPage: 'https://other.example/live' }, /Browser journey outside application origin/],
    [{ ...config, revision: '0' }, /Exact stream identity and revision required/],
  ];
  for (const [changed, message] of changes) {
    await assert.rejects(readBrowserManifest(await artifact('rejected.json', changed)), message);
  }
  const authority = JSON.parse(await readFile(config.authority.path, 'utf8'));
  for (const field of ['runId', 'candidate', 'interface', 'topology', 'generator', 'lane', 'expiresAt']) {
    const changed = { ...authority, [field]: field === 'expiresAt' ? '2000-01-01T00:00:00Z' : 'unreviewed' };
    const reference = await artifact('unadmitted-authority.json', changed);
    await assert.rejects(readBrowserManifest(await artifact('unadmitted.json', { ...config, authority: reference })), /Browser workload admission does not bind this run/);
  }
  const license = JSON.parse(await readFile(config.license.path, 'utf8'));
  const expiredLicense = await artifact('expired-license.json', { ...license, validUntil: '2000-01-01T00:00:00Z' });
  await assert.rejects(readBrowserManifest(await artifact('unlicensed.json', { ...config, license: expiredLicense })), /Licensed content authority missing/);
  const profile = JSON.parse(await readFile(config.networkProfile.path, 'utf8'));
  const unapprovedProfile = await artifact('unapproved-profile.json', { ...profile, region: 'unlicensed-region' });
  await assert.rejects(readBrowserManifest(await artifact('unapproved.json', { ...config, networkProfile: unapprovedProfile })), /Approved browser network profile missing/);
}

test(browser_source_inventory_retains_evidence_checks);

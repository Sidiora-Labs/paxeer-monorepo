import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mergeEnvironment, environmentBlocks, environmentNames, fingerprint, releaseId, siteConfig, sha256, bindArchiveManifest } from './release-config.mjs';
import { makeZip } from './package-release.mjs';

test('environment merge preserves comments, exports, multiline values and unrelated assignments', () => {
  const previous = '# operator settings\nexport KEEP="first\nRELEASE_ID=inside-the-value\nlast"\nCUSTOM=42\nRELEASE_ID=old\n';
  const result = mergeEnvironment(previous, { RELEASE_ID: 'docs-20260914', PORT: '3000' });
  assert.ok(result.startsWith('# operator settings\nexport KEEP="first\nRELEASE_ID=inside-the-value\nlast"\nCUSTOM=42\n'));
  assert.equal(environmentBlocks(result).filter(block => block.key === 'RELEASE_ID').length, 1);
  assert.ok(result.endsWith('RELEASE_ID="docs-20260914"\nPORT="3000"\n'));
  assert.deepEqual(environmentNames(result), ['CUSTOM', 'KEEP', 'PORT', 'RELEASE_ID']);
});

test('managed duplicates are replaced without changing unrelated duplicates or quoted content', () => {
  const previous = 'CUSTOM=one\nPORT=80\nCUSTOM=two\nPORT="8080"\nTEXT="a \\"quoted\\" word"\n';
  const result = mergeEnvironment(previous, { PORT: '3000' });
  assert.equal(environmentBlocks(result).filter(block => block.key === 'PORT').length, 1);
  assert.ok(result.includes('CUSTOM=one\nCUSTOM=two\n'));
  assert.ok(result.includes('TEXT="a \\"quoted\\" word"'));
});

test('unterminated existing quotes and multiline managed values are rejected', () => {
  assert.throws(() => mergeEnvironment('CUSTOM="unterminated', { PORT: '3000' }), { code: 'invalid_existing_environment' });
  assert.throws(() => mergeEnvironment('', { PORT: '3000\nINJECTED=1' }), { code: 'invalid_environment_value' });
});

test('escaped single quotes and dotenv dotted keys retain embedded managed-looking lines', () => {
  const previous = String.raw`OTHER.SECRET='first\'
PORT=inside-unrelated-secret
last'
KEEP-DASH="line one
RELEASE_ID=also-inside
last"
KEEP=value
`;
  const result = mergeEnvironment(previous, { PORT: '3000', RELEASE_ID: 'r1' });
  assert.ok(result.startsWith(previous));
  assert.equal(environmentBlocks(result).filter(block => block.key === 'PORT').length, 1);
  assert.equal(environmentBlocks(result).filter(block => block.key === 'RELEASE_ID').length, 1);
});

test('release and site identities reject paths and unrelated applications', () => {
  assert.equal(releaseId('docs-2026.09.14-r1'), 'docs-2026.09.14-r1');
  assert.throws(() => releaseId('../release'), { code: 'invalid_release' });
  assert.throws(() => siteConfig('centra-webapp-canary'), { code: 'invalid_site' });
  assert.throws(() => siteConfig('__proto__'), { code: 'invalid_site' });
  assert.equal(siteConfig('paxeer-docs').origin, 'https://docs.paxeer.app');
});

test('file fingerprints bind path, bytes and content independent of entry order', () => {
  const a = { path: 'a', sha256: sha256('alpha'), bytes: 5 }, b = { path: 'b', sha256: sha256('beta'), bytes: 4 };
  assert.equal(fingerprint([a, b]), fingerprint([b, a]));
  assert.notEqual(fingerprint([a]), fingerprint([{ ...a, path: 'renamed' }]));
  assert.notEqual(fingerprint([a]), fingerprint([{ ...a, sha256: sha256('changed') }]));
});

test('ZIP round-trips through Python standard library with CRC and UTF-8 validation', () => {
  const entries = [{ path: 'Dockerfile', data: Buffer.from('FROM node:22-bookworm-slim\n') }, { path: 'app/out/docs/café.txt', data: Buffer.from('A documentation page.\n'.repeat(100)) }, { path: 'app/empty', data: Buffer.alloc(0) }];
  const archive = makeZip(entries);
  assert.deepEqual(archive, makeZip(entries));
  const result = spawnSync('python3', ['-c', 'import io,json,sys,zipfile,hashlib\nz=zipfile.ZipFile(io.BytesIO(sys.stdin.buffer.read()))\nassert z.testzip() is None\nprint(json.dumps([{ "path":i.filename,"sha256":hashlib.sha256(z.read(i)).hexdigest(),"mode":i.external_attr>>16,"date":i.date_time } for i in z.infolist()]))'], { input: archive, maxBuffer: 1024 * 1024 });
  assert.equal(result.status, 0, result.stderr.toString());
  const decoded = JSON.parse(result.stdout.toString());
  assert.deepEqual(decoded.map(({ path, sha256 }) => ({ path, sha256 })), entries.map(entry => ({ path: entry.path, sha256: sha256(entry.data) })));
  assert.ok(decoded.every(entry => entry.mode === 0o100644 && entry.date[0] === 1980));
});

test('ZIP does not admit parent or absolute archive paths', () => {
  assert.throws(() => makeZip([{ path: '../outside', data: Buffer.from('x') }]), { code: 'unsafe_zip_path' });
  assert.throws(() => makeZip([{ path: '/outside', data: Buffer.from('x') }]), { code: 'unsafe_zip_path' });
});

test('archive metadata prevents relabeling an existing ZIP as another site or release', () => {
  const metadata = { release: 'r1', site: 'learn-centra', origin: 'https://learn.centra.ag', sourceSha256: sha256('source'), buildSha256: sha256('build') };
  const zip = makeZip([{ path: 'app/release.json', data: Buffer.from(JSON.stringify(metadata)) }]);
  bindArchiveManifest(metadata, zip);
  assert.throws(() => bindArchiveManifest({ ...metadata, site: 'learn-workmates', origin: 'https://learn.workmates.ag' }, zip), { code: 'archive_manifest_mismatch' });
  assert.throws(() => bindArchiveManifest({ ...metadata, release: 'r2' }, zip), { code: 'archive_manifest_mismatch' });
});

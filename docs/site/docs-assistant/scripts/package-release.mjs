import { readFile, readdir, lstat, mkdir, writeFile, unlink } from 'node:fs/promises';
import { dirname, resolve, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { deflateRawSync } from 'node:zlib';
import { SITES, ensure, fingerprint, parseArgs, releaseId, safeFailure, sha256, siteConfig } from './release-config.mjs';

const workspace = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const blocked = part => part === 'node_modules' || part === '.git' || part === '.codegraph' || part === '.env' || part.startsWith('.env.') || /\.(?:pem|key|p12|pfx|sqlite|db)$/i.test(part);
const crcTable = Array.from({ length: 256 }, (_, i) => { let value = i; for (let bit = 0; bit < 8; bit++) value = value & 1 ? 0xedb88320 ^ value >>> 1 : value >>> 1; return value >>> 0; });
function crc32(data) { let value = 0xffffffff; for (const byte of data) value = crcTable[(value ^ byte) & 255] ^ value >>> 8; return (value ^ 0xffffffff) >>> 0; }
async function filesAt(root, base = root) {
  const info = await lstat(root);
  ensure(!info.isSymbolicLink(), 'symlink_input', 'Release inputs must be regular files and directories, without symlinks.');
  if (info.isFile()) return [{ absolute: root, path: relative(base, root).split(sep).join('/'), info }];
  ensure(info.isDirectory(), 'special_input', 'A release input is not a regular file or directory.');
  const files = [];
  for (const item of (await readdir(root)).sort()) {
    ensure(!blocked(item), 'excluded_release_input', 'A build output contains node_modules, dotenv, history, credentials or a database; rebuild a self-contained output before packaging.');
    files.push(...await filesAt(resolve(root, item), base));
  }
  return files;
}
async function optionalSource(root, name) {
  try { return await filesAt(resolve(root, name), workspace); }
  catch (error) { if (error.code === 'ENOENT') return []; throw error; }
}
async function snapshot(files) {
  return Promise.all(files.map(async file => {
    const data = await readFile(file.absolute), after = await lstat(file.absolute);
    ensure(after.size === file.info.size && after.mtimeMs === file.info.mtimeMs, 'input_changed', 'An input changed during packaging. Finish the build before packaging.');
    return { path: file.path, sha256: sha256(data), bytes: data.length };
  }));
}
export function makeZip(entries) {
  ensure(entries.length < 65535, 'zip_limit', 'Release has too many files for the ZIP format.');
  const chunks = [], central = []; let offset = 0;
  for (const entry of entries) {
    const name = Buffer.from(entry.path, 'utf8'), data = entry.data;
    ensure(!entry.path.startsWith('/') && !entry.path.split('/').some(part => part === '..' || !part), 'unsafe_zip_path', 'Archive paths must stay inside the release.');
    const packed = deflateRawSync(data, { level: 6 }), crc = crc32(data);
    const header = Buffer.alloc(30); header.writeUInt32LE(0x04034b50); header.writeUInt16LE(20, 4); header.writeUInt16LE(0x800, 6); header.writeUInt16LE(8, 8); header.writeUInt16LE(33, 12); header.writeUInt32LE(crc, 14); header.writeUInt32LE(packed.length, 18); header.writeUInt32LE(data.length, 22); header.writeUInt16LE(name.length, 26);
    chunks.push(header, name, packed);
    const index = Buffer.alloc(46); index.writeUInt32LE(0x02014b50); index.writeUInt16LE(0x314, 4); index.writeUInt16LE(20, 6); index.writeUInt16LE(0x800, 8); index.writeUInt16LE(8, 10); index.writeUInt16LE(33, 14); index.writeUInt32LE(crc, 16); index.writeUInt32LE(packed.length, 20); index.writeUInt32LE(data.length, 24); index.writeUInt16LE(name.length, 28); index.writeUInt32LE((0o100644 << 16) >>> 0, 38); index.writeUInt32LE(offset, 42);
    central.push(index, name); offset += header.length + name.length + packed.length;
    ensure(offset < 0xffffffff, 'zip_limit', 'Release exceeds the supported ZIP size.');
  }
  const directory = Buffer.concat(central), end = Buffer.alloc(22); end.writeUInt32LE(0x06054b50); end.writeUInt16LE(entries.length, 8); end.writeUInt16LE(entries.length, 10); end.writeUInt32LE(directory.length, 12); end.writeUInt32LE(offset, 16);
  return Buffer.concat([...chunks, directory, end]);
}
function dockerfile(release) {
  return `FROM node:22-bookworm-slim\nWORKDIR /app\nENV NODE_ENV=production PORT=3000 TRUST_PROXY=1 RELEASE_ID=${release}\nCOPY app/ /app/\nCOPY docs-assistant/server/ /docs-assistant/server/\nUSER node\nEXPOSE 3000\nHEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD node -e "fetch('http://127.0.0.1:'+process.env.PORT+'/healthz').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"\nCMD ["node", "scripts/serve.mjs"]\n`;
}
export async function packageSite(site, release, outputDirectory) {
  const config = siteConfig(site), root = resolve(workspace, site);
  const sources = [];
  for (const name of ['src', 'public', 'scripts', 'package.json', 'package-lock.json', 'pnpm-lock.yaml', '.npmrc', 'Dockerfile', '.dockerignore', 'vite.config.ts', 'next.config.js', 'postcss.config.js', 'postcss.config.mjs', 'tsconfig.json']) sources.push(...await optionalSource(root, name));
  for (const name of ['server', 'src', 'scripts', 'package.json']) sources.push(...await optionalSource(resolve(workspace, 'docs-assistant'), name));
  const sourceFiles = await snapshot(sources), sourceHash = fingerprint(sourceFiles);
  const outputRoot = resolve(root, config.output), buildFiles = await filesAt(outputRoot);
  ensure(buildFiles.length > 0, 'missing_build', 'Build the selected documentation site before packaging.');
  const corpusPath = resolve(root, '.generated/assistant-docs.json'), corpusData = await readFile(corpusPath), corpus = JSON.parse(corpusData);
  ensure(corpus.origin === config.origin && corpus.pages?.length > 0, 'corpus_origin', 'The assistant index must contain documents for this site’s canonical origin.');
  const entries = [], buildHashes = [];
  for (const file of buildFiles) {
    const data = await readFile(file.absolute), after = await lstat(file.absolute);
    ensure(after.size === file.info.size && after.mtimeMs === file.info.mtimeMs, 'build_changed', 'A build file changed during packaging. Finish the build before packaging.');
    const path = `app/${config.output}/${file.path}`;
    entries.push({ path, data }); buildHashes.push({ path, sha256: sha256(data), bytes: data.length });
  }
  const required = config.kind === 'worker' ? 'app/.output/server/index.mjs' : 'app/out/index.html';
  ensure(entries.some(entry => entry.path === required), 'missing_entrypoint', 'The selected site’s built entrypoint is missing.');
  entries.push({ path: 'app/.generated/assistant-docs.json', data: corpusData });
  entries.push({ path: 'app/scripts/serve.mjs', data: await readFile(resolve(root, 'scripts/serve.mjs')) });
  for (const file of await filesAt(resolve(workspace, 'docs-assistant/server'))) entries.push({ path: `docs-assistant/server/${file.path}`, data: await readFile(file.absolute) });
  const buildHash = fingerprint(buildHashes);
  entries.push({ path: 'app/release.json', data: Buffer.from(JSON.stringify({ release, site, origin: config.origin, sourceSha256: sourceHash, buildSha256: buildHash }) + '\n') });
  entries.push({ path: 'Dockerfile', data: Buffer.from(dockerfile(release)) });
  entries.sort((a, b) => a.path.localeCompare(b.path));
  ensure(fingerprint(await snapshot(sources)) === sourceHash, 'source_changed', 'Source files changed during packaging. Package again after editing finishes.');
  const currentBuild = await filesAt(outputRoot);
  const verifiedBuild = (await snapshot(currentBuild)).map(file => ({ ...file, path: `app/${config.output}/${file.path}` }));
  ensure(fingerprint(verifiedBuild) === buildHash, 'build_changed', 'The build output changed during packaging. Finish the build before packaging.');
  ensure(sha256(await readFile(corpusPath)) === sha256(corpusData), 'corpus_changed', 'The assistant index changed during packaging.');
  const zip = makeZip(entries), stem = `${site}-${release}`, zipName = `${stem}.zip`;
  const manifest = { schema: 1, createdAt: new Date().toISOString(), release, site, origin: config.origin, kind: config.kind, port: 3000, model: 'openrouter/sakana/fugu-max', zipFile: zipName, zipSha256: sha256(zip), zipBytes: zip.length, sourceSha256: sourceHash, buildSha256: buildHash, documents: corpus.pages.length, sourceFiles, buildFiles: buildHashes, archiveFiles: entries.map(entry => ({ path: entry.path, sha256: sha256(entry.data), bytes: entry.data.length })) };
  await mkdir(outputDirectory, { recursive: true });
  const zipPath = resolve(outputDirectory, zipName), manifestPath = resolve(outputDirectory, `${stem}.manifest.json`);
  await writeFile(zipPath, zip, { flag: 'wx', mode: 0o600 });
  try { await writeFile(manifestPath, JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx', mode: 0o600 }); }
  catch (error) { await unlink(zipPath); throw error; }
  return { site, release, manifest: manifestPath, zip: zipPath, zipSha256: manifest.zipSha256, zipBytes: zip.length, files: entries.length, sourceSha256: sourceHash, buildSha256: buildHash };
}
async function main() {
  const args = parseArgs(process.argv.slice(2), ['--site', '--release', '--out'], ['--help']);
  if (args['--help']) { console.log('Usage: node docs-assistant/scripts/package-release.mjs --site <learn-centra|learn-workmates|paxeer-docs|all> --release <id> --out <directory>\nPackages prebuilt outputs only. Creates immutable ZIPs and sanitized hash manifests; never builds or deploys.'); return; }
  const release = releaseId(args['--release']); ensure(args['--out'], 'missing_output', 'Supply --out for the release directory.');
  const sites = args['--site'] === 'all' ? Object.keys(SITES) : [args['--site']]; sites.forEach(siteConfig);
  const results = []; for (const site of sites) results.push(await packageSite(site, release, resolve(args['--out'])));
  console.log(JSON.stringify({ status: 'packaged', releases: results }, null, 2));
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main().catch(error => { console.error(JSON.stringify({ status: 'failed', ...safeFailure(error) })); process.exitCode = 1; });

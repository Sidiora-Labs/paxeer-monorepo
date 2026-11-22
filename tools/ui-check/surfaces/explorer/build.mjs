import { execFileSync, spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFile, readdir, readlink, stat, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const environment = JSON.parse(await readFile(new URL('./environment.json', import.meta.url), 'utf8'));
const options = {
  stdio: 'inherit',
  env: {
    ...process.env,
    ...environment,
    DISABLE_TRACKING: 'true',
    NEXT_TELEMETRY_DISABLED: '1',
    NEXT_PRIVATE_OUTPUT_TRACE_ROOT: process.cwd(),
  },
};
const repository = fileURLToPath(new URL('../../../../', import.meta.url));
const compilationFile = '.next/cache/explorer-compilation.json';
async function sourceFingerprint() {
  const hash = createHash('sha256');
  const git = (args) => execFileSync('git', ['-C', repository, ...args], { maxBuffer: 64 * 1024 * 1024 });
  hash.update(process.version);
  hash.update(JSON.stringify(Object.entries(options.env).sort(([a], [b]) => a.localeCompare(b))));
  hash.update(git(['rev-parse', 'HEAD']));
  hash.update(git(['diff', '--binary', 'HEAD', '--']));
  const files = git(['ls-files', '--others', '--exclude-standard', '-z']).toString().split('\0').filter(Boolean).sort();
  for (const file of files) {
    hash.update(file + '\0');
    hash.update(await readFile(join(repository, file)));
  }
  return hash.digest('hex');
}
async function outputFingerprint() {
  const hash = createHash('sha256');
  async function visit(filename) {
    const info = await stat(filename);
    hash.update(filename + '\0');
    if (info.isDirectory()) {
      for (const entry of (await readdir(filename, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name))) {
        const child = join(filename, entry.name);
        if (entry.isSymbolicLink()) {
          hash.update(child + '\0' + await readlink(child) + '\0');
        } else {
          await visit(child);
        }
      }
    } else {
      hash.update(await readFile(filename));
    }
  }
  try {
    for (const filename of [
      '.next/server', '.next/static', '.next/BUILD_ID', '.next/required-server-files.json',
      '.next/build-manifest.json', '.next/routes-manifest.json', '.next/prerender-manifest.json',
      '.next/react-loadable-manifest.json', '.next/cache/explorer-transaction-answers.json',
      '/tmp/explorer-block-pages.json', '/tmp/explorer-icons/sprite.svg', '/tmp/explorer-icons/registry.json',
    ]) await visit(filename);
  } catch (error) {
    if (error.code === 'ENOENT') return null;
    throw error;
  }
  return hash.digest('hex');
}
async function readCompilation() {
  try {
    return JSON.parse(await readFile(compilationFile, 'utf8'));
  } catch (error) {
    if (error.code === 'ENOENT') return null;
    throw error;
  }
}
async function run(command, args) {
  const child = spawn(command, args, options);
  await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', (code, signal) => code === 0 ? resolve() : reject(new Error(`${command} ended with ${signal ?? code}`)));
  });
}
async function icons(directory, prefix = '') {
  const entries = await readdir(directory, { withFileTypes: true });
  const groups = await Promise.all(entries.map(async (entry) => {
    const filename = directory + '/' + entry.name;
    const name = prefix + entry.name;
    if (entry.isDirectory()) return icons(filename, name + '/');
    return entry.name.endsWith('.svg') ? [{ name, file_size: (await stat(filename)).size }] : [];
  }));
  return groups.flat();
}
async function assertExplorerPortAvailable() {
  const listener = createServer();
  await new Promise((resolve, reject) => {
    listener.once('error', reject);
    listener.listen(4173, 'localhost', () => {
      listener.close((error) => error ? reject(error) : resolve());
    });
  });
}
await assertExplorerPortAvailable();
await run(process.execPath, [
  '--test',
  fileURLToPath(new URL('./routes.test.mjs', import.meta.url)),
  fileURLToPath(new URL('./trace-readlinks.test.mjs', import.meta.url)),
  fileURLToPath(new URL('./lifecycle.test.mjs', import.meta.url)),
]);
await run('bash', ['-c', 'corepack enable && yarn install --frozen-lockfile --ignore-scripts']);
await run(process.execPath, ['node_modules/svg-icons-cli/index.js', 'build', '-i', './icons', '-o', relative(process.cwd(), '/tmp/explorer-icons'), '--optimize']);
await writeFile('/tmp/explorer-icons/registry.json', JSON.stringify(await icons('./icons')));
const source = await sourceFingerprint();
const compilation = await readCompilation();
if (compilation?.source === source && typeof compilation.output === 'string' && compilation.output === await outputFingerprint()) {
  console.log('Explorer compilation matches its inputs and output.');
} else {
  console.log('Compiling explorer sources.');
  await run(process.execPath, [
    '--no-async-stack-traces', '--import', fileURLToPath(new URL('./build-config.mjs', import.meta.url)),
    'node_modules/next/dist/bin/next', 'build',
  ]);
}
await (await import('./transactions.mjs')).prepareTransactionAnswers();
await run(process.execPath, [fileURLToPath(new URL('./start.mjs', import.meta.url)), '--prepare']);
if (source !== await sourceFingerprint()) throw new Error('Explorer compilation requires stable inputs');
const output = await outputFingerprint();
if (!output) throw new Error('Explorer compilation requires complete output');
await writeFile(compilationFile, JSON.stringify({ source, output }));

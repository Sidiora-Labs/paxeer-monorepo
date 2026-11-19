import { spawn } from 'node:child_process';
import { readFile, readdir, stat, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { relative } from 'node:path';
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
await run(process.execPath, [
  '--no-async-stack-traces', '--import', fileURLToPath(new URL('./build-config.mjs', import.meta.url)),
  'node_modules/next/dist/bin/next', 'build',
]);
await (await import('./transactions.mjs')).prepareTransactionAnswers();
await run(process.execPath, [fileURLToPath(new URL('./start.mjs', import.meta.url)), '--prepare']);

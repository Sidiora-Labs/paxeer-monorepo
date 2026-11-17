import { execFileSync, spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { applicationEnvironment } from './environment.mjs';

function run(args) {
  return new Promise((resolve, reject) => {
    const child = spawn('pnpm', args, { stdio: 'inherit', env: applicationEnvironment() });
    child.on('error', reject);
    child.on('exit', code => code === 0 ? resolve() : reject(new Error(`Build exited ${code}`)));
  });
}

const inputs = execFileSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--',
  'apps/launchpad/web', 'apps/launchpad/package.json', 'apps/launchpad/pnpm-lock.yaml',
  'apps/launchpad/pnpm-workspace.yaml', 'sdk/launchpad', 'services/launchpad/security',
  'precompiles/launchpad/abi', 'tools/ui-check/surfaces/launchpad/build.mjs',
  'tools/ui-check/surfaces/launchpad/environment.mjs',
]).toString().split('\0').filter(Boolean).sort();
const environment = Object.entries(applicationEnvironment())
  .filter(([name]) => /^(NEXT_|WEB_|SENTRY_|NODE_ENV$|NODE_OPTIONS$|VERCEL$|CI$)/.test(name))
  .sort(([left], [right]) => left.localeCompare(right));
const digest = createHash('sha256').update(JSON.stringify(environment)).update(process.version);
for (const input of inputs) {
  digest.update(input);
  digest.update(readFileSync(input));
}
const signature = digest.digest('hex');
const receipt = join(tmpdir(), `launchpad-web-${signature}`);
const output = 'apps/launchpad/web/.next/BUILD_ID';
if (existsSync(receipt) && existsSync(output) && readFileSync(receipt, 'utf8') === readFileSync(output, 'utf8')) {
  console.log('Production output matches the application inputs.');
} else {
  await run(['--dir', 'apps/launchpad', '-r', '--filter', '@kindle/sdk...', '--filter', '@kindle/sdk-v2...', '--filter', 'stats-sdk...', 'run', 'build']);
  await run(['--dir', 'apps/launchpad/web', 'run', 'build']);
  writeFileSync(receipt, readFileSync(output));
}

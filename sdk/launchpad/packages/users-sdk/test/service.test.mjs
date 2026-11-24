import { test } from 'node:test';
import { mkdtemp, readFile, writeFile, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import assert from 'node:assert/strict';

async function usersSDKReadsServiceResponses(t) {
  const root = fileURLToPath(new URL('../../../../..', import.meta.url));
  const service = join(root, 'services/launchpad/user');
  const shared = join(root, 'sdk/launchpad/shared');
  const directory = await mkdtemp(join(tmpdir(), 'users-sdk-contract-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  let manifest = await readFile(join(shared, 'go.mod'), 'utf8');
  manifest = manifest.replace(/^module .*$/m, 'module github.com/Sidiora-Technologies/KindleLaunch/media/user');
  manifest = manifest.replace('=> ../protocol', `=> ${join(root, 'sdk/launchpad/protocol')}`);
  manifest = manifest.replace('github.com/jackc/pgx/v5 v5.10.0', 'github.com/jackc/pgx/v5 v5.11.0');
  manifest += `\nrequire (\n github.com/Sidiora-Technologies/KindleLaunch/shared v0.0.0\n github.com/pressly/goose/v3 v3.24.3\n)\nreplace github.com/Sidiora-Technologies/KindleLaunch/shared => ${shared}\n`;
  await writeFile(join(directory, 'go.mod'), manifest);
  await writeFile(join(directory, 'go.sum'), await readFile(join(shared, 'go.sum')));
  for (const name of ['internal', 'pkg', 'migrations']) await symlink(join(service, name), join(directory, name), 'dir');
  await symlink(fileURLToPath(new URL('service/contract_test.go', import.meta.url)), join(directory, 'contract_test.go'));
  const env = { ...process.env, USERS_SDK_CONSUMER: resolve(fileURLToPath(new URL('service-cases.mjs', import.meta.url))) };
  const child = spawn('go', ['test', '-mod=mod', '-count=1', '-timeout=2m', '-v', '.'], { cwd: directory, env, stdio: ['ignore', 'pipe', 'pipe'] });
  t.after(() => { if (child.exitCode === null) child.kill('SIGKILL'); });
  let output = '';
  child.stdout.setEncoding('utf8').on('data', chunk => { output += chunk; });
  child.stderr.setEncoding('utf8').on('data', chunk => { output += chunk; });
  const code = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', resolve);
  });
  assert.equal(code, 0, output);
  assert.match(output, /--- PASS: TestUsersSDKService/);
}

test(usersSDKReadsServiceResponses, { timeout: 600_000 });

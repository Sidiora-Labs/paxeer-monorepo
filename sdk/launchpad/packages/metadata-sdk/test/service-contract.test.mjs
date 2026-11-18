import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, readFile, writeFile, copyFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

async function serviceRoutesPreserveMetadataAndCreatorSession() {
  const directory = dirname(fileURLToPath(import.meta.url));
  const service = resolve(directory, '../../../../../services/launchpad/metadata');
  const temporary = await mkdtemp(join(tmpdir(), 'metadata-sdk-contract-'));
  try {
    const manifest = await readFile(join(service, 'go.mod'), 'utf8');
    const module = /^module (.+)$/m.exec(manifest)[1];
    const dependencies = manifest
      .replace(/^module .+$/m, `module ${module}/sdkcontract`)
      .replace(/^(replace .+ => )(\..+)$/gm, (_line, prefix, path) => prefix + resolve(service, path));
    await writeFile(join(temporary, 'go.mod'), `${dependencies}\nrequire ${module} v0.0.0\nreplace ${module} => ${service}\n`);
    await copyFile(join(service, 'go.sum'), join(temporary, 'go.sum'));
    await copyFile(join(directory, 'service-contract.go'), join(temporary, 'contract_test.go'));
    const command = spawn('go', ['test', '-mod=mod', '-count=1', '-timeout=10m', '-run=^TestMetadataSDKContract$', '-v', '.'], {
      cwd: temporary,
      env: { ...process.env, GOWORK: 'off', METADATA_SDK_CONSUMER: join(directory, 'service-consumer.mjs') },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    const output = [];
    command.stdout.on('data', chunk => output.push(chunk));
    command.stderr.on('data', chunk => output.push(chunk));
    const code = await new Promise((resolve, reject) => {
      command.once('error', reject);
      command.once('close', resolve);
    });
    assert.equal(code, 0, Buffer.concat(output).toString());
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
}

test(serviceRoutesPreserveMetadataAndCreatorSession);

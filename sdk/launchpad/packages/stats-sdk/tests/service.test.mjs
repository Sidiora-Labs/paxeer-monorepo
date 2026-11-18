import test, { after, before } from 'node:test';
import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { StatsHttpClient, StatsHttpError, parsePoolStats } from '../dist/index.js';
import { poolStatsFields } from '../dist/types.js';

let service;
let directory;
let client;
let contract;
let completion;
let output = '';
let pending;

function serviceMessage() {
  assert.equal(pending, undefined);
  return new Promise((resolve, reject) => { pending = { resolve, reject }; });
}

async function command(value) {
  const message = serviceMessage();
  service.stdin.write(value + '\n');
  assert.equal(await message, value);
}

before(async function startStatsService() {
  directory = await mkdtemp(join(tmpdir(), 'stats-sdk-'));
  const root = fileURLToPath(new URL('../../../../../', import.meta.url));
  const serviceRoot = join(root, 'services/launchpad/stats-workers');
  const overlay = join(directory, 'overlay.json');
  await writeFile(overlay, JSON.stringify({ Replace: {
    [join(serviceRoot, 'internal/httpapi/stats_sdk_contract_test.go')]: fileURLToPath(new URL('./service.go', import.meta.url)),
  } }));
  const binary = join(directory, 'stats-service');
  await promisify(execFile)('go', ['test', '-overlay', overlay, '-c', '-o', binary, './internal/httpapi'], {
    cwd: serviceRoot,
  });
  const ready = serviceMessage();
  service = spawn(binary, ['-test.v', '-test.run', '^TestStatsSDKService$'], {
    cwd: serviceRoot,
    stdio: ['pipe', 'pipe', 'pipe'],
  });
  const lines = createInterface({ input: service.stdout });
  lines.on('line', line => {
    output += line + '\n';
    if (line.startsWith('stats-service ')) {
      const waiter = pending;
      pending = undefined;
      waiter.resolve(line.slice('stats-service '.length));
    }
  });
  service.stderr.on('data', chunk => { output += chunk; });
  completion = new Promise((resolve, reject) => {
    service.on('error', error => { pending?.reject(error); pending = undefined; reject(error); });
    service.on('close', code => {
      const error = new Error(`Stats service exited ${code}: ${output}`);
      pending?.reject(error);
      pending = undefined;
      resolve(code);
    });
  });
  contract = JSON.parse(await ready);
  client = new StatsHttpClient();
  client.configure({ baseUrl: contract.baseUrl });
});

after(async function stopStatsService() {
  if (service && service.exitCode === null) service.stdin.end('stop\n');
  if (completion) assert.equal(await completion, 0, output);
  if (directory) await rm(directory, { recursive: true });
});

async function singlePoolReadsServiceRowsAndNotFound() {
  for (const pool of [contract.pool, contract.otherPool]) {
    const expected = contract.rows[pool];
    assert.deepEqual(Object.keys(expected).sort(), Object.keys(poolStatsFields).sort());
    assert.deepEqual(parsePoolStats(expected), expected);
    assert.deepEqual(await client.getPoolStats(pool), expected);
    assert.deepEqual(await client.getPoolStats(pool), expected);
    assert.equal(expected.price, '9007199254740993123456789');
    assert.equal(expected.marketCap, '123456789012345678901234567890');
    assert.equal(expected.createdAt, 1700000000);
    assert.equal(expected.updatedAt, 1700000123);
  }
  assert.equal(contract.rows[contract.pool].riskFactors, null);
  assert.equal(contract.rows[contract.pool].creatorAddress, null);
  assert.equal(contract.rows[contract.otherPool].riskFactors, '["concentration"]');
  assert.equal(contract.rows[contract.otherPool].creatorAddress, '0x' + 'Ab'.repeat(20));
  await assert.rejects(client.getPoolStats(contract.missingPool), error => {
    assert.ok(error instanceof StatsHttpError);
    assert.equal(error.status, 404);
    assert.equal(error.body, '{"statusCode":404,"error":"Not Found","message":"Pool not found"}\n');
    return true;
  });
}

async function batchReadsServiceCacheAndDatabaseAndOmitsMissingPools() {
  await command('clear-cache');
  await client.getPoolStats(contract.pool);
  const pools = [contract.otherPool, contract.missingPool, contract.pool, contract.otherPool];
  assert.deepEqual(await client.getPoolStatsBatch(pools), contract.rows);
  assert.deepEqual(await client.getPoolStatsBatch(pools), contract.rows);
  assert.deepEqual(await client.getPoolStatsBatch([contract.missingPool]), {});
}

async function serviceDatabaseErrorsRetainStatusAndEnvelope() {
  await command('database-offline');
  for (const request of [() => client.getPoolStats(contract.pool), () => client.getPoolStatsBatch([contract.pool])]) {
    await assert.rejects(request(), error => {
      assert.ok(error instanceof StatsHttpError);
      assert.equal(error.status, 500);
      assert.equal(error.body, '{"statusCode":500,"error":"Internal Server Error","message":"An unexpected error occurred"}\n');
      return true;
    });
  }
}

test(singlePoolReadsServiceRowsAndNotFound);
test(batchReadsServiceCacheAndDatabaseAndOmitsMissingPools);
test(serviceDatabaseErrorsRetainStatusAndEnvelope);

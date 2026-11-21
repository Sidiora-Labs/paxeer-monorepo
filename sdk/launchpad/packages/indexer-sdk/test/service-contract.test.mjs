import { after, before, test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { copyFile, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { HttpClient, HttpError, getRecentTrades, httpClient } from '../dist/index.js';

const root = fileURLToPath(new URL('../../../../../', import.meta.url));
const service = join(root, 'services/launchpad/stats-workers');
const pool = `0x${'a'.repeat(40)}`;
const mixedPool = `0x${'aA'.repeat(20)}`;
const route = `/stats/${pool}/transactions`;
const client = new HttpClient();
let directory;
let producer;
let baseUrl;
let databaseClosed;

before(async function openTransactionsProducer() {
  directory = await mkdtemp(join(tmpdir(), 'indexer-sdk-transactions-'));
  let manifest = await readFile(join(service, 'go.mod'), 'utf8');
  manifest = manifest.replace(/^module (.+)$/m, 'module $1/indexer-sdk-contract');
  manifest = manifest.replaceAll('=> ../../../sdk/launchpad/shared', `=> ${JSON.stringify(join(root, 'sdk/launchpad/shared'))}`);
  manifest = manifest.replaceAll('=> ../../../sdk/launchpad/protocol', `=> ${JSON.stringify(join(root, 'sdk/launchpad/protocol'))}`);
  manifest += '\nrequire github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers v0.0.0\n';
  manifest += `replace github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers => ${JSON.stringify(service)}\n`;
  await writeFile(join(directory, 'go.mod'), manifest);
  await copyFile(join(service, 'go.sum'), join(directory, 'go.sum'));
  await copyFile(new URL('./transactions-producer.go', import.meta.url), join(directory, 'main.go'));
  producer = spawn('go', ['run', '-mod=mod', '.'], {
    cwd: directory, detached: true, stdio: ['pipe', 'pipe', 'pipe'],
    env: { ...process.env, GOWORK: 'off' },
  });
  let diagnostics = '';
  producer.stderr.setEncoding('utf8').on('data', chunk => { diagnostics += chunk; });
  const lines = createInterface({ input: producer.stdout });
  baseUrl = await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`Transactions producer startup timed out: ${diagnostics}`)), 540000);
    producer.once('error', error => { clearTimeout(timer); reject(error); });
    producer.once('close', code => {
      clearTimeout(timer);
      reject(new Error(`Transactions producer exited ${code}: ${diagnostics}`));
    });
    lines.once('line', line => {
      clearTimeout(timer);
      if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(line)) {
        reject(new Error(`Unexpected transactions producer address: ${line}`));
        return;
      }
      databaseClosed = once(lines, 'line');
      resolve(line);
    });
  });
  client.configure({ baseUrl });
  httpClient.configure({ baseUrl });
}, { timeout: 600000 });

after(async function closeTransactionsProducer() {
  if (producer && producer.exitCode === null && producer.signalCode === null) {
    const closed = once(producer, 'close');
    producer.stdin.end();
    const [code] = await closed;
    assert.equal(code, 0);
  }
  if (directory) await rm(directory, { recursive: true, force: true });
});

function transaction(index) {
  const txHash = `0x${(index + 1).toString(16).padStart(64, '0')}`;
  return {
    id: `${txHash}-0`, poolAddress: pool, sender: `0x${'b'.repeat(40)}`, isBuy: index % 2 === 0,
    amountIn: '999999999999999999999999999999999999', amountOut: '1000000000000000001',
    price: '0.000000000000000001', fee: '1000001', blockTimestamp: 1700000000 + Math.floor(index / 2), txHash,
  };
}

async function recentTradesReadsEveryServiceField() {
  const expected = Array.from({ length: 50 }, (_, index) => transaction(51 - index));
  const page = await client.get(route);
  assert.deepEqual(page, { transactions: expected, limit: 50, offset: 0 });
  assert.deepEqual(await client.recentTrades(mixedPool), { pool: mixedPool, trades: expected });
  assert.deepEqual(await getRecentTrades(pool), { pool, trades: expected });
  const emptyPool = `0x${'c'.repeat(40)}`;
  assert.deepEqual(await client.recentTrades(emptyPool), { pool: emptyPool, trades: [] });
  assert.deepEqual(await getRecentTrades(emptyPool), { pool: emptyPool, trades: [] });
}
test(recentTradesReadsEveryServiceField);

async function getReadsServicePagingAndSideFilters() {
  assert.deepEqual(await client.get(`${route}?limit=1&offset=1&type=buy`), {
    transactions: [transaction(48)], limit: 1, offset: 1,
  });
  assert.deepEqual(await client.get(`${route}?limit=500&type=sell`), {
    transactions: Array.from({ length: 26 }, (_, index) => transaction(51 - index * 2)), limit: 500, offset: 0,
  });
  assert.deepEqual(await client.get(`${route}?limit=2&offset=50&type=all`), {
    transactions: [transaction(1), transaction(0)], limit: 2, offset: 50,
  });
  assert.deepEqual(await client.get(`${route}?offset=10000`), { transactions: [], limit: 50, offset: 10000 });
}
test(getReadsServicePagingAndSideFilters);

async function httpErrorsPreserveServiceStatusAndBody() {
  for (const query of ['limit=0', 'limit=501', 'offset=-1', 'offset=10001']) {
    await assert.rejects(client.get(`${route}?${query}`), error => {
      assert.ok(error instanceof HttpError);
      assert.equal(error.status, 400);
      assert.equal(error.url, `${baseUrl}${route}?${query}`);
      assert.deepEqual(JSON.parse(error.body), {
        statusCode: 400, error: 'Bad Request', message: 'pagination outside bounds; use a cursor for deep history',
      });
      return true;
    });
  }
  for (const query of ['limit=', 'limit=1&limit=2', 'offset=']) {
    await assert.rejects(client.get(`${route}?${query}`), error => {
      assert.ok(error instanceof HttpError);
      assert.equal(error.status, 400);
      assert.deepEqual(JSON.parse(error.body), { statusCode: 400, error: 'Bad Request', message: 'invalid pagination' });
      return true;
    });
  }
  const cursor = Buffer.from(JSON.stringify({
    v: 1, scope: `transactions:${pool}:`, after: { timestamp: 1700000000, id: transaction(0).id },
  })).toString('base64url');
  await assert.rejects(client.get(`${route}?cursor=${cursor}`), error => {
    assert.ok(error instanceof HttpError);
    assert.equal(error.status, 400);
    assert.deepEqual(JSON.parse(error.body), {
      statusCode: 400, error: 'Bad Request', message: 'cursor is not supported on this route',
    });
    return true;
  });
  await assert.rejects(client.get('/bff/recent-trades'), error => {
    assert.ok(error instanceof HttpError);
    assert.equal(error.status, 404);
    assert.equal(error.body, '404 page not found\n');
    return true;
  });
  producer.stdin.write('close database\n');
  assert.deepEqual(await databaseClosed, ['database closed']);
  for (const operation of [() => client.get(route), () => client.recentTrades(pool), () => getRecentTrades(pool)]) {
    await assert.rejects(operation, error => {
      assert.ok(error instanceof HttpError);
      assert.equal(error.status, 500);
      assert.equal(error.url, baseUrl + route);
      assert.deepEqual(JSON.parse(error.body), {
        statusCode: 500, error: 'Internal Server Error', message: 'An unexpected error occurred',
      });
      return true;
    });
  }
}
test(httpErrorsPreserveServiceStatusAndBody);

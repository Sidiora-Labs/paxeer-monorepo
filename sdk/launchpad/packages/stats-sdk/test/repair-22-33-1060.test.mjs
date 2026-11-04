import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { gzipSync } from 'node:zlib';
import { StatsHttpClient, StatsHttpError, StatsResponseError } from '../dist/index.js';
import { poolStatsFields } from '../dist/types.js';

// Task 22.33.1060, finding 37: repair37OversizedSingle, repair37OversizedBatch,
// repair37OversizedError prove refusal and transport cancellation before EOF.
// Baseline was not run: the ordered gate instructions require implementation
// before any testing. The old response.text() path would consume EOF and either
// return the oversized valid DTO or propagate the full oversized HTTP error.
// Wiring requests: none; the repair and proof are entirely within this task.
const pool = '0x' + 'a'.repeat(40);
const otherPool = '0x' + 'b'.repeat(40);
const singleLimit = 64 * 1024;
const batchLimit = 1024 * 1024;
const errorLimit = 16 * 1024;
const fixture = Object.fromEntries(Object.entries(poolStatsFields).map(([key, kind]) => [
  key, kind === 'number' ? 0 : kind === 'string | null' ? null : '900719925474099300000',
]));
fixture.poolAddress = pool;
fixture.tokenAddress = otherPool;

async function serve(handler, run) {
  const server = createServer(handler);
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const baseUrl = `http://127.0.0.1:${server.address().port}`;
  const client = new StatsHttpClient();
  client.configure({ baseUrl });
  try { await run(client, baseUrl); }
  finally {
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
  }
}

function oversizedError(status, limitBytes) {
  return error => {
    assert.ok(error instanceof StatsResponseError);
    assert.equal(error.name, 'StatsResponseTooLargeError');
    assert.equal(error.status, status);
    assert.equal(error.limitBytes, limitBytes);
    assert.match(error.message, /response exceeds/);
    return true;
  };
}

async function proveOversizedCancellation(status, batch, limitBytes) {
  const payloadBytes = 32 * 1024 * 1024;
  const chunk = Buffer.alloc(16 * 1024, 'x');
  let sent = 0;
  let ended = false;
  let sawRequest = false;
  let resolveClosed;
  const closed = new Promise(resolve => { resolveClosed = resolve; });
  await serve((request, response) => {
    sawRequest = true;
    assert.equal(request.headers.accept, 'application/json');
    assert.equal(request.url, batch ? `/stats/batch?pools=${pool}` : `/stats/${pool}`);
    response.on('close', resolveClosed);
    response.writeHead(status, { 'Content-Type': 'application/json' });
    const { riskFactors, ...fields } = fixture;
    const prefix = JSON.stringify({ ...fields, riskFactors: '' }).slice(0, -2);
    response.write((batch ? `{"${pool}":` : '') + prefix);
    function pump() {
      if (response.destroyed) return;
      if (sent === payloadBytes) {
        ended = true;
        response.end(batch ? '"}}' : '"}');
        return;
      }
      sent += chunk.length;
      if (response.write(chunk)) setImmediate(pump);
      else response.once('drain', () => setImmediate(pump));
    }
    setImmediate(pump);
  }, async client => {
    const result = batch ? client.getPoolStatsBatch([pool]) : client.getPoolStats(pool);
    await assert.rejects(result, oversizedError(status, limitBytes));
    await closed;
    assert.equal(sawRequest, true);
    assert.ok(sent >= limitBytes);
    assert.ok(sent < payloadBytes, `producer sent the entire ${sent}-byte body`);
    assert.equal(ended, false, 'the transport must close before the producer reaches EOF');
  });
}

async function repair37OversizedSingle() {
  await proveOversizedCancellation(200, false, singleLimit);
}
async function repair37OversizedBatch() {
  await proveOversizedCancellation(200, true, batchLimit);
}
async function repair37OversizedError() {
  await proveOversizedCancellation(503, false, errorLimit);
}

function paddedStats(size, batch = false) {
  const wrap = row => batch ? { [pool]: row } : row;
  const empty = JSON.stringify(wrap({ ...fixture, riskFactors: '' }));
  const paddingBytes = size - Buffer.byteLength(empty);
  const padding = '€'.repeat(Math.floor(paddingBytes / 3)) + 'x'.repeat(paddingBytes % 3);
  const row = { ...fixture, riskFactors: padding };
  const body = Buffer.from(JSON.stringify(wrap(row)));
  assert.equal(body.byteLength, size);
  return { body, row };
}

async function repair37ExactByteBoundariesAndUtf8() {
  for (const [batch, limit] of [[false, singleLimit], [true, batchLimit]]) {
    const { body, row } = paddedStats(limit, batch);
    const split = body.indexOf(Buffer.from('€')) + 1;
    await serve((_request, response) => {
      response.writeHead(200, { 'Content-Length': body.length });
      response.write(body.subarray(0, split));
      setImmediate(() => response.end(body.subarray(split)));
    }, async client => {
      const actual = batch ? await client.getPoolStatsBatch([pool]) : await client.getPoolStats(pool);
      assert.deepEqual(actual, batch ? { [pool]: row } : row);
    });
    const oversized = paddedStats(limit + 1, batch).body;
    assert.ok(oversized.toString().length < limit, 'fixture must distinguish UTF-8 bytes from characters');
    await serve((_request, response) => response.end(oversized), async client => {
      await assert.rejects(batch ? client.getPoolStatsBatch([pool]) : client.getPoolStats(pool), oversizedError(200, limit));
    });
  }
}

async function repair37CompressedResponseCountsDecodedBytes() {
  const body = gzipSync(paddedStats(singleLimit + 1).body);
  assert.ok(body.length < singleLimit);
  await serve((_request, response) => {
    response.writeHead(200, { 'Content-Encoding': 'gzip', 'Content-Length': body.length });
    response.end(body);
  }, async client => {
    await assert.rejects(client.getPoolStats(pool), oversizedError(200, singleLimit));
  });
}

async function repair37PreservesHttpStatusAndBody() {
  for (const [status, body] of [[429, 'slow down'], [503, '€'.repeat(errorLimit / 4) + 'x'.repeat(errorLimit / 4)], [404, '']]) {
    await serve((_request, response) => {
      response.writeHead(status);
      response.end(body);
    }, async client => {
      await assert.rejects(client.getPoolStats(pool), error => {
        assert.ok(error instanceof StatsHttpError);
        assert.equal(error.status, status);
        assert.equal(error.body, body);
        return true;
      });
    });
  }
}

async function repair37PreservesDtoValidation() {
  const missing = { ...fixture };
  delete missing.price;
  const bodies = ['{', 'null', '[]', JSON.stringify(missing),
    JSON.stringify({ ...fixture, price: 1 }),
    JSON.stringify({ ...fixture, buyCount24h: Number.MAX_SAFE_INTEGER + 1 }),
    JSON.stringify({ ...fixture, riskFactors: false })];
  for (const body of bodies) {
    for (const batch of [false, true]) {
      await serve((_request, response) => response.end(batch ? `{"${pool}":${body}}` : body), async client => {
        await assert.rejects(batch ? client.getPoolStatsBatch([pool]) : client.getPoolStats(pool), StatsResponseError);
      });
    }
  }
  await serve((request, response) => {
    const batch = request.url.startsWith('/stats/batch?');
    response.end(JSON.stringify(batch ? { [pool]: fixture } : fixture));
  }, async client => {
    assert.deepEqual(await client.getPoolStats(pool), fixture);
    assert.deepEqual(await client.getPoolStatsBatch([pool, otherPool]), { [pool]: fixture });
  });
}

async function repair37PreservesDeadlineDuringBodyRead() {
  let received = false;
  await serve((_request, response) => {
    received = true;
    response.writeHead(200);
    response.write('{');
  }, async (client, baseUrl) => {
    client.configure({ baseUrl, timeoutMs: 200 });
    await assert.rejects(client.getPoolStats(pool), error => {
      assert.ok(error instanceof DOMException);
      assert.ok(['TimeoutError', 'AbortError'].includes(error.name));
      return true;
    });
    assert.equal(received, true);
  });
}

async function repair37PreservesCallerAbortDuringBodyRead() {
  const controller = new AbortController();
  let resolveReceived;
  const received = new Promise(resolve => { resolveReceived = resolve; });
  await serve((_request, response) => {
    response.writeHead(200);
    response.write('{');
    resolveReceived();
  }, async client => {
    const result = client.getPoolStats(pool, { signal: controller.signal });
    const rejection = assert.rejects(result, error => {
      assert.ok(error instanceof DOMException);
      assert.equal(error.name, 'AbortError');
      return true;
    });
    await received;
    controller.abort();
    await rejection;
    const alreadyAborted = new AbortController();
    alreadyAborted.abort();
    await assert.rejects(client.getPoolStats(pool, { signal: alreadyAborted.signal }), error => error.name === 'AbortError');
  });
}

test(repair37OversizedSingle);
test(repair37OversizedBatch);
test(repair37OversizedError);
test(repair37ExactByteBoundariesAndUtf8);
test(repair37CompressedResponseCountsDecodedBytes);
test(repair37PreservesHttpStatusAndBody);
test(repair37PreservesDtoValidation);
test(repair37PreservesDeadlineDuringBodyRead);
test(repair37PreservesCallerAbortDuringBodyRead);

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { getEventListeners } from 'node:events';
import { HttpClient, HttpError, httpClient, getRecentTrades } from '../dist/index.js';

// Task 22.33.1055 closes audit 22.30.41 finding 30 with the Repair30 tests below.
// Baseline not executed: the task's implementation-first, gate-only order takes
// precedence over its contradictory request for an unchanged-tree verification.
// No production wiring outside this task's paths is required.
const pool = `0x${'1'.repeat(40)}`;
const route = `/stats/${pool}/transactions`;
const successLimit = 1024 * 1024;
const errorLimit = 16 * 1024;
const trade = {
  id: '1', sender: pool, isBuy: true,
  amountIn: '999999999999999999999999999999999999', amountOut: '1',
  price: '0.0000001', fee: '1', blockTimestamp: 123, txHash: '0x123',
};

async function withHttp(handler, run) {
  const requests = [];
  const server = createServer((req, res) => {
    const { promise: closed, resolve } = Promise.withResolvers();
    res.once('close', resolve);
    requests.push({ url: req.url, method: req.method, headers: req.headers, closed });
    handler(req, res);
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  let watchdogFired = false;
  const watchdog = setTimeout(() => {
    watchdogFired = true;
    server.closeAllConnections();
  }, 15_000);
  try {
    await run(`http://127.0.0.1:${server.address().port}`, requests);
    assert.equal(watchdogFired, false, 'requests must finish before the fixture watchdog');
  } finally {
    clearTimeout(watchdog);
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
  }
}

function configuredClient(baseUrl, timeoutMs = 250) {
  const client = new HttpClient();
  client.configure({ baseUrl, timeoutMs });
  return client;
}

async function Repair30DefaultDeadlineBoundsStalledHeaders() {
  await withHttp(() => {}, async (baseUrl, requests) => {
    const client = new HttpClient();
    client.configure({ baseUrl });
    await assert.rejects(client.recentTrades(pool), { name: 'TimeoutError' });
    assert.equal(requests.length, 1);
    assert.equal(requests[0].url, route);
    await requests[0].closed;
  });
}

async function Repair30ConfiguredDeadlineBoundsHeadersAndStreamingBodies() {
  for (const mode of ['headers', 'body', 'error', 'drip']) {
    await withHttp((_req, res) => {
      if (mode === 'headers') return;
      res.writeHead(mode === 'error' ? 503 : 200, { 'Content-Type': 'application/json' });
      res.flushHeaders();
      if (mode === 'drip') {
        res.write('{');
        const interval = setInterval(() => res.write(' '), 10);
        res.once('close', () => clearInterval(interval));
      }
    }, async (baseUrl, requests) => {
      const controller = new AbortController();
      const before = getEventListeners(controller.signal, 'abort');
      const start = performance.now();
      await assert.rejects(configuredClient(baseUrl).recentTrades(pool, { signal: controller.signal }), { name: 'TimeoutError' });
      assert.ok(performance.now() - start < 2_000, `${mode} must obey the configured total deadline`);
      assert.equal(controller.signal.aborted, false, 'the internal deadline must not abort the caller');
      assert.deepEqual(getEventListeners(controller.signal, 'abort'), before);
      assert.equal(requests.length, 1);
      await requests[0].closed;
    });
  }
}

async function Repair30CallerCancellationCombinesWithDeadlineAndCleansListeners() {
  for (const mode of ['headers', 'body']) {
    const controller = new AbortController();
    const reason = new Error(`caller stopped ${mode}`);
    const unrelatedListener = () => {};
    controller.signal.addEventListener('abort', unrelatedListener);
    const before = getEventListeners(controller.signal, 'abort');
    await withHttp((_req, res) => {
      if (mode === 'body') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.write('{');
      }
      const timer = setTimeout(() => controller.abort(reason), 50);
      res.once('close', () => clearTimeout(timer));
    }, async (baseUrl, requests) => {
      await assert.rejects(configuredClient(baseUrl, 2_000).recentTrades(pool, { signal: controller.signal }), error => error === reason);
      assert.deepEqual(getEventListeners(controller.signal, 'abort'), before);
      await requests[0].closed;
    });
    controller.signal.removeEventListener('abort', unrelatedListener);
  }
  await withHttp((_req, res) => res.end('{}'), async (baseUrl, requests) => {
    const controller = new AbortController();
    controller.abort();
    await assert.rejects(configuredClient(baseUrl).recentTrades(pool, { signal: controller.signal }), error => error === controller.signal.reason);
    assert.equal(requests.length, 0, 'pre-cancelled requests must not reach HTTP');
    assert.equal(getEventListeners(controller.signal, 'abort').length, 0);
  });
}

async function Repair30OversizedSuccessAndErrorStreamsAreRefusedAndClosed() {
  for (const status of [200, 503]) {
    const limit = status === 200 ? successLimit : errorLimit;
    let sent = 0;
    const total = limit * 16;
    await withHttp((_req, res) => {
      res.writeHead(status, { 'Content-Type': 'application/json' });
      // Chunked and never completed before refusal: no Content-Length shortcut
      // or full-body JSON/text buffering can satisfy this proof.
      const interval = setInterval(() => {
        res.write(Buffer.alloc(8 * 1024, 0x20));
        sent += 8 * 1024;
        if (sent >= total) {
          clearInterval(interval);
          res.end();
        }
      }, 1);
      res.once('close', () => clearInterval(interval));
    }, async (baseUrl, requests) => {
      const controller = new AbortController();
      const { ResponseTooLargeError } = await import('../dist/index.js');
      assert.equal(typeof ResponseTooLargeError, 'function');
      await assert.rejects(configuredClient(baseUrl, 5_000).recentTrades(pool, { signal: controller.signal }), error => {
        assert.ok(error instanceof ResponseTooLargeError);
        assert.equal(error.name, 'ResponseTooLargeError');
        assert.equal(error.status, status);
        assert.equal(error.limitBytes, limit);
        assert.equal(error.url, baseUrl + route);
        return true;
      });
      await requests[0].closed;
      assert.ok(sent > limit);
      assert.ok(sent < total, 'the server must stop before the full oversized body is sent');
      assert.equal(controller.signal.aborted, false);
      assert.equal(getEventListeners(controller.signal, 'abort').length, 0);
    });
  }
}

async function Repair30ByteBoundariesAndUtf8DecodingArePreserved() {
  let payload;
  let status = 200;
  await withHttp((_req, res) => {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    // Split multibyte characters across network writes.
    const bytes = Buffer.from(payload);
    const split = bytes.indexOf(Buffer.from('é')) + 1;
    res.write(bytes.subarray(0, split));
    const timer = setTimeout(() => res.end(bytes.subarray(split)), 10);
    res.once('close', () => clearTimeout(timer));
  }, async baseUrl => {
    const client = configuredClient(baseUrl, 2_000);
    const controller = new AbortController();
    const options = { signal: controller.signal };
    payload = JSON.stringify({ transactions: [{ ...trade, id: 'é' }] });
    payload += ' '.repeat(successLimit - Buffer.byteLength(payload));
    assert.equal(Buffer.byteLength(payload), successLimit);
    assert.deepEqual(await client.recentTrades(pool, options), { pool, trades: [{ ...trade, id: 'é' }] });
    status = 502;
    payload = 'é'.repeat(errorLimit / 2);
    await assert.rejects(client.get('/unavailable', options), error => {
      assert.ok(error instanceof HttpError);
      assert.equal(error.status, 502);
      assert.equal(error.body, payload);
      assert.equal(error.url, baseUrl + '/unavailable');
      return true;
    });
    payload += 'é';
    const { ResponseTooLargeError } = await import('../dist/index.js');
    await assert.rejects(client.get('/unavailable', options), error => error instanceof ResponseTooLargeError && error.limitBytes === errorLimit);
    assert.equal(getEventListeners(controller.signal, 'abort').length, 0);
  });
}

async function Repair30PreservesHttpErrorsShapesAndListenerCleanupAcrossRequests() {
  let body = { transactions: [trade], limit: 50, offset: 0 };
  let status = 200;
  let raw = false;
  await withHttp((_req, res) => {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    res.end(raw ? body : JSON.stringify(body));
  }, async (baseUrl, requests) => {
    const client = configuredClient(baseUrl, 2_000);
    const controller = new AbortController();
    const unrelatedListener = () => {};
    controller.signal.addEventListener('abort', unrelatedListener);
    const before = getEventListeners(controller.signal, 'abort');
    const options = { signal: controller.signal };
    assert.deepEqual(await client.recentTrades(pool, options), { pool, trades: [trade] });
    httpClient.configure({ baseUrl, timeoutMs: 2_000, headers: { 'X-Indexer-Test': 'consumer' } });
    assert.deepEqual(await getRecentTrades(pool, options), { pool, trades: [trade] });
    assert.equal(requests[1].headers['x-indexer-test'], 'consumer');
    assert.equal(requests[1].headers.accept, 'application/json');
    body = { transactions: [] };
    assert.deepEqual(await client.recentTrades(pool, options), { pool, trades: [] });
    for (const invalid of [
      null, { pool, trades: [trade] }, { transactions: null }, { transactions: {} },
      { transactions: [null] },
      ...Object.keys(trade).map(key => ({ transactions: [{ ...trade, [key]: null }] })),
      { transactions: [{ ...trade, amountIn: 123 }] },
      { transactions: [{ ...trade, blockTimestamp: 1.5 }] },
      { transactions: [{ ...trade, blockTimestamp: Number.MAX_SAFE_INTEGER + 1 }] },
    ]) {
      body = invalid;
      await assert.rejects(client.recentTrades(pool, options), /Invalid recent trades response/);
      assert.deepEqual(getEventListeners(controller.signal, 'abort'), before);
    }
    raw = true;
    body = '{invalid';
    await assert.rejects(client.recentTrades(pool, options), SyntaxError);
    assert.deepEqual(getEventListeners(controller.signal, 'abort'), before);
    status = 503;
    body = 'database unavailable';
    await assert.rejects(client.recentTrades(pool, options), error => error instanceof HttpError && error.status === 503 && error.body === body && error.url === baseUrl + route);
    assert.deepEqual(getEventListeners(controller.signal, 'abort'), before);
    for (const timeoutMs of [0, -1, 0.5, NaN, Infinity, 120_001]) {
      assert.throws(() => client.configure({ baseUrl, timeoutMs }), /timeoutMs/);
    }
    const count = requests.length;
    await assert.rejects(client.recentTrades('invalid', options), /Invalid pool/);
    await assert.rejects(client.get('//other-host', options), /resource path/);
    assert.equal(requests.length, count);
    assert.deepEqual(getEventListeners(controller.signal, 'abort'), before);
    controller.signal.removeEventListener('abort', unrelatedListener);
  });
}

test(Repair30DefaultDeadlineBoundsStalledHeaders);
test(Repair30ConfiguredDeadlineBoundsHeadersAndStreamingBodies);
test(Repair30CallerCancellationCombinesWithDeadlineAndCleansListeners);
test(Repair30OversizedSuccessAndErrorStreamsAreRefusedAndClosed);
test(Repair30ByteBoundariesAndUtf8DecodingArePreserved);
test(Repair30PreservesHttpErrorsShapesAndListenerCleanupAcrossRequests);

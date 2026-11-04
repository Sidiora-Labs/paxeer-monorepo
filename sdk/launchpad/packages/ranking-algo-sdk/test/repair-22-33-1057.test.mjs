// Finding 32: repair32DefaultDeadline, repair32StalledHeaders,
// repair32StalledBodies, repair32CallerCancellation, repair32StreamedByteCaps,
// repair32DeclaredByteCaps, and repair32ResponsesAndCleanup prove the repair.
// Baseline verification was not run: the ordered gate rules require complete
// implementation before qualification and prohibit a separate verify run.
// No out-of-path wiring is required.
import assert from 'node:assert/strict';
import { once, getEventListeners } from 'node:events';
import { createServer } from 'node:http';
import test from 'node:test';
import { gzipSync } from 'node:zlib';
import {
  DEFAULT_RANKING_TIMEOUT_MS,
  MAX_RANKING_TIMEOUT_MS,
  MAX_RANKINGS_RESPONSE_BYTES,
  MAX_RANKING_ERROR_RESPONSE_BYTES,
  RankingHttpClient,
  RankingHttpError,
  RankingResponseTooLargeError,
} from '../dist/index.js';

function rankingFixture() {
  const computedAt = '2026-01-01T00:00:00.000Z';
  const watermark = {
    chainId: '125', height: '100', blockHash: `0x${'ab'.repeat(32)}`,
    projectionRevision: '1', observedAt: computedAt, integrity: 'healthy',
  };
  return {
    data: { snapshot: {
      snapshotId: '00000000-0000-4000-8000-000000000001', category: 'trending',
      modelVersion: 'v1', computedAt, watermark,
      items: [{ poolAddress: `0x${'12'.repeat(20)}`, category: 'trending',
        score: '0.000000000000000001', rank: 1, modelVersion: 'v1', computedAt }],
      totalEligible: '1', truncated: false, replace: true,
    } },
    requestId: '00000000-0000-4000-8000-000000000002', generatedAt: computedAt,
    nextCursor: null, revision: '1', watermark, quality: 'complete',
  };
}

function errorFixture() {
  return {
    code: 'DEPENDENCY_UNAVAILABLE', message: 'Ranking source unavailable',
    requestId: '00000000-0000-4000-8000-000000000003', retryable: true,
    retryAfterMs: 1000, fieldErrors: [], details: { dependency: 'ranking' },
  };
}

async function withinDeadline(promise, ms = 2000) {
  let timer;
  try {
    return await Promise.race([
      promise,
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error('HTTP proof exceeded its deadline')), ms);
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

async function loopback(t, handler, config = {}) {
  const server = createServer(handler);
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(async () => {
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  });
  const client = new RankingHttpClient();
  client.configure({ baseUrl: `http://127.0.0.1:${server.address().port}`, ...config });
  return client;
}

function assertNoListeners(signal) {
  assert.equal(getEventListeners(signal, 'abort').length, 0, 'caller abort listener was removed');
}

function isOversize(status, limit) {
  return error => {
    assert.ok(error instanceof RankingResponseTooLargeError);
    assert.ok(error instanceof RangeError);
    assert.equal(error.status, status);
    assert.equal(error.limit, limit);
    return true;
  };
}

async function repair32DefaultDeadline(t) {
  let received = false;
  const client = await loopback(t, () => { received = true; });
  assert.equal(DEFAULT_RANKING_TIMEOUT_MS, 10_000);
  await assert.rejects(
    withinDeadline(client.getRankings('trending'), DEFAULT_RANKING_TIMEOUT_MS + 3000),
    { name: 'TimeoutError', message: 'Ranking request deadline exceeded' },
  );
  assert.equal(received, true);
}

async function repair32StalledHeaders(t) {
  let received = false;
  let disconnected;
  const closed = new Promise(resolve => { disconnected = resolve; });
  const client = await loopback(t, (req, res) => {
    received = true;
    res.on('close', disconnected);
  }, { timeoutMs: 200 });
  const caller = new AbortController();
  await assert.rejects(withinDeadline(client.getRankings('trending', { signal: caller.signal })), { name: 'TimeoutError' });
  assert.equal(received, true);
  assert.equal(caller.signal.aborted, false);
  assertNoListeners(caller.signal);
  await withinDeadline(closed);
}

async function repair32StalledBodies(t) {
  for (const status of [200, 503]) {
    for (const trickle of [false, true]) {
      await t.test(`status ${status}, trickle ${trickle}`, async t => {
        let disconnected;
        const closed = new Promise(resolve => { disconnected = resolve; });
        const client = await loopback(t, (req, res) => {
          res.writeHead(status, { 'Content-Type': 'application/json' });
          res.flushHeaders();
          res.write(' ');
          const timer = trickle ? setInterval(() => res.write(' '), 25) : undefined;
          res.on('close', () => { clearInterval(timer); disconnected(); });
        }, { timeoutMs: 200 });
        const caller = new AbortController();
        await assert.rejects(withinDeadline(client.getRankings('trending', { signal: caller.signal })), { name: 'TimeoutError' });
        assertNoListeners(caller.signal);
        assert.equal(caller.signal.aborted, false);
        await withinDeadline(closed);
      });
    }
  }
}

async function repair32CallerCancellation(t) {
  for (const phase of ['headers', 'body']) {
    await t.test(phase, async t => {
      let ready;
      const received = new Promise(resolve => { ready = resolve; });
      let disconnected;
      const closed = new Promise(resolve => { disconnected = resolve; });
      const client = await loopback(t, (req, res) => {
        if (phase === 'body') { res.writeHead(200); res.write('{'); }
        res.on('close', disconnected);
        ready();
      }, { timeoutMs: 1000 });
      const caller = new AbortController();
      const reason = new DOMException(`Caller stopped ${phase}`, 'AbortError');
      const pending = client.getRankings('trending', { signal: caller.signal });
      const rejected = assert.rejects(withinDeadline(pending), error => error === reason);
      await withinDeadline(received);
      assert.equal(getEventListeners(caller.signal, 'abort').length, 1);
      caller.abort(reason);
      await rejected;
      assertNoListeners(caller.signal);
      await withinDeadline(closed);
    });
  }
  let requests = 0;
  const client = await loopback(t, (req, res) => { requests++; res.end('{}'); });
  const caller = new AbortController();
  const reason = new Error('Already stopped');
  caller.abort(reason);
  await assert.rejects(client.getRankings('trending', { signal: caller.signal }), error => error === reason);
  assert.equal(requests, 0);
  assertNoListeners(caller.signal);
}

async function repair32StreamedByteCaps(t) {
  for (const status of [200, 503]) {
    for (const compressed of [false, true]) {
      await t.test(`status ${status}, gzip ${compressed}`, async t => {
        const limit = status === 200 ? MAX_RANKINGS_RESPONSE_BYTES : MAX_RANKING_ERROR_RESPONSE_BYTES;
        // Multibyte text is fewer than limit characters but more than limit bytes.
        const bytes = Buffer.from('☃'.repeat(Math.floor(limit / 3) + 1));
        assert.ok(bytes.toString().length < limit);
        assert.ok(bytes.length > limit);
        let disconnected;
        const closed = new Promise(resolve => { disconnected = resolve; });
        const client = await loopback(t, (req, res) => {
          res.writeHead(status, compressed ? { 'Content-Encoding': 'gzip' } : {});
          res.on('close', disconnected);
          const payload = compressed ? gzipSync(bytes) : bytes;
          // Leave the transport open: the SDK must refuse during consumption.
          for (let offset = 0; offset < payload.length; offset += 4093) {
            res.write(payload.subarray(offset, offset + 4093));
          }
        }, { timeoutMs: 1000 });
        const caller = new AbortController();
        await assert.rejects(withinDeadline(client.getRankings('trending', { signal: caller.signal })), isOversize(status, limit));
        assertNoListeners(caller.signal);
        await withinDeadline(closed);
      });
    }
  }
}

async function repair32DeclaredByteCaps(t) {
  for (const status of [200, 503]) {
    await t.test(`status ${status}`, async t => {
      const limit = status === 200 ? MAX_RANKINGS_RESPONSE_BYTES : MAX_RANKING_ERROR_RESPONSE_BYTES;
      const client = await loopback(t, (req, res) => {
        res.writeHead(status, { 'Content-Length': String(limit + 1) });
        res.flushHeaders();
      }, { timeoutMs: 1000 });
      const caller = new AbortController();
      await assert.rejects(withinDeadline(client.getRankings('trending', { signal: caller.signal })), isOversize(status, limit));
      assertNoListeners(caller.signal);
    });
  }
}

async function repair32ResponsesAndCleanup(t) {
  const expected = rankingFixture();
  const typedError = errorFixture();
  const requests = [];
  const client = await loopback(t, (req, res) => {
    requests.push({ url: req.url, accept: req.headers.accept, authorization: req.headers.authorization });
    if (req.url.startsWith('/rankings/unusual')) {
      res.writeHead(503, { 'Retry-After': '1' });
      const text = JSON.stringify(typedError);
      res.end(text + ' '.repeat(MAX_RANKING_ERROR_RESPONSE_BYTES - Buffer.byteLength(text)));
    } else if (req.url.startsWith('/rankings/movers')) {
      res.writeHead(502); res.end('unavailable');
    } else if (req.url.startsWith('/rankings/losers')) {
      res.writeHead(400); res.end(JSON.stringify({ ...typedError, retryable: 'invalid' }));
    } else if (req.url.startsWith('/rankings/new')) {
      res.end('{');
    } else if (req.url.startsWith('/rankings/volume')) {
      res.end(JSON.stringify({ ...expected, quality: 'untrusted' }));
    } else if (req.url.startsWith('/rankings/gainers')) {
      res.end(JSON.stringify({ ...expected, data: { snapshot: { ...expected.data.snapshot, category: 'trending' } } }));
    } else {
      const text = JSON.stringify(expected);
      res.end(text + ' '.repeat(MAX_RANKINGS_RESPONSE_BYTES - Buffer.byteLength(text)));
    }
  }, { timeoutMs: 1000, headers: { Authorization: 'Bearer loopback-fixture' } });
  const caller = new AbortController();
  for (let i = 0; i < 3; i++) {
    assert.deepEqual(await client.getRankings('trending', { limit: 1, cursor: 'next+page', signal: caller.signal }), expected);
    assertNoListeners(caller.signal);
    await assert.rejects(client.getRankings('unusual', { signal: caller.signal }), error => {
      assert.ok(error instanceof RankingHttpError);
      assert.equal(error.status, 503);
      assert.equal(error.retryAfter, '1');
      assert.equal(Buffer.byteLength(error.body), MAX_RANKING_ERROR_RESPONSE_BYTES);
      assert.deepEqual(error.error, typedError);
      return true;
    });
    assertNoListeners(caller.signal);
  }
  assert.deepEqual(requests[0], {
    url: '/rankings/trending?limit=1&cursor=next%2Bpage', accept: 'application/json', authorization: 'Bearer loopback-fixture',
  });
  await assert.rejects(client.getRankings('movers', { signal: caller.signal }), error => {
    assert.ok(error instanceof RankingHttpError);
    assert.equal(error.body, 'unavailable');
    assert.equal(error.error, undefined);
    return true;
  });
  assertNoListeners(caller.signal);
  await assert.rejects(client.getRankings('losers', { signal: caller.signal }), error => {
    assert.ok(error instanceof RankingHttpError);
    assert.equal(error.error, undefined);
    return true;
  });
  assertNoListeners(caller.signal);
  await assert.rejects(client.getRankings('new', { signal: caller.signal }), SyntaxError);
  assertNoListeners(caller.signal);
  for (const category of ['volume', 'gainers']) {
    await assert.rejects(client.getRankings(category, { signal: caller.signal }), { name: 'TypeError', message: 'Invalid rankings response' });
    assertNoListeners(caller.signal);
  }
  caller.abort();
}

function repair32ConfigurationBounds() {
  const client = new RankingHttpClient();
  const baseUrl = 'http://127.0.0.1';
  for (const timeoutMs of [0, -1, 0.5, NaN, Infinity, MAX_RANKING_TIMEOUT_MS + 1]) {
    assert.throws(() => client.configure({ baseUrl, timeoutMs }), RangeError);
  }
  for (const timeoutMs of [1, DEFAULT_RANKING_TIMEOUT_MS, MAX_RANKING_TIMEOUT_MS]) {
    client.configure({ baseUrl, timeoutMs });
  }
}

test(repair32DefaultDeadline);
test(repair32StalledHeaders);
test(repair32StalledBodies);
test(repair32CallerCancellation);
test(repair32StreamedByteCaps);
test(repair32DeclaredByteCaps);
test(repair32ResponsesAndCleanup);
test(repair32ConfigurationBounds);

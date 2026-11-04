import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';
import { gzipSync } from 'node:zlib';
import { CandlesClient, CandlesHttpError, UdfResponseError } from '../dist/index.js';

// Task 22.33.1061 closes audit 22.30.41 finding 38 with finding38FastOversizedStreamCancelled
// and finding38DeclaredOversizeCancelled. No out-of-path wiring is required.
// Baseline verification is unrun: the task's ordered gate rules require implementation first.
const byteLimit = 1024 * 1024;
const time = 1800000000;
const historyRequest = { symbol: 'SID', resolution: '1', from: 0, to: 120 };
const config = {
  supports_search: true, supports_group_request: false, supports_marks: false,
  supports_timescale_marks: false, supports_time: true,
  exchanges: [{ value: 'sidiora', name: 'Sidiora', desc: 'Launchpad 🕯️' }],
  symbols_types: [{ name: 'token', value: 'token' }],
  supported_resolutions: ['1', '5', '15', '60', '240', '1D', '1W'],
  currency_codes: [{ id: 'USDL', code: 'USDL', description: 'USDL Stablecoin' }],
};
const symbol = {
  name: 'SID', full_name: 'SID/USDL', description: 'Token 🕯️', type: 'token', session: '24x7',
  exchange: 'sidiora', listed_exchange: 'sidiora', timezone: 'Etc/UTC',
  has_intraday: true, has_weekly_and_monthly: true,
  supported_resolutions: ['1', '5', '15', '60', '240', '1D', '1W'],
  pricescale: 100000000, ticker: 'SID', currency_code: 'USDL', has_empty_bars: false,
};
const bars = { s: 'ok', t: [60, 120], o: [1, 2], h: [3, 4], l: [0.5, 1], c: [2, 3], v: [100, 200] };

async function fixture(handler, run, timeoutMs = 10000) {
  const server = createServer(handler);
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const client = new CandlesClient();
  client.configure({ baseUrl: `http://127.0.0.1:${address.port}/udf`, timeoutMs });
  try {
    await run(client);
  } finally {
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
  }
}

function oversized(error) {
  assert.ok(error instanceof UdfResponseError);
  assert.equal(error.message, `Candles response exceeds ${byteLimit} bytes`);
  return true;
}

function largeResponse(response, state, headers = {}, status = 200) {
  const chunk = Buffer.alloc(64 * 1024, ' ');
  const paddingBytes = 16 * byteLimit;
  state.total = paddingBytes + String(time).length;
  state.sent = 0;
  state.closed = once(response, 'close');
  response.writeHead(status, { 'Content-Type': 'application/json', ...headers });
  response.flushHeaders();
  let scheduled;
  response.once('close', () => clearImmediate(scheduled));
  function pump() {
    if (response.destroyed) return;
    if (state.sent === paddingBytes) {
      state.sent += String(time).length;
      response.end(String(time));
      return;
    }
    state.sent += chunk.length;
    if (response.write(chunk)) scheduled = setImmediate(pump);
    else response.once('drain', () => { scheduled = setImmediate(pump); });
  }
  scheduled = setImmediate(pump);
}

async function finding38FastOversizedStreamCancelled() {
  const state = {};
  await fixture((request, response) => {
    assert.equal(request.url, '/udf/time');
    assert.equal(request.headers.accept, 'application/json');
    largeResponse(response, state);
  }, async client => {
    await assert.rejects(client.getTime(), oversized);
    await state.closed;
    assert.ok(state.sent >= byteLimit);
    assert.ok(state.sent < state.total, 'the server must observe cancellation before sending the entire fast body');
  });
}
test(finding38FastOversizedStreamCancelled);

async function finding38DeclaredOversizeCancelled() {
  const state = {};
  await fixture((_request, response) => {
    largeResponse(response, state, { 'Content-Length': 16 * byteLimit + String(time).length });
  }, async client => {
    await assert.rejects(client.getTime(), oversized);
    await state.closed;
    assert.ok(state.sent < state.total, 'declared oversize must be refused before full buffering');
  });
}
test(finding38DeclaredOversizeCancelled);

async function finding38ByteBoundary() {
  for (const declaredLength of [false, true]) {
    for (const length of [byteLimit, byteLimit + 1]) {
      const body = Buffer.from(`${' '.repeat(length - String(time).length)}${time}`);
      await fixture((_request, response) => {
        response.writeHead(200, {
          'Content-Type': 'application/json',
          ...(declaredLength ? { 'Content-Length': body.length } : {}),
        });
        response.write(body.subarray(0, 32768));
        response.end(body.subarray(32768));
      }, async client => {
        if (length === byteLimit) assert.equal(await client.getTime(), time);
        else await assert.rejects(client.getTime(), oversized);
      });
    }
  }
}
test(finding38ByteBoundary);

async function finding38CountsDecodedResponseBytes() {
  const body = gzipSync(Buffer.from(`${' '.repeat(2 * byteLimit)}${time}`));
  assert.ok(body.length < byteLimit);
  await fixture((_request, response) => {
    response.writeHead(200, {
      'Content-Type': 'application/json', 'Content-Encoding': 'gzip', 'Content-Length': body.length,
    });
    response.end(body);
  }, async client => {
    await assert.rejects(client.getTime(), oversized);
  });

  const unicode = JSON.stringify({ ...symbol, description: 'é'.repeat(600000) });
  assert.ok(unicode.length < byteLimit);
  assert.ok(Buffer.byteLength(unicode) > byteLimit);
  await fixture((_request, response) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.write(unicode);
    response.end();
  }, async client => {
    await assert.rejects(client.getSymbol('SID'), oversized);
  });
}
test(finding38CountsDecodedResponseBytes);

async function finding38PreservesDtos() {
  const seen = [];
  await fixture((request, response) => {
    const url = new URL(request.url, 'http://localhost');
    seen.push(url.pathname);
    const values = { '/udf/config': config, '/udf/symbols': symbol, '/udf/history': bars, '/udf/time': time };
    assert.ok(Object.hasOwn(values, url.pathname));
    const body = Buffer.from(JSON.stringify(values[url.pathname]));
    const unicodeOffset = body.indexOf(Buffer.from('🕯'));
    const split = unicodeOffset === -1 ? 1 : unicodeOffset + 1;
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.write(body.subarray(0, split));
    setImmediate(() => response.end(body.subarray(split)));
  }, async client => {
    assert.deepEqual(await client.getConfig(), config);
    assert.deepEqual(await client.getSymbol('SID'), symbol);
    assert.deepEqual(await client.getHistory(historyRequest), bars);
    assert.equal(await client.getTime(), time);
  });
  assert.deepEqual(seen, ['/udf/config', '/udf/symbols', '/udf/history', '/udf/time']);

  for (const [value, consume] of [
    [{ ...config, supports_time: 'true' }, client => client.getConfig()],
    [{ ...symbol, pricescale: 0 }, client => client.getSymbol('SID')],
    [{ ...bars, c: [2] }, client => client.getHistory(historyRequest)],
    ['1800000000', client => client.getTime()],
  ]) {
    await fixture((_request, response) => response.end(JSON.stringify(value)), async client => {
      await assert.rejects(consume(client), UdfResponseError);
    });
  }
  for (const body of ['', '{invalid']) {
    await fixture((_request, response) => response.end(body), async client => {
      await assert.rejects(client.getTime(), SyntaxError);
    });
  }
}
test(finding38PreservesDtos);

async function finding38PreservesFullHistoryPage() {
  const count = 5000;
  const value = Number.MAX_VALUE;
  const page = {
    s: 'ok', t: Array.from({ length: count }, (_, index) => Number.MAX_SAFE_INTEGER - count + index),
    ...Object.fromEntries(['o', 'h', 'l', 'c', 'v'].map(field => [field, Array(count).fill(value)])),
    nextCursor: `${'a'.repeat(127)}.${'b'.repeat(128)}`,
  };
  const body = JSON.stringify(page);
  assert.ok(Buffer.byteLength(body) < byteLimit);
  await fixture((_request, response) => response.end(body), async client => {
    assert.deepEqual(await client.getHistory({ symbol: 'SID', resolution: '1', to: Number.MAX_SAFE_INTEGER, countback: count }), page);
  });
}
test(finding38PreservesFullHistoryPage);

async function finding38PreservesHttpStatusAndCancelsBody() {
  const state = {};
  await fixture((_request, response) => {
    largeResponse(response, state, { 'Content-Length': 16 * byteLimit + String(time).length }, 503);
  }, async client => {
    await assert.rejects(client.getTime(), error => error instanceof CandlesHttpError && error.status === 503);
    await state.closed;
    assert.ok(state.sent < state.total);
  });
}
test(finding38PreservesHttpStatusAndCancelsBody);

async function finding38PreservesDeadlineAndCallerAbort() {
  for (const callerAbort of [false, true]) {
    let closed;
    let received;
    const bodyStarted = new Promise(resolve => { received = resolve; });
    await fixture((_request, response) => {
      closed = once(response, 'close');
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.write(' ');
      received();
    }, async client => {
      const controller = new AbortController();
      const pending = client.getTime({ signal: controller.signal });
      const rejected = assert.rejects(pending, error => {
        assert.equal(error.name, callerAbort ? 'AbortError' : 'TimeoutError');
        return true;
      });
      await bodyStarted;
      if (callerAbort) controller.abort();
      await rejected;
      await closed;
      const alreadyAborted = new AbortController();
      alreadyAborted.abort();
      await assert.rejects(client.getTime({ signal: alreadyAborted.signal }), error => error.name === 'AbortError');
    }, callerAbort ? 10000 : 100);
  }
}
test(finding38PreservesDeadlineAndCallerAbort);

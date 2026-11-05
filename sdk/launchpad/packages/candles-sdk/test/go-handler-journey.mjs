import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { CandlesClient, CandlesHttpError } from '../src/index.ts';

const repository = new URL('../../../../../', import.meta.url);
const fields = ['t', 'o', 'h', 'l', 'c', 'v'];

function assertPage(page, first, count) {
  assert.equal(page.s, 'ok');
  assert.deepEqual(page.t, Array.from({ length: count }, (_, index) => (first + index) * 60));
  for (const [field, value] of Object.entries({ o: 1e-18, h: 2e-18, l: 1e-18, c: 2e-18, v: 1.000001 })) {
    assert.deepEqual(page[field], Array(count).fill(value), field);
  }
}

function parameters(url) {
  return [...url.searchParams.entries()].sort(([left], [right]) => left.localeCompare(right));
}

export async function finding3GoHandlerJourney() {
  const transcript = JSON.parse(await readFile(new URL('./fixtures/go-udf-22-33-1045.json', import.meta.url), 'utf8'));
  assert.equal(transcript.producer.registration, 'RegisterUDFWithHistory');
  assert.match(transcript.producer.revision, /^[a-f0-9]{40}$/);
  assert.equal(transcript.pool, '0x000000000000000000000000000000000000eeee');
  assert.deepEqual(Object.keys(transcript.producer.sources).sort(), [
    'sdk/launchpad/shared/http/errors.go',
    'services/launchpad/trading-charts/internal/httpapi/history.go',
    'services/launchpad/trading-charts/internal/httpapi/symbols.go',
    'services/launchpad/trading-charts/internal/httpapi/udf.go',
  ]);
  for (const [path, digest] of Object.entries(transcript.producer.sources)) {
    const actual = createHash('sha256').update(await readFile(new URL(path, repository))).digest('hex');
    assert.equal(actual, digest, `Go producer changed: regenerate the transcript for ${path}`);
  }

  const seen = [];
  const failures = [];
  const server = createServer((request, response) => {
    const exchange = transcript.exchanges[seen.length];
    const actual = new URL(request.url, 'http://localhost');
    seen.push(actual);
    try {
      assert.ok(exchange, `Unrecorded request: ${request.url}`);
      const expected = new URL(exchange.request.url, 'http://localhost');
      assert.equal(request.method, exchange.request.method);
      assert.equal(actual.pathname, expected.pathname);
      assert.deepEqual(parameters(actual), parameters(expected));
      assert.equal(request.headers.accept, 'application/json');
      assert.equal(request.headers.cookie, undefined);
      assert.equal(request.headers.authorization, undefined);
      response.writeHead(exchange.response.status, exchange.response.headers);
      response.end(exchange.response.body);
    } catch (error) {
      failures.push(error);
      response.writeHead(500);
      response.end();
    }
  });

  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const client = new CandlesClient();
  client.configure({ baseUrl: `http://127.0.0.1:${server.address().port}/udf/` });
  const symbol = transcript.pool;
  const range = { symbol, resolution: '1', from: 60, to: 300180 };
  try {
    const config = await client.getConfig();
    assert.deepEqual(config, JSON.parse(transcript.exchanges[0].response.body));
    assert.deepEqual(config.supported_resolutions, ['1', '5', '15', '60', '240', '1D', 'D', '1W', 'W']);
    assert.equal(config.supports_time, true);
    assert.deepEqual(config.exchanges, [{ value: 'sidiora', name: 'Sidiora', desc: 'Sidiora Launchpad' }]);

    const resolved = await client.getSymbol(symbol);
    assert.deepEqual(resolved, {
      name: symbol, full_name: `${symbol}/USDL`, description: `Token ${symbol}`,
      type: 'token', session: '24x7', exchange: 'sidiora', listed_exchange: 'sidiora',
      timezone: 'Etc/UTC', has_intraday: true, has_weekly_and_monthly: true,
      supported_resolutions: config.supported_resolutions, pricescale: 100000000,
      ticker: symbol, currency_code: 'USDL', has_empty_bars: false,
    });
    assert.equal(await client.getTime(), JSON.parse(transcript.exchanges[2].response.body));

    const first = await client.getHistory(range);
    assertPage(first, 1, 5000);
    assert.equal(typeof first.nextCursor, 'string');
    assert.match(first.nextCursor, /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/);
    assert.ok(first.nextCursor.length <= 256);
    const tail = await client.getHistory({ ...range, cursor: first.nextCursor });
    assertPage(tail, 5001, 2);
    assert.equal(tail.nextCursor, undefined);
    const forward = first.t.concat(tail.t);
    assert.deepEqual(forward, Array.from({ length: 5002 }, (_, index) => (index + 1) * 60));
    assert.equal(new Set(forward).size, 5002);

    const countback = { symbol, resolution: '1', to: range.to, countback: 5000 };
    const latest = await client.getHistory(countback);
    assertPage(latest, 3, 5000);
    assert.equal(typeof latest.nextCursor, 'string');
    assert.notEqual(latest.nextCursor, first.nextCursor);
    const earlier = await client.getHistory({ ...countback, cursor: latest.nextCursor });
    assertPage(earlier, 1, 2);
    assert.equal(earlier.nextCursor, undefined);
    assert.deepEqual(earlier.t.concat(latest.t), forward);

    assert.deepEqual(await client.getHistory({ ...range, from: 300180, to: 300240 }), { s: 'no_data' });

    async function assertRefusal(operation, status, code, message, retryable) {
      const index = seen.length;
      await assert.rejects(operation, error => {
        assert.ok(error instanceof CandlesHttpError);
        assert.equal(error.status, status);
        assert.equal(error.message, `Candles HTTP request failed: ${status}`);
        return true;
      });
      const recorded = transcript.exchanges[index].response;
      assert.equal(recorded.status, status);
      const body = JSON.parse(recorded.body);
      assert.deepEqual(Object.keys(body).sort(), ['code', 'details', 'fieldErrors', 'message', 'requestId', 'retryAfterMs', 'retryable']);
      assert.equal(body.code, code);
      assert.equal(body.message, message);
      assert.equal(body.retryable, retryable);
      assert.equal(body.retryAfterMs, retryable ? 1000 : null);
      assert.deepEqual(body.fieldErrors, []);
      assert.equal(body.details, null);
      assert.match(body.requestId, /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/);
      if (retryable) assert.equal(recorded.headers['Retry-After'][0], '1');
    }

    for (const invalid of ['SID', 'SID & USDL']) {
      await assertRefusal(() => client.getSymbol(invalid), 422, 'FIELD_INVALID', 'symbol must be a pool address', false);
    }
    await assertRefusal(() => client.getHistory({ ...range, symbol: 'SID' }), 422, 'FIELD_INVALID', 'invalid pool address', false);
    const [body, signature] = first.nextCursor.split('.');
    const tampered = `${body}.${signature[0] === 'A' ? 'B' : 'A'}${signature.slice(1)}`;
    await assertRefusal(() => client.getHistory({ ...range, cursor: tampered }), 400, 'CURSOR_INVALID', 'invalid history cursor', false);
    await assertRefusal(() => client.getHistory({ ...range, to: 300240, cursor: first.nextCursor }), 400, 'CURSOR_INVALID', 'invalid history cursor', false);
    await assertRefusal(() => client.getHistory(range), 503, 'DEPENDENCY_UNAVAILABLE', 'history unavailable', true);
    await assertRefusal(() => client.getHistory(range), 503, 'INTEGRITY_QUARANTINED', 'malformed stored candle', false);

    assert.equal(seen.length, transcript.exchanges.length, 'Every recorded producer exchange must execute');
    assert.deepEqual(failures, [], 'Every SDK request must match the Go-handler transcript');
    assert.equal(seen.length, 15);
    for (const field of fields) assert.equal(first[field].length, 5000);
  } finally {
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
}

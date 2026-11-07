import { test } from 'node:test';
import assert from 'node:assert/strict';
import { getEventListeners } from 'node:events';
import { createServer } from 'node:http';
import { gzipSync } from 'node:zlib';
import {
  UsersHttpClient, UsersApiError, UsersResponseError, UsersResponseTooLargeError,
  USERS_DEFAULT_TIMEOUT_MS, USERS_MAX_TIMEOUT_MS, USERS_RESPONSE_LIMITS,
} from '../dist/index.js';

// Audit 22.30.41 finding 33: each named test exercises the production HTTP client.
const wallet = '0x' + 'a'.repeat(40);
const pool = '0x' + 'b'.repeat(40);
const auth = { signature: 'caller-signed-value', message: 'caller-message' };
const data = { display_name: 'Name', bio: '', website: '', twitter: '', telegram: '', discord: '' };
const profile = {
  wallet_address: wallet, display_name: null, bio: null,
  socials: { website: null, twitter: null, telegram: null, discord: null },
  images: { avatar: null, banner: null }, created_pools: [], created_at: null, updated_at: null,
};
const watchlist = { wallet_address: wallet, pools: [{ pool_address: pool, added_at: 1 }] };
const uploaded = { success: true, url: 'https://images.example/avatar.png' };
const routes = [
  { name: 'profile', method: 'GET', path: `/users/${wallet}`, payload: profile, limit: 1024 * 1024,
    call: (client, options) => client.getProfile(wallet, options) },
  { name: 'watchlist', method: 'GET', path: `/users/${wallet}/watchlist`, payload: watchlist, limit: 1024 * 1024,
    call: (client, options) => client.getWatchlist(wallet, options) },
  { name: 'update', method: 'POST', path: `/users/${wallet}`, payload: { success: true }, limit: 16 * 1024,
    call: (client, options) => client.updateProfile(wallet, { ...auth, data }, options) },
  { name: 'add', method: 'PUT', path: `/users/${wallet}/watchlist/${pool}`, payload: { success: true }, limit: 16 * 1024,
    call: (client, options) => client.addWatchlist(wallet, pool, auth, options) },
  { name: 'remove', method: 'DELETE', path: `/users/${wallet}/watchlist/${pool}`, payload: { success: true }, limit: 16 * 1024,
    call: (client, options) => client.removeWatchlist(wallet, pool, auth, options) },
  ...['avatar', 'banner'].map(kind => ({
    name: kind, method: 'POST', path: `/users/${wallet}/${kind}`, payload: uploaded, limit: 16 * 1024,
    call: (client, options) => client.uploadImage(wallet, kind, new Blob(['image'], { type: 'image/png' }), auth, options),
  })),
];

async function loopback(t, handler) {
  const server = createServer(handler);
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(async () => {
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  });
  return `http://127.0.0.1:${server.address().port}`;
}

function deferred() {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
}

async function bounded(promise, message) {
  let timer;
  try {
    return await Promise.race([
      promise,
      new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(message)), 2000); }),
    ]);
  } finally { clearTimeout(timer); }
}

function noCallerListeners(signal) {
  assert.deepEqual(getEventListeners(signal, 'abort'), []);
}

async function finding33_configuredDeadlineAndValidation(t) {
  const baseUrl = await loopback(t, (_req, res) => res.end(JSON.stringify(profile)));
  assert.equal(USERS_DEFAULT_TIMEOUT_MS, 10_000);
  assert.equal(USERS_MAX_TIMEOUT_MS, 120_000);
  assert.deepEqual(USERS_RESPONSE_LIMITS, {
    profile: 1024 * 1024, watchlist: 1024 * 1024, mutation: 16 * 1024, upload: 16 * 1024, error: 16 * 1024,
  });
  const client = new UsersHttpClient({ baseUrl, timeoutMs: 500 });
  for (const timeoutMs of [0, -1, 0.5, NaN, Infinity, 120_001, Number.MAX_SAFE_INTEGER]) {
    assert.throws(() => new UsersHttpClient({ baseUrl, timeoutMs }), RangeError);
    assert.throws(() => client.configure({ baseUrl: 'http://unreachable.invalid', timeoutMs }), RangeError);
  }
  assert.deepEqual(await client.getProfile(wallet), profile);
  client.configure({ baseUrl });
  assert.deepEqual(await client.getProfile(wallet), profile);
  assert.throws(() => client.configure({ baseUrl: 'ftp://invalid.example' }), TypeError);
}

async function finding33_defaultDeadlineWithoutCallerSignal(t) {
  const closed = deferred();
  let received = false;
  const baseUrl = await loopback(t, (_req, res) => {
    received = true;
    res.once('close', closed.resolve);
  });
  const client = new UsersHttpClient({ baseUrl });
  const started = performance.now();
  await assert.rejects(client.getProfile(wallet), error => error.name === 'TimeoutError');
  assert.equal(received, true);
  assert.ok(performance.now() - started < 14_000, 'the default deadline bounds stalled headers');
  await bounded(closed.promise, 'default deadline did not close the HTTP response');
}

async function finding33_deadlineCoversHeadersAndSuccessAndErrorBodies(t) {
  for (const stage of ['headers', 'body', 'error-body']) {
    await t.test(stage, async t => {
      const closed = deferred();
      let received = false;
      const baseUrl = await loopback(t, (_req, res) => {
        received = true;
        res.once('close', closed.resolve);
        if (stage !== 'headers') {
          res.writeHead(stage === 'error-body' ? 503 : 200, { 'Content-Type': 'application/json' });
          res.write('{');
        }
      });
      const client = new UsersHttpClient({ baseUrl, timeoutMs: 150 });
      const caller = new AbortController();
      await assert.rejects(client.getProfile(wallet, { signal: caller.signal }), error => error.name === 'TimeoutError');
      assert.equal(received, true);
      noCallerListeners(caller.signal);
      await bounded(closed.promise, `${stage} timeout did not close the response`);
    });
  }
}

async function finding33_combinesCallerCancellationAndCleansListeners(t) {
  let requests = 0;
  const arrived = deferred();
  const closed = deferred();
  const baseUrl = await loopback(t, (_req, res) => {
    requests++;
    res.once('close', closed.resolve);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.write('{');
    arrived.resolve();
  });
  const client = new UsersHttpClient({ baseUrl, timeoutMs: 120_000 });
  const preAborted = new AbortController();
  const beforeReason = new Error('cancelled before sending');
  preAborted.abort(beforeReason);
  await assert.rejects(client.getProfile(wallet, { signal: preAborted.signal }), error => error === beforeReason);
  assert.equal(requests, 0);
  noCallerListeners(preAborted.signal);

  const caller = new AbortController();
  const reason = new Error('cancelled during body');
  const pending = client.getProfile(wallet, { signal: caller.signal });
  const rejected = assert.rejects(pending, error => error === reason);
  await bounded(arrived.promise, 'request did not reach the HTTP server');
  assert.equal(getEventListeners(caller.signal, 'abort').length, 1);
  caller.abort(reason);
  await bounded(rejected, 'caller cancellation did not interrupt the body read');
  noCallerListeners(caller.signal);
  await bounded(closed.promise, 'caller cancellation did not close the response');

  const waiting = deferred();
  const waitingClosed = deferred();
  const headersUrl = await loopback(t, (_req, res) => {
    res.once('close', waitingClosed.resolve);
    waiting.resolve();
  });
  client.configure({ baseUrl: headersUrl, timeoutMs: 120_000 });
  const headersCaller = new AbortController();
  const headersReason = new DOMException('cancelled during headers', 'AbortError');
  const headersPending = assert.rejects(client.getProfile(wallet, { signal: headersCaller.signal }), error => error === headersReason);
  await bounded(waiting.promise, 'headers request did not arrive');
  headersCaller.abort(headersReason);
  await bounded(headersPending, 'caller cancellation did not interrupt headers');
  noCallerListeners(headersCaller.signal);
  await bounded(waitingClosed.promise, 'headers cancellation did not close the response');
}

async function finding33_endpointCapsAndExactBoundaries(t) {
  for (const route of routes) {
    await t.test(route.name, async t => {
      let oversize = false;
      const requests = [];
      const baseUrl = await loopback(t, async (req, res) => {
        let body = '';
        for await (const chunk of req) body += chunk;
        requests.push({ method: req.method, path: req.url, headers: req.headers, body });
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(route.payload).padEnd(route.limit + (oversize ? 1 : 0), ' '));
      });
      const client = new UsersHttpClient({ baseUrl, timeoutMs: 2000 });
      const caller = new AbortController();
      assert.deepEqual(await route.call(client, { signal: caller.signal }), route.payload);
      noCallerListeners(caller.signal);
      oversize = true;
      await assert.rejects(route.call(client, { signal: caller.signal }), error =>
        error instanceof UsersResponseTooLargeError && error instanceof UsersResponseError && error.limit === route.limit);
      noCallerListeners(caller.signal);
      for (const request of requests) {
        assert.equal(request.method, route.method);
        assert.equal(request.path, route.path);
        if (route.name === 'update') assert.deepEqual(JSON.parse(request.body), { ...auth, data });
        if (route.name === 'add' || route.name === 'remove') assert.deepEqual(JSON.parse(request.body), auth);
        if (route.name === 'avatar' || route.name === 'banner') {
          assert.match(request.headers['content-type'], /^multipart\/form-data; boundary=/);
          assert.ok(request.body.includes(`name="${route.name}"`));
          assert.match(request.body, /name="signature"/);
          assert.match(request.body, /caller-signed-value/);
          assert.match(request.body, /caller-message/);
        }
      }
    });
  }
}

async function finding33_streamingOversizeClosesBeforeEnd(t) {
  for (const status of [200, 503]) {
    await t.test(String(status), async t => {
      const closed = deferred();
      let response;
      const limit = status === 200 ? 1024 * 1024 : 16 * 1024;
      const baseUrl = await loopback(t, (_req, res) => {
        response = res;
        res.once('close', closed.resolve);
        res.writeHead(status, { 'Content-Type': 'application/json' });
        res.write(' '.repeat(limit));
        res.write(' ');
        // Deliberately leave the HTTP stream open after exceeding the byte cap.
      });
      const client = new UsersHttpClient({ baseUrl, timeoutMs: 3000 });
      const caller = new AbortController();
      await bounded(assert.rejects(client.getProfile(wallet, { signal: caller.signal }), error =>
        error instanceof UsersResponseTooLargeError && error.limit === limit), 'oversized response was buffered until EOF');
      noCallerListeners(caller.signal);
      await bounded(closed.promise, 'oversized stream was not cancelled');
      assert.equal(response.writableEnded, false);
    });
  }
}

async function finding33_countsUtf8AndDecompressedBytes(t) {
  // Fewer UTF-16 characters than the cap, but more UTF-8 bytes, with valid DTO shape.
  const payload = JSON.stringify({ ...profile, bio: 'é'.repeat(600_000) });
  assert.ok(payload.length < 1024 * 1024);
  assert.ok(Buffer.byteLength(payload) > 1024 * 1024);
  for (const compressed of [false, true]) {
    await t.test(compressed ? 'gzip' : 'utf8', async t => {
      const wire = compressed ? gzipSync(payload) : Buffer.from(payload);
      if (compressed) assert.ok(wire.byteLength < 16 * 1024);
      const baseUrl = await loopback(t, (_req, res) => {
        res.writeHead(200, {
          'Content-Type': 'application/json', 'Content-Length': wire.byteLength,
          ...(compressed ? { 'Content-Encoding': 'gzip' } : {}),
        });
        res.end(wire);
      });
      const client = new UsersHttpClient({ baseUrl });
      await assert.rejects(client.getProfile(wallet), error => error instanceof UsersResponseTooLargeError && error.limit === 1024 * 1024);
    });
  }
}

async function finding33_preservesErrorsShapesAndListenerCleanup(t) {
  let status = 200;
  let payload = JSON.stringify(profile);
  const baseUrl = await loopback(t, (_req, res) => {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    res.end(payload);
  });
  const client = new UsersHttpClient({ baseUrl, timeoutMs: 1000 });
  const caller = new AbortController();
  const options = { signal: caller.signal };
  for (let index = 0; index < 12; index++) {
    assert.deepEqual(await client.getProfile(wallet, options), profile);
    noCallerListeners(caller.signal);
  }
  for (const invalid of [
    '{', '{}', JSON.stringify({ ...profile, created_pools: null }),
    JSON.stringify({ ...profile, socials: {} }), JSON.stringify({ ...profile, updated_at: 2 ** 53 }),
    JSON.stringify({ ...profile, images: { avatar: 3, banner: null } }),
  ]) {
    payload = invalid;
    await assert.rejects(client.getProfile(wallet, options), UsersResponseError);
    noCallerListeners(caller.signal);
  }
  for (const route of routes.slice(1)) {
    payload = '{}';
    await assert.rejects(route.call(client, options), UsersResponseError);
    noCallerListeners(caller.signal);
  }
  status = 403;
  const apiBody = { statusCode: 403, error: 'Forbidden', message: 'Invalid signature' };
  payload = JSON.stringify(apiBody);
  await assert.rejects(client.getProfile(wallet, options), error => {
    assert.ok(error instanceof UsersApiError);
    assert.equal(error.status, 403);
    assert.deepEqual(error.body, apiBody);
    return true;
  });
  noCallerListeners(caller.signal);
  status = 502;
  payload = 'proxy unavailable'.padEnd(16 * 1024, ' ');
  await assert.rejects(client.getWatchlist(wallet, options), error =>
    error instanceof UsersApiError && error.status === 502 && error.body === payload);
  noCallerListeners(caller.signal);
  payload += ' ';
  for (const route of routes) {
    await assert.rejects(route.call(client, options), error => error instanceof UsersResponseTooLargeError && error.limit === 16 * 1024);
    noCallerListeners(caller.signal);
  }
  caller.abort(new Error('finished requests'));
}

async function finding33_networkFailureCleansListeners(t) {
  const baseUrl = await loopback(t, (_req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.write('{');
    setImmediate(() => res.destroy());
  });
  const caller = new AbortController();
  const client = new UsersHttpClient({ baseUrl, timeoutMs: 1000 });
  await assert.rejects(client.getProfile(wallet, { signal: caller.signal }), TypeError);
  noCallerListeners(caller.signal);
}

test(finding33_configuredDeadlineAndValidation);
test(finding33_defaultDeadlineWithoutCallerSignal, { timeout: 15_000 });
test(finding33_deadlineCoversHeadersAndSuccessAndErrorBodies, { timeout: 5000 });
test(finding33_combinesCallerCancellationAndCleansListeners, { timeout: 5000 });
test(finding33_endpointCapsAndExactBoundaries, { timeout: 15_000 });
test(finding33_streamingOversizeClosesBeforeEnd, { timeout: 10_000 });
test(finding33_countsUtf8AndDecompressedBytes, { timeout: 5000 });
test(finding33_preservesErrorsShapesAndListenerCleanup, { timeout: 10_000 });
test(finding33_networkFailureCleansListeners, { timeout: 5000 });
